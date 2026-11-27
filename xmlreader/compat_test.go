package xmlreader_test

import (
	"bufio"
	"errors"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	gx "github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxsdgo/xmlreader"
)

// Sygnatury funkcji i metod z v0.3.0: zmiana którejkolwiek przerywa kompilację testu.
var (
	_ func() xmlreader.Limits                                 = xmlreader.DefaultLimits
	_ func(io.Reader, xmlreader.Options) *xmlreader.Reader    = xmlreader.New
	_ func(*xmlreader.Error) string                           = (*xmlreader.Error).Error
	_ func(*xmlreader.Reader) []xmlreader.Binding             = (*xmlreader.Reader).DeclaredNamespaces
	_ func(*xmlreader.Reader, string) (string, bool)          = (*xmlreader.Reader).LookupNamespace
	_ func(*xmlreader.Reader) (xmlreader.Token, error)        = (*xmlreader.Reader).Next
	_ func(*xmlreader.Reader) []xmlreader.Notation            = (*xmlreader.Reader).Notations
	_ func(*xmlreader.Reader, string) bool                    = (*xmlreader.Reader).UnparsedEntity
	_ xmlreader.Kind                                          = xmlreader.KindDoctype
	_ *gx.Reader                                              = (*xmlreader.Reader)(nil)
	_ *xmlreader.Error                                        = (*gx.Error)(nil)
	_ interface{ Next() (gx.Token, error) }                   = (*xmlreader.Reader)(nil)
	_ func(io.Reader, gx.Options) *gx.Reader                  = xmlreader.New
	_ func() gx.Limits                                        = xmlreader.DefaultLimits
	_ gx.Kind                                                 = xmlreader.KindStartElement
	_ [len(xmlreader.CodeConditionalSection)]struct{}         = [len(gx.CodeConditionalSection)]struct{}{}
	_ [len(xmlreader.AttrTypeEnumeration) + 1]struct{}        = [len(gx.AttrTypeEnumeration) + 1]struct{}{}
	_ [xmlreader.KindDoctype - xmlreader.KindStartElement]int = [gx.KindDoctype - gx.KindStartElement]int{}
)

