package schema

// Grupy podstawienia (XSD 1.0 2e, część 1, 3.3.6: Substitution Group, Substitution Group OK (Transitive)).

// substitutionGroups liczy dla każdej globalnej deklaracji elementu jej ·substitution group·: zbiór deklaracji,
// które mogą stanąć w miejscu głowy. Kolejność jest powtarzalna: głowa (jeśli nie jest abstrakcyjna), potem
// członkowie w kolejności budowy deklaracji.
func substitutionGroups(decls []*ElementDecl) map[*ElementDecl][]*ElementDecl {
	// P (potential substitution group): domknięcie ze względu na {substitution group affiliation}.
	members := map[*ElementDecl][]*ElementDecl{}
	for _, e := range decls {
		if !e.Scope.Global {
			continue
		}
		// e należy do P każdej głowy na swoim łańcuchu przynależności (cykle odrzuca e-props-correct.6).
		seen := map[*ElementDecl]bool{e: true}
		for h := e.SubstitutionGroup; h != nil && !seen[h]; h = h.SubstitutionGroup {
			seen[h] = true
			members[h] = append(members[h], e)
		}
	}
	out := map[*ElementDecl][]*ElementDecl{}
	for _, head := range decls {
		if !head.Scope.Global {
			continue
		}
		var group []*ElementDecl
		if !head.Abstract {
			group = append(group, head)
		}
		for _, m := range members[head] {
			if !m.Abstract && substitutable(m, head, head.Block) {
				group = append(group, m)
			}
		}
		out[head] = group
	}
	return out
}

// substitutable sprawdza Substitution Group OK (Transitive): czy d może zastąpić c przy ograniczeniu blocking.
func substitutable(d, c *ElementDecl, blocking DerivationSet) bool {
	if d == c {
		return true
	}
	if blocking.Has(DerivationSubstitution) {
		return false
	}
	chain := false
	for h, n := d.SubstitutionGroup, 0; h != nil && n < 1<<16; h, n = h.SubstitutionGroup, n+1 {
		if h == c {
			chain = true
			break
		}
	}
	if !chain || d.Type == nil || c.Type == nil {
		return chain
	}
	block := blocking
	if ct, ok := c.Type.(*ComplexType); ok {
		block |= ct.Block
	}
	methods, ok := derivationMethods(d.Type, c.Type)
	if !ok {
		// Typ d nie leży na łańcuchu wyprowadzeń typu c (np. typ członkowski unii): decyduje Type Derivation OK.
		return derivedOK(d.Type, c.Type, block&(DerivationExtension|DerivationRestriction))
	}
	return methods&block == 0
}

// derivationMethods zbiera metody wyprowadzenia na łańcuchu {base type definition} od d do c, razem
// z {prohibited substitutions} typów pośrednich (punkt 2.3 Substitution Group OK (Transitive)). ok == false, gdy
// c nie leży na łańcuchu.
func derivationMethods(d, c Type) (DerivationSet, bool) {
	initBuiltins()
	var methods DerivationSet
	for t, n := d, 0; n < 1<<16; n++ {
		if t == c {
			return methods, true
		}
		switch x := t.(type) {
		case *ComplexType:
			if x == anyType {
				return 0, false
			}
			if t != d {
				methods |= x.Block
			}
			methods |= x.Derivation
			t = x.Base
		case *SimpleType:
			if x == anySimple || x.Base == nil {
				if c == Type(anyType) && x == anySimple {
					return methods | DerivationRestriction, true
				}
				return 0, false
			}
			methods |= DerivationRestriction
			t = x.Base
		default:
			return 0, false
		}
	}
	return 0, false
}
