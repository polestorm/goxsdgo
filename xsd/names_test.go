package xsd

import (
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

// TestNameRulesOption sprawdza opcję Config.NameRules na małym schemacie: „Dĳkstra” (U+0133, litera spoza
// dodatku B XML 1.0 2e, NameStartChar w 5. wydaniu) jest NCName tylko z NamesXML10Edition5, także w nazwie
// komponentu schematu, w wartości atrybutu typu NCName i w klasie \i wzorca. REQ: DT-020, SCH-021
func TestNameRulesOption(t *testing.T) {
	const name = "Dĳkstra"
	schemaDoc := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="` + name + `"><xs:complexType>
    <xs:attribute name="n" type="xs:NCName"/>
    <xs:attribute name="p"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="\i\c*"/></xs:restriction></xs:simpleType></xs:attribute>
  </xs:complexType></xs:element>
  <xs:element name="plain"><xs:complexType>
    <xs:attribute name="n" type="xs:NCName"/>
    <xs:attribute name="p"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="\i\c*"/></xs:restriction></xs:simpleType></xs:attribute>
  </xs:complexType></xs:element>
</xs:schema>`
	fsys := fstest.MapFS{"s.xsd": {Data: []byte(schemaDoc)}}

	// Wydanie 2. (domyślne): nazwa elementu w schemacie nie jest NCName, schemat jest błędny.
	_, err := Compile(fsys, "s.xsd")
	var se *SchemaError
	if !errors.As(err, &se) {
		t.Fatalf("edition 2: schema with %s accepted (err %v)", name, err)
	}

	// Wydanie 5.: schemat poprawny; wartości atrybutu typu NCName i wzorca z \i przyjmowane.
	set5, err := Config{NameRules: NamesXML10Edition5}.Compile(fsys, "s.xsd")
	if err != nil {
		t.Fatalf("edition 5: %v", err)
	}
	doc := `<` + name + ` n="` + name + `" p="` + name + `"/>`
	errs, err := set5.Validate(strings.NewReader(doc))
	if err != nil || len(errs) != 0 {
		t.Fatalf("edition 5: document %q: %v %v", doc, errs, err)
	}

	// Wartości w dokumencie przy schemacie bez takiej nazwy: 2. wydanie odrzuca, 5. przyjmuje.
	plain := fstest.MapFS{"s.xsd": {Data: []byte(strings.Replace(schemaDoc, `<xs:element name="`+name+`">`, `<xs:element name="other">`, 1))}}
	doc2 := `<plain n="` + name + `" p="` + name + `"/>`
	set2, err := Compile(plain, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	errs, _ = set2.Validate(strings.NewReader(doc2))
	if len(errs) != 2 {
		t.Fatalf("edition 2: want 2 errors (NCName, pattern), got %v", errs)
	}
	set5b, _ := Config{NameRules: NamesXML10Edition5}.Compile(plain, "s.xsd")
	if errs, _ := set5b.Validate(strings.NewReader(doc2)); len(errs) != 0 {
		t.Fatalf("edition 5: %v", errs)
	}
}

// TestW3CSuiteNameRules5 mierzy zestaw W3C z opcją NamesXML10Edition5 (ADR 0012) i porównuje z ustawieniem
// domyślnym. Bez osobnej zapadki: K1 liczymy w ustawieniu domyślnym (TestW3CSuite); tu test wymaga tylko, żeby
// opcja zmieniała wyłącznie przypadki zależne od znaków nazw (grupa XmlVersions i wzorce z \i, \c), i raportuje
// liczby. REQ: DT-020
func TestW3CSuiteNameRules5(t *testing.T) {
	if _, err := os.Stat(w3cRoot + "/suite.xml"); err != nil {
		t.Skip(testkit.FetchHint)
	}
	cases, err := w3c.Load(os.DirFS(w3cRoot), w3c.Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	src, err := testkit.LoadSources(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	const extra = "_w3c-schemas"
	fsys := testkit.Overlay(os.DirFS(w3cRoot), extra, os.DirFS(w3cSchemas))
	base := Config{Locations: testkit.PrefixLocations(src.Locations("w3c-schemas"), extra)}
	schemaExpected := map[string]w3c.Validity{}
	for _, c := range cases {
		if c.Kind == w3c.SchemaTest {
			schemaExpected[c.Set+"/"+c.Group] = c.Expected
		}
	}
	run := func(cfg Config) map[string]bool {
		passed := map[string]bool{}
		schemas := map[string]compiled{}
		obs := newCodeObserver()
		for _, c := range cases {
			if !c.InScope() {
				continue
			}
			if got := runW3CCase(cfg, fsys, c, schemas, schemaExpected, obs); got != "" && c.Matches(got) {
				passed[c.ID] = true
			}
		}
		return passed
	}
	p2 := run(base)
	cfg5 := base
	cfg5.NameRules = NamesXML10Edition5
	p5 := run(cfg5)

	var gained, lost []string
	inScope := 0
	for _, c := range cases {
		if !c.InScope() {
			continue
		}
		inScope++
		switch {
		case p5[c.ID] && !p2[c.ID]:
			gained = append(gained, c.ID)
		case p2[c.ID] && !p5[c.ID]:
			lost = append(lost, c.ID)
		}
	}
	sort.Strings(gained)
	sort.Strings(lost)
	t.Logf("W3C, names by XML 1.0 2e (default): %d / %d; by 5e: %d / %d; gained %d, lost %d",
		len(p2), inScope, len(p5), inScope, len(gained), len(lost))
	for _, id := range gained {
		t.Logf("  gained %s", id)
	}
	for _, id := range lost {
		t.Logf("  lost   %s", id)
	}
	for _, id := range append(append([]string{}, gained...), lost...) {
		if !strings.HasPrefix(id, "XmlVersions/") && !strings.HasPrefix(id, "MS-Regex") {
			t.Errorf("NameRules changed a case outside name-character groups: %s", id)
		}
	}
}
