package schema

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/polestorm/goxsdgo/internal/datatypes"
	"github.com/polestorm/goxsdgo/internal/xmlchars"
)

// Kontrola dokumentu schematu według schematu dla schematów (część 1, dodatek A; część 2, dodatek A). Każdy element
// z przestrzeni XSD ma kontekst wynikający z rodzica (np. xs:element w xs:schema ma typ topLevelElement, w xs:all
// typ narrowMaxMin). Kontekst określa dozwolone dzieci (wyrażenie nad nazwami dzieci), dozwolone atrybuty bez
// przestrzeni nazw z ich typami i atrybuty wymagane. Atrybuty z innych przestrzeni są dozwolone (anyAttribute
// ##other lax); z przestrzeni xml sprawdzamy według xml.xsd, bo schemat dla schematów go importuje.

// attrKind to typ wartości atrybutu w schemacie dla schematów.
type attrKind uint8

const (
	akID attrKind = iota
	akNCName
	akQName
	akQNameList
	akAnyURI
	akToken
	akString
	akAnySimple
	akBoolean
	akNonNegInt
	akPosInt
	akAllNNI
	akZeroOne    // minOccurs w xs:all: 0 albo 1
	akOne        // maxOccurs xs:all: 1
	akZeroOneNNI // maxOccurs elementu w xs:all: 0 albo 1
	akForm
	akDerivationSet       // #all | (extension | restriction)*
	akBlockSet            // #all | (extension | restriction | substitution)*
	akFullDerivationSet   // #all | (extension | restriction | list | union)*
	akSimpleDerivationSet // #all | (list | union | restriction)*
	akUse
	akProcessContents
	akNamespaceList
	akWhiteSpace
)

type sfsContext struct {
	// content to wyrażenie nad nazwami dzieci; "*" oznacza dowolną treść (appinfo, documentation).
	content  string
	attrs    map[string]attrKind
	required []string
	// mixed: dozwolony tekst (appinfo, documentation).
	mixed bool
	re    *regexp.Regexp
}

const facetNames = "(minExclusive|minInclusive|maxExclusive|maxInclusive|totalDigits|fractionDigits|length|minLength|maxLength|enumeration|whiteSpace|pattern)"

func attrs(kv ...any) map[string]attrKind {
	m := map[string]attrKind{"id": akID}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1].(attrKind)
	}
	return m
}

