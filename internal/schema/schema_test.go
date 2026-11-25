package schema

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

const xs = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

// schemaCase to mały schemat (pliki w MapFS) z oczekiwanym kodem błędu ("" = poprawny).
type schemaCase struct {
	name  string
	files map[string]string
	main  []string
	want  string
	cfg   Config
}

func doc(body string) string { return `<xs:schema ` + xs + `>` + body + `</xs:schema>` }
func docNS(tns, body string) string {
	return `<xs:schema ` + xs + ` targetNamespace="` + tns + `" xmlns:t="` + tns + `">` + body + `</xs:schema>`
}

func run(t *testing.T, cases []schemaCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := fstest.MapFS{}
			for name, body := range c.files {
				fsys[name] = &fstest.MapFile{Data: []byte(body)}
			}
			main := c.main
			if main == nil {
				main = []string{"main.xsd"}
			}
			_, err := Load(fsys, c.cfg, main...)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want a valid schema, got %v", err)
				}
				return
			}
			var list ErrorList
			if !errors.As(err, &list) {
				t.Fatalf("want error %s, got %v", c.want, err)
			}
			for _, e := range list {
				if e.Code == c.want {
					if e.Pos.Document == "" || e.Pos.Line == 0 {
						t.Errorf("error %s has no position: %+v", e.Code, e.Pos)
					}
					return
				}
			}
			t.Fatalf("want error %s, got %v", c.want, list)
		})
	}
}

func one(body string) map[string]string { return map[string]string{"main.xsd": doc(body)} }

// REQ: SCH-001
func TestDocuments(t *testing.T) {
	run(t, []schemaCase{
		{name: "not-well-formed", files: map[string]string{"main.xsd": `<xs:schema ` + xs + `><xs:element name="a">`}, want: CodeNotWellFormed},
		{name: "not-schema", files: map[string]string{"main.xsd": `<schema/>`}, want: CodeNotSchema},
		{name: "include-not-schema", files: map[string]string{"main.xsd": doc(`<xs:include schemaLocation="b.xml"/>`),
			"b.xml": `<a/>`}, want: CodeInclude + ".1"},
		{name: "minimal", files: one(``)},
	})
	_, err := Load(fstest.MapFS{"main.xsd": {Data: []byte(`<!DOCTYPE s [<!ENTITY e SYSTEM "x">]><xs:schema ` + xs + `>&e;</xs:schema>`)}}, Config{}, "main.xsd")
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("external entity in a schema document: want ErrUnsupported, got %v", err)
	}
}

// REQ: SCH-002
func TestSchemaForSchemas(t *testing.T) {
	run(t, []schemaCase{
		{name: "wrong-child-order", files: one(`<xs:element name="a"/><xs:include schemaLocation="b.xsd"/>`), want: CodeContent},
		{name: "unknown-xsd-element", files: one(`<xs:elephant name="a"/>`), want: CodeContent},
		{name: "attribute-not-allowed", files: one(`<xs:element name="a" minOccurs="1"/>`), want: CodeAttribute},
		{name: "xsd-namespace-attribute", files: one(`<xs:element name="a" xs:type="b"/>`), want: CodeAttribute},
		{name: "foreign-attribute-ok", files: one(`<xs:element name="a" xmlns:f="urn:f" f:x="1"/>`)},
		{name: "required-missing", files: one(`<xs:element/>`), want: CodeAttributeMissing},
		{name: "bad-boolean", files: one(`<xs:element name="a" nillable="yes"/>`), want: CodeAttributeValue},
		{name: "bad-ncname", files: one(`<xs:element name="1a"/>`), want: CodeAttributeValue},
		{name: "bad-block", files: one(`<xs:element name="a" block="#all extension"/>`), want: CodeAttributeValue},
		{name: "bad-xml-lang", files: map[string]string{"main.xsd": `<xs:schema ` + xs + ` xml:lang="not a lang"/>`}, want: CodeAttributeValue},
		{name: "text-in-element", files: one(`<xs:element name="a">text</xs:element>`), want: CodeText},
		{name: "text-in-documentation-ok", files: one(`<xs:annotation><xs:documentation>tekst <b>x</b></xs:documentation></xs:annotation>`)},
		{name: "duplicate-id", files: one(`<xs:element name="a" id="x"/><xs:element name="b" id="x"/>`), want: CodeDuplicateID},
		{name: "all-max-two", files: one(`<xs:complexType name="t"><xs:all><xs:element name="a" maxOccurs="2"/></xs:all></xs:complexType>`), want: CodeAttributeValue},
		{name: "local-element-final", files: one(`<xs:complexType name="t"><xs:sequence><xs:element name="a" final="#all"/></xs:sequence></xs:complexType>`), want: CodeAttribute},
	})
}

