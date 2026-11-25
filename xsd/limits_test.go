package xsd

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// TestDocumentLimits: domyślne limity czytania dokumentu są w jednym miejscu (DefaultDocumentLimits) i zgadzają się
// z opisem w ADR 0011; Config.Document zmienia je dla jednego Set, a przekroczenie daje ErrLimit, nie błąd dokumentu.
// REQ: SAFE-001
func TestDocumentLimits(t *testing.T) {
	want := DocumentLimits{MaxDepth: 512, MaxNameLength: 16 << 10, MaxAttributes: 4096, MaxTextSize: 16 << 20,
		MaxEntityExpansion: 4 << 20, MaxEntityDepth: 32, MaxDTDSize: 1 << 20}
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
