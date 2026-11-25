package datatypes_test

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxsdgo/internal/datatypes"
	"github.com/polestorm/goxsdgo/internal/testkit"
	"github.com/polestorm/goxsdgo/internal/w3c"
)

// Pomiar pośredni etapu 2 (nie kryterium K1, bo etap 3 jeszcze nie istnieje). Adapter w tym pliku czyta
// tylko schematy o prostym kształcie: deklaracje elementów globalnych z typem prostym i nazwane albo
// anonimowe typy proste (restrykcja, lista, unia). Taki kształt mają przypadki NIST i część przypadków
// Microsoft (DataTypes, Regex). Schemat albo dokument, którego adapter nie umie odczytać, nie jest liczony
// ani jako zaliczony, ani jako niezaliczony, tylko jako nieodczytany.

const (
	suiteRoot   = "../../testdata/w3c/xsdtests"
	xsdNS       = "http://www.w3.org/2001/XMLSchema"
	ratchetNIST = "../../testdata/w3c/nist-datatypes.ratchet"
	ratchetMS   = "../../testdata/w3c/ms-datatypes.ratchet"
)

var update = flag.Bool("update", false, "rewrite ratchet files")

// node to element dokumentu XML z przestrzeniami nazw w zasięgu.
type node struct {
	ns, local string
	attrs     map[string]string // nazwa lokalna atrybutu bez przestrzeni → wartość
	scope     map[string]string // prefiks → przestrzeń nazw
	children  []*node
	text      strings.Builder
	mixed     bool // ma elementy-dzieci
}

func (n *node) LookupPrefix(p string) (string, bool) {
	ns, ok := n.scope[p]
	if !ok && p == "" {
		return "", true
	}
	return ns, ok
}

func parseTree(r io.Reader) (*node, error) {
	dec := xml.NewDecoder(r)
	var stack []*node
	var root *node
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{attrs: map[string]string{}, scope: map[string]string{}}
			if len(stack) > 0 {
				for k, v := range stack[len(stack)-1].scope {
					n.scope[k] = v
				}
			}
			for i := range t.Attr {
				// Normalizacja wartości atrybutu CDATA (XML 1.0, 3.3.3): dosłowne #x9, #xA i #xD stają się spacją.
				// Adapter nie odróżnia ich od odwołań &#xD; (encoding/xml rozwija odwołania wcześniej); dokładnie
				// zrobi to czytnik z etapu 1.
				t.Attr[i].Value = strings.Map(func(r rune) rune {
					if r == '\t' || r == '\n' || r == '\r' {
						return ' '
					}
					return r
				}, t.Attr[i].Value)
			}
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "xmlns":
					n.scope[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					n.scope[""] = a.Value
				}
			}
			for _, a := range t.Attr {
				if a.Name.Space == "" && a.Name.Local != "xmlns" {
					n.attrs[a.Name.Local] = a.Value
				} else if a.Name.Space != "xmlns" && a.Name.Space != "" {
					n.attrs[n.scope[a.Name.Space]+" "+a.Name.Local] = a.Value
				}
			}
			n.ns, n.local = n.scope[t.Name.Space], t.Name.Local
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
				p.mixed = true
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("no root element")
	}
	return root, nil
}

// errUnsupported oznacza konstrukcję, której adapter nie czyta (np. typ złożony): przypadek nieodczytany.
var errUnsupported = errors.New("adapter: unsupported schema construct")

type schema struct {
	target   string
	types    map[string]*node // nazwa lokalna → xs:simpleType
	elements map[string]*node // nazwa lokalna → xs:element
	built    map[*node]*datatypes.Type
	building map[*node]bool
}

func loadSchema(fsys fs.FS, path string) (*schema, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	root, err := parseTree(f)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUnsupported, err)
	}
	if root.ns != xsdNS || root.local != "schema" {
		return nil, errUnsupported
	}
	s := &schema{target: root.attrs["targetNamespace"], types: map[string]*node{}, elements: map[string]*node{},
		built: map[*node]*datatypes.Type{}, building: map[*node]bool{}}
	for _, c := range root.children {
		if c.ns != xsdNS {
			return nil, errUnsupported
		}
		switch c.local {
		case "annotation":
		case "simpleType":
			s.types[c.attrs["name"]] = c
		case "element":
			s.elements[c.attrs["name"]] = c
		default:
			return nil, errUnsupported
		}
	}
	return s, nil
}

