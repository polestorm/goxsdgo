// Package validate waliduje dokument XML według skompilowanego schematu (XSD 1.0 Second Edition, część 1,
// rozdział 3: Validation Rules, i rozdział 5) strumieniowo: w jednym przejściu czytnika xmlreader, bez
// budowania drzewa. Pamięć zależy od głębokości dokumentu, od tabel ID/IDREF i od ograniczeń tożsamości, nie od
// rozmiaru pliku (ADR 0008).
//
// Pakiet służy bibliotece (xsd.Set.Validate); stan walidacji jest osobny dla każdego wywołania, więc jeden schemat
// może walidować wiele dokumentów równocześnie.
package validate

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/polestorm/goxsdgo/internal/datatypes"
	"github.com/polestorm/goxsdgo/internal/schema"
	"github.com/polestorm/goxsdgo/internal/xmlchars"
	"github.com/polestorm/goxsdgo/xmlreader"
)

// Hint to wskazówka lokalizacji schematu z xsi:schemaLocation (para przestrzeń, adres) albo
// xsi:noNamespaceSchemaLocation (Namespace pusta).
type Hint struct {
	Namespace, Location string
}

// HintLoader dostarcza schemat poszerzony o dokumenty ze wskazówek (4.3.2). Zwraca nowy schemat, ten sam (nil,
// gdy wskazówki nic nie wnoszą) albo błąd: schema.ErrorList dla niepoprawnego schematu, błąd opakowujący
// errors.ErrUnsupported dla konstrukcji nieobsługiwanej.
type HintLoader interface {
	LoadHints(hints []Hint) (*schema.Schema, error)
}

// Options ustawia walidację. Wartość zerowa to ustawienia domyślne.
type Options struct {
	// MaxErrors przerywa walidację po tylu błędach (ErrTooManyErrors). 0: 1000; wartość ujemna: bez limitu.
	MaxErrors int
	// MaxIdentityEntries ogranicza łączną liczbę wpisów tabel ograniczeń tożsamości i ID w pamięci. 0: 4 mln.
	MaxIdentityEntries int
	// MaxTextSize ogranicza tekst jednego elementu o treści prostej, w bajtach. 0: 16 MiB.
	MaxTextSize int
	// Hints, jeśli nie nil, wczytuje schematy ze wskazówek xsi:schemaLocation (domyślnie wyłączone, ADR 0001).
	Hints HintLoader
	// Reader to limity czytnika.
	Reader xmlreader.Limits
	// OnValue, jeśli nie nil, dostaje każdą poprawną wartość typu prostego: treść elementu o typie prostym albo
	// treści prostej i wartość atrybutu (pytanie 6, ADR 0009). Wywoływana synchronicznie, w kolejności dokumentu.
	OnValue func(ValueEvent)
	// Names to reguły znaków nazw dla wartości typów nazw i wzorców z \i, \c (ADR 0012).
	Names xmlchars.Edition
}

// ValueEvent to poprawna wartość typu prostego w dokumencie.
type ValueEvent struct {
	// Path to ścieżka elementu albo atrybutu (ADR 0008); Name to nazwa lokalna; Attribute: wartość atrybutu.
	Path, Name string
	Attribute  bool
	// Type to nazwa typu ({przestrzeń}nazwa albo nazwa) albo "" dla typu anonimowego; Builtin to nazwa
	// najbliższego typu wbudowanego, z którego typ jest wyprowadzony (np. "decimal", "date", "anySimpleType").
	Type, Builtin string
	// Canonical to wartość w postaci kanonicznej typu (XSD 2: postać kanoniczna; dla integer bez kropki).
	Canonical string
	// Line i Column to pozycja jak w błędach: koniec znacznika początkowego elementu albo atrybut.
	Line, Column int
}

// emitValue przekazuje poprawną wartość do Options.OnValue.
func (v *validator) emitValue(st *schema.SimpleType, val datatypes.Value, path, name string, attr bool, line, col int) {
	ev := ValueEvent{Path: path, Name: name, Attribute: attr, Line: line, Column: col, Canonical: st.DT.Canonical(val)}
	if st.Name.Local != "" {
		ev.Type = st.Name.String()
	}
	for b := st; b != nil; {
		if b.Builtin {
			ev.Builtin = b.Name.Local
			break
		}
		next, _ := b.Base.(*schema.SimpleType)
		b = next
	}
	v.opts.OnValue(ev)
}

// Validate waliduje dokument z r według schematu sch. Pusta lista błędów oznacza dokument poprawny. Drugi wynik
// to błąd wejścia, przekroczenie limitu (ErrLimit), przerwanie po MaxErrors (ErrTooManyErrors, z błędami) albo
// konstrukcja nieobsługiwana (errors.ErrUnsupported).
func Validate(sch *schema.Schema, r io.Reader, opts Options) ([]Error, error) {
	if opts.MaxErrors == 0 {
		opts.MaxErrors = 1000
	}
	if opts.MaxIdentityEntries == 0 {
		opts.MaxIdentityEntries = 4 << 20
	}
	if opts.MaxTextSize == 0 {
		opts.MaxTextSize = 16 << 20
	}
	v := &validator{sch: sch, opts: opts, rd: xmlreader.New(r, xmlreader.Options{Limits: opts.Reader}),
		uses: map[*schema.ComplexType]map[schema.QName]int{}, ids: map[string]bool{},
		special: map[*schema.SimpleType]specialKind{}}
	v.ctx.Namespaces = v
	v.ctx.Names = opts.Names
	err := v.run()
	return v.errs, err
}

// stop to sygnał wewnętrzny: walidacja kończy się błędem fatal.
type stop struct{ err error }

type mode uint8

const (
	modeStrict mode = iota // element z deklaracją albo typem
	modeLax                // ocena luźna bez deklaracji i typu: dzieci i atrybuty luźno
	modeSkip               // treść pomijana (symbol wieloznaczny skip)
)

