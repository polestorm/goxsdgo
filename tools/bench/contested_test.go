package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/polestorm/goxsdgo/internal/w3c"
)

// TestContestedRules: każda reguła wskazująca grupy z nazwy trafia w istniejące przypadki w zakresie, a nakładanie
// kategorii jest jawne (liczone i raportowane).
func TestContestedRules(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "w3c", "xsdtests")
	if _, err := os.Stat(filepath.Join(dir, "suite.xml")); err != nil {
		t.Skip("W3C suite not fetched")
	}
	cases, err := w3c.Load(os.DirFS(dir), w3c.Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	rules := contestRules()
	for _, r := range rules {
		for _, g := range r.groups {
			found := false
			for _, c := range cases {
				if c.InScope() && c.Contributor == r.contributor && c.Group == g &&
					(!r.instances || c.Kind == w3c.InstanceTest) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("rule %s: no in-scope case in group %s (%s)", r.Category, g, r.contributor)
			}
		}
	}
	cl := classify(cases)
	overlap := 0
	for _, c := range cases {
		if !c.InScope() {
			if _, ok := cl[c.ID]; ok {
				t.Errorf("%s is out of scope but classified", c.ID)
			}
			continue
		}
		n := 0
		for _, r := range rules {
			if r.matches(c) {
				n++
			}
		}
		if n > 1 {
			overlap++
		}
	}
	t.Logf("contested: %d, matched by more than one rule (first wins): %d", len(cl), overlap)
	// Przypadki ErrLimit nie są sporne (ograniczenie goxsdgo, nie spór).
	for id := range cl {
		for _, g := range []string{"particlesZ035_a", "particlesZ036_b1", "particlesZ036_b2", "particlesZ036_c"} {
			if c := find(cases, id); c != nil && c.Group == g {
				t.Errorf("%s must not be contested", id)
			}
		}
	}
}

func find(cases []*w3c.Case, id string) *w3c.Case {
	for _, c := range cases {
		if c.ID == id {
			return c
		}
	}
	return nil
}
