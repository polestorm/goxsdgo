package xsd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// prefixRow to wiersz tabeli prefiksów w requirements/README.md, np. "| `DT` | `datatypes.md` | ...".
var prefixRow = regexp.MustCompile("^\\| `([A-Z]{2,5})` \\|")

// TestRequirementsCovered mierzy kryterium K4: każde wymaganie z requirements/ ma test albo plik korpusu,
// a każde odwołanie „REQ:” wskazuje istniejące wymaganie. Prefiksy identyfikatorów bierze z tabeli
// w requirements/README.md (przeniesione z gomdgo). Wymaganie delegowane do goxmlgo (ADR 0014) nie ma testu
// w goxsdgo: test sprawdza zamiast tego, że odsyłacz wskazuje wersję goxmlgo z go.mod i wymaganie, które w tej
// wersji istnieje.
func TestRequirementsCovered(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	var prefixes []string
	for _, line := range strings.Split(read("../requirements/README.md"), "\n") {
		if m := prefixRow.FindStringSubmatch(line); m != nil {
			prefixes = append(prefixes, m[1])
		}
	}
	if len(prefixes) == 0 {
		t.Fatal("no prefixes in requirements/README.md")
	}
	reqRef := regexp.MustCompile(`\b(?:` + strings.Join(prefixes, "|") + `)-\d{3}\b`)

	defined := map[string]bool{}
	delegated := map[string]string{} // id → odsyłacz
	docs, _ := filepath.Glob("../requirements/*.md")
	for _, doc := range docs {
		if filepath.Base(doc) == "README.md" {
			continue
		}
		for _, line := range strings.Split(read(doc), "\n") {
			if id := reqRef.FindString(line); id != "" && strings.HasPrefix(line, "| "+id) {
				if defined[id] {
					t.Errorf("requirement %s is defined twice", id)
				}
				defined[id] = true
				if m := delegatedRow.FindStringSubmatch(line); m != nil {
					delegated[id] = m[0]
					checkDelegated(t, id, m[1], m[2], strings.Fields(m[3]))
				} else if strings.Contains(line, "Delegowane") {
					t.Errorf("%s: malformed delegation %q", id, line)
				}
			}
		}
	}

	var sources []string
	for _, pattern := range []string{"*_test.go", "../internal/*/*_test.go", "../xmlreader/*_test.go", "resolve/*_test.go", "../tools/evidence/*_test.go", "../testdata/corpus/*"} {
		m, _ := filepath.Glob(pattern)
		sources = append(sources, m...)
	}
	used := map[string]bool{}
	for _, f := range sources {
		for _, line := range strings.Split(read(f), "\n") {
			if !strings.Contains(line, "REQ:") {
				continue
			}
			for _, id := range reqRef.FindAllString(line, -1) {
				used[id] = true
				if !defined[id] {
					t.Errorf("%s refers to unknown requirement %s", f, id)
				}
			}
		}
	}
	for id := range defined {
		if _, ok := delegated[id]; ok {
			if used[id] {
				t.Errorf("requirement %s is delegated to goxmlgo, but a goxsdgo test refers to it", id)
			}
			continue
		}
		if !used[id] {
			t.Errorf("requirement %s has no corpus file or test", id)
		}
	}
	t.Logf("%d requirement(s): %d referenced, %d delegated to goxmlgo", len(defined), len(used), len(delegated))
}

// delegatedRow to treść wymagania delegowanego: „Delegowane: goxmlgo v0.2.0, `requirements/xml.md` XML-001.”
var delegatedRow = regexp.MustCompile("\\| Delegowane: goxmlgo (v[0-9][^ ,]*), `requirements/([a-z]+\\.md)` ((?:[A-Z]+-\\d{3} ?)+)")

// checkDelegated sprawdza odsyłacz wymagania id: wersja jak w go.mod, a każde wskazane wymaganie jest zdefiniowane
// w pliku requirements/file tej wersji goxmlgo.
func checkDelegated(t *testing.T, id, version, file string, targets []string) {
	t.Helper()
	dir, want := goxmlgo(t)
	if version != want {
		t.Errorf("%s is delegated to goxmlgo %s, go.mod requires %s", id, version, want)
	}
	b, err := os.ReadFile(filepath.Join(dir, "requirements", file))
	if err != nil {
		t.Errorf("%s: %v", id, err)
		return
	}
	for _, target := range targets {
		if !strings.Contains(string(b), "\n| "+target+" |") {
			t.Errorf("%s is delegated to %s in goxmlgo %s requirements/%s, which does not define it", id, target, version, file)
		}
	}
}
