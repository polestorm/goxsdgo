package xmlreader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf16"
)

// dump czyta cały dokument i zapisuje tokeny w zwartej postaci; zwraca też błąd.
func dump(doc []byte, opts Options) (string, error) {
	r := New(bytes.NewReader(doc), opts)
	var b strings.Builder
	for {
		t, err := r.Next()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return b.String(), err
		}
		switch t.Kind {
		case KindStartElement:
			fmt.Fprintf(&b, "<{%s}%s", t.Name.Space, t.Name.Local)
			for _, a := range t.Attrs {
				spec := ""
				if !a.Specified {
					spec = "*"
				}
				fmt.Fprintf(&b, " {%s}%s%s=%q", a.Name.Space, a.Name.Local, spec, a.Value)
			}
			b.WriteString(">")
		case KindEndElement:
			fmt.Fprintf(&b, "</%s>", t.Name.Local)
		case KindText:
			if t.CDATA {
				b.WriteString("CDATA")
			}
			fmt.Fprintf(&b, "%q", t.Data)
		case KindComment:
			fmt.Fprintf(&b, "<!--%s-->", t.Data)
		case KindProcInst:
			fmt.Fprintf(&b, "<?%s %s?>", t.Name.Local, t.Data)
		case KindDoctype:
			fmt.Fprintf(&b, "<!DOCTYPE %s>", t.Name.Local)
		}
	}
}

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "other: " + err.Error()
}

func wantTokens(t *testing.T, doc, want string) {
	t.Helper()
	got, err := dump([]byte(doc), Options{})
	if err != nil {
		t.Errorf("%q: %v", doc, err)
		return
	}
	if got != want {
		t.Errorf("%q:\n got  %s\n want %s", doc, got, want)
	}
}

func wantError(t *testing.T, doc, wantCode string, opts ...Options) {
	t.Helper()
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	_, err := dump([]byte(doc), o)
	if code(err) != wantCode {
		t.Errorf("%q: error %v, want code %s", doc, err, wantCode)
	}
}

// REQ: XML-001
func TestDocumentStructure(t *testing.T) {
	wantTokens(t, "<?xml version='1.0'?>\n<!-- c --><a/>\n<?p d?>\n", `<!-- c --><{}a></a><?p d?>`)
	wantError(t, "", CodeNoRoot)
	wantError(t, "<!-- only -->", CodeNoRoot)
	wantError(t, "<a/><b/>", CodeOutsideRoot)
	wantError(t, "x<a/>", CodeOutsideRoot)
	wantError(t, "<a/>x", CodeOutsideRoot)
	wantError(t, "<a/>&amp;", CodeOutsideRoot)
	wantError(t, "<a/><!DOCTYPE a>", CodeSyntax)
	wantError(t, "<a>", CodeEOF)
}

// REQ: XML-002
func TestCharacters(t *testing.T) {
	wantError(t, "<a>\x01</a>", CodeChar)
	wantError(t, "<a>\xff</a>", CodeEncodingInvalid)
	wantError(t, "<a>\xed\xa0\x80</a>", CodeEncodingInvalid) // zakodowany surogat
	wantError(t, "<a>\xef\xbf\xbe</a>", CodeChar)            // U+FFFE
	wantTokens(t, "<a>zażółć \U0001F600</a>", `<{}a>"zażółć 😀"</a>`)
}

func utf16Doc(s string, bigEndian, bom bool) []byte {
	var b []byte
	if bom {
		if bigEndian {
			b = append(b, 0xFE, 0xFF)
		} else {
			b = append(b, 0xFF, 0xFE)
		}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		if bigEndian {
			b = append(b, byte(u>>8), byte(u))
		} else {
			b = append(b, byte(u), byte(u>>8))
		}
	}
	return b
}