// resolve zwraca typ dla QName z atrybutu (type, base, itemType, memberTypes).
func (s *schema) resolve(ctx *node, qname string) (*datatypes.Type, error) {
	prefix, local, found := strings.Cut(qname, ":")
	if !found {
		prefix, local = "", qname
	}
	ns, ok := ctx.LookupPrefix(prefix)
	if !ok {
		return nil, errUnsupported
	}
	if ns == xsdNS {
		if t := datatypes.Builtin(local); t != nil {
			return t, nil
		}
		return nil, errUnsupported
	}
	if ns != s.target {
		return nil, errUnsupported
	}
	n, ok := s.types[local]
	if !ok {
		return nil, errUnsupported
	}
	return s.build(n)
}

// build buduje typ z elementu xs:simpleType: restrykcja, lista albo unia (część 2, 4.1.2).
func (s *schema) build(st *node) (*datatypes.Type, error) {
	if t, ok := s.built[st]; ok {
		return t, nil
	}
	if s.building[st] {
		return nil, errUnsupported // cykl: reguła etapu 3
	}
	s.building[st] = true
	defer delete(s.building, st)
	var deriv *node
	for _, c := range st.children {
		if c.ns == xsdNS && c.local != "annotation" {
			deriv = c
		}
	}
	if deriv == nil {
		return nil, errUnsupported
	}
	inline := func(n *node) []*node {
		var out []*node
		for _, c := range n.children {
			if c.ns == xsdNS && c.local == "simpleType" {
				out = append(out, c)
			}
		}
		return out
	}
	var t *datatypes.Type
	var err error
	switch deriv.local {
	case "restriction":
		var base *datatypes.Type
		if b, ok := deriv.attrs["base"]; ok {
			base, err = s.resolve(deriv, b)
		} else if in := inline(deriv); len(in) == 1 {
			base, err = s.build(in[0])
		} else {
			return nil, errUnsupported
		}
		if err != nil {
			return nil, err
		}
		var specs []datatypes.FacetSpec
		for _, c := range deriv.children {
			if c.ns != xsdNS || c.local == "annotation" || c.local == "simpleType" {
				continue
			}
			k, ok := datatypes.FacetByName(c.local)
			if !ok {
				return nil, errUnsupported
			}
			specs = append(specs, datatypes.FacetSpec{Kind: k, Value: c.attrs["value"], Fixed: c.attrs["fixed"] == "true"})
		}
		t, err = datatypes.Restrict(base, specs, &datatypes.Context{Namespaces: deriv})
	case "list":
		var item *datatypes.Type
		if it, ok := deriv.attrs["itemType"]; ok {
			item, err = s.resolve(deriv, it)
		} else if in := inline(deriv); len(in) == 1 {
			item, err = s.build(in[0])
		} else {
			return nil, errUnsupported
		}
		if err != nil {
			return nil, err
		}
		t, err = datatypes.NewList(item)
	case "union":
		var members []*datatypes.Type
		for _, q := range strings.Fields(deriv.attrs["memberTypes"]) {
			m, err := s.resolve(deriv, q)
			if err != nil {
				return nil, err
			}
			members = append(members, m)
		}
		for _, in := range inline(deriv) {
			m, err := s.build(in)
			if err != nil {
				return nil, err
			}
			members = append(members, m)
		}
		t, err = datatypes.NewUnion(members...)
	default:
		return nil, errUnsupported
	}
	if err != nil {
		return nil, err
	}
	s.built[st] = t
	return t, nil
}

