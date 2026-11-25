package schema

import (
	"strings"
	"testing"
)

// Budowanie cząstek w testach: el("a", 0, 1), seq(1, 1, ...), cho(...), wild(...).

func el(name string, mn, mx int64) *Particle {
	return &Particle{MinOccurs: mn, MaxOccurs: mx, Term: &ElementDecl{Name: QName{Local: name}}}
}

func grp(c Compositor, mn, mx int64, kids ...*Particle) *Particle {
	return &Particle{MinOccurs: mn, MaxOccurs: mx, Term: &ModelGroup{Compositor: c, Particles: kids}}
}

func seq(mn, mx int64, kids ...*Particle) *Particle { return grp(CompositorSequence, mn, mx, kids...) }
func cho(mn, mx int64, kids ...*Particle) *Particle { return grp(CompositorChoice, mn, mx, kids...) }
func all(mn int64, kids ...*Particle) *Particle     { return grp(CompositorAll, mn, 1, kids...) }

func wild(mn, mx int64, nc NamespaceConstraint) *Particle {
	return &Particle{MinOccurs: mn, MaxOccurs: mx, Term: &Wildcard{Namespaces: nc}}
}

// accepts dopasowuje ciąg nazw (lokalnych, bez przestrzeni; "ns:x" to element x w przestrzeni ns).
func accepts(m *ContentModel, input string) bool {
	mt := m.NewMatcher()
	for _, w := range strings.Fields(input) {
		q := QName{Local: w}
		if i := strings.IndexByte(w, ':'); i >= 0 {
			q = QName{Space: w[:i], Local: w[i+1:]}
		}
		if _, ok := mt.Step(q); !ok {
			return false
		}
	}
	return mt.Accepting()
}

// REQ: CM-001, CM-002
func TestModelMatching(t *testing.T) {
	cases := []struct {
		name  string
		p     *Particle
		yes   []string
		no    []string
		upaOK bool
	}{
		{"sequence", seq(1, 1, el("a", 1, 1), el("b", 0, 1), el("c", 1, 1)), []string{"a c", "a b c"}, []string{"", "a", "a b", "b c", "a c c"}, true},
		{"choice", cho(1, 1, el("a", 1, 1), el("b", 1, 1)), []string{"a", "b"}, []string{"", "a b"}, true},
		{"counted leaf", seq(1, 1, el("a", 2, 4)), []string{"a a", "a a a a"}, []string{"a", "a a a a a"}, true},
		{"counted group", seq(2, 3, el("a", 1, 1), el("b", 1, 1)), []string{"a b a b", "a b a b a b"}, []string{"a b", "a b a b a b a b", "a b a"}, true},
		{"sequence a{2,2} a", seq(1, 1, el("a", 2, 2), el("b", 0, 1), el("a", 1, 1)), []string{"a a a", "a a b a"}, []string{"a a", "a a a a"}, true},
		{"nested counters", seq(1, 3, el("x", 1, 1), el("a", 2, 3)), []string{"x a a", "x a a a x a a"}, []string{"x a", "x a a a a"}, true},
		{"unbounded with min", seq(1, 1, el("a", 3, Unbounded)), []string{"a a a", "a a a a a a a"}, []string{"a a"}, true},
		{"optional group", seq(0, 1, el("a", 1, 1)), []string{"", "a"}, []string{"a a"}, true},
		{"empty choice", cho(1, 1), nil, []string{"", "a"}, true},
		{"empty sequence", seq(1, 1), []string{""}, []string{"a"}, true},
		{"choice of sequences", cho(1, Unbounded, seq(1, 1, el("a", 1, 1), el("b", 1, 1)), el("c", 1, 1)), []string{"a b", "c a b c"}, []string{"a", "b"}, true},
		{"large count", seq(1, 1, el("a", 0, 100000), el("b", 1, 1)), []string{"b", "a a a b"}, []string{"a"}, true},
		{"emptiable body", seq(2, 2, el("a", 0, 1)), []string{"", "a", "a a"}, []string{"a a a"}, true},
		{"wildcard", seq(1, 1, el("a", 1, 1), wild(0, Unbounded, NamespaceConstraint{Kind: NamespaceNot, Not: ""})),
			[]string{"a", "a u:x u:y"}, []string{"a x", "u:x"}, true},
	}
	for _, c := range cases {
		m := compileModel(c.p, nil)
		if conflict := m.checkUPA(); (conflict == nil) != c.upaOK {
			t.Errorf("%s: UPA %v", c.name, conflict)
		}
		for _, in := range c.yes {
			if !accepts(m, in) {
				t.Errorf("%s: %q rejected", c.name, in)
			}
		}
		for _, in := range c.no {
			if accepts(m, in) {
				t.Errorf("%s: %q accepted", c.name, in)
			}
		}
	}
}