// REQ: SCH-003
func TestQNameResolution(t *testing.T) {
	run(t, []schemaCase{
		{name: "undeclared-prefix", files: one(`<xs:element name="a" type="p:t"/>`), want: CodeQName},
		{name: "unresolved", files: one(`<xs:element name="a" type="t"/>`), want: CodeResolve},
		{name: "builtin", files: one(`<xs:element name="a" type="xs:decimal"/>`)},
		{name: "not-imported", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:element name="a" xmlns:b="urn:b" type="b:t"/>`)}, want: CodeResolve4 + ".2"},
		{name: "absent-needs-no-tns", files: map[string]string{"main.xsd": `<xs:schema ` + xs + ` targetNamespace="urn:a"><xs:element name="a" type="t"/><xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`}, want: CodeResolve4 + ".1"},
		{name: "default-namespace", files: map[string]string{"main.xsd": `<xs:schema ` + xs + ` targetNamespace="urn:a" xmlns="urn:a"><xs:element name="a" type="t"/><xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`}},
		{name: "xml-lang-builtin", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="http://www.w3.org/XML/1998/namespace"/><xs:attributeGroup name="g"><xs:attribute ref="xml:lang"/></xs:attributeGroup>`)}},
		{name: "wrong-kind", files: one(`<xs:element name="a" type="e"/><xs:element name="e"/>`), want: CodeResolve},
	})
}

// REQ: SCH-004
func TestLocations(t *testing.T) {
	b := doc(`<xs:element name="b"/>`)
	run(t, []schemaCase{
		{name: "relative", files: map[string]string{"main.xsd": doc(`<xs:include schemaLocation="sub/b.xsd"/><xs:element name="a"><xs:complexType><xs:sequence><xs:element ref="b"/></xs:sequence></xs:complexType></xs:element>`),
			"sub/b.xsd": b}},
		{name: "unresolved-include-ok", files: one(`<xs:include schemaLocation="missing.xsd"/>`)},
		{name: "network-not-used", files: one(`<xs:include schemaLocation="http://example.com/b.xsd"/>`)},
		{name: "locations", files: map[string]string{"main.xsd": doc(`<xs:include schemaLocation="http://example.com/b.xsd"/><xs:element name="a"><xs:complexType><xs:sequence><xs:element ref="b"/></xs:sequence></xs:complexType></xs:element>`),
			"local/b.xsd": b}, cfg: Config{Locations: map[string]string{"http://example.com/b.xsd": "local/b.xsd"}}},
		{name: "relative-to-url-base", files: map[string]string{
			"main.xsd":    doc(`<xs:include schemaLocation="http://example.com/x/b.xsd"/>`),
			"local/b.xsd": doc(`<xs:include schemaLocation="c.xsd"/>`), "local/c.xsd": doc(`<xs:element name="c"/>`)},
			cfg: Config{Locations: map[string]string{"http://example.com/x/b.xsd": "local/b.xsd", "http://example.com/x/c.xsd": "local/c.xsd"}}},
		{name: "case-must-match", files: map[string]string{"main.xsd": doc(`<xs:include schemaLocation="B.xsd"/><xs:element name="a" type="t"/>`),
			"b.xsd": doc(`<xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType>`)}, want: CodeResolve},
		{name: "outside-fs", files: one(`<xs:include schemaLocation="../../etc/passwd"/>`)},
		{name: "namespaces", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:b"/><xs:element name="a" xmlns:b="urn:b" type="b:t"/>`),
			"b.xsd": docNS("urn:b", `<xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType>`)},
			cfg: Config{Namespaces: map[string]string{"urn:b": "b.xsd"}}},
		{name: "redefine-unresolved", files: one(`<xs:redefine schemaLocation="missing.xsd"><xs:simpleType name="t"><xs:restriction base="t"/></xs:simpleType></xs:redefine>`), want: CodeRedefine + ".1"},
		{name: "import-xsd-skipped", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="http://www.w3.org/2001/XMLSchema" schemaLocation="XMLSchema.xsd"/>`)}},
	})
}

// REQ: SCH-005
func TestInclude(t *testing.T) {
	run(t, []schemaCase{
		{name: "namespace-mismatch", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:include schemaLocation="b.xsd"/>`), "b.xsd": docNS("urn:b", ``)}, want: CodeInclude + ".2.1"},
		{name: "chameleon", files: map[string]string{
			"main.xsd": docNS("urn:a", `<xs:include schemaLocation="b.xsd"/><xs:element name="a" type="t:t"/>`),
			"b.xsd":    doc(`<xs:simpleType name="t"><xs:restriction base="u"/></xs:simpleType><xs:simpleType name="u"><xs:restriction base="xs:string"/></xs:simpleType>`)}},
		{name: "included-twice", files: map[string]string{
			"main.xsd": doc(`<xs:include schemaLocation="b.xsd"/><xs:include schemaLocation="b.xsd"/>`), "b.xsd": doc(`<xs:element name="b"/>`)}},
		{name: "listed-and-included", files: map[string]string{
			"main.xsd": doc(`<xs:include schemaLocation="b.xsd"/>`), "b.xsd": doc(`<xs:element name="b"/>`)}, main: []string{"main.xsd", "b.xsd"}},
		{name: "include-cycle", files: map[string]string{
			"main.xsd": doc(`<xs:include schemaLocation="b.xsd"/>`), "b.xsd": doc(`<xs:include schemaLocation="main.xsd"/>`)}},
	})
	// Kameleon: nazwy, odwołania i symbole wieloznaczne dostają przestrzeń włączającego.
	fsys := fstest.MapFS{
		"main.xsd": {Data: []byte(docNS("urn:a", `<xs:include schemaLocation="b.xsd"/>`))},
		"b.xsd":    {Data: []byte(doc(`<xs:complexType name="c"><xs:sequence><xs:any namespace="##targetNamespace"/></xs:sequence><xs:anyAttribute namespace="##other"/></xs:complexType>`))},
	}
	sch, err := Load(fsys, Config{}, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	ct, ok := sch.Types[QName{Space: "urn:a", Local: "c"}].(*ComplexType)
	if !ok {
		t.Fatalf("chameleon type not in urn:a: %v", sch.Types)
	}
	w := ct.Particle.Term.(*ModelGroup).Particles[0].Term.(*Wildcard)
	if w.Namespaces.Kind != NamespaceSet || len(w.Namespaces.Set) != 1 || w.Namespaces.Set[0] != "urn:a" {
		t.Errorf("##targetNamespace in a chameleon: %+v", w.Namespaces)
	}
	if aw := ct.AttributeWildcard.Namespaces; aw.Kind != NamespaceNot || aw.Not != "urn:a" {
		t.Errorf("##other in a chameleon: %+v", aw)
	}
}

