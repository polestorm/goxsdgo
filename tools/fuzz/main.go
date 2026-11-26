// Command fuzz prowadzi kampanię fuzzingu goxsdgo (kryterium K5, ADR 0011): znajduje wszystkie fuzzery w module
// biblioteki, uruchamia je po kolei przez `go test -fuzz` z równym podziałem czasu i dopisuje wynik każdego
// przebiegu do dziennika docs/fuzzing.md (data, commit, fuzzer, czas, wykonania, wynik).
//
// Użycie (z katalogu głównego repozytorium):
//
//	go -C tools/fuzz run . -root ../.. -duration 3h          # wszystkie fuzzery, łącznie 3 h
//	go -C tools/fuzz run . -root ../.. -duration 21h -rounds 3 # trzy rundy po 7 h
//	go -C tools/fuzz run . -root ../.. -only Validate -duration 30m
//	go -C tools/fuzz run . -root ../.. -list                  # tylko lista fuzzerów
//	go -C tools/fuzz run . -root ../.. -pack ../../out/fuzzpack # paczka dla serwera bez Go (pack.go)
//	go -C tools/fuzz run . -root ../.. -import journal.tsv      # wiersze z dziennika serwera do docs/fuzzing.md
//
// Pełne wyjście każdego przebiegu trafia do <root>/out/fuzz/. Znalezione wejście, które łamie niezmiennik, Go zapisuje
// w testdata/fuzz/<Fuzzer>/ pakietu; trzeba je przejrzeć, naprawić błąd i zostawić plik jako test regresji.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fuzzer struct {
	pkg  string // ścieżka pakietu względem root, np. ./xmlreader
	name string
}

type result struct {
	fuzzer
	start    time.Time
	elapsed  time.Duration
	execs    int64
	ok       bool
	detail   string
	logPath  string
	commit   string
	modified bool
	host     string // gdzie przebieg działał, gdy nie lokalnie (np. "Synology")
}

var (
	funcRe  = regexp.MustCompile(`(?m)^func (Fuzz\w+)\(f \*testing\.F\)`)
	execsRe = regexp.MustCompile(`execs: (\d+)`)
	failRe  = regexp.MustCompile(`Failing input written to (\S+)`)
)

func main() {
	root := flag.String("root", ".", "repository root")
	duration := flag.Duration("duration", 24*time.Hour, "total fuzzing time across all fuzzers and rounds")
	rounds := flag.Int("rounds", 1, "number of rounds; each round runs every fuzzer once")
	only := flag.String("only", "", "regular expression selecting fuzzers by name")
	parallel := flag.Int("parallel", max(1, runtime.NumCPU()-2), "fuzzing workers per fuzzer (-parallel of go test)")
	journal := flag.String("journal", "docs/fuzzing.md", "journal file, relative to -root")
	list := flag.Bool("list", false, "only list the fuzzers")
	packDir := flag.String("pack", "", "build a self-contained package for a server without Go into this directory")
	targets := flag.String("targets", "linux/amd64,linux/arm64", "-pack: GOOS/GOARCH list")
	withData := flag.Bool("data", true, "-pack: include the fetched test data the fuzzers read as seed inputs")
	importPath := flag.String("import", "", "append the rows of a server journal (journal.tsv) to the journal")
	host := flag.String("host", "Synology", "-import: where the runs happened")
	flag.Parse()

	fuzzers, err := discover(*root)
	if err != nil {
		fail(err)
	}
	if *only != "" {
		re, err := regexp.Compile(*only)
		if err != nil {
			fail(err)
		}
		var sel []fuzzer
		for _, f := range fuzzers {
			if re.MatchString(f.name) {
				sel = append(sel, f)
			}
		}
		fuzzers = sel
	}
	if len(fuzzers) == 0 {
		fail(fmt.Errorf("no fuzzers selected"))
	}
	if *list {
		for _, f := range fuzzers {
			fmt.Printf("%s %s\n", f.pkg, f.name)
		}
		return
	}
	if *packDir != "" {
		if err := pack(*root, *packDir, fuzzers, strings.Split(*targets, ","), *withData); err != nil {
			fail(err)
		}
		return
	}
	if *importPath != "" {
		if err := importJournal(*importPath, journalPath(*root, *journal), *host); err != nil {
			fail(err)
		}
		return
	}
	slot := (*duration / time.Duration(len(fuzzers)**rounds)).Round(time.Second)
	if slot < 10*time.Second {
		fail(fmt.Errorf("%v per fuzzer is too short", slot))
	}
	commit, modified := gitState(*root)
	fmt.Printf("campaign: %d fuzzers × %d round(s) × %v, commit %s%s, -parallel %d\n",
		len(fuzzers), *rounds, slot, commit, dirtyMark(modified), *parallel)
	logDir := filepath.Join(*root, "out", "fuzz")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fail(err)
	}
	var results []result
	failed := 0
	for r := 0; r < *rounds; r++ {
		for _, f := range fuzzers {
			res := run(*root, logDir, f, slot, *parallel)
			res.commit, res.modified = commit, modified
			results = append(results, res)
			status := "ok"
			if !res.ok {
				status = "FAIL: " + res.detail
				failed++
			}
			fmt.Printf("%-22s %-20s %8v %12d execs  %s\n", f.pkg, f.name, res.elapsed.Round(time.Second), res.execs, status)
			if err := appendJournal(journalPath(*root, *journal), []result{res}); err != nil {
				fail(err)
			}
		}
	}
	var total time.Duration
	var execs int64
	for _, r := range results {
		total += r.elapsed
		execs += r.execs
	}
	fmt.Printf("total %v, %d execs, %d failure(s); journal: %s\n", total.Round(time.Second), execs, failed, *journal)
	if failed > 0 {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fuzz:", err)
	os.Exit(1)
}