// elementType zwraca typ prosty deklaracji elementu.
func (s *schema) elementType(el *node) (*datatypes.Type, error) {
	for k := range el.attrs {
		switch k {
		case "name", "type", "id":
		default:
			return nil, errUnsupported // nillable, default, fixed, abstract… należą do etapów 3–5
		}
	}
	if q, ok := el.attrs["type"]; ok {
		return s.resolve(el, q)
	}
	for _, c := range el.children {
		if c.ns == xsdNS && c.local == "simpleType" {
			return s.build(c)
		}
		if c.ns == xsdNS && c.local != "annotation" {
			return nil, errUnsupported
		}
	}
	return datatypes.AnySimpleType(), nil
}

// container to jedyny kształt typu złożonego, jaki czyta adapter (przypadki MS-Regex i MS-DataTypes):
// opcjonalnie jeden element lokalny o typie prostym w xs:sequence albo xs:choice oraz atrybuty o typach
// prostych. Liczność i atrybuty sprawdzamy według cvc-complex-type (część 1) tylko w tym zakresie.
type container struct {
	child              string
	childType          *datatypes.Type
	minOccurs          int
	maxOccurs          int // -1: unbounded
	attrs              map[string]*datatypes.Type
	required           map[string]bool
	hasParticle, empty bool
}

func (s *schema) containerType(ct *node) (*container, error) {
	if len(ct.attrs) != 0 {
		return nil, errUnsupported
	}
	c := &container{attrs: map[string]*datatypes.Type{}, required: map[string]bool{}, minOccurs: 1, maxOccurs: 1}
	occurs := func(v string, def int) (int, bool) {
		if v == "" {
			return def, true
		}
		if v == "unbounded" {
			return -1, true
		}
		n := 0
		for _, r := range v {
			if r < '0' || r > '9' || n > 1e6 {
				return 0, false
			}
			n = n*10 + int(r-'0')
		}
		return n, v != ""
	}
	for _, k := range ct.children {
		if k.ns != xsdNS {
			return nil, errUnsupported
		}
		switch k.local {
		case "annotation":
		case "sequence", "choice":
			if c.hasParticle || len(k.attrs) != 0 {
				return nil, errUnsupported
			}
			var els []*node
			for _, e := range k.children {
				if e.ns == xsdNS && e.local == "element" {
					els = append(els, e)
				} else if !(e.ns == xsdNS && e.local == "annotation") {
					return nil, errUnsupported
				}
			}
			if len(els) != 1 {
				return nil, errUnsupported
			}
			e := els[0]
			for a := range e.attrs {
				if a != "name" && a != "type" && a != "minOccurs" && a != "maxOccurs" {
					return nil, errUnsupported
				}
			}
			var ok1, ok2 bool
			if c.minOccurs, ok1 = occurs(e.attrs["minOccurs"], 1); !ok1 || c.minOccurs < 0 {
				return nil, errUnsupported
			}
			if c.maxOccurs, ok2 = occurs(e.attrs["maxOccurs"], 1); !ok2 {
				return nil, errUnsupported
			}
			decl := &node{attrs: map[string]string{"name": e.attrs["name"]}, scope: e.scope, children: e.children, ns: e.ns, local: e.local}
			if t, ok := e.attrs["type"]; ok {
				decl.attrs["type"] = t
			}
			t, err := s.elementType(decl)
			if err != nil {
				return nil, err
			}
			c.child, c.childType, c.hasParticle = e.attrs["name"], t, true
		case "attribute":
			for a := range k.attrs {
				if a != "name" && a != "type" && a != "use" {
					return nil, errUnsupported
				}
			}
			var t *datatypes.Type
			var err error
			if q, ok := k.attrs["type"]; ok {
				t, err = s.resolve(k, q)
			} else {
				t = datatypes.AnySimpleType()
				for _, st := range k.children {
					if st.ns == xsdNS && st.local == "simpleType" {
						t, err = s.build(st)
					}
				}
			}
			if err != nil {
				return nil, err
			}
			name := k.attrs["name"]
			c.attrs[name] = t
			switch k.attrs["use"] {
			case "required":
				c.required[name] = true
			case "", "optional":
			default:
				return nil, errUnsupported
			}
		default:
			return nil, errUnsupported
		}
	}
	return c, nil
}

