package xsd

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// TestPathological (kryterium K5, ADR 0011): wejścia patologiczne dla całej biblioteki — wzorce, liczności,
// zagnieżdżenia, długie atrybuty i teksty, tysiące atrybutów, kluczy i wartości wyliczenia, długie listy, wiele
// symboli wieloznacznych, encje wewnętrzne. Każdy przypadek mierzy dwa rozmiary (n i 2n) i sprawdza, że czas rośnie
// liniowo: iloraz czasów nie przekracza progu (2 to wzrost liniowy, 4 kwadratowy). Przypadki ponad limitem kończą
// się błędem ErrLimit w ograniczonym czasie, nie paniką. Modele zawartości z setkami tysięcy powtórzeń bada też
// internal/schema TestPathological. REQ: DET-001, DET-002, SAFE-003
func TestPathological(t *testing.T) {
	const ratioLimit = 3.2
	type row struct {
		name   string
		n      int
		ratio  float64
		tSmall time.Duration
	}
	var rows []row
	linear := func(name string, n int, build func(n int) (schema, doc string)) {
		t.Helper()
		measure := func(n int) time.Duration {
			schema, doc := build(n)
			set, err := Compile(fstest.MapFS{"s.xsd": {Data: []byte(schema)}}, "s.xsd")
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			best := time.Duration(1 << 62)
			// Najlepszy z 5 przebiegów, bez odśmiecania w trakcie pomiaru: na współdzielonych maszynach CI
			// odśmiecanie w przebiegu 2n zawyżało iloraz ponad próg, choć walidacja jest liniowa.
			defer debug.SetGCPercent(debug.SetGCPercent(-1))
			for i := 0; i < 5; i++ {
				runtime.GC()
				start := time.Now()
				errs, err := set.Validate(strings.NewReader(doc))
				d := time.Since(start)
				if err != nil || len(errs) != 0 {
					t.Fatalf("%s (n=%d): %v %v", name, n, err, errs[:min(2, len(errs))])
				}
				best = min(best, d)
			}
			return best
		}
		// Do trzech prób: prawdziwy wzrost nieliniowy przekracza próg za każdym razem, szum maszyny współdzielonej
		// (np. testy innych pakietów w tym samym czasie) zwykle tylko raz.
		var a, b time.Duration
		var r float64
		for try := 0; try < 3; try++ {
			a, b = measure(n), measure(2*n)
			r = float64(b) / float64(max(a, time.Microsecond))
			if r <= ratioLimit || b <= 20*time.Millisecond {
				break
			}
		}
		rows = append(rows, row{name, n, r, a})
		if r > ratioLimit && b > 20*time.Millisecond {
			t.Errorf("%s: time grows faster than linearly: n=%d %v, 2n %v (ratio %.2f)", name, n, a, b, r)
		}
	}
	const xs = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`

	linear("pattern (a|aa)*b, long value", 100000, func(n int) (string, string) {
		return xs + `<xs:element name="e"><xs:simpleType><xs:restriction base="xs:string">
			<xs:pattern value="(a|aa)*b"/></xs:restriction></xs:simpleType></xs:element></xs:schema>`,
			"<e>" + strings.Repeat("a", n) + "b</e>"
	})
	linear("pattern ([a-z]+)*[0-9]{1,3}, long value", 100000, func(n int) (string, string) {
		return xs + `<xs:element name="e"><xs:simpleType><xs:restriction base="xs:string">
			<xs:pattern value="([a-z]+)*[0-9]{1,3}"/></xs:restriction></xs:simpleType></xs:element></xs:schema>`,
			"<e>" + strings.Repeat("ab", n/2) + "7</e>"
	})
	linear("maxOccurs 500000, many children", 100000, func(n int) (string, string) {
		return xs + `<xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="b" type="xs:int" maxOccurs="500000"/></xs:sequence></xs:complexType></xs:element></xs:schema>`,
			"<r>" + strings.Repeat("<b>1</b>", n) + "</r>"
	})
	linear("nested counters (a{2,2} b?){1,50000}", 20000, func(n int) (string, string) {
		return xs + `<xs:element name="r"><xs:complexType><xs:sequence minOccurs="1" maxOccurs="50000">
			<xs:element name="a" minOccurs="2" maxOccurs="2"/><xs:element name="b" minOccurs="0"/></xs:sequence>
			</xs:complexType></xs:element></xs:schema>`,
			"<r>" + strings.Repeat("<a/><a/><b/>", n) + "</r>"
	})
	linear("deep nesting (recursive type)", 200, func(n int) (string, string) {
		return xs + `<xs:element name="e" type="t"/><xs:complexType name="t"><xs:sequence>
			<xs:element name="e" type="t" minOccurs="0"/></xs:sequence></xs:complexType></xs:schema>`,
			strings.Repeat("<e>", n) + strings.Repeat("</e>", n)
	})
	linear("long attribute value", 1<<20, func(n int) (string, string) {
		return xs + `<xs:element name="e"><xs:complexType><xs:attribute name="a" type="xs:string"/></xs:complexType>
			</xs:element></xs:schema>`, `<e a="` + strings.Repeat("x", n) + `"/>`
	})
	linear("long text (normalizedString)", 2<<20, func(n int) (string, string) {
		return xs + `<xs:element name="e" type="xs:normalizedString"/></xs:schema>`,
			"<e>" + strings.Repeat("ab cd ", n/6) + "</e>"
	})
	linear("thousands of attributes (wildcard, prefixed)", 1000, func(n int) (string, string) {
		var b strings.Builder
		b.WriteString(`<e xmlns:p="urn:p"`)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, ` p:a%d="v"`, i)
		}
		b.WriteString("/>")
		return xs + `<xs:element name="e"><xs:complexType><xs:anyAttribute namespace="##any" processContents="skip"/>
			</xs:complexType></xs:element></xs:schema>`, b.String()
	})
	linear("thousands of keys and keyrefs", 20000, func(n int) (string, string) {
		var b strings.Builder
		b.WriteString("<r>")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `<i id="k%d" ref="k%d"/>`, i, (i*7)%n)
		}
		b.WriteString("</r>")
		return xs + `<xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="i" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:string"/>
			<xs:attribute name="ref" type="xs:string"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
			<xs:key name="k"><xs:selector xpath="i"/><xs:field xpath="@id"/></xs:key>
			<xs:keyref name="kr" refer="k"><xs:selector xpath="i"/><xs:field xpath="@ref"/></xs:keyref>
			</xs:element></xs:schema>`, b.String()
	})
	linear("ID and IDREFS", 20000, func(n int) (string, string) {
		var b strings.Builder
		b.WriteString("<r>")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `<i id="k%d" ref="k%d k%d"/>`, i, (i*7)%n, (i*3)%n)
		}
		b.WriteString("</r>")
		return xs + `<xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="i" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:ID"/>
			<xs:attribute name="ref" type="xs:IDREFS"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
			</xs:element></xs:schema>`, b.String()
	})
	linear("large enumeration, many values", 5000, func(n int) (string, string) {
		var s, d strings.Builder
		s.WriteString(xs + `<xs:simpleType name="t"><xs:restriction base="xs:token">`)
		for i := 0; i < 5000; i++ {
			fmt.Fprintf(&s, `<xs:enumeration value="v%d"/>`, i)
		}
		s.WriteString(`</xs:restriction></xs:simpleType><xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="e" type="t" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
		d.WriteString("<r>")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&d, "<e>v%d</e>", (i*13)%5000)
		}
		d.WriteString("</r>")
		return s.String(), d.String()
	})
	linear("long list of decimals", 100000, func(n int) (string, string) {
		return xs + `<xs:element name="e"><xs:simpleType><xs:list itemType="xs:decimal"/></xs:simpleType>
			</xs:element></xs:schema>`, "<e>" + strings.Repeat("12.50 ", n) + "</e>"
	})
	linear("many wildcards in a choice", 20000, func(n int) (string, string) {
		var s, d strings.Builder
		s.WriteString(xs + `<xs:element name="r"><xs:complexType><xs:choice maxOccurs="unbounded">`)
		for i := 0; i < 300; i++ {
			fmt.Fprintf(&s, `<xs:any namespace="urn:w%d" processContents="skip"/>`, i)
		}
		s.WriteString(`</xs:choice></xs:complexType></xs:element></xs:schema>`)
		d.WriteString("<r>")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&d, `<x xmlns="urn:w%d"/>`, (i*17)%300)
		}
		d.WriteString("</r>")
		return s.String(), d.String()
	})
	linear("internal entities, expansion below the limit", 50000, func(n int) (string, string) {
		return xs + `<xs:element name="e" type="xs:string"/></xs:schema>`,
			`<!DOCTYPE e [<!ENTITY a "abcdefghij">]><e>` + strings.Repeat("&a;", n) + `</e>`
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%-48s %8s %10s %6s\n", "case", "n", "t(n)", "t(2n)/t(n)")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-48s %8d %10v %6.2f\n", r.name, r.n, r.tSmall.Round(10*time.Microsecond), r.ratio)
	}
	t.Log("\n" + b.String())

	// Ponad limitem: błąd limitu w ograniczonym czasie, bez paniki.
	beyond := []struct {
		name   string
		schema string
		doc    []byte
		cfg    Config
	}{
		{"nesting beyond MaxDepth", xs + `<xs:element name="e" type="t"/><xs:complexType name="t"><xs:sequence>
			<xs:element name="e" type="t" minOccurs="0"/></xs:sequence></xs:complexType></xs:schema>`,
			[]byte(strings.Repeat("<e>", 600) + strings.Repeat("</e>", 600)), Config{}},
		{"billion laughs", xs + `<xs:element name="e" type="xs:string"/></xs:schema>`, billionLaughs(), Config{}},
		{"attributes beyond MaxAttributes", xs + `<xs:element name="e"><xs:complexType>
			<xs:anyAttribute processContents="skip"/></xs:complexType></xs:element></xs:schema>`,
			manyAttributes(5000), Config{}},
		{"namespace declarations beyond MaxNamespaces", xs + `<xs:element name="e"><xs:complexType>
			<xs:anyAttribute processContents="skip"/></xs:complexType></xs:element></xs:schema>`,
			manyNamespaces(3000), Config{}},
		{"text beyond MaxTextSize", xs + `<xs:element name="e" type="xs:string"/></xs:schema>`,
			[]byte("<e>" + strings.Repeat("x", 2<<20) + "</e>"), Config{Document: DocumentLimits{MaxTextSize: 1 << 20}}},
		// Model zgodny z UPA, ale niejednoznaczny co do liczników: po każdym a liczba możliwych stanów liczników rośnie,
		// więc ponad MaxConfigurations walidacja kończy się limitem, nie werdyktem (ADR 0007, ADR 0011).
		{"counter-ambiguous model beyond MaxConfigurations", xs + `<xs:element name="r"><xs:complexType>
			<xs:sequence minOccurs="1" maxOccurs="50000"><xs:element name="a" minOccurs="2" maxOccurs="3"/></xs:sequence>
			</xs:complexType></xs:element></xs:schema>`, []byte("<r>" + strings.Repeat("<a/>", 60000) + "</r>"), Config{}},
		{"keys beyond MaxIdentityEntries", xs + `<xs:element name="r"><xs:complexType><xs:sequence>
			<xs:element name="i" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:string"/>
			</xs:complexType></xs:element></xs:sequence></xs:complexType>
			<xs:unique name="u"><xs:selector xpath="i"/><xs:field xpath="@id"/></xs:unique></xs:element></xs:schema>`,
			manyKeys(3000), Config{MaxIdentityEntries: 1000}},
	}
	for _, c := range beyond {
		set, err := c.cfg.Compile(fstest.MapFS{"s.xsd": {Data: []byte(c.schema)}}, "s.xsd")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		start := time.Now()
		_, err = set.Validate(bytes.NewReader(c.doc))
		d := time.Since(start)
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: err = %v, want ErrLimit", c.name, err)
		}
		if d > 2*time.Second {
			t.Errorf("%s: took %v", c.name, d)
		}
	}
}