// REQ: XML-003
func TestEncodings(t *testing.T) {
	for _, be := range []bool{false, true} {
		got, err := dump(utf16Doc("<?xml version='1.0' encoding='UTF-16'?><a>ż😀</a>", be, true), Options{})
		if err != nil || got != `<{}a>"ż😀"</a>` {
			t.Errorf("UTF-16 (big endian %v): %s, %v", be, got, err)
		}
	}
	if _, err := dump(utf16Doc("<a/>", false, false), Options{}); code(err) != CodeEncodingInvalid {
		t.Errorf("UTF-16 without BOM: %v", err)
	}
	if _, err := dump(utf16Doc("<?xml version='1.0' encoding='UTF-8'?><a/>", false, true), Options{}); code(err) != CodeEncodingMismatch {
		t.Errorf("UTF-16 declaring UTF-8: %v", err)
	}
	got, err := dump([]byte("<?xml version='1.0' encoding='ISO-8859-1'?><a b='\xe9'>\xf3</a>"), Options{})
	if err != nil || got != `<{}a {}b="é">"ó"</a>` {
		t.Errorf("ISO-8859-1: %s, %v", got, err)
	}
	wantError(t, "<?xml version='1.0' encoding='US-ASCII'?><a>\xe9</a>", CodeEncodingInvalid)
	wantError(t, "\xef\xbb\xbf<?xml version='1.0' encoding='ISO-8859-1'?><a/>", CodeEncodingMismatch)
	wantError(t, "<?xml version='1.0' encoding='windows-1250'?><a/>", CodeEncodingUnsupported)
	wantError(t, "\x00\x00\xfe\xff", CodeEncodingUnsupported)
	wantTokens(t, "\xef\xbb\xbf<a/>", `<{}a></a>`)
}

// REQ: XML-004
func TestLineEnds(t *testing.T) {
	wantTokens(t, "<a>1\r\n2\r3\n4&#13;</a>", `<{}a>"1\n2\n3\n4\r"</a>`)
	// CR na granicy porcji wejścia: para CR LF daje jedno LF.
	doc := "<a>" + strings.Repeat("x", chunkSize-4) + "\r\ny</a>"
	got, err := dump([]byte(doc), Options{})
	if err != nil || !strings.HasSuffix(got, `x\ny"</a>`) {
		t.Errorf("CR LF across chunks: %v %s", err, got[len(got)-20:])
	}
}

// REQ: XML-005
func TestNames(t *testing.T) {
	wantTokens(t, "<ąę.-·x/>", `<{}ąę.-·x></ąę.-·x>`)
	wantError(t, "<.a/>", CodeName)
	wantError(t, "<a b.='1' -c='2'/>", CodeName)
	wantTokens(t, "<a\u0300/>", `<{}à></à>`)
	wantError(t, "<\u0300/>", CodeName)
}

// REQ: XML-006
func TestTags(t *testing.T) {
	wantTokens(t, "<a x='1' y=\"2\" ><b/></a >", `<{}a {}x="1" {}y="2">"" `[:0]+`<{}a {}x="1" {}y="2"><{}b></b></a>`)
	wantError(t, "<a></b>", CodeElementTypeMatch)
	wantError(t, "<a x='1' x='2'/>", CodeUniqueAttSpec)
	wantError(t, "<a x='1'y='2'/>", CodeSyntax)
	wantError(t, "<a x=1/>", CodeSyntax)
	wantError(t, "<a x='1", CodeEOF)
	wantError(t, "<a / >", CodeSyntax)
}

// REQ: XML-007
func TestAttributeValues(t *testing.T) {
	wantTokens(t, "<a x=' 1\t2\n3 &#10; ' y='&lt;&amp;'/>", `<{}a {}x=" 1 2 3 \n " {}y="<&"></a>`)
	wantError(t, "<a x='<'/>", CodeSyntax)
	wantTokens(t, `<!DOCTYPE a [<!ATTLIST a t NMTOKENS #IMPLIED c CDATA #IMPLIED>]><a t='  x   y ' c='  x  '/>`,
		`<!DOCTYPE a><{}a {}t="x y" {}c="  x  "></a>`)
	wantTokens(t, `<!DOCTYPE a [<!ENTITY e "x&#9;y	z"><!ATTLIST a v CDATA #IMPLIED>]><a v='&e;'/>`,
		`<!DOCTYPE a><{}a {}v="x y z"></a>`)
}

