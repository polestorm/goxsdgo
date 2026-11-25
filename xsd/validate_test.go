package xsd

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// Testy walidacji dokumentu (etap 5) na małych schematach. Każdy przypadek podaje oczekiwane kody błędów
// w kolejności; pusta lista oznacza dokument poprawny.

const testNS = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

type valCase struct {
	name   string
	schema string
	doc    string
	codes  []string
}

func runValCases(t *testing.T, cfg Config, cases []valCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set, err := cfg.Compile(fstest.MapFS{"s.xsd": {Data: []byte(c.schema)}}, "s.xsd")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			errs, err := set.Validate(strings.NewReader(c.doc))
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			var got []string
			for _, e := range errs {
				got = append(got, e.Code)
			}
			if strings.Join(got, " ") != strings.Join(c.codes, " ") {
				t.Fatalf("codes %v, want %v\n%v", got, c.codes, errs)
			}
		})
	}
}

// REQ: VAL-001, VAL-002, VAL-003, VAL-004, VAL-005
func TestValidateElementRules(t *testing.T) {
	s := `<xs:schema ` + testNS + `>
	  <xs:element name="r" type="T"/>
	  <xs:element name="abs" abstract="true" type="xs:string"/>
	  <xs:element name="n" type="xs:int" nillable="true"/>
	  <xs:element name="nf" type="xs:int" nillable="true" fixed="5"/>
	  <xs:element name="d" type="xs:int" default="7"/>
	  <xs:element name="f" type="xs:int" fixed="7"/>
	  <xs:element name="fm" fixed="abc"><xs:complexType mixed="true"/></xs:element>
	  <xs:element name="b" type="B" block="extension"/>
	  <xs:complexType name="T"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
	  <xs:complexType name="B"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
	  <xs:complexType name="BX"><xs:complexContent><xs:extension base="B">
	    <xs:sequence><xs:element name="c" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
	  <xs:simpleType name="Short"><xs:restriction base="xs:int"><xs:maxInclusive value="9"/></xs:restriction></xs:simpleType>
	</xs:schema>`
	const xsi = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	runValCases(t, Config{}, []valCase{
		{"declared root", s, `<r><a>x</a></r>`, nil},
		{"undeclared root", s, `<q/>`, []string{"cvc-elt.1"}},
		{"abstract", s, `<abs>x</abs>`, []string{"cvc-elt.2"}},
		{"nil", s, `<n` + xsi + ` xsi:nil="true"/>`, nil},
		{"nil not nillable", s, `<d` + xsi + ` xsi:nil="true">1</d>`, []string{"cvc-elt.3.1"}},
		{"nil with content", s, `<n` + xsi + ` xsi:nil="true">1</n>`, []string{"cvc-elt.3.2.1"}},
		{"nil with fixed", s, `<nf` + xsi + ` xsi:nil="true"/>`, []string{"cvc-elt.3.2.2"}},
		{"xsi:type bad QName", s, `<n` + xsi + ` xsi:type="1x">1</n>`, []string{"cvc-elt.4.1"}},
		{"xsi:type unresolved", s, `<n` + xsi + ` xsi:type="Nope">1</n>`, []string{"cvc-elt.4.2"}},
		{"xsi:type derived", s, `<n` + xsi + ` xsi:type="Short">5</n>`, nil},
		{"xsi:type derived facet", s, `<n` + xsi + ` xsi:type="Short">12</n>`, []string{"cvc-maxInclusive-valid"}},
		{"xsi:type not derived", s, `<n` + xsi + ` xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:string">x</n>`, []string{"cvc-elt.4.3", "cvc-datatype-valid.1.2.1"}},
		{"xsi:type undeclared prefix", s, `<n` + xsi + ` xsi:type="xs:string">1</n>`, []string{"cvc-elt.4.1"}},
		{"xsi:type blocked", s, `<b` + xsi + ` xsi:type="BX"><a>x</a><c>y</c></b>`, []string{"cvc-elt.4.3", "cvc-complex-type.2.4"}},
		{"default", s, `<d/>`, nil},
		{"fixed ok", s, `<f>07</f>`, nil},
		{"fixed wrong", s, `<f>8</f>`, []string{"cvc-elt.5.2.2.2.2"}},
		{"fixed empty", s, `<f/>`, nil},
		{"fixed mixed", s, `<fm>abd</fm>`, []string{"cvc-elt.5.2.2.2.1"}},
		// Typ mixed bez cząstki ma pusty model (3.4.2), więc dziecko narusza też model zawartości.
		{"fixed mixed child", s, `<fm>ab<x/></fm>`, []string{"cvc-complex-type.2.4", "cvc-elt.5.2.2.1"}},
	})
}

