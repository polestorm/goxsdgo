package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Model zawartości skompilowany do automatu pozycji z licznikami.
//
// Konstrukcja: automat pozycji (Glushkov; V. M. Glushkov, „The abstract theory of automata”, 1961; A. Brüggemann-Klein,
// D. Wood, „One-unambiguous regular languages”, Information and Computation 140, 1998) rozszerzony o liczniki dla
// cząstek z licznościami innymi niż 0..1, 1..1 i 0..unbounded/1..unbounded (C. M. Sperberg-McQueen, „Notes on finite
// state automata with counters”, W3C 2004; W. Gelade, M. Gyssens, W. Martens, „Regular expressions with counting:
// weak versus strong determinism”, SIAM J. Comput. 41(1), 2012). Liczności nie są rozwijane: maxOccurs="100000" to
// jeden licznik, nie 100 000 stanów.
//
// Pozycje to cząstki-liście (deklaracje elementów i symbole wieloznaczne). Konfiguracja to pozycja ostatnio
// dopasowanego dziecka i wartości liczników jej przodków z licznikiem (od korzenia). Przejście z pozycji p do q
// zachodzi w węźle J: albo J jest sekwencją, w której po gałęzi z p stoją (po gałęziach dopuszczających pustkę)
// gałęzie z q, albo J jest cząstką powtarzaną, której kolejna iteracja zaczyna się od q. Przejście wymaga, by węzły
// opuszczane pod J miały wykonane co najmniej minOccurs iteracji („wyjście”), a powtórzenie w J — by licznik J był
// mniejszy od maxOccurs; węzły wchodzone nad q dostają licznik 1.
//
// Przejść nie materializujemy parami (dla choice* z n cząstkami byłoby ich n²): z każdej pozycji prowadzi kilka
// krawędzi, a każda wskazuje zbiór celów — pierwsze pozycje węzła albo kolejnych dzieci sekwencji. Każdy węzeł ma
// indeks swoich pierwszych pozycji według nazwy elementu i przestrzeni nazw symbolu wieloznacznego. Krok dopasowania
// kosztuje więc tyle, ile krawędzi ma pozycja, niezależnie od długości dokumentu.
//
// Dla modeli spełniających Unique Particle Attribution każde dziecko trafia w jedną pozycję. Wartości liczników
// mogą jednak zależeć od tego, na którym poziomie nastąpiło powtórzenie (np. sekwencja z maxOccurs="2" i w niej
// element z minOccurs="2" maxOccurs="3"); wtedy dopasowanie trzyma zbiór konfiguracji, ograniczony przez
// MaxConfigurations (ADR 0007).

// MaxConfigurations ogranicza liczbę równoczesnych konfiguracji dopasowania jednego elementu. Modele bez
// niejednoznaczności liczników mają zawsze jedną konfigurację.
const MaxConfigurations = 1024

type cmKind uint8

const (
	cmElement cmKind = iota
	cmWildcard
	cmSequence
	cmChoice
)

type cmNode struct {
	kind     cmKind
	id       int
	min, max int64
	particle *Particle
	parent   *cmNode
	kids     []*cmNode
	// counted: węzeł trzyma licznik iteracji; cap to największa wartość, którą warto pamiętać.
	counted      bool
	cap          int64
	bodyNullable bool
	nullable     bool
	first        []int   // pozycje
	lastSet      bitset  // pozycje, po których węzeł może się skończyć
	idx          *posIdx // indeks pierwszych pozycji
	// Liście:
	pos     int
	decl    *ElementDecl
	members []*ElementDecl
	byName  map[QName]*ElementDecl // członkowie według nazwy, dla dużych grup podstawienia
	wild    *Wildcard
}

func (n *cmNode) repeatable() bool { return n.max == Unbounded || n.max > 1 }

// posIdx indeksuje zbiór pozycji według etykiety.
type posIdx struct {
	byName    map[QName]int // pozycja elementu; UPA wyklucza dwie różne, ale bez UPA bierzemy pierwszą
	byNameAll map[QName][]int
	wildSet   map[string][]int // symbole wieloznaczne z listą przestrzeni, według przestrzeni
	wildOther []int            // ##any i ##other
	all       []int
	// small to te same wpisy co byName (pierwsza pozycja dla nazwy), gdy jest ich najwyżej smallIndex: porównanie
	// kilku napisów jest szybsze niż mieszanie QName w mapie, a w modelach JPK większość indeksów ma jedną nazwę
	// (etap 7, ADR 0011). useSmall mówi, czy lista jest kompletna.
	small    []namePos
	useSmall bool
}

type namePos struct {
	name QName
	pos  int
}

