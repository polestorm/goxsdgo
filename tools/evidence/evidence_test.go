package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRepo buduje w katalogu tymczasowym minimalne repozytorium: wymagania, moduł z testami z odwołaniami REQ:,
// zapadkę, dziennik fuzzingu i pomiary.
func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"requirements/README.md": "| Prefiks | Plik | Zakres |\n|---|---|---|\n| `AB` | `a.md` | a |\n| `CDE` | `c.md` | c |\n",
		"requirements/a.md":      "| Id | Wymaganie |\n|---|---|\n| AB-001 | pierwsze |\n| AB-002 | drugie, z \\| w treści |\n| AB-003 | tylko pomocnicza |\n| AB-004 | benchmark |\n| AB-005 | na żądanie |\n",
		"requirements/c.md":      "| Id | Wymaganie |\n|---|---|\n| CDE-001 | bez testu |\n| CDE-002 | Delegowane: goxmlgo v0.2.0, `requirements/xml.md` XML-001. |\n",
		"go.mod":                 "module example.com/m\n\ngo 1.22\n",
		"p/p_test.go": `package p

import "testing"

// TestOne sprawdza pierwsze. REQ: AB-001
func TestOne(t *testing.T) {}

func TestTwo(t *testing.T) {
	// REQ: AB-002, XY-001
}

// REQ: AB-002, AB-003
func helper() {}

// REQ: AB-004
func BenchmarkFour(b *testing.B) {}

// REQ: AB-005
func TestFive(t *testing.T) {}
`,
		"testdata/w3c/xsdtests.ratchet": "# header\n# in scope 10, passing 8\n# mode: failing\nS/xv003/xv003.v01.xml\nS/grpQ_12/grpQ_12\n",
		"docs/differences.md":           "## A\n\n| x | y |\n|---|---|\n| xv002–xv100 | z |\n\n## B\n\nnic\n",
		"docs/fuzzing.md": "| Data | Commit | Pakiet | Fuzzer | Czas | Wykonania | Wynik |\n|---|---|---|---|---|---|---|\n" +
			"| 2026-11-25 15:59 | `abc1234` | ./x | `FuzzA` | 15m3s | 10 | ok; przerwany |\n" +
			"| 2026-11-25 16:00 | `abc1234` (zmiany) | ./x | `FuzzA` | 2h0m0s | 10 | ok |\n" +
			"| 2026-11-26 10:00 | `def5678` | ./x | `FuzzB` | 3h0m0s | 10 | FAIL rc=1 |\n" +
			"| 2026-11-26 11:00 | `def5678` | ./x | `FuzzB` | — | — | nieuruchomiony |\n",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const fakeEvents = `{"Action":"output","Package":"example.com/m/p","Test":"TestOne","Output":"=== RUN TestOne\n"}
{"Action":"pass","Package":"example.com/m/p","Test":"TestOne"}
{"Action":"pass","Package":"example.com/m/p","Test":"TestOne/sub"}
{"Action":"skip","Package":"example.com/m/p","Test":"TestTwo"}
{"Action":"output","Package":"example.com/m/p","Test":"TestFive","Output":"    x_test.go:1: set GOXSDGO_DUMP_ERRORS=file to dump all errors\n"}
{"Action":"skip","Package":"example.com/m/p","Test":"TestFive"}
{"Action":"pass","Package":"example.com/m/p"}
{"Action":"fail","Package":"example.com/m/q"}
`

