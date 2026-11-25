package xsd

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

const (
	repoRoot       = ".."
	corpusPassing  = "../testdata/passing-corpus.txt"
	expectedPath   = "../testdata/instances/expected.json"
	schemasDir     = "../testdata/schemas/mf"
	instancesDir   = "../testdata/instances"
	schemasSetName = "mf-schemas"
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
	if _, err := os.Stat(schemasDir); err != nil {
		t.Skip(testkit.FetchHint)
	}
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
	ratchet(t, "schema:", passed)
}

// TestInstanceCorpus mierzy drugą część kryterium K2: każdy dokument z testdata/instances/expected.json daje
// werdykt z tego pliku.
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
	cfg := Config{Locations: src.Locations(schemasSetName)}
	fsys := os.DirFS(schemasDir)
	passed := map[string]bool{}
	checked := 0
	for _, e := range expected {
		if e.Schema == "" {
			continue
		}
		checked++
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
		if (len(errs) == 0) == e.Valid {
			passed["instance:"+e.Path] = true
		} else {
			t.Logf("instance %s: valid=%v, want %v; first error: %v", e.Path, len(errs) == 0, e.Valid, errs[:min(1, len(errs))])
		}
	}
	t.Logf("MF instances with the expected verdict: %d/%d", len(passed), checked)
	ratchet(t, "instance:", passed)
}

// ratchet porównuje zaliczenia z listą w testdata/passing-corpus.txt w obrębie prefiksu i z -update ją zapisuje.
func ratchet(t *testing.T, prefix string, passed map[string]bool) {
	t.Helper()
	list, err := testkit.ReadList(corpusPassing)
	if err != nil {
		t.Fatal(err)
	}
	mine := map[string]bool{}
	for id := range list {
		if strings.HasPrefix(id, prefix) {
			mine[id] = true
		}
	}
	if regressed := testkit.Regressions(mine, passed); len(regressed) > 0 {
		t.Errorf("%d corpus case(s) from passing-corpus.txt no longer pass: %s", len(regressed), strings.Join(regressed, ", "))
	}
	if !*update {
		return
	}
	for id := range mine {
		delete(list, id)
	}
	for id := range passed {
		list[id] = true
	}
	header := "MF corpus: schemas that compile and documents with the expected verdict\n" +
		"(TestSchemaCorpus, TestInstanceCorpus; go test ./xsd -update)."
	if err := testkit.WriteList(corpusPassing, header, list); err != nil {
		t.Fatal(err)
	}
}