// REQ: CM-003
func TestModelAll(t *testing.T) {
	m := compileModel(all(1, el("a", 1, 1), el("b", 0, 1), el("c", 1, 1)), nil)
	for _, in := range []string{"a c", "c a", "b c a", "a b c"} {
		if !accepts(m, in) {
			t.Errorf("all: %q rejected", in)
		}
	}
	for _, in := range []string{"", "a", "a a c", "a c d"} {
		if accepts(m, in) {
			t.Errorf("all: %q accepted", in)
		}
	}
	if opt := compileModel(all(0, el("a", 1, 1)), nil); !accepts(opt, "") || !accepts(opt, "a") {
		t.Error("all with minOccurs 0 must accept nothing and the element")
	}
}

// REQ: CM-004, CM-006
func TestModelMatchResultAndExpected(t *testing.T) {
	p := seq(1, 1, el("a", 1, 1), cho(1, 1, el("b", 1, 1), wild(1, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{"urn:x"}})))
	m := compileModel(p, nil)
	mt := m.NewMatcher()
	got, ok := mt.Step(QName{Local: "a"})
	if !ok || got.Element == nil || got.Element.Name.Local != "a" || got.Particle == nil {
		t.Fatalf("a: %+v %v", got, ok)
	}
	exp := mt.Expected()
	if len(exp) != 2 || exp[0].Element == nil || exp[0].Element.Name.Local != "b" || exp[1].Wildcard == nil {
		t.Fatalf("expected after a: %+v", exp)
	}
	if _, ok := mt.Step(QName{Local: "c"}); ok {
		t.Fatal("c accepted")
	}
	got, ok = mt.Step(QName{Space: "urn:x", Local: "c"})
	if !ok || got.Wildcard == nil {
		t.Fatalf("urn:x c: %+v %v", got, ok)
	}
	if !mt.Accepting() {
		t.Fatal("not accepting after a, urn:x c")
	}
}

// REQ: CM-005
func TestModelSubstitutionGroups(t *testing.T) {
	head := &ElementDecl{Name: QName{Local: "h"}, Scope: Scope{Global: true}, Type: anyTypeForTest()}
	m1 := &ElementDecl{Name: QName{Local: "m1"}, Scope: Scope{Global: true}, SubstitutionGroup: head, Type: head.Type}
	m2 := &ElementDecl{Name: QName{Local: "m2"}, Scope: Scope{Global: true}, SubstitutionGroup: m1, Type: head.Type}
	abs := &ElementDecl{Name: QName{Local: "abs"}, Scope: Scope{Global: true}, SubstitutionGroup: head, Abstract: true, Type: head.Type}
	subst := substitutionGroups([]*ElementDecl{head, m1, m2, abs})
	names := func(list []*ElementDecl) string {
		var s []string
		for _, e := range list {
			s = append(s, e.Name.Local)
		}
		return strings.Join(s, " ")
	}
	if got := names(subst[head]); got != "h m1 m2" {
		t.Fatalf("substitution group of h: %q", got)
	}
	m := compileModel(&Particle{MinOccurs: 1, MaxOccurs: Unbounded, Term: head}, subst)
	mt := m.NewMatcher()
	got, ok := mt.Step(QName{Local: "m2"})
	if !ok || got.Element != m2 {
		t.Fatalf("m2: %+v %v", got, ok)
	}
	if _, ok := mt.Step(QName{Local: "abs"}); ok {
		t.Fatal("abstract member accepted")
	}
	head.Block = DerivationSubstitution
	if got := names(substitutionGroups([]*ElementDecl{head, m1, m2, abs})[head]); got != "h" {
		t.Fatalf("blocked substitution group of h: %q", got)
	}
}