var sfsContexts = map[string]*sfsContext{
	"schema": {content: "(include|import|redefine|annotation)*((simpleType|complexType|group|attributeGroup|element|attribute|notation)annotation*)*",
		attrs: attrs("targetNamespace", akAnyURI, "version", akToken, "finalDefault", akFullDerivationSet,
			"blockDefault", akBlockSet, "attributeFormDefault", akForm, "elementFormDefault", akForm)},
	"annotation":    {content: "(appinfo|documentation)*", attrs: attrs()},
	"appinfo":       {content: "*", mixed: true, attrs: map[string]attrKind{"source": akAnyURI}},
	"documentation": {content: "*", mixed: true, attrs: map[string]attrKind{"source": akAnyURI}},

	"topElement": {content: "annotation?(simpleType|complexType)?(unique|key|keyref)*",
		attrs: attrs("name", akNCName, "type", akQName, "substitutionGroup", akQName, "default", akString,
			"fixed", akString, "nillable", akBoolean, "abstract", akBoolean, "final", akDerivationSet, "block", akBlockSet),
		required: []string{"name"}},
	"localElement": {content: "annotation?(simpleType|complexType)?(unique|key|keyref)*",
		attrs: attrs("name", akNCName, "ref", akQName, "type", akQName, "minOccurs", akNonNegInt, "maxOccurs", akAllNNI,
			"default", akString, "fixed", akString, "nillable", akBoolean, "block", akBlockSet, "form", akForm)},
	"allElement": {content: "annotation?(simpleType|complexType)?(unique|key|keyref)*",
		attrs: attrs("name", akNCName, "ref", akQName, "type", akQName, "minOccurs", akZeroOne, "maxOccurs", akZeroOneNNI,
			"default", akString, "fixed", akString, "nillable", akBoolean, "block", akBlockSet, "form", akForm)},

	"topAttribute": {content: "annotation?simpleType?",
		attrs:    attrs("name", akNCName, "type", akQName, "default", akString, "fixed", akString),
		required: []string{"name"}},
	"attribute": {content: "annotation?simpleType?",
		attrs: attrs("name", akNCName, "ref", akQName, "type", akQName, "use", akUse, "default", akString,
			"fixed", akString, "form", akForm)},

	"topComplexType": {content: "annotation?(simpleContent|complexContent|((group|all|choice|sequence)?(attribute|attributeGroup)*anyAttribute?))",
		attrs:    attrs("name", akNCName, "mixed", akBoolean, "abstract", akBoolean, "final", akDerivationSet, "block", akDerivationSet),
		required: []string{"name"}},
	"localComplexType": {content: "annotation?(simpleContent|complexContent|((group|all|choice|sequence)?(attribute|attributeGroup)*anyAttribute?))",
		attrs: attrs("mixed", akBoolean)},
	"simpleContent":      {content: "annotation?(restriction|extension)", attrs: attrs()},
	"complexContent":     {content: "annotation?(restriction|extension)", attrs: attrs("mixed", akBoolean)},
	"simpleRestriction":  {content: "annotation?(simpleType?" + facetNames + "*)?(attribute|attributeGroup)*anyAttribute?", attrs: attrs("base", akQName), required: []string{"base"}},
	"simpleExtension":    {content: "annotation?(attribute|attributeGroup)*anyAttribute?", attrs: attrs("base", akQName), required: []string{"base"}},
	"complexRestriction": {content: "annotation?(group|all|choice|sequence)?(attribute|attributeGroup)*anyAttribute?", attrs: attrs("base", akQName), required: []string{"base"}},
	"complexExtension":   {content: "annotation?(group|all|choice|sequence)?(attribute|attributeGroup)*anyAttribute?", attrs: attrs("base", akQName), required: []string{"base"}},

	"namedGroup":          {content: "annotation?(all|choice|sequence)", attrs: attrs("name", akNCName), required: []string{"name"}},
	"groupRef":            {content: "annotation?", attrs: attrs("ref", akQName, "minOccurs", akNonNegInt, "maxOccurs", akAllNNI), required: []string{"ref"}},
	"explicitGroup":       {content: "annotation?(element|group|choice|sequence|any)*", attrs: attrs("minOccurs", akNonNegInt, "maxOccurs", akAllNNI)},
	"simpleExplicitGroup": {content: "annotation?(element|group|choice|sequence|any)*", attrs: attrs()},
	"all":                 {content: "annotation?element*", attrs: attrs("minOccurs", akZeroOne, "maxOccurs", akOne)},
	"namedAll":            {content: "annotation?element*", attrs: attrs()},
	"any":                 {content: "annotation?", attrs: attrs("namespace", akNamespaceList, "processContents", akProcessContents, "minOccurs", akNonNegInt, "maxOccurs", akAllNNI)},
	"anyAttribute":        {content: "annotation?", attrs: attrs("namespace", akNamespaceList, "processContents", akProcessContents)},

	"namedAttributeGroup": {content: "annotation?(attribute|attributeGroup)*anyAttribute?", attrs: attrs("name", akNCName), required: []string{"name"}},
	"attributeGroupRef":   {content: "annotation?", attrs: attrs("ref", akQName), required: []string{"ref"}},

	"include":  {content: "annotation?", attrs: attrs("schemaLocation", akAnyURI), required: []string{"schemaLocation"}},
	"import":   {content: "annotation?", attrs: attrs("namespace", akAnyURI, "schemaLocation", akAnyURI)},
	"redefine": {content: "(annotation|simpleType|complexType|group|attributeGroup)*", attrs: attrs("schemaLocation", akAnyURI), required: []string{"schemaLocation"}},
	"notation": {content: "annotation?", attrs: attrs("name", akNCName, "public", akToken, "system", akAnyURI), required: []string{"name"}},
	"keybase":  {content: "annotation?selector(field)+", attrs: attrs("name", akNCName), required: []string{"name"}},
	"keyref":   {content: "annotation?selector(field)+", attrs: attrs("name", akNCName, "refer", akQName), required: []string{"name", "refer"}},
	"selector": {content: "annotation?", attrs: attrs("xpath", akToken), required: []string{"xpath"}},
	"field":    {content: "annotation?", attrs: attrs("xpath", akToken), required: []string{"xpath"}},

	"topSimpleType":   {content: "annotation?(restriction|list|union)", attrs: attrs("name", akNCName, "final", akSimpleDerivationSet), required: []string{"name"}},
	"localSimpleType": {content: "annotation?(restriction|list|union)", attrs: attrs()},
	"stRestriction":   {content: "annotation?simpleType?" + facetNames + "*", attrs: attrs("base", akQName)},
	"list":            {content: "annotation?simpleType?", attrs: attrs("itemType", akQName)},
	"union":           {content: "annotation?simpleType*", attrs: attrs("memberTypes", akQNameList)},

	"facet":        {content: "annotation?", attrs: attrs("value", akAnySimple, "fixed", akBoolean), required: []string{"value"}},
	"numFacet":     {content: "annotation?", attrs: attrs("value", akNonNegInt, "fixed", akBoolean), required: []string{"value"}},
	"totalDigits":  {content: "annotation?", attrs: attrs("value", akPosInt, "fixed", akBoolean), required: []string{"value"}},
	"noFixedFacet": {content: "annotation?", attrs: attrs("value", akAnySimple), required: []string{"value"}},
	"pattern":      {content: "annotation?", attrs: attrs("value", akString), required: []string{"value"}},
	"whiteSpace":   {content: "annotation?", attrs: attrs("value", akWhiteSpace, "fixed", akBoolean), required: []string{"value"}},
}