// REQ: VAL-006, VAL-007, VAL-010, VAL-022
func TestValidateContent(t *testing.T) {
	s := `<xs:schema ` + testNS + ` targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">
	  <xs:element name="seq"><xs:complexType><xs:sequence>
	    <xs:element name="a" type="xs:string"/><xs:element name="b" type="xs:string" minOccurs="0"/>
	  </xs:sequence></xs:complexType></xs:element>
	  <xs:element name="empty"><xs:complexType/></xs:element>
	  <xs:element name="simple"><xs:complexType><xs:simpleContent><xs:extension base="xs:int"/></xs:simpleContent></xs:complexType></xs:element>
	  <xs:element name="mixed"><xs:complexType mixed="true"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType></xs:element>
	  <xs:element name="strict"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="strict"/></xs:sequence></xs:complexType></xs:element>
	  <xs:element name="lax"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="lax"/></xs:sequence></xs:complexType></xs:element>
	  <xs:element name="skip"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="skip"/></xs:sequence></xs:complexType></xs:element>
	  <xs:element name="g"><xs:complexType><xs:sequence><xs:element ref="head" maxOccurs="2"/></xs:sequence></xs:complexType></xs:element>
	  <xs:element name="head" type="xs:string" abstract="true"/>
	  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
	  <xs:element name="i" type="xs:int"/>
	</xs:schema>`
	const n = ` xmlns="urn:t"`
	runValCases(t, Config{}, []valCase{
		{"sequence ok", s, `<seq` + n + `><a/><b/></seq>`, nil},
		{"sequence optional", s, `<seq` + n + `><a/></seq>`, nil},
		{"sequence wrong order", s, `<seq` + n + `><b/><a/></seq>`, []string{"cvc-complex-type.2.4"}},
		{"sequence incomplete", s, `<seq` + n + `></seq>`, []string{"cvc-complex-type.2.4"}},
		{"sequence extra", s, `<seq` + n + `><a/><b/><b/></seq>`, []string{"cvc-complex-type.2.4"}},
		{"element-only text", s, `<seq` + n + `>x<a/></seq>`, []string{"cvc-complex-type.2.3"}},
		{"element-only whitespace", s, `<seq` + n + `> <a/>
		</seq>`, nil},
		{"empty ok", s, `<empty` + n + `/>`, nil},
		{"empty whitespace", s, `<empty` + n + `> </empty>`, []string{"cvc-complex-type.2.1"}},
		{"empty child", s, `<empty` + n + `><a/></empty>`, []string{"cvc-complex-type.2.1"}},
		{"simple content", s, `<simple` + n + `>12</simple>`, nil},
		{"simple content child", s, `<simple` + n + `><a/></simple>`, []string{"cvc-complex-type.2.2"}},
		{"mixed", s, `<mixed` + n + `>x<a/>y</mixed>`, nil},
		{"strict wildcard declared", s, `<strict` + n + `><i>1</i></strict>`, nil},
		{"strict wildcard declared invalid", s, `<strict` + n + `><i>x</i></strict>`, []string{"cvc-datatype-valid.1.2.1"}},
		{"strict wildcard undeclared", s, `<strict` + n + `><zz/></strict>`, []string{"cvc-elt.1"}},
		{"lax wildcard undeclared", s, `<lax` + n + `><zz a="1"><yy/></zz></lax>`, nil},
		{"lax wildcard declared invalid", s, `<lax` + n + `><i>x</i></lax>`, []string{"cvc-datatype-valid.1.2.1"}},
		{"skip wildcard", s, `<skip` + n + `><i>x</i></skip>`, nil},
		{"substitution member", s, `<g` + n + `><member/><member/></g>`, nil},
		// Głowa abstrakcyjna nie pasuje do modelu; po błędzie modelu dziecko jest oceniane luźno względem deklaracji
		// globalnej, która jest abstrakcyjna.
		{"substitution abstract head", s, `<g` + n + `><head/></g>`, []string{"cvc-complex-type.2.4", "cvc-elt.2"}},
	})
}

