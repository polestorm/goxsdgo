package schema

import (
	"math"
	"strings"

	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// Particle Valid (Restriction) (XSD 1.0 2e, część 1, 3.9.6) i reguły pomocnicze: Occurrence Range OK,
// NameAndTypeOK, NSCompat, RecurseAsIfGroup, NSSubset, NSRecurseCheckCardinality, Recurse, RecurseLax,
// RecurseUnordered, MapAndSum, Particle Emptiable.

// rKind to rodzaj cząstki po normalizacji.
type rKind uint8

const (
	rElement rKind = iota
	rWildcard
	rAll
	rChoice
	rSequence
)

// rp to cząstka po normalizacji (punkty 2.1 i 2.2 Particle Valid (Restriction)).
type rp struct {
	min, max int64
	kind     rKind
	decl     *ElementDecl
	wild     *Wildcard
	kids     []*rp
	orig     *Particle
	// subst: choice powstały z głowy grupy podstawienia (punkt 2.1); kolejność jego cząstek nie jest określona.
	subst bool
}

// restrictor sprawdza restrykcje cząstek w jednym schemacie; anyTypeWild to symbol wieloznaczny treści anyType
// (wyjątek w NSSubset, punkt 3).
type restrictor struct {
	subst map[*ElementDecl][]*ElementDecl
	memo  map[[2]*rp]bool
	// why: pierwszy powód odrzucenia (do komunikatu).
	why string
}

func newRestrictor(subst map[*ElementDecl][]*ElementDecl) *restrictor {
	return &restrictor{subst: subst, memo: map[[2]*rp]bool{}}
}

// validRestriction mówi, czy cząstka r jest ·valid restriction· cząstki b.
func (x *restrictor) validRestriction(r, b *Particle) bool {
	if r == b {
		return true // punkt 1: ta sama cząstka
	}
	nr, nb := x.normalize(r, nil), x.normalize(b, nil)
	switch {
	case nr == nil && nb == nil:
		return true
	case nr == nil:
		return emptiableRP(nb) || x.fail("the restriction is empty but the base content is not emptiable")
	case nb == nil:
		return x.fail("the base content is empty")
	}
	return x.ok(nr, nb)
}

func (x *restrictor) fail(why string) bool {
	if x.why == "" {
		x.why = why
	}
	return false
}

// normalize stosuje punkt 2.1 (głowa grupy podstawienia jak choice) i 2.2 (pominięcie zbędnych grup) do cząstki p,
// której rodzicem jest grupa parent (nil na szczycie).
func (x *restrictor) normalize(p *Particle, parent *ModelGroup) *rp {
	if p == nil {
		return nil
	}
	switch t := p.Term.(type) {
	case *ElementDecl:
		if t.Scope.Global {
			if g := x.subst[t]; len(g) > 1 || (len(g) == 1 && g[0] != t) {
				c := &rp{min: p.MinOccurs, max: p.MaxOccurs, kind: rChoice, orig: p, subst: true}
				for _, e := range g {
					c.kids = append(c.kids, &rp{min: 1, max: 1, kind: rElement, decl: e, orig: p})
				}
				return c
			}
		}
		return &rp{min: p.MinOccurs, max: p.MaxOccurs, kind: rElement, decl: t, orig: p}
	case *Wildcard:
		return &rp{min: p.MinOccurs, max: p.MaxOccurs, kind: rWildcard, wild: t, orig: p}
	case *ModelGroup:
		n := &rp{min: p.MinOccurs, max: p.MaxOccurs, orig: p}
		switch t.Compositor {
		case CompositorAll:
			n.kind = rAll
		case CompositorChoice:
			n.kind = rChoice
		default:
			n.kind = rSequence
		}
		for _, k := range t.Particles {
			c := x.normalize(k, t)
			if c == nil {
				continue
			}
			// Zbędna grupa dziecka tego samego rodzaju (sekwencja w sekwencji, choice w choice) z licznością 1..1:
			// jej cząstki wchodzą wprost do rodzica.
			// Dotyczy też głowy grupy podstawienia rozwiniętej do choice (punkt 2.1) wewnątrz choice.
			if c.min == 1 && c.max == 1 && c.kind == n.kind && (n.kind == rSequence || n.kind == rChoice) && c.orig == k {
				n.kids = append(n.kids, c.kids...)
				n.subst = n.subst || c.subst
				continue
			}
			n.kids = append(n.kids, c)
		}
		one := p.MinOccurs == 1 && p.MaxOccurs == 1
		switch n.kind {
		case rSequence:
			if len(n.kids) == 0 {
				return nil
			}
			if one && len(n.kids) == 1 {
				return n.kids[0]
			}
		case rAll:
			if len(n.kids) == 0 {
				return nil
			}
			if len(n.kids) == 1 && one {
				return n.kids[0]
			}
		case rChoice:
			if len(n.kids) == 0 && p.MinOccurs == 0 {
				return nil
			}
			if one && len(n.kids) == 1 {
				return n.kids[0]
			}
		}
		return n
	}
	return nil
}

// occursOK sprawdza Occurrence Range OK.
func occursOK(rmin, rmax, bmin, bmax int64) bool {
	if rmin < bmin {
		return false
	}
	return bmax == Unbounded || (rmax != Unbounded && rmax <= bmax)
}

func (x *restrictor) ok(r, b *rp) bool {
	key := [2]*rp{r, b}
	if v, seen := x.memo[key]; seen {
		return v
	}
	x.memo[key] = false // ochrona przed zapętleniem
	v := x.dispatch(r, b)
	x.memo[key] = v
	return v
}

func (x *restrictor) dispatch(r, b *rp) bool {
	switch r.kind {
	case rElement:
		switch b.kind {
		case rElement:
			return x.nameAndTypeOK(r, b)
		case rWildcard:
			return x.nsCompat(r, b)
		default:
			// RecurseAsIfGroup: element jako grupa rodzaju B z licznością 1..1, w której element zachowuje swoją
			// liczność (dosłownie według tekstu; particlesHa161 i K006 tego wymagają, particlesZ001 oczekuje
			// odwrotnie, differences.md, ADR 0007).
			g := &rp{min: 1, max: 1, kind: b.kind, kids: []*rp{r}}
			if b.kind == rChoice {
				return x.recurseLax(g, b)
			}
			return x.recurse(g, b)
		}
	case rWildcard:
		if b.kind == rWildcard {
			return x.nsSubset(r, b)
		}
		return x.fail("a wildcard cannot restrict an element or a group")
	case rAll:
		switch b.kind {
		case rWildcard:
			return x.nsRecurseCheckCardinality(r, b)
		case rAll:
			return x.recurse(r, b)
		}
	case rChoice:
		switch b.kind {
		case rWildcard:
			return x.nsRecurseCheckCardinality(r, b)
		case rChoice:
			return x.recurseLax(r, b)
		}
	case rSequence:
		switch b.kind {
		case rWildcard:
			return x.nsRecurseCheckCardinality(r, b)
		case rAll:
			return x.recurseUnordered(r, b)
		case rChoice:
			return x.mapAndSum(r, b)
		case rSequence:
			return x.recurse(r, b)
		}
	}
	return x.fail("a " + kindName(r.kind) + " cannot restrict a " + kindName(b.kind))
}

func kindName(k rKind) string {
	return [...]string{"element", "wildcard", "all group", "choice", "sequence"}[k]
}

// nameAndTypeOK: Particle Restriction OK (Elt:Elt -- NameAndTypeOK).
func (x *restrictor) nameAndTypeOK(r, b *rp) bool {
	rd, bd := r.decl, b.decl
	if rd.Name != bd.Name {
		return x.fail("element " + rd.Name.String() + " does not correspond to " + bd.Name.String())
	}
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of " + rd.Name.String() + " is wider than in the base")
	}
	if rd.Scope.Global && bd.Scope.Global {
		return true // 3.1
	}
	if rd.Nillable && !bd.Nillable {
		return x.fail("element " + rd.Name.String() + " is nillable, but not in the base")
	}
	if bv := bd.Value; bv != nil && bv.Fixed {
		rv := rd.Value
		if rv == nil || !rv.Fixed || !sameElementValue(rv, bv, rd.Type, bd.Type) {
			return x.fail("element " + rd.Name.String() + " must keep the fixed value of the base")
		}
	}
	for _, ic := range rd.IdentityConstraints {
		found := false
		for _, bic := range bd.IdentityConstraints {
			if bic == ic || bic.Name == ic.Name {
				found = true
			}
		}
		if !found {
			return x.fail("element " + rd.Name.String() + " has identity constraints the base does not have")
		}
	}
	if rd.Block&bd.Block != bd.Block {
		return x.fail("element " + rd.Name.String() + " must block at least the substitutions blocked in the base")
	}
	if rd.Type != nil && bd.Type != nil && !derivedOK(rd.Type, bd.Type, DerivationExtension|DerivationList|DerivationUnion) {
		return x.fail("the type of element " + rd.Name.String() + " is not a restriction of its type in the base")
	}
	return true
}