// smallIndex to największa liczba nazw, dla której indeks przeszukujemy liniowo.
const smallIndex = 8

type bitset []uint64

func newBitset(n int) bitset    { return make(bitset, (n+63)/64) }
func (b bitset) set(i int)      { b[i/64] |= 1 << (uint(i) % 64) }
func (b bitset) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

// cmEdge to grupa przejść z jednej pozycji (albo ze startu): do pierwszych pozycji węzła target, albo do pierwszych
// pozycji dzieci sekwencji seq od indeksu from do pierwszego dziecka niedopuszczającego pustki.
type cmEdge struct {
	target *cmNode
	seq    *cmNode
	from   int
	join   *cmNode
	// iter: węzeł powtarzany (nil dla przejścia w sekwencji i startu); incr: indeks jego licznika na ścieżce źródła
	// (-1 bez licznika); keep: liczba liczników przenoszonych bez zmian; exits: indeksy liczników źródła, które
	// muszą pozwolić na wyjście.
	iter  *cmNode
	incr  int
	keep  int
	exits []int
	// lazy to scalony indeks pozycji krawędzi w sekwencji (etap 7, ADR 0011), budowany przy pierwszym użyciu;
	// wskaźnik, bo krawędzie są kopiowane, a indeks ma być jeden. nil dla krawędzi do jednego węzła.
	lazy *edgeIndex
}

// edgeIndex to leniwie budowany scalony indeks krawędzi; model jest używany z wielu goroutines, więc budowa
// przez sync.Once.
type edgeIndex struct {
	once sync.Once
	ix   *mergedIdx
}

// mergedIdx łączy indeksy kolejnych dzieci sekwencji, do których prowadzi krawędź (do pierwszego dziecka bez
// pustki), z tym samym pierwszeństwem co przeglądanie po kolei: wcześniejsze dziecko wygrywa, a w obrębie jednego
// dziecka deklaracja elementu wygrywa z symbolem wieloznacznym.
type mergedIdx struct {
	byName map[QName]orderedPos
	wild   []orderedIdx // dzieci z symbolami wieloznacznymi, w kolejności
}

type orderedPos struct{ pos, order int }

type orderedIdx struct {
	order int
	ix    *posIdx
}

// Rozpiętość krawędzi (liczba przeglądanych dzieci), dla której opłaca się indeks scalony: przy krótszej
// przeglądanie po kolei jest tańsze, dłuższa ograniczałaby pamięć modelu z tysiącami cząstek.
const (
	minMergedSpan = 5
	maxMergedSpan = 256
)

// mergedIndex zwraca indeks scalony krawędzi albo nil, gdy jej rozpiętość jest poza zakresem.
func (e *cmEdge) mergedIndex() *mergedIdx {
	if e.lazy == nil {
		return nil
	}
	e.lazy.once.Do(func() {
		var span []*cmNode
		for _, k := range e.seq.kids[e.from:] {
			span = append(span, k)
			if !k.nullable || len(span) > maxMergedSpan {
				break
			}
		}
		if len(span) < minMergedSpan || len(span) > maxMergedSpan {
			return
		}
		mi := &mergedIdx{byName: map[QName]orderedPos{}}
		for order, k := range span {
			for name, p := range k.idx.byName {
				if _, ok := mi.byName[name]; !ok {
					mi.byName[name] = orderedPos{p, order}
				}
			}
			if len(k.idx.wildSet) > 0 || len(k.idx.wildOther) > 0 {
				mi.wild = append(mi.wild, orderedIdx{order, k.idx})
			}
		}
		e.lazy.ix = mi
	})
	return e.lazy.ix
}

func (mi *mergedIdx) lookup(name QName, m *ContentModel) int {
	p, order := -1, maxMergedSpan+1
	if op, ok := mi.byName[name]; ok {
		p, order = op.pos, op.order
	}
	for _, w := range mi.wild {
		if w.order >= order {
			break
		}
		if q := w.ix.lookupWild(name, m); q >= 0 {
			return q
		}
	}
	return p
}

// lookup zwraca pozycję, do której krawędź prowadzi dla elementu name, albo -1.
func (e *cmEdge) lookup(name QName, m *ContentModel) int {
	if e.target != nil {
		return e.target.idx.lookup(name, m)
	}
	if mi := e.mergedIndex(); mi != nil {
		return mi.lookup(name, m)
	}
	p := -1
	e.indexes(func(ix *posIdx) bool {
		p = ix.lookup(name, m)
		return p < 0
	})
	return p
}