// frame to stan otwartego elementu.
type frame struct {
	name schema.QName
	// depth to pozycja w stosie; index to numer wystąpienia nazwy wśród rodzeństwa (od 1). Ścieżkę liczy
	// pathOf dopiero przy błędzie.
	depth int
	index int
	line  int
	col   int
	// startLine i startCol to początek znacznika początkowego („<”); selfClosing: element zapisany jako <x/>.
	startLine, startCol int
	selfClosing         bool
	// childCount liczy dzieci według nazwy, do ścieżek [n].
	childCount map[schema.QName]int
	nsMark     int

	mode mode
	decl *schema.ElementDecl
	typ  schema.Type
	ct   *schema.ComplexType
	// st to typ prosty treści (typ prosty elementu albo treść prosta typu złożonego); nil dla innych.
	st *schema.SimpleType
	// laxChildren: dzieci oceniane luźno (anyType, element bez deklaracji).
	laxChildren bool
	matcher     *schema.Matcher
	// contentFailed: po pierwszym błędzie modelu dzieci są oceniane luźno, bez kolejnych błędów modelu.
	contentFailed bool
	nilled        bool
	hasElem       bool
	hasText       bool // jakikolwiek znak (także biały)
	textReported  bool
	needText      bool
	text          []byte

	// value to wartość elementu o treści prostej (dla pól ograniczeń tożsamości), valueOK: wartość jest poprawna.
	value   datatypes.Value
	valueOK bool

	// node to identyfikator elementu dla ograniczeń tożsamości.
	node int
	ic   *icFrame
}

type nsBinding struct{ prefix, uri string }

type validator struct {
	sch  *schema.Schema
	opts Options
	// endLine i endCol to pozycja bieżącego znacznika końcowego (miejsce wstawienia brakującego dziecka).
	endLine, endCol int
	rd              *xmlreader.Reader
	errs            []Error
	// ctx rozwiązuje prefiksy wartości QName w bieżącym elemencie.
	ctx   datatypes.Context
	stack []*frame
	ns    []nsBinding
	uses  map[*schema.ComplexType]map[schema.QName]int
	free  []*frame
	// special: rodzaj wartości o znaczeniu dla dokumentu (ID, IDREF, ENTITY, NOTATION) według typu prostego.
	special map[*schema.SimpleType]specialKind
	present []bool

	ids     map[string]bool
	idrefs  []idref
	entries int
	nodes   int
	ics     identityState
	// attrs to atrybuty bieżącego elementu z typami (z wartościami domyślnymi), do pól ograniczeń tożsamości.
	attrs []typedAttr
	// seenNS: przestrzenie nazw elementów i atrybutów, które już wystąpiły (4.3.2: wskazówka po pierwszym
	// wystąpieniu przestrzeni jest błędem); używane tylko z włączonymi wskazówkami.
	seenNS map[string]bool
	// seenHints: wskazówki już przetworzone; powtórzenie tej samej wskazówki niczego nie dodaje do schematu.
	seenHints map[Hint]bool
	// rootNS: przestrzeń nazw elementu głównego. Komunikaty podają nazwy z tej przestrzeni bez niej
	// („KodFormularza” zamiast „{http://crd.gov.pl/…}KodFormularza”), a nazwy z innych w postaci
	// {przestrzeń}nazwa (ADR 0009). Pól Name i Expected to nie dotyczy.
	rootNS string
}

type idref struct {
	value        string
	line, column int
	path         string
}

type typedAttr struct {
	name  schema.QName
	value datatypes.Value
	ok    bool
	// simple: atrybut ma typ prosty (ma deklarację); bez niej pole nie ma typu prostego.
	simple bool
}

// LookupPrefix spełnia datatypes.NamespaceResolver dla przestrzeni nazw bieżącego elementu.
func (v *validator) LookupPrefix(prefix string) (string, bool) {
	if prefix == "xml" {
		return schema.NamespaceXML, true
	}
	for i := len(v.ns) - 1; i >= 0; i-- {
		if v.ns[i].prefix == prefix {
			if v.ns[i].uri == "" {
				return "", prefix == ""
			}
			return v.ns[i].uri, true
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

func (v *validator) run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			s, ok := r.(stop)
			if !ok {
				panic(r)
			}
			err = s.err
		}
	}()
	for {
		tok, err := v.rd.Next()
		if err == io.EOF {
			v.finish()
			return nil
		}
		if err != nil {
			var xe *xmlreader.Error
			if !errors.As(err, &xe) {
				return err
			}
			switch {
			case strings.HasPrefix(xe.Code, "unsupported-"):
				return fmt.Errorf("%w: %s", errors.ErrUnsupported, xe.Error())
			case strings.HasPrefix(xe.Code, "limit-"):
				return fmt.Errorf("%w: %s", ErrLimit, xe.Error())
			}
			v.errs = append(v.errs, Error{Code: xe.Code, Line: xe.Line, Column: xe.Column, Path: v.currentPath(),
				Message: xe.Message})
			return nil
		}
		switch tok.Kind {
		case xmlreader.KindStartElement:
			v.start(&tok)
		case xmlreader.KindEndElement:
			v.end(&tok)
		case xmlreader.KindText:
			v.text(&tok)
		}
	}
}

func (v *validator) currentPath() string {
	if len(v.stack) == 0 {
		return ""
	}
	return v.pathOf(v.stack[len(v.stack)-1])
}

// pathOf składa ścieżkę elementu fr, który jest na stosie (ADR 0008): nazwy lokalne bez prefiksów, [n] od
// drugiego wystąpienia nazwy wśród rodzeństwa.
func (v *validator) pathOf(fr *frame) string {
	var b strings.Builder
	for _, f := range v.stack[:fr.depth+1] {
		b.WriteByte('/')
		b.WriteString(f.name.Local)
		if f.index > 1 {
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(f.index))
			b.WriteByte(']')
		}
	}
	return b.String()
}

