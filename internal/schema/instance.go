package schema

// Pomocnicze funkcje dla walidacji dokumentu (etap 5, internal/validate). Udostępniają reguły części 1, które
// wczytanie schematu ma już zaimplementowane, bez ujawniania struktur ładowania.

// TypeDerivationOK sprawdza Type Derivation OK (Complex) (3.4.6) albo (Simple) (3.14.6): czy d jest poprawnie
// wyprowadzony z b przy zbiorze zablokowanych metod subset. Dla xsi:type (cvc-elt.4.3) subset to suma
// {disallowed substitutions} deklaracji i {prohibited substitutions} typu deklaracji.
func TypeDerivationOK(d, b Type, subset DerivationSet) bool {
	return derivedOK(d, b, subset)
}

// XMLAttribute zwraca wbudowaną deklarację atrybutu przestrzeni xml (lang, space, base, id) albo nil. Używa jej
// walidacja dokumentu, gdy żaden dokument schematu nie dostarcza deklaracji tej przestrzeni (ADR 0006).
func XMLAttribute(local string) *AttributeDecl {
	initBuiltins()
	return xmlAttributes[local]
}

// LookupAttribute zwraca globalną deklarację atrybutu q: ze schematu, a dla przestrzeni xml także wbudowaną.
func (s *Schema) LookupAttribute(q QName) *AttributeDecl {
	if a, ok := s.Attributes[q]; ok {
		return a
	}
	if q.Space == NamespaceXML {
		return XMLAttribute(q.Local)
	}
	return nil
}

// IsAnyType mówi, czy t to ur-typ anyType.
func IsAnyType(t Type) bool {
	initBuiltins()
	return t == Type(anyType)
}

// IsIDType mówi, czy typ prosty t to ID albo typ z niego wyprowadzony (3.15.5, ID/IDREF Table).
func IsIDType(t Type) bool { return isID(t) }

// DerivedFromBuiltin mówi, czy typ prosty t jest wbudowanym typem name albo z niego wyprowadzonym przez
// restrykcję (np. IDREF, ENTITY, NOTATION).
func DerivedFromBuiltin(t *SimpleType, name string) bool {
	b := BuiltinType(name)
	if b == nil || t == nil {
		return false
	}
	return t.DT == b.DT || t.DT.IsDerivedFrom(b.DT)
}

// Empty zwraca schemat bez komponentów (poza wbudowanymi). Służy walidacji dokumentu, którego schemat wskażą
// dopiero wskazówki xsi:schemaLocation.
func Empty() *Schema {
	return &Schema{
		Types: map[QName]Type{}, Elements: map[QName]*ElementDecl{}, Attributes: map[QName]*AttributeDecl{},
		AttributeGroups: map[QName]*AttributeGroup{}, Groups: map[QName]*ModelGroupDef{},
		Notations: map[QName]*Notation{}, IdentityConstraints: map[QName]*IdentityConstraint{},
	}
}

// XSIAttribute zwraca wbudowaną deklarację atrybutu xsi (type, nil, schemaLocation, noNamespaceSchemaLocation,
// 3.2.7) albo nil.
func XSIAttribute(local string) *AttributeDecl {
	initBuiltins()
	return xsiAttributes[local]
}
