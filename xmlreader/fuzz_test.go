package xmlreader

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

// FuzzReader: dowolne wejście nie powoduje paniki, czytanie się kończy, a pozycje tokenów i błędów leżą
// w granicach wejścia (wiersz nie większy niż liczba wierszy, kolumna nie większa niż długość wiersza + 1).
// Strumień tokenów i błędów nie zależy od podziału wejścia na odczyty (po jednym bajcie i porcjami po 7 bajtów
// daje to samo co jeden odczyt; od 2026-11-26, wcześniej fuzzer podawał wejście jednym odczytem).
func FuzzReader(f *testing.F) {
	for _, s := range []string{
		"<a/>",
		"<?xml version='1.0' encoding='UTF-8'?><a x='1' xmlns:p='u'><p:b>t&amp;<![CDATA[c]]></p:b><!--c--><?pi d?></a>",
		`<!DOCTYPE a [<!ENTITY e "x<b>&f;</b>"><!ENTITY f "y"><!ATTLIST a d CDATA "v">]><a>&e;</a>`,
		"\xff\xfe<\x00a\x00/\x00>\x00",
		"<?xml version='1.0' encoding='ISO-8859-1'?><a>\xe9</a>",
		"<a>\r\n\r</a>",
	} {
		f.Add([]byte(s))
	}
	// Wejścia startowe z pobranych danych (ADR 0011): przykłady MF, schematy MF, próbka XML Conformance.
	for _, dir := range []struct {
		path string
		max  int
	}{{"../testdata/instances/mf", 20}, {"../testdata/schemas/mf", 30}, {"../testdata/w3c/xmlconf", 150}} {
		for _, b := range testkit.SampleFiles(dir.path, []string{".xml", ".xsd"}, dir.max, 128<<10) {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		whole := dumpStream(bytes.NewReader(data), Options{})
		for name, src := range map[string]io.Reader{
			"one byte at a time": iotest.OneByteReader(bytes.NewReader(data)),
			"7 bytes at a time":  &chunked{data: data, n: 7},
		} {
			if got := dumpStream(src, Options{}); got != whole {
				t.Fatalf("%s: stream differs from reading the whole input\n%s", name, firstDiff(got, whole))
			}
		}
		lines := bytes.Count(data, []byte{'\n'}) + bytes.Count(data, []byte{'\r'}) + 1
		longest := 0
		for _, l := range bytes.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' }) {
			// kolumna liczy znaki; w kodowaniach jednobajtowych i przy złym UTF-8 znaków nie jest więcej niż bajtów
			longest = max(longest, len(l), utf8.RuneCount(l))
		}
		if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
			// W UTF-16 bajt 0x0A albo 0x0D nie musi być końcem wiersza: granice liczymy w bajtach.
			lines, longest = len(data)+1, len(data)
		}
		check := func(l, c int) {
			if l < 1 || l > lines || c < 1 || c > longest+1 {
				t.Fatalf("position %d:%d outside input (%d lines, longest %d)", l, c, lines, longest)
			}
		}
		r := New(bytes.NewReader(data), Options{})
		for i := 0; ; i++ {
			if i > 2*len(data)+10 {
				t.Fatal("reader does not terminate")
			}
			tok, err := r.Next()
			if err == io.EOF {
				return
			}
			if err != nil {
				var e *Error
				if !errors.As(err, &e) {
					t.Fatalf("error of unexpected type: %v", err)
				}
				check(e.Line, e.Column)
				return
			}
			check(tok.Line, tok.Column)
		}
	})
}