// ContentModel to skompilowany model zawartości typu złożonego. Jest tylko do odczytu, więc można go używać
// z wielu goroutines naraz; stan dopasowania trzyma Matcher.
type ContentModel struct {
	root      *cmNode
	nodes     []*cmNode
	leaves    []*cmNode
	counters  [][]*cmNode // liczniki na ścieżce każdej pozycji, od korzenia
	start     []cmEdge
	from      [][]cmEdge
	all       *allModel
	nullable  bool
	ambiguous bool // niejednoznaczność liczników: dopasowanie może trzymać kilka konfiguracji
}

// allModel to grupa all (3.8): każdy element najwyżej raz, w dowolnej kolejności.
type allModel struct {
	min    int64
	leaves []*cmNode
	byName map[QName]int
}

// compileModel buduje automat dla cząstki p (nil: treść pusta). subst to grupy podstawienia schematu.
func compileModel(p *Particle, subst map[*ElementDecl][]*ElementDecl) *ContentModel {
	m := &ContentModel{}
	if p == nil {
		m.nullable = true
		return m
	}
	if g, ok := p.Term.(*ModelGroup); ok && g.Compositor == CompositorAll {
		a := &allModel{min: p.MinOccurs, byName: map[QName]int{}}
		for _, k := range g.Particles {
			if n := m.leaf(k, subst); n != nil {
				a.leaves = append(a.leaves, n)
			}
		}
		for i, n := range a.leaves {
			for _, e := range n.members {
				if _, dup := a.byName[e.Name]; !dup {
					a.byName[e.Name] = i
				}
			}
		}
		m.all = a
		m.nullable = true
		for _, n := range a.leaves {
			if n.min > 0 {
				m.nullable = p.MinOccurs == 0
			}
		}
		return m
	}
	m.root = m.build(p, nil, subst)
	if m.root == nil {
		m.nullable = true
		return m
	}
	m.counters = make([][]*cmNode, len(m.leaves))
	for i, lf := range m.leaves {
		var path []*cmNode
		for n := lf; n != nil; n = n.parent {
			if n.counted {
				path = append(path, n)
			}
		}
		for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
			path[a], path[b] = path[b], path[a]
		}
		m.counters[i] = path
	}
	m.computeSets(m.root)
	m.nullable = m.root.nullable
	m.start = []cmEdge{{target: m.root, incr: -1}}
	m.from = make([][]cmEdge, len(m.leaves))
	for i := range m.leaves {
		m.from[i] = m.edges(m.leaves[i])
	}
	return m
}

func (m *ContentModel) node(n *cmNode) *cmNode {
	n.id = len(m.nodes)
	m.nodes = append(m.nodes, n)
	return n
}

func (m *ContentModel) leaf(p *Particle, subst map[*ElementDecl][]*ElementDecl) *cmNode {
	n := &cmNode{min: p.MinOccurs, max: p.MaxOccurs, particle: p, pos: len(m.leaves)}
	switch t := p.Term.(type) {
	case *ElementDecl:
		n.kind = cmElement
		n.decl = t
		if t.Scope.Global && subst != nil {
			n.members = subst[t]
		} else {
			n.members = []*ElementDecl{t}
		}
		if len(n.members) > 8 {
			n.byName = make(map[QName]*ElementDecl, len(n.members))
			for _, e := range n.members {
				if _, dup := n.byName[e.Name]; !dup {
					n.byName[e.Name] = e
				}
			}
		}
	case *Wildcard:
		n.kind = cmWildcard
		n.wild = t
	default:
		return nil
	}
	m.leaves = append(m.leaves, n)
	n.first = []int{n.pos}
	return m.node(n)
}

func (m *ContentModel) build(p *Particle, parent *cmNode, subst map[*ElementDecl][]*ElementDecl) *cmNode {
	var n *cmNode
	if g, ok := p.Term.(*ModelGroup); ok {
		n = m.node(&cmNode{min: p.MinOccurs, max: p.MaxOccurs, particle: p, kind: cmSequence})
		if g.Compositor == CompositorChoice {
			n.kind = cmChoice
		}
		// Grupa all poza korzeniem jest błędem schematu (cos-all-limited); traktujemy ją jak sekwencję, żeby
		// automat dało się zbudować, a błąd zgłosiła już kontrola z etapu 3.
		for _, k := range g.Particles {
			if c := m.build(k, n, subst); c != nil {
				n.kids = append(n.kids, c)
			}
		}
	} else if n = m.leaf(p, subst); n == nil {
		return nil
	}
	n.parent = parent
	switch {
	case n.max == Unbounded && n.min <= 1:
	case n.max == 1 && n.min <= 1:
	default:
		n.counted = true
		n.cap = n.max
		if n.max == Unbounded {
			n.cap = n.min
		}
	}
	return n
}