// REQ: SCH-006
func TestImport(t *testing.T) {
	run(t, []schemaCase{
		{name: "own-namespace", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:a"/>`)}, want: CodeImport + ".1.1"},
		{name: "no-namespace-no-tns", files: one(`<xs:import/>`), want: CodeImport + ".1.2"},
		{name: "namespace-mismatch", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:b" schemaLocation="b.xsd"/>`), "b.xsd": docNS("urn:c", ``)}, want: CodeImport + ".3.1"},
		{name: "no-namespace-but-tns", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import schemaLocation="b.xsd"/>`), "b.xsd": docNS("urn:c", ``)}, want: CodeImport + ".3.2"},
		{name: "mismatch-cached", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:b" schemaLocation="b.xsd"/><xs:import namespace="urn:c" schemaLocation="b.xsd"/>`), "b.xsd": docNS("urn:b", ``)}, want: CodeImport + ".3.1"},
		{name: "ok", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:b" schemaLocation="b.xsd"/><xs:element name="a" xmlns:b="urn:b" type="b:t"/>`),
			"b.xsd": docNS("urn:b", `<xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType>`)}},
	})
}

// REQ: SCH-007
func TestRedefine(t *testing.T) {
	orig := doc(`<xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType>
		<xs:complexType name="c"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
		<xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
		<xs:attributeGroup name="ag"><xs:attribute name="x"/></xs:attributeGroup>
		<xs:element name="e" type="t"/>`)
	run(t, []schemaCase{
		{name: "ok", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd">
			<xs:simpleType name="t"><xs:restriction base="t"><xs:maxLength value="3"/></xs:restriction></xs:simpleType>
			<xs:complexType name="c"><xs:complexContent><xs:extension base="c"><xs:sequence><xs:element name="b"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
			<xs:group name="g"><xs:sequence><xs:group ref="g"/><xs:element name="b"/></xs:sequence></xs:group>
			<xs:attributeGroup name="ag"><xs:attributeGroup ref="ag"/><xs:attribute name="y"/></xs:attributeGroup>
			</xs:redefine>`)}},
		{name: "namespace-mismatch", files: map[string]string{"orig.xsd": docNS("urn:b", ``), "main.xsd": docNS("urn:a", `<xs:redefine schemaLocation="orig.xsd"/>`)}, want: CodeRedefine + ".3.1"},
		{name: "type-not-self", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType></xs:redefine>`)}, want: CodeRedefine + ".5"},
		{name: "group-two-self-refs", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:group name="g"><xs:sequence><xs:group ref="g"/><xs:group ref="g"/></xs:sequence></xs:group></xs:redefine>`)}, want: CodeRedefine + ".6.1.1"},
		{name: "group-self-ref-occurs", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:group name="g"><xs:sequence><xs:group ref="g" maxOccurs="2"/></xs:sequence></xs:group></xs:redefine>`)}, want: CodeRedefine + ".6.1.2"},
		{name: "attribute-group-two-self-refs", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:attributeGroup name="ag"><xs:attributeGroup ref="ag"/><xs:attributeGroup ref="ag"/></xs:attributeGroup></xs:redefine>`)}, want: CodeRedefine + ".7.1"},
		{name: "attribute-group-not-restriction", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:attributeGroup name="ag"><xs:attribute name="z"/></xs:attributeGroup></xs:redefine>`)}, want: CodeRedefine + ".7.2.2.2.2"},
		{name: "missing-component", files: map[string]string{"orig.xsd": orig, "main.xsd": doc(`<xs:redefine schemaLocation="orig.xsd"><xs:simpleType name="nope"><xs:restriction base="nope"/></xs:simpleType></xs:redefine>`)}, want: CodeRedefine},
	})
	// Redefinicja działa wszędzie: element e z dokumentu redefiniowanego ma nowy typ t.
	fsys := fstest.MapFS{"orig.xsd": {Data: []byte(orig)}, "main.xsd": {Data: []byte(doc(`<xs:redefine schemaLocation="orig.xsd">
		<xs:simpleType name="t"><xs:restriction base="t"><xs:maxLength value="3"/></xs:restriction></xs:simpleType></xs:redefine>`))}}
	sch, err := Load(fsys, Config{}, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	st := sch.Elements[QName{Local: "e"}].Type.(*SimpleType)
	if st != sch.Types[QName{Local: "t"}] || st.Base.(*SimpleType).Name.Local != "" {
		t.Errorf("redefinition is not pervasive or its base is not the nameless original")
	}
}