func sameElementValue(a, b *ValueConstraint, at, bt Type) bool {
	as, bs := simpleOf(at), simpleOf(bt)
	if as != nil && bs != nil {
		return sameValue(a, b, as, bs)
	}
	return strings.TrimSpace(a.Lexical) == strings.TrimSpace(b.Lexical)
}

func simpleOf(t Type) *SimpleType {
	switch t := t.(type) {
	case *SimpleType:
		return t
	case *ComplexType:
		if t.Content == ContentSimple {
			return t.SimpleContent
		}
	}
	return nil
}

// nsCompat: Particle Derivation OK (Elt:Any -- NSCompat).
func (x *restrictor) nsCompat(r, b *rp) bool {
	if !b.wild.Namespaces.Allows(r.decl.Name.Space) {
		return x.fail("element " + r.decl.Name.String() + " is not allowed by the base wildcard")
	}
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of " + r.decl.Name.String() + " is wider than that of the base wildcard")
	}
	return true
}

// nsSubset: Particle Derivation OK (Any:Any -- NSSubset).
func (x *restrictor) nsSubset(r, b *rp) bool {
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of the wildcard is wider than in the base")
	}
	if !wildcardSubset(r.wild.Namespaces, b.wild.Namespaces) {
		return x.fail("the wildcard allows namespaces the base wildcard does not")
	}
	if b.wild != anyTypeWildcard() && r.wild.Process > b.wild.Process {
		return x.fail("the wildcard must process contents at least as strictly as the base")
	}
	return true
}