// computeSets liczy nullable, first, last i indeksy pozycji od liści w górę.
func (m *ContentModel) computeSets(n *cmNode) {
	for _, k := range n.kids {
		m.computeSets(k)
	}
	var last []int
	switch n.kind {
	case cmElement, cmWildcard:
		last = n.first
	case cmSequence:
		n.bodyNullable = true
		for _, k := range n.kids {
			if n.bodyNullable {
				n.first = append(n.first, k.first...)
			}
			if !k.nullable {
				n.bodyNullable = false
			}
		}
		for i := len(n.kids) - 1; i >= 0; i-- {
			k := n.kids[i]
			last = append(last, m.positions(k.lastSet)...)
			if !k.nullable {
				break
			}
		}
	case cmChoice:
		for _, k := range n.kids {
			n.first = append(n.first, k.first...)
			last = append(last, m.positions(k.lastSet)...)
			if k.nullable {
				n.bodyNullable = true
			}
		}
	}
	n.nullable = n.min == 0 || n.bodyNullable
	n.lastSet = newBitset(len(m.leaves))
	for _, p := range last {
		n.lastSet.set(p)
	}
	n.idx = m.index(n.first)
}

func (m *ContentModel) positions(b bitset) []int {
	var out []int
	for i := range m.leaves {
		if b.has(i) {
			out = append(out, i)
		}
	}
	return out
}

func (m *ContentModel) index(first []int) *posIdx {
	ix := &posIdx{byName: map[QName]int{}, byNameAll: map[QName][]int{}, wildSet: map[string][]int{}, all: first}
	for _, p := range first {
		lf := m.leaves[p]
		if lf.kind == cmWildcard {
			if lf.wild.Namespaces.Kind == NamespaceSet {
				for _, s := range lf.wild.Namespaces.Set {
					ix.wildSet[s] = append(ix.wildSet[s], p)
				}
			} else {
				ix.wildOther = append(ix.wildOther, p)
			}
			continue
		}
		for _, e := range lf.members {
			if _, ok := ix.byName[e.Name]; !ok {
				ix.byName[e.Name] = p
				ix.small = append(ix.small, namePos{e.Name, p})
			}
			ix.byNameAll[e.Name] = append(ix.byNameAll[e.Name], p)
		}
	}
	if len(ix.small) <= smallIndex {
		ix.useSmall = true
	} else {
		ix.small = nil
	}
	return ix
}

// lookup zwraca pozycję z indeksu dla elementu name (najpierw deklaracje, potem symbole wieloznaczne) albo -1.
func (ix *posIdx) lookup(name QName, m *ContentModel) int {
	if ix.useSmall {
		for i := range ix.small {
			if ix.small[i].name.Local == name.Local && ix.small[i].name.Space == name.Space {
				return ix.small[i].pos
			}
		}
	} else if p, ok := ix.byName[name]; ok {
		return p
	}
	return ix.lookupWild(name, m)
}

// lookupWild zwraca pozycję symbolu wieloznacznego z indeksu, który dopuszcza element name, albo -1.
func (ix *posIdx) lookupWild(name QName, m *ContentModel) int {
	if ps := ix.wildSet[name.Space]; len(ps) > 0 {
		return ps[0]
	}
	for _, p := range ix.wildOther {
		if m.leaves[p].wild.Namespaces.Allows(name.Space) {
			return p
		}
	}
	return -1
}

func (m *ContentModel) counterIndexOf(pos int, n *cmNode) int {
	for i, c := range m.counters[pos] {
		if c == n {
			return i
		}
	}
	return -1
}

func isAncestorOrSelf(a, n *cmNode) bool {
	for ; n != nil; n = n.parent {
		if n == a {
			return true
		}
	}
	return false
}

// edges buduje krawędzie z pozycji src.
func (m *ContentModel) edges(src *cmNode) []cmEdge {
	p := src.pos
	path := m.counters[p]
	keepFor := func(j *cmNode) int {
		k := 0
		for _, c := range path {
			if isAncestorOrSelf(c, j) {
				k++
			}
		}
		return k
	}
	var out []cmEdge
	var exits []int
	for cur := src; cur != nil; cur = cur.parent {
		if !cur.lastSet.has(p) {
			break
		}
		// Powtórzenie cur: kolejna iteracja od pierwszych pozycji jego ciała.
		if cur.repeatable() && len(cur.first) > 0 {
			e := cmEdge{target: cur, join: cur, iter: cur, incr: -1, keep: keepFor(cur), exits: append([]int(nil), exits...)}
			if cur.counted {
				e.incr = m.counterIndexOf(p, cur)
			}
			out = append(out, e)
		}
		// Wyjście z cur: licznik cur musi pozwolić na zakończenie.
		if cur.counted {
			exits = append(exits, m.counterIndexOf(p, cur))
		}
		if par := cur.parent; par != nil && par.kind == cmSequence {
			if i := indexOf(par.kids, cur); i+1 < len(par.kids) {
				out = append(out, cmEdge{seq: par, from: i + 1, join: par, incr: -1, keep: keepFor(par),
					exits: append([]int(nil), exits...), lazy: &edgeIndex{}})
			}
		}
	}
	return out
}

