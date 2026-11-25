package xsd

import (
	"bytes"
	"testing"
	"testing/fstest"
)

// Szkielet fuzzingu (kryterium K5). Dziś sprawdza tylko brak paniki; po etapach 3 i 5 dojdą niezmienniki
// (powtarzalność wyniku, pozycje błędów w granicach dokumentu, czas ograniczony).

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
	f.Fuzz(func(t *testing.T, schema []byte) {
		fsys := fstest.MapFS{"s.xsd": {Data: schema}}
		Compile(fsys, "s.xsd")
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
