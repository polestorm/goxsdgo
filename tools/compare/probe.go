package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing/fstest"
	"unicode/utf16"

	"github.com/polestorm/goxsdgo/xsd"
	"github.com/polestorm/goxsdval"
)

// Próbki: małe dokumenty i schematy, każdy ma wywołać jeden kod goxsdgo, którego zestaw W3C i korpus nie
// zestawiają z kodem libxml2. Oba walidatory oceniają tę samą próbkę, a para kodów trafia do
// out/compare/code-pairs-probe.tsv (podstawa tabeli compat dla tych kodów, ADR 0009).

const (
	xsNS     = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	xsiNS    = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	anySchem = `<xs:schema ` + xsNS + `><xs:element name="a"/></xs:schema>`
)

type probe struct {
	code   string // kod goxsdgo, którego dotyczy próbka
	schema string // dokument schematu (dla próbek dokumentu: schemat, względem którego walidujemy)
	doc    string // dokument; pusty dla próbek schematu
}

func utf16le(s string) string {
	u := utf16.Encode([]rune(s))
	b := []byte{0xff, 0xfe}
	for _, r := range u {
		b = append(b, byte(r), byte(r>>8))
	}
	return string(b)
}

func schemaDoc(body string) string {
	return `<xs:schema ` + xsNS + `>` + body + `</xs:schema>`
}

