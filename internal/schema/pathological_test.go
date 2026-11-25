package schema

import (
	"fmt"
	"testing"
	"time"
)

// TestPathological (kryterium K5) mierzy kompilację i dopasowanie modeli zawartości w przypadkach patologicznych
// dla dwóch rozmiarów n i 2n. Czas liniowy daje iloraz bliski 2; test dopuszcza 3,5, żeby szum pomiaru nie
// dawał fałszywych alarmów, i odrzuca wzrost kwadratowy (iloraz 4). REQ: CM-013
func TestPathological(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	type build func(n int) (*ContentModel, []QName)
	name := func(s string) QName { return QName{Local: s} }
	repeat := func(q QName, n int) []QName {
		out := make([]QName, n)
		for i := range out {
			out[i] = q
		}
		return out
	}
	cases := []struct {
		name string
		n    int
		b    build
	}{
		{"large maxOccurs", 100000, func(n int) (*ContentModel, []QName) {
			// maxOccurs rośnie razem z liczbą dzieci; liczba stanów automatu się nie zmienia.
			return compileModel(seq(1, 1, el("a", 0, int64(n)), el("b", 1, 1)), nil), append(repeat(name("a"), n), name("b"))
		}},
		{"nested counters", 50000, func(n int) (*ContentModel, []QName) {
			m := compileModel(seq(1, int64(n), el("x", 1, 1), el("a", 1, 100)), nil)
			var in []QName
			for len(in) < n {
				in = append(in, name("x"))
				in = append(in, repeat(name("a"), 50)...)
			}
			return m, in
		}},
		{"long child sequence", 200000, func(n int) (*ContentModel, []QName) {
			m := compileModel(cho(0, Unbounded, el("a", 1, 1), el("b", 1, 1), el("c", 1, 1)), nil)
			in := make([]QName, n)
			for i := range in {
				in[i] = name(string(rune('a' + i%3)))
			}
			return m, in
		}},
		{"long model sequence", 2000, func(n int) (*ContentModel, []QName) {
			var kids []*Particle
			var in []QName
			for i := 0; i < n; i++ {
				q := fmt.Sprintf("e%d", i)
				kids = append(kids, el(q, 1, 1))
				in = append(in, name(q))
			}
			return compileModel(seq(1, 1, kids...), nil), in
		}},
		{"many wildcards", 400, func(n int) (*ContentModel, []QName) {
			var kids []*Particle
			var in []QName
			for i := 0; i < n; i++ {
				ns := fmt.Sprintf("urn:%d", i)
				kids = append(kids, wild(1, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{ns}}))
				in = append(in, QName{Space: ns, Local: "x"})
			}
			return compileModel(cho(0, Unbounded, kids...), nil), append(in, in...)
		}},
		{"large substitution group", 2000, func(n int) (*ContentModel, []QName) {
			head := &ElementDecl{Name: name("h"), Scope: Scope{Global: true}}
			decls := []*ElementDecl{head}
			var in []QName
			for i := 0; i < n; i++ {
				m := &ElementDecl{Name: name(fmt.Sprintf("m%d", i)), Scope: Scope{Global: true}, SubstitutionGroup: head}
				decls = append(decls, m)
				in = append(in, m.Name)
			}
			subst := substitutionGroups(decls)
			return compileModel(&Particle{MinOccurs: 0, MaxOccurs: Unbounded, Term: head}, subst), append(in, in...)
		}},
	}
	measure := func(b build, n int) time.Duration {
		best := time.Duration(1<<63 - 1)
		for r := 0; r < 3; r++ {
			start := time.Now()
			m, in := b(n)
			if c := m.checkUPA(); c != nil {
				t.Fatalf("UPA: %v", c)
			}
			mt := m.NewMatcher()
			for _, q := range in {
				if _, ok := mt.Step(q); !ok {
					t.Fatalf("child %s rejected", q)
				}
			}
			if !mt.Accepting() {
				t.Fatal("not accepting")
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	for _, c := range cases {
		t1 := measure(c.b, c.n)
		t2 := measure(c.b, 2*c.n)
		ratio := float64(t2) / float64(max(t1, time.Microsecond))
		t.Logf("%-26s n=%-7d %10v  2n %10v  ratio %.2f", c.name, c.n, t1, t2, ratio)
		if ratio > 3.5 && t2 > 20*time.Millisecond {
			t.Errorf("%s: time grows faster than linearly (ratio %.2f)", c.name, ratio)
		}
	}
}
