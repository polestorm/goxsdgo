package xsd

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

// TestDocumentLimits: domyślne limity czytania dokumentu są w jednym miejscu (DefaultDocumentLimits) i zgadzają się
// z opisem w ADR 0011; Config.Document zmienia je dla jednego Set, a przekroczenie daje ErrLimit, nie błąd dokumentu.
// REQ: SAFE-001
func TestDocumentLimits(t *testing.T) {
	want := DocumentLimits{MaxDepth: 512, MaxNameLength: 16 << 10, MaxAttributes: 4096, MaxTextSize: 16 << 20,
		MaxEntityExpansion: 4 << 20, MaxEntityDepth: 32, MaxDTDSize: 1 << 20, MaxNamespaces: 256}
	if got := DefaultDocumentLimits(); got != want {
		t.Fatalf("DefaultDocumentLimits() = %+v, want %+v", got, want)
	}
	schema := fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:element name="e" type="t"/><xs:complexType name="t"><xs:sequence>
		<xs:element name="e" type="t" minOccurs="0"/></xs:sequence></xs:complexType></xs:schema>`)}}
	doc := strings.Repeat("<e>", 20) + strings.Repeat("</e>", 20)
	cases := []struct {
		cfg     Config
		limited bool
	}{
		{Config{}, false},
		{Config{Document: DocumentLimits{MaxDepth: 10}}, true},
		{Config{Document: DocumentLimits{MaxDepth: -1}}, false},
	}
	for _, c := range cases {
		set, err := c.cfg.Compile(schema, "s.xsd")
		if err != nil {
			t.Fatal(err)
		}
		errs, err := set.Validate(strings.NewReader(doc))
		if got := errors.Is(err, ErrLimit); got != c.limited {
			t.Errorf("%+v: ErrLimit = %v, want %v (errs %v)", c.cfg.Document, got, c.limited, errs)
		}
		if !c.limited && (err != nil || len(errs) != 0) {
			t.Errorf("%+v: %v %v", c.cfg.Document, err, errs)
		}
	}
}

// TestNamespaceLimit: limit deklaracji przestrzeni nazw w zasięgu (goxmlgo od v0.4.0, ADR 0015) działa domyślnie,
// daje ErrLimit z kodem limit-namespaces i daje się podnieść albo wyłączyć przez Config.Document.
// REQ: SAFE-001
func TestNamespaceLimit(t *testing.T) {
	schema := fstest.MapFS{"s.xsd": {Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:element name="e"><xs:complexType><xs:sequence>
		<xs:element ref="e" minOccurs="0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)}}
	// 300 deklaracji w zasięgu naraz: po 30 na każdym z 10 zagnieżdżonych elementów.
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("<e")
		for j := 0; j < 30; j++ {
			fmt.Fprintf(&b, ` xmlns:p%d_%d="urn:p%d"`, i, j, j)
		}
		b.WriteString(">")
	}
	b.WriteString(strings.Repeat("</e>", 10))
	doc := b.String()
	cases := []struct {
		limits  DocumentLimits
		limited bool
	}{
		{DocumentLimits{}, true},
		{DocumentLimits{MaxNamespaces: 299}, true},
		{DocumentLimits{MaxNamespaces: 300}, false},
		{DocumentLimits{MaxNamespaces: -1}, false},
	}
	for _, c := range cases {
		set, err := Config{Document: c.limits}.Compile(schema, "s.xsd")
		if err != nil {
			t.Fatal(err)
		}
		errs, err := set.Validate(strings.NewReader(doc))
		if got := errors.Is(err, ErrLimit); got != c.limited {
			t.Errorf("MaxNamespaces %d: ErrLimit = %v, want %v (errs %v)", c.limits.MaxNamespaces, got, c.limited, errs)
		}
		if c.limited && !strings.Contains(err.Error(), "limit-namespaces") {
			t.Errorf("MaxNamespaces %d: %v, want the code limit-namespaces", c.limits.MaxNamespaces, err)
		}
		if !c.limited && (err != nil || len(errs) != 0) {
			t.Errorf("MaxNamespaces %d: %v %v", c.limits.MaxNamespaces, err, errs)
		}
	}
}