// REQ: SCH-008
func TestSchemaProperties(t *testing.T) {
	run(t, []schemaCase{
		{name: "duplicate-element", files: one(`<xs:element name="a"/><xs:element name="a"/>`), want: CodeSchemaProps + ".2"},
		{name: "duplicate-across-documents", files: map[string]string{"main.xsd": doc(`<xs:include schemaLocation="b.xsd"/><xs:simpleType name="t"><xs:restriction base="xs:string"/></xs:simpleType>`),
			"b.xsd": doc(`<xs:complexType name="t"/>`)}, want: CodeSchemaProps + ".2"},
		{name: "same-name-other-symbol-space", files: one(`<xs:element name="a"/><xs:complexType name="a"/>`)},
		{name: "duplicate-identity", files: one(`<xs:element name="a"><xs:key name="k"><xs:selector xpath="b"/><xs:field xpath="@c"/></xs:key></xs:element>
			<xs:element name="b"><xs:unique name="k"><xs:selector xpath="b"/><xs:field xpath="@c"/></xs:unique></xs:element>`), want: CodeSchemaProps + ".2"},
		{name: "empty-target-namespace", files: map[string]string{"main.xsd": `<xs:schema ` + xs + ` targetNamespace=""/>`}, want: CodeSchemaProps + ".1"},
	})
}