// newFrame bierze ramkę z puli; ramki zdjęte ze stosu wracają do puli (bez alokacji na każdy element).
func (v *validator) newFrame() *frame {
	if n := len(v.free); n > 0 {
		fr := v.free[n-1]
		v.free = v.free[:n-1]
		return fr
	}
	return &frame{}
}

func (v *validator) releaseFrame(fr *frame) {
	cc, text := fr.childCount, fr.text[:0]
	clear(cc)
	*fr = frame{childCount: cc, text: text}
	v.free = append(v.free, fr)
}

// report zapisuje błąd; po MaxErrors przerywa walidację.
func (v *validator) report(e Error) {
	if e.Message == "" {
		e.Message = e.Code
	}
	if v.rootNS != "" {
		e.Message = strings.ReplaceAll(e.Message, "{"+v.rootNS+"}", "")
	}
	v.errs = append(v.errs, e)
	if v.opts.MaxErrors > 0 && len(v.errs) >= v.opts.MaxErrors {
		panic(stop{ErrTooManyErrors})
	}
}

func (v *validator) errorAt(fr *frame, code, format string, args ...any) {
	v.report(Error{Code: code, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: fr.name.Local,
		Namespace: fr.name.Space, Message: fmt.Sprintf(format, args...)})
}

func qnameOf(n xmlreader.Name) schema.QName { return schema.QName{Space: n.Space, Local: n.Local} }

// ---- początek elementu

func (v *validator) start(tok *xmlreader.Token) {
	var parent *frame
	if len(v.stack) > 0 {
		parent = v.stack[len(v.stack)-1]
	}
	name := qnameOf(tok.Name)
	fr := v.newFrame()
	fr.name, fr.line, fr.col, fr.nsMark, fr.depth, fr.index = name, tok.EndLine, tok.EndColumn, len(v.ns), len(v.stack), 1
	fr.startLine, fr.startCol, fr.selfClosing = tok.Line, tok.Column, tok.SelfClosing
	v.nodes++
	fr.node = v.nodes
	if parent != nil {
		parent.hasElem = true
		if parent.childCount == nil {
			parent.childCount = map[schema.QName]int{}
		}
		parent.childCount[name]++
		fr.index = parent.childCount[name]
	}
	for _, b := range v.rd.DeclaredNamespaces() {
		v.ns = append(v.ns, nsBinding{b.Prefix, b.URI})
	}
	v.stack = append(v.stack, fr)

	if v.opts.Hints != nil {
		if parent == nil || parent.mode != modeSkip {
			v.loadHints(fr, tok)
		}
		if v.seenNS == nil {
			v.seenNS = map[string]bool{}
		}
		v.seenNS[name.Space] = true
		for i := range tok.Attrs {
			v.seenNS[tok.Attrs[i].Name.Space] = true
		}
	}

	v.attrs = v.attrs[:0]
	// Deklaracja z kontekstu (3.3.4, Schema-Validity Assessment (Element)).
	var decl *schema.ElementDecl
	process := schema.ProcessStrict
	if parent == nil {
		v.rootNS = name.Space
	}
	switch {
	case parent == nil:
		decl = v.sch.Elements[name]
	case parent.mode == modeSkip:
		fr.mode = modeSkip
		v.identityStart(fr, tok)
		return
	default:
		decl, process = v.childDecl(parent, fr, name)
	}
	if process == schema.ProcessSkip {
		fr.mode = modeSkip
		v.identityStart(fr, tok)
		return
	}
	xsiType, xsiNil, nilSet := v.xsiAttrs(fr, tok)
	switch {
	case decl != nil:
		v.assessDecl(fr, decl, xsiType, xsiNil, nilSet)
	default:
		if xsiType != nil {
			fr.mode = modeStrict
			v.setType(fr, xsiType)
		} else {
			if process == schema.ProcessStrict {
				if parent == nil {
					v.errorAt(fr, CodeEltDecl, "no global declaration for the root element %s", name)
				} else {
					v.errorAt(fr, CodeEltDecl, "no global declaration for element %s, required by a strict wildcard", name)
				}
			}
			fr.mode = modeLax
			fr.laxChildren = true
		}
	}
	v.attributes(fr, tok)
	v.identityStart(fr, tok)
}

// childDecl ustala deklarację dziecka z modelu zawartości rodzica (cvc-complex-type.2.4) i sposób oceny.
func (v *validator) childDecl(parent *frame, fr *frame, name schema.QName) (*schema.ElementDecl, schema.ProcessContents) {
	switch {
	case parent.laxChildren || parent.contentFailed:
		return v.sch.Elements[name], schema.ProcessLax
	case parent.nilled:
		// cvc-elt.3.2.1 zgłoszony na końcu rodzica; dzieci luźno.
		return v.sch.Elements[name], schema.ProcessLax
	case parent.st != nil:
		if parent.ct != nil {
			v.errorAt(parent, CodeComplexSimple, "element %s has simple content, child element %s is not allowed", parent.name, name)
		} else {
			v.errorAt(parent, CodeTypeSimpleChild, "element %s has a simple type, child element %s is not allowed", parent.name, name)
		}
		parent.contentFailed = true
		return v.sch.Elements[name], schema.ProcessLax
	case parent.ct != nil && parent.ct.Content == schema.ContentEmpty:
		v.errorAt(parent, CodeComplexEmpty, "element %s must be empty, child element %s is not allowed", parent.name, name)
		parent.contentFailed = true
		return v.sch.Elements[name], schema.ProcessLax
	case parent.matcher == nil:
		return v.sch.Elements[name], schema.ProcessLax
	}
	m, ok := parent.matcher.Step(name)
	if !ok {
		if parent.matcher.Overflow {
			panic(stop{fmt.Errorf("%w: content model of %s at %s needs more than %d configurations", ErrLimit,
				parent.name, v.pathOf(parent), schema.MaxConfigurations)})
		}
		exp := expectedNames(parent.matcher.Expected())
		e := Error{Code: CodeComplexContent, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: name.Local, Expected: exp,
			Namespace: name.Space, Reason: ReasonUnexpected, InsertLine: fr.startLine, InsertColumn: fr.startCol}
		if ns, ok := sameLocalOtherSpace(name, exp); ok {
			e.Reason, e.ExpectedNamespace = ReasonWrongNamespace, ns
		}
		if len(exp) == 0 {
			e.Message = fmt.Sprintf("unexpected element %s in %s; no more child elements are allowed here", name, parent.name)
		} else {
			e.Message = fmt.Sprintf("unexpected element %s in %s; %s", name, parent.name, expectedPhrase(exp))
		}
		v.report(e)
		parent.contentFailed = true
		return v.sch.Elements[name], schema.ProcessLax
	}
	if m.Wildcard != nil {
		switch m.Wildcard.Process {
		case schema.ProcessSkip:
			return nil, schema.ProcessSkip
		case schema.ProcessLax:
			return v.sch.Elements[name], schema.ProcessLax
		}
		return v.sch.Elements[name], schema.ProcessStrict
	}
	return m.Element, schema.ProcessStrict
}