// journalPath zwraca ścieżkę dziennika: bezwzględną bez zmian, względną względem root.
func journalPath(root, journal string) string {
	if filepath.IsAbs(journal) {
		return journal
	}
	return filepath.Join(root, journal)
}

func dirtyMark(m bool) string {
	if m {
		return " (with uncommitted changes)"
	}
	return ""
}

// discover znajduje fuzzery w module biblioteki (bez tools/ i testdata/).
func discover(root string) ([]fuzzer, error) {
	var out []fuzzer
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			base := d.Name()
			if rel != "." && (base == "tools" || base == "testdata" || base == "out" || strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range funcRe.FindAllSubmatch(b, -1) {
			out = append(out, fuzzer{pkg: "./" + filepath.ToSlash(filepath.Dir(rel)), name: string(m[1])})
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].pkg != out[j].pkg {
			return out[i].pkg < out[j].pkg
		}
		return out[i].name < out[j].name
	})
	return out, err
}

// gitState zwraca skrót bieżącego commitu i to, czy śledzone pliki poza dziennikiem mają niezatwierdzone zmiany.
func gitState(root string) (string, bool) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown", true
	}
	commit := strings.TrimSpace(string(out))
	st, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=no").Output()
	if err != nil {
		return commit, true
	}
	modified := false
	for _, line := range strings.Split(strings.TrimSpace(string(st)), "\n") {
		if line != "" && !strings.HasSuffix(line, "docs/fuzzing.md") {
			modified = true
		}
	}
	return commit, modified
}

func run(root, logDir string, f fuzzer, d time.Duration, parallel int) result {
	res := result{fuzzer: f, start: time.Now()}
	res.logPath = filepath.Join(logDir, res.start.Format("20060102-150405")+"-"+f.name+".log")
	logf, err := os.Create(res.logPath)
	if err != nil {
		res.detail = err.Error()
		return res
	}
	defer logf.Close()
	cmd := exec.Command("go", "test", f.pkg, "-run", "^$", "-fuzz", "^"+f.name+"$",
		"-fuzztime", d.String(), "-parallel", strconv.Itoa(parallel))
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		res.detail = err.Error()
		return res
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		res.detail = err.Error()
		return res
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var failInput string
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(logf, line)
		if m := execsRe.FindStringSubmatch(line); m != nil {
			if n, err := strconv.ParseInt(m[1], 10, 64); err == nil && n > res.execs {
				res.execs = n
			}
		}
		if m := failRe.FindStringSubmatch(line); m != nil {
			failInput = m[1]
		}
	}
	err = cmd.Wait()
	res.elapsed = time.Since(res.start)
	res.ok = err == nil
	if !res.ok {
		res.detail = "see " + res.logPath
		if failInput != "" {
			res.detail = "input " + failInput
		}
	}
	return res
}

// appendJournal dopisuje wiersze do tabeli dziennika; plik powinien kończyć się tabelą (docs/fuzzing.md).
func appendJournal(path string, rs []result) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range rs {
		status := "ok"
		if !r.ok {
			status = "**błąd**: " + r.detail
		}
		if r.host != "" {
			status += " (" + r.host + ")"
		}
		fmt.Fprintf(f, "| %s | `%s`%s | %s | `%s` | %s | %d | %s |\n", r.start.Format("2006-01-02 15:04"), r.commit,
			map[bool]string{true: " (zmiany)", false: ""}[r.modified], r.pkg, r.name,
			r.elapsed.Round(time.Second), r.execs, status)
	}
	return nil
}