// REQ: XML-008
func TestReferences(t *testing.T) {
	wantTokens(t, "<a>&lt;&gt;&amp;&apos;&quot;&#65;&#x42;&#x1F600;</a>", `<{}a>"<>&'\"AB😀"</a>`)
	wantTokens(t, `<!DOCTYPE a [<!ENTITY lt "&#38;#60;">]><a>&lt;</a>`, `<!DOCTYPE a><{}a>"<"</a>`)
	wantError(t, "<a>&#0;</a>", CodeLegalCharacter)
	wantError(t, "<a>&#xD800;</a>", CodeLegalCharacter)
	wantError(t, "<a>&#x110000;</a>", CodeLegalCharacter)
	wantError(t, "<a>&#x;</a>", CodeSyntax)
	wantError(t, "<a>&#65</a>", CodeSyntax)
	wantError(t, "<a>&amp</a>", CodeSyntax)
}

// REQ: XML-009
func TestCommentsPIsCDATA(t *testing.T) {
	wantTokens(t, "<a><!-- x - y --><?t  data ?><![CDATA[<&]]>]]&gt;</a>",
		`<{}a><!-- x - y --><?t data ?>CDATA"<&""]]>"</a>`)
	wantError(t, "<a><!-- a -- b --></a>", CodeComment)
	wantError(t, "<a><!-- a ---></a>", CodeComment)
	wantError(t, "<a><?xml version='1.0'?></a>", CodeReservedPITarget)
	wantError(t, "<a><?XmL x?></a>", CodeReservedPITarget)
	wantError(t, "<a>]]></a>", CodeCDATAEnd)
	wantError(t, " <?xml version='1.0'?><a/>", CodeReservedPITarget)
	wantError(t, "<?xml version='2.0'?><a/>", CodeXMLDecl)
	wantTokens(t, "<?xml version='1.1'?><a/>", `<{}a></a>`)
	wantError(t, "<?xml encoding='UTF-8'?><a/>", CodeXMLDecl)
	wantError(t, "<?xml version='1.0' standalone='yes' encoding='UTF-8'?><a/>", CodeXMLDecl)
	wantError(t, "<?xml version='1.0' standalone='maybe'?><a/>", CodeXMLDecl)
	wantTokens(t, "<?xml-stylesheet href='x'?><a/>", `<?xml-stylesheet href='x'?><{}a></a>`)
}

// REQ: XML-010
func TestDoctypeSyntax(t *testing.T) {
	wantTokens(t, `<!DOCTYPE a PUBLIC "-//X//Y" "a.dtd" [
		<!ELEMENT a (b|(c,d)*)+> <!ELEMENT b EMPTY> <!ELEMENT c ANY> <!ELEMENT d (#PCDATA|b)*> <!ELEMENT e (#PCDATA)>
		<!ATTLIST a x ID #REQUIRED y (p|q) "p" z NOTATION (n) #IMPLIED w CDATA #FIXED "w">
		<!NOTATION n PUBLIC "-//N"> <!NOTATION m SYSTEM "m">
		<!-- c --> <?pi d?>
		<!ENTITY u SYSTEM "u.bin" NDATA n>
	]><a x="1"/>`, `<!DOCTYPE a><{}a {}x="1" {}y*="p" {}w*="w"></a>`)
	for _, bad := range []string{
		`<!DOCTYPE a [<!ELEMENT a (b,c|d)>]><a/>`,
		`<!DOCTYPE a [<!ELEMENT a (#PCDATA|b)>]><a/>`,
		`<!DOCTYPE a [<!ELEMENT a EMPTYX>]><a/>`,
		`<!DOCTYPE a [<!ATTLIST a x BOGUS #IMPLIED>]><a/>`,
		`<!DOCTYPE a [<!ATTLIST a x CDATA>]><a/>`,
		`<!DOCTYPE a [<!ENTITY e>]><a/>`,
		`<!DOCTYPE a [<!NOTATION n>]><a/>`,
		`<!DOCTYPE a [<!FOO>]><a/>`,
		`<!DOCTYPE a [<![INCLUDE[<!ELEMENT a ANY>]]>]><a/>`,
		`<!DOCTYPE a [<!ENTITY e PUBLIC "a\b" "x">]><a/>`,
	} {
		_, err := dump([]byte(bad), Options{})
		if c := code(err); c != CodeSyntax && c != CodeName && c != CodeEOF {
			t.Errorf("%q: %v", bad, err)
		}
	}
	wantError(t, `<!DOCTYPE a [<!ENTITY % p "x"><!ELEMENT a %p;>]><a/>`, CodePEsInInternalSubset)
	wantError(t, `<!DOCTYPE a [<!ENTITY % p "x"><!ENTITY e "%p;">]><a/>`, CodePEsInInternalSubset)
}