// expectedPhrase opisuje elementy dozwolone w miejscu błędu: „expected X” albo „expected one of: X, Y”.
func expectedPhrase(exp []string) string {
	if len(exp) == 1 {
		return "expected " + exp[0]
	}
	return "expected one of: " + strings.Join(exp, ", ")
}

func expectedNames(list []schema.Expectation) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, e := range list {
		var s string
		switch {
		case e.Element != nil:
			s = e.Element.Name.String()
		case e.Wildcard != nil:
			s = wildcardLabel(e.Wildcard)
		}
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func wildcardLabel(w *schema.Wildcard) string {
	switch w.Namespaces.Kind {
	case schema.NamespaceAny:
		return "*"
	case schema.NamespaceNot:
		if w.Namespaces.Not == "" {
			return "{##other}*"
		}
		return "{not " + w.Namespaces.Not + "}*"
	}
	parts := make([]string, len(w.Namespaces.Set))
	for i, s := range w.Namespaces.Set {
		if s == "" {
			parts[i] = "*"
		} else {
			parts[i] = "{" + s + "}*"
		}
	}
	return strings.Join(parts, "|")
}

// xsiAttrs czyta xsi:type i xsi:nil (cvc-elt.3, cvc-elt.4.1–4.2). Zwraca typ lokalny (albo nil) i wartość nil.
func (v *validator) xsiAttrs(fr *frame, tok *xmlreader.Token) (schema.Type, bool, bool) {
	var typ schema.Type
	var nilValue, nilSet bool
	for i := range tok.Attrs {
		a := &tok.Attrs[i]
		if a.Name.Space != schema.NamespaceXSI {
			continue
		}
		// Atrybuty xsi są węzłami dokumentu: pole ograniczenia tożsamości (np. "@*") może je wskazać.
		ta := typedAttr{name: qnameOf(a.Name), simple: true}
		if d := schema.XSIAttribute(a.Name.Local); d != nil && d.Type != nil {
			if val, err := d.Type.DT.Validate(a.Value, &v.ctx); err == nil {
				ta.value, ta.ok = val, true
			}
		} else {
			ta.simple = false
		}
		v.attrs = append(v.attrs, ta)
		switch a.Name.Local {
		case "type":
			val, err := schema.BuiltinType("QName").DT.Validate(a.Value, &v.ctx)
			if err != nil {
				v.report(Error{Code: CodeEltTypeQName, Line: a.Line, Column: a.Column, Path: v.pathOf(fr) + "/@type",
					Name: "type", Value: a.Value, Message: fmt.Sprintf("xsi:type %q is not a valid QName: %v", a.Value, err)})
				continue
			}
			q := val.(datatypes.QNameValue)
			t := v.sch.LookupType(schema.QName{Space: q.Namespace, Local: q.Local})
			if t == nil {
				v.report(Error{Code: CodeEltTypeResolve, Line: a.Line, Column: a.Column, Path: v.pathOf(fr) + "/@type",
					Name: "type", Value: a.Value, Message: fmt.Sprintf("xsi:type %q does not resolve to a type definition", a.Value)})
				continue
			}
			typ = t
		case "nil":
			val, err := schema.BuiltinType("boolean").DT.Validate(a.Value, nil)
			if err != nil {
				v.report(dtError(err, a.Line, a.Column, v.pathOf(fr)+"/@nil", "nil", schema.NamespaceXSI, schema.BuiltinType("boolean")))
				continue
			}
			nilSet = true
			nilValue = bool(val.(datatypes.BooleanValue))
		}
	}
	return typ, nilValue, nilSet
}

// assessDecl ocenia element według deklaracji (Element Locally Valid (Element), 3.3.4).
func (v *validator) assessDecl(fr *frame, decl *schema.ElementDecl, xsiType schema.Type, nilValue, nilSet bool) {
	fr.mode = modeStrict
	fr.decl = decl
	if decl.Abstract {
		v.errorAt(fr, CodeEltAbstract, "element declaration %s is abstract", decl.Name)
	}
	if nilSet {
		switch {
		case !decl.Nillable:
			v.errorAt(fr, CodeEltNotNillable, "element %s is not nillable but has xsi:nil", decl.Name)
		case nilValue:
			fr.nilled = true
			if decl.Value != nil && decl.Value.Fixed {
				v.errorAt(fr, CodeEltNilFixed, "element %s has xsi:nil=\"true\" and a fixed value", decl.Name)
			}
		}
	}
	typ := decl.Type
	if typ == nil {
		typ = schema.AnyType()
	}
	if xsiType != nil && xsiType != typ {
		var block schema.DerivationSet = decl.Block
		if ct, ok := typ.(*schema.ComplexType); ok {
			block |= ct.Block
		}
		if _, simple := xsiType.(*schema.SimpleType); simple {
			block = decl.Block
		}
		if schema.TypeDerivationOK(xsiType, typ, block) {
			typ = xsiType
		} else {
			v.errorAt(fr, CodeEltTypeDerived, "xsi:type %s is not validly derived from %s", typeLabel(xsiType), typeLabel(typ))
		}
	}
	v.setType(fr, typ)
}

func typeLabel(t schema.Type) string {
	if n := t.TypeName(); n.Local != "" {
		return n.String()
	}
	return "an anonymous type"
}

// setType ustawia typ elementu i sposób oceny treści (Element Locally Valid (Type), 3.3.4).
func (v *validator) setType(fr *frame, typ schema.Type) {
	fr.typ = typ
	switch t := typ.(type) {
	case *schema.SimpleType:
		fr.st = t
		fr.needText = true
	case *schema.ComplexType:
		fr.ct = t
		if t.Abstract {
			v.errorAt(fr, CodeTypeAbstract, "type %s is abstract", typeLabel(t))
		}
		switch {
		case schema.IsAnyType(t):
			fr.laxChildren = true
			fr.needText = fr.decl != nil && fr.decl.Value != nil
		case t.Content == schema.ContentSimple:
			fr.st = t.SimpleContent
			fr.needText = true
		case t.Content == schema.ContentEmpty:
		default:
			if t.Model == nil {
				panic(stop{fmt.Errorf("%w: type %s has no compiled content model", errors.ErrUnsupported, typeLabel(t))})
			}
			fr.matcher = t.Model.NewMatcher()
			if t.Content == schema.ContentMixed && fr.decl != nil && fr.decl.Value != nil {
				fr.needText = true
			}
		}
	}
}

// ---- atrybuty (cvc-complex-type.3, 4, 5; cvc-attribute; cvc-au)

func (v *validator) attributes(fr *frame, tok *xmlreader.Token) {
	var uses map[schema.QName]int
	if fr.ct != nil && !schema.IsAnyType(fr.ct) {
		uses = v.attributeUses(fr.ct)
		v.present = v.present[:0]
		for range fr.ct.Attributes {
			v.present = append(v.present, false)
		}
	}
	wildIDs := 0
	for i := range tok.Attrs {
		a := &tok.Attrs[i]
		q := qnameOf(a.Name)
		if q.Space == schema.NamespaceXSI {
			switch q.Local {
			case "type", "nil", "schemaLocation", "noNamespaceSchemaLocation":
				continue
			}
		}
		path := lazyPath{v: v, fr: fr, attr: q.Local}
		switch {
		case fr.mode == modeLax:
			// Ocena luźna: atrybut z deklaracją globalną jest sprawdzany, bez niej pomijany.
			if d := v.sch.LookupAttribute(q); d != nil {
				v.attribute(a, d, nil, path)
			}
		case fr.st != nil && fr.ct == nil:
			v.report(Error{Code: CodeTypeSimpleAttrs, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
				Message: fmt.Sprintf("element %s has a simple type and cannot have attribute %s", fr.name, q)})
		case schema.IsAnyType(fr.ct):
			if d := v.sch.LookupAttribute(q); d != nil {
				v.attribute(a, d, nil, path)
			}
		default:
			if ui, ok := uses[q]; ok {
				u := fr.ct.Attributes[ui]
				v.present[ui] = true
				v.attribute(a, u.Decl, u, path)
				continue
			}
			w := fr.ct.AttributeWildcard
			if w == nil {
				v.report(Error{Code: CodeComplexAttr, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
					Message: fmt.Sprintf("attribute %s is not allowed on element %s", q, fr.name)})
				continue
			}
			if !w.Namespaces.Allows(q.Space) {
				v.report(Error{Code: CodeComplexAttrWild, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
					Message: fmt.Sprintf("attribute %s is not allowed on element %s: its namespace is not allowed by the attribute wildcard", q, fr.name)})
				continue
			}
			if w.Process == schema.ProcessSkip {
				continue
			}
			d := v.sch.LookupAttribute(q)
			if d == nil {
				if w.Process == schema.ProcessStrict {
					v.report(Error{Code: CodeAttrDecl, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
						Message: fmt.Sprintf("no global declaration for attribute %s, required by a strict wildcard", q)})
				}
				continue
			}
			if schema.IsIDType(d.Type) {
				wildIDs++
				if wildIDs == 2 {
					v.report(Error{Code: CodeComplexWildIDs, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
						Message: fmt.Sprintf("element %s has more than one attribute of type ID through the attribute wildcard", fr.name)})
				}
				if hasIDUse(fr.ct) {
					v.report(Error{Code: CodeComplexWildIDUse, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space,
						Message: fmt.Sprintf("attribute %s of type ID matches the wildcard of %s, which already declares an ID attribute", q, fr.name)})
				}
			}
			v.attribute(a, d, nil, path)
		}
	}
	if uses == nil {
		return
	}
	for ui, u := range fr.ct.Attributes {
		if v.present[ui] {
			continue
		}
		if u.Required {
			v.errorAt(fr, CodeComplexRequired, "element %s is missing required attribute %s", fr.name, u.Decl.Name)
			continue
		}
		// Atrybut z wartości domyślnej (3.4.5) jest widoczny dla pól ograniczeń tożsamości.
		if vc := u.EffectiveValue(); vc != nil && vc.Value != nil {
			v.attrs = append(v.attrs, typedAttr{name: u.Decl.Name, value: vc.Value, ok: true, simple: true})
			// Wartość domyślna należy do tabeli ID/IDREF tak samo jak podana (3.15.5 liczy atrybuty PSVI).
			v.specialValues(u.Decl.Type, vc.Value, fr.line, fr.col, lazyPath{v: v, fr: fr, attr: u.Decl.Name.Local}, u.Decl.Name.Local)
		}
	}
}

func hasIDUse(ct *schema.ComplexType) bool {
	for _, u := range ct.Attributes {
		if schema.IsIDType(u.Decl.Type) {
			return true
		}
	}
	return false
}

func (v *validator) attributeUses(ct *schema.ComplexType) map[schema.QName]int {
	if m, ok := v.uses[ct]; ok {
		return m
	}
	m := make(map[schema.QName]int, len(ct.Attributes))
	for i, u := range ct.Attributes {
		m[u.Decl.Name] = i
	}
	v.uses[ct] = m
	return m
}

// attribute sprawdza wartość atrybutu według deklaracji (Attribute Locally Valid, 3.2.4) i użycia (3.5.4).
func (v *validator) attribute(a *xmlreader.Attr, d *schema.AttributeDecl, u *schema.AttributeUse, path lazyPath) {
	q := qnameOf(a.Name)
	ta := typedAttr{name: q, simple: true}
	if d.Type == nil {
		v.attrs = append(v.attrs, ta)
		return
	}
	val, err := d.Type.DT.Validate(a.Value, &v.ctx)
	if err != nil {
		v.report(dtError(err, a.Line, a.Column, path.String(), q.Local, q.Space, d.Type))
		v.attrs = append(v.attrs, ta)
		return
	}
	ta.value, ta.ok = val, true
	v.attrs = append(v.attrs, ta)
	if d.Value != nil && d.Value.Fixed && d.Value.Value != nil && !datatypes.Identical(val, d.Value.Value) {
		v.report(Error{Code: CodeAttrFixed, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space, Value: a.Value,
			Message: fmt.Sprintf("attribute %s must have the fixed value %q", q, d.Value.Lexical)})
	} else if u != nil && u.Value != nil && u.Value.Fixed && u.Value.Value != nil && !datatypes.Identical(val, u.Value.Value) {
		v.report(Error{Code: CodeAUFixed, Line: a.Line, Column: a.Column, Path: path.String(), Name: q.Local, Namespace: q.Space, Value: a.Value,
			Message: fmt.Sprintf("attribute %s must have the fixed value %q", q, u.Value.Lexical)})
	}
	v.specialValues(d.Type, val, a.Line, a.Column, path, q.Local)
	if v.opts.OnValue != nil {
		v.emitValue(d.Type, val, path.String(), q.Local, true, a.Line, a.Column)
	}
}

// lazyPath to ścieżka elementu albo jego atrybutu, liczona dopiero przy błędzie.
type lazyPath struct {
	v    *validator
	fr   *frame
	attr string
}

func (p lazyPath) String() string {
	s := p.v.pathOf(p.fr)
	if p.attr != "" {
		s += "/@" + p.attr
	}
	return s
}

// typeNames zwraca nazwę i przestrzeń typu (puste dla typu anonimowego) oraz najbliższy typ wbudowany.
func typeNames(st *schema.SimpleType) (name, space, builtin string) {
	if st == nil {
		return "", "", ""
	}
	name, space = st.Name.Local, st.Name.Space
	for b := st; b != nil; {
		if b.Builtin {
			builtin = b.Name.Local
			break
		}
		next, _ := b.Base.(*schema.SimpleType)
		b = next
	}
	return name, space, builtin
}

// sameLocalOtherSpace szuka wśród oczekiwanych elementów ("nazwa" albo "{przestrzeń}nazwa") elementu o nazwie
// lokalnej name w innej przestrzeni nazw; zwraca tę przestrzeń.
func sameLocalOtherSpace(name schema.QName, expected []string) (string, bool) {
	for _, e := range expected {
		space, local := "", e
		if strings.HasPrefix(e, "{") {
			if i := strings.IndexByte(e, '}'); i > 0 {
				space, local = e[1:i], e[i+1:]
			}
		}
		if local == name.Local && space != name.Space {
			return space, true
		}
	}
	return "", false
}

// dtError zamienia błąd typu prostego na błąd dokumentu z kodem reguły części 2.
func dtError(err error, line, col int, path, name, space string, st *schema.SimpleType) Error {
	e := Error{Code: datatypes.CodeDatatypeAtomic, Line: line, Column: col, Path: path, Name: name, Namespace: space,
		Message: err.Error()}
	e.Type, e.TypeNamespace, e.Builtin = typeNames(st)
	var de *datatypes.Error
	if errors.As(err, &de) {
		// Najbardziej szczegółowa reguła: błąd składnika listy albo unii wskazuje przyczynę.
		inner := de
		for {
			var next *datatypes.Error
			if inner.Cause == nil || !errors.As(inner.Cause, &next) || next == inner {
				break
			}
			inner = next
		}
		e.Code = de.Code
		if inner.Code != datatypes.CodeDatatypeAtomic && inner != de && de.Code == datatypes.CodeDatatypeList {
			e.Code = inner.Code
		}
		e.Value = de.Value
		e.Message = valueMessage(de, path, name)
		if de.Facet != "" {
			e.Facet = de.Facet
			if de.Detail != "" {
				e.Facet = de.Detail
				if !strings.HasPrefix(de.Detail, de.Facet) {
					e.Facet = de.Facet + " " + de.Detail
				}
			}
		}
	}
	return e
}

// specialValues sprawdza wartości typów, których poprawność zależy od dokumentu albo schematu: ID, IDREF(S),
// ENTITY(IES), NOTATION (3.15.5; część 2, 3.2.19, 3.3.8–3.3.12).
type specialKind uint8

const (
	specialNone specialKind = iota
	specialID
	specialIDREF
	specialENTITY
	specialNOTATION
)

func (v *validator) specialOf(st *schema.SimpleType) specialKind {
	if k, ok := v.special[st]; ok {
		return k
	}
	k := specialNone
	switch {
	case schema.DerivedFromBuiltin(st, "ID"):
		k = specialID
	case schema.DerivedFromBuiltin(st, "IDREF"):
		k = specialIDREF
	case schema.DerivedFromBuiltin(st, "ENTITY"):
		k = specialENTITY
	case schema.DerivedFromBuiltin(st, "NOTATION"):
		k = specialNOTATION
	}
	v.special[st] = k
	return k
}

func (v *validator) specialValues(st *schema.SimpleType, val datatypes.Value, line, col int, lp lazyPath, name string) {
	if st == nil || val == nil {
		return
	}
	itemType := st
	var one [1]datatypes.Value
	one[0] = val
	values := one[:]
	if lst, ok := val.(datatypes.ListValue); ok {
		if st.Item == nil {
			return
		}
		itemType = st.Item
		values = lst
	}
	kind := v.specialOf(itemType)
	if kind == specialNone {
		return
	}
	path := lp.String()
	switch kind {
	case specialID:
		for _, x := range values {
			s := stringOf(x)
			if v.ids[s] {
				v.report(Error{Code: CodeIDDuplicate, Line: line, Column: col, Path: path, Name: name, Value: s,
					Message: fmt.Sprintf("ID %q is not unique in the document", s)})
				continue
			}
			v.ids[s] = true
			v.grow(1)
		}
	case specialIDREF:
		for _, x := range values {
			v.idrefs = append(v.idrefs, idref{value: stringOf(x), line: line, column: col, path: path})
			v.grow(1)
		}
	case specialENTITY:
		for _, x := range values {
			if s := stringOf(x); !v.rd.UnparsedEntity(s) {
				v.report(Error{Code: CodeEntity, Line: line, Column: col, Path: path, Name: name, Value: s,
					Message: fmt.Sprintf("%q is not the name of an unparsed entity declared in the document", s)})
			}
		}
	case specialNOTATION:
		for _, x := range values {
			q, ok := x.(datatypes.QNameValue)
			if !ok {
				continue
			}
			if _, ok := v.sch.Notations[schema.QName{Space: q.Namespace, Local: q.Local}]; !ok {
				v.report(Error{Code: CodeNotation, Line: line, Column: col, Path: path, Name: name, Value: q.Local,
					Message: fmt.Sprintf("%s is not a notation declared in the schema", schema.QName{Space: q.Namespace, Local: q.Local})})
			}
		}
	}
}

func stringOf(x datatypes.Value) string {
	if s, ok := x.(datatypes.StringValue); ok {
		return s.S
	}
	return datatypes.Canonical(x)
}

func (v *validator) grow(n int) {
	v.entries += n
	if v.entries > v.opts.MaxIdentityEntries {
		panic(stop{fmt.Errorf("%w: more than %d ID and identity-constraint entries", ErrLimit, v.opts.MaxIdentityEntries)})
	}
}

// ---- tekst

func (v *validator) text(tok *xmlreader.Token) {
	if len(v.stack) == 0 {
		return
	}
	fr := v.stack[len(v.stack)-1]
	if fr.mode == modeSkip || len(tok.Data) == 0 {
		return
	}
	fr.hasText = true
	if fr.needText {
		if len(fr.text)+len(tok.Data) > v.opts.MaxTextSize {
			panic(stop{fmt.Errorf("%w: text of element %s at %s is longer than %d bytes", ErrLimit, fr.name, v.pathOf(fr), v.opts.MaxTextSize)})
		}
		fr.text = append(fr.text, tok.Data...)
	}
	if fr.textReported || fr.nilled || fr.contentFailed {
		return
	}
	switch {
	case fr.ct != nil && fr.ct.Content == schema.ContentEmpty:
		fr.textReported = true
		v.report(Error{Code: CodeComplexEmpty, Line: tok.Line, Column: tok.Column, Path: v.pathOf(fr), Name: fr.name.Local,
			Message: fmt.Sprintf("element %s must be empty, text is not allowed", fr.name)})
	case fr.ct != nil && fr.ct.Content == schema.ContentElementOnly && !tok.Whitespace:
		fr.textReported = true
		v.report(Error{Code: CodeComplexText, Line: tok.Line, Column: tok.Column, Path: v.pathOf(fr), Name: fr.name.Local,
			Value: abbreviate(string(tok.Data)), Message: fmt.Sprintf("element %s has element-only content, text is not allowed", fr.name)})
	}
}

func abbreviate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 64 {
		r := []rune(s)
		if len(r) > 60 {
			return string(r[:60]) + "…"
		}
	}
	return s
}

