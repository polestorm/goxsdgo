package schema

import (
	"errors"
	"strings"

	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// finalOf zwraca {final} z atrybutu final albo finalDefault dokumentu, zawężone do mask.
func finalOf(n *node, mask DerivationSet) DerivationSet {
	if v, ok := n.attr("final"); ok {
		return parseDerivationSet(v) & mask
	}
	return n.doc.finalDefault & mask
}

// blockOf zwraca {prohibited substitutions} albo {disallowed substitutions} z atrybutu block albo blockDefault.
func blockOf(n *node, mask DerivationSet) DerivationSet {
	if v, ok := n.attr("block"); ok {
		return parseDerivationSet(v) & mask
	}
	return n.doc.blockDefault & mask
}

const simpleFinalMask = DerivationExtension | DerivationRestriction | DerivationList | DerivationUnion

// buildSimpleType wypełnia st z elementu xs:simpleType n (3.14.2). self to def redefiniowanego typu albo nil.
func (l *loader) buildSimpleType(n *node, st *SimpleType) {
	initBuiltins()
	st.Pos = n.pos()
	if n.parent != nil && (n.parent.is("schema") || n.parent.is("redefine")) {
		st.Final = finalOf(n, simpleFinalMask)
	}
	// Wartości zastępcze na wypadek błędu: schemat i tak jest błędny, ale komponent musi być spójny.
	st.DT, st.Base = anySimple.DT, anySimple
	defer func() { l.simpleTypes = append(l.simpleTypes, st) }()
	var k *node
	for _, c := range n.xsdKids() {
		k = c
		break
	}
	if k == nil {
		return // brak restriction/list/union zgłosiła kontrola według schematu dla schematów
	}
	switch k.local {
	case "restriction":
		l.buildSimpleRestriction(k, st)
	case "list":
		l.buildList(k, st)
	case "union":
		l.buildUnion(k, st)
	}
}

func (l *loader) buildSimpleRestriction(k *node, st *SimpleType) {
	inner := k.kid("simpleType")
	baseAttr, hasBase := k.attr("base")
	if hasBase == (inner != nil) {
		l.errorf(CodeSimpleTypeRep+".2", k.pos(), "xs:restriction must have either a base attribute or a simpleType child, not both or neither")
	}
	var base *SimpleType
	switch {
	case inner != nil && !hasBase:
		base = &SimpleType{}
		l.buildSimpleType(inner, base)
	case hasBase:
		switch t := l.ensuredType(k, "base", baseAttr, true, CodeSimpleTypeProps+".2").(type) {
		case *SimpleType:
			base = t
		case *ComplexType:
			l.errorf(CodeSimpleTypeProps+".1", k.attrPos("base"), "the base of a simple type definition must be a simple type, %s is complex", t.Name)
		}
	}
	if base == nil {
		return
	}
	if base == anySimple {
		// Derivation Valid (Restriction, Simple) 1.1: typ atomowy przez restrykcję wymaga atomowego typu bazowego
		// albo wbudowanego typu pierwotnego; anySimpleType nim nie jest.
		l.errorf(CodeSimpleRestricts+".1.1", k.attrPos("base"), "anySimpleType cannot be the base of a restriction")
		return
	}
	if base.Final.Has(DerivationRestriction) {
		l.errorf(CodeSimpleTypeProps+".3", k.attrPos("base"), "the base type %s has restriction in its final", typeLabel(base))
	}
	dt, err := datatypes.Restrict(base.DT, facetSpecs(k), &datatypes.Context{Namespaces: k, Names: k.doc.names})
	if err != nil {
		l.datatypeError(err, k)
		return
	}
	st.DT, st.Base, st.Item, st.Members = dt, base, base.Item, base.Members
	if dt.Variety() == datatypes.Atomic && dt.Primitive() == datatypes.KindNOTATION {
		l.notationTypes = append(l.notationTypes, notationType{st: st, n: k})
	}
}

func (l *loader) buildList(k *node, st *SimpleType) {
	inner := k.kid("simpleType")
	itemAttr, hasItem := k.attr("itemType")
	if hasItem == (inner != nil) {
		l.errorf(CodeSimpleTypeRep+".3", k.pos(), "xs:list must have either an itemType attribute or a simpleType child, not both or neither")
	}
	var item *SimpleType
	switch {
	case inner != nil && !hasItem:
		item = &SimpleType{}
		l.buildSimpleType(inner, item)
	case hasItem:
		switch t := l.ensuredType(k, "itemType", itemAttr, false, CodeSimpleTypeProps+".2").(type) {
		case *SimpleType:
			item = t
		case *ComplexType:
			l.errorf(CodeSimpleTypeProps+".1", k.attrPos("itemType"), "the item type of a list must be a simple type, %s is complex", t.Name)
		}
	}
	if item == nil {
		return
	}
	if item.Final.Has(DerivationList) {
		l.errorf(CodeSimpleRestricts+".2.3.1.1", k.pos(), "the item type %s has list in its final", typeLabel(item))
	}
	if item.DT.Variety() == datatypes.List {
		l.errorf(CodeSimpleRestricts+".2.1", k.pos(), "the item type of a list must be atomic or a union, %s is a list", typeLabel(item))
		return
	}
	for _, m := range item.Members {
		if m.DT.Variety() == datatypes.List {
			l.errorf(CodeSimpleRestricts+".2.1", k.pos(), "the item type of a list is a union with a list member")
			return
		}
	}
	dt, err := datatypes.NewList(item.DT)
	if err != nil {
		l.datatypeError(err, k)
		return
	}
	st.DT, st.Base, st.Item = dt, anySimple, item
}

func (l *loader) buildUnion(k *node, st *SimpleType) {
	var members []*SimpleType
	if v, ok := k.attr("memberTypes"); ok {
		for _, f := range strings.Fields(v) {
			switch t := l.ensuredType(k, "memberTypes", f, false, CodeSimpleTypeRep+".4").(type) {
			case *SimpleType:
				members = append(members, t)
			case *ComplexType:
				l.errorf(CodeSimpleTypeProps+".1", k.attrPos("memberTypes"), "member types of a union must be simple, %s is complex", t.Name)
			}
		}
	}
	for _, c := range k.kids {
		if c.is("simpleType") {
			m := &SimpleType{}
			l.buildSimpleType(c, m)
			members = append(members, m)
		}
	}
	if len(members) == 0 {
		l.errorf(CodeSimpleTypeRep, k.pos(), "xs:union must have at least one member type")
		return
	}
	// Spłaszczenie: unie wśród składowych zastępują ich składowe (3.14.2, {member type definitions} 1).
	var flat []*SimpleType
	var dts []*datatypes.Type
	for _, m := range members {
		if m.Final.Has(DerivationUnion) {
			l.errorf(CodeSimpleRestricts+".3.3.1.1", k.pos(), "the member type %s has union in its final", typeLabel(m))
		}
		if m.DT.Variety() == datatypes.Union {
			flat = append(flat, m.Members...)
			for _, x := range m.Members {
				dts = append(dts, x.DT)
			}
			continue
		}
		flat = append(flat, m)
		dts = append(dts, m.DT)
	}
	dt, err := datatypes.NewUnion(dts...)
	if err != nil {
		l.datatypeError(err, k)
		return
	}
	st.DT, st.Base, st.Members = dt, anySimple, flat
}

// facetSpecs zbiera aspekty z dzieci elementu xs:restriction.
func facetSpecs(k *node) []datatypes.FacetSpec {
	var specs []datatypes.FacetSpec
	for _, c := range k.kids {
		if c.space != NamespaceXSD {
			continue
		}
		kind, ok := datatypes.FacetByName(c.local)
		if !ok {
			continue
		}
		v, _ := c.attr("value")
		fixed := false
		if f, ok := c.attr("fixed"); ok {
			f = collapse(f)
			fixed = f == "true" || f == "1"
		}
		specs = append(specs, datatypes.FacetSpec{Kind: kind, Value: v, Fixed: fixed})
	}
	return specs
}

// datatypeError zamienia błąd definicji typu z pakietu datatypes (albo regex) na błąd schematu; wskazuje
// element aspektu, którego dotyczy, gdy go znajdzie.
func (l *loader) datatypeError(err error, k *node) {
	pos := k.pos()
	code := CodeSimpleTypeProps
	var de *datatypes.Error
	if errors.As(err, &de) {
		code = de.Code
		for _, c := range k.kids {
			if c.space == NamespaceXSD && c.local == de.Facet {
				pos = c.pos()
				break
			}
		}
	}
	l.errs = append(l.errs, &Error{Code: code, Pos: pos, Message: err.Error(), Cause: err})
}

func typeLabel(t Type) string {
	if q := t.TypeName(); q.Local != "" {
		return q.String()
	}
	return "(anonymous)"
}