// TestAPIv030 porównuje warstwę zgodności z wzorcem API v0.3.0 (testdata/api-v0.3.0.txt, kopia
// ../goxmlgo/api/baseline/xmlreader-goxsdgo-v0.3.0.txt): każda stała ma tę samą wartość, każdy typ istnieje
// i jest tym samym typem co w goxmlgo/xmlreader, każde pole struktury ma ten sam typ, każda metoda istnieje.
// REQ: XML-020
func TestAPIv030(t *testing.T) {
	f, err := os.Open("testdata/api-v0.3.0.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	constValues := map[string]any{}
	for k, v := range map[string]any{
		"AttrTypeCDATA": xmlreader.AttrTypeCDATA, "AttrTypeID": xmlreader.AttrTypeID, "AttrTypeIDREF": xmlreader.AttrTypeIDREF,
		"AttrTypeIDREFS": xmlreader.AttrTypeIDREFS, "AttrTypeENTITY": xmlreader.AttrTypeENTITY,
		"AttrTypeENTITIES": xmlreader.AttrTypeENTITIES, "AttrTypeNMTOKEN": xmlreader.AttrTypeNMTOKEN,
		"AttrTypeNMTOKENS": xmlreader.AttrTypeNMTOKENS, "AttrTypeNotation": xmlreader.AttrTypeNotation,
		"AttrTypeEnumeration": xmlreader.AttrTypeEnumeration,
		"CodeEncodingInvalid": xmlreader.CodeEncodingInvalid, "CodeEncodingUnsupported": xmlreader.CodeEncodingUnsupported,
		"CodeEncodingMismatch": xmlreader.CodeEncodingMismatch, "CodeChar": xmlreader.CodeChar,
		"CodeSyntax": xmlreader.CodeSyntax, "CodeEOF": xmlreader.CodeEOF, "CodeName": xmlreader.CodeName,
		"CodeNoRoot": xmlreader.CodeNoRoot, "CodeOutsideRoot": xmlreader.CodeOutsideRoot,
		"CodeCDATAEnd": xmlreader.CodeCDATAEnd, "CodeComment": xmlreader.CodeComment,
		"CodeReservedPITarget": xmlreader.CodeReservedPITarget, "CodeXMLDecl": xmlreader.CodeXMLDecl,
		"CodeEntityStructure": xmlreader.CodeEntityStructure, "CodeElementTypeMatch": xmlreader.CodeElementTypeMatch,
		"CodeUniqueAttSpec": xmlreader.CodeUniqueAttSpec, "CodeNoExternalEntityRef": xmlreader.CodeNoExternalEntityRef,
		"CodeNoLtInAttValue": xmlreader.CodeNoLtInAttValue, "CodeLegalCharacter": xmlreader.CodeLegalCharacter,
		"CodeEntityDeclared": xmlreader.CodeEntityDeclared, "CodeParsedEntity": xmlreader.CodeParsedEntity,
		"CodeNoRecursion": xmlreader.CodeNoRecursion, "CodePEsInInternalSubset": xmlreader.CodePEsInInternalSubset,
		"CodePrefixDeclared": xmlreader.CodePrefixDeclared, "CodeNoPrefixUndeclaring": xmlreader.CodeNoPrefixUndeclaring,
		"CodeAttributesUnique": xmlreader.CodeAttributesUnique, "CodeReservedPrefixes": xmlreader.CodeReservedPrefixes,
		"CodeQName": xmlreader.CodeQName, "CodeLimitDepth": xmlreader.CodeLimitDepth,
		"CodeLimitNameLength": xmlreader.CodeLimitNameLength, "CodeLimitAttributes": xmlreader.CodeLimitAttributes,
		"CodeLimitTextSize": xmlreader.CodeLimitTextSize, "CodeLimitEntityExpand": xmlreader.CodeLimitEntityExpand,
		"CodeLimitEntityDepth": xmlreader.CodeLimitEntityDepth, "CodeLimitDTDSize": xmlreader.CodeLimitDTDSize,
		"CodeExternalEntity": xmlreader.CodeExternalEntity, "CodeEntityUnresolved": xmlreader.CodeEntityUnresolved,
		"CodeConditionalSection": xmlreader.CodeConditionalSection,
		"KindStartElement":       xmlreader.KindStartElement, "KindEndElement": xmlreader.KindEndElement,
		"KindText": xmlreader.KindText, "KindComment": xmlreader.KindComment, "KindProcInst": xmlreader.KindProcInst,
		"KindDoctype": xmlreader.KindDoctype,
	} {
		constValues[k] = v
	}
	types := map[string]reflect.Type{
		"Attr": reflect.TypeOf(xmlreader.Attr{}), "Binding": reflect.TypeOf(xmlreader.Binding{}),
		"Error": reflect.TypeOf(xmlreader.Error{}), "Kind": reflect.TypeOf(xmlreader.Kind(0)),
		"Limits": reflect.TypeOf(xmlreader.Limits{}), "Name": reflect.TypeOf(xmlreader.Name{}),
		"Notation": reflect.TypeOf(xmlreader.Notation{}), "Options": reflect.TypeOf(xmlreader.Options{}),
		"Reader": reflect.TypeOf(xmlreader.Reader{}), "Token": reflect.TypeOf(xmlreader.Token{}),
	}
	same := map[string]reflect.Type{
		"Attr": reflect.TypeOf(gx.Attr{}), "Binding": reflect.TypeOf(gx.Binding{}), "Error": reflect.TypeOf(gx.Error{}),
		"Kind": reflect.TypeOf(gx.Kind(0)), "Limits": reflect.TypeOf(gx.Limits{}), "Name": reflect.TypeOf(gx.Name{}),
		"Notation": reflect.TypeOf(gx.Notation{}), "Options": reflect.TypeOf(gx.Options{}),
		"Reader": reflect.TypeOf(gx.Reader{}), "Token": reflect.TypeOf(gx.Token{}),
	}
	funcs := map[string]bool{"DefaultLimits": true, "New": true}

	var (
		constRe  = regexp.MustCompile(`^\t(\w+)(?:\s+Kind)?(?:\s+=\s+(.*))?$`)
		typeRe   = regexp.MustCompile(`^type (\w+) (.*)$`)
		fieldRe  = regexp.MustCompile(`^\t([\w, ]+?)\s+(\S+)$`)
		funcRe   = regexp.MustCompile(`^func (\w+)\(`)
		methodRe = regexp.MustCompile(`^func \(\w+ \*?(\w+)\) (\w+)\(`)
	)
	var block, inType string
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "const (":
			block = "const"
			continue
		case line == ")" || line == "}":
			block, inType = "", ""
			continue
		case line == "" || strings.HasPrefix(line, "package ") || strings.HasPrefix(line, "\t//") ||
			strings.TrimSpace(line) == "// contains filtered or unexported fields":
			continue
		}
		n++
		if block == "const" {
			m := constRe.FindStringSubmatch(line)
			v, ok := constValues[m[1]]
			switch {
			case m == nil:
				t.Errorf("baseline: unparsed constant line %q", line)
			case !ok:
				t.Errorf("constant %s of v0.3.0 is missing", m[1])
			case m[2] != "" && strings.HasPrefix(m[2], `"`) && `"`+v.(string)+`"` != m[2]:
				t.Errorf("constant %s = %q, v0.3.0 has %s", m[1], v, m[2])
			}
			continue
		}
		if inType != "" {
			m := fieldRe.FindStringSubmatch(line)
			if m == nil {
				t.Errorf("baseline: unparsed field line %q", line)
				continue
			}
			for _, name := range strings.Split(m[1], ", ") {
				fld, ok := types[inType].FieldByName(name)
				if !ok {
					t.Errorf("field %s.%s of v0.3.0 is missing", inType, name)
					continue
				}
				if got := strings.NewReplacer("xmlreader.", "", "uint8", "byte").Replace(fld.Type.String()); got != m[2] {
					t.Errorf("field %s.%s has type %s, v0.3.0 has %s", inType, name, got, m[2])
				}
			}
			continue
		}
		if m := typeRe.FindStringSubmatch(line); m != nil {
			typ, ok := types[m[1]]
			switch {
			case !ok:
				t.Errorf("type %s of v0.3.0 is missing", m[1])
			case typ != same[m[1]]:
				t.Errorf("type %s is not the type of goxmlgo/xmlreader", m[1])
			}
			if strings.HasSuffix(m[2], "{") {
				inType = m[1]
			} else if strings.HasPrefix(m[2], "struct{") {
				for _, name := range regexp.MustCompile(`\w+`).FindAllString(strings.TrimSuffix(strings.TrimPrefix(m[2], "struct{ "), " string }"), -1) {
					if _, ok := typ.FieldByName(name); !ok {
						t.Errorf("field %s.%s of v0.3.0 is missing", m[1], name)
					}
				}
			}
			continue
		}
		if m := methodRe.FindStringSubmatch(line); m != nil {
			if _, ok := reflect.PointerTo(types[m[1]]).MethodByName(m[2]); !ok {
				t.Errorf("method (*%s).%s of v0.3.0 is missing", m[1], m[2])
			}
			continue
		}
		if m := funcRe.FindStringSubmatch(line); m != nil {
			if !funcs[m[1]] {
				t.Errorf("function %s of v0.3.0 is not checked", m[1])
			}
			continue
		}
		t.Errorf("baseline: unparsed line %q", line)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 80 {
		t.Fatalf("baseline: only %d declarations read", n)
	}
	t.Logf("%d declarations of v0.3.0 checked", n)
}