func indexOf(list []*cmNode, n *cmNode) int {
	for i, k := range list {
		if k == n {
			return i
		}
	}
	return -1
}

// targets wywołuje f dla indeksów pozycji, do których prowadzi krawędź.
func (e *cmEdge) indexes(f func(*posIdx) bool) {
	if e.target != nil {
		f(e.target.idx)
		return
	}
	for _, k := range e.seq.kids[e.from:] {
		if !f(k.idx) || !k.nullable {
			return
		}
	}
}

// ---- Unique Particle Attribution (cos-nonambig) i niejednoznaczność liczników

// guardBox zwraca przedział wartości każdego licznika źródła dopuszczony przez warunki krawędzi e.
func (m *ContentModel) guardBox(src int, e *cmEdge) (lo, hi []int64) {
	if src < 0 {
		return nil, nil
	}
	path := m.counters[src]
	lo = make([]int64, len(path))
	hi = make([]int64, len(path))
	for i, c := range path {
		lo[i], hi[i] = 1, c.cap
	}
	for _, i := range e.exits {
		c := path[i]
		if !c.bodyNullable && c.min > lo[i] {
			lo[i] = c.min
		}
	}
	if e.incr >= 0 && e.iter.max != Unbounded && e.iter.max-1 < hi[e.incr] {
		hi[e.incr] = e.iter.max - 1
	}
	return lo, hi
}

func boxesMeet(alo, ahi, blo, bhi []int64) bool {
	for i := range alo {
		if max(alo[i], blo[i]) > min(ahi[i], bhi[i]) {
			return false
		}
	}
	return true
}

func boxEmpty(lo, hi []int64) bool {
	for i := range lo {
		if lo[i] > hi[i] {
			return true
		}
	}
	return false
}

// upaConflict opisuje naruszenie Unique Particle Attribution.
type upaConflict struct {
	a, b *cmNode
	name string
}

// upaItem to cel krawędzi: pozycja i krawędź, którą się do niej dochodzi.
type upaItem struct {
	pos  int
	edge int
}

// checkUPA sprawdza Unique Particle Attribution (3.8.6) z uwzględnieniem liczników, tak jak opisuje to dodatek H:
// liczności są rozwinięte, a nie pominięte, więc np. sekwencja (a{2,2}, a) jest jednoznaczna. Dwa przejścia z tego
// samego źródła do różnych pozycji o wspólnej etykiecie są w konflikcie, gdy ich warunki da się spełnić tą samą
// konfiguracją. Gdy model ma niejednoznaczność liczników (do tej samej pozycji prowadzą dwie krawędzie o różnym
// działaniu na liczniki), konfiguracji bywa kilka i wystarczy, że każdy warunek osobno jest spełnialny.
func (m *ContentModel) checkUPA() *upaConflict {
	if m.all != nil {
		for i, a := range m.all.leaves {
			for _, b := range m.all.leaves[i+1:] {
				if name, ok := labelsOverlap(a, b); ok {
					return &upaConflict{a, b, name}
				}
			}
		}
		return nil
	}
	if m.root == nil {
		return nil
	}
	m.ambiguous = false
	for pass := 0; pass < 2; pass++ {
		// Pierwszy przebieg szuka niejednoznaczności liczników, drugi konfliktów (z jej uwzględnieniem).
		done := map[string]bool{}
		check := func(src int, edges []cmEdge) *upaConflict {
			key := m.sourceKey(src, edges)
			if done[key] {
				return nil
			}
			done[key] = true
			return m.checkSource(src, edges, pass == 0)
		}
		if c := check(-1, m.start); c != nil {
			return c
		}
		for i := range m.from {
			if c := check(i, m.from[i]); c != nil {
				return c
			}
		}
	}
	return nil
}

