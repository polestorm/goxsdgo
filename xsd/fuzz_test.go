package xsd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"
)

// Fuzzing (kryterium K5). FuzzCompile sprawdza od etapu 3: brak paniki, powtarzalność (dwa wywołania dają ten sam
// wynik), kod i pozycję w granicach dokumentu w każdym błędzie schematu oraz ograniczony czas. FuzzValidate dostanie
// niezmienniki w etapie 5.

const fuzzSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a">
    <xs:complexType><xs:sequence>
      <xs:element name="b" type="xs:decimal" maxOccurs="unbounded"/>
    </xs:sequence><xs:attribute name="c" type="xs:string"/></xs:complexType>
  </xs:element>
</xs:schema>`

func FuzzCompile(f *testing.F) {
	f.Add([]byte(fuzzSchema))
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="s.xsd"/></xs:schema>`))
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="p"><xs:restriction base="xs:string"><xs:pattern value="(a*)*b"/></xs:restriction></xs:simpleType></xs:schema>`))
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" xmlns:a="urn:a">
		<xs:redefine schemaLocation="s.xsd"><xs:group name="g"><xs:sequence><xs:group ref="a:g"/></xs:sequence></xs:group></xs:redefine>
		<xs:group name="g"><xs:choice><xs:element name="e" type="a:t"/></xs:choice></xs:group>
		<xs:complexType name="t"><xs:complexContent><xs:extension base="a:t"/></xs:complexContent></xs:complexType>
		<xs:element name="k"><xs:key name="k"><xs:selector xpath=".//a:x|y"/><xs:field xpath="@z"/></xs:key></xs:element>
		</xs:schema>`))
	f.Fuzz(func(t *testing.T, schema []byte) {
		fsys := fstest.MapFS{"s.xsd": {Data: schema}}
		start := time.Now()
		_, err1 := Compile(fsys, "s.xsd")
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("Compile took %v for %d bytes", d, len(schema))
		}
		_, err2 := Compile(fsys, "s.xsd")
		if fmt.Sprint(err1) != fmt.Sprint(err2) {
			t.Fatalf("Compile is not deterministic:\n%v\n%v", err1, err2)
		}
		var se *SchemaError
		if errors.As(err1, &se) {
			// Wiersze po normalizacji końców wierszy (XML 1.0, 2.11): CR LF i samo CR kończą wiersz.
			norm := bytes.ReplaceAll(bytes.ReplaceAll(schema, []byte("\r\n"), []byte("\n")), []byte("\r"), []byte("\n"))
			lines := bytes.Count(norm, []byte("\n")) + 1
			for _, e := range se.Errors {
				if e.Code == "" {
					t.Fatalf("schema error without a code: %v", e)
				}
				if e.Document == "s.xsd" && (e.Line < 0 || e.Line > lines || e.Column < 0) {
					t.Fatalf("schema error outside the document (%d lines): %v", lines, e)
				}
			}
		}
	})
}

const fuzzIdentitySchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r"><xs:complexType><xs:choice maxOccurs="unbounded">
    <xs:element name="i" nillable="true"><xs:complexType><xs:simpleContent><xs:extension base="xs:int">
      <xs:attribute name="id" type="xs:ID"/><xs:attribute name="ref" type="xs:IDREFS"/></xs:extension></xs:simpleContent></xs:complexType></xs:element>
    <xs:any namespace="##other" processContents="lax"/>
  </xs:choice><xs:anyAttribute processContents="lax"/></xs:complexType>
    <xs:key name="k"><xs:selector xpath=".//i"/><xs:field xpath="."/></xs:key>
    <xs:keyref name="kr" refer="k"><xs:selector xpath="i"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`

// FuzzValidate (K5) sprawdza walidację dokumentu: brak paniki, powtarzalność (dwa wywołania dają te same błędy),
// każdy błąd z kodem i pozycją w granicach dokumentu, ograniczony czas. REQ: VAL-019
func FuzzValidate(f *testing.F) {
	f.Add([]byte(fuzzSchema), []byte(`<a c="x"><b>1.5</b><b>2</b></a>`))
	f.Add([]byte(fuzzSchema), []byte(`<a><b>x</b></a>`))
	f.Add([]byte(fuzzSchema), []byte(`<!DOCTYPE a [<!ENTITY e SYSTEM "file:///etc/passwd">]><a>&e;</a>`))
	f.Add([]byte(fuzzIdentitySchema), []byte(`<r><i id="a">1</i><i ref="a">2</i><x:y xmlns:x="u"/></r>`))
	f.Add([]byte(fuzzIdentitySchema), []byte(`<r xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><i xsi:nil="true"/><i>1</i><i>1</i></r>`))
	f.Fuzz(func(t *testing.T, schema, doc []byte) {
		set, err := Compile(fstest.MapFS{"s.xsd": {Data: schema}}, "s.xsd")
		if err != nil {
			return
		}
		start := time.Now()
		errs1, err1 := set.Validate(bytes.NewReader(doc))
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("Validate took %v for %d bytes", d, len(doc))
		}
		errs2, err2 := set.Validate(bytes.NewReader(doc))
		if fmt.Sprint(errs1, err1) != fmt.Sprint(errs2, err2) {
			t.Fatalf("Validate is not deterministic:\n%v %v\n%v %v", errs1, err1, errs2, err2)
		}
		norm := bytes.ReplaceAll(bytes.ReplaceAll(doc, []byte("\r\n"), []byte("\n")), []byte("\r"), []byte("\n"))
		lines := bytes.Count(norm, []byte("\n")) + 1
		for _, e := range errs1 {
			if e.Code == "" || e.Message == "" {
				t.Fatalf("error without a code or message: %+v", e)
			}
			if e.Line < 0 || e.Line > lines || e.Column < 0 {
				t.Fatalf("error outside the document (%d lines): %+v", lines, e)
			}
		}
	})
}