// ---- koniec elementu

func (v *validator) end(tok *xmlreader.Token) {
	fr := v.stack[len(v.stack)-1]
	v.endLine, v.endCol = tok.Line, tok.Column
	if fr.mode != modeSkip {
		v.finishElement(fr)
	}
	v.identityEnd(fr)
	v.stack = v.stack[:len(v.stack)-1]
	v.ns = v.ns[:fr.nsMark]
	v.releaseFrame(fr)
}

func (v *validator) finishElement(fr *frame) {
	if fr.nilled {
		if fr.hasElem || fr.hasText {
			v.errorAt(fr, CodeEltNilContent, "element %s has xsi:nil=\"true\" and must have no content", fr.name)
		}
		return
	}
	if fr.matcher != nil && !fr.contentFailed && !fr.matcher.Accepting() {
		exp := expectedNames(fr.matcher.Expected())
		// Brakujące dziecko powinno stanąć przed znacznikiem końcowym; element <x/> trzeba najpierw rozwinąć,
		// więc wskazujemy koniec jego znacznika.
		insL, insC := v.endLine, v.endCol
		if fr.selfClosing {
			insL, insC = fr.line, fr.col
		}
		v.report(Error{Code: CodeComplexContent, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: fr.name.Local,
			Namespace: fr.name.Space, Reason: ReasonMissing, InsertLine: insL, InsertColumn: insC,
			Expected: exp, Message: fmt.Sprintf("element %s is incomplete; %s", fr.name, expectedPhrase(exp))})
	}
	decl := fr.decl
	if fr.st != nil && !(fr.contentFailed && fr.hasElem) {
		empty := !fr.hasElem && !fr.hasText
		text := string(fr.text)
		lexical := text
		useDefault := empty && decl != nil && decl.Value != nil
		if useDefault {
			lexical = decl.Value.Lexical
		}
		val, err := fr.st.DT.Validate(lexical, &v.ctx)
		if err != nil {
			code := ""
			if useDefault && fr.typ != decl.Type {
				code = CodeEltDefault
			}
			e := dtError(err, fr.line, fr.col, v.pathOf(fr), fr.name.Local, fr.name.Space, fr.st)
			if code != "" {
				e.Code = code
			}
			v.report(e)
		} else {
			fr.value, fr.valueOK = val, true
			if decl != nil && decl.Value != nil && decl.Value.Fixed && !useDefault && !v.matchesFixed(fr.st, val, decl) {
				v.report(Error{Code: CodeEltFixedValue, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: fr.name.Local,
					Value: abbreviate(text), Message: fmt.Sprintf("element %s must have the fixed value %q", fr.name, decl.Value.Lexical)})
			}
			v.specialValues(fr.st, val, fr.line, fr.col, lazyPath{v: v, fr: fr}, fr.name.Local)
			if v.opts.OnValue != nil {
				v.emitValue(fr.st, val, v.pathOf(fr), fr.name.Local, false, fr.line, fr.col)
			}
		}
	}
	if decl != nil && decl.Value != nil && decl.Value.Fixed && fr.ct != nil && fr.ct.Content == schema.ContentMixed {
		switch {
		case fr.hasElem:
			v.errorAt(fr, CodeEltFixedChild, "element %s has a fixed value and must not have child elements", fr.name)
		case fr.hasText && string(fr.text) != decl.Value.Lexical:
			v.report(Error{Code: CodeEltFixedMixed, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: fr.name.Local,
				Value: abbreviate(string(fr.text)), Message: fmt.Sprintf("element %s must have the fixed value %q", fr.name, decl.Value.Lexical)})
		}
	}
}

