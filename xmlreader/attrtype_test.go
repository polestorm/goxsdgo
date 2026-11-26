package xmlreader

import (
	"io"
	"strings"
	"testing"
)

// attrTypes czyta dokument i zwraca typy atrybutów wszystkich elementów jako "element@atrybut=typ".
func attrTypes(t *testing.T, doc string, opts Options) []string {
	t.Helper()
	r := New(strings.NewReader(doc), opts)
	var out []string
	for {
		tok, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("%q: %v", doc, err)
		}
		if tok.Kind == KindStartElement {
			for _, a := range tok.Attrs {
				name := a.Name.Local
				if a.Name.Prefix != "" {
					name = a.Name.Prefix + ":" + name
				}
				out = append(out, tok.Name.Local+"@"+name+"="+a.Type)
			}
		}
	}
}

// TestAttrType: Attr.Type podaje typ z deklaracji ATTLIST podzbioru wewnętrznego (wartości [attribute type]
// z XML Infoset), także dla atrybutów dodanych z wartości domyślnej; atrybut bez deklaracji ma typ pusty.
// REQ: XML-018
func TestAttrType(t *testing.T) {
	doc := `<!DOCTYPE doc [
<!NOTATION n SYSTEM "n">
<!ENTITY u SYSTEM "u" NDATA n>
<!ATTLIST e c CDATA #IMPLIED i ID #IMPLIED r IDREF #IMPLIED rs IDREFS #IMPLIED
            en ENTITY #IMPLIED ens ENTITIES #IMPLIED t NMTOKEN #IMPLIED ts NMTOKENS #IMPLIED
            no NOTATION (n) #IMPLIED k (p|q) #IMPLIED d CDATA "default">
<!ATTLIST e i CDATA #IMPLIED>
]>
<doc><e c="1" i="x" r="x" rs="x" en="u" ens="u" t="a" ts="a b" no="n" k="p" other="z"/></doc>`
	want := []string{"e@c=CDATA", "e@i=ID", "e@r=IDREF", "e@rs=IDREFS", "e@en=ENTITY", "e@ens=ENTITIES",
		"e@t=NMTOKEN", "e@ts=NMTOKENS", "e@no=NOTATION", "e@k=ENUMERATION", "e@other=", "e@d=CDATA"}
	for _, opts := range []Options{{}, {NoNamespaces: true}} {
		got := attrTypes(t, doc, opts)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("NoNamespaces=%v:\n got  %v\n want %v", opts.NoNamespaces, got, want)
		}
	}
	// Atrybut z prefiksem i dokument bez DOCTYPE.
	got := attrTypes(t, `<!DOCTYPE a [<!ATTLIST a xml:id ID #IMPLIED p:x IDREF #IMPLIED>]><a xmlns:p="urn:p" xml:id="i" p:x="i" y="1"/>`, Options{})
	if s := strings.Join(got, " "); s != "a@xml:id=ID a@p:x=IDREF a@y=" {
		t.Errorf("prefixed: %s", s)
	}
	if got := attrTypes(t, `<a id="1"/>`, Options{}); strings.Join(got, " ") != "a@id=" {
		t.Errorf("no DOCTYPE: %v", got)
	}
}
