package xmlreader

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestManyPrefixedAttributes: przy wielu atrybutach z prefiksem sprawdzenie NSC Attributes Unique używa zbioru
// (etap 7, liniowo), a powtórzenie nazwy rozwiniętej jest dalej wykrywane, także przez dwa różne prefiksy tej samej
// przestrzeni. REQ: XML-005, DET-001
func TestManyPrefixedAttributes(t *testing.T) {
	build := func(n int, dup string) string {
		var b strings.Builder
		b.WriteString(`<e xmlns:p="urn:x" xmlns:q="urn:x"`)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, ` p:a%d="v"`, i)
		}
		b.WriteString(dup + "/>")
		return b.String()
	}
	read := func(doc string) error {
		r := New(strings.NewReader(doc), Options{})
		for {
			_, err := r.Next()
			if err != nil {
				if err.Error() == "EOF" {
					return nil
				}
				return err
			}
		}
	}
	if err := read(build(100, "")); err != nil {
		t.Fatalf("100 distinct attributes: %v", err)
	}
	for _, dup := range []string{` q:a3="w"`, ` q:a99="w"`} {
		err := read(build(100, dup))
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeAttributesUnique {
			t.Errorf("duplicate%s: err = %v, want %s", dup, err, CodeAttributesUnique)
		}
	}
	// Kolejny element zaczyna z pustym zbiorem.
	doc := `<r xmlns:p="urn:x">` + strings.TrimSuffix(build(40, ""), "/>") + "/>" + strings.TrimSuffix(build(40, ""), "/>") + "/></r>"
	if err := read(doc); err != nil {
		t.Errorf("two elements with the same 40 attributes: %v", err)
	}
}
