package schema

import (
	"github.com/polestorm/goxsdgo/internal/datatypes"
)

const (
	elementBlockMask = DerivationExtension | DerivationRestriction | DerivationSubstitution
	elementFinalMask = DerivationExtension | DerivationRestriction
)

// ensure buduje komponent globalny df, jeśli jeszcze nie jest zbudowany. Wywołujący, którzy mogą trafić na cykl,
// sprawdzają stan stateBuilding przed wywołaniem.
func (l *loader) ensure(df *def) {
	if df.state != stateNew {
		return
	}
	df.state = stateBuilding
	switch df.kind {
	case kindType:
		switch c := df.comp.(type) {
		case *SimpleType:
			l.buildSimpleType(df.node, c)
		case *ComplexType:
			l.buildComplexType(df.node, c)
		}
	case kindElement:
		l.buildGlobalElement(df)
	case kindAttribute:
		l.buildGlobalAttribute(df)
	case kindAttributeGroup:
		l.buildAttributeGroup(df)
	case kindGroup:
		l.buildGroupDef(df)
	case kindNotation:
		nt := df.comp.(*Notation)
		pub, hasPublic := df.node.attr("public")
		nt.Public = collapse(pub)
		sys, hasSystem := df.node.attr("system")
		nt.System = collapse(sys)
		if !hasPublic && !hasSystem {
			// Erratum E1-16 uczyniło public opcjonalnym; deklaracja notacji musi jednak identyfikować notację,
			// więc wymagamy co najmniej jednego z public i system (Notation Declaration Correct, ADR 0006).
			l.errorf(CodeNotationProps, df.node.pos(), "notation %s needs a public or a system identifier", nt.Name)
		}
	case kindIdentity:
		l.buildIdentity(df)
	}
	df.state = stateBuilt
}

// build buduje wszystkie komponenty i uruchamia kontrole końcowe.
func (l *loader) build() {
	initBuiltins()
	var all []*def
	seen := map[*def]bool{}
	var add func(df *def)
	add = func(df *def) {
		if df == nil || seen[df] {
			return
		}
		seen[df] = true
		if df.orig != nil {
			add(df.orig)
		}
		all = append(all, df)
	}
	for _, d := range l.docs {
		var walk func(n *node)
		walk = func(n *node) {
			if df := l.defOf[n]; df != nil {
				add(df)
			}
			for _, k := range n.kids {
				if k.space == NamespaceXSD && k.local != "annotation" {
					walk(k)
				}
			}
		}
		walk(d.root)
	}
	for _, df := range all {
		l.ensure(df)
		l.drain()
	}
	for i := 0; i < len(l.checks); i++ {
		l.checks[i]()
	}
	l.finalChecks()
	for k := defKind(0); k < kindCount; k++ {
		for q, df := range l.index[k] {
			switch k {
			case kindType:
				l.sch.Types[q] = df.comp.(Type)
			case kindElement:
				l.sch.Elements[q] = df.comp.(*ElementDecl)
			case kindAttribute:
				l.sch.Attributes[q] = df.comp.(*AttributeDecl)
			case kindAttributeGroup:
				l.sch.AttributeGroups[q] = df.comp.(*AttributeGroup)
			case kindGroup:
				l.sch.Groups[q] = df.comp.(*ModelGroupDef)
			case kindNotation:
				l.sch.Notations[q] = df.comp.(*Notation)
			case kindIdentity:
				l.sch.IdentityConstraints[q] = df.comp.(*IdentityConstraint)
			}
		}
	}
}

// drain buduje odłożone typy anonimowe.
func (l *loader) drain() {
	for len(l.queue) > 0 {
		f := l.queue[0]
		l.queue = l.queue[1:]
		f()
	}
}

// ---- deklaracje elementów (3.3.2)

func (l *loader) buildElementParticle(n *node, scope *ComplexType) *Particle {
	ref, hasRef := n.attr("ref")
	_, hasName := n.attr("name")
	if hasRef == hasName {
		l.errorf(CodeElementRep+".2.1", n.pos(), "a local xs:element must have either a name or a ref attribute, not both or neither")
	}
	if hasRef {
		for _, a := range []string{"nillable", "default", "fixed", "form", "block", "type"} {
			if n.has(a) {
				l.errorf(CodeElementRep+".2.2", n.attrPos(a), "xs:element with ref cannot have attribute %s", a)
			}
		}
		for _, k := range n.xsdKids() {
			l.errorf(CodeElementRep+".2.2", k.pos(), "xs:element with ref cannot have a %s child", k.name())
		}
	}
	p, ok := l.particleRange(n)
	if hasRef && !hasName {
		df := l.resolve(n, "ref", ref, kindElement)
		if !ok || df == nil {
			return nil
		}
		p.Term = df.comp.(*ElementDecl)
		return p
	}
	if !ok {
		return nil
	}
	name, _ := n.attr("name")
	q := QName{Local: collapse(name)}
	qualified := n.doc.elementQualified
	if v, ok := n.attr("form"); ok {
		qualified = collapse(v) == "qualified"
	}
	if qualified {
		q.Space = n.doc.tns
	}
	e := &ElementDecl{Name: q, Scope: Scope{Parent: scope}, Pos: n.pos()}
	l.fillElement(n, e, nil)
	p.Term = e
	return p
}