// billionLaughs to dokument z dziesięcioma poziomami encji, każda dziesięć razy poprzednia (10^10 znaków po
// rozwinięciu).
func billionLaughs() []byte {
	var b bytes.Buffer
	b.WriteString(`<!DOCTYPE e [<!ENTITY l0 "ha">`)
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&b, `<!ENTITY l%d "%s">`, i, strings.Repeat(fmt.Sprintf("&l%d;", i-1), 10))
	}
	b.WriteString(`]><e>&l10;</e>`)
	return b.Bytes()
}

func manyAttributes(n int) []byte {
	var b bytes.Buffer
	b.WriteString("<e")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, ` a%d="v"`, i)
	}
	b.WriteString("/>")
	return b.Bytes()
}

func manyKeys(n int) []byte {
	var b bytes.Buffer
	b.WriteString("<r>")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<i id="k%d"/>`, i)
	}
	b.WriteString("</r>")
	return b.Bytes()
}

// manyNamespaces to element z n deklaracjami przestrzeni nazw (ponad domyślny MaxNamespaces, poniżej MaxAttributes).
func manyNamespaces(n int) []byte {
	var b bytes.Buffer
	b.WriteString("<e")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, ` xmlns:p%d="urn:p%d"`, i, i)
	}
	b.WriteString("/>")
	return b.Bytes()
}
