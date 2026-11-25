package schema

import (
	"testing"
)

// FuzzContentModel porównuje automat z licznikami z prostym dopasowaniem wzorcowym, napisanym wprost z definicji
// (Element Sequence Locally Valid (Particle), 3.9.4) przez zbiory pozycji końcowych, bez automatu i bez liczników.
// Model i ciąg dzieci powstają z danych fuzzera; porównujemy tylko modele spełniające UPA, bo dla innych automat
// z definicji wybiera jedną cząstkę. REQ: CM-002, CM-007, CM-014
func FuzzContentModel(f *testing.F) {
	f.Add([]byte{2, 1, 0, 3, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 2, 0, 1, 1, 0})
	f.Add([]byte{3, 0, 2, 2, 0, 0, 2, 4, 0, 1, 1, 2, 0, 0, 0, 1, 1, 0, 2})
	f.Add([]byte{2, 2, 1, 2, 0, 0, 3, 1, 1, 2, 1, 0, 2, 2, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		g := &modelGen{data: data}
		p := g.particle(0)
		if p == nil {
			return
		}
		m := compileModel(p, nil)
		if m.checkUPA() != nil {
			return
		}
		in := g.input()
		mt := m.NewMatcher()
		got := true
		for _, q := range in {
			if _, ok := mt.Step(q); !ok {
				got = false
				break
			}
		}
		if mt.Overflow {
			return
		}
		got = got && mt.Accepting()
		want := refEnds(p, in, 0)[len(in)]
		if got != want {
			t.Fatalf("model %s, input %v: automaton %v, reference %v", describeParticle(p), in, got, want)
		}
	})
}

type modelGen struct {
	data []byte
	i    int
}

func (g *modelGen) next() int {
	if g.i >= len(g.data) {
		return 0
	}
	b := g.data[g.i]
	g.i++
	return int(b)
}

var fuzzNames = []string{"a", "b", "c"}

func (g *modelGen) particle(depth int) *Particle {
	kind := g.next() % 4
	if depth >= 3 && kind >= 2 {
		kind = 0
	}
	mn := int64(g.next() % 3)
	mx := mn + int64(g.next()%3)
	if mx == 0 {
		mx = 1
	}
	if g.next()%5 == 0 {
		mx = Unbounded
	}
	switch kind {
	case 0:
		return el(fuzzNames[g.next()%3], mn, mx)
	case 1:
		nc := NamespaceConstraint{Kind: NamespaceSet, Set: []string{"u"}}
		if g.next()%2 == 0 {
			nc = NamespaceConstraint{Kind: NamespaceNot, Not: ""}
		}
		return wild(mn, mx, nc)
	}
	n := g.next() % 4
	var kids []*Particle
	for i := 0; i < n; i++ {
		if k := g.particle(depth + 1); k != nil {
			kids = append(kids, k)
		}
	}
	if kind == 2 {
		return seq(mn, mx, kids...)
	}
	return cho(mn, mx, kids...)
}

func (g *modelGen) input() []QName {
	n := g.next() % 9
	out := make([]QName, 0, n)
	for i := 0; i < n; i++ {
		switch v := g.next() % 4; v {
		case 3:
			out = append(out, QName{Space: "u", Local: "x"})
		default:
			out = append(out, QName{Local: fuzzNames[v]})
		}
	}
	return out
}

// refEnds zwraca zbiór pozycji w in, na których może się skończyć dopasowanie cząstki p zaczęte w start.
func refEnds(p *Particle, in []QName, start int) map[int]bool {
	out := map[int]bool{}
	if p.MinOccurs == 0 {
		out[start] = true
	}
	cur := map[int]bool{start: true}
	limit := int64(len(in)-start) + p.MinOccurs + 1
	for k := int64(1); (p.MaxOccurs == Unbounded || k <= p.MaxOccurs) && k <= limit && len(cur) > 0; k++ {
		next := map[int]bool{}
		for s := range cur {
			for e := range refBody(p.Term, in, s) {
				next[e] = true
			}
		}
		if k >= p.MinOccurs {
			for e := range next {
				out[e] = true
			}
		}
		cur = next
	}
	return out
}

func refBody(t Term, in []QName, s int) map[int]bool {
	out := map[int]bool{}
	switch t := t.(type) {
	case *ElementDecl:
		if s < len(in) && in[s] == t.Name {
			out[s+1] = true
		}
	case *Wildcard:
		if s < len(in) && t.Namespaces.Allows(in[s].Space) {
			out[s+1] = true
		}
	case *ModelGroup:
		switch t.Compositor {
		case CompositorChoice:
			for _, k := range t.Particles {
				for e := range refEnds(k, in, s) {
					out[e] = true
				}
			}
		default:
			cur := map[int]bool{s: true}
			for _, k := range t.Particles {
				next := map[int]bool{}
				for x := range cur {
					for e := range refEnds(k, in, x) {
						next[e] = true
					}
				}
				cur = next
			}
			out = cur
		}
	}
	return out
}

func describeParticle(p *Particle) string {
	occ := func() string {
		mx := "*"
		if p.MaxOccurs != Unbounded {
			mx = itoa(p.MaxOccurs)
		}
		return "{" + itoa(p.MinOccurs) + "," + mx + "}"
	}
	switch t := p.Term.(type) {
	case *ElementDecl:
		return t.Name.Local + occ()
	case *Wildcard:
		if t.Namespaces.Kind == NamespaceSet {
			return "any(u)" + occ()
		}
		return "any(##other)" + occ()
	case *ModelGroup:
		s := "seq("
		if t.Compositor == CompositorChoice {
			s = "cho("
		}
		for i, k := range t.Particles {
			if i > 0 {
				s += " "
			}
			s += describeParticle(k)
		}
		return s + ")" + occ()
	}
	return "?"
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
