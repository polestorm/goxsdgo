package xsd

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// Schemat w kształcie nagłówka JPK: wystarcza do czterech przypadków, które xml.pro rozpoznaje dziś z tekstu
// komunikatu libxml2 (brak elementu, element niedozwolony, zła przestrzeń nazw, pusta wartość typu TData).
const detailsSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:jpk" xmlns="urn:jpk" elementFormDefault="qualified">
 <xs:simpleType name="TData"><xs:restriction base="xs:date"/></xs:simpleType>
 <xs:element name="JPK"><xs:complexType><xs:sequence>
  <xs:element name="Naglowek"><xs:complexType><xs:sequence>
    <xs:element name="Rok" type="xs:gYear"/>
    <xs:element name="Miesiac"><xs:simpleType><xs:restriction base="xs:int"><xs:maxInclusive value="12"/></xs:restriction></xs:simpleType></xs:element>
    <xs:element name="DataWytworzenia" type="TData"/>
    <xs:choice minOccurs="0"><xs:element name="KodA" type="xs:string"/><xs:element name="KodB" type="xs:string"/></xs:choice>
  </xs:sequence></xs:complexType></xs:element>
 </xs:sequence></xs:complexType></xs:element></xs:schema>`

func detailsSet(t *testing.T) *Set {
	t.Helper()
	set, err := Compile(fstest.MapFS{"s.xsd": {Data: []byte(detailsSchema)}}, "s.xsd")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func validateOne(t *testing.T, set *Set, doc string) Error {
	t.Helper()
	errs, err := set.Validate(strings.NewReader(doc))
	if err != nil || len(errs) != 1 {
		t.Fatalf("want exactly one error, got %v, %v", errs, err)
	}
	return errs[0]
}

// col zwraca kolumnę (od 1) pierwszego wystąpienia s w jednowierszowym dokumencie ASCII.
func col(t *testing.T, doc, s string) int {
	t.Helper()
	i := strings.Index(doc, s)
	if i < 0 {
		t.Fatalf("%q not in document", s)
	}
	return i + 1
}

// TestErrorDetails sprawdza pola dodane w etapie 6b na czterech przypadkach z xml.pro.
// REQ: ERR-009, ERR-010, ERR-011, ERR-012
func TestErrorDetails(t *testing.T) {
	set := detailsSet(t)

	t.Run("missing child", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok>2026</Rok><Miesiac>1</Miesiac></Naglowek></JPK>`
		e := validateOne(t, set, doc)
		want := Error{Code: "cvc-complex-type.2.4", Reason: ReasonMissing, Name: "Naglowek", Namespace: "urn:jpk",
			Path: "/JPK/Naglowek", Expected: []string{"{urn:jpk}DataWytworzenia"},
			Line: 1, Column: col(t, doc, "<Rok>"), InsertLine: 1, InsertColumn: col(t, doc, "</Naglowek>")}
		e.Message = ""
		if !reflect.DeepEqual(e, want) {
			t.Fatalf("got  %+v\nwant %+v", e, want)
		}
	})

	t.Run("missing child inside an empty-element tag", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek/></JPK>`
		e := validateOne(t, set, doc)
		if e.Reason != ReasonMissing || e.InsertLine != 1 || e.InsertColumn != col(t, doc, "</JPK>") {
			t.Fatalf("got %+v", e)
		}
	})

	t.Run("unexpected element", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok>2026</Rok><Dzien>1</Dzien></Naglowek></JPK>`
		e := validateOne(t, set, doc)
		want := Error{Code: "cvc-complex-type.2.4", Reason: ReasonUnexpected, Name: "Dzien", Namespace: "urn:jpk",
			Path: "/JPK/Naglowek/Dzien", Expected: []string{"{urn:jpk}Miesiac"},
			Line: 1, Column: col(t, doc, "1</Dzien>"), InsertLine: 1, InsertColumn: col(t, doc, "<Dzien>")}
		e.Message = ""
		if !reflect.DeepEqual(e, want) {
			t.Fatalf("got  %+v\nwant %+v", e, want)
		}
	})

	t.Run("wrong namespace", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok xmlns="urn:inny">2026</Rok></Naglowek></JPK>`
		e := validateOne(t, set, doc)
		if e.Reason != ReasonWrongNamespace || e.Name != "Rok" || e.Namespace != "urn:inny" || e.ExpectedNamespace != "urn:jpk" ||
			e.InsertColumn != col(t, doc, "<Rok") {
			t.Fatalf("got %+v", e)
		}
	})

	t.Run("empty value of a named type", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok>2026</Rok><Miesiac>1</Miesiac><DataWytworzenia/></Naglowek></JPK>`
		e := validateOne(t, set, doc)
		if e.Code != "cvc-datatype-valid.1.2.1" || e.Type != "TData" || e.TypeNamespace != "urn:jpk" || e.Builtin != "date" ||
			e.Value != "" || e.Name != "DataWytworzenia" || e.Namespace != "urn:jpk" || e.Reason != "" || e.InsertLine != 0 {
			t.Fatalf("got %+v", e)
		}
	})

	t.Run("facet on an anonymous type", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok>2026</Rok><Miesiac>13</Miesiac><DataWytworzenia>2026-01-01</DataWytworzenia></Naglowek></JPK>`
		e := validateOne(t, set, doc)
		if e.Code != "cvc-maxInclusive-valid" || e.Type != "" || e.Builtin != "int" || e.Value != "13" || e.Facet != "maxInclusive 12" {
			t.Fatalf("got %+v", e)
		}
	})

	t.Run("missing one of a choice is not reported when optional", func(t *testing.T) {
		doc := `<JPK xmlns="urn:jpk"><Naglowek><Rok>2026</Rok><Miesiac>1</Miesiac><DataWytworzenia>2026-01-01</DataWytworzenia></Naglowek></JPK>`
		if errs, err := set.Validate(strings.NewReader(doc)); err != nil || len(errs) != 0 {
			t.Fatalf("got %v, %v", errs, err)
		}
	})
}
