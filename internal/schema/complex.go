package schema

import (
	"github.com/polestorm/goxsdgo/internal/datatypes"
)

const complexMask = DerivationExtension | DerivationRestriction

func boolAttr(n *node, name string) (bool, bool) {
	v, ok := n.attr(name)
	if !ok {
		return false, false
	}
	v = collapse(v)
	return v == "true" || v == "1", true
}

// buildComplexType wypełnia ct z elementu xs:complexType n (3.4.2).
func (l *loader) buildComplexType(n *node, ct *ComplexType) {
	initBuiltins()
	ct.Pos = n.pos()
	l.sch.ComplexTypes = append(l.sch.ComplexTypes, ct)
	ct.Abstract, _ = boolAttr(n, "abstract")
	if n.parent != nil && (n.parent.is("schema") || n.parent.is("redefine")) {
		ct.Final = finalOf(n, complexMask)
		ct.Block = blockOf(n, complexMask)
	}
	// Wartości zastępcze na wypadek błędu.
	ct.Base, ct.Derivation = anyType, DerivationRestriction
	mixedType, _ := boolAttr(n, "mixed")

	sc, cc := n.kid("simpleContent"), n.kid("complexContent")
	switch {
	case sc != nil:
		l.buildSimpleContent(sc, ct)
	case cc != nil:
		var der *node
		for _, k := range cc.xsdKids() {
			der = k
			break
		}
		if der == nil {
			return
		}
		mixed := mixedType
		if v, ok := boolAttr(cc, "mixed"); ok {
			mixed = v
		}
		base := l.complexBase(der, ct)
		baseCT, isCT := base.(*ComplexType)
		if base != nil && !isCT {
			l.errorf(CodeComplexTypeRep+".1", der.attrPos("base"), "the base of complex content must be a complex type, %s is simple", typeLabel(base))
		}
		if !isCT {
			baseCT = anyType
		}
		ct.Base = baseCT
		if der.local == "extension" {
			ct.Derivation = DerivationExtension
		}
		l.buildComplexContent(der, ct, baseCT, mixed)
	default:
		ct.Base, ct.Derivation = anyType, DerivationRestriction
		l.buildComplexContent(n, ct, anyType, mixedType)
	}
}

// complexBase rozwiązuje atrybut base elementu restriction albo extension i buduje typ bazowy.
func (l *loader) complexBase(der *node, ct *ComplexType) Type {
	v, ok := der.attr("base")
	if !ok {
		return nil
	}
	return l.ensuredType(der, "base", v, true, CodeComplexTypeProps+".3")
}

// buildSimpleContent obsługuje wariant xs:simpleContent (3.4.2, Complex Type Definition with simple content).
func (l *loader) buildSimpleContent(sc *node, ct *ComplexType) {
	var der *node
	for _, k := range sc.xsdKids() {
		der = k
		break
	}
	ct.Content = ContentSimple
	ct.SimpleContent = anySimple
	if der == nil {
		return
	}
	base := l.complexBase(der, ct)
	if base == nil {
		l.buildAttributes(der, ct, nil, der.local == "restriction")
		return
	}
	ct.Base = base
	baseCT, isCT := base.(*ComplexType)
	if der.local == "extension" {
		ct.Derivation = DerivationExtension
		switch {
		case isCT && baseCT.Content == ContentSimple:
			ct.SimpleContent = baseCT.SimpleContent
		case isCT:
			l.errorf(CodeComplexTypeRep+".2.1", der.attrPos("base"), "simple content extension needs a base with simple content or a simple type; %s has complex content", typeLabel(base))
		default:
			ct.SimpleContent = base.(*SimpleType)
		}
		l.buildAttributes(der, ct, baseCT, false)
		return
	}
	ct.Derivation = DerivationRestriction
	inner := der.kid("simpleType")
	var start *SimpleType
	switch {
	case !isCT:
		l.errorf(CodeComplexTypeRep+".2.1", der.attrPos("base"), "simple content restriction needs a complex base type, %s is simple", typeLabel(base))
	case baseCT.Content == ContentSimple:
		start = baseCT.SimpleContent
		if inner != nil {
			start = &SimpleType{}
			l.buildSimpleType(inner, start)
		}
	case baseCT.Content == ContentMixed && emptiable(baseCT.Particle):
		if inner == nil {
			l.errorf(CodeComplexTypeRep+".2.2", der.pos(), "restricting mixed content to simple content requires a simpleType child")
		} else {
			start = &SimpleType{}
			l.buildSimpleType(inner, start)
		}
	default:
		l.errorf(CodeComplexTypeRep+".2.1", der.attrPos("base"), "simple content restriction needs a base with simple content or emptiable mixed content")
	}
	if start != nil {
		ct.SimpleContent = start
		if specs := facetSpecs(der); len(specs) > 0 {
			if start == anySimple {
				l.errorf(CodeSimpleRestricts+".1.1", der.pos(), "anySimpleType cannot be the base of a restriction")
			} else if dt, err := datatypes.Restrict(start.DT, specs, &datatypes.Context{Namespaces: der, Names: der.doc.names}); err != nil {
				l.datatypeError(err, der)
			} else {
				st := &SimpleType{DT: dt, Base: start, Item: start.Item, Members: start.Members, Pos: der.pos()}
				l.simpleTypes = append(l.simpleTypes, st)
				ct.SimpleContent = st
			}
		}
	}
	if isCT {
		l.buildAttributes(der, ct, baseCT, true)
	} else {
		l.buildAttributes(der, ct, nil, true)
	}
}