// REQ: XML-011
func TestInternalEntities(t *testing.T) {
	wantTokens(t, `<!DOCTYPE a [<!ENTITY e "x<b>&f;</b>y"><!ENTITY f "&#38;#38;z">]><a>1&e;2</a>`,
		`<!DOCTYPE a><{}a>"1x"<{}b>"&z"</b>"y2"</a>`)
	wantTokens(t, `<!DOCTYPE a [<!ENTITY % d "<!ENTITY e 'v'>"> %d;]><a>&e;</a>`, `<!DOCTYPE a><{}a>"v"</a>`)
	wantError(t, `<a>&e;</a>`, CodeEntityDeclared)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "<b>">]><a>&e;</b></a>`, CodeEntityStructure)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "</a>">]><a>&e;</a>`, CodeEntityStructure)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "<b x='">]><a>&e;'/></a>`, CodeEntityStructure)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "&f;"><!ENTITY f "&e;">]><a>&e;</a>`, CodeNoRecursion)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "&f;"><!ENTITY f "&e;">]><a x='&e;'/>`, CodeNoRecursion)
	wantError(t, `<!DOCTYPE a [<!ENTITY e "<">]><a x='&e;'/>`, CodeNoLtInAttValue)
	wantError(t, `<!DOCTYPE a [<!NOTATION n SYSTEM "n"><!ENTITY u SYSTEM "u" NDATA n>]><a>&u;</a>`, CodeParsedEntity)
	wantError(t, `<!DOCTYPE a [<!NOTATION n SYSTEM "n"><!ENTITY u SYSTEM "u" NDATA n>]><a x='&u;'/>`, CodeParsedEntity)
	// Po nieczytanej encji parametrycznej dalsze deklaracje encji nie są przetwarzane (5.1).
	wantError(t, `<!DOCTYPE a [<!ENTITY % x SYSTEM "x.ent"> %x; <!ENTITY e "v">]><a>&e;</a>`, CodeEntityUnresolved)
	wantTokens(t, `<?xml version='1.0' standalone='yes'?><!DOCTYPE a [<!ENTITY % x SYSTEM "x.ent"> %x; <!ENTITY e "v">]><a>&e;</a>`,
		`<!DOCTYPE a><{}a>"v"</a>`)
	// Pierwsza deklaracja encji obowiązuje.
	wantTokens(t, `<!DOCTYPE a [<!ENTITY e "1"><!ENTITY e "2">]><a>&e;</a>`, `<!DOCTYPE a><{}a>"1"</a>`)
}