func complexOf(el *node) *node {
	for _, c := range el.children {
		if c.ns == xsdNS && c.local == "complexType" {
			return c
		}
	}
	return nil
}

// validateContainer sprawdza dokument według typu container; błędy struktury dostają kody z części 1.
func validateContainer(c *container, root *node) (w3c.Validity, string) {
	for k, v := range root.attrs {
		if strings.HasPrefix(k, "http://www.w3.org/2001/XMLSchema-instance ") {
			continue
		}
		t, ok := c.attrs[k]
		if !ok || strings.Contains(k, " ") {
			return w3c.Invalid, "cvc-complex-type.3.2.2"
		}
		if _, err := t.Validate(v, &datatypes.Context{Namespaces: root}); err != nil {
			return w3c.Invalid, err.(*datatypes.Error).Code
		}
	}
	for name := range c.required {
		if _, ok := root.attrs[name]; !ok {
			return w3c.Invalid, "cvc-complex-type.4"
		}
	}
	if !c.hasParticle {
		if root.mixed || root.text.Len() != 0 {
			return w3c.Invalid, "cvc-complex-type.2.1"
		}
		return w3c.Valid, ""
	}
	if strings.TrimSpace(root.text.String()) != "" {
		return w3c.Invalid, "cvc-complex-type.2.3"
	}
	n := 0
	for _, ch := range root.children {
		if ch.local != c.child || ch.ns != "" || ch.mixed {
			return w3c.Invalid, "cvc-complex-type.2.4"
		}
		for k := range ch.attrs {
			if !strings.HasPrefix(k, "http://www.w3.org/2001/XMLSchema-instance ") {
				return w3c.Invalid, "cvc-complex-type.3.2.2"
			}
			return "", "xsi attribute on a child"
		}
		n++
		text := ch.text.String()
		if _, err := c.childType.Validate(text, &datatypes.Context{Namespaces: ch}); err != nil {
			return w3c.Invalid, err.(*datatypes.Error).Code
		}
	}
	if n < c.minOccurs || (c.maxOccurs >= 0 && n > c.maxOccurs) {
		return w3c.Invalid, "cvc-complex-type.2.4"
	}
	return w3c.Valid, ""
}

// verdict zwraca werdykt adaptera: "valid", "invalid" albo "" dla przypadku nieodczytanego.
func verdict(fsys fs.FS, c *w3c.Case) (w3c.Validity, string) {
	if len(c.SchemaDocs) != 1 {
		return "", "several schema documents"
	}
	s, err := loadSchema(fsys, c.SchemaDocs[0])
	if err != nil {
		return "", err.Error()
	}
	var dtErr *datatypes.Error
	classify := func(err error) (w3c.Validity, string) {
		if errors.As(err, &dtErr) {
			return w3c.Invalid, dtErr.Code
		}
		return "", err.Error()
	}
	names := make([]string, 0, len(s.types))
	for n := range s.types {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, err := s.build(s.types[n]); err != nil {
			return classify(err)
		}
	}
	containers := map[string]*container{}
	for name, el := range s.elements {
		if ct := complexOf(el); ct != nil {
			for a := range el.attrs {
				if a != "name" {
					return "", "element attributes not supported"
				}
			}
			c, err := s.containerType(ct)
			if err != nil {
				return classify(err)
			}
			containers[name] = c
			continue
		}
		if _, err := s.elementType(el); err != nil {
			return classify(err)
		}
	}
	if c.Kind == w3c.SchemaTest {
		return w3c.Valid, ""
	}
	f, err := fsys.Open(c.Instance)
	if err != nil {
		return "", err.Error()
	}
	defer f.Close()
	root, err := parseTree(f)
	if err != nil {
		return "", "instance not readable by the adapter"
	}
	if root.ns != s.target {
		return "", "instance shape not supported"
	}
	if c, ok := containers[root.local]; ok {
		if s.target != "" {
			return "", "qualified local elements not supported"
		}
		return validateContainer(c, root)
	}
	if root.mixed {
		return "", "instance shape not supported"
	}
	for k := range root.attrs {
		if !strings.HasPrefix(k, "http://www.w3.org/2001/XMLSchema-instance ") {
			return "", "instance attributes not supported"
		}
		if k != "http://www.w3.org/2001/XMLSchema-instance schemaLocation" && k != "http://www.w3.org/2001/XMLSchema-instance noNamespaceSchemaLocation" {
			return "", "xsi attribute not supported"
		}
	}
	el, ok := s.elements[root.local]
	if !ok {
		return w3c.Invalid, "undeclared root element"
	}
	t, err := s.elementType(el)
	if err != nil {
		return classify(err)
	}
	// Końce wierszy (XML 1.0, 2.11) normalizuje już encoding/xml; odwołanie &#xD; zostaje znakiem CR.
	text := root.text.String()
	if _, err := t.Validate(text, &datatypes.Context{Namespaces: root}); err != nil {
		return classify(err)
	}
	return w3c.Valid, ""
}