// sourceKey opisuje źródło przez jego krawędzie i liczniki: źródła o tym samym opisie mają te same konflikty.
func (m *ContentModel) sourceKey(src int, edges []cmEdge) string {
	var b strings.Builder
	if src >= 0 {
		for _, c := range m.counters[src] {
			b.WriteString(strconv.Itoa(c.id))
			b.WriteByte(',')
		}
	}
	for _, e := range edges {
		b.WriteByte('|')
		if e.target != nil {
			b.WriteString("t" + strconv.Itoa(e.target.id))
		} else {
			b.WriteString("s" + strconv.Itoa(e.seq.id) + "." + strconv.Itoa(e.from))
		}
		if e.iter != nil {
			b.WriteString("i" + strconv.Itoa(e.iter.id))
		}
		b.WriteString(":" + strconv.Itoa(e.keep) + ":" + strconv.Itoa(e.incr) + ":")
		for _, x := range e.exits {
			b.WriteString(strconv.Itoa(x) + ",")
		}
	}
	return b.String()
}

// checkSource sprawdza jedno źródło. W trybie ambiguityOnly szuka tylko niejednoznaczności liczników.
func (m *ContentModel) checkSource(src int, edges []cmEdge, ambiguityOnly bool) *upaConflict {
	boxLo := make([][]int64, len(edges))
	boxHi := make([][]int64, len(edges))
	for i := range edges {
		boxLo[i], boxHi[i] = m.guardBox(src, &edges[i])
	}
	conflict := func(a, b upaItem) bool {
		if src < 0 {
			return true
		}
		if m.ambiguous {
			return !boxEmpty(boxLo[a.edge], boxHi[a.edge]) && !boxEmpty(boxLo[b.edge], boxHi[b.edge])
		}
		return boxesMeet(boxLo[a.edge], boxHi[a.edge], boxLo[b.edge], boxHi[b.edge])
	}
	names := map[QName][]upaItem{}
	var nameOrder []QName
	var wilds []upaItem
	reached := map[int]int{} // pozycja → pierwsza krawędź (niejednoznaczność liczników)
	for ei := range edges {
		e := &edges[ei]
		e.indexes(func(ix *posIdx) bool {
			for _, p := range ix.all {
				if ambiguityOnly {
					if prev, ok := reached[p]; ok {
						pe := &edges[prev]
						if (pe.keep != e.keep || pe.incr != e.incr) && (len(m.counters[p]) > 0 || (src >= 0 && len(m.counters[src]) > 0)) &&
							src >= 0 && boxesMeet(boxLo[prev], boxHi[prev], boxLo[ei], boxHi[ei]) {
							m.ambiguous = true
						}
					} else {
						reached[p] = ei
					}
					continue
				}
				it := upaItem{p, ei}
				lf := m.leaves[p]
				if lf.kind == cmWildcard {
					wilds = append(wilds, it)
					continue
				}
				for _, mem := range lf.members {
					if _, ok := names[mem.Name]; !ok {
						nameOrder = append(nameOrder, mem.Name)
					}
					names[mem.Name] = append(names[mem.Name], it)
				}
			}
			return true
		})
	}
	if ambiguityOnly {
		return nil
	}
	// Symbole wieloznaczne z listą przestrzeni w kubełkach według przestrzeni; ##any i ##other osobno (zwykle mało).
	wildByNS := map[string][]upaItem{}
	var wildOther []upaItem
	for _, w := range wilds {
		nc := m.leaves[w.pos].wild.Namespaces
		if nc.Kind != NamespaceSet {
			wildOther = append(wildOther, w)
			continue
		}
		for _, s := range nc.Set {
			wildByNS[s] = append(wildByNS[s], w)
		}
	}
	for _, q := range nameOrder {
		list := names[q]
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				if list[i].pos != list[j].pos && conflict(list[i], list[j]) {
					return &upaConflict{m.leaves[list[i].pos], m.leaves[list[j].pos], q.String()}
				}
			}
		}
		for _, group := range [][]upaItem{wildByNS[q.Space], wildOther} {
			for _, w := range group {
				if !m.leaves[w.pos].wild.Namespaces.Allows(q.Space) {
					continue
				}
				for _, it := range list {
					if conflict(it, w) {
						return &upaConflict{m.leaves[it.pos], m.leaves[w.pos], q.String()}
					}
				}
			}
		}
	}
	pair := func(a, b upaItem) *upaConflict {
		if a.pos != b.pos && wildcardsMeet(m.leaves[a.pos].wild.Namespaces, m.leaves[b.pos].wild.Namespaces) && conflict(a, b) {
			return &upaConflict{m.leaves[a.pos], m.leaves[b.pos], "*"}
		}
		return nil
	}
	// Dwa symbole z listami przestrzeni spotykają się tylko we wspólnej przestrzeni; ##any i ##other z każdym.
	nss := make([]string, 0, len(wildByNS))
	for s := range wildByNS {
		nss = append(nss, s)
	}
	sort.Strings(nss)
	for _, s := range nss {
		list := wildByNS[s]
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				if c := pair(list[i], list[j]); c != nil {
					return c
				}
			}
		}
	}
	for _, a := range wildOther {
		for _, b := range wilds {
			if b == a {
				continue
			}
			if c := pair(a, b); c != nil {
				return c
			}
		}
	}
	return nil
}