func (l *loader) buildGlobalElement(df *def) {
	l.fillElement(df.node, df.comp.(*ElementDecl), df)
}

// fillElement wypełnia deklarację elementu e z n; df jest niepuste dla deklaracji globalnych.
func (l *loader) fillElement(n *node, e *ElementDecl, df *def) {
	initBuiltins()
	l.sch.ElementDecls = append(l.sch.ElementDecls, e)
	_, hasDefault := n.attr("default")
	_, hasFixed := n.attr("fixed")
	if hasDefault && hasFixed {
		l.errorf(CodeElementRep+".1", n.pos(), "xs:element cannot have both default and fixed")
	}
	st, ct := n.kid("simpleType"), n.kid("complexType")
	typeAttr, hasType := n.attr("type")
	if hasType && (st != nil || ct != nil) {
		l.errorf(CodeElementRep+".3", n.pos(), "xs:element cannot have both a type attribute and an anonymous type")
	}
	e.Nillable, _ = boolAttr(n, "nillable")
	e.Block = blockOf(n, elementBlockMask)
	e.SubstitutionExclusions = finalOf(n, elementFinalMask)
	if df != nil {
		e.Abstract, _ = boolAttr(n, "abstract")
	}
	// {substitution group affiliation}: głowa musi być zbudowana, bo może dać typ (3.3.2).
	if v, ok := n.attr("substitutionGroup"); ok && df != nil {
		if hd := l.resolve(n, "substitutionGroup", v, kindElement); hd != nil {
			if hd.state == stateBuilding {
				l.errorf(CodeElementProps+".6", n.attrPos("substitutionGroup"), "circular substitution group through %s", hd.name)
			} else {
				l.ensure(hd)
				e.SubstitutionGroup = hd.comp.(*ElementDecl)
			}
		}
	}
	switch {
	case st != nil:
		t := &SimpleType{}
		l.buildSimpleType(st, t)
		e.Type = t
	case ct != nil:
		t := &ComplexType{}
		e.Type = t
		l.queue = append(l.queue, func() { l.buildComplexType(ct, t) })
	case hasType:
		if t, _ := l.typeDef(n, "type", typeAttr, false); t != nil {
			e.Type = t
		}
	case e.SubstitutionGroup != nil && e.SubstitutionGroup.Type != nil:
		e.Type = e.SubstitutionGroup.Type
	}
	if e.Type == nil {
		e.Type = anyType
	}
	if v, ok := n.attr("fixed"); ok {
		e.Value = &ValueConstraint{Fixed: true, Lexical: v, ctx: n}
	} else if v, ok := n.attr("default"); ok {
		e.Value = &ValueConstraint{Lexical: v, ctx: n}
	}
	for _, k := range n.kids {
		if k.is("key") || k.is("unique") || k.is("keyref") {
			if idf := l.defOf[k]; idf != nil {
				l.ensure(idf)
				e.IdentityConstraints = append(e.IdentityConstraints, idf.comp.(*IdentityConstraint))
			}
		}
	}
	l.checks = append(l.checks, func() { l.checkElement(n, e) })
}

// ---- deklaracje atrybutów (3.2.2)

func (l *loader) buildGlobalAttribute(df *def) {
	n := df.node
	a := df.comp.(*AttributeDecl)
	l.checkAttributeRep(n, true)
	a.Type = l.attributeType(n)
	if v, ok := n.attr("fixed"); ok {
		a.Value = &ValueConstraint{Fixed: true, Lexical: v, ctx: n}
	} else if v, ok := n.attr("default"); ok {
		a.Value = &ValueConstraint{Lexical: v, ctx: n}
	}
	l.attrDecls = append(l.attrDecls, a)
}

// checkAttributeRep sprawdza Attribute Declaration Representation OK (3.2.3).
func (l *loader) checkAttributeRep(n *node, top bool) {
	_, hasDefault := n.attr("default")
	_, hasFixed := n.attr("fixed")
	if hasDefault && hasFixed {
		l.errorf(CodeAttributeRep+".1", n.pos(), "xs:attribute cannot have both default and fixed")
	}
	if use, ok := n.attr("use"); ok && hasDefault && collapse(use) != "optional" {
		l.errorf(CodeAttributeRep+".2", n.attrPos("use"), "an attribute with a default must have use=\"optional\"")
	}
	if !top {
		_, hasRef := n.attr("ref")
		_, hasName := n.attr("name")
		if hasRef == hasName {
			l.errorf(CodeAttributeRep+".3.1", n.pos(), "a local xs:attribute must have either a name or a ref attribute, not both or neither")
		}
		if hasRef {
			for _, a := range []string{"form", "type"} {
				if n.has(a) {
					l.errorf(CodeAttributeRep+".3.2", n.attrPos(a), "xs:attribute with ref cannot have attribute %s", a)
				}
			}
			if n.kid("simpleType") != nil {
				l.errorf(CodeAttributeRep+".3.2", n.pos(), "xs:attribute with ref cannot have a simpleType child")
			}
		}
	}
	if n.has("type") && n.kid("simpleType") != nil {
		l.errorf(CodeAttributeRep+".4", n.pos(), "xs:attribute cannot have both a type attribute and a simpleType child")
	}
}

