package schema

import (
	"sync"

	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// Komponenty wbudowane: ur-typ anyType (3.4.7), prosty ur-typ anySimpleType (3.14.7), 44 typy z części 2
// i cztery deklaracje atrybutów xsi (3.2.7). Są niezmienne i wspólne dla wszystkich schematów.
var (
	builtinOnce   sync.Once
	anyType       *ComplexType
	anySimple     *SimpleType
	builtinTypes  map[string]*SimpleType
	builtinByDT   map[*datatypes.Type]*SimpleType
	xsiAttributes map[string]*AttributeDecl
	xmlAttributes map[string]*AttributeDecl
)

func initBuiltins() {
	builtinOnce.Do(func() {
		anyType = &ComplexType{Name: QName{Space: NamespaceXSD, Local: "anyType"}, Derivation: DerivationRestriction,
			Content: ContentMixed}
		anyType.Base = anyType
		wild := &Wildcard{Namespaces: NamespaceConstraint{Kind: NamespaceAny}, Process: ProcessLax}
		anyType.Particle = &Particle{MinOccurs: 1, MaxOccurs: 1, Term: &ModelGroup{Compositor: CompositorSequence,
			Particles: []*Particle{{MinOccurs: 0, MaxOccurs: Unbounded, Term: wild}}}}
		anyType.AttributeWildcard = &Wildcard{Namespaces: NamespaceConstraint{Kind: NamespaceAny}, Process: ProcessLax}

		anySimple = &SimpleType{Name: QName{Space: NamespaceXSD, Local: "anySimpleType"}, DT: datatypes.AnySimpleType(),
			Base: anyType, Builtin: true}
		builtinTypes = map[string]*SimpleType{"anySimpleType": anySimple}
		builtinByDT = map[*datatypes.Type]*SimpleType{anySimple.DT: anySimple}
		for _, name := range datatypes.BuiltinNames() {
			dt := datatypes.Builtin(name)
			st := &SimpleType{Name: QName{Space: NamespaceXSD, Local: name}, DT: dt, Builtin: true}
			builtinTypes[name] = st
			builtinByDT[dt] = st
		}
		for _, st := range builtinTypes {
			if st == anySimple {
				continue
			}
			if b := builtinByDT[st.DT.Base()]; b != nil {
				st.Base = b
			} else {
				st.Base = anySimple
			}
			if it := st.DT.ItemType(); it != nil {
				st.Item = builtinByDT[it]
			}
		}
		// Atrybuty przestrzeni xml (xml:lang, xml:space, xml:base, xml:id) z typami jak w schemacie tej
		// przestrzeni opublikowanym przez W3C. Używamy ich, gdy schemat importuje przestrzeń xml bez dokumentu
		// (strategia 1 z 4.3.2: komponenty są znane procesorowi), ADR 0006.
		empty, _ := datatypes.Restrict(datatypes.Builtin("string"), []datatypes.FacetSpec{{Kind: datatypes.FacetEnumeration, Value: ""}}, nil)
		langDT, _ := datatypes.NewUnion(datatypes.Builtin("language"), empty)
		spaceDT, _ := datatypes.Restrict(datatypes.Builtin("NCName"), []datatypes.FacetSpec{
			{Kind: datatypes.FacetEnumeration, Value: "default"}, {Kind: datatypes.FacetEnumeration, Value: "preserve"}}, nil)
		emptyST := &SimpleType{DT: empty, Base: builtinTypes["string"], Builtin: true}
		xmlAttributes = map[string]*AttributeDecl{
			"lang": {Name: QName{Space: NamespaceXML, Local: "lang"}, Scope: Scope{Global: true},
				Type: &SimpleType{DT: langDT, Base: anySimple, Members: []*SimpleType{builtinTypes["language"], emptyST}, Builtin: true}},
			"space": {Name: QName{Space: NamespaceXML, Local: "space"}, Scope: Scope{Global: true},
				Type: &SimpleType{DT: spaceDT, Base: builtinTypes["NCName"], Builtin: true}},
			"base": {Name: QName{Space: NamespaceXML, Local: "base"}, Scope: Scope{Global: true}, Type: builtinTypes["anyURI"]},
			"id":   {Name: QName{Space: NamespaceXML, Local: "id"}, Scope: Scope{Global: true}, Type: builtinTypes["ID"]},
		}
		xsiAttributes = map[string]*AttributeDecl{}
		for name, typ := range map[string]string{"type": "QName", "nil": "boolean", "schemaLocation": "",
			"noNamespaceSchemaLocation": "anyURI"} {
			t := builtinTypes[typ]
			if typ == "" {
				// xsi:schemaLocation: lista anyURI (3.2.7).
				dt, _ := datatypes.NewList(datatypes.Builtin("anyURI"))
				t = &SimpleType{DT: dt, Base: anySimple, Item: builtinTypes["anyURI"], Builtin: true}
			}
			xsiAttributes[name] = &AttributeDecl{Name: QName{Space: NamespaceXSI, Local: name}, Type: t,
				Scope: Scope{Global: true}}
		}
	})
}

// AnyType zwraca ur-typ anyType.
func AnyType() *ComplexType { initBuiltins(); return anyType }

// AnySimpleType zwraca prosty ur-typ anySimpleType.
func AnySimpleType() *SimpleType { initBuiltins(); return anySimple }

// BuiltinType zwraca wbudowany typ prosty o nazwie lokalnej name albo nil.
func BuiltinType(name string) *SimpleType { initBuiltins(); return builtinTypes[name] }

// LookupType zwraca typ globalny o nazwie q, także wbudowany (dla xsi:type w etapie 5).
func (s *Schema) LookupType(q QName) Type {
	if q.Space == NamespaceXSD {
		initBuiltins()
		if q.Local == "anyType" {
			return anyType
		}
		if st := builtinTypes[q.Local]; st != nil {
			return st
		}
	}
	if t, ok := s.Types[q]; ok {
		return t
	}
	return nil
}
