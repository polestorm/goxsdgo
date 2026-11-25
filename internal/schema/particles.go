package schema

import (
	"math"
	"math/big"
	"sort"
	"strings"
)

// particlePlace mówi, gdzie stoi cząstka; od tego zależy dopuszczalność grupy all (cos-all-limited).
type particlePlace uint8

const (
	nested particlePlace = iota
	contentTop
	groupDefTop
)

// occurs czyta minOccurs i maxOccurs (domyślnie 1). Liczby większe niż int64 obcinamy do math.MaxInt64.
func occurs(n *node) (int64, int64) {
	parse := func(name string) int64 {
		v, ok := n.attr(name)
		if !ok {
			return 1
		}
		v = collapse(v)
		if v == "unbounded" {
			return Unbounded
		}
		b, ok := new(big.Int).SetString(strings.TrimPrefix(v, "+"), 10)
		if !ok || b.Sign() < 0 {
			return 1 // wartość błędną zgłosiła kontrola według schematu dla schematów
		}
		if !b.IsInt64() {
			return math.MaxInt64
		}
		return b.Int64()
	}
	return parse("minOccurs"), parse("maxOccurs")
}

// particleRange buduje cząstkę z liczności elementu n i sprawdza Particle Correct (3.9.6). Zwraca nil, gdy
// minOccurs = maxOccurs = 0 (element nie odpowiada żadnemu komponentowi).
func (l *loader) particleRange(n *node) (*Particle, bool) {
	mn, mx := occurs(n)
	if mn == 0 && mx == 0 {
		return nil, false
	}
	if mx != Unbounded && mn > mx {
		l.errorf(CodeParticleProps+".2.1", n.pos(), "minOccurs (%d) is greater than maxOccurs (%d)", mn, mx)
	}
	if mx != Unbounded && mx < 1 {
		l.errorf(CodeParticleProps+".2.2", n.pos(), "maxOccurs must be at least 1")
	}
	return &Particle{MinOccurs: mn, MaxOccurs: mx, Pos: n.pos()}, true
}

// buildParticle buduje cząstkę z elementu element, group (ref), all, choice, sequence albo any. scope to typ
// złożony, w którym leżą deklaracje lokalne (nil w grupie nazwanej).
func (l *loader) buildParticle(n *node, scope *ComplexType, place particlePlace) *Particle {
	switch n.local {
	case "element":
		return l.buildElementParticle(n, scope)
	case "any":
		p, ok := l.particleRange(n)
		if !ok {
			return nil
		}
		p.Term = l.buildWildcard(n)
		return p
	case "group":
		v, ok := n.attr("ref")
		if !ok {
			return nil
		}
		p, ok := l.particleRange(n)
		df := l.resolve(n, "ref", v, kindGroup)
		if !ok || df == nil {
			return nil
		}
		if df.state == stateBuilding {
			l.errorf(CodeModelGroupProps+".2", n.attrPos("ref"), "circular model group: %s contains itself", df.name)
			return nil
		}
		l.ensure(df)
		g := df.comp.(*ModelGroupDef).Group
		p.Term = g
		if g.Compositor == CompositorAll {
			l.checkAllPlace(n, p, place)
		}
		return p
	case "all", "choice", "sequence":
		p := &Particle{MinOccurs: 1, MaxOccurs: 1, Pos: n.pos()}
		if place != groupDefTop {
			var ok bool
			if p, ok = l.particleRange(n); !ok {
				return nil
			}
		}
		g := &ModelGroup{Pos: n.pos()}
		switch n.local {
		case "all":
			g.Compositor = CompositorAll
			l.checkAllPlace(n, p, place)
		case "choice":
			g.Compositor = CompositorChoice
		}
		for _, k := range n.xsdKids() {
			if c := l.buildParticle(k, scope, nested); c != nil {
				g.Particles = append(g.Particles, c)
			}
		}
		p.Term = g
		return p
	}
	return nil
}

