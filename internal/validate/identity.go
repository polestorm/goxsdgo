package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxsdgo/internal/datatypes"
	"github.com/polestorm/goxsdgo/internal/schema"
)

// Ograniczenia tożsamości (3.11.4 Identity-constraint Satisfied, 3.11.5 Identity-constraint Table) strumieniowo.
//
// Element z deklaracją, która ma ograniczenia, otwiera zakres (scope) każdego z nich. Selector jest sprawdzany na
// każdym elemencie wewnątrz zakresu na podstawie nazw elementów od elementu zakresu w dół (podzbiór XPath z 3.11.6
// ma tylko osie child, attribute i descendant-or-self na początku), więc nie trzeba drzewa. Wybrany element (cel)
// czeka na wartości pól z elementów i atrybutów pod nim; po jego końcu ciąg kluczowy trafia do tabeli zakresu.
// Na końcu elementu zakresu sprawdzane są keyref, a tabele kluczy przechodzą do rodzica (tabela węzłów, 3.11.5)
// tylko dla ograniczeń, do których odwołuje się jakiś keyref.

type icScope struct {
	ic    *schema.IdentityConstraint
	fr    *frame
	depth int
	// table: ciąg kluczowy → węzeł (unique, key); refs: ciągi keyref do sprawdzenia na końcu zakresu.
	table map[string]int
	refs  []keyrefEntry
}

type keyrefEntry struct {
	key          string
	line, column int
	path         string
	display      string
}

type icTarget struct {
	scope        *icScope
	node         int
	depth        int
	line, column int
	// fr to ramka celu; jest na stosie do końca celu, więc ścieżkę liczymy dopiero przy błędzie.
	fr     *frame
	fields []fieldValue
}

type fieldValue struct {
	count int
	set   bool
	value datatypes.Value
	nil   bool
	bad   bool // pole wskazuje węzeł bez typu prostego (cvc-identity-constraint.3)
}

type fieldRef struct {
	t *icTarget
	i int
}

// icFrame to dane ograniczeń tożsamości elementu.
type icFrame struct {
	scopes []*icScope
	fields []fieldRef
	// bubble: tabele węzłów z potomków (3.11.5) dla ograniczeń, do których odwołuje się keyref; -1 oznacza
	// ciąg kluczowy w konflikcie.
	bubble map[*schema.IdentityConstraint]map[string]int
}

type identityState struct {
	scopes  []*icScope
	targets []*icTarget
	// referenced: ograniczenia wskazane przez jakiś keyref (liczone przy pierwszym zakresie).
	referenced map[*schema.IdentityConstraint]bool
}

func (v *validator) referenced(ic *schema.IdentityConstraint) bool {
	if v.ics.referenced == nil {
		v.ics.referenced = map[*schema.IdentityConstraint]bool{}
		for _, c := range v.sch.IdentityConstraints {
			if c.Refer != nil {
				v.ics.referenced[c.Refer] = true
			}
		}
		for _, d := range v.sch.ElementDecls {
			for _, c := range d.IdentityConstraints {
				if c.Refer != nil {
					v.ics.referenced[c.Refer] = true
				}
			}
		}
	}
	return v.ics.referenced[ic]
}

func (v *validator) identityStart(fr *frame, tok *xmlreader.Token) {
	depth := len(v.stack) - 1
	if fr.decl != nil && fr.mode != modeSkip && len(fr.decl.IdentityConstraints) > 0 {
		fr.ic = &icFrame{}
		for _, ic := range fr.decl.IdentityConstraints {
			s := &icScope{ic: ic, fr: fr, depth: depth}
			if ic.Category != schema.ICKeyRef {
				s.table = map[string]int{}
			}
			fr.ic.scopes = append(fr.ic.scopes, s)
			v.ics.scopes = append(v.ics.scopes, s)
		}
	}
	if len(v.ics.scopes) == 0 {
		return
	}
	// Selektory: czy bieżący element jest celem któregoś zakresu.
	for _, s := range v.ics.scopes {
		if matchesAny(s.ic.Selector, v.stack[s.depth+1:depth+1], false) {
			t := &icTarget{scope: s, node: fr.node, depth: depth, line: fr.line, column: fr.col, fr: fr,
				fields: make([]fieldValue, len(s.ic.Fields))}
			v.ics.targets = append(v.ics.targets, t)
		}
	}
	// Pola: elementy i atrybuty pod otwartymi celami.
	for _, t := range v.ics.targets {
		names := v.stack[t.depth+1 : depth+1]
		for i, f := range t.scope.ic.Fields {
			// Węzeł liczy się raz, nawet gdy pasuje do kilku alternatyw ścieżki pola (np. "@id|@id").
			elemCounted := false
			attrCounted := map[schema.QName]bool{}
			for _, p := range f.Paths {
				if !pathMatches(p, names) {
					continue
				}
				if p.Attribute == nil {
					if elemCounted {
						continue
					}
					elemCounted = true
					if fr.ic == nil {
						fr.ic = &icFrame{}
					}
					fr.ic.fields = append(fr.ic.fields, fieldRef{t, i})
					t.fields[i].count++
					continue
				}
				for _, a := range v.attrs {
					if !nameMatches(*p.Attribute, a.name) || attrCounted[a.name] {
						continue
					}
					attrCounted[a.name] = true
					fv := &t.fields[i]
					fv.count++
					switch {
					case fr.mode == modeSkip || !a.simple:
						fv.bad = true
					case a.ok:
						fv.set, fv.value = true, a.value
					}
				}
			}
		}
	}
}