// REQ: XML-012
func TestDefaultAttributes(t *testing.T) {
	wantTokens(t, `<!DOCTYPE a [<!ATTLIST a x CDATA "1" y CDATA #IMPLIED><!ATTLIST a x CDATA "2" z CDATA "3">]><a/>`,
		`<!DOCTYPE a><{}a {}x*="1" {}z*="3"></a>`)
	wantTokens(t, `<!DOCTYPE a [<!ATTLIST a x CDATA "1">]><a x="0"/>`, `<!DOCTYPE a><{}a {}x="0"></a>`)
	// Domyślna deklaracja przestrzeni nazw z ATTLIST działa jak podana w znaczniku.
	wantTokens(t, `<!DOCTYPE p:a [<!ATTLIST p:a xmlns:p CDATA "urn:p">]><p:a/>`, `<!DOCTYPE p:a><{urn:p}a></a>`)
}

// REQ: XML-013
func TestNamespaces(t *testing.T) {
	wantTokens(t, `<a xmlns="urn:d" xmlns:p="urn:p" p:x="1" y="2"><p:b/><c xmlns=""/></a>`,
		`<{urn:d}a {urn:p}x="1" {}y="2"><{urn:p}b></b><{}c></c></a>`)
	wantTokens(t, `<a xml:lang="pl"/>`, `<{}a {http://www.w3.org/XML/1998/namespace}lang="pl"></a>`)
	wantError(t, `<p:a/>`, CodePrefixDeclared)
	wantError(t, `<a p:x="1"/>`, CodePrefixDeclared)
	wantError(t, `<a xmlns:p=""/>`, CodeNoPrefixUndeclaring)
	wantError(t, `<a xmlns:xmlns="urn:x"/>`, CodeReservedPrefixes)
	wantError(t, `<a xmlns:xml="urn:x"/>`, CodeReservedPrefixes)
	wantError(t, `<a xmlns:p="http://www.w3.org/XML/1998/namespace"/>`, CodeReservedPrefixes)
	wantError(t, `<a xmlns="http://www.w3.org/2000/xmlns/"/>`, CodeReservedPrefixes)
	wantError(t, `<xmlns:a xmlns:xmlns="x"/>`, CodeReservedPrefixes)
	wantError(t, `<a xmlns:p="urn:x" xmlns:q="urn:x" p:x="1" q:x="2"/>`, CodeAttributesUnique)
	wantError(t, `<a:b:c xmlns:a="u"/>`, CodeQName)
	wantError(t, `<:a/>`, CodeQName)
	wantError(t, `<a><?p:q x?></a>`, CodeQName)
	wantError(t, `<!DOCTYPE a [<!ENTITY a:b "x">]><a/>`, CodeQName)
	// Bez przestrzeni nazw te same dokumenty są dobrze uformowanym XML 1.0.
	if _, err := dump([]byte(`<a:b:c x:y="1"/>`), Options{NoNamespaces: true}); err != nil {
		t.Errorf("NoNamespaces: %v", err)
	}
	// Zasięg deklaracji: po końcu elementu prefiks jest niedostępny.
	r := New(strings.NewReader(`<a><b xmlns:p="urn:p"/><c/></a>`), Options{})
	for {
		tk, err := r.Next()
		if err != nil {
			break
		}
		_, ok := r.LookupNamespace("p")
		if tk.Kind == KindStartElement && (tk.Name.Local == "b") != ok {
			t.Errorf("prefix p in scope at <%s>: %v", tk.Name.Local, ok)
		}
	}
}