func TestW3CDatatypes(t *testing.T) {
	if _, err := os.Stat(suiteRoot + "/suite.xml"); err != nil {
		t.Skip(testkit.FetchHint)
	}
	fsys := os.DirFS(suiteRoot)
	cases, err := w3c.Load(fsys, w3c.Tokens1_0)
	if err != nil {
		t.Fatal(err)
	}
	type stat struct{ inScope, read, passed int }
	byGroup := map[string]map[string]*stat{"NIST": {}, "Microsoft": {}}
	passed := map[string]map[string]bool{"NIST": {}, "Microsoft": {}}
	inScope := map[string]map[string]bool{"NIST": {}, "Microsoft": {}}
	var failures []string
	for _, c := range cases {
		groups, ok := byGroup[c.Contributor]
		if !ok || !c.InScope() {
			continue
		}
		if c.Contributor == "Microsoft" && !strings.HasPrefix(c.Set, "MS-DataTypes") && !strings.HasPrefix(c.Set, "MS-Regex") {
			continue
		}
		key := c.Set
		if c.Contributor == "NIST" {
			// Grupy NIST nazywają się np. atomic-decimal-minExclusive-3: odmiana, typ bazowy, aspekt.
			parts := strings.Split(c.Group, "-")
			if len(parts) >= 3 {
				key = parts[0] + "-" + parts[1]
			}
		}
		st := groups[key]
		if st == nil {
			st = &stat{}
			groups[key] = st
		}
		st.inScope++
		inScope[c.Contributor][c.ID] = true
		got, why := verdict(fsys, c)
		if got == "" {
			continue
		}
		st.read++
		if c.Matches(got) {
			st.passed++
			passed[c.Contributor][c.ID] = true
		} else if len(failures) < 40 {
			failures = append(failures, fmt.Sprintf("%s: got %s (%s), want %s", c.ID, got, why, c.Expected))
		}
	}
	for _, who := range []string{"NIST", "Microsoft"} {
		var b strings.Builder
		keys := make([]string, 0)
		for k := range byGroup[who] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var tot stat
		for _, k := range keys {
			s := byGroup[who][k]
			tot.inScope, tot.read, tot.passed = tot.inScope+s.inScope, tot.read+s.read, tot.passed+s.passed
			fmt.Fprintf(&b, "  %-28s passed %5d / read %5d / in scope %5d\n", k, s.passed, s.read, s.inScope)
		}
		t.Logf("%s datatypes (adapter, stage 2): passed %d / read %d / in scope %d\n%s", who, tot.passed, tot.read, tot.inScope, b.String())
	}
	for _, f := range failures {
		t.Log("fail: " + f)
	}
	for who, path := range map[string]string{"NIST": ratchetNIST, "Microsoft": ratchetMS} {
		header := who + " datatype cases passing through the stage 2 adapter (internal/datatypes, TestW3CDatatypes, -update).\n" +
			"Intermediate measure, not criterion K1."
		testkit.Ratchet(t, path, header, inScope[who], passed[who], *update)
	}
}
