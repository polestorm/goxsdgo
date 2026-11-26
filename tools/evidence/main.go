// Command evidence składa pakiet dowodów goxsdgo (kryterium K9, ADR 0001, docs/evidence.md): wynik ostatniego
// przebiegu testów, macierz śledzenia wymagania → testy → wynik, tabelę K1–K9 ze źródłami, sumy danych testowych,
// stan zapadek, listę decyzji i różnic.
//
//	go -C tools/evidence run . -root ../..                  # pakiet w out/evidence/<data>-<commit>/
//	go -C tools/evidence run . -root ../.. -tar             # także archiwum .tar.gz
//	go -C tools/evidence run . -root ../.. -stream-mb 1024  # K6 zmierzone w tym przebiegu na pliku 1 GB
//
// Narzędzie jest osobnym modułem bez zależności: uruchamia go test, git i python3, czyli kod, który nie należy do
// biblioteki. Nie sięga do sieci, nie uruchamia fuzzingu ani libxml2 i nie czyta korpusu prywatnego (EVD-005).
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

func main() {
	root := flag.String("root", ".", "goxsdgo repository root")
	outBase := flag.String("out", "", "directory for packages (default <root>/out/evidence)")
	makeTar := flag.Bool("tar", false, "also write <package>.tar.gz")
	streamMB := flag.Int("stream-mb", 0, "run TestStreamingMemory on a document of this size (GOXSDGO_STREAM_MB), e.g. 1024 for K6")
	flag.Parse()

	abs, err := filepath.Abs(*root)
	if err != nil {
		fail(err)
	}
	if *outBase == "" {
		*outBase = filepath.Join(abs, "out", "evidence")
	}
	e, raw, err := build(abs, *streamMB, time.Now())
	if err != nil {
		fail(err)
	}
	dir := filepath.Join(*outBase, time.Now().Format("2006-01-02")+"-"+e.Commit)
	if e.Dirty {
		dir += "-dirty"
	}
	if err := write(dir, e, raw); err != nil {
		fail(err)
	}
	if *makeTar {
		if err := tarDir(dir, dir+".tar.gz"); err != nil {
			fail(err)
		}
	}
	fmt.Printf("evidence package: %s\n", dir)
	for _, c := range e.Criteria {
		fmt.Printf("  %-3s %-8s %s\n", c.ID, c.Status, c.Value)
	}
	for _, c := range e.Criteria {
		if (c.ID == "K4" || c.ID == "K9") && c.Status == "not_met" {
			os.Exit(1)
		}
	}
}