// REQ: XML-014
func TestPositions(t *testing.T) {
	r := New(strings.NewReader("<a>\r\n  <ż x='1'\n     y='2'>t</ż></a>"), Options{})
	var got []string
	for {
		tk, err := r.Next()
		if err != nil {
			break
		}
		s := fmt.Sprintf("%d:%d", tk.Line, tk.Column)
		for _, a := range tk.Attrs {
			s += fmt.Sprintf(" %s@%d:%d", a.Name.Local, a.Line, a.Column)
		}
		got = append(got, s)
	}
	want := "1:1 | 1:4 | 2:3 x@2:6 y@3:6 | 3:12 | 3:13 | 3:17"
	if g := strings.Join(got, " | "); g != want {
		t.Errorf("positions:\n got  %s\n want %s", g, want)
	}
	_, err := dump([]byte("<a>\n  <ąę></b></a>"), Options{})
	var e *Error
	if !errors.As(err, &e) || e.Line != 2 || e.Column != 7 {
		t.Errorf("end tag mismatch position: %v", err)
	}
	_, err = dump([]byte("<!DOCTYPE a [<!ENTITY e '&#0;'>]>"), Options{})
	if code(err) != CodeLegalCharacter {
		t.Errorf("error in entity value: %v", err)
	}
	_, err = dump([]byte("<!DOCTYPE a [<!ENTITY e '<b>'>]>\n<a>xx&e;</a>"), Options{})
	if !errors.As(err, &e) || e.Line != 2 || e.Column != 6 {
		t.Errorf("error inside entity text reported at the reference: %v", err)
	}
	r = New(strings.NewReader("<a></b>"), Options{})
	r.Next()
	_, err1 := r.Next()
	_, err2 := r.Next()
	if err1 == nil || err1 != err2 {
		t.Errorf("fatal error not sticky: %v, %v", err1, err2)
	}
}

// REQ: XML-015
func TestNoExternalEntities(t *testing.T) {
	wantError(t, `<!DOCTYPE a [<!ENTITY e SYSTEM "file:///etc/passwd">]><a>&e;</a>`, CodeExternalEntity)
	wantError(t, `<!DOCTYPE a [<!ENTITY e SYSTEM "http://example.com/x">]><a x='&e;'/>`, CodeNoExternalEntityRef)
	wantError(t, `<!DOCTYPE a SYSTEM "a.dtd"><a>&e;</a>`, CodeEntityUnresolved)
	wantTokens(t, `<!DOCTYPE a SYSTEM "http://example.com/a.dtd"><a/>`, `<!DOCTYPE a><{}a></a>`)
}

// REQ: XML-016
func TestLimits(t *testing.T) {
	deep := strings.Repeat("<a>", 600) + strings.Repeat("</a>", 600)
	wantError(t, deep, CodeLimitDepth)
	if _, err := dump([]byte(deep), Options{Limits: Limits{MaxDepth: -1}}); err != nil {
		t.Errorf("unlimited depth: %v", err)
	}
	wantError(t, "<"+strings.Repeat("n", 20000)+"/>", CodeLimitNameLength)
	var attrs strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&attrs, " a%d='1'", i)
	}
	wantError(t, "<a"+attrs.String()+"/>", CodeLimitAttributes, Options{Limits: Limits{MaxAttributes: 10}})
	wantError(t, "<a>"+strings.Repeat("x", 2000)+"</a>", CodeLimitTextSize, Options{Limits: Limits{MaxTextSize: 1000}})
	wantError(t, "<a x='"+strings.Repeat("x", 2000)+"'/>", CodeLimitTextSize, Options{Limits: Limits{MaxTextSize: 1000}})
	// „Billion laughs”: dziesięć poziomów po dziesięć odwołań.
	var lol strings.Builder
	lol.WriteString(`<!DOCTYPE a [<!ENTITY l0 "lol">`)
	for i := 1; i <= 9; i++ {
		fmt.Fprintf(&lol, `<!ENTITY l%d "%s">`, i, strings.Repeat(fmt.Sprintf("&l%d;", i-1), 10))
	}
	lol.WriteString(`]><a>&l9;</a>`)
	wantError(t, lol.String(), CodeLimitEntityExpand)
	wantError(t, strings.Replace(lol.String(), "<a>&l9;</a>", "<a x='&l9;'/>", 1), CodeLimitEntityExpand)
	var chain strings.Builder
	chain.WriteString(`<!DOCTYPE a [<!ENTITY e0 "x">`)
	for i := 1; i < 40; i++ {
		fmt.Fprintf(&chain, `<!ENTITY e%d "&e%d;">`, i, i-1)
	}
	chain.WriteString(`]><a>&e39;</a>`)
	wantError(t, chain.String(), CodeLimitEntityDepth)
	wantError(t, `<!DOCTYPE a [<!-- `+strings.Repeat("x", 3000)+` -->]><a/>`, CodeLimitDTDSize, Options{Limits: Limits{MaxDTDSize: 1000}})
}

