package xsd

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"sort"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

const (
	repoRoot         = ".."
	schemasRatchet   = "../testdata/mf-schemas.ratchet"
	instancesRatchet = "../testdata/mf-instances.ratchet"
	expectedPath     = "../testdata/instances/expected.json"
	schemasDir       = "../testdata/schemas/mf"
	instancesDir     = "../testdata/instances"
	schemasSetName   = "mf-schemas"
)

// expectedInstance to wpis w testdata/instances/expected.json: oczekiwany werdykt dokumentu korpusu.
type expectedInstance struct {
	// Path to ścieżka dokumentu względem testdata/instances.
	Path string `json:"path"`
	// Schema to ścieżka schematu względem testdata/schemas/mf; pusta, gdy korpus nie ma schematu dokumentu.
	Schema string `json:"schema"`
	Valid  bool   `json:"valid"`
	// Errors to oczekiwane błędy (kod, wiersz, ścieżka) dla dokumentów błędnych; uzupełnia etap 6.
	Errors []expectedError `json:"errors,omitempty"`
	Note   string          `json:"note,omitempty"`
}

type expectedError struct {
	Code string `json:"code"`
	Line int    `json:"line"`
	Path string `json:"path,omitempty"`
}

// TestSchemaCorpus mierzy pierwszą część kryterium K2: każdy schemat MF z manifestu kompiluje się bez błędów,
// a importy rozwiązuje katalog adresów z manifestu, bez sieci.
func TestSchemaCorpus(t *testing.T) {
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	skipIfSchemasMissing(t, src)
	cfg := Config{Locations: src.Locations(schemasSetName)}
	fsys := os.DirFS(schemasDir)
	passed := map[string]bool{}
	var paths []string
	for _, ds := range src.Sets {
		if ds.Name != schemasSetName {
			continue
		}
		for _, f := range ds.Files {
			if _, err := os.Stat(path.Join(schemasDir, f.Path)); err != nil && f.LocalOnly {
				t.Logf("schema %s: local only and not fetched, skipped", f.Path)
				continue
			}
			paths = append(paths, f.Path)
		}
	}
	sort.Strings(paths)
	unsupported := 0
	for _, p := range paths {
		_, err := cfg.Compile(fsys, p)
		switch {
		case err == nil:
			passed["schema:"+p] = true
		case errors.Is(err, errors.ErrUnsupported):
			unsupported++
		default:
			t.Logf("schema %s: %v", p, err)
		}
	}
	t.Logf("MF schemas compiled: %d/%d (not yet supported: %d)", len(passed), len(paths), unsupported)
	inScope := map[string]bool{}
	for _, p := range paths {
		inScope["schema:"+p] = true
	}
	testkit.Ratchet(t, schemasRatchet, "MF schemas that compile (TestSchemaCorpus, go test ./xsd -update).", inScope, passed, *update)
}

// TestInstanceCorpus mierzy drugą część kryterium K2: każdy dokument z testdata/instances/expected.json daje
// werdykt z tego pliku.
// REQ: VAL-021
func TestInstanceCorpus(t *testing.T) {
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected []expectedInstance
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(instancesDir + "/mf"); err != nil {
		t.Skip(testkit.FetchHint)
	}
	skipIfSchemasMissing(t, src)
	cfg := Config{Locations: src.Locations(schemasSetName)}
	fsys := os.DirFS(schemasDir)
	passed := map[string]bool{}
	inScope := map[string]bool{}
	checked := 0
	obs := newCodeObserver()
	for _, e := range expected {
		if e.Schema == "" {
			continue
		}
		checked++
		inScope["instance:"+e.Path] = true
		set, err := cfg.Compile(fsys, e.Schema)
		if err != nil {
			if !errors.Is(err, errors.ErrUnsupported) {
				t.Errorf("%s: schema %s: %v", e.Path, e.Schema, err)
			}
			continue
		}
		f, err := os.Open(path.Join(instancesDir, e.Path))
		if err != nil {
			t.Fatal(err)
		}
		errs, err := set.Validate(f)
		f.Close()
		if err != nil {
			continue
		}
		obs.add(KindDocument, errs)
		if (len(errs) == 0) != e.Valid {
			t.Logf("instance %s: valid=%v, want %v; first error: %v", e.Path, len(errs) == 0, e.Valid, errs[:min(1, len(errs))])
			continue
		}
		// Oczekiwane błędy (kod, wiersz, ścieżka) to początek listy błędów walidacji, w tej samej kolejności.
		ok := len(errs) >= len(e.Errors)
		for i := 0; ok && i < len(e.Errors); i++ {
			w, g := e.Errors[i], errs[i]
			if g.Code != w.Code || g.Line != w.Line || (w.Path != "" && g.Path != w.Path) {
				t.Errorf("instance %s: error %d is %s at line %d, %s; want %s at line %d, %s", e.Path, i+1, g.Code, g.Line,
					g.Path, w.Code, w.Line, w.Path)
				ok = false
			}
		}
		if ok {
			passed["instance:"+e.Path] = true
		}
	}
	checkObservedCodes(t, obs)
	t.Logf("MF and synthetic documents with the expected verdict and errors: %d/%d", len(passed), checked)
	testkit.Ratchet(t, instancesRatchet, "MF documents with the expected verdict (TestInstanceCorpus, go test ./xsd -update).",
		inScope, passed, *update)
}

// skipIfSchemasMissing pomija test, gdy brakuje któregoś schematu MF z manifestu (poza plikami local_only): bez
// kompletu importów wynik byłby fałszywy. Dotyczy CI, z którego epuap.gov.pl nie odpowiada; lokalnie komplet
// sprawdza `go -C tools/fetch run . -root ../.. -verify`.
func skipIfSchemasMissing(t *testing.T, src *testkit.Sources) {
	t.Helper()
	missing := 0
	for _, ds := range src.Sets {
		if ds.Name != schemasSetName {
			continue
		}
		for _, f := range ds.Files {
			if _, err := os.Stat(path.Join(schemasDir, f.Path)); err != nil && !f.LocalOnly {
				missing++
			}
		}
	}
	if missing > 0 {
		t.Skipf("%d MF schema file(s) missing; %s", missing, testkit.FetchHint)
	}
}