// REQ: EVD-002
func TestTraceabilityMatrix(t *testing.T) {
	root := fakeRepo(t)
	reqs, idRe, err := parseRequirements(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 7 || reqs[1].Text != `drugie, z \| w treści` {
		t.Fatalf("requirements: %+v", reqs)
	}
	refs, err := collectRefs(root, idRe)
	if err != nil {
		t.Fatal(err)
	}
	run, err := parseTestEvents("m", "go test", strings.NewReader(fakeEvents))
	if err != nil {
		t.Fatal(err)
	}
	if run.Passed != 1 || run.Skipped != 2 || len(run.BuildFailed) != 1 || run.BuildFailed[0] != "example.com/m/q" {
		t.Fatalf("run: %+v", run)
	}
	traces, unknown := buildTraces(reqs, refs, []*TestRun{run})
	want := map[string]string{"AB-001": "pass", "AB-002": "skip", "AB-003": "unresolved", "AB-004": "manual",
		"AB-005": "manual", "CDE-001": "missing", "CDE-002": "delegated"}
	for _, tr := range traces {
		if tr.Status != want[tr.ID] {
			t.Errorf("%s: status %s, want %s (evidence %v)", tr.ID, tr.Status, want[tr.ID], tr.Evidence)
		}
	}
	// AB-002 ma dwa dowody: TestTwo (pominięty bez zmiennej GOXSDGO_*, czyli z braku danych) i funkcję pomocniczą
	// (dowód wspierający, nie pogarsza stanu).
	if len(traces[1].Evidence) != 2 {
		t.Errorf("AB-002 evidence: %v", traces[1].Evidence)
	}
	// XY-001 nie jest prefiksem z README, więc nie jest w ogóle odwołaniem; nieznanych wymagań brak.
	if len(unknown) != 0 {
		t.Errorf("unknown refs: %v", unknown)
	}
}

// REQ: EVD-003
func TestFuzzHoursCountOnlyCleanRunsOfTheCandidate(t *testing.T) {
	root := fakeRepo(t)
	b, _ := os.ReadFile(filepath.Join(root, "docs", "fuzzing.md"))
	runs := parseFuzzJournal(string(b))
	if len(runs) != 4 {
		t.Fatalf("%d runs: %+v", len(runs), runs)
	}
	total := countFuzzHours(runs, func(c string) bool { return c == "abc1234" || c == "def5678" })
	if total != 15*time.Minute+3*time.Second {
		t.Errorf("counted %v; rows %+v", total, runs)
	}
	if !runs[1].Dirty || runs[1].Counted || runs[2].Counted || runs[3].Counted {
		t.Errorf("flags: %+v", runs)
	}
}

// REQ: EVD-003
func TestCriteriaFromInputs(t *testing.T) {
	m := &Measurements{}
	m.Candidate.Commit = "abc1234"
	m.K3.InstanceVerdictSamePercent, m.K3.FirstErrorLineSamePercent = 99.69, 99.7
	m.K3.VerdictThresholdPercent, m.K3.LineThresholdPercent = 99.5, 95
	m.K6.SizeMB, m.K6.PeakMB, m.K6.LimitMB = 1024, 6.3, 64
	m.K7.SyntheticRatio, m.K7.PrivateCorpusRatio, m.K7.PrivateCorpusLargestRatio, m.K7.LimitRatio = 1.05, 1.19, 1.33, 3
	m.K8.Status = "pending"
	events := strings.Join([]string{
		`{"Action":"output","Package":"` + pkgXSD + `","Test":"TestW3CSuite","Output":"        all        schema 9/10  instance 0/0  total 9/10\n"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestW3CSuite"}`,
		`{"Action":"output","Package":"` + pkgXSD + `","Test":"TestSchemaCorpus","Output":"    corpus_test.go:1: MF schemas compiled: 170/170 (not yet supported: 0)\n"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestSchemaCorpus"}`,
		`{"Action":"output","Package":"` + pkgXSD + `","Test":"TestInstanceCorpus","Output":"    corpus_test.go:1: MF and synthetic documents with the expected verdict and errors: 15/15\n"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestInstanceCorpus"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestPathological"}`,
		`{"Action":"pass","Package":"` + pkgSchema + `","Test":"TestPathological"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestSafeByDefault"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestStreamingMemory"}`,
		`{"Action":"pass","Package":"` + pkgXSD + `","Test":"TestCompatTable"}`,
	}, "\n")
	run, _ := parseTestEvents("m", "go test", strings.NewReader(events))
	rows, _ := parseW3C(run.Output(pkgXSD + ".TestW3CSuite"))
	in := Inputs{Runs: []*TestRun{run}, Measurements: m, W3C: rows, DiffFound: true, DiffTotal: 3,
		PythonTests: PythonRun{OK: true}, FetchVerify: FetchVerify{OK: true}, FuzzHours: 15 * time.Minute,
		Traces:     []Trace{{Requirement: Requirement{ID: "AB-001"}, Status: "pass"}},
		W3CFailing: []NamedCase{{ID: "S/a/a", Named: true}}}
	got := map[string]string{}
	for _, c := range buildCriteria(in) {
		got[c.ID] = c.Status
	}
	want := map[string]string{"K1": "met", "K2": "met", "K3": "met", "K4": "met", "K5": "pending", "K6": "met", "K7": "met", "K8": "pending", "K9": "met"}
	for id, s := range want {
		if got[id] != s {
			t.Errorf("%s: %s, want %s", id, got[id], s)
		}
	}
	// Niezaliczony przypadek W3C bez nazwy w differences.md: K1 czeka; 24 h fuzzingu: K5 spełnione.
	in.W3CFailing[0].Named = false
	in.FuzzHours = 25 * time.Hour
	for _, c := range buildCriteria(in) {
		if c.ID == "K1" && c.Status != "pending" || c.ID == "K5" && c.Status != "met" {
			t.Errorf("%s: %s", c.ID, c.Status)
		}
	}
}

// REQ: EVD-001, EVD-003
func TestRatchetsAndDifferences(t *testing.T) {
	root := fakeRepo(t)
	rs, err := readRatchets(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].InScope != 10 || rs[0].Passing != 8 || rs[0].Mode != "failing" || rs[0].Listed != 2 {
		t.Fatalf("ratchets: %+v", rs)
	}
	diff, _ := os.ReadFile(filepath.Join(root, "docs", "differences.md"))
	named := namedInDifferences(rs[0].IDs, string(diff))
	if !named[0].Named || named[1].Named {
		t.Errorf("named: %+v", named)
	}
	sections := diffSections(string(diff))
	if len(sections) != 2 || sections[0].Rows != 1 || sections[1].Rows != 0 {
		t.Errorf("sections: %+v", sections)
	}
	total, uncat, _, ok := differencesLibxml2("Rozbieżności: 528 (testy schematów 452, testy dokumentów 76); bez kategorii: 0. x")
	if !ok || total != 528 || uncat != 0 {
		t.Errorf("libxml2 differences: %d %d %v", total, uncat, ok)
	}
}

// REQ: EVD-004
func TestReportIsDeterministic(t *testing.T) {
	e := &Evidence{Commit: "abc1234", Candidate: "abc1234",
		Criteria: []Criterion{{ID: "K1", Name: "x", Value: "a | b", Status: "met"}},
		Traces:   []Trace{{Requirement: Requirement{ID: "AB-001", File: "requirements/a.md", Text: "t"}, Evidence: []string{"p.TestOne"}, Status: "pass"}},
		Files:    []string{"evidence.json", "report.md"}}
	render := func(when string) (string, string) {
		c := *e
		c.Generated = when
		js, err := marshal(&c)
		if err != nil {
			t.Fatal(err)
		}
		return renderReport(&c), string(js)
	}
	r1, j1 := render("2026-11-25 10:00")
	r2, j2 := render("2026-11-26 11:00")
	if strings.Replace(r1, "2026-11-25 10:00", "2026-11-26 11:00", 1) != r2 {
		t.Error("report differs beyond the generation time")
	}
	var m1, m2 map[string]any
	json.Unmarshal([]byte(j1), &m1)
	json.Unmarshal([]byte(j2), &m2)
	delete(m1, "generated")
	delete(m2, "generated")
	a, _ := json.Marshal(m1)
	b, _ := json.Marshal(m2)
	if string(a) != string(b) {
		t.Error("evidence.json differs beyond the generation time")
	}
	if !strings.Contains(r1, `a \| b`) {
		t.Error("table cell not escaped")
	}
}

// REQ: EVD-005
func TestEnvironmentWithoutPrivateCorpus(t *testing.T) {
	t.Setenv("GOXSDGO_PRIVATE_CORPUS", "/secret")
	t.Setenv("GOXSDGO_DUMP_ERRORS", "1")
	t.Setenv("GOXSDGO_STREAM_MB", "1024")
	for _, kv := range testEnv() {
		for _, name := range []string{"GOXSDGO_PRIVATE_CORPUS=", "GOXSDGO_DUMP_ERRORS=", "GOXSDGO_STREAM_MB="} {
			if strings.HasPrefix(kv, name) {
				t.Errorf("test environment keeps %s", kv)
			}
		}
	}
}