// nsRecurseCheckCardinality: Particle Derivation OK (All/Choice/Sequence:Any -- NSRecurseCheckCardinality).
// Punkt 1 sprawdzamy względem symbolu wieloznacznego bez ograniczenia liczności (0..unbounded): liczność całej
// grupy sprawdza punkt 2. Dosłowne odczytanie (liczność każdej cząstki względem liczności symbolu) odrzuca np.
// all(e1, e2, e3) jako restrykcję any{3,3}, a zestaw W3C (particlesHa070, Q013, R013) oczekuje „poprawny” (ADR 0007).
func (x *restrictor) nsRecurseCheckCardinality(r, b *rp) bool {
	loose := &rp{min: 0, max: Unbounded, kind: rWildcard, wild: b.wild, orig: b.orig}
	for _, k := range r.kids {
		if !x.ok(k, loose) {
			return false
		}
	}
	mn, mx := effectiveRange(r)
	if !occursOK(mn, mx, b.min, b.max) {
		return x.fail("the effective total range of the group is wider than that of the base wildcard")
	}
	return true
}

// recurse: Particle Derivation OK (All:All,Sequence:Sequence -- Recurse). Odwzorowanie zachowujące kolejność
// szukamy programowaniem dynamicznym: pierwsze pasujące dopasowanie nie zawsze jest właściwe.
func (x *restrictor) recurse(r, b *rp) bool {
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of the group is wider than in the base")
	}
	n, m := len(r.kids), len(b.kids)
	memo := make(map[[2]int]bool)
	var feasible func(i, j int) bool
	feasible = func(i, j int) bool {
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		v := false
		if i == n {
			v = true
			for _, k := range b.kids[j:] {
				if !emptiableRP(k) {
					v = false
					break
				}
			}
		} else {
			for k := j; k < m; k++ {
				if x.ok(r.kids[i], b.kids[k]) && feasible(i+1, k+1) {
					v = true
					break
				}
				if !emptiableRP(b.kids[k]) {
					break
				}
			}
		}
		memo[key] = v
		return v
	}
	if !feasible(0, 0) {
		return x.fail("the particles of the group do not map in order onto the particles of the base")
	}
	return true
}

// recurseLax: Particle Derivation OK (Choice:Choice -- RecurseLax).
func (x *restrictor) recurseLax(r, b *rp) bool {
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of the choice is wider than in the base")
	}
	if r.subst || b.subst {
		// Choice z grupy podstawienia: kolejność członków nie jest określona, więc odwzorowanie nie może od niej
		// zależeć (każda cząstka R ma swoją, różną cząstkę B).
		if !x.injective(r.kids, b.kids) {
			return x.fail("the particles of the choice do not map onto the particles of the base choice")
		}
		return true
	}
	n, m := len(r.kids), len(b.kids)
	memo := make(map[[2]int]bool)
	var feasible func(i, j int) bool
	feasible = func(i, j int) bool {
		if i == n {
			return true
		}
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		v := false
		for k := j; k < m; k++ {
			if x.ok(r.kids[i], b.kids[k]) && feasible(i+1, k+1) {
				v = true
				break
			}
		}
		memo[key] = v
		return v
	}
	if !feasible(0, 0) {
		return x.fail("the particles of the choice do not map in order onto the particles of the base choice")
	}
	return true
}