// endless podaje dokument z n rekordami bez trzymania go w pamięci.
type endless struct {
	head, rec, tail []byte
	n               int
	state           int
	pending         []byte
}

func (e *endless) Read(p []byte) (int, error) {
	for len(e.pending) == 0 {
		switch {
		case e.state == 0:
			e.pending, e.state = e.head, 1
		case e.state == 1 && e.n > 0:
			e.pending = e.rec
			e.n--
		case e.state == 1:
			e.pending, e.state = e.tail, 2
		default:
			return 0, io.EOF
		}
	}
	n := copy(p, e.pending)
	e.pending = e.pending[n:]
	return n, nil
}

func jpkLike(records int) *endless {
	return &endless{
		head: []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<tns:JPK xmlns:tns="http://crd.gov.pl/wzor/2025/12/19/14090/" xmlns:etd="http://crd.gov.pl/xml/schematy/dziedzinowe/mf/2022/01/05/eD/DefinicjeTypy/">` + "\n" +
			`<tns:Naglowek><tns:KodFormularza kodSystemowy="JPK_V7M (3)" wersjaSchemy="1-0E">JPK_VAT</tns:KodFormularza></tns:Naglowek><tns:Ewidencja>` + "\n"),
		rec: []byte(`<tns:SprzedazWiersz><tns:LpSprzedazy>1</tns:LpSprzedazy><tns:KodKrajuNadaniaTIN>PL</tns:KodKrajuNadaniaTIN>` +
			`<tns:NrKontrahenta>5260250274</tns:NrKontrahenta><tns:NazwaKontrahenta>Przedsiębiorstwo Usługowe „Żółw” sp. z o.o.</tns:NazwaKontrahenta>` +
			`<tns:DowodSprzedazy>FV/2026/01/000123</tns:DowodSprzedazy><tns:DataWystawienia>2026-01-15</tns:DataWystawienia>` +
			`<tns:K_19>1234.56</tns:K_19><tns:K_20>283.95</tns:K_20></tns:SprzedazWiersz>` + "\n"),
		tail: []byte("</tns:Ewidencja></tns:JPK>\n"),
		n:    records,
	}
}

func readAll(t testing.TB, r io.Reader) int {
	rd := New(r, Options{})
	n := 0
	for {
		_, err := rd.Next()
		if err == io.EOF {
			return n
		}
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
}

// REQ: XML-017
func TestStreamingMemory(t *testing.T) {
	src := jpkLike(200000) // ok. 100 MB
	rd := New(src, Options{})
	maxBuf := 0
	for {
		_, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		maxBuf = max(maxBuf, cap(rd.docF.buf), cap(rd.text), cap(rd.vbuf))
	}
	if maxBuf > 1<<20 {
		t.Errorf("reader buffers grew to %d bytes on a streamed document", maxBuf)
	}
}

func BenchmarkJPK(b *testing.B) {
	const records = 20000
	var size int64
	{
		e := jpkLike(records)
		n, _ := io.Copy(io.Discard, e)
		size = n
	}
	b.SetBytes(size)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		readAll(b, jpkLike(records))
	}
}

// BenchmarkJPKStdlib to punkt odniesienia: encoding/xml na tym samym dokumencie (tylko tokeny, bez sprawdzania
// przestrzeni nazw w pełnym zakresie rekomendacji).
func BenchmarkJPKStdlib(b *testing.B) {
	const records = 20000
	var size int64
	{
		n, _ := io.Copy(io.Discard, jpkLike(records))
		size = n
	}
	b.SetBytes(size)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d := stdDecoder(jpkLike(records))
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
