package xsd

import (
	"strings"
	"testing"
	"testing/fstest"
)

// Zmiany zachowania czytnika goxmlgo między v0.2.0 a v0.5.0 (ADR 0015), które nie mogą zmienić werdyktu walidatora.

// TestEmptyEntityText: od goxmlgo v0.3.0 odwołanie do encji, która nie daje żadnego znaku przed znacznikiem, nie
// tworzy pustego tokenu tekstu. Walidator i tak nie liczył pustego tekstu jako treści, więc werdykty są te same:
// treść pusta i treść tylko z elementów z taką encją są poprawne, a encja z tekstem dalej jest tekstem.
// REQ: VAL-007
func TestEmptyEntityText(t *testing.T) {
	schema := fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="empty" minOccurs="0"><xs:complexType/></xs:element>
			<xs:element name="only" minOccurs="0"><xs:complexType><xs:sequence>
				<xs:element name="b" minOccurs="0" maxOccurs="unbounded"><xs:complexType/></xs:element>
			</xs:sequence></xs:complexType></xs:element>
			<xs:element name="fixed" minOccurs="0" type="xs:string" fixed=""/>
		</xs:sequence></xs:complexType></xs:element></xs:schema>`)}}
	set, err := Compile(schema, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	const dtd = `<!DOCTYPE r [<!ENTITY e ""><!ENTITY tag "<b/>"><!ENTITY x "x">]>`
	cases := []struct {
		doc   string
		codes string
	}{
		{`<r><empty>&e;</empty></r>`, ""},
		{`<r><only>&e;<b/>&e;</only></r>`, ""},
		{`<r><only>&tag;&tag;</only></r>`, ""},
		{`<r><fixed>&e;</fixed></r>`, ""},
		{`<r><empty>&x;</empty></r>`, "cvc-complex-type.2.1"},
		{`<r><only>&x;<b/></only></r>`, "cvc-complex-type.2.3"},
	}
	for _, c := range cases {
		errs, err := set.Validate(strings.NewReader(dtd + c.doc))
		if err != nil {
			t.Fatalf("%s: %v", c.doc, err)
		}
		var codes []string
		for _, e := range errs {
			codes = append(codes, e.Code)
		}
		if got := strings.Join(codes, " "); got != c.codes {
			t.Errorf("%s: codes %q, want %q (%v)", c.doc, got, c.codes, errs)
		}
	}
}

// TestXMLPrefixDeclared: od goxmlgo v0.4.0 deklaracja xmlns:xml (dozwolona, Namespaces in XML 1.0, rozdz. 3) jest
// na liście deklaracji elementu jak każda inna. Walidator i kompilator ją przyjmują, a prefiks xml rozwiązuje się
// tak samo jak bez deklaracji: w dokumencie schematu (ref="xml:lang") i w wartości typu QName.
// REQ: SCH-001, VAL-013
func TestXMLPrefixDeclared(t *testing.T) {
	const xmlNS = `xmlns:xml="http://www.w3.org/XML/1998/namespace"`
	for _, decl := range []string{"", " " + xmlNS} {
		schema := fstest.MapFS{
			"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"` + decl + `>
				<xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/>
				<xs:element name="r"><xs:complexType><xs:simpleContent><xs:extension base="xs:QName">
					<xs:attribute ref="xml:lang"/></xs:extension></xs:simpleContent></xs:complexType></xs:element>
				</xs:schema>`)},
			"xml.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"` + decl + `
				targetNamespace="http://www.w3.org/XML/1998/namespace">
				<xs:attribute name="lang" type="xs:language"/></xs:schema>`)},
		}
		set, err := Compile(schema, "s.xsd")
		if err != nil {
			t.Fatalf("schema with %q: %v", decl, err)
		}
		for _, doc := range []string{
			`<r xml:lang="pl">xml:lang</r>`,
			`<r ` + xmlNS + ` xml:lang="pl">xml:lang</r>`,
		} {
			errs, err := set.Validate(strings.NewReader(doc))
			if err != nil || len(errs) > 0 {
				t.Errorf("schema with %q, %s: %v %v", decl, doc, err, errs)
			}
		}
		// Zła wartość dalej jest błędem: deklaracja xmlns:xml nie zmienia rozwiązywania prefiksów.
		errs, err := set.Validate(strings.NewReader(`<r ` + xmlNS + `>nope:lang</r>`))
		if err != nil || len(errs) != 1 {
			t.Errorf("schema with %q, undeclared prefix: %v %v", decl, err, errs)
		}
	}
}
