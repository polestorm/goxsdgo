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
// w requirements/README.md (przeniesione z gomdgo).
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
			}
		}
	}

	var sources []string
	for _, pattern := range []string{"*_test.go", "../internal/*/*_test.go", "../messages/*_test.go", "../testdata/corpus/*"} {
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
		if !used[id] {
			t.Errorf("requirement %s has no corpus file or test", id)
		}
	}
	t.Logf("%d requirement(s), %d referenced", len(defined), len(used))
}