func anyTypeForTest() Type {
	initBuiltins()
	return anyType
}

// REQ: CM-007
func TestUPAWithCounters(t *testing.T) {
	ns := NamespaceConstraint{Kind: NamespaceAny}
	cases := []struct {
		name string
		p    *Particle
		ok   bool
	}{
		{"a? a", seq(1, 1, el("a", 0, 1), el("a", 1, 1)), false},
		{"a{2,2} a", seq(1, 1, el("a", 2, 2), el("a", 1, 1)), true},
		{"a{1,2} a?", seq(1, 1, el("a", 1, 2), el("a", 0, 1)), false},
		{"choice a a", cho(1, 1, el("a", 1, 1), el("a", 1, 1)), false},
		{"a* a", seq(1, 1, el("a", 0, Unbounded), el("a", 1, 1)), false},
		{"(a b)* a", seq(1, 1, seq(0, Unbounded, el("a", 1, 1), el("b", 1, 1)), el("a", 1, 1)), false},
		{"wildcard vs element", seq(1, 1, wild(0, 1, ns), el("a", 1, 1)), false},
		{"element then wildcard", seq(1, 1, el("a", 1, 1), wild(0, 1, ns)), true},
		{"two wildcards", cho(1, 1, wild(1, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{"u"}}), wild(1, 1, NamespaceConstraint{Kind: NamespaceNot, Not: "v"})), false},
		{"disjoint wildcards", cho(1, 1, wild(1, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{"u"}}), wild(1, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{"v"}})), true},
		{"(a{2,2}){2,2} a", seq(1, 1, seq(2, 2, el("a", 2, 2)), el("a", 1, 1)), true},
		{"all overlap", all(1, el("a", 1, 1), el("a", 0, 1)), false},
	}
	for _, c := range cases {
		m := compileModel(c.p, nil)
		if conflict := m.checkUPA(); (conflict == nil) != c.ok {
			t.Errorf("%s: want ok=%v, got %v", c.name, c.ok, conflict)
		}
	}
}

// REQ: CM-014
func TestCounterAmbiguityConfigurations(t *testing.T) {
	// sequence{1,2}(a{2,3}): po trzech a licznik może być (1, 3) albo (2, 1).
	m := compileModel(seq(1, 2, el("a", 2, 3)), nil)
	if c := m.checkUPA(); c != nil {
		t.Fatalf("UPA: %v", c)
	}
	if !m.ambiguous {
		t.Fatal("counter ambiguity not detected")
	}
	for n, want := range map[int]bool{1: false, 2: true, 3: true, 4: true, 5: true, 6: true, 7: false} {
		if got := accepts(m, strings.Repeat("a ", n)); got != want {
			t.Errorf("a^%d: %v, want %v", n, got, want)
		}
	}
	// Zbiór konfiguracji rośnie z liczbą powtórzeń w (a{1,1000}){1,1000}; limit kończy dopasowanie sygnałem.
	big := compileModel(seq(1, 1000, el("a", 1, 1000)), nil)
	mt := big.NewMatcher()
	for i := 0; i < 5000 && !mt.Overflow; i++ {
		if _, ok := mt.Step(QName{Local: "a"}); !ok {
			break
		}
	}
	if !mt.Overflow {
		t.Fatal("configuration limit not reached")
	}
}