// attributeType zwraca {type definition} deklaracji atrybutu z n.
func (l *loader) attributeType(n *node) *SimpleType {
	if st := n.kid("simpleType"); st != nil {
		t := &SimpleType{}
		l.buildSimpleType(st, t)
		return t
	}
	if v, ok := n.attr("type"); ok {
		t, _ := l.typeDef(n, "type", v, false)
		switch t := t.(type) {
		case *SimpleType:
			return t
		case *ComplexType:
			l.errorf(CodeResolve, n.attrPos("type"), "the type of an attribute must be a simple type, %s is complex", typeLabel(t))
		}
	}
	return anySimple
}

// buildAttributeUse buduje użycie atrybutu z elementu attribute w typie albo grupie atrybutów. Zwraca też
// nazwę atrybutu i to, czy use="prohibited" (wtedy element nie odpowiada żadnemu komponentowi).
func (l *loader) buildAttributeUse(n *node, scope Scope) (*AttributeUse, QName, bool) {
	l.checkAttributeRep(n, false)
	use, _ := n.attr("use")
	use = collapse(use)
	u := &AttributeUse{Required: use == "required", Pos: n.pos()}
	if v, ok := n.attr("ref"); ok {
		df := l.resolve(n, "ref", v, kindAttribute)
		var decl *AttributeDecl
		if df != nil {
			l.ensure(df)
			decl = df.comp.(*AttributeDecl)
		}
		if decl == nil {
			return nil, QName{}, use == "prohibited"
		}
		if use == "prohibited" {
			return nil, decl.Name, true
		}
		u.Decl = decl
	} else {
		name, _ := n.attr("name")
		q := QName{Local: collapse(name)}
		qualified := n.doc.attributeQualified
		if v, ok := n.attr("form"); ok {
			qualified = collapse(v) == "qualified"
		}
		if qualified {
			q.Space = n.doc.tns
		}
		if use == "prohibited" {
			return nil, q, true
		}
		decl := &AttributeDecl{Name: q, Scope: scope, Pos: n.pos()}
		decl.Type = l.attributeType(n)
		l.attrDecls = append(l.attrDecls, decl)
		u.Decl = decl
	}
	if v, ok := n.attr("fixed"); ok {
		u.Value = &ValueConstraint{Fixed: true, Lexical: v, ctx: n}
	} else if v, ok := n.attr("default"); ok {
		u.Value = &ValueConstraint{Lexical: v, ctx: n}
	}
	l.attrUses = append(l.attrUses, u)
	return u, u.Decl.Name, false
}

// ---- ograniczenia tożsamości (3.11.2)

func (l *loader) buildIdentity(df *def) {
	n := df.node
	ic := df.comp.(*IdentityConstraint)
	switch n.local {
	case "key":
		ic.Category = ICKey
	case "keyref":
		ic.Category = ICKeyRef
	}
	for _, k := range n.kids {
		switch {
		case k.is("selector"):
			v, _ := k.attr("xpath")
			x, err := parseXPath(v, false, k, l)
			if err != "" {
				l.errorf(CodeSelectorXPath, k.attrPos("xpath"), "selector %q: %s", v, err)
			}
			ic.Selector = x
		case k.is("field"):
			v, _ := k.attr("xpath")
			x, err := parseXPath(v, true, k, l)
			if err != "" {
				l.errorf(CodeFieldsXPath, k.attrPos("xpath"), "field %q: %s", v, err)
			}
			ic.Fields = append(ic.Fields, x)
		}
	}
	if ic.Category == ICKeyRef {
		l.checks = append(l.checks, func() {
			v, _ := n.attr("refer")
			rdf := l.resolve(n, "refer", v, kindIdentity)
			if rdf == nil {
				return
			}
			ref := rdf.comp.(*IdentityConstraint)
			if ref.Category == ICKeyRef {
				l.errorf(CodeIdentityProps+".1", n.attrPos("refer"), "keyref %s refers to keyref %s; it must refer to a key or unique", ic.Name, ref.Name)
				return
			}
			ic.Refer = ref
			if len(ref.Fields) != len(ic.Fields) {
				l.errorf(CodeIdentityProps+".2", n.pos(), "keyref %s has %d fields, the referenced %s has %d", ic.Name,
					len(ic.Fields), ref.Name, len(ref.Fields))
			}
		})
	}
}

// valueOf sprawdza wartość ograniczenia vc względem typu prostego st i zapisuje wartość.
func valueOf(vc *ValueConstraint, st *SimpleType) error {
	ctx := &datatypes.Context{}
	if vc.ctx != nil {
		ctx.Namespaces = vc.ctx
	}
	v, err := st.DT.Validate(vc.Lexical, ctx)
	if err != nil {
		return err
	}
	vc.Value = v
	return nil
}
