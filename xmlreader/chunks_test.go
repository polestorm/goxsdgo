package xmlreader

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"testing/iotest"
)

// chunked oddaje dane porcjami po n bajtów.
type chunked struct {
	data []byte
	n    int
}

func (c *chunked) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	k := min(c.n, len(c.data), len(p))
	copy(p, c.data[:k])
	c.data = c.data[k:]
	return k, nil
}

// dumpStream zapisuje cały strumień tokenów (z pozycjami, atrybutami i ich typami) i kończący go błąd (kod,
// wiersz, kolumna); panikę zamienia na opis, żeby test pokazał wszystkie przypadki naraz.
func dumpStream(src io.Reader, opts Options) (out string) {
	var b strings.Builder
	defer func() {
		if p := recover(); p != nil {
			out = b.String() + fmt.Sprintf("PANIC: %v\n", p)
		}
	}()
	r := New(src, opts)
	for {
		t, err := r.Next()
		if err == io.EOF {
			b.WriteString("eof\n")
			return b.String()
		}
		if err != nil {
			if e, ok := err.(*Error); ok {
				fmt.Fprintf(&b, "error %s %d:%d\n", e.Code, e.Line, e.Column)
			} else {
				fmt.Fprintf(&b, "error %v\n", err)
			}
			return b.String()
		}
		fmt.Fprintf(&b, "%d:%d kind=%d %+q %+q cdata=%v ws=%v self=%v end=%d:%d\n", t.Line, t.Column, t.Kind,
			t.Name, t.Data, t.CDATA, t.Whitespace, t.SelfClosing, t.EndLine, t.EndColumn)
		for _, a := range t.Attrs {
			fmt.Fprintf(&b, "  @%+q=%+q type=%s specified=%v %d:%d\n", a.Name, a.Value, a.Type, a.Specified, a.Line, a.Column)
		}
		if t.Kind == KindStartElement {
			for _, ns := range r.DeclaredNamespaces() {
				fmt.Fprintf(&b, "  ns %+q=%+q\n", ns.Prefix, ns.URI)
			}
		}
	}
}

func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			return fmt.Sprintf("line %d:\n got  %s\n want %s", i+1, a, b)
		}
	}
	return ""
}

// TestChunkedInput: strumień tokenów i błędów nie zależy od podziału wejścia na odczyty. Każdy przypadek W3C XML
// Conformance w zakresie (z ustawieniem przestrzeni nazw jak w TestXMLConformance) jest czytany w całości, po
// jednym bajcie i porcjami po 7 bajtów.
// REQ: XML-019
func TestChunkedInput(t *testing.T) {
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip()
	}
	bad, n := 0, 0
	for _, ct := range loadConformance(t) {
		if ct.excluded != "" {
			continue
		}
		data, err := os.ReadFile(path.Join(xmlconfRoot, ct.dir, ct.URI))
		if err != nil {
			t.Fatal(err)
		}
		n++
		opts := Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")}
		want := dumpStream(bytes.NewReader(data), opts)
		for name, src := range map[string]io.Reader{
			"one byte at a time": iotest.OneByteReader(bytes.NewReader(data)),
			"7 bytes at a time":  &chunked{data: data, n: 7},
		} {
			if got := dumpStream(src, opts); got != want {
				bad++
				if bad <= 15 {
					t.Errorf("%s/%s, %s: stream differs from reading the whole input\n%s", ct.Group, ct.ID, name, firstDiff(got, want))
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d differences in %d cases", bad, n)
	}
}

// TestXMLDeclSmallDocument: deklaracja XML w dokumencie krótszym niż nazwa następnego pseudoatrybutu, z białymi
// znakami przed „?>” i bez nich, daje ten sam wynik w całości, po jednym bajcie i porcjami po 3 bajty; brak wersji
// jest zgłaszany przed białymi znakami.
// REQ: XML-019 XML-005
func TestXMLDeclSmallDocument(t *testing.T) {
	for doc, want := range map[string]string{
		`<?xml version="1.0"?><a/>`:                   "eof",
		`<?xml version="1.0" ?><a/>`:                  "eof",
		`<?xml version="1.0" ?><abc/>`:                "eof",
		`<?xml version="1.0" encoding="UTF-8" ?><a/>`: "eof",
		"<?xml version='1.0'\n\t?><a/>":               "eof",
		`<?xml version="1.0" ?>`:                      "error xml-no-root 1:23",
		`<?xml version="1.0" `:                        "error xml-eof 1:21",
		`<?xml version="1.0" x?><a/>`:                 "error xml-decl 1:21",
		`<?xml ?><a/>`:                                "error xml-decl 1:6",
	} {
		for name, src := range map[string]io.Reader{
			"whole":       bytes.NewReader([]byte(doc)),
			"one byte":    iotest.OneByteReader(bytes.NewReader([]byte(doc))),
			"three bytes": &chunked{data: []byte(doc), n: 3},
		} {
			got := dumpStream(src, Options{})
			if last := got[strings.LastIndexByte(got[:len(got)-1], '\n')+1:]; last != want+"\n" {
				t.Errorf("%q (%s): %q, want %q", doc, name, last, want)
			}
		}
	}
}