// REQ: VAL-008, VAL-009
func TestValidateAttributes(t *testing.T) {
	s := `<xs:schema ` + testNS + ` targetNamespace="urn:t" xmlns="urn:t">
	  <xs:attribute name="g" type="xs:int"/>
	  <xs:element name="e"><xs:complexType>
	    <xs:attribute name="req" type="xs:int" use="required"/>
	    <xs:attribute name="fix" type="xs:decimal" fixed="1.5"/>
	    <xs:attribute name="len"><xs:simpleType><xs:restriction base="xs:string"><xs:maxLength value="3"/></xs:restriction></xs:simpleType></xs:attribute>
	  </xs:complexType></xs:element>
	  <xs:element name="w"><xs:complexType><xs:anyAttribute namespace="##targetNamespace" processContents="strict"/></xs:complexType></xs:element>
	  <xs:element name="wl"><xs:complexType><xs:anyAttribute namespace="##any" processContents="lax"/></xs:complexType></xs:element>
	  <xs:element name="s" type="xs:int"/>
	</xs:schema>`
	const n = ` xmlns="urn:t" xmlns:t="urn:t"`
	runValCases(t, Config{}, []valCase{
		{"ok", s, `<e` + n + ` req="1" fix="1.50" len="abc"/>`, nil},
		{"missing required", s, `<e` + n + `/>`, []string{"cvc-complex-type.4"}},
		{"undeclared", s, `<e` + n + ` req="1" x="2"/>`, []string{"cvc-complex-type.3.2.1"}},
		// Wartość stała atrybutu lokalnego należy do użycia atrybutu (3.2.2), więc reguła to cvc-au.
		{"fixed", s, `<e` + n + ` req="1" fix="2"/>`, []string{"cvc-au"}},
		{"value", s, `<e` + n + ` req="x"/>`, []string{"cvc-datatype-valid.1.2.1"}},
		{"facet", s, `<e` + n + ` req="1" len="abcd"/>`, []string{"cvc-maxLength-valid"}},
		{"strict wildcard declared", s, `<w` + n + ` t:g="1"/>`, nil},
		{"strict wildcard declared invalid", s, `<w` + n + ` t:g="x"/>`, []string{"cvc-datatype-valid.1.2.1"}},
		{"strict wildcard undeclared", s, `<w` + n + ` t:zz="1"/>`, []string{"cvc-attribute.1"}},
		{"wildcard namespace", s, `<w` + n + ` zz="1"/>`, []string{"cvc-complex-type.3.2.2"}},
		{"lax wildcard", s, `<wl` + n + ` zz="1" t:g="2"/>`, nil},
		{"simple type attribute", s, `<s` + n + ` a="1">1</s>`, []string{"cvc-type.3.1.1"}},
	})
}

// REQ: VAL-011, VAL-012, VAL-013
func TestValidateIdentity(t *testing.T) {
	s := `<xs:schema ` + testNS + `>
	  <xs:element name="r"><xs:complexType><xs:sequence>
	    <xs:element name="item" maxOccurs="unbounded"><xs:complexType>
	      <xs:attribute name="id" type="xs:ID"/><xs:attribute name="ref" type="xs:IDREFS"/>
	      <xs:attribute name="k" type="xs:decimal"/><xs:attribute name="u" type="xs:string"/>
	    </xs:complexType></xs:element>
	    <xs:element name="use" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:attribute name="k" type="xs:decimal"/></xs:complexType></xs:element>
	  </xs:sequence></xs:complexType>
	    <xs:key name="K"><xs:selector xpath="item"/><xs:field xpath="@k"/></xs:key>
	    <xs:unique name="U"><xs:selector xpath="item"/><xs:field xpath="@u"/></xs:unique>
	    <xs:keyref name="R" refer="K"><xs:selector xpath="use"/><xs:field xpath="@k"/></xs:keyref>
	  </xs:element>
	  <xs:element name="n"><xs:complexType><xs:attribute name="x" type="N"/></xs:complexType></xs:element>
	  <xs:notation name="png" public="image/png"/>
	  <xs:simpleType name="N"><xs:restriction base="xs:NOTATION"><xs:enumeration value="png"/></xs:restriction></xs:simpleType>
	</xs:schema>`
	runValCases(t, Config{}, []valCase{
		{"ok", s, `<r><item id="a" k="1" u="x"/><item id="b" ref="a b" k="2"/><use k="2.0"/></r>`, nil},
		{"duplicate ID", s, `<r><item id="a" k="1"/><item id="a" k="2"/></r>`, []string{"cvc-id.2"}},
		{"unresolved IDREF", s, `<r><item ref="zz" k="1"/></r>`, []string{"cvc-id.1"}},
		{"key missing field", s, `<r><item/></r>`, []string{"cvc-identity-constraint.4.2.1"}},
		{"key duplicate by value", s, `<r><item k="1"/><item k="1.0"/></r>`, []string{"cvc-identity-constraint.4.2.2"}},
		{"unique duplicate", s, `<r><item k="1" u="x"/><item k="2" u="x"/></r>`, []string{"cvc-identity-constraint.4.1"}},
		{"keyref unmatched", s, `<r><item k="1"/><use k="3"/></r>`, []string{"cvc-identity-constraint.4.3"}},
		{"notation", s, `<n x="png"/>`, nil},
	})
}