// buildComplexContent obsługuje wariant xs:complexContent i skrót bez niego (3.4.2, Complex Type Definition
// with complex content). der to element restriction albo extension (albo sam complexType dla skrótu).
func (l *loader) buildComplexContent(der *node, ct *ComplexType, base *ComplexType, mixed bool) {
	var pn *node
	for _, k := range der.xsdKids() {
		switch k.local {
		case "group", "all", "choice", "sequence":
			pn = k
		}
		if pn != nil {
			break
		}
	}
	// ·effective content· (punkt 2): pusta albo cząstka.
	var eff *Particle
	empty := pn == nil
	if pn != nil {
		switch pn.local {
		case "all", "sequence":
			empty = len(pn.xsdKids()) == 0
		case "choice":
			if len(pn.xsdKids()) == 0 {
				if mn, ok := pn.attr("minOccurs"); ok && isZero(mn) {
					empty = true
				}
			}
		}
		if !empty {
			eff = l.buildParticle(pn, ct, contentTop)
			if eff == nil {
				empty = true // minOccurs = maxOccurs = 0: cząstka nie odpowiada żadnemu komponentowi
			}
		}
	}
	if empty && mixed {
		eff = &Particle{MinOccurs: 1, MaxOccurs: 1, Term: &ModelGroup{Compositor: CompositorSequence, Pos: der.pos()}, Pos: der.pos()}
		empty = false
	}
	kind := ContentElementOnly
	if mixed {
		kind = ContentMixed
	}
	isExtension := der.local == "extension"
	switch {
	case !isExtension:
		if empty {
			ct.Content = ContentEmpty
		} else {
			ct.Content, ct.Particle = kind, eff
		}
	case empty:
		ct.Content, ct.SimpleContent, ct.Particle = base.Content, base.SimpleContent, base.Particle
	case base.Content == ContentEmpty:
		ct.Content, ct.Particle = kind, eff
	case base.Content == ContentSimple:
		l.errorf(CodeExtends+".1.4", der.pos(), "complex content cannot extend %s, which has simple content", typeLabel(base))
		ct.Content, ct.Particle = kind, eff
	default:
		ct.Content = kind
		ct.Particle = &Particle{MinOccurs: 1, MaxOccurs: 1, Pos: der.pos(), Term: &ModelGroup{Compositor: CompositorSequence,
			Particles: []*Particle{base.Particle, eff}, Pos: der.pos()}}
		if isAll(base.Particle) || isAll(eff) {
			l.errorf(CodeAllLimited+".1.2", der.pos(), "an all group cannot be part of a sequence created by extension")
		}
		if base.Content != kind {
			l.errorf(CodeExtends+".1.4.3.2.2.1", der.pos(), "extension must keep the content of %s mixed or element-only, as it is", typeLabel(base))
		}
	}
	// Skrót bez complexContent (der to sam complexType) jest restrykcją anyType.
	l.buildAttributes(der, ct, base, !isExtension)
}

func isZero(v string) bool {
	v = collapse(v)
	for _, c := range v {
		if c != '0' && c != '+' {
			return false
		}
	}
	return v != ""
}

func isAll(p *Particle) bool {
	if p == nil {
		return false
	}
	g, ok := p.Term.(*ModelGroup)
	return ok && g.Compositor == CompositorAll
}