// labelsOverlap mówi, czy jakiś element może pasować do obu pozycji; zwraca przykładową nazwę.
func labelsOverlap(a, b *cmNode) (string, bool) {
	switch {
	case a.kind == cmElement && b.kind == cmElement:
		for _, x := range a.members {
			for _, y := range b.members {
				if x.Name == y.Name {
					return x.Name.String(), true
				}
			}
		}
	case a.kind == cmElement && b.kind == cmWildcard:
		for _, x := range a.members {
			if b.wild.Namespaces.Allows(x.Name.Space) {
				return x.Name.String(), true
			}
		}
	case a.kind == cmWildcard && b.kind == cmElement:
		return labelsOverlap(b, a)
	default:
		if wildcardsMeet(a.wild.Namespaces, b.wild.Namespaces) {
			return "*", true
		}
	}
	return "", false
}

// wildcardsMeet mówi, czy przecięcie dwóch ograniczeń przestrzeni nazw jest niepuste.
func wildcardsMeet(a, b NamespaceConstraint) bool {
	switch {
	case a.Kind == NamespaceAny || b.Kind == NamespaceAny:
		return true
	case a.Kind == NamespaceNot && b.Kind == NamespaceNot:
		return true // not(x) ∩ not(y) zawiera wszystkie inne przestrzenie
	case a.Kind == NamespaceSet && b.Kind == NamespaceSet:
		for _, s := range a.Set {
			if b.has(s) {
				return true
			}
		}
		return false
	case a.Kind == NamespaceNot:
		a, b = b, a
	}
	for _, s := range a.Set { // a: zbiór, b: not
		if b.Allows(s) {
			return true
		}
	}
	return false
}

func (c *upaConflict) describe() string {
	return fmt.Sprintf("two particles can match the same element %s (%s and %s)", c.name, particleLabel(c.a), particleLabel(c.b))
}

func particleLabel(n *cmNode) string {
	pos := n.particle.Pos
	what := "wildcard"
	if n.kind == cmElement {
		what = "element " + n.decl.Name.String()
	}
	if pos.Line > 0 {
		return fmt.Sprintf("%s at line %d", what, pos.Line)
	}
	return what
}

// ---- dopasowanie (dla walidacji dokumentu, etap 5)

// Match to wynik dopasowania dziecka: cząstka i deklaracja elementu (także członek grupy podstawienia) albo
// symbol wieloznaczny.
type Match struct {
	Particle *Particle
	Element  *ElementDecl
	Wildcard *Wildcard
}

// Expectation to jedna z dopuszczalnych następnych pozycji: element albo symbol wieloznaczny.
type Expectation struct {
	Element  *ElementDecl
	Wildcard *Wildcard
}

type config struct {
	pos  int
	vals []int64
}

// Matcher dopasowuje kolejne dzieci jednego elementu do modelu zawartości. Nie jest bezpieczny dla wielu
// goroutines; ContentModel jest (tylko odczyt).
type Matcher struct {
	m       *ContentModel
	started bool
	configs []config
	next    []config
	used    []bool // grupa all
	any     bool
	// Overflow: przekroczono MaxConfigurations (Step zwraca wtedy false). Dziecka nie należy wtedy uznawać za
	// błędne według rekomendacji, tylko zgłosić przekroczenie limitu.
	Overflow bool
}

// NewMatcher zaczyna dopasowanie dzieci jednego elementu.
func (m *ContentModel) NewMatcher() *Matcher {
	mt := &Matcher{m: m}
	if m.all != nil {
		mt.used = make([]bool, len(m.all.leaves))
	}
	return mt
}

