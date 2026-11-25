package schema

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestMergedIndexMatchesSequential: scalony indeks krawędzi sekwencji (etap 7, ADR 0011) daje dla każdej nazwy tę
// samą pozycję co przeglądanie dzieci po kolei, także z powtórzonymi nazwami, symbolami wieloznacznymi przed
// i po deklaracjach tej samej przestrzeni i dziećmi-grupami. Modele nie muszą spełniać UPA: liczy się tylko
// równoważność obu dróg wyszukiwania. REQ: DET-001, CM-013
func TestMergedIndexMatchesSequential(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	spaces := []string{"", "urn:a", "urn:b"}
	names := []QName{}
	for _, s := range spaces {
		for i := 0; i < 6; i++ {
			names = append(names, QName{Space: s, Local: fmt.Sprintf("e%d", i)})
		}
	}
	leaf := func() *Particle {
		switch rnd.Intn(6) {
		case 0:
			return wild(0, 1, NamespaceConstraint{Kind: NamespaceSet, Set: []string{spaces[rnd.Intn(3)]}})
		case 1:
			return wild(0, 1, NamespaceConstraint{Kind: NamespaceNot, Not: spaces[rnd.Intn(3)]})
		case 2:
			return cho(0, 1, el(fmt.Sprintf("e%d", rnd.Intn(6)), 1, 1), el(fmt.Sprintf("e%d", rnd.Intn(6)), 1, 1))
		}
		n := names[rnd.Intn(len(names))]
		p := el(n.Local, 0, 1)
		p.Term.(*ElementDecl).Name = n
		return p
	}
	checked, merged := 0, 0
	for trial := 0; trial < 300; trial++ {
		var kids []*Particle
		for i := 0; i < 3+rnd.Intn(30); i++ {
			kids = append(kids, leaf())
		}
		if rnd.Intn(3) == 0 {
			kids[rnd.Intn(len(kids))].MinOccurs = 1 // dziecko bez pustki skraca rozpiętość krawędzi
		}
		m := compileModel(seq(1, 1, kids...), nil)
		var edges []*cmEdge
		for i := range m.start {
			edges = append(edges, &m.start[i])
		}
		for p := range m.from {
			for i := range m.from[p] {
				edges = append(edges, &m.from[p][i])
			}
		}
		for _, e := range edges {
			if e.mergedIndex() != nil {
				merged++
			}
			for _, n := range append(names, QName{Space: "urn:c", Local: "x"}) {
				want := -1
				if e.target != nil {
					want = e.target.idx.lookup(n, m)
				} else {
					e.indexes(func(ix *posIdx) bool {
						want = ix.lookup(n, m)
						return want < 0
					})
				}
				if got := e.lookup(n, m); got != want {
					t.Fatalf("trial %d, name %v: merged lookup %d, sequential %d", trial, n, got, want)
				}
				checked++
			}
		}
	}
	if merged == 0 {
		t.Fatal("no edge used a merged index")
	}
	t.Logf("%d lookups compared, %d edges with a merged index", checked, merged)
}