// matchesFixed sprawdza cvc-elt.5.2.2.2.2: wartość elementu odpowiada postaci kanonicznej wartości stałej. Postać
// kanoniczną odczytujemy faktycznym typem elementu, bo xsi:type mógł go zmienić względem typu deklaracji.
func (v *validator) matchesFixed(st *schema.SimpleType, val datatypes.Value, decl *schema.ElementDecl) bool {
	vc := decl.Value
	declST := simpleTypeOf(decl.Type)
	if declST == st && vc.Value != nil {
		return datatypes.Identical(val, vc.Value)
	}
	lexical := vc.Lexical
	if vc.Value != nil && declST != nil {
		lexical = declST.DT.Canonical(vc.Value)
	}
	fixed, err := st.DT.Validate(lexical, &v.ctx)
	if err != nil {
		return false
	}
	return datatypes.Identical(val, fixed)
}

// simpleTypeOf zwraca typ prosty treści: sam typ prosty albo treść prostą typu złożonego.
func simpleTypeOf(t schema.Type) *schema.SimpleType {
	switch t := t.(type) {
	case *schema.SimpleType:
		return t
	case *schema.ComplexType:
		if t.Content == schema.ContentSimple {
			return t.SimpleContent
		}
	}
	return nil
}

// finish sprawdza tabelę ID/IDREF korzenia (Validation Root Valid (ID/IDREF), 3.3.4).
func (v *validator) finish() {
	for _, r := range v.idrefs {
		if !v.ids[r.value] {
			v.report(Error{Code: CodeIDUnresolved, Line: r.line, Column: r.column, Path: r.path, Value: r.value,
				Message: fmt.Sprintf("IDREF %q does not match any ID in the document", r.value)})
		}
	}
}

