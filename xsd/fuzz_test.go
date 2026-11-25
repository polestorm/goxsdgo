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

func FuzzValidate(f *testing.F) {
	f.Add([]byte(fuzzSchema), []byte(`<a c="x"><b>1.5</b><b>2</b></a>`))
	f.Add([]byte(fuzzSchema), []byte(`<a><b>x</b></a>`))
	f.Add([]byte(fuzzSchema), []byte(`<!DOCTYPE a [<!ENTITY e SYSTEM "file:///etc/passwd">]><a>&e;</a>`))
	f.Fuzz(func(t *testing.T, schema, doc []byte) {
		set, err := Compile(fstest.MapFS{"s.xsd": {Data: schema}}, "s.xsd")
		if err != nil {
			return
		}
		set.Validate(bytes.NewReader(doc))
	})
}