// childNames to wszystkie nazwy elementów XSD, które mogą wystąpić w modelach treści; każda dostaje jedną literę.
var childNames = []string{"include", "import", "redefine", "annotation", "simpleType", "complexType", "group",
	"attributeGroup", "element", "attribute", "notation", "appinfo", "documentation", "unique", "key", "keyref",
	"simpleContent", "complexContent", "all", "choice", "sequence", "anyAttribute", "restriction", "extension",
	"any", "selector", "field", "list", "union", "minExclusive", "minInclusive", "maxExclusive", "maxInclusive",
	"totalDigits", "fractionDigits", "length", "minLength", "maxLength", "enumeration", "whiteSpace", "pattern"}

var (
	childLetter map[string]rune
	sfsOnce     sync.Once
)

// foreignLetter oznacza dziecko spoza znanych nazw XSD (inna przestrzeń albo nieznana nazwa): nie pasuje do
// żadnego modelu.
const foreignLetter = '~'

func sfsInit() {
	childLetter = map[string]rune{}
	// Znaki z obszaru prywatnego Unicode: nie są metaznakami wyrażeń regularnych i nie występują w modelach.
	for i, n := range childNames {
		childLetter[n] = rune(0xE000 + i)
	}
	// Zamieniamy nazwy na litery od najdłuższych, żeby np. "attributeGroup" nie zostało rozbite przez "attribute".
	names := append([]string(nil), childNames...)
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && len(names[j]) > len(names[j-1]); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	for key, c := range sfsContexts {
		if c.content == "*" {
			continue
		}
		expr := c.content
		for _, n := range names {
			expr = replaceWord(expr, n, string(childLetter[n]))
		}
		c.re = regexp.MustCompile("^(?:" + expr + ")$")
		_ = key
	}
}

// replaceWord zamienia w wyrażeniu całe słowa name na repl (słowo: ciąg liter ograniczony znakami spoza liter).
func replaceWord(expr, name, repl string) string {
	var b strings.Builder
	isLetter := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	for i := 0; i < len(expr); {
		if strings.HasPrefix(expr[i:], name) && (i == 0 || !isLetter(expr[i-1])) &&
			(i+len(name) == len(expr) || !isLetter(expr[i+len(name)])) {
			b.WriteString(repl)
			i += len(name)
			continue
		}
		b.WriteByte(expr[i])
		i++
	}
	return b.String()
}

// contextOf zwraca klucz kontekstu elementu XSD albo "" dla nazwy spoza schematu dla schematów.
func contextOf(n *node) string {
	p := ""
	if n.parent != nil && n.parent.space == NamespaceXSD {
		p = n.parent.local
	}
	topLevel := p == "schema" || p == "redefine"
	switch n.local {
	case "schema", "annotation", "appinfo", "documentation", "simpleContent", "complexContent", "any", "anyAttribute",
		"include", "import", "redefine", "notation", "keyref", "selector", "field", "list", "union", "pattern",
		"whiteSpace", "totalDigits":
		return n.local
	case "element":
		switch p {
		case "schema":
			return "topElement"
		case "all":
			return "allElement"
		}
		return "localElement"
	case "attribute":
		if p == "schema" {
			return "topAttribute"
		}
		return "attribute"
	case "complexType":
		if topLevel {
			return "topComplexType"
		}
		return "localComplexType"
	case "simpleType":
		if topLevel {
			return "topSimpleType"
		}
		return "localSimpleType"
	case "group":
		if topLevel {
			return "namedGroup"
		}
		return "groupRef"
	case "attributeGroup":
		if topLevel {
			return "namedAttributeGroup"
		}
		return "attributeGroupRef"
	case "all":
		if p == "group" && n.parent.parent != nil && (n.parent.parent.is("schema") || n.parent.parent.is("redefine")) {
			return "namedAll"
		}
		return "all"
	case "choice", "sequence":
		if p == "group" && n.parent.parent != nil && (n.parent.parent.is("schema") || n.parent.parent.is("redefine")) {
			return "simpleExplicitGroup"
		}
		return "explicitGroup"
	case "restriction":
		switch p {
		case "simpleContent":
			return "simpleRestriction"
		case "complexContent":
			return "complexRestriction"
		}
		return "stRestriction"
	case "extension":
		if p == "simpleContent" {
			return "simpleExtension"
		}
		return "complexExtension"
	case "unique", "key":
		return "keybase"
	case "minExclusive", "minInclusive", "maxExclusive", "maxInclusive":
		return "facet"
	case "fractionDigits", "length", "minLength", "maxLength":
		return "numFacet"
	case "enumeration":
		return "noFixedFacet"
	}
	return ""
}