// recurseUnordered: Particle Derivation OK (Sequence:All -- RecurseUnordered).
func (x *restrictor) recurseUnordered(r, b *rp) bool {
	if !occursOK(r.min, r.max, b.min, b.max) {
		return x.fail("the occurrence range of the sequence is wider than that of the base all group")
	}
	used := make([]bool, len(b.kids))
	// Dopasowanie w grafie dwudzielnym (ścieżki powiększające); grupy all są małe.
	match := make([]int, len(b.kids))
	for i := range match {
		match[i] = -1
	}
	var try func(i int, seen []bool) bool
	try = func(i int, seen []bool) bool {
		for k := range b.kids {
			if seen[k] || !x.ok(r.kids[i], b.kids[k]) {
				continue
			}
			seen[k] = true
			if match[k] < 0 || try(match[k], seen) {
				match[k] = i
				return true
			}
		}
		return false
	}
	for i := range r.kids {
		if !try(i, make([]bool, len(b.kids))) {
			return x.fail("a particle of the sequence does not map onto a particle of the base all group")
		}
	}
	for k := range b.kids {
		used[k] = match[k] >= 0
		if !used[k] && !emptiableRP(b.kids[k]) {
			return x.fail("a required particle of the base all group is missing")
		}
	}
	return true
}

// injective szuka odwzorowania różnowartościowego cząstek rs w bs (ścieżki powiększające).
func (x *restrictor) injective(rs, bs []*rp) bool {
	match := make([]int, len(bs))
	for i := range match {
		match[i] = -1
	}
	var try func(i int, seen []bool) bool
	try = func(i int, seen []bool) bool {
		for k := range bs {
			if seen[k] || !x.ok(rs[i], bs[k]) {
				continue
			}
			seen[k] = true
			if match[k] < 0 || try(match[k], seen) {
				match[k] = i
				return true
			}
		}
		return false
	}
	for i := range rs {
		if !try(i, make([]bool, len(bs))) {
			return false
		}
	}
	return true
}

// mapAndSum: Particle Derivation OK (Sequence:Choice -- MapAndSum).
func (x *restrictor) mapAndSum(r, b *rp) bool {
	for _, k := range r.kids {
		found := false
		for _, bk := range b.kids {
			if x.ok(k, bk) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	n := int64(len(r.kids))
	mn := mulOcc(r.min, n)
	mx := int64(Unbounded)
	if r.max != Unbounded {
		mx = mulOcc(r.max, n)
	}
	if !occursOK(mn, mx, b.min, b.max) {
		return x.fail("the sequence occurs more often than the base choice allows")
	}
	return true
}

func mulOcc(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

func addOcc(a, b int64) int64 {
	if a == Unbounded || b == Unbounded {
		return Unbounded
	}
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// effectiveRange liczy Effective Total Range (all and sequence) albo (choice) grupy r.
func effectiveRange(r *rp) (int64, int64) {
	var mn, mx int64
	first := true
	unb := false
	for _, k := range r.kids {
		kmin, kmax := k.min, k.max
		if k.kind == rAll || k.kind == rChoice || k.kind == rSequence {
			kmin, kmax = effectiveRange(k)
		}
		if kmax == Unbounded {
			unb = true
		}
		switch r.kind {
		case rChoice:
			if first || kmin < mn {
				mn = kmin
			}
			if kmax != Unbounded && (first || kmax > mx) {
				mx = kmax
			}
		default:
			mn = addOcc(mn, kmin)
			if kmax != Unbounded {
				mx = addOcc(mx, kmax)
			}
		}
		first = false
	}
	mn = mulOcc(r.min, mn)
	switch {
	case unb:
		return mn, Unbounded
	case r.max == Unbounded && mx > 0:
		return mn, Unbounded
	case r.max == Unbounded:
		return mn, 0
	}
	return mn, mulOcc(r.max, mx)
}

func emptiableRP(r *rp) bool {
	if r.min == 0 {
		return true
	}
	if r.kind == rElement || r.kind == rWildcard {
		return false
	}
	mn, _ := effectiveRange(r)
	return mn == 0
}

// anyTypeWildcard zwraca symbol wieloznaczny treści anyType (NSSubset, punkt 3).
func anyTypeWildcard() *Wildcard {
	initBuiltins()
	if anyType.Particle == nil {
		return nil
	}
	g, ok := anyType.Particle.Term.(*ModelGroup)
	if !ok || len(g.Particles) == 0 {
		return nil
	}
	w, _ := g.Particles[0].Term.(*Wildcard)
	return w
}

var _ = datatypes.Builtin