func matchesAny(x *schema.XPath, names []*frame, _ bool) bool {
	if x == nil {
		return false
	}
	for _, p := range x.Paths {
		if p.Attribute == nil && pathMatches(p, names) {
			return true
		}
	}
	return false
}

// pathMatches mówi, czy ścieżka p (bez końcowego testu atrybutu) prowadzi od węzła kontekstu do ostatniego
// elementu names; names to elementy od dziecka węzła kontekstu do sprawdzanego elementu (puste: sam węzeł).
func pathMatches(p schema.XPathPath, names []*frame) bool {
	steps := make([]schema.NameTest, 0, len(p.Steps))
	for _, s := range p.Steps {
		if !s.Self {
			steps = append(steps, s.Name)
		}
	}
	if p.Descendant {
		if len(names) < len(steps) {
			return false
		}
		names = names[len(names)-len(steps):]
	} else if len(names) != len(steps) {
		return false
	}
	for i, s := range steps {
		if !nameMatches(s, names[i].name) {
			return false
		}
	}
	return true
}

func nameMatches(t schema.NameTest, q schema.QName) bool {
	switch {
	case t.AnyName:
		return true
	case t.AnyLocal:
		return t.Space == q.Space
	}
	return t.Space == q.Space && t.Local == q.Local
}

func (v *validator) identityEnd(fr *frame) {
	if fr.ic != nil {
		for _, f := range fr.ic.fields {
			fv := &f.t.fields[f.i]
			switch {
			case fr.mode == modeSkip || fr.st == nil:
				fv.bad = true
			case fr.nilled:
				fv.nil = true
			case fr.valueOK:
				fv.set, fv.value = true, fr.value
			}
		}
	}
	if len(v.ics.targets) > 0 {
		kept := v.ics.targets[:0]
		for _, t := range v.ics.targets {
			if t.node == fr.node {
				v.closeTarget(t)
			} else {
				kept = append(kept, t)
			}
		}
		v.ics.targets = kept
	}
	if fr.ic != nil && len(fr.ic.scopes) > 0 {
		v.closeScopes(fr)
		v.ics.scopes = v.ics.scopes[:len(v.ics.scopes)-len(fr.ic.scopes)]
	}
	// Tabele węzłów idą do rodzica (3.11.5).
	if fr.ic != nil && len(fr.ic.bubble) > 0 && len(v.stack) >= 2 {
		parent := v.stack[len(v.stack)-2]
		if parent.ic == nil {
			parent.ic = &icFrame{}
		}
		if parent.ic.bubble == nil {
			parent.ic.bubble = map[*schema.IdentityConstraint]map[string]int{}
		}
		for ic, tab := range fr.ic.bubble {
			dst := parent.ic.bubble[ic]
			if dst == nil {
				parent.ic.bubble[ic] = tab
				continue
			}
			for k, n := range tab {
				if old, ok := dst[k]; ok && old != n {
					dst[k] = -1
				} else if !ok {
					dst[k] = n
					v.grow(1)
				}
			}
		}
	}
}

