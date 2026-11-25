package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Requirement to wiersz tabeli wymagań z requirements/*.md.
type Requirement struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Text string `json:"text"`
}

// Ref to odwołanie „REQ:” w teście albo w pliku korpusu. Test jest pusty, gdy odwołania nie da się przypisać
// do funkcji testowej (np. komentarz przy funkcji pomocniczej).
type Ref struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Package string `json:"package,omitempty"`
	Test    string `json:"test,omitempty"`
	Corpus  bool   `json:"corpus,omitempty"`
}

// Key to klucz wyniku testu: ścieżka importu pakietu i nazwa testu najwyższego poziomu.
func (r Ref) Key() string { return r.Package + "." + r.Test }

var prefixRow = regexp.MustCompile("^\\| `([A-Z]{2,5})` \\|")

// parseRequirements czyta prefiksy z tabeli w requirements/README.md i wymagania ze wszystkich pozostałych plików
// requirements/*.md, w kolejności plików i wierszy.
func parseRequirements(root string) ([]Requirement, *regexp.Regexp, error) {
	readme, err := os.ReadFile(filepath.Join(root, "requirements", "README.md"))
	if err != nil {
		return nil, nil, err
	}
	var prefixes []string
	for _, line := range strings.Split(string(readme), "\n") {
		if m := prefixRow.FindStringSubmatch(line); m != nil {
			prefixes = append(prefixes, m[1])
		}
	}
	if len(prefixes) == 0 {
		return nil, nil, fmt.Errorf("no prefixes in requirements/README.md")
	}
	idRe := regexp.MustCompile(`\b(?:` + strings.Join(prefixes, "|") + `)-\d{3}\b`)
	files, _ := filepath.Glob(filepath.Join(root, "requirements", "*.md"))
	sort.Strings(files)
	var reqs []Requirement
	seen := map[string]bool{}
	for _, f := range files {
		if filepath.Base(f) == "README.md" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		for _, line := range strings.Split(string(b), "\n") {
			id := idRe.FindString(line)
			if id == "" || !strings.HasPrefix(line, "| "+id+" |") {
				continue
			}
			if seen[id] {
				return nil, nil, fmt.Errorf("requirement %s defined twice", id)
			}
			seen[id] = true
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "| "+id+" |")), "|"))
			reqs = append(reqs, Requirement{ID: id, File: rel(root, f), Text: text})
		}
	}
	return reqs, idRe, nil
}

// skipDirs to katalogi, których nie przeszukujemy: dane pobrane, wyniki i metadane.
var skipDirs = map[string]bool{".git": true, ".claude": true, "out": true, "node_modules": true}

// collectRefs zbiera odwołania „REQ:” z komentarzy w plikach *_test.go (każdego modułu w repozytorium) i z plików
// testdata/corpus/*. Komentarz wewnątrz funkcji należy do tej funkcji; komentarz poza funkcją — do najbliższej
// funkcji poniżej. Funkcje Test*, Fuzz*, Example* i Benchmark* są dowodem; inne (pomocnicze) — dowodem wspierającym.
func collectRefs(root string, idRe *regexp.Regexp) ([]Ref, error) {
	var refs []Ref
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (skipDirs[name] || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fileRefs, err := refsInGoFile(root, p, idRe)
		refs = append(refs, fileRefs...)
		return err
	})
	if err != nil {
		return nil, err
	}
	corpus, _ := filepath.Glob(filepath.Join(root, "testdata", "corpus", "*"))
	sort.Strings(corpus)
	for _, f := range corpus {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.Index(line, "REQ:"); i >= 0 {
				for _, id := range idRe.FindAllString(line[i:], -1) {
					refs = append(refs, Ref{ID: id, File: rel(root, f), Corpus: true})
				}
			}
		}
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].ID != refs[j].ID {
			return refs[i].ID < refs[j].ID
		}
		return refs[i].File+refs[i].Test < refs[j].File+refs[j].Test
	})
	return dedupRefs(refs), nil
}

func refsInGoFile(root, p string, idRe *regexp.Regexp) ([]Ref, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel(root, p), err)
	}
	pkg, err := importPath(root, filepath.Dir(p))
	if err != nil {
		return nil, err
	}
	var funcs []*ast.FuncDecl
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			funcs = append(funcs, fn)
		}
	}
	var refs []Ref
	for _, cg := range f.Comments {
		text := cg.Text()
		i := strings.Index(text, "REQ:")
		if i < 0 {
			continue
		}
		ids := idRe.FindAllString(text[i:], -1)
		var owner *ast.FuncDecl
		for _, fn := range funcs {
			if cg.Pos() >= fn.Pos() && cg.End() <= fn.End() {
				owner = fn
				break
			}
		}
		if owner == nil {
			for _, fn := range funcs {
				if fn.Pos() > cg.End() {
					owner = fn
					break
				}
			}
		}
		test := ""
		if owner != nil && isTestName(owner.Name.Name) {
			test = owner.Name.Name
		}
		for _, id := range ids {
			refs = append(refs, Ref{ID: id, File: rel(root, p), Package: pkg, Test: test})
		}
	}
	return refs, nil
}

func isTestName(name string) bool {
	for _, prefix := range []string{"Test", "Fuzz", "Example", "Benchmark"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func dedupRefs(refs []Ref) []Ref {
	seen := map[Ref]bool{}
	out := refs[:0]
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// importPath ustala ścieżkę importu katalogu z najbliższego go.mod w górę drzewa (do root włącznie).
func importPath(root, dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		b, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			mod := ""
			for _, line := range strings.Split(string(b), "\n") {
				if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					mod = strings.TrimSpace(m)
					break
				}
			}
			if mod == "" {
				return "", fmt.Errorf("%s/go.mod: no module line", d)
			}
			r, err := filepath.Rel(d, dir)
			if err != nil {
				return "", err
			}
			if r == "." {
				return mod, nil
			}
			return mod + "/" + filepath.ToSlash(r), nil
		}
		if d == root || d == filepath.Dir(d) {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}

// TestRun to wynik jednego przebiegu `go test -json`.
type TestRun struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
	// FailedTests i SkippedTests to klucze testów najwyższego poziomu (pakiet.Test), posortowane.
	FailedTests  []string `json:"failed_tests,omitempty"`
	SkippedTests []string `json:"skipped_tests,omitempty"`
	// BuildFailed to pakiety, które się nie skompilowały albo zakończyły bez wyniku testu.
	BuildFailed []string `json:"build_failed,omitempty"`
	results     map[string]string
	output      map[string][]string
}

type testEvent struct {
	Action, Package, Test, Output string
}

// parseTestEvents czyta strumień `go test -json` i zbiera wyniki oraz wyjście testów najwyższego poziomu.
func parseTestEvents(name, command string, r io.Reader) (*TestRun, error) {
	run := &TestRun{Name: name, Command: command, results: map[string]string{}, output: map[string][]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	pkgFailed := map[string]bool{}
	pkgHasTests := map[string]bool{}
	for sc.Scan() {
		var e testEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Test == "" {
			if e.Action == "fail" {
				pkgFailed[e.Package] = true
			}
			continue
		}
		if strings.Contains(e.Test, "/") {
			continue
		}
		key := e.Package + "." + e.Test
		pkgHasTests[e.Package] = true
		switch e.Action {
		case "pass", "fail", "skip":
			run.results[key] = e.Action
		case "output":
			run.output[key] = append(run.output[key], strings.TrimRight(e.Output, "\n"))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for key, action := range run.results {
		switch action {
		case "pass":
			run.Passed++
		case "fail":
			run.Failed++
			run.FailedTests = append(run.FailedTests, key)
		case "skip":
			run.Skipped++
			run.SkippedTests = append(run.SkippedTests, key)
		}
	}
	for pkg := range pkgFailed {
		if !pkgHasTests[pkg] {
			run.BuildFailed = append(run.BuildFailed, pkg)
		}
	}
	sort.Strings(run.FailedTests)
	sort.Strings(run.SkippedTests)
	sort.Strings(run.BuildFailed)
	return run, nil
}

// Result zwraca wynik testu (pass, fail, skip) albo "" dla nieznanego.
func (r *TestRun) Result(key string) string { return r.results[key] }

// Output zwraca wiersze wyjścia testu.
func (r *TestRun) Output(key string) []string { return r.output[key] }

// Trace to wiersz macierzy śledzenia: wymaganie, jego dowody i wynik.
type Trace struct {
	Requirement
	Evidence []string `json:"evidence"`
	// Status: pass (wszystkie testy przeszły), manual (dowód uruchamiany na żądanie: benchmark albo test pomijany
	// bez zmiennej GOXSDGO_*), skip (test pominięty, np. bez pobranych danych), unresolved (tylko funkcje pomocnicze
	// albo test nieznany w przebiegu), fail, missing (brak dowodu).
	Status string `json:"status"`
}

var statusRank = map[string]int{"pass": 0, "manual": 1, "skip": 2, "unresolved": 3, "fail": 4, "missing": 5}

// buildTraces łączy wymagania, odwołania i wyniki przebiegów w macierz śledzenia; zwraca też odwołania do
// nieistniejących wymagań. Odwołanie przy funkcji pomocniczej (np. sprawdzeniu wołanym przez kilka testów) jest
// dowodem wspierającym: nie pogarsza stanu, ale samo nie wystarcza.
func buildTraces(reqs []Requirement, refs []Ref, runs []*TestRun) ([]Trace, []Ref) {
	defined := map[string]bool{}
	for _, r := range reqs {
		defined[r.ID] = true
	}
	byID := map[string][]Ref{}
	var unknown []Ref
	for _, ref := range refs {
		if !defined[ref.ID] {
			unknown = append(unknown, ref)
			continue
		}
		byID[ref.ID] = append(byID[ref.ID], ref)
	}
	lookup := func(key string) (string, []string) {
		for _, run := range runs {
			if a := run.Result(key); a != "" {
				return a, run.Output(key)
			}
		}
		return "", nil
	}
	var traces []Trace
	for _, req := range reqs {
		t := Trace{Requirement: req, Status: "pass"}
		resolved, supporting := 0, 0
		worse := func(s string) {
			if statusRank[s] > statusRank[t.Status] {
				t.Status = s
			}
		}
		for _, ref := range byID[req.ID] {
			switch {
			case ref.Corpus:
				t.Evidence = append(t.Evidence, ref.File)
				resolved++
			case ref.Test == "":
				t.Evidence = append(t.Evidence, ref.File+" (funkcja pomocnicza)")
				supporting++
			case strings.HasPrefix(ref.Test, "Benchmark"):
				t.Evidence = append(t.Evidence, ref.Key()+" (benchmark, na żądanie)")
				resolved++
				worse("manual")
			default:
				resolved++
				res, out := lookup(ref.Key())
				switch res {
				case "pass":
					t.Evidence = append(t.Evidence, ref.Key())
				case "skip":
					if onDemand(out) {
						t.Evidence = append(t.Evidence, ref.Key()+" (na żądanie)")
						worse("manual")
					} else {
						t.Evidence = append(t.Evidence, ref.Key())
						worse("skip")
					}
				case "fail":
					t.Evidence = append(t.Evidence, ref.Key())
					worse("fail")
				default:
					t.Evidence = append(t.Evidence, ref.Key()+" (brak w przebiegu)")
					worse("unresolved")
				}
			}
		}
		switch {
		case resolved == 0 && supporting > 0:
			t.Status = "unresolved"
		case resolved == 0:
			t.Status = "missing"
		}
		traces = append(traces, t)
	}
	return traces, unknown
}

// onDemand mówi, czy test pominięto dlatego, że uruchamia się go zmienną środowiskową GOXSDGO_* (np. zrzut błędów),
// a nie z braku danych.
func onDemand(output []string) bool {
	for _, l := range output {
		if strings.Contains(l, "GOXSDGO_") && !strings.Contains(l, "not fetched") {
			return true
		}
	}
	return false
}

// W3CRow to wiersz raportu TestW3CSuite dla jednego dostawcy (albo „all”).
type W3CRow struct {
	Contributor string `json:"contributor"`
	SchemaOK    int    `json:"schema_passed"`
	SchemaIn    int    `json:"schema_in_scope"`
	InstanceOK  int    `json:"instance_passed"`
	InstanceIn  int    `json:"instance_in_scope"`
	TotalOK     int    `json:"total_passed"`
	TotalIn     int    `json:"total_in_scope"`
}

// W3CExclusion to liczba przypadków wyłączonych z danego powodu.
type W3CExclusion struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

var (
	w3cRowRe      = regexp.MustCompile(`^\s*(\S+)\s+schema\s+(\d+)/(\d+)\s+instance\s+(\d+)/(\d+)\s+total\s+(\d+)/(\d+)`)
	w3cExcludedRe = regexp.MustCompile(`^\s*excluded\s+(\d+): (.+)$`)
)

// parseW3C odczytuje tabelę zaliczeń i wyłączeń z wyjścia TestW3CSuite.
func parseW3C(lines []string) ([]W3CRow, []W3CExclusion) {
	var rows []W3CRow
	var excl []W3CExclusion
	for _, l := range lines {
		for _, part := range strings.Split(l, "\n") {
			if m := w3cRowRe.FindStringSubmatch(part); m != nil {
				n := atoi(m[2:])
				rows = append(rows, W3CRow{m[1], n[0], n[1], n[2], n[3], n[4], n[5]})
			} else if m := w3cExcludedRe.FindStringSubmatch(part); m != nil {
				c, _ := strconv.Atoi(m[1])
				excl = append(excl, W3CExclusion{strings.TrimSpace(m[2]), c})
			}
		}
	}
	return rows, excl
}

func atoi(ss []string) []int {
	out := make([]int, len(ss))
	for i, s := range ss {
		out[i], _ = strconv.Atoi(s)
	}
	return out
}

// findCount szuka w wierszach wyjścia pierwszego dopasowania wyrażenia z dwiema liczbami (x/y).
func findCount(lines []string, re *regexp.Regexp) (int, int, bool) {
	for _, l := range lines {
		if m := re.FindStringSubmatch(l); m != nil {
			n := atoi(m[1:3])
			return n[0], n[1], true
		}
	}
	return 0, 0, false
}

// Ratchet to nagłówek pliku zapadki.
type Ratchet struct {
	File    string   `json:"file"`
	InScope int      `json:"in_scope"`
	Passing int      `json:"passing"`
	Mode    string   `json:"mode"`
	Listed  int      `json:"listed"`
	IDs     []string `json:"-"`
}

var ratchetCountsRe = regexp.MustCompile(`^# in scope (\d+), passing (\d+)`)

// readRatchets czyta nagłówki wszystkich plików *.ratchet w testdata (bez pobranych katalogów).
func readRatchets(root string) ([]Ratchet, error) {
	var out []Ratchet
	err := filepath.WalkDir(filepath.Join(root, "testdata"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(p, ".ratchet") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		r := Ratchet{File: rel(root, p), Mode: "passing"}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case ratchetCountsRe.MatchString(line):
				n := atoi(ratchetCountsRe.FindStringSubmatch(line)[1:3])
				r.InScope, r.Passing = n[0], n[1]
			case strings.HasPrefix(line, "# mode: "):
				r.Mode = strings.TrimPrefix(line, "# mode: ")
			case line == "" || strings.HasPrefix(line, "#"):
			default:
				r.IDs = append(r.IDs, line)
			}
		}
		r.Listed = len(r.IDs)
		out = append(out, r)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, err
}

// NamedCase to przypadek W3C, który nie przechodzi, i informacja, czy differences.md go nazywa.
type NamedCase struct {
	ID    string `json:"id"`
	Named bool   `json:"named"`
}

var (
	wordRe  = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_]*`)
	rangeRe = regexp.MustCompile(`\b([A-Za-z_]+?)(\d+)[–-]([A-Za-z_]*?)(\d+)\b`)
)

// namedInDifferences sprawdza dla każdego niezaliczonego przypadku W3C, czy differences.md wymienia jego grupę albo
// nazwę testu (dosłownie, bez przyrostka numeru grupy MS „_123” i rozszerzenia, albo w zakresie typu „xv002–xv100”).
func namedInDifferences(ids []string, differences string) []NamedCase {
	words := map[string]bool{}
	for _, w := range wordRe.FindAllString(differences, -1) {
		words[w] = true
	}
	type span struct {
		prefix   string
		from, to int
	}
	var spans []span
	for _, m := range rangeRe.FindAllStringSubmatch(differences, -1) {
		if m[3] != "" && m[3] != m[1] {
			continue
		}
		from, _ := strconv.Atoi(m[2])
		to, _ := strconv.Atoi(m[4])
		spans = append(spans, span{m[1], from, to})
	}
	numbered := regexp.MustCompile(`^([A-Za-z_]+?)(\d+)`)
	candidates := func(id string) []string {
		parts := strings.Split(id, "/")
		var out []string
		for _, p := range parts[1:] {
			p = strings.TrimSuffix(p, path.Ext(p))
			p = strings.TrimSuffix(p, path.Ext(p))
			out = append(out, p, regexp.MustCompile(`_\d+$`).ReplaceAllString(p, ""))
			for _, prefix := range []string{"particles", "elem", "att", "ste"} {
				if s, ok := strings.CutPrefix(p, prefix); ok && s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	var out []NamedCase
	for _, id := range ids {
		named := false
		for _, c := range candidates(id) {
			if words[c] {
				named = true
				break
			}
			if m := numbered.FindStringSubmatch(c); m != nil {
				n, _ := strconv.Atoi(m[2])
				for _, s := range spans {
					if s.prefix == m[1] && n >= s.from && n <= s.to {
						named = true
					}
				}
			}
		}
		out = append(out, NamedCase{id, named})
	}
	return out
}

// FuzzRun to wiersz dziennika fuzzingu z docs/fuzzing.md.
type FuzzRun struct {
	Date     string `json:"date"`
	Commit   string `json:"commit"`
	Package  string `json:"package"`
	Fuzzer   string `json:"fuzzer"`
	Duration string `json:"duration"`
	Execs    string `json:"execs"`
	Result   string `json:"result"`
	// Dirty: przebieg na kodzie z niezatwierdzonymi zmianami (oznaczenie „(zmiany)”); nie liczy się do K5.
	Dirty bool `json:"dirty,omitempty"`
	// Counted: wiersz liczy się do K5 (wynik „ok”, commit kandydata albo bez zmian biblioteki, bez „(zmiany)”).
	Counted bool `json:"counted"`
	seconds time.Duration
}

var (
	hexRe      = regexp.MustCompile(`[0-9a-f]{7,40}`)
	fuzzDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
)

// parseFuzzJournal czyta tabelę przebiegów z docs/fuzzing.md: wiersze z datą, commitem i nazwą fuzzera.
func parseFuzzJournal(text string) []FuzzRun {
	var out []FuzzRun
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 7 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		fuzzer := strings.Trim(cells[3], "`")
		if !fuzzDateRe.MatchString(cells[0]) || !strings.HasPrefix(fuzzer, "Fuzz") {
			continue
		}
		r := FuzzRun{Date: cells[0], Commit: hexRe.FindString(cells[1]), Package: cells[2], Fuzzer: fuzzer,
			Duration: cells[4], Execs: cells[5], Result: strings.Join(cells[6:], "|")}
		r.Dirty = strings.Contains(cells[1], "(zmiany)") || strings.Contains(r.Result, "(zmiany)")
		if d, err := time.ParseDuration(r.Duration); err == nil {
			r.seconds = d
		}
		out = append(out, r)
	}
	return out
}

// countFuzzHours oznacza wiersze liczone do K5 i zwraca ich łączny czas. same mówi, czy commit ma ten sam kod
// biblioteki co kandydat.
func countFuzzHours(runs []FuzzRun, same func(commit string) bool) time.Duration {
	var total time.Duration
	for i := range runs {
		r := &runs[i]
		r.Counted = strings.HasPrefix(r.Result, "ok") && !r.Dirty && r.seconds > 0 && r.Commit != "" && same(r.Commit)
		if r.Counted {
			total += r.seconds
		}
	}
	return total
}

// ADR to decyzja z docs/adr.
type ADR struct {
	File   string `json:"file"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func readADRs(root string) []ADR {
	files, _ := filepath.Glob(filepath.Join(root, "docs", "adr", "[0-9]*.md"))
	sort.Strings(files)
	var out []ADR
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		a := ADR{File: rel(root, f)}
		for _, line := range strings.Split(string(b), "\n") {
			if a.Title == "" && strings.HasPrefix(line, "# ") {
				a.Title = strings.TrimPrefix(line, "# ")
			}
			if s, ok := strings.CutPrefix(line, "Status: "); ok && a.Status == "" {
				a.Status = strings.TrimSpace(s)
			}
		}
		out = append(out, a)
	}
	return out
}

// DiffSection to sekcja docs/differences.md z liczbą wierszy tabeli.
type DiffSection struct {
	Title string `json:"title"`
	Rows  int    `json:"rows"`
}

func diffSections(text string) []DiffSection {
	var out []DiffSection
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			out = append(out, DiffSection{Title: strings.TrimPrefix(line, "## ")})
		case len(out) > 0 && strings.HasPrefix(line, "| ") && !strings.HasPrefix(line, "|---"):
			out[len(out)-1].Rows++
		}
	}
	for i := range out {
		if out[i].Rows > 0 {
			out[i].Rows-- // wiersz nagłówka tabeli
		}
	}
	return out
}

// SourceSet to zestaw danych testowych z testdata/sources.json.
type SourceSet struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Dest       string `json:"dest"`
	Files      int    `json:"files"`
	SHA256     string `json:"sha256,omitempty"`
	TreeSHA256 string `json:"tree_sha256,omitempty"`
	LocalOnly  int    `json:"local_only,omitempty"`
}

func readSources(root string) ([]SourceSet, string, error) {
	b, err := os.ReadFile(filepath.Join(root, "testdata", "sources.json"))
	if err != nil {
		return nil, "", err
	}
	var m struct {
		Sets []struct {
			Name, Kind, Dest string
			SHA256           string `json:"sha256"`
			TreeSHA256       string `json:"tree_sha256"`
			FileCount        int    `json:"file_count"`
			Files            []struct {
				LocalOnly bool `json:"local_only"`
			}
		}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, "", err
	}
	var out []SourceSet
	for _, s := range m.Sets {
		set := SourceSet{Name: s.Name, Kind: s.Kind, Dest: s.Dest, SHA256: s.SHA256, TreeSHA256: s.TreeSHA256, Files: s.FileCount}
		if s.Kind == "files" {
			set.Files = len(s.Files)
			for _, f := range s.Files {
				if f.LocalOnly {
					set.LocalOnly++
				}
			}
		}
		out = append(out, set)
	}
	sum := sha256.Sum256(b)
	return out, hex.EncodeToString(sum[:]), nil
}

// Measurements to docs/measurements.json: pomiary spoza `go test`.
type Measurements struct {
	Candidate struct {
		Commit string `json:"commit"`
		Note   string `json:"note"`
	} `json:"candidate"`
	K3 struct {
		Date, Commit, Method            string
		InstanceVerdictSamePercent      float64 `json:"instance_verdict_same_percent"`
		InstanceComparable              int     `json:"instance_comparable"`
		FirstErrorLineSamePercent       float64 `json:"first_error_line_same_percent"`
		FirstErrorLineCompared          int     `json:"first_error_line_compared"`
		SchemaVerdictSamePercent        float64 `json:"schema_verdict_same_percent"`
		SchemaComparable                int     `json:"schema_comparable"`
		CorpusVerdictSame               string  `json:"corpus_verdict_same"`
		PrivateCorpusVerdictSame        string  `json:"private_corpus_verdict_same"`
		PrivateCorpusFirstErrorLineSame string  `json:"private_corpus_first_error_line_same"`
		VerdictThresholdPercent         float64 `json:"verdict_threshold_percent"`
		LineThresholdPercent            float64 `json:"line_threshold_percent"`
		DifferencesFile                 string  `json:"differences_file"`
		PrivateCorpusCommit             string  `json:"private_corpus_commit"`
	} `json:"k3"`
	K6 struct {
		Date, Commit, Method string
		SizeMB               float64 `json:"size_mb"`
		PeakMB               float64 `json:"peak_mb"`
		LimitMB              float64 `json:"limit_mb"`
	} `json:"k6"`
	K7 struct {
		Date, Commit, Method      string
		SyntheticSizeMB           float64 `json:"synthetic_size_mb"`
		SyntheticRatio            float64 `json:"synthetic_ratio"`
		PrivateCorpusFiles        int     `json:"private_corpus_files"`
		PrivateCorpusSizeMB       float64 `json:"private_corpus_size_mb"`
		PrivateCorpusRatio        float64 `json:"private_corpus_ratio"`
		PrivateCorpusLargestRatio float64 `json:"private_corpus_largest_ratio"`
		LimitRatio                float64 `json:"limit_ratio"`
	} `json:"k7"`
	K8 struct {
		Status, Consumer, Branch, Commit, Date, Result, Note string
	} `json:"k8"`
}

func readMeasurements(root string) (*Measurements, error) {
	b, err := os.ReadFile(filepath.Join(root, "docs", "measurements.json"))
	if err != nil {
		return nil, err
	}
	var m Measurements
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("docs/measurements.json: %w", err)
	}
	return &m, nil
}

var diffCountRe = regexp.MustCompile(`Rozbieżności: (\d+) .*bez kategorii: (\d+)`)

// differencesLibxml2 odczytuje z docs/differences-libxml2.md liczbę rozbieżności i liczbę bez kategorii.
func differencesLibxml2(text string) (total, uncategorized int, stamp string, ok bool) {
	for _, line := range strings.Split(text, "\n") {
		if s, found := strings.CutPrefix(line, "Stan: "); found && stamp == "" {
			stamp, _, _ = strings.Cut(strings.TrimSpace(s), ". ")
			stamp = strings.TrimSuffix(stamp, ".")
		}
		if m := diffCountRe.FindStringSubmatch(line); m != nil {
			n := atoi(m[1:3])
			total, uncategorized, ok = n[0], n[1], true
		}
	}
	return
}

func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}
