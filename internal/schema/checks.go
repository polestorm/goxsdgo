package schema

import (
	"strings"

	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// finalChecks sprawdza ograniczenia komponentów, które wymagają zbudowanego schematu, w tym ograniczenia modeli
// zawartości (etap 4, ADR 0007): Unique Particle Attribution (cos-nonambig), Element Declarations Consistent
// (cos-element-consistent) i Particle Valid (Restriction) (cos-particle-restrict, w derivation-ok-restriction.5.4.2
// i src-redefine.6.2.2).
func (l *loader) finalChecks() {
	l.subst = substitutionGroups(l.sch.ElementDecls)
	l.rs = newRestrictor(l.subst)
	for _, a := range l.attrDecls {
		l.checkAttributeDecl(a)
	}
	for _, u := range l.attrUses {
		l.checkAttributeUse(u)
	}
	for _, ct := range l.sch.ComplexTypes {
		l.checkComplexType(ct)
		l.checkContentModel(ct)
	}
	for _, df := range l.groupRedefs {
		l.checkGroupRedefine(df)
	}
	for _, nt := range l.notationTypes {
		for _, lex := range nt.st.DT.Enumeration() {
			q, ok := l.qname(nt.n, lex)
			if !ok || l.index[kindNotation][q] == nil {
				l.errorf(CodeResolve, nt.n.pos(), "enumeration value %q of a NOTATION type is not a declared notation", lex)
			}
		}
	}
}

func isID(t Type) bool {
	idt := datatypes.Builtin("ID")
	switch t := t.(type) {
	case *SimpleType:
		return t.DT.IsDerivedFrom(idt)
	case *ComplexType:
		return t.Content == ContentSimple && t.SimpleContent != nil && t.SimpleContent.DT.IsDerivedFrom(idt)
	}
	return false
}

func (l *loader) checkAttributeDecl(a *AttributeDecl) {
	if a.Name.Local == "xmlns" {
		l.errorf(CodeNoXMLNS, a.Pos, "an attribute cannot be named xmlns")
	}
	if a.Name.Space == NamespaceXSI {
		l.errorf(CodeNoXSI, a.Pos, "attributes in the XML Schema instance namespace cannot be declared")
	}
	if a.Type.DT.NeedsNotationEnumeration() {
		l.errorf(datatypes.CodeNotationEnumeration, a.Pos, "an attribute cannot use NOTATION directly; derive a type with enumeration")
	}
	if a.Value != nil {
		if err := valueOf(a.Value, a.Type); err != nil {
			l.errorf(CodeAttributeProps+".2", a.Pos, "the %s value %q is not valid for the attribute type: %v", vcKind(a.Value), a.Value.Lexical, err)
		}
		if isID(a.Type) {
			l.errorf(CodeAttributeProps+".3", a.Pos, "an attribute of type ID cannot have a default or fixed value")
		}
	}
}

func vcKind(vc *ValueConstraint) string {
	if vc.Fixed {
		return "fixed"
	}
	return "default"
}

func (l *loader) checkAttributeUse(u *AttributeUse) {
	if u.Value == nil {
		return
	}
	if err := valueOf(u.Value, u.Decl.Type); err != nil {
		l.errorf(CodeAttributeProps+".2", u.Pos, "the %s value %q is not valid for the attribute type: %v", vcKind(u.Value), u.Value.Lexical, err)
		return
	}
	if isID(u.Decl.Type) {
		l.errorf(CodeAttributeProps+".3", u.Pos, "an attribute of type ID cannot have a default or fixed value")
	}
	if d := u.Decl.Value; d != nil && d.Fixed {
		if d.Value == nil {
			_ = valueOf(d, u.Decl.Type)
		}
		if !u.Value.Fixed || d.Value == nil || !datatypes.Identical(u.Value.Value, d.Value) {
			l.errorf(CodeAttributeUseProps+".2", u.Pos, "the declaration of %s fixes its value to %q; the use must fix the same value", u.Decl.Name, d.Lexical)
		}
	}
}

// checkElement sprawdza Element Declaration Properties Correct (3.3.6) i NOTATION.
func (l *loader) checkElement(n *node, e *ElementDecl) {
	if st, ok := e.Type.(*SimpleType); ok && st.DT.NeedsNotationEnumeration() {
		l.errorf(datatypes.CodeNotationEnumeration, e.Pos, "an element cannot use NOTATION directly; derive a type with enumeration")
	}
	if e.Value != nil {
		switch t := e.Type.(type) {
		case *SimpleType:
			if err := valueOf(e.Value, t); err != nil {
				l.errorf(CodeElementProps+".2", e.Pos, "the %s value %q is not valid for the element type: %v", vcKind(e.Value), e.Value.Lexical, err)
			}
		case *ComplexType:
			switch t.Content {
			case ContentSimple:
				if err := valueOf(e.Value, t.SimpleContent); err != nil {
					l.errorf(CodeElementProps+".2", e.Pos, "the %s value %q is not valid for the element content: %v", vcKind(e.Value), e.Value.Lexical, err)
				}
			case ContentMixed:
				if !emptiable(t.Particle) {
					l.errorf(CodeValidDefault+".2.2.2", e.Pos, "a %s value needs mixed content with an emptiable particle", vcKind(e.Value))
				}
			default:
				l.errorf(CodeValidDefault+".2.1", e.Pos, "a %s value needs simple or mixed content", vcKind(e.Value))
			}
		}
		if isID(e.Type) {
			l.errorf(CodeElementProps+".5", e.Pos, "an element of type ID cannot have a default or fixed value")
		}
	}
	if h := e.SubstitutionGroup; h != nil && h.Type != nil && e.Type != nil {
		if !derivedOK(e.Type, h.Type, h.SubstitutionExclusions) {
			l.errorf(CodeElementProps+".4", e.Pos, "the type of %s is not validly derived from the type of its substitution group head %s",
				e.Name, h.Name)
		}
	}
}

// checkAttributeGroup sprawdza Attribute Group Definition Properties Correct (3.6.6) i dla redefinicji bez
// odwołania do siebie punkt 7.2.2 z Redefinition Constraints and Semantics.
func (l *loader) checkAttributeGroup(df *def) {
	ag := df.comp.(*AttributeGroup)
	seen := map[QName]bool{}
	ids := 0
	for _, u := range ag.Uses {
		if seen[u.Decl.Name] {
			l.errorf(CodeAttributeGroupProps+".2", ag.Pos, "attribute %s is declared twice in attribute group %s", u.Decl.Name, ag.Name)
		}
		seen[u.Decl.Name] = true
		if isID(u.Decl.Type) {
			ids++
		}
	}
	if ids > 1 {
		l.errorf(CodeAttributeGroupProps+".3", ag.Pos, "attribute group %s has more than one attribute of type ID", ag.Name)
	}
	if df.orig != nil && len(selfRefs(df.node, "attributeGroup", df.name, l)) == 0 {
		orig := df.comp.(*AttributeGroup)
		if o, ok := df.orig.comp.(*AttributeGroup); ok {
			orig = o
		}
		l.checkAttributeRestriction(ag.Uses, ag.Wildcard, orig.Uses, orig.Wildcard, false, df.node.pos(),
			redefineAttrCodes)
	}
}

// checkComplexType sprawdza Complex Type Definition Properties Correct, Derivation Valid (Extension) i Derivation
// Valid (Restriction, Complex) (3.4.6) poza częściami, które wymagają restrykcji cząstek (etap 4).
func (l *loader) checkComplexType(ct *ComplexType) {
	if ct == anyType {
		return
	}
	seen := map[QName]bool{}
	ids := 0
	for _, u := range ct.Attributes {
		if seen[u.Decl.Name] {
			l.errorf(CodeComplexTypeProps+".4", ct.Pos, "attribute %s is declared twice in %s", u.Decl.Name, typeLabel(ct))
		}
		seen[u.Decl.Name] = true
		if isID(u.Decl.Type) {
			ids++
		}
	}
	if ids > 1 {
		l.errorf(CodeComplexTypeProps+".5", ct.Pos, "%s has more than one attribute of type ID", typeLabel(ct))
	}
	switch base := ct.Base.(type) {
	case *SimpleType:
		if ct.Derivation != DerivationExtension {
			l.errorf(CodeComplexTypeProps+".2", ct.Pos, "a complex type with a simple base must be an extension")
		}
		if base.Final.Has(DerivationExtension) {
			l.errorf(CodeExtends+".2.2", ct.Pos, "the base type %s has extension in its final", typeLabel(base))
		}
	case *ComplexType:
		if ct.Derivation == DerivationExtension {
			l.checkExtension(ct, base)
		} else {
			l.checkRestriction(ct, base)
		}
	}
}

func (l *loader) checkExtension(ct, base *ComplexType) {
	if base.Final.Has(DerivationExtension) {
		l.errorf(CodeExtends+".1.1", ct.Pos, "the base type %s has extension in its final", typeLabel(base))
	}
	if bw := base.AttributeWildcard; bw != nil {
		if ct.AttributeWildcard == nil || !wildcardSubset(bw.Namespaces, ct.AttributeWildcard.Namespaces) {
			l.errorf(CodeExtends+".1.3", ct.Pos, "the attribute wildcard of %s must include that of its base", typeLabel(ct))
		}
	}
	if base.Content == ContentSimple && ct.Content == ContentSimple && base.SimpleContent != ct.SimpleContent {
		l.errorf(CodeExtends+".1.4.1", ct.Pos, "an extension must keep the simple content type of its base")
	}
	// Punkt 1.5 (wyprowadzenie w dwóch krokach) i pełna kontrola cząstek: etap 4.
}

func (l *loader) checkRestriction(ct, base *ComplexType) {
	if base.Final.Has(DerivationRestriction) {
		l.errorf(CodeRestriction+".1", ct.Pos, "the base type %s has restriction in its final", typeLabel(base))
	}
	l.checkAttributeRestriction(ct.Attributes, ct.AttributeWildcard, base.Attributes, base.AttributeWildcard,
		base == anyType, ct.Pos, restrictionAttrCodes)
	if base == anyType {
		return // 5.1
	}
	switch ct.Content {
	case ContentSimple:
		switch {
		case base.Content == ContentSimple:
			if !derivedOK(ct.SimpleContent, base.SimpleContent, 0) {
				l.errorf(CodeRestriction+".5.2.2.1", ct.Pos, "the simple content of %s is not derived from that of its base", typeLabel(ct))
			}
		case base.Content == ContentMixed && emptiable(base.Particle):
		default:
			l.errorf(CodeRestriction+".5.2", ct.Pos, "simple content cannot restrict the content of %s", typeLabel(base))
		}
	case ContentEmpty:
		if base.Content != ContentEmpty && !((base.Content == ContentElementOnly || base.Content == ContentMixed) && emptiable(base.Particle)) {
			l.errorf(CodeRestriction+".5.3", ct.Pos, "empty content cannot restrict the content of %s", typeLabel(base))
		}
	case ContentElementOnly, ContentMixed:
		if base.Content == ContentEmpty || base.Content == ContentSimple {
			l.errorf(CodeRestriction+".5.4", ct.Pos, "element content cannot restrict the %s content of %s", contentName(base.Content), typeLabel(base))
		} else if ct.Content == ContentMixed && base.Content != ContentMixed {
			l.errorf(CodeRestriction+".5.4.1.2", ct.Pos, "mixed content cannot restrict element-only content")
		}
		// 5.4.2: cząstka musi być poprawną restrykcją cząstki bazowej (cos-particle-restrict).
		if ct.Particle != nil && base.Particle != nil {
			l.rs.why = ""
			if !l.rs.validRestriction(ct.Particle, base.Particle) {
				l.errorf(CodeRestriction+".5.4.2", ct.Pos, "the content of %s is not a valid restriction of the content of %s: %s",
					typeLabel(ct), typeLabel(base), l.rs.why)
			}
		}
	}
}

func contentName(k ContentKind) string {
	return [...]string{"empty", "simple", "element-only", "mixed"}[k]
}

// attrRestrictionCodes to kody punktów 2–4 Derivation Valid (Restriction, Complex) (3.4.6) dla użyć atrybutów
// i symbolu wieloznacznego. Te same punkty sprawdza redefinicja grupy atrybutów (src-redefine.7.2.2); jej kody to
// src-redefine.7.2.2 z dopisanym punktem derivation-ok-restriction.
type attrRestrictionCodes struct {
	notAllowed, required, typeDerived, fixed, missing, wildNeeded, wildSubset, wildProcess string
}

var restrictionAttrCodes = attrRestrictionCodes{
	notAllowed:  CodeRestriction + ".2.2",
	required:    CodeRestriction + ".2.1.1",
	typeDerived: CodeRestriction + ".2.1.2",
	fixed:       CodeRestriction + ".2.1.3",
	missing:     CodeRestriction + ".3",
	wildNeeded:  CodeRestriction + ".4.1",
	wildSubset:  CodeRestriction + ".4.2",
	wildProcess: CodeRestriction + ".4.3",
}

var redefineAttrCodes = attrRestrictionCodes{
	notAllowed:  CodeRedefine + ".7.2.2.2.2",
	required:    CodeRedefine + ".7.2.2.2.1.1",
	typeDerived: CodeRedefine + ".7.2.2.2.1.2",
	fixed:       CodeRedefine + ".7.2.2.2.1.3",
	missing:     CodeRedefine + ".7.2.2.3",
	wildNeeded:  CodeRedefine + ".7.2.2.4.1",
	wildSubset:  CodeRedefine + ".7.2.2.4.2",
	wildProcess: CodeRedefine + ".7.2.2.4.3",
}

// checkAttributeRestriction sprawdza punkty 2, 3 i 4 z Derivation Valid (Restriction, Complex) dla użyć atrybutów
// i symbolu wieloznacznego (także dla redefinicji grupy atrybutów).
func (l *loader) checkAttributeRestriction(uses []*AttributeUse, wild *Wildcard, baseUses []*AttributeUse,
	baseWild *Wildcard, baseIsAnyType bool, pos Position, code attrRestrictionCodes) {
	byName := map[QName]*AttributeUse{}
	for _, b := range baseUses {
		byName[b.Decl.Name] = b
	}
	mine := map[QName]*AttributeUse{}
	for _, r := range uses {
		mine[r.Decl.Name] = r
		b := byName[r.Decl.Name]
		if b == nil {
			if baseWild == nil || !baseWild.Namespaces.Allows(r.Decl.Name.Space) {
				l.errorf(code.notAllowed, r.Pos, "attribute %s is not allowed by the base type", r.Decl.Name)
			}
			continue
		}
		if b == r {
			continue
		}
		if b.Required && !r.Required {
			l.errorf(code.required, r.Pos, "attribute %s is required in the base and must stay required", r.Decl.Name)
		}
		if !derivedOK(r.Decl.Type, b.Decl.Type, 0) {
			l.errorf(code.typeDerived, r.Pos, "the type of attribute %s is not derived from its type in the base", r.Decl.Name)
		}
		if bv := b.EffectiveValue(); bv != nil && bv.Fixed {
			rv := r.EffectiveValue()
			if rv == nil || !rv.Fixed || !sameValue(rv, bv, r.Decl.Type, b.Decl.Type) {
				l.errorf(code.fixed, r.Pos, "attribute %s is fixed to %q in the base and must keep that value", r.Decl.Name, bv.Lexical)
			}
		}
	}
	for _, b := range baseUses {
		if b.Required && (mine[b.Decl.Name] == nil || !mine[b.Decl.Name].Required) {
			l.errorf(code.missing, pos, "required attribute %s of the base is missing or optional", b.Decl.Name)
		}
	}
	if wild != nil {
		switch {
		case baseWild == nil:
			l.errorf(code.wildNeeded, wild.Pos, "an attribute wildcard needs one in the base")
		case !wildcardSubset(wild.Namespaces, baseWild.Namespaces):
			l.errorf(code.wildSubset, wild.Pos, "the attribute wildcard is not a subset of the base wildcard")
		case !baseIsAnyType && wild.Process < baseWild.Process:
			// ProcessStrict < ProcessLax < ProcessSkip: mniejsza wartość to silniejsza.
		case !baseIsAnyType && wild.Process > baseWild.Process:
			l.errorf(code.wildProcess, wild.Pos, "the attribute wildcard must process contents at least as strictly as the base")
		}
	}
}

func sameValue(a, b *ValueConstraint, at, bt *SimpleType) bool {
	if a.Value == nil {
		_ = valueOf(a, at)
	}
	if b.Value == nil {
		_ = valueOf(b, bt)
	}
	if a.Value != nil && b.Value != nil {
		return datatypes.Identical(a.Value, b.Value)
	}
	return strings.TrimSpace(a.Lexical) == strings.TrimSpace(b.Lexical)
}

// ---- Type Derivation OK (Simple) i (Complex), 3.14.6 i 3.4.6

func derivedOK(d, b Type, subset DerivationSet) bool {
	switch d := d.(type) {
	case *SimpleType:
		return derivedOKSimple(d, b, subset)
	case *ComplexType:
		return derivedOKComplex(d, b, subset)
	}
	return false
}

func derivedOKSimple(d *SimpleType, b Type, subset DerivationSet) bool {
	initBuiltins()
	if Type(d) == b {
		return true
	}
	if d == anySimple {
		return b == Type(anyType)
	}
	if subset.Has(DerivationRestriction) {
		return false
	}
	if bs, ok := d.Base.(*SimpleType); ok && bs.Final.Has(DerivationRestriction) {
		return false
	}
	if d.Base == b {
		return true
	}
	if bs, ok := d.Base.(*SimpleType); ok && d.Base != Type(anyType) && derivedOKSimple(bs, b, subset) {
		return true
	}
	if v := d.DT.Variety(); (v == datatypes.List || v == datatypes.Union) && b == Type(anySimple) {
		return true
	}
	if bs, ok := b.(*SimpleType); ok && bs.DT.Variety() == datatypes.Union {
		for _, m := range bs.Members {
			if derivedOKSimple(d, m, subset) {
				return true
			}
		}
	}
	return false
}

func derivedOKComplex(d *ComplexType, b Type, subset DerivationSet) bool {
	initBuiltins()
	if Type(d) == b {
		return true
	}
	if subset&d.Derivation != 0 {
		return false
	}
	if d.Base == b {
		return true
	}
	if d.Base == Type(anyType) {
		return false
	}
	return derivedOK(d.Base, b, subset)
}
