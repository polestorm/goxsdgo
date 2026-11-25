package schema

// Kontrole modeli zawartości (etap 4, ADR 0007).

// checkContentModel kompiluje model zawartości typu złożonego i sprawdza Unique Particle Attribution oraz Element
// Declarations Consistent (3.8.6).
func (l *loader) checkContentModel(ct *ComplexType) {
	switch ct.Content {
	case ContentEmpty:
		ct.Model = compileModel(nil, l.subst)
		return
	case ContentElementOnly, ContentMixed:
	default:
		return
	}
	m := compileModel(ct.Particle, l.subst)
	ct.Model = m
	if c := m.checkUPA(); c != nil {
		l.errorf(CodeNonambig, ct.Pos, "the content model of %s is ambiguous: %s", typeLabel(ct), c.describe())
	}
	l.checkEDC(ct)
}

// checkEDC sprawdza Element Declarations Consistent: deklaracje o tej samej nazwie w jednym modelu zawartości
// (wprost, pośrednio albo przez grupy podstawienia) mają ten sam nazwany typ.
func (l *loader) checkEDC(ct *ComplexType) {
	seen := map[QName]*ElementDecl{}
	reported := map[QName]bool{}
	visit := func(e *ElementDecl) {
		prev, ok := seen[e.Name]
		if !ok {
			seen[e.Name] = e
			return
		}
		if prev == e || reported[e.Name] {
			return
		}
		if prev.Type == nil || e.Type == nil || prev.Type != e.Type || prev.Type.TypeName().Local == "" {
			reported[e.Name] = true
			l.errorf(CodeElementConsistent, ct.Pos, "the content model of %s declares %s more than once with different or anonymous types",
				typeLabel(ct), e.Name)
		}
	}
	var walk func(p *Particle, depth int)
	walk = func(p *Particle, depth int) {
		if p == nil || depth > 1<<12 {
			return
		}
		switch t := p.Term.(type) {
		case *ElementDecl:
			visit(t)
			if t.Scope.Global {
				for _, m := range l.subst[t] {
					visit(m)
				}
			}
		case *ModelGroup:
			for _, k := range t.Particles {
				walk(k, depth+1)
			}
		}
	}
	walk(ct.Particle, 0)
}

// checkGroupRedefine sprawdza src-redefine.6.2.2: redefinicja grupy bez odwołania do siebie musi być poprawną
// restrykcją grupy oryginalnej.
func (l *loader) checkGroupRedefine(df *def) {
	if len(selfRefs(df.node, "group", df.name, l)) > 0 {
		return
	}
	gd, ok := df.comp.(*ModelGroupDef)
	if !ok || df.orig == nil {
		return
	}
	orig, ok := df.orig.comp.(*ModelGroupDef)
	if !ok || orig.Group == nil || gd.Group == nil {
		return
	}
	r := &Particle{MinOccurs: 1, MaxOccurs: 1, Term: gd.Group, Pos: gd.Pos}
	b := &Particle{MinOccurs: 1, MaxOccurs: 1, Term: orig.Group, Pos: orig.Pos}
	l.rs.why = ""
	if !l.rs.validRestriction(r, b) {
		l.errorf(CodeRedefine+".6.2.2", df.node.pos(), "the redefined group %s is not a valid restriction of the original: %s", df.name, l.rs.why)
	}
}