// checkAllPlace sprawdza All Group Limited (3.8.6): grupa all tylko jako cała treść typu złożonego z maxOccurs = 1
// albo jako grupa modelu definicji grupy.
func (l *loader) checkAllPlace(n *node, p *Particle, place particlePlace) {
	switch {
	case place == nested:
		l.errorf(CodeAllLimited+".1.2", n.pos(), "an all group may only be the whole content model of a type or a named group")
	case place == contentTop && p.MaxOccurs != 1:
		l.errorf(CodeAllLimited+".1.2", n.pos(), "an all group used as content must have maxOccurs 1")
	}
}

// buildGroupDef wypełnia definicję grupy modelu (3.7.2).
func (l *loader) buildGroupDef(df *def) {
	gd := df.comp.(*ModelGroupDef)
	for _, k := range df.node.xsdKids() {
		if k.local == "all" || k.local == "choice" || k.local == "sequence" {
			p := l.buildParticle(k, nil, groupDefTop)
			if p != nil {
				*gd.Group = *p.Term.(*ModelGroup)
			}
			if df.orig != nil {
				l.groupRedefs = append(l.groupRedefs, df)
			}
			return
		}
	}
}

// buildWildcard buduje symbol wieloznaczny z any albo anyAttribute (3.10.2).
func (l *loader) buildWildcard(n *node) *Wildcard {
	w := &Wildcard{Pos: n.pos()}
	switch v, _ := n.attr("processContents"); collapse(v) {
	case "lax":
		w.Process = ProcessLax
	case "skip":
		w.Process = ProcessSkip
	}
	v, ok := n.attr("namespace")
	f := strings.Fields(v)
	switch {
	case !ok || len(f) == 1 && f[0] == "##any":
		w.Namespaces = NamespaceConstraint{Kind: NamespaceAny}
	case len(f) == 1 && f[0] == "##other":
		w.Namespaces = NamespaceConstraint{Kind: NamespaceNot, Not: n.doc.tns}
	default:
		set := map[string]bool{}
		for _, t := range f {
			switch t {
			case "##targetNamespace":
				set[n.doc.tns] = true
			case "##local":
				set[""] = true
			default:
				set[t] = true
			}
		}
		w.Namespaces = NamespaceConstraint{Kind: NamespaceSet, Set: sortedSet(set)}
	}
	return w
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// emptiable mówi, czy cząstka jest ·emptiable· (Particle Emptiable, 3.9.6).
func emptiable(p *Particle) bool {
	return p == nil || minCount(p) == 0
}

// minCount to minimum z ·effective total range· cząstki (3.8.6).
func minCount(p *Particle) int64 {
	if p.MinOccurs == 0 {
		return 0
	}
	g, ok := p.Term.(*ModelGroup)
	if !ok {
		return p.MinOccurs
	}
	var m int64
	switch g.Compositor {
	case CompositorChoice:
		m = -1
		for _, c := range g.Particles {
			if x := minCount(c); m < 0 || x < m {
				m = x
			}
		}
		if m < 0 {
			m = 0
		}
	default:
		for _, c := range g.Particles {
			m += minCount(c)
		}
	}
	return p.MinOccurs * m
}

// ---- symbole wieloznaczne: Wildcard allows Namespace Name, Wildcard Subset, Union, Intersection (3.10.4, 3.10.6)

// Allows mówi, czy przestrzeń ns (pusta: brak) jest ·valid· względem ograniczenia (Wildcard allows Namespace Name).
func (c NamespaceConstraint) Allows(ns string) bool {
	switch c.Kind {
	case NamespaceAny:
		return true
	case NamespaceNot:
		return ns != c.Not && ns != ""
	}
	for _, s := range c.Set {
		if s == ns {
			return true
		}
	}
	return false
}

func (c NamespaceConstraint) has(ns string) bool {
	for _, s := range c.Set {
		if s == ns {
			return true
		}
	}
	return false
}

// wildcardSubset mówi, czy sub jest ·intensional subset· super (Wildcard Subset).
func wildcardSubset(sub, super NamespaceConstraint) bool {
	switch {
	case super.Kind == NamespaceAny:
		return true
	case sub.Kind == NamespaceNot:
		// Rekomendacja (punkt 2.2) wymaga tej samej wartości. not(x) wyklucza jednak zawsze także brak przestrzeni
		// (Wildcard allows Namespace Name 2.3), więc jest podzbiorem not(absent) dla każdego x; tak to czytamy,
		// zgodnie z oczekiwaniami zestawu W3C (msData/wildcards, wildZ013), ADR 0006.
		return super.Kind == NamespaceNot && (super.Not == sub.Not || super.Not == "")
	case sub.Kind == NamespaceSet:
		if super.Kind == NamespaceSet {
			for _, s := range sub.Set {
				if !super.has(s) {
					return false
				}
			}
			return true
		}
		if super.Kind == NamespaceNot {
			return !sub.has(super.Not) && !sub.has("")
		}
	}
	return false
}

func equalConstraint(a, b NamespaceConstraint) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case NamespaceNot:
		return a.Not == b.Not
	case NamespaceSet:
		if len(a.Set) != len(b.Set) {
			return false
		}
		for i := range a.Set {
			if a.Set[i] != b.Set[i] {
				return false
			}
		}
	}
	return true
}