// TestErrorBothPaths: błąd czytnika utworzonego przez dowolną ze ścieżek importu daje się odebrać przez errors.As
// jako *xmlreader.Error z obu ścieżek i jest tą samą wartością. (xsd nie zwraca *xmlreader.Error: błąd czytnika
// staje się xsd.Error z tym samym kodem albo treścią błędu ErrLimit lub errors.ErrUnsupported, jak w v0.3.0.)
// REQ: XML-020
func TestErrorBothPaths(t *testing.T) {
	const doc = "<a>\n  <b></a>"
	for name, r := range map[string]interface{ Next() (gx.Token, error) }{
		"goxsdgo/xmlreader.New": xmlreader.New(strings.NewReader(doc), xmlreader.Options{}),
		"goxmlgo/xmlreader.New": gx.New(strings.NewReader(doc), gx.Options{}),
	} {
		var err error
		for err == nil {
			_, err = r.Next()
		}
		var old *xmlreader.Error
		var cur *gx.Error
		if !errors.As(err, &old) || !errors.As(err, &cur) {
			t.Fatalf("%s: errors.As: goxsdgo/xmlreader %v, goxmlgo/xmlreader %v, error %v", name, old != nil, cur != nil, err)
		}
		if old != cur || old.Code != xmlreader.CodeElementTypeMatch || old.Line != 2 || old.Column != 6 {
			t.Errorf("%s: got %+v and %+v", name, old, cur)
		}
	}
}