// checkSFS sprawdza poddrzewo elementu XSD n według schematu dla schematów i zbiera błędy.
func (l *loader) checkSFS(n *node, ids map[string]bool) {
	sfsOnce.Do(sfsInit)
	key := contextOf(n)
	ctx := sfsContexts[key]
	if ctx == nil {
		l.errorf(CodeContent, n.pos(), "element %s is not allowed here", n.name())
		return
	}
	// Atrybuty.
	for _, a := range n.attrs {
		pos := Position{Document: n.doc.path, Line: a.line, Column: a.col}
		switch a.space {
		case "":
			kind, ok := ctx.attrs[a.local]
			if !ok {
				l.errorf(CodeAttribute, pos, "attribute %s is not allowed on %s", a.local, n.name())
				continue
			}
			if msg := checkAttrValue(kind, a.value, n.doc.names); msg != "" {
				l.errorf(CodeAttributeValue, pos, "attribute %s of %s: %s", a.local, n.name(), msg)
				continue
			}
			if kind == akID {
				v := collapse(a.value)
				if ids[v] {
					l.errorf(CodeDuplicateID, pos, "id %q is not unique in the schema document", v)
				}
				ids[v] = true
			}
		case NamespaceXSD:
			l.errorf(CodeAttribute, pos, "attribute %s from the XML Schema namespace is not allowed on %s", a.local, n.name())
		case NamespaceXML:
			kind := akString
			switch a.local {
			case "lang":
				kind = akToken // sprawdzane niżej jako language
				if _, err := datatypes.Builtin("language").Validate(a.value, nil); err != nil {
					l.errorf(CodeAttributeValue, pos, "xml:lang %q is not a language tag", a.value)
				}
				continue
			case "space":
				if v := collapse(a.value); v != "default" && v != "preserve" {
					l.errorf(CodeAttributeValue, pos, "xml:space must be default or preserve")
				}
				continue
			case "base":
				kind = akAnyURI
			case "id":
				kind = akID
			}
			if msg := checkAttrValue(kind, a.value, n.doc.names); msg != "" {
				l.errorf(CodeAttributeValue, pos, "attribute xml:%s: %s", a.local, msg)
			}
		}
	}
	for _, r := range ctx.required {
		if !n.has(r) {
			l.errorf(CodeAttributeMissing, n.pos(), "%s requires attribute %s", n.name(), r)
		}
	}
	if ctx.content == "*" {
		return
	}
	if n.text && !ctx.mixed {
		l.errorf(CodeText, Position{Document: n.doc.path, Line: n.textLine, Column: n.textCol},
			"text is not allowed in %s", n.name())
	}
	var seq []rune
	for _, k := range n.kids {
		c, ok := childLetter[k.local]
		if k.space != NamespaceXSD || !ok {
			c = foreignLetter
		}
		seq = append(seq, c)
	}
	if !ctx.re.MatchString(string(seq)) {
		l.errorf(CodeContent, n.pos(), "content of %s does not match %s", n.name(), describeContent(ctx.content, n))
	}
	for _, k := range n.kids {
		if k.space == NamespaceXSD {
			l.checkSFS(k, ids)
		}
	}
}