// wildcardUnion liczy ·intensional union· (Attribute Wildcard Union); ok == false, gdy nie da się jej wyrazić.
func wildcardUnion(a, b NamespaceConstraint) (NamespaceConstraint, bool) {
	any := NamespaceConstraint{Kind: NamespaceAny}
	switch {
	case equalConstraint(a, b):
		return a, true
	case a.Kind == NamespaceAny || b.Kind == NamespaceAny:
		return any, true
	case a.Kind == NamespaceSet && b.Kind == NamespaceSet:
		m := map[string]bool{}
		for _, s := range a.Set {
			m[s] = true
		}
		for _, s := range b.Set {
			m[s] = true
		}
		return NamespaceConstraint{Kind: NamespaceSet, Set: sortedSet(m)}, true
	case a.Kind == NamespaceNot && b.Kind == NamespaceNot:
		return NamespaceConstraint{Kind: NamespaceNot, Not: ""}, true
	}
	not, set := a, b
	if a.Kind == NamespaceSet {
		not, set = b, a
	}
	if not.Not != "" {
		hasNeg, hasAbsent := set.has(not.Not), set.has("")
		switch {
		case hasNeg && hasAbsent:
			return any, true
		case hasNeg:
			return NamespaceConstraint{Kind: NamespaceNot, Not: ""}, true
		case hasAbsent:
			return NamespaceConstraint{}, false
		}
		return not, true
	}
	if set.has("") {
		return any, true
	}
	return NamespaceConstraint{Kind: NamespaceNot, Not: ""}, true
}

// wildcardIntersect liczy ·intensional intersection· (Attribute Wildcard Intersection).
func wildcardIntersect(a, b NamespaceConstraint) (NamespaceConstraint, bool) {
	switch {
	case equalConstraint(a, b):
		return a, true
	case a.Kind == NamespaceAny:
		return b, true
	case b.Kind == NamespaceAny:
		return a, true
	case a.Kind == NamespaceSet && b.Kind == NamespaceSet:
		m := map[string]bool{}
		for _, s := range a.Set {
			if b.has(s) {
				m[s] = true
			}
		}
		return NamespaceConstraint{Kind: NamespaceSet, Set: sortedSet(m)}, true
	case a.Kind == NamespaceNot && b.Kind == NamespaceNot:
		switch {
		case a.Not == "":
			return b, true
		case b.Not == "":
			return a, true
		}
		return NamespaceConstraint{}, false
	}
	not, set := a, b
	if a.Kind == NamespaceSet {
		not, set = b, a
	}
	m := map[string]bool{}
	for _, s := range set.Set {
		if s != not.Not && s != "" {
			m[s] = true
		}
	}
	return NamespaceConstraint{Kind: NamespaceSet, Set: sortedSet(m)}, true
}