// closeTarget liczy ciąg kluczowy celu (3.11.4, punkty 3 i 4).
func (v *validator) closeTarget(t *icTarget) {
	ic := t.scope.ic
	var key strings.Builder
	var display []string
	qualified := true
	for i := range t.fields {
		fv := &t.fields[i]
		switch {
		case fv.count > 1:
			v.report(Error{Code: CodeICField, Line: t.line, Column: t.column, Path: v.pathOf(t.fr),
				Message: fmt.Sprintf("field %q of %s selects more than one node", ic.Fields[i].Source, ic.Name)})
			return
		case fv.bad:
			v.report(Error{Code: CodeICField, Line: t.line, Column: t.column, Path: v.pathOf(t.fr),
				Message: fmt.Sprintf("field %q of %s selects a node without a simple type", ic.Fields[i].Source, ic.Name)})
			return
		case fv.nil:
			if ic.Category == schema.ICKey {
				v.report(Error{Code: CodeICKeyNil, Line: t.line, Column: t.column, Path: v.pathOf(t.fr),
					Message: fmt.Sprintf("field %q of key %s selects a nilled element", ic.Fields[i].Source, ic.Name)})
			}
			qualified = false
		case !fv.set:
			qualified = false
		default:
			key.WriteString(valueKey(fv.value))
			key.WriteByte(0x1f)
			display = append(display, datatypes.Canonical(fv.value))
		}
	}
	if !qualified {
		if ic.Category == schema.ICKey {
			v.report(Error{Code: CodeICKeyMiss, Line: t.line, Column: t.column, Path: v.pathOf(t.fr),
				Message: fmt.Sprintf("key %s: a field of the selected element is missing", ic.Name)})
		}
		return
	}
	k := key.String()
	switch ic.Category {
	case schema.ICKeyRef:
		t.scope.refs = append(t.scope.refs, keyrefEntry{key: k, line: t.line, column: t.column, path: v.pathOf(t.fr),
			display: strings.Join(display, ", ")})
		v.grow(1)
	default:
		if _, dup := t.scope.table[k]; dup {
			code := CodeICUnique
			if ic.Category == schema.ICKey {
				code = CodeICKeyDup
			}
			v.report(Error{Code: code, Line: t.line, Column: t.column, Path: v.pathOf(t.fr), Value: strings.Join(display, ", "),
				Message: fmt.Sprintf("duplicate value (%s) for %s", strings.Join(display, ", "), ic.Name)})
			return
		}
		t.scope.table[k] = t.node
		v.grow(1)
	}
}

// valueKey to klucz porównania w przestrzeni wartości: wartości równe dają ten sam napis (rodzaj pierwotny
// i postać kanoniczna).
func valueKey(x datatypes.Value) string {
	if l, ok := x.(datatypes.ListValue); ok {
		parts := make([]string, len(l))
		for i, it := range l {
			parts[i] = valueKey(it)
		}
		return "L(" + strings.Join(parts, "\x1e") + ")"
	}
	return strconv.Itoa(int(x.Kind())) + ":" + datatypes.Canonical(x)
}

// closeScopes kończy zakresy ograniczeń elementu fr: sprawdza keyref i przekazuje tabele węzłów.
func (v *validator) closeScopes(fr *frame) {
	// Tabela węzłów elementu (3.11.5): własne tabele unique/key mają pierwszeństwo przed tabelami potomków.
	tables := map[*schema.IdentityConstraint]map[string]int{}
	for ic, tab := range fr.ic.bubble {
		tables[ic] = tab
	}
	for _, s := range fr.ic.scopes {
		if s.table == nil {
			continue
		}
		if !v.referenced(s.ic) {
			continue
		}
		merged := map[string]int{}
		for k, n := range tables[s.ic] {
			if n >= 0 {
				merged[k] = n
			}
		}
		for k, n := range s.table {
			merged[k] = n
		}
		tables[s.ic] = merged
	}
	for _, s := range fr.ic.scopes {
		if s.ic.Category != schema.ICKeyRef || s.ic.Refer == nil {
			continue
		}
		tab := tables[s.ic.Refer]
		for _, r := range s.refs {
			if n, ok := tab[r.key]; !ok || n < 0 {
				v.report(Error{Code: CodeICKeyRef, Line: r.line, Column: r.column, Path: r.path, Value: r.display,
					Message: fmt.Sprintf("keyref %s: no %s matches the value (%s)", s.ic.Name, s.ic.Refer.Name, r.display)})
			}
		}
	}
	if len(tables) > 0 {
		fr.ic.bubble = tables
	}
}
