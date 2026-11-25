package xmlreader

import (
	"strings"
	"testing"
)

// TestStartTagEndPosition: znacznik początkowy na kilku wierszach podaje pozycję początku i końca. REQ: XML-017
func TestStartTagEndPosition(t *testing.T) {
	r := New(strings.NewReader("<a\n  x='1'\n>t<b/></a>"), Options{})
	tok, err := r.Next()
	if err != nil || tok.Line != 1 || tok.Column != 1 || tok.EndLine != 3 || tok.EndColumn != 2 {
		t.Fatalf("a: %+v %v", tok, err)
	}
	r.Next() // tekst
	tok, err = r.Next()
	if err != nil || tok.Name.Local != "b" || tok.EndLine != 3 || tok.EndColumn != 7 {
		t.Fatalf("b: %+v %v", tok, err)
	}
}