var probes = []probe{
	// czytnik XML (dokument, schemat bez znaczenia)
	{"xml-encoding-invalid", anySchem, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><a>\xff</a>"},
	{"xml-encoding-unsupported", anySchem, `<?xml version="1.0" encoding="X-UNKNOWN-1"?><a/>`},
	{"xml-encoding-mismatch", anySchem, utf16le(`<?xml version="1.0" encoding="ISO-8859-1"?><a/>`)},
	{"xml-char", anySchem, "<a>\x01</a>"},
	{"xml-syntax", anySchem, `<a b></a>`},
	{"xml-eof", anySchem, `<a>`},
	{"xml-name", anySchem, `<a><1/></a>`},
	{"xml-no-root", anySchem, `<?xml version="1.0"?>`},
	{"xml-content-outside-root", anySchem, `<a/>text`},
	{"xml-cdata-end", anySchem, `<a>]]></a>`},
	{"xml-comment", anySchem, `<a><!-- x -- y --></a>`},
	{"xml-reserved-pi-target", anySchem, `<a><?XmL x?></a>`},
	{"xml-decl", anySchem, `<?xml encoding="UTF-8"?><a/>`},
	{"xml-entity-structure", anySchem, `<!DOCTYPE a [<!ENTITY e "<b>">]><a>&e;</a>`},
	{"wfc-element-type-match", anySchem, `<a></b>`},
	{"wfc-unique-att-spec", anySchem, `<a b="1" b="2"/>`},
	{"wfc-no-external-entity-references", anySchem, `<!DOCTYPE a [<!ENTITY e SYSTEM "x.xml">]><a b="&e;"/>`},
	{"wfc-no-lt-in-attribute-values", anySchem, `<!DOCTYPE a [<!ENTITY e "&#60;">]><a b="&e;"/>`},
	{"wfc-legal-character", anySchem, `<a>&#1;</a>`},
	{"wfc-entity-declared", anySchem, `<a>&e;</a>`},
	{"wfc-parsed-entity", anySchem, `<!DOCTYPE a [<!NOTATION n SYSTEM "n"><!ENTITY e SYSTEM "x" NDATA n>]><a>&e;</a>`},
	{"wfc-no-recursion", anySchem, `<!DOCTYPE a [<!ENTITY e "&e;">]><a>&e;</a>`},
	{"wfc-pes-in-internal-subset", anySchem, `<!DOCTYPE a [<!ENTITY % p "x"><!ENTITY e "%p;">]><a/>`},
	{"nsc-prefix-declared", anySchem, `<p:a/>`},
	{"nsc-no-prefix-undeclaring", anySchem, `<a xmlns:p=""/>`},
	{"nsc-attributes-unique", anySchem, `<a xmlns:p="u" xmlns:q="u" p:b="1" q:b="2"/>`},
	{"nsc-reserved-prefixes", anySchem, `<a xmlns:xml="http://example.com/x"/>`},
	{"ns-qname", anySchem, `<a:b:c xmlns:a="u"/>`},
	// dokument
	{"cvc-elt.4.1", schemaDoc(`<xs:element name="a" type="xs:string"/>`), `<a ` + xsiNS + ` xsi:type="1x"/>`},
	{"cvc-elt.5.2.2.1", schemaDoc(`<xs:element name="a" fixed="x"><xs:complexType mixed="true"><xs:sequence><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>`), `<a><b/></a>`},
	{"cvc-id.1", schemaDoc(`<xs:element name="a"><xs:complexType><xs:attribute name="r" type="xs:IDREF"/></xs:complexType></xs:element>`), `<a r="missing"/>`},
	{"cvc-identity-constraint.4.2.3", schemaDoc(`<xs:element name="a"><xs:complexType><xs:sequence><xs:element name="b" maxOccurs="unbounded"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" nillable="true"/></xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="key"><xs:selector xpath="b"/><xs:field xpath="k"/></xs:key></xs:element>`), `<a ` + xsiNS + `><b><k xsi:nil="true"/></b></a>`},
	{"cvc-complex-type.5.2", schemaDoc(`<xs:element name="a"><xs:complexType><xs:attribute name="i" type="xs:ID"/><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element><xs:attribute name="j" type="xs:ID"/>`), `<a i="x1" j="x2"/>`},
	// schemat
	{"cos-list-of-atomic", schemaDoc(`<xs:simpleType name="l"><xs:list itemType="xs:IDREFS"/></xs:simpleType>`), ""},
	{"src-single-facet-value", schemaDoc(`<xs:simpleType name="t"><xs:restriction base="xs:string"><xs:length value="1"/><xs:length value="2"/></xs:restriction></xs:simpleType>`), ""},
	{"length-valid-restriction", schemaDoc(`<xs:simpleType name="b"><xs:restriction base="xs:string"><xs:length value="2"/></xs:restriction></xs:simpleType><xs:simpleType name="t"><xs:restriction base="b"><xs:length value="3"/></xs:restriction></xs:simpleType>`), ""},
	{"minInclusive-minExclusive", schemaDoc(`<xs:simpleType name="t"><xs:restriction base="xs:int"><xs:minInclusive value="1"/><xs:minExclusive value="0"/></xs:restriction></xs:simpleType>`), ""},
	{"fractionDigits-valid-restriction", schemaDoc(`<xs:simpleType name="b"><xs:restriction base="xs:decimal"><xs:fractionDigits value="2"/></xs:restriction></xs:simpleType><xs:simpleType name="t"><xs:restriction base="b"><xs:fractionDigits value="3"/></xs:restriction></xs:simpleType>`), ""},
	{"ag-props-correct.3", schemaDoc(`<xs:attributeGroup name="g"><xs:attribute name="a" type="xs:ID"/><xs:attribute name="b" type="xs:ID"/></xs:attributeGroup>`), ""},
	{"src-attribute_group.2", schemaDoc(`<xs:attributeGroup name="h"><xs:attribute name="a"/></xs:attributeGroup><xs:attributeGroup name="g"><xs:attribute name="a"/><xs:attributeGroup ref="h"/></xs:attributeGroup>`), ""},
	{"src-ct.4", schemaDoc(`<xs:attributeGroup name="h"><xs:attribute name="a"/></xs:attributeGroup><xs:complexType name="c"><xs:attribute name="a"/><xs:attributeGroup ref="h"/></xs:complexType>`), ""},
	{"enumeration-required-notation", schemaDoc(`<xs:simpleType name="t"><xs:restriction base="xs:NOTATION"/></xs:simpleType><xs:attribute name="n" type="t"/>`), ""},
	{"cos-ct-extends.1.3", schemaDoc(`<xs:complexType name="b"><xs:anyAttribute namespace="##local"/></xs:complexType><xs:complexType name="c"><xs:complexContent><xs:extension base="b"><xs:anyAttribute namespace="urn:x"/></xs:extension></xs:complexContent></xs:complexType>`), ""},
	{"cos-ct-extends.1.4.1", schemaDoc(`<xs:complexType name="b"><xs:simpleContent><xs:extension base="xs:int"/></xs:simpleContent></xs:complexType><xs:complexType name="c"><xs:simpleContent><xs:restriction base="b"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction></xs:simpleContent></xs:complexType>`), ""},
	{"cos-valid-default.2.2.2", schemaDoc(`<xs:element name="a" default="x"><xs:complexType mixed="true"><xs:sequence><xs:element name="b"/></xs:sequence></xs:complexType></xs:element>`), ""},
}

// runProbe ocenia próbki obu walidatorami i zapisuje pary kodów.
func runProbe(out string) error {
	pairs := newPairCounter()
	var report []string
	for _, p := range probes {
		set, gerr := xsd.Compile(fstest.MapFS{"s.xsd": {Data: []byte(p.schema)}}, "s.xsd")
		if p.doc == "" {
			// próbka schematu
			goCode := ""
			var se *xsd.SchemaError
			if errors.As(gerr, &se) && len(se.Errors) > 0 {
				goCode = se.Errors[0].Code
			}
			lib := 0
			s, lerr := goxsdval.ParseSchemaBytes([]byte(p.schema))
			var spe *goxsdval.SchemaParseError
			if errors.As(lerr, &spe) {
				lib = firstError(spe.Errors).Code
			} else if s != nil {
				s.Close()
			}
			pairs.add(xsd.KindSchema, goCode, lib, true)
			report = append(report, fmt.Sprintf("%s\t%s\t%d\t%s", p.code, goCode, lib, okText(goCode == p.code)))
			continue
		}
		if gerr != nil {
			report = append(report, fmt.Sprintf("%s\tschema error %v\t\t", p.code, gerr))
			continue
		}
		gerrs, verr := set.Validate(strings.NewReader(p.doc))
		goCode := ""
		if len(gerrs) > 0 {
			goCode = gerrs[0].Code
		} else if verr != nil {
			goCode = "error: " + verr.Error()
		}
		s, err := goxsdval.ParseSchemaBytes([]byte(p.schema))
		if err != nil {
			return fmt.Errorf("probe %s: libxml2 rejects the schema: %w", p.code, err)
		}
		lerrs, _ := s.Validate([]byte(p.doc))
		s.Close()
		pairs.addDocument(gerrs, lerrs)
		report = append(report, fmt.Sprintf("%s\t%s\t%d\t%s", p.code, goCode, firstError(lerrs).Code, okText(goCode == p.code)))
	}
	if err := writeLines(filepath.Join(out, "probe.tsv"), "probe\tgoxsdgo\tlibxml2\tgoxsdgo gives the probed code", report); err != nil {
		return err
	}
	for _, r := range report {
		fmt.Println(r)
	}
	return pairs.write(filepath.Join(out, "code-pairs-probe.tsv"))
}

func okText(ok bool) string {
	if ok {
		return "ok"
	}
	return "DIFFERENT CODE"
}