// REQ: SCH-009
func TestSimpleTypes(t *testing.T) {
	st := func(body string) map[string]string {
		return one(`<xs:simpleType name="t">` + body + `</xs:simpleType>`)
	}
	run(t, []schemaCase{
		{name: "restriction-both", files: st(`<xs:restriction base="xs:string"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction>`), want: CodeSimpleTypeRep + ".2"},
		{name: "list-neither", files: st(`<xs:list/>`), want: CodeSimpleTypeRep + ".3"},
		{name: "circular-union", files: one(`<xs:simpleType name="t"><xs:union memberTypes="u"/></xs:simpleType><xs:simpleType name="u"><xs:union memberTypes="t"/></xs:simpleType>`), want: CodeSimpleTypeRep + ".4"},
		{name: "complex-base", files: one(`<xs:simpleType name="t"><xs:restriction base="c"/></xs:simpleType><xs:complexType name="c"/>`), want: CodeSimpleTypeProps + ".1"},
		{name: "circular-restriction", files: one(`<xs:simpleType name="t"><xs:restriction base="u"/></xs:simpleType><xs:simpleType name="u"><xs:restriction base="t"/></xs:simpleType>`), want: CodeSimpleTypeProps + ".2"},
		{name: "final-restriction", files: one(`<xs:simpleType name="t" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="u"><xs:restriction base="t"/></xs:simpleType>`), want: CodeSimpleTypeProps + ".3"},
		{name: "any-simple-type-base", files: st(`<xs:restriction base="xs:anySimpleType"/>`), want: CodeSimpleRestricts + ".1.1"},
		{name: "list-of-list", files: st(`<xs:list itemType="xs:IDREFS"/>`), want: CodeSimpleRestricts + ".2.1"},
		{name: "final-list", files: one(`<xs:simpleType name="u" final="list"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="t"><xs:list itemType="u"/></xs:simpleType>`), want: CodeSimpleRestricts + ".2.3.1.1"},
		{name: "final-union", files: one(`<xs:simpleType name="u" final="#all"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="t"><xs:union memberTypes="u"/></xs:simpleType>`), want: CodeSimpleRestricts + ".3.3.1.1"},
		{name: "facet-error-from-datatypes", files: st(`<xs:restriction base="xs:string"><xs:totalDigits value="3"/></xs:restriction>`), want: "cos-applicable-facets"},
		{name: "union-flattened", files: one(`<xs:simpleType name="u"><xs:union memberTypes="xs:int xs:date"/></xs:simpleType><xs:simpleType name="t"><xs:union memberTypes="u xs:boolean"/></xs:simpleType>`)},
	})
	sch, err := Load(fstest.MapFS{"main.xsd": {Data: []byte(doc(`<xs:simpleType name="u"><xs:union memberTypes="xs:int xs:date"/></xs:simpleType><xs:simpleType name="t"><xs:union memberTypes="u xs:boolean"/></xs:simpleType>`))}}, Config{}, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if m := sch.Types[QName{Local: "t"}].(*SimpleType).Members; len(m) != 3 {
		t.Errorf("flattened union has %d members, want 3", len(m))
	}
}

// REQ: SCH-010
func TestAttributes(t *testing.T) {
	ct := func(body string) map[string]string {
		return one(`<xs:complexType name="c">` + body + `</xs:complexType>`)
	}
	run(t, []schemaCase{
		{name: "default-and-fixed", files: one(`<xs:attribute name="a" default="1" fixed="1"/>`), want: CodeAttributeRep + ".1"},
		{name: "default-required", files: ct(`<xs:attribute name="a" default="1" use="required"/>`), want: CodeAttributeRep + ".2"},
		{name: "ref-and-name", files: ct(`<xs:attribute name="a" ref="b"/>`), want: CodeAttributeRep + ".3.1"},
		{name: "ref-and-type", files: ct(`<xs:attribute ref="b" type="xs:int"/>`) /* type sprawdza też schemat dla schematów */, want: CodeAttributeRep + ".3.2"},
		{name: "type-and-simple-type", files: one(`<xs:attribute name="a" type="xs:int"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:attribute>`), want: CodeAttributeRep + ".4"},
		{name: "bad-default", files: one(`<xs:attribute name="a" type="xs:int" default="x"/>`), want: CodeAttributeProps + ".2"},
		{name: "bad-local-default", files: ct(`<xs:attribute name="a" type="xs:int" default="x"/>`), want: CodeAttributeProps + ".2"},
		{name: "id-with-default", files: one(`<xs:attribute name="a" type="xs:ID" default="x"/>`), want: CodeAttributeProps + ".3"},
		{name: "xmlns", files: one(`<xs:attribute name="xmlns"/>`), want: CodeNoXMLNS},
		{name: "xsi", files: map[string]string{"main.xsd": docNS(NamespaceXSI, `<xs:attribute name="a"/>`)}, want: CodeNoXSI},
		{name: "fixed-mismatch", files: one(`<xs:attribute name="a" fixed="1"/><xs:complexType name="c"><xs:attribute ref="a" fixed="2"/></xs:complexType>`), want: CodeAttributeUseProps + ".2"},
		{name: "notation-direct", files: one(`<xs:attribute name="a" type="xs:NOTATION"/>`), want: "enumeration-required-notation"},
		{name: "prohibited-no-component", files: ct(`<xs:attribute name="a" use="prohibited" type="xs:int"/>`)},
	})
}

// REQ: SCH-011
func TestElements(t *testing.T) {
	seq := func(body string) map[string]string {
		return one(`<xs:complexType name="c"><xs:sequence>` + body + `</xs:sequence></xs:complexType><xs:element name="g"/>`)
	}
	run(t, []schemaCase{
		{name: "default-and-fixed", files: one(`<xs:element name="a" default="1" fixed="1"/>`), want: CodeElementRep + ".1"},
		{name: "ref-and-name", files: seq(`<xs:element name="a" ref="g"/>`), want: CodeElementRep + ".2.1"},
		{name: "ref-with-type", files: seq(`<xs:element ref="g" type="xs:int"/>`), want: CodeElementRep + ".2.2"},
		{name: "type-and-anonymous", files: one(`<xs:element name="a" type="xs:int"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:element>`), want: CodeElementRep + ".3"},
		{name: "bad-default", files: one(`<xs:element name="a" type="xs:int" default="x"/>`), want: CodeElementProps + ".2"},
		{name: "default-element-only", files: one(`<xs:element name="a" default="x"><xs:complexType><xs:sequence><xs:element name="b"/></xs:sequence></xs:complexType></xs:element>`), want: CodeValidDefault + ".2.1"},
		{name: "default-mixed-not-emptiable", files: one(`<xs:element name="a" default="x"><xs:complexType mixed="true"><xs:sequence><xs:element name="b"/></xs:sequence></xs:complexType></xs:element>`), want: CodeValidDefault + ".2.2.2"},
		{name: "default-mixed-emptiable", files: one(`<xs:element name="a" default="x"><xs:complexType mixed="true"><xs:sequence><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>`)},
		{name: "substitution-type", files: one(`<xs:element name="h" type="xs:int"/><xs:element name="m" type="xs:string" substitutionGroup="h"/>`), want: CodeElementProps + ".4"},
		{name: "substitution-final", files: one(`<xs:element name="h" type="xs:decimal" final="restriction"/><xs:element name="m" type="xs:int" substitutionGroup="h"/>`), want: CodeElementProps + ".4"},
		{name: "id-default", files: one(`<xs:element name="a" type="xs:ID" default="x"/>`), want: CodeElementProps + ".5"},
		{name: "substitution-cycle", files: one(`<xs:element name="a" substitutionGroup="b"/><xs:element name="b" substitutionGroup="a"/>`), want: CodeElementProps + ".6"},
		{name: "zero-occurs-no-component", files: seq(`<xs:element name="a" minOccurs="0" maxOccurs="0"/>`)},
	})
	sch, err := Load(fstest.MapFS{"main.xsd": {Data: []byte(doc(`<xs:element name="h" type="xs:int"/><xs:element name="m" substitutionGroup="h"/><xs:element name="n"/>`))}}, Config{}, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if sch.Elements[QName{Local: "m"}].Type != BuiltinType("int") || sch.Elements[QName{Local: "n"}].Type != AnyType() {
		t.Errorf("type defaulting from the substitution group head or anyType failed")
	}
}

// REQ: SCH-012
func TestComplexTypeMapping(t *testing.T) {
	run(t, []schemaCase{
		{name: "complex-content-simple-base", files: one(`<xs:complexType name="c"><xs:complexContent><xs:extension base="xs:int"/></xs:complexContent></xs:complexType>`), want: CodeComplexTypeRep + ".1"},
		{name: "simple-content-from-element-only", files: one(`<xs:complexType name="b"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
			<xs:complexType name="c"><xs:simpleContent><xs:extension base="b"/></xs:simpleContent></xs:complexType>`), want: CodeComplexTypeRep + ".2.1"},
		{name: "simple-content-from-mixed-needs-simple-type", files: one(`<xs:complexType name="b" mixed="true"><xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType>
			<xs:complexType name="c"><xs:simpleContent><xs:restriction base="b"/></xs:simpleContent></xs:complexType>`), want: CodeComplexTypeRep + ".2.2"},
		{name: "unexpressible-intersection", files: one(`<xs:attributeGroup name="g1"><xs:anyAttribute namespace="##other"/></xs:attributeGroup>
			<xs:complexType name="c"><xs:attributeGroup ref="g1"/><xs:anyAttribute namespace="##other"/></xs:complexType>`)},
		{name: "simple-content-ok", files: one(`<xs:complexType name="c"><xs:simpleContent><xs:extension base="xs:int"><xs:attribute name="u"/></xs:extension></xs:simpleContent></xs:complexType>
			<xs:complexType name="d"><xs:simpleContent><xs:restriction base="c"><xs:maxInclusive value="5"/></xs:restriction></xs:simpleContent></xs:complexType>`)},
	})
	sch, err := Load(fstest.MapFS{"main.xsd": {Data: []byte(doc(`
		<xs:complexType name="b"><xs:sequence><xs:element name="x"/></xs:sequence><xs:attribute name="p"/><xs:attribute name="q"/></xs:complexType>
		<xs:complexType name="e"><xs:complexContent><xs:extension base="b"><xs:sequence><xs:element name="y"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
		<xs:complexType name="r"><xs:complexContent><xs:restriction base="b"><xs:sequence><xs:element name="x"/></xs:sequence><xs:attribute name="q" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType>
		<xs:complexType name="empty"><xs:sequence/></xs:complexType>
		<xs:complexType name="m" mixed="true"/>`))}}, Config{}, "main.xsd")
	if err != nil {
		t.Fatal(err)
	}
	e := sch.Types[QName{Local: "e"}].(*ComplexType)
	if g := e.Particle.Term.(*ModelGroup); len(g.Particles) != 2 || len(e.Attributes) != 2 {
		t.Errorf("extension: want sequence(base, own) and 2 inherited attributes, got %d particles, %d attributes", len(g.Particles), len(e.Attributes))
	}
	if r := sch.Types[QName{Local: "r"}].(*ComplexType); len(r.Attributes) != 1 || r.Attributes[0].Decl.Name.Local != "p" {
		t.Errorf("restriction with use=prohibited: %v", r.Attributes)
	}
	if c := sch.Types[QName{Local: "empty"}].(*ComplexType); c.Content != ContentEmpty {
		t.Errorf("empty sequence: content %v, want empty", c.Content)
	}
	if c := sch.Types[QName{Local: "m"}].(*ComplexType); c.Content != ContentMixed || c.Particle == nil {
		t.Errorf("mixed without particle: content %v", c.Content)
	}
}

// REQ: SCH-013
func TestComplexTypeConstraints(t *testing.T) {
	base := `<xs:complexType name="b"><xs:sequence><xs:element name="x" minOccurs="0"/></xs:sequence><xs:attribute name="p" use="required"/><xs:attribute name="f" fixed="1"/></xs:complexType>`
	run(t, []schemaCase{
		{name: "duplicate-attribute", files: one(`<xs:complexType name="c"><xs:attribute name="a"/><xs:attribute name="a"/></xs:complexType>`), want: CodeComplexTypeProps + ".4"},
		{name: "two-ids", files: one(`<xs:complexType name="c"><xs:attribute name="a" type="xs:ID"/><xs:attribute name="b" type="xs:ID"/></xs:complexType>`), want: CodeComplexTypeProps + ".5"},
		{name: "extension-final", files: one(`<xs:complexType name="b" final="extension"/><xs:complexType name="c"><xs:complexContent><xs:extension base="b"/></xs:complexContent></xs:complexType>`), want: CodeExtends + ".1.1"},
		{name: "extension-mixed-mismatch", files: one(`<xs:complexType name="b" mixed="true"><xs:sequence><xs:element name="x"/></xs:sequence></xs:complexType>
			<xs:complexType name="c"><xs:complexContent><xs:extension base="b"><xs:sequence><xs:element name="y"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>`), want: CodeExtends + ".1.4.3.2.2.1"},
		{name: "extension-simple-final", files: one(`<xs:simpleType name="s" final="extension"><xs:restriction base="xs:int"/></xs:simpleType>
			<xs:complexType name="c"><xs:simpleContent><xs:extension base="s"/></xs:simpleContent></xs:complexType>`), want: CodeExtends + ".2.2"},
		{name: "restriction-final", files: one(`<xs:complexType name="b" final="#all"/><xs:complexType name="c"><xs:complexContent><xs:restriction base="b"/></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".1"},
		{name: "restriction-required-dropped", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="p"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".2.1.1"},
		{name: "restriction-type-widened", files: one(`<xs:complexType name="b"><xs:attribute name="p" type="xs:int"/></xs:complexType>
			<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="p" type="xs:string"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".2.1.2"},
		{name: "restriction-fixed-changed", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="f" fixed="2"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".2.1.3"},
		{name: "restriction-new-attribute", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="z"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".2.2"},
		{name: "restriction-required-prohibited", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="p" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".3"},
		{name: "restriction-wildcard-not-in-base", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:anyAttribute/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".4.1"},
		{name: "restriction-empty-ok", files: one(base + `<xs:complexType name="c"><xs:complexContent><xs:restriction base="b"><xs:attribute name="p" use="required"/></xs:restriction></xs:complexContent></xs:complexType>`)},
		{name: "restriction-mixed-of-element-only", files: one(base + `<xs:complexType name="c" mixed="true"><xs:complexContent><xs:restriction base="b"><xs:sequence><xs:element name="x"/></xs:sequence><xs:attribute name="p" use="required"/></xs:restriction></xs:complexContent></xs:complexType>`), want: CodeRestriction + ".5.4.1.2"},
		{name: "restriction-simple-of-element-only", files: one(base + `<xs:complexType name="c"><xs:simpleContent><xs:restriction base="b"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:restriction></xs:simpleContent></xs:complexType>`), want: CodeComplexTypeRep + ".2.1"},
		{name: "simple-restriction-not-derived", files: one(`<xs:complexType name="b"><xs:simpleContent><xs:extension base="xs:int"/></xs:simpleContent></xs:complexType>
			<xs:complexType name="c"><xs:simpleContent><xs:restriction base="b"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction></xs:simpleContent></xs:complexType>`), want: CodeRestriction + ".5.2.2.1"},
	})
}

// REQ: SCH-014
func TestAttributeGroups(t *testing.T) {
	run(t, []schemaCase{
		{name: "circular", files: one(`<xs:attributeGroup name="a"><xs:attributeGroup ref="b"/></xs:attributeGroup><xs:attributeGroup name="b"><xs:attributeGroup ref="a"/></xs:attributeGroup>`), want: CodeAttributeGroupRep + ".3"},
		{name: "duplicate", files: one(`<xs:attributeGroup name="a"><xs:attribute name="x"/><xs:attribute name="x"/></xs:attributeGroup>`), want: CodeAttributeGroupProps + ".2"},
		{name: "two-ids", files: one(`<xs:attributeGroup name="a"><xs:attribute name="x" type="xs:ID"/><xs:attribute name="y" type="xs:ID"/></xs:attributeGroup>`), want: CodeAttributeGroupProps + ".3"},
		{name: "unexpressible", files: map[string]string{"main.xsd": docNS("urn:a", `<xs:import namespace="urn:b" schemaLocation="b.xsd"/>
			<xs:attributeGroup name="g" xmlns:b="urn:b"><xs:attributeGroup ref="b:g"/><xs:anyAttribute namespace="##other"/></xs:attributeGroup>`),
			"b.xsd": docNS("urn:b", `<xs:attributeGroup name="g"><xs:anyAttribute namespace="##other"/></xs:attributeGroup>`)}, want: CodeAttributeGroupRep + ".2"},
	})
}

// REQ: SCH-015
func TestModelGroups(t *testing.T) {
	run(t, []schemaCase{
		{name: "circular", files: one(`<xs:group name="a"><xs:sequence><xs:group ref="b"/></xs:sequence></xs:group><xs:group name="b"><xs:choice><xs:group ref="a"/></xs:choice></xs:group>`), want: CodeModelGroupProps + ".2"},
		{name: "recursion-through-element-ok", files: one(`<xs:group name="a"><xs:sequence><xs:element name="e"><xs:complexType><xs:group ref="a"/></xs:complexType></xs:element></xs:sequence></xs:group>`)},
		{name: "all-nested", files: one(`<xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group><xs:complexType name="c"><xs:sequence><xs:group ref="g"/></xs:sequence></xs:complexType>`), want: CodeAllLimited + ".1.2"},
		{name: "all-max", files: one(`<xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group><xs:complexType name="c"><xs:group ref="g" maxOccurs="2"/></xs:complexType>`), want: CodeAllLimited + ".1.2"},
		{name: "all-in-extension", files: one(`<xs:complexType name="b"><xs:sequence><xs:element name="x"/></xs:sequence></xs:complexType>
			<xs:complexType name="c"><xs:complexContent><xs:extension base="b"><xs:all><xs:element name="a"/></xs:all></xs:extension></xs:complexContent></xs:complexType>`), want: CodeAllLimited + ".1.2"},
		{name: "min-greater-than-max", files: one(`<xs:complexType name="c"><xs:sequence minOccurs="3" maxOccurs="2"><xs:element name="a"/></xs:sequence></xs:complexType>`), want: CodeParticleProps + ".2.1"},
		{name: "empty-sequence-no-particle", files: one(`<xs:complexType name="c"><xs:sequence minOccurs="3" maxOccurs="2"/></xs:complexType>`)},
		{name: "max-zero", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a" minOccurs="1" maxOccurs="0"/></xs:sequence></xs:complexType>`), want: CodeParticleProps + ".2.1"},
		{name: "huge-max", files: one(`<xs:complexType name="c"><xs:sequence><xs:element name="a" maxOccurs="99999999999999999999999"/></xs:sequence></xs:complexType>`)},
	})
}

// REQ: SCH-016
func TestWildcards(t *testing.T) {
	set := func(s ...string) NamespaceConstraint { return NamespaceConstraint{Kind: NamespaceSet, Set: s} }
	not := func(s string) NamespaceConstraint { return NamespaceConstraint{Kind: NamespaceNot, Not: s} }
	any := NamespaceConstraint{Kind: NamespaceAny}
	if !wildcardSubset(set("a"), not("b")) || wildcardSubset(set("", "a"), not("b")) || !wildcardSubset(not("a"), not("")) ||
		wildcardSubset(not(""), not("a")) || !wildcardSubset(not("a"), any) || wildcardSubset(any, set("a")) {
		t.Error("Wildcard Subset")
	}
	if u, ok := wildcardUnion(not("a"), set("a", "b")); !ok || u.Kind != NamespaceNot || u.Not != "" {
		t.Errorf("union 5.2: %+v %v", u, ok)
	}
	if _, ok := wildcardUnion(not("a"), set("")); ok {
		t.Error("union 5.3 must not be expressible")
	}
	if u, ok := wildcardUnion(not("a"), set("a", "")); !ok || u.Kind != NamespaceAny {
		t.Errorf("union 5.1: %+v", u)
	}
	if x, ok := wildcardIntersect(not("a"), set("a", "b", "")); !ok || len(x.Set) != 1 || x.Set[0] != "b" {
		t.Errorf("intersection 3: %+v", x)
	}
	if _, ok := wildcardIntersect(not("a"), not("b")); ok {
		t.Error("intersection 5 must not be expressible")
	}
	if !not("a").Allows("b") || not("a").Allows("") || !set("").Allows("") {
		t.Error("Wildcard allows Namespace Name")
	}
}

// REQ: SCH-017
func TestIdentityConstraints(t *testing.T) {
	elt := func(ic string) map[string]string {
		return map[string]string{"main.xsd": `<xs:schema ` + xs + ` xmlns:p="urn:p"><xs:element name="a">` + ic + `</xs:element></xs:schema>`}
	}
	run(t, []schemaCase{
		{name: "ok", files: elt(`<xs:key name="k"><xs:selector xpath=".//p:b | c/d"/><xs:field xpath="@x"/><xs:field xpath="child :: e/attribute :: p:y"/></xs:key>
			<xs:keyref name="r" refer="k"><xs:selector xpath="."/><xs:field xpath="f"/><xs:field xpath="p:*"/></xs:keyref>`)},
		{name: "selector-attribute", files: elt(`<xs:key name="k"><xs:selector xpath="@x"/><xs:field xpath="."/></xs:key>`), want: CodeSelectorXPath},
		{name: "selector-descendant-inside", files: elt(`<xs:key name="k"><xs:selector xpath="a//b"/><xs:field xpath="."/></xs:key>`), want: CodeSelectorXPath},
		{name: "field-attribute-not-last", files: elt(`<xs:key name="k"><xs:selector xpath="b"/><xs:field xpath="@x/y"/></xs:key>`), want: CodeFieldsXPath},
		{name: "undeclared-prefix", files: elt(`<xs:key name="k"><xs:selector xpath="q:b"/><xs:field xpath="."/></xs:key>`), want: CodeSelectorXPath},
		{name: "other-axis", files: elt(`<xs:key name="k"><xs:selector xpath="descendant::b"/><xs:field xpath="."/></xs:key>`), want: CodeSelectorXPath},
		{name: "refer-missing", files: elt(`<xs:keyref name="r" refer="nope"><xs:selector xpath="b"/><xs:field xpath="."/></xs:keyref>`), want: CodeResolve},
		{name: "refer-keyref", files: elt(`<xs:keyref name="r" refer="r"><xs:selector xpath="b"/><xs:field xpath="."/></xs:keyref>`), want: CodeIdentityProps + ".1"},
		{name: "field-count", files: elt(`<xs:key name="k"><xs:selector xpath="b"/><xs:field xpath="."/></xs:key><xs:keyref name="r" refer="k"><xs:selector xpath="b"/><xs:field xpath="."/><xs:field xpath="@x"/></xs:keyref>`), want: CodeIdentityProps + ".2"},
	})
}

// REQ: SCH-018
func TestNotations(t *testing.T) {
	run(t, []schemaCase{
		{name: "no-identifier", files: one(`<xs:notation name="n"/>`), want: CodeNotationProps},
		{name: "system-only", files: one(`<xs:notation name="n" system="x"/>`)},
		{name: "enumeration-undeclared", files: one(`<xs:simpleType name="t"><xs:restriction base="xs:NOTATION"><xs:enumeration value="n"/></xs:restriction></xs:simpleType>`), want: CodeResolve},
		{name: "enumeration-declared", files: one(`<xs:notation name="n" public="p"/><xs:simpleType name="t"><xs:restriction base="xs:NOTATION"><xs:enumeration value="n"/></xs:restriction></xs:simpleType>`)},
	})
}

// REQ: SCH-019
func TestErrorsDeterministic(t *testing.T) {
	fsys := fstest.MapFS{"main.xsd": {Data: []byte(doc(`<xs:element name="a" type="x"/><xs:element name="b" type="y"/><xs:element name="a"/>
		<xs:complexType name="c"><xs:attribute name="d"/><xs:attribute name="d"/></xs:complexType>`))}}
	var first string
	for i := 0; i < 5; i++ {
		_, err := Load(fsys, Config{}, "main.xsd")
		var list ErrorList
		if !errors.As(err, &list) || len(list) < 3 {
			t.Fatalf("want several errors, got %v", err)
		}
		var b strings.Builder
		for _, e := range list {
			b.WriteString(e.Error() + "\n")
		}
		if i == 0 {
			first = b.String()
		} else if b.String() != first {
			t.Fatalf("errors differ between runs:\n%s\nvs\n%s", first, b.String())
		}
	}
}

// REQ: SCH-020
func TestLimits(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 10; i++ {
		files[string(rune('a'+i))+".xsd"] = doc(`<xs:include schemaLocation="` + string(rune('a'+i+1)) + `.xsd"/>`)
	}
	files["main.xsd"] = doc(`<xs:include schemaLocation="a.xsd"/>`)
	run(t, []schemaCase{
		{name: "max-documents", files: files, cfg: Config{MaxDocuments: 5}, want: CodeLimit},
		{name: "max-depth", files: files, cfg: Config{MaxDepth: 3}, want: CodeLimit},
		{name: "within-limits", files: files},
	})
}