// buildAttributes wypełnia {attribute uses} i {attribute wildcard} typu z dzieci der (3.4.2). base to typ bazowy
// złożony (nil dla typu prostego), restriction mówi, czy to restrykcja.
func (l *loader) buildAttributes(der *node, ct *ComplexType, base *ComplexType, restriction bool) {
	uses, prohibited, local, groupWilds := l.attributeUses(der, Scope{Parent: ct})
	complete := l.completeWildcard(der, local, groupWilds)
	own := map[QName]bool{}
	for _, u := range uses {
		own[u.Decl.Name] = true
	}
	ct.Attributes = uses
	if base != nil {
		for _, bu := range base.Attributes {
			if restriction && (own[bu.Decl.Name] || prohibited[bu.Decl.Name]) {
				continue
			}
			ct.Attributes = append(ct.Attributes, bu)
		}
	}
	if restriction {
		ct.AttributeWildcard = complete
		return
	}
	var bw *Wildcard
	if base != nil {
		bw = base.AttributeWildcard
	}
	switch {
	case bw == nil:
		ct.AttributeWildcard = complete
	case complete == nil:
		ct.AttributeWildcard = bw
	default:
		u, ok := wildcardUnion(complete.Namespaces, bw.Namespaces)
		if !ok {
			l.errorf(CodeWildcardUnion, der.pos(), "the union of the attribute wildcards is not expressible")
		}
		ct.AttributeWildcard = &Wildcard{Namespaces: u, Process: complete.Process, Pos: complete.Pos}
	}
}

// attributeUses buduje użycia atrybutów z dzieci attribute i attributeGroup elementu der. Zwraca użycia, nazwy
// atrybutów z use="prohibited", lokalny symbol wieloznaczny i symbole wieloznaczne grup atrybutów.
func (l *loader) attributeUses(der *node, scope Scope) ([]*AttributeUse, map[QName]bool, *Wildcard, []*Wildcard) {
	var uses []*AttributeUse
	prohibited := map[QName]bool{}
	var local *Wildcard
	var groupWilds []*Wildcard
	for _, k := range der.kids {
		switch {
		case k.is("attribute"):
			u, name, isProhibited := l.buildAttributeUse(k, scope)
			if isProhibited {
				if name.Local != "" {
					prohibited[name] = true
				}
				continue
			}
			if u != nil {
				uses = append(uses, u)
			}
		case k.is("attributeGroup"):
			v, ok := k.attr("ref")
			if !ok {
				continue
			}
			df := l.resolve(k, "ref", v, kindAttributeGroup)
			if df == nil {
				continue
			}
			if df.state == stateBuilding {
				l.errorf(CodeAttributeGroupRep+".3", k.attrPos("ref"), "circular attribute group reference to %s", df.name)
				continue
			}
			l.ensure(df)
			ag := df.comp.(*AttributeGroup)
			uses = append(uses, ag.Uses...)
			if ag.Wildcard != nil {
				groupWilds = append(groupWilds, ag.Wildcard)
			}
		case k.is("anyAttribute"):
			local = l.buildWildcard(k)
		}
	}
	return uses, prohibited, local, groupWilds
}

// completeWildcard liczy ·complete wildcard· (3.4.2, {attribute wildcard} punkt 2).
func (l *loader) completeWildcard(der *node, local *Wildcard, groupWilds []*Wildcard) *Wildcard {
	if len(groupWilds) == 0 {
		return local
	}
	var ns NamespaceConstraint
	var process ProcessContents
	start := 0
	if local != nil {
		ns, process = local.Namespaces, local.Process
	} else {
		ns, process = groupWilds[0].Namespaces, groupWilds[0].Process
		start = 1
	}
	for _, g := range groupWilds[start:] {
		x, ok := wildcardIntersect(ns, g.Namespaces)
		if !ok {
			code := CodeComplexTypeRep + ".4"
			if der.is("attributeGroup") {
				code = CodeAttributeGroupRep + ".2"
			}
			l.errorf(code, der.pos(), "the intersection of the attribute wildcards is not expressible")
			return &Wildcard{Namespaces: ns, Process: process, Pos: der.pos()}
		}
		ns = x
	}
	pos := der.pos()
	if local != nil {
		pos = local.Pos
	}
	return &Wildcard{Namespaces: ns, Process: process, Pos: pos}
}

// buildAttributeGroup wypełnia definicję grupy atrybutów (3.6.2).
func (l *loader) buildAttributeGroup(df *def) {
	ag := df.comp.(*AttributeGroup)
	n := df.node
	uses, _, local, groupWilds := l.attributeUses(n, Scope{})
	ag.Uses = uses
	ag.Wildcard = l.completeWildcard(n, local, groupWilds)
	l.checks = append(l.checks, func() { l.checkAttributeGroup(df) })
}