// ---- wskazówki xsi:schemaLocation (4.3.2)

func (v *validator) loadHints(fr *frame, tok *xmlreader.Token) {
	var hints []Hint
	for i := range tok.Attrs {
		a := &tok.Attrs[i]
		if a.Name.Space != schema.NamespaceXSI {
			continue
		}
		switch a.Name.Local {
		case "schemaLocation":
			f := strings.Fields(a.Value)
			for j := 0; j+1 < len(f); j += 2 {
				hints = append(hints, Hint{Namespace: f[j], Location: f[j+1]})
			}
		case "noNamespaceSchemaLocation":
			if s := strings.TrimSpace(a.Value); s != "" {
				hints = append(hints, Hint{Location: s})
			}
		}
	}
	if len(hints) == 0 {
		return
	}
	if v.seenHints == nil {
		v.seenHints = map[Hint]bool{}
	}
	for _, h := range hints {
		if v.seenHints[h] {
			continue
		}
		v.seenHints[h] = true
		if v.seenNS[h.Namespace] {
			ns := h.Namespace
			if ns == "" {
				ns = "no namespace"
			}
			v.errorAt(fr, CodeSchemaHints, "xsi:schemaLocation for %s comes after the first element or attribute in that namespace", ns)
		}
	}
	sch, err := v.opts.Hints.LoadHints(hints)
	if err != nil {
		var list schema.ErrorList
		if errors.As(err, &list) {
			for _, e := range list {
				v.report(Error{Code: CodeSchemaHints, Line: fr.line, Column: fr.col, Path: v.pathOf(fr), Name: fr.name.Local,
					Message: "schema from xsi:schemaLocation is invalid: " + e.Error()})
			}
			return
		}
		panic(stop{err})
	}
	if sch != nil {
		v.sch = sch
	}
}

// sortedKeys pomaga w powtarzalnych komunikatach.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