// build uruchamia przebiegi i składa pakiet. raw to surowe wyniki przebiegów (nazwa pliku → treść); zawierają czasy,
// więc nie wchodzą do części powtarzalnej.
func build(root string, streamMB int, now time.Time) (*Evidence, map[string][]byte, error) {
	reqs, idRe, err := parseRequirements(root)
	if err != nil {
		return nil, nil, err
	}
	refs, err := collectRefs(root, idRe)
	if err != nil {
		return nil, nil, err
	}
	meas, err := readMeasurements(root)
	if err != nil {
		return nil, nil, err
	}
	raw := map[string][]byte{}
	var runs []*TestRun
	for _, spec := range []struct{ name, dir, file string }{
		{"moduł główny", ".", "go-test-module.json"},
		{"tools/fetch", "tools/fetch", "go-test-fetch.json"},
		{"tools/evidence", "tools/evidence", "go-test-evidence.json"},
	} {
		env := testEnv()
		if streamMB > 0 && spec.dir == "." {
			env = append(env, "GOXSDGO_STREAM_MB="+strconv.Itoa(streamMB))
		}
		out := runCmd(filepath.Join(root, spec.dir), env, "go", "test", "-count=1", "-json", "./...")
		raw[spec.file] = out
		cmd := "go test -count=1 -json ./..."
		if spec.dir != "." {
			cmd = "go -C " + spec.dir + " test -count=1 -json ./..."
		}
		run, err := parseTestEvents(spec.name, cmd, bytes.NewReader(out))
		if err != nil {
			return nil, nil, err
		}
		runs = append(runs, run)
	}
	py := PythonRun{Command: "python3 -I -B -m unittest discover -s tools/fetch -v"}
	pyOut := runCmd(root, testEnv(), "python3", "-I", "-B", "-m", "unittest", "discover", "-s", "tools/fetch", "-v")
	raw["python-fetch.txt"] = pyOut
	if m := regexp.MustCompile(`Ran (\d+) tests?`).FindSubmatch(pyOut); m != nil {
		py.Ran, _ = strconv.Atoi(string(m[1]))
	}
	lines := strings.Split(strings.TrimSpace(string(pyOut)), "\n")
	py.Summary = strings.TrimSpace(lines[len(lines)-1])
	py.OK = py.Ran > 0 && strings.HasPrefix(py.Summary, "OK")
	if py.Ran > 0 {
		py.Summary = fmt.Sprintf("%d testów, %s", py.Ran, py.Summary)
	}

	fv := FetchVerify{Command: "go -C tools/fetch run . -root ../.. -verify"}
	verifyOut, verifyErr := runCmdErr(filepath.Join(root, "tools", "fetch"), testEnv(), "go", "run", ".", "-root", "../..", "-verify")
	raw["fetch-verify.txt"] = verifyOut
	fv.OK = verifyErr == nil
	for _, l := range strings.Split(strings.TrimSpace(string(verifyOut)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			fv.Lines = append(fv.Lines, l)
		}
	}

	traces, unknown := buildTraces(reqs, refs, runs)
	in := Inputs{Runs: runs, PythonTests: py, Traces: traces, UnknownRefs: unknown, Measurements: meas, FetchVerify: fv, StreamMB: streamMB}
	w3cRows, w3cExcl := parseW3C(in.output(pkgXSD + ".TestW3CSuite"))
	in.W3C = w3cRows

	ratchets, err := readRatchets(root)
	if err != nil {
		return nil, nil, err
	}
	diffText, _ := os.ReadFile(filepath.Join(root, "docs", "differences.md"))
	for i := range ratchets {
		if ratchets[i].File == "testdata/w3c/xsdtests.ratchet" && ratchets[i].Mode == "failing" {
			in.W3CRatchet = &ratchets[i]
			in.W3CFailing = namedInDifferences(ratchets[i].IDs, string(diffText))
		}
	}

	journal, _ := os.ReadFile(filepath.Join(root, "docs", "fuzzing.md"))
	fuzz := parseFuzzJournal(string(journal))
	candidate := meas.Candidate.Commit
	in.FuzzHours = countFuzzHours(fuzz, func(commit string) bool { return sameLibrary(root, candidate, commit) })

	libText, _ := os.ReadFile(filepath.Join(root, "docs", "differences-libxml2.md"))
	in.DiffTotal, in.DiffUncat, in.DiffStamp, in.DiffFound = differencesLibxml2(string(libText))

	sources, sourcesSHA, err := readSources(root)
	if err != nil {
		return nil, nil, err
	}
	commit := strings.TrimSpace(string(runCmd(root, nil, "git", "rev-parse", "--short", "HEAD")))
	dirty := strings.TrimSpace(string(runCmd(root, nil, "git", "status", "--porcelain", "--untracked-files=no"))) != ""
	goVersion := strings.TrimSpace(string(runCmd(root, nil, "go", "env", "GOVERSION")))
	python := strings.TrimSpace(string(runCmd(root, nil, "python3", "--version")))

	e := &Evidence{
		Generated:   now.Format("2006-01-02 15:04"),
		Commit:      commit,
		Dirty:       dirty,
		GoVersion:   goVersion,
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Python:      python,
		StreamMB:    streamMB,
		Candidate:   candidate,
		Criteria:    buildCriteria(in),
		Runs:        runs,
		PythonTests: py,
		Traces:      traces,
		UnknownRefs: unknown,
		W3C:         w3cRows,
		W3CExcluded: w3cExcl,
		W3CFailing:  in.W3CFailing,
		Ratchets:    ratchets,
		Fuzz:        fuzz,
		FuzzHours:   in.FuzzHours.Hours(),
		Sources:     sources,
		SourcesSHA:  sourcesSHA,
		FetchVerify: fv,
		ADRs:        readADRs(root),
		Differences: diffSections(string(diffText)),
	}
	for name := range raw {
		e.Files = append(e.Files, name)
	}
	e.Files = append(e.Files, "evidence.json", "report.md")
	sort.Strings(e.Files)
	return e, raw, nil
}

// testEnv to środowisko przebiegów bez zmiennych, które sięgają po korpus prywatny albo zmieniają testy (EVD-005).
func testEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "GOXSDGO_PRIVATE_CORPUS", "GOXSDGO_DUMP_ERRORS", "GOXSDGO_STREAM_MB":
			continue
		}
		env = append(env, kv)
	}
	return env
}

// sameLibrary mówi, czy commit ma ten sam kod biblioteki co kandydat: różnica poza testami, danymi testowymi
// (w każdym katalogu testdata), dokumentami (docs i pliki .md), narzędziami, CI i ustawieniami Claude jest pusta
// (docs/fuzzing.md, „Jak liczymy godziny do K5”). W pathspec bez „glob” gwiazdka obejmuje też „/”.
func sameLibrary(root, candidate, commit string) bool {
	if candidate == "" || commit == "" {
		return false
	}
	if strings.HasPrefix(candidate, commit) || strings.HasPrefix(commit, candidate) {
		return true
	}
	cmd := exec.Command("git", append([]string{"diff", "--quiet", candidate, commit, "--", "."}, libraryExcludes...)...)
	cmd.Dir = root
	return cmd.Run() == nil
}

// libraryExcludes: ścieżki, które nie są kodem biblioteki (sameLibrary, docs/evidence.md K5).
var libraryExcludes = []string{":!*_test.go", ":!*testdata/*", ":!docs", ":!*.md", ":!tools", ":!.github", ":!.claude"}

func runCmd(dir string, env []string, name string, args ...string) []byte {
	out, _ := runCmdErr(dir, env, name, args...)
	return out
}

func runCmdErr(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func write(dir string, e *Evidence, raw map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, b := range raw {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	js, err := marshal(e)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.json"), js, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(renderReport(e)), 0o644)
}

func marshal(e *Evidence) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(e)
	return buf.Bytes(), err
}

func tarDir(dir, target string) error {
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	base := filepath.Base(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		b, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: base + "/" + ent.Name(), Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := io.Copy(tw, bytes.NewReader(b)); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "evidence:", err)
	os.Exit(1)
}