// REQ: VAL-014
func TestValidateSchemaLocationHints(t *testing.T) {
	fsys := fstest.MapFS{
		"a.xsd": {Data: []byte(`<xs:schema ` + testNS + ` targetNamespace="urn:a"><xs:element name="r"><xs:complexType><xs:sequence>
			<xs:any namespace="urn:b" processContents="strict"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)},
		"b.xsd": {Data: []byte(`<xs:schema ` + testNS + ` targetNamespace="urn:b"><xs:element name="e" type="xs:int"/></xs:schema>`)},
	}
	doc := `<a:r xmlns:a="urn:a" xmlns:b="urn:b" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
		xsi:schemaLocation="urn:b b.xsd"><b:e>1</b:e></a:r>`
	codes := func(cfg Config, doc string) []string {
		set, err := cfg.Compile(fsys, "a.xsd")
		if err != nil {
			t.Fatal(err)
		}
		errs, err := set.ValidateDocument(strings.NewReader(doc), "doc.xml")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range errs {
			out = append(out, e.Code)
		}
		return out
	}
	// Domyślnie wskazówki są pomijane: deklaracji urn:b nie ma.
	if got := codes(Config{}, doc); strings.Join(got, " ") != "cvc-elt.1" {
		t.Errorf("hints off: %v", got)
	}
	if got := codes(Config{SchemaLocationHints: true}, doc); len(got) != 0 {
		t.Errorf("hints on: %v", got)
	}
	late := `<a:r xmlns:a="urn:a" xmlns:b="urn:b" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
		<b:e xsi:schemaLocation="urn:a a2.xsd urn:b b.xsd">1</b:e></a:r>`
	if got := codes(Config{SchemaLocationHints: true}, late); len(got) == 0 || got[0] != "schema-location-hint" {
		t.Errorf("late hint: %v", got)
	}
}

// REQ: VAL-015, VAL-016, VAL-017
func TestValidateErrorDetails(t *testing.T) {
	s := `<xs:schema ` + testNS + `>
	  <xs:element name="r"><xs:complexType><xs:sequence>
	    <xs:element name="w" maxOccurs="unbounded"><xs:complexType><xs:sequence>
	      <xs:element name="v" type="xs:int"/></xs:sequence><xs:attribute name="a" type="xs:int"/></xs:complexType></xs:element>
	  </xs:sequence></xs:complexType></xs:element>
	</xs:schema>`
	set, err := Compile(fstest.MapFS{"s.xsd": {Data: []byte(s)}}, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	errs, err := set.Validate(strings.NewReader("<r>\n<w><v>1</v></w>\n<w a=\"x\"><v>y</v></w>\n<w><q/></w></r>"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Error{
		{Code: "cvc-datatype-valid.1.2.1", Line: 3, Path: "/r/w[2]/@a", Name: "a", Value: "x"},
		{Code: "cvc-datatype-valid.1.2.1", Line: 3, Path: "/r/w[2]/v", Name: "v", Value: "y"},
		{Code: "cvc-complex-type.2.4", Line: 4, Path: "/r/w[3]/q", Name: "q", Expected: []string{"v"}},
	}
	if len(errs) != len(want) {
		t.Fatalf("errors: %v", errs)
	}
	for i, w := range want {
		g := errs[i]
		if g.Code != w.Code || g.Line != w.Line || g.Path != w.Path || g.Name != w.Name || g.Value != w.Value ||
			strings.Join(g.Expected, ",") != strings.Join(w.Expected, ",") || g.Column == 0 || g.Message == "" {
			t.Errorf("error %d: %+v, want %+v", i, g, w)
		}
	}

	// MaxErrors: walidacja przerwana, błędy zwrócone razem z ErrTooManyErrors.
	limited, err := Config{MaxErrors: 2}.Compile(fstest.MapFS{"s.xsd": {Data: []byte(s)}}, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	errs, err = limited.Validate(strings.NewReader(`<r><w a="x"><v>y</v></w><w a="x"><v>y</v></w></r>`))
	if !errors.Is(err, ErrTooManyErrors) || len(errs) != 2 {
		t.Errorf("MaxErrors: %v, %d errors", err, len(errs))
	}

	// Błąd dobrej formy to błąd dokumentu z kodem czytnika.
	errs, err = set.Validate(strings.NewReader(`<r><w></r>`))
	if err != nil || len(errs) != 1 || errs[0].Code != "wfc-element-type-match" {
		t.Errorf("well-formedness: %v, %v", err, errs)
	}
}
