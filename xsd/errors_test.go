package xsd

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

const errSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t"
  elementFormDefault="qualified">
  <xs:element name="Doc">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="Kod" type="Kod"/>
        <xs:element name="Kwota" type="xs:decimal" maxOccurs="unbounded"/>
        <xs:element name="Ref" minOccurs="0"><xs:complexType><xs:attribute name="to" type="xs:IDREF"/></xs:complexType></xs:element>
      </xs:sequence>
      <xs:attribute name="id" type="xs:ID"/>
      <xs:attribute name="rok" type="xs:gYear"/>
    </xs:complexType>
  </xs:element>
  <xs:simpleType name="Kod"><xs:restriction base="xs:string"><xs:pattern value="[A-Z]{3}"/></xs:restriction></xs:simpleType>
</xs:schema>`

func compileErrSchema(t *testing.T, cfg Config) *Set {
	t.Helper()
	set, err := cfg.Compile(fstest.MapFS{"s.xsd": {Data: []byte(errSchema)}}, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// TestErrorFields: każdy błąd ma kod z katalogu, pozycję od 1, ścieżkę elementu i parametry z katalogu; błąd
// schematu ma dokument i pozycję w nim. REQ: ERR-001, ERR-002, ERR-003, ERR-004
func TestErrorFields(t *testing.T) {
	set := compileErrSchema(t, Config{})
	doc := `<Doc xmlns="urn:t" rok="20x6">
  <Kod>ab1</Kod>
  <Kwota>1.5</Kwota>
  <Kwota>x</Kwota>
  <Obcy/>
</Doc>`
	errs, err := set.Validate(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ code, path string }{
		{"cvc-datatype-valid.1.2.1", "/Doc/@rok"},
		{"cvc-pattern-valid", "/Doc/Kod"},
		{"cvc-datatype-valid.1.2.1", "/Doc/Kwota[2]"},
		{"cvc-complex-type.2.4", "/Doc/Obcy"},
	}
	if len(errs) != len(want) {
		t.Fatalf("got %d errors, want %d: %v", len(errs), len(want), errs)
	}
	for i, w := range want {
		e := errs[i]
		if e.Code != w.code || e.Path != w.path || e.Line < 1 || e.Column < 1 || e.Message == "" {
			t.Errorf("error %d: %s %s at %d:%d, want %s %s", i, e.Code, e.Path, e.Line, e.Column, w.code, w.path)
		}
		info, ok := LookupCode(e.Code)
		if !ok || info.Kind != KindDocument {
			t.Errorf("error %d: code %s not a document code in the catalog", i, e.Code)
		}
	}
	if errs[1].Facet == "" || errs[1].Value != "ab1" {
		t.Errorf("pattern error without facet or value: %+v", errs[1])
	}
	if len(errs[3].Expected) == 0 || errs[3].Expected[0] != "{urn:t}Kwota" {
		t.Errorf("content error expected list %v", errs[3].Expected)
	}

	_, err = Compile(fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="nope"/></xs:schema>`)}}, "s.xsd")
	se, ok := err.(*SchemaError)
	if !ok || len(se.Errors) == 0 {
		t.Fatalf("schema error expected, got %v", err)
	}
	if e := se.Errors[0]; e.Code != "src-resolve" || e.Document != "s.xsd" || e.Line != 2 || e.Column < 1 {
		t.Errorf("schema error %+v", e)
	}
}

// TestErrorMessages: komunikaty są zdaniami po angielsku bez kodu na początku; nazwy z przestrzeni elementu
// głównego są bez przestrzeni, a błąd modelu zawartości mówi, czego oczekiwano. REQ: ERR-007
func TestErrorMessages(t *testing.T) {
	set := compileErrSchema(t, Config{})
	doc := `<Doc xmlns="urn:t"><Kwota>1</Kwota></Doc>`
	errs, _ := set.Validate(strings.NewReader(doc))
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
	if got, want := errs[0].Message, "unexpected element Kwota in Doc; expected Kod"; got != want {
		t.Errorf("message %q, want %q", got, want)
	}
	errs, _ = set.Validate(strings.NewReader(`<Doc xmlns="urn:t"><Kod>abc</Kod><Kwota>1</Kwota></Doc>`))
	if len(errs) != 1 || errs[0].Message != `element Kod: value "abc" does not match the pattern [A-Z]{3}` {
		t.Errorf("got %v", errs)
	}
	for _, e := range errs {
		if strings.HasPrefix(e.Message, e.Code) || strings.Contains(e.Message, "{urn:t}") {
			t.Errorf("message %q repeats the code or the root namespace", e.Message)
		}
	}
}