func describeContent(model string, n *node) string {
	var names []string
	for _, k := range n.kids {
		names = append(names, k.name())
	}
	return fmt.Sprintf("(%s); found (%s)", model, strings.Join(names, ", "))
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// checkAttrValue sprawdza wartość atrybutu według jego typu i zwraca opis błędu albo "".
func checkAttrValue(kind attrKind, v string, ed xmlchars.Edition) string {
	builtin := func(name string) string {
		if _, err := datatypes.Builtin(name).Validate(v, &datatypes.Context{Names: ed}); err != nil {
			return fmt.Sprintf("%q is not a valid %s", v, name)
		}
		return ""
	}
	tokens := func(allowed ...string) string {
		set := map[string]bool{}
		for _, a := range allowed {
			set[a] = true
		}
		f := strings.Fields(v)
		if len(f) == 1 && f[0] == "#all" {
			return ""
		}
		for _, t := range f {
			if !set[t] {
				return fmt.Sprintf("%q is not #all or a list of %s", v, strings.Join(allowed, ", "))
			}
		}
		return ""
	}
	oneOf := func(allowed ...string) string {
		c := collapse(v)
		for _, a := range allowed {
			if c == a {
				return ""
			}
		}
		return fmt.Sprintf("%q must be one of %s", v, strings.Join(allowed, ", "))
	}
	switch kind {
	case akID:
		return builtin("ID")
	case akNCName:
		return builtin("NCName")
	case akQName:
		if !validQNameLexical(collapse(v), ed) {
			return fmt.Sprintf("%q is not a valid QName", v)
		}
	case akQNameList:
		for _, f := range strings.Fields(v) {
			if !validQNameLexical(f, ed) {
				return fmt.Sprintf("%q is not a valid QName", f)
			}
		}
	case akAnyURI:
		return builtin("anyURI")
	case akToken:
		return builtin("token")
	case akString, akAnySimple:
	case akBoolean:
		return builtin("boolean")
	case akNonNegInt:
		return builtin("nonNegativeInteger")
	case akPosInt:
		return builtin("positiveInteger")
	case akAllNNI:
		if collapse(v) == "unbounded" {
			return ""
		}
		return builtin("nonNegativeInteger")
	case akZeroOne:
		if msg := builtin("nonNegativeInteger"); msg != "" {
			return msg
		}
		if n := strings.TrimLeft(collapse(v), "+0"); n != "" && n != "1" {
			return fmt.Sprintf("%q must be 0 or 1", v)
		}
	case akOne:
		if msg := builtin("nonNegativeInteger"); msg != "" {
			return msg
		}
		if n := strings.TrimLeft(collapse(v), "+0"); n != "1" {
			return fmt.Sprintf("%q must be 1", v)
		}
	case akZeroOneNNI:
		c := collapse(v)
		if c == "unbounded" {
			return fmt.Sprintf("%q must be 0 or 1", v)
		}
		if msg := builtin("nonNegativeInteger"); msg != "" {
			return msg
		}
		if n := strings.TrimLeft(c, "+0"); n != "" && n != "1" {
			return fmt.Sprintf("%q must be 0 or 1", v)
		}
	case akForm:
		return oneOf("qualified", "unqualified")
	case akDerivationSet:
		return tokens("extension", "restriction")
	case akBlockSet:
		return tokens("extension", "restriction", "substitution")
	case akFullDerivationSet:
		return tokens("extension", "restriction", "list", "union")
	case akSimpleDerivationSet:
		return tokens("list", "union", "restriction")
	case akUse:
		return oneOf("prohibited", "optional", "required")
	case akProcessContents:
		return oneOf("skip", "lax", "strict")
	case akWhiteSpace:
		return oneOf("preserve", "replace", "collapse")
	case akNamespaceList:
		f := strings.Fields(v)
		if len(f) == 1 && (f[0] == "##any" || f[0] == "##other") {
			return ""
		}
		for _, t := range f {
			if t == "##targetNamespace" || t == "##local" {
				continue
			}
			if _, err := datatypes.Builtin("anyURI").Validate(t, nil); err != nil || strings.HasPrefix(t, "##") {
				return fmt.Sprintf("%q is not a valid namespace list", v)
			}
		}
	}
	return ""
}

// validQNameLexical sprawdza postać leksykalną QName ([7] QName z Namespaces in XML).
func validQNameLexical(s string, ed xmlchars.Edition) bool {
	prefix, local, ok := strings.Cut(s, ":")
	if !ok {
		return validNCName(s, ed)
	}
	return validNCName(prefix, ed) && validNCName(local, ed)
}

func validNCName(s string, ed xmlchars.Edition) bool {
	if s == "" || strings.Contains(s, ":") {
		return false
	}
	_, err := datatypes.Builtin("NCName").Validate(s, &datatypes.Context{Names: ed})
	return err == nil && !strings.ContainsAny(s, " \t\r\n")
}
