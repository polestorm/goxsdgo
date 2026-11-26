package xmlreader

import (
	"io"
	"strings"
	"testing"
)

func TestDeclaredNamespaces(t *testing.T) {
	r := New(strings.NewReader(`<a xmlns="u:d" xmlns:p="u:p"><b xmlns:q="u:q" xmlns=""/><c/></a>`), Options{})
	var got []string
	for {
		tok, err := r.Next()
		if err != nil {
			break
		}
		if tok.Kind == KindStartElement {
			s := tok.Name.Local + ":"
			for _, b := range r.DeclaredNamespaces() {
				s += " " + b.Prefix + "=" + b.URI
			}
			got = append(got, s)
		}
	}
	want := []string{"a: =u:d p=u:p", "b: q=u:q =", "c:"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestWhitespaceAcrossBufferBoundary: tekst z samych białych znaków, który przecina granicę bufora wejścia
// (64 KiB), ma flagę Whitespace. REQ: XML-009, XML-017
func TestWhitespaceAcrossBufferBoundary(t *testing.T) {
	// Długie odcinki białych znaków przeplatane elementami: na kilku granicach bufora wypadnie środek odcinka.
	for _, run := range []int{7, 61, 509} {
		var b strings.Builder
		b.WriteString("<a>")
		for b.Len() < 4*chunkSize {
			b.WriteString("<e/>")
			b.WriteString("\n" + strings.Repeat("\t", run))
		}
		b.WriteString("</a>")
		r := New(strings.NewReader(b.String()), Options{})
		for {
			tok, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if tok.Kind == KindText && strings.TrimSpace(string(tok.Data)) == "" && !tok.Whitespace {
				t.Fatalf("run %d: whitespace text at %d:%d without the Whitespace flag", run, tok.Line, tok.Column)
			}
		}
	}
}