// TestErrorOrder: błędy są w kolejności wykrycia w jednym przejściu; nierozwiązane IDREF (cvc-id.1) są zgłaszane
// na końcu dokumentu, bo dopiero wtedy wiadomo, że ID nie wystąpi. REQ: ERR-005
func TestErrorOrder(t *testing.T) {
	set := compileErrSchema(t, Config{})
	doc := `<Doc xmlns="urn:t" id="a1">
  <Kod>ab</Kod>
  <Kwota>-</Kwota>
  <Ref to="b2"/>
</Doc>`
	errs, _ := set.Validate(strings.NewReader(doc))
	var codes []string
	for _, e := range errs {
		codes = append(codes, e.Code)
	}
	if got, want := strings.Join(codes, " "), "cvc-pattern-valid cvc-datatype-valid.1.2.1 cvc-id.1"; got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
	if errs[0].Line != 2 || errs[1].Line != 3 || errs[2].Line != 4 {
		t.Errorf("lines %d %d %d", errs[0].Line, errs[1].Line, errs[2].Line)
	}
}

// TestCompatLookup: odpowiedniki kodów libxml2 dla najczęstszych błędów korpusu MF. REQ: ERR-006
func TestCompatLookup(t *testing.T) {
	for code, want := range map[string]int{
		"cvc-complex-type.2.4": 1871, "cvc-pattern-valid": 1839, "cvc-datatype-valid.1.2.1": 1824,
		"cvc-maxInclusive-valid": 1834, "cvc-complex-type.3.2.1": 1866, "wfc-element-type-match": 76,
	} {
		if got, ok := LibXML2Code(code); !ok || got != want {
			t.Errorf("LibXML2Code(%s) = %d, %v; want %d", code, got, ok, want)
		}
	}
	if _, ok := LibXML2Code("limit-depth"); ok {
		t.Error("limit codes must not have a libxml2 counterpart")
	}
}

// TestTypedValues: Config.OnValue dostaje poprawne wartości typów prostych z typem, typem wbudowanym i postacią
// kanoniczną, w kolejności dokumentu; wartości błędne nie trafiają do funkcji. REQ: ERR-008
func TestTypedValues(t *testing.T) {
	var got []TypedValue
	set := compileErrSchema(t, Config{OnValue: func(v TypedValue) { got = append(got, v) }})
	doc := `<Doc xmlns="urn:t" rok="2026"><Kod>ABC</Kod><Kwota> 012.50 </Kwota><Kwota>x</Kwota></Doc>`
	if _, err := set.Validate(strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	want := []TypedValue{
		{Path: "/Doc/@rok", Name: "rok", Attribute: true, Type: "{http://www.w3.org/2001/XMLSchema}gYear", Builtin: "gYear", Canonical: "2026"},
		{Path: "/Doc/Kod", Name: "Kod", Type: "{urn:t}Kod", Builtin: "string", Canonical: "ABC"},
		{Path: "/Doc/Kwota", Name: "Kwota", Type: "{http://www.w3.org/2001/XMLSchema}decimal", Builtin: "decimal", Canonical: "12.5"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d values, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		g.Line, g.Column = 0, 0
		if g != w {
			t.Errorf("value %d: %+v, want %+v", i, g, w)
		}
	}
}

// BenchmarkValidateJPKOnValue mierzy koszt Config.OnValue na syntetycznym JPK (porównanie z BenchmarkValidateJPK).
func BenchmarkValidateJPKOnValue(b *testing.B) {
	src := compileJPK(b)
	n := 0
	set := &Set{schema: src.schema, cfg: Config{OnValue: func(TypedValue) { n++ }}, fsys: src.fsys, names: src.names}
	doc, err := io.ReadAll(newJPKReader(loadJPKParts(b), 20<<20))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if errs, err := set.Validate(bytes.NewReader(doc)); err != nil || len(errs) != 0 {
			b.Fatal(err, errs)
		}
	}
	if n == 0 {
		b.Fatal("OnValue not called")
	}
}
