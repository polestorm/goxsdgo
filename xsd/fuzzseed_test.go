package xsd

import (
	"os"
	"testing"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

// Wejścia startowe fuzzerów z pobranych danych testowych (ADR 0011): schematy i dokumenty MF oraz próbka przypadków
// W3C. Nie trafiają do repozytorium; bez pobranych danych fuzzery startują z wejść wpisanych w kod.

// seedSchemas dodaje schematy MF i schematy z próbki zestawu W3C.
func seedSchemas(f *testing.F) {
	for _, b := range testkit.SampleFiles(schemasDir, []string{".xsd"}, 60, 256<<10) {
		f.Add(b)
	}
	for _, b := range testkit.SampleFiles(w3cRoot+"/msData", []string{".xsd"}, 120, 64<<10) {
		f.Add(b)
	}
	for _, b := range testkit.SampleFiles(w3cRoot+"/sunData", []string{".xsd"}, 40, 64<<10) {
		f.Add(b)
	}
}

// seedPairs dodaje pary (schemat, dokument) z przypadków W3C, w których schemat ma jeden dokument: tylko takie
// przypadki da się odtworzyć w fuzzerze z jednym plikiem s.xsd.
func seedPairs(f *testing.F, limit int) {
	if _, err := os.Stat(w3cRoot + "/suite.xml"); err != nil {
		return
	}
	fsys := os.DirFS(w3cRoot)
	cases, err := w3c.Load(fsys, w3c.Tokens1_0)
	if err != nil {
		return
	}
	var pairs []*w3c.Case
	for _, c := range cases {
		if c.InScope() && c.Kind == w3c.InstanceTest && len(c.SchemaDocs) == 1 {
			pairs = append(pairs, c)
		}
	}
	step := max(1, len(pairs)/limit)
	for i := 0; i < len(pairs); i += step {
		s, err1 := os.ReadFile(w3cRoot + "/" + pairs[i].SchemaDocs[0])
		d, err2 := os.ReadFile(w3cRoot + "/" + pairs[i].Instance)
		if err1 == nil && err2 == nil && len(s) < 64<<10 && len(d) < 64<<10 {
			f.Add(s, d)
		}
	}
}