// Step dopasowuje następne dziecko o nazwie name. ok == false: dziecko nie pasuje (cvc-complex-type.2.4); stan
// dopasowania się wtedy nie zmienia.
func (mt *Matcher) Step(name QName) (Match, bool) {
	m := mt.m
	if m.all != nil {
		i, ok := m.all.byName[name]
		if !ok || mt.used[i] {
			return Match{}, false
		}
		mt.used[i] = true
		mt.any = true
		lf := m.all.leaves[i]
		return Match{Particle: lf.particle, Element: memberNamed(lf, name)}, true
	}
	if m.root == nil {
		return Match{}, false
	}
	mt.next = mt.next[:0]
	target := -1
	try := func(src int, vals []int64, edges []cmEdge) {
		for ei := range edges {
			e := &edges[ei]
			if src >= 0 && !m.guardHolds(src, vals, e) {
				continue
			}
			p := e.lookup(name, m)
			if p < 0 || (target >= 0 && p != target) {
				continue // drugi cel byłby naruszeniem UPA; schemat poprawny go nie ma
			}
			target = p
			mt.push(m.apply(vals, e, p))
		}
	}
	if !mt.started {
		try(-1, nil, m.start)
	} else {
		for _, c := range mt.configs {
			try(c.pos, c.vals, m.from[c.pos])
		}
	}
	if target < 0 {
		return Match{}, false
	}
	if len(mt.next) > MaxConfigurations {
		mt.Overflow = true
		return Match{}, false
	}
	mt.started = true
	mt.configs, mt.next = mt.next, mt.configs
	lf := m.leaves[target]
	if lf.kind == cmWildcard {
		return Match{Particle: lf.particle, Wildcard: lf.wild}, true
	}
	return Match{Particle: lf.particle, Element: memberNamed(lf, name)}, true
}

func memberNamed(lf *cmNode, name QName) *ElementDecl {
	if lf.byName != nil {
		return lf.byName[name]
	}
	for _, e := range lf.members {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func (m *ContentModel) guardHolds(src int, vals []int64, e *cmEdge) bool {
	path := m.counters[src]
	for _, i := range e.exits {
		c := path[i]
		if !c.bodyNullable && vals[i] < c.min {
			return false
		}
	}
	return e.incr < 0 || e.iter.max == Unbounded || vals[e.incr] < e.iter.max
}

func (m *ContentModel) apply(vals []int64, e *cmEdge, to int) config {
	path := m.counters[to]
	nv := make([]int64, len(path))
	copy(nv, vals[:min(e.keep, len(vals))])
	for i := e.keep; i < len(path); i++ {
		nv[i] = 1
	}
	if e.incr >= 0 {
		if c := path[e.incr]; nv[e.incr] < c.cap {
			nv[e.incr]++
		}
	}
	return config{pos: to, vals: nv}
}

func (mt *Matcher) push(c config) {
	for _, o := range mt.next {
		if o.pos == c.pos && equalVals(o.vals, c.vals) {
			return
		}
	}
	mt.next = append(mt.next, c)
}

func equalVals(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Accepting mówi, czy dotychczasowe dzieci tworzą kompletną treść.
func (mt *Matcher) Accepting() bool {
	m := mt.m
	if m.all != nil {
		if !mt.any {
			return m.nullable
		}
		for i, lf := range m.all.leaves {
			if lf.min > 0 && !mt.used[i] {
				return false
			}
		}
		return true
	}
	if !mt.started {
		return m.nullable
	}
	for _, c := range mt.configs {
		if m.canEnd(c) {
			return true
		}
	}
	return false
}

func (m *ContentModel) canEnd(c config) bool {
	if !m.root.lastSet.has(c.pos) {
		return false
	}
	for i, n := range m.counters[c.pos] {
		if !n.bodyNullable && c.vals[i] < n.min {
			return false
		}
	}
	return true
}

// Expected zwraca pozycje, które mogą przyjąć następne dziecko, w powtarzalnej kolejności (do komunikatów).
func (mt *Matcher) Expected() []Expectation {
	m := mt.m
	var out []Expectation
	seen := map[int]bool{}
	addLeaf := func(lf *cmNode) {
		if seen[lf.pos] {
			return
		}
		seen[lf.pos] = true
		if lf.kind == cmWildcard {
			out = append(out, Expectation{Wildcard: lf.wild})
			return
		}
		for _, e := range lf.members {
			out = append(out, Expectation{Element: e})
		}
	}
	if m.all != nil {
		for i, lf := range m.all.leaves {
			if !mt.used[i] {
				addLeaf(lf)
			}
		}
		return out
	}
	if m.root == nil {
		return nil
	}
	var cand []int
	collect := func(src int, vals []int64, edges []cmEdge) {
		for ei := range edges {
			e := &edges[ei]
			if src >= 0 && !m.guardHolds(src, vals, e) {
				continue
			}
			e.indexes(func(ix *posIdx) bool {
				cand = append(cand, ix.all...)
				return true
			})
		}
	}
	if !mt.started {
		collect(-1, nil, m.start)
	} else {
		for _, c := range mt.configs {
			collect(c.pos, c.vals, m.from[c.pos])
		}
	}
	sort.Ints(cand)
	for _, p := range cand {
		addLeaf(m.leaves[p])
	}
	return out
}

// Nullable mówi, czy model dopuszcza brak dzieci.
func (m *ContentModel) Nullable() bool { return m.nullable }
