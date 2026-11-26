// Package xmlreader is a streaming XML 1.0 (Fifth Edition) reader with namespaces (Namespaces in XML 1.0, Third
// Edition), written from the text of the recommendations. It is the reader goxsdgo validates with, and it can be
// used on its own.
//
// It checks every well-formedness and namespace constraint (1725 of the 1727 in-scope cases of the W3C XML
// Conformance suite), gives the line and column of every token and error, reports errors with a fixed code (the
// Code* constants), never reads external entities or the network, and enforces limits on depth, names,
// attributes, text and entity expansion (Limits). It does not validate against a DTD; from the internal subset it
// takes what the recommendation requires of a non-validating processor (section 5.1): internal entities, default
// values and types of attributes, notations.
//
// Tokens are not copied: the Attrs and Data of a Token are valid until the next call to Next. Memory does not grow
// with the size of the document, only with its depth and the size of its largest token.
//
// Supported encodings: UTF-8, UTF-16 (LE and BE), ISO-8859-1 and US-ASCII.
package xmlreader

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

// Kind is the kind of a token.
type Kind uint8

const (
	KindStartElement Kind = iota + 1
	KindEndElement
	KindText
	KindComment
	KindProcInst
	KindDoctype
)

// Name is a name after namespace resolution: Space is the namespace name, Local the local part, Prefix the prefix
// as written. Without namespace processing (Options.NoNamespaces) Local holds the whole name and Space and Prefix
// are empty.
type Name struct {
	Space, Local, Prefix string
}

// Attr is an attribute of an element. Namespace declarations (xmlns, xmlns:p) are not attributes; see
// LookupNamespace and DeclaredNamespaces.
type Attr struct {
	Name  Name
	Value string // normalized (3.3.3), by the type declared in ATTLIST, if any
	// Type is the type declared for the attribute in the internal subset (one of the AttrType* constants), or ""
	// when it has no declaration that was processed.
	Type string
	// Specified is false for an attribute added from a default value in ATTLIST.
	Specified    bool
	Line, Column int
}

// Attribute types (Attr.Type): the values of the [attribute type] property of the XML Information Set, from the
// attribute-list declarations of the internal subset.
const (
	AttrTypeCDATA       = "CDATA"
	AttrTypeID          = "ID"
	AttrTypeIDREF       = "IDREF"
	AttrTypeIDREFS      = "IDREFS"
	AttrTypeENTITY      = "ENTITY"
	AttrTypeENTITIES    = "ENTITIES"
	AttrTypeNMTOKEN     = "NMTOKEN"
	AttrTypeNMTOKENS    = "NMTOKENS"
	AttrTypeNotation    = "NOTATION"
	AttrTypeEnumeration = "ENUMERATION"
)

// Token is one unit of the document. Attrs and Data are valid until the next call to Next.
type Token struct {
	Kind  Kind
	Name  Name // the element; KindProcInst: Local is the PI target; KindDoctype: Local is the name in DOCTYPE
	Attrs []Attr
	// Data is the text (KindText), the content of a comment, or the data of a PI (without leading white space).
	Data []byte
	// CDATA: the text comes from a CDATA section. Whitespace: the text consists of S characters ([3]) only.
	CDATA, Whitespace bool
	// SelfClosing: an empty-element tag; the next token is its KindEndElement.
	SelfClosing bool
	// Line and Column are the position of the first character of the token, from 1; the column counts characters.
	Line, Column int
	// EndLine and EndColumn (KindStartElement): the position just after the end of the start tag (">" or "/>").
	// goxsdgo reports element errors at this position, as libxml2 does.
	EndLine, EndColumn int
}

// Limits are the limits of the reader. Zero means the default (DefaultLimits), a negative value no limit.
// Exceeding a limit is an *Error with a limit-* code.
type Limits struct {
	MaxDepth           int // nesting of elements
	MaxNameLength      int // length of a name in UTF-8 bytes
	MaxAttributes      int // attributes of one element, namespace declarations included
	MaxTextSize        int // one text, comment or PI token, in bytes
	MaxEntityExpansion int // total length of the replacement text of expanded entities, in bytes
	MaxEntityDepth     int // nesting of entity references
	MaxDTDSize         int // length of the DOCTYPE declaration with its internal subset, in bytes
}

// DefaultLimits returns the default limits: depth 512, names 16 KiB, 4096 attributes, text tokens 16 MiB, entity
// expansion 4 MiB in total, entity nesting 32, DOCTYPE 1 MiB.
func DefaultLimits() Limits {
	return Limits{
		MaxDepth:           512,
		MaxNameLength:      16 << 10,
		MaxAttributes:      4096,
		MaxTextSize:        16 << 20,
		MaxEntityExpansion: 4 << 20,
		MaxEntityDepth:     32,
		MaxDTDSize:         1 << 20,
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	pick := func(v, def int) int {
		switch {
		case v == 0:
			return def
		case v < 0:
			return int(^uint(0) >> 1)
		}
		return v
	}
	return Limits{
		MaxDepth:           pick(l.MaxDepth, d.MaxDepth),
		MaxNameLength:      pick(l.MaxNameLength, d.MaxNameLength),
		MaxAttributes:      pick(l.MaxAttributes, d.MaxAttributes),
		MaxTextSize:        pick(l.MaxTextSize, d.MaxTextSize),
		MaxEntityExpansion: pick(l.MaxEntityExpansion, d.MaxEntityExpansion),
		MaxEntityDepth:     pick(l.MaxEntityDepth, d.MaxEntityDepth),
		MaxDTDSize:         pick(l.MaxDTDSize, d.MaxDTDSize),
	}
}

// Options configure a Reader. The zero value is the default.
type Options struct {
	Limits Limits
	// NoNamespaces turns off namespace processing: only XML 1.0 well-formedness is checked, and colons in names
	// have no meaning.
	NoNamespaces bool
}

// Notation is a notation declared in the internal subset.
type Notation struct {
	Name, PublicID, SystemID string
	HasPublic, HasSystem     bool
}

const (
	nsXML   = "http://www.w3.org/XML/1998/namespace"
	nsXMLNS = "http://www.w3.org/2000/xmlns/"
)

// frame to źródło znaków: dokument albo tekst zastępczy encji.
type frame struct {
	buf []byte
	pos int
	doc bool
	ent *entity
	// depth to głębokość elementów w chwili wejścia w encję; po wyjściu musi być taka sama (4.3.2).
	depth int
	// line, col: miejsce odwołania w dokumencie (dla błędów w tekście encji).
	line, col int
}

type element struct {
	raw          string
	name         Name
	nsMark       int
	line, column int
}

type binding struct{ prefix, uri string }

type rawAttr struct {
	name         string
	typ          string
	value        string
	specified    bool
	line, column int
}

// Reader reads an XML document token by token.
type Reader struct {
	in     *input
	lim    Limits
	noNS   bool
	docF   frame
	cur    *frame
	frames []*frame

	line, col int // pozycja docF.buf[docF.pos]
	consumed  int // bajty dokumentu zużyte od początku

	started, seenRoot, seenDoctype bool
	depth                          int
	stack                          []element
	ns                             []binding
	pendingEnd                     bool
	err                            error

	text  []byte
	nbuf  []byte
	vbuf  []byte
	attrs []rawAttr
	// attrSet przyspiesza sprawdzanie powtórzeń przy wielu atrybutach (bez niego czas byłby kwadratowy).
	attrSet map[string]bool
	// expSet robi to samo dla nazw rozwiniętych atrybutów z przestrzenią nazw (NSC Attributes Unique, etap 7):
	// klucz to przestrzeń i nazwa lokalna, bez prefiksu.
	expSet map[expandedName]bool
	out    []Attr
	names  map[string]string
	qnames map[string]qsplit

	dtd        dtd
	expanded   int
	standalone bool
	bomUTF8    bool
}

// New returns a Reader of the document in r. Reads from r are buffered.
func New(r io.Reader, opts Options) *Reader {
	rd := &Reader{
		in:    newInput(r),
		lim:   opts.Limits.withDefaults(),
		noNS:  opts.NoNamespaces,
		line:  1,
		col:   1,
		names: map[string]string{},
	}
	rd.docF.doc = true
	rd.docF.buf = make([]byte, 0, chunkSize+1024)
	rd.cur = &rd.docF
	rd.dtd.init()
	return rd
}

// Next returns the next token, or io.EOF at the end of the document. Well-formedness and namespace errors and
// exceeded limits are an *Error; a read error of r is returned as it is. After the first error Next returns the
// same error.
func (r *Reader) Next() (Token, error) {
	if r.err != nil {
		return Token{}, r.err
	}
	t, err := r.next()
	if err != nil {
		r.err = err
	}
	return t, err
}

// LookupNamespace returns the namespace bound to prefix in the scope of the current element (after
// KindStartElement: that element; after KindEndElement: its parent). The empty prefix is the default namespace.
func (r *Reader) LookupNamespace(prefix string) (string, bool) {
	if prefix == "xml" {
		return nsXML, true
	}
	for i := len(r.ns) - 1; i >= 0; i-- {
		if r.ns[i].prefix == prefix {
			return r.ns[i].uri, r.ns[i].uri != "" || prefix != ""
		}
	}
	return "", false
}

// Binding is a namespace declaration: the prefix (empty for the default namespace) and the namespace name (empty
// when xmlns="" undeclares the default namespace).
type Binding struct{ Prefix, URI string }

// DeclaredNamespaces returns the namespace declarations of the current element (after KindStartElement), in
// document order. The result is a copy and stays valid.
func (r *Reader) DeclaredNamespaces() []Binding {
	if len(r.stack) == 0 {
		return nil
	}
	top := r.stack[len(r.stack)-1]
	if top.nsMark >= len(r.ns) {
		return nil
	}
	out := make([]Binding, 0, len(r.ns)-top.nsMark)
	for _, b := range r.ns[top.nsMark:] {
		out = append(out, Binding{Prefix: b.prefix, URI: b.uri})
	}
	return out
}

// Notations returns the notations declared in the internal subset, in declaration order.
func (r *Reader) Notations() []Notation { return r.dtd.notations }

// UnparsedEntity reports whether name is a declared unparsed entity (for the XSD types ENTITY and ENTITIES).
func (r *Reader) UnparsedEntity(name string) bool {
	e := r.dtd.general[name]
	return e != nil && e.ndata != ""
}

// ---- pozycje i błędy ----

func (r *Reader) position() (int, int) {
	if r.cur.doc {
		return r.line, r.col
	}
	return r.cur.line, r.cur.col
}

func (r *Reader) errorf(code, msg string) error {
	l, c := r.position()
	return &Error{Code: code, Line: l, Column: c, Message: msg}
}

func errorAt(code string, l, c int, msg string) error {
	return &Error{Code: code, Line: l, Column: c, Message: msg}
}

// inputErr zwraca błąd wejścia, gdy dokument nie może dostarczyć więcej danych z powodu złego kodowania albo
// błędu odczytu.
func (r *Reader) inputErr() error {
	if !r.cur.doc {
		return nil
	}
	if r.in.ioErr != nil {
		return r.in.ioErr
	}
	if r.in.bad {
		return r.errorf(CodeEncodingInvalid, "byte sequence not valid in the document encoding")
	}
	return nil
}

// eofErr to błąd końca danych w środku konstrukcji: koniec dokumentu albo koniec tekstu encji.
func (r *Reader) eofErr(what string) error {
	if err := r.inputErr(); err != nil {
		return err
	}
	if !r.cur.doc {
		return r.errorf(CodeEntityStructure, what+" not contained in the entity's replacement text")
	}
	return r.errorf(CodeEOF, "unexpected end of document in "+what)
}

// ---- prymitywy wejścia ----

func (r *Reader) avail() int { return len(r.cur.buf) - r.cur.pos }

// fill zapewnia co najmniej n bajtów w bieżącym źródle; false, gdy to niemożliwe.
func (r *Reader) fill(n int) bool {
	if len(r.cur.buf)-r.cur.pos >= n {
		return true
	}
	if !r.cur.doc {
		return false
	}
	f := &r.docF
	for len(f.buf)-f.pos < n {
		if r.in.done() {
			return false
		}
		if f.pos > 0 {
			k := copy(f.buf, f.buf[f.pos:])
			f.buf = f.buf[:k]
			f.pos = 0
		}
		r.in.decode(&f.buf)
	}
	return true
}

func (r *Reader) advance(n int) {
	f := r.cur
	if f.doc {
		for _, b := range f.buf[f.pos : f.pos+n] {
			if b == '\n' {
				r.line++
				r.col = 1
			} else if b&0xC0 != 0x80 {
				r.col++
			}
		}
		r.consumed += n
	}
	f.pos += n
}

// hasPrefix mówi, czy w bieżącym źródle zaczyna się s (bez zużywania).
func (r *Reader) hasPrefix(s string) bool {
	if !r.fill(len(s)) {
		return false
	}
	return string(r.cur.buf[r.cur.pos:r.cur.pos+len(s)]) == s
}

// peekRune dekoduje znak w bieżącym źródle i sprawdza, czy jest dozwolony ([2] Char).
func (r *Reader) peekRune() (rune, int, error) {
	if !r.fill(1) {
		return 0, 0, io.ErrUnexpectedEOF
	}
	b := r.cur.buf[r.cur.pos]
	if b < utf8.RuneSelf {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			return 0, 0, r.errorf(CodeChar, "character not allowed in XML")
		}
		return rune(b), 1, nil
	}
	r.fill(utf8.UTFMax)
	c, size := utf8.DecodeRune(r.cur.buf[r.cur.pos:])
	if c == utf8.RuneError && size <= 1 {
		if !utf8.FullRune(r.cur.buf[r.cur.pos:]) {
			if err := r.inputErr(); err != nil {
				return 0, 0, err
			}
		}
		return 0, 0, r.errorf(CodeEncodingInvalid, "invalid UTF-8 byte sequence")
	}
	if !isChar(c) {
		return 0, 0, r.errorf(CodeChar, "character not allowed in XML")
	}
	return c, size, nil
}

func (r *Reader) skipSpace() bool {
	any := false
	for r.fill(1) && isSpace(r.cur.buf[r.cur.pos]) {
		r.advance(1)
		any = true
	}
	return any
}

func (r *Reader) expect(s, what string) error {
	if !r.fill(len(s)) {
		if r.avail() > 0 && !bytes.HasPrefix([]byte(s), r.cur.buf[r.cur.pos:]) {
			return r.errorf(CodeSyntax, "expected "+what)
		}
		return r.eofErr(what)
	}
	if string(r.cur.buf[r.cur.pos:r.cur.pos+len(s)]) != s {
		return r.errorf(CodeSyntax, "expected "+what)
	}
	r.advance(len(s))
	return nil
}

func (r *Reader) intern(b []byte) string {
	if s, ok := r.names[string(b)]; ok {
		return s
	}
	s := string(b)
	if len(r.names) < 8192 && len(b) <= 256 {
		r.names[s] = s
	}
	return s
}

// advanceASCII przesuwa o n znaków ASCII bez końców wierszy.
func (r *Reader) advanceASCII(n int) {
	if r.cur.doc {
		r.col += n
		r.consumed += n
	}
	r.cur.pos += n
}

// readName czyta [5] Name.
func (r *Reader) readName(what string) (string, error) {
	// Szybka ścieżka: nazwa ASCII w całości w buforze, zakończona znakiem ASCII.
	if buf := r.cur.buf[r.cur.pos:]; len(buf) > 0 && buf[0] < utf8.RuneSelf && asciiName[buf[0]]&1 != 0 {
		i := 1
		for i < len(buf) && buf[i] < utf8.RuneSelf && asciiName[buf[i]]&2 != 0 {
			i++
		}
		if i < len(buf) && buf[i] < utf8.RuneSelf && i <= r.lim.MaxNameLength {
			s := r.intern(buf[:i])
			r.advanceASCII(i)
			return s, nil
		}
	}
	r.nbuf = r.nbuf[:0]
	c, size, err := r.peekRune()
	if err == io.ErrUnexpectedEOF {
		return "", r.eofErr(what)
	}
	if err != nil {
		return "", err
	}
	if !isNameStart(c) {
		return "", r.errorf(CodeName, what+" must start with a name character")
	}
	for {
		r.nbuf = append(r.nbuf, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
		r.advance(size)
		if len(r.nbuf) > r.lim.MaxNameLength {
			return "", r.errorf(CodeLimitNameLength, what+" longer than the limit")
		}
		// szybka ścieżka ASCII
		for r.avail() > 0 {
			b := r.cur.buf[r.cur.pos]
			if b >= utf8.RuneSelf || asciiName[b]&2 == 0 {
				break
			}
			r.nbuf = append(r.nbuf, b)
			r.advance(1)
		}
		if len(r.nbuf) > r.lim.MaxNameLength {
			return "", r.errorf(CodeLimitNameLength, what+" longer than the limit")
		}
		if !r.fill(1) {
			break
		}
		c, size, err = r.peekRune()
		if err != nil {
			return "", err
		}
		if !isNameChar(c) {
			break
		}
	}
	return r.intern(r.nbuf), nil
}

// ---- frames (encje w treści) ----

// push wchodzi w tekst zastępczy encji e; l, c to miejsce odwołania.
func (r *Reader) push(e *entity, l, c int) error {
	if len(r.frames)+1 > r.lim.MaxEntityDepth {
		return r.errorf(CodeLimitEntityDepth, "entity references nested deeper than the limit")
	}
	for _, f := range r.frames {
		if f.ent == e {
			return r.errorf(CodeNoRecursion, "entity &"+e.name+"; refers to itself")
		}
	}
	r.expanded += len(e.value)
	if r.expanded > r.lim.MaxEntityExpansion {
		return r.errorf(CodeLimitEntityExpand, "entity expansion larger than the limit")
	}
	if !r.cur.doc {
		l, c = r.cur.line, r.cur.col
	}
	f := &frame{buf: e.value, ent: e, depth: r.depth, line: l, col: c}
	r.frames = append(r.frames, f)
	r.cur = f
	return nil
}

func (r *Reader) pop() {
	r.frames = r.frames[:len(r.frames)-1]
	if len(r.frames) == 0 {
		r.cur = &r.docF
	} else {
		r.cur = r.frames[len(r.frames)-1]
	}
}

// ---- dokument ----

func (r *Reader) start() error {
	r.started = true
	head := r.in.peekRaw(4)
	switch {
	case bytes.HasPrefix(head, []byte{0xEF, 0xBB, 0xBF}):
		r.in.skipRaw(3)
		r.bomUTF8 = true
	case bytes.HasPrefix(head, []byte{0xFF, 0xFE, 0, 0}), bytes.HasPrefix(head, []byte{0, 0, 0xFE, 0xFF}):
		return errorAt(CodeEncodingUnsupported, 1, 1, "UTF-32 is not supported")
	case bytes.HasPrefix(head, []byte{0xFE, 0xFF}):
		r.in.skipRaw(2)
		r.in.enc = encUTF16BE
	case bytes.HasPrefix(head, []byte{0xFF, 0xFE}):
		r.in.skipRaw(2)
		r.in.enc = encUTF16LE
	case len(head) >= 2 && (head[0] == 0) != (head[1] == 0):
		// Dokument w UTF-16 musi zaczynać się od BOM (4.3.3); w UTF-8 bajt zerowy jest niedozwolony.
		return errorAt(CodeEncodingInvalid, 1, 1, "UTF-16 document without a byte order mark")
	case bytes.Equal(head, []byte{0x4C, 0x6F, 0xA7, 0x94}):
		return errorAt(CodeEncodingUnsupported, 1, 1, "EBCDIC is not supported")
	}
	if r.in.ioErr != nil {
		return r.in.ioErr
	}
	if r.hasPrefix("<?xml") && r.fill(6) && isSpace(r.cur.buf[r.cur.pos+5]) {
		return r.xmlDecl()
	}
	return nil
}

func (r *Reader) next() (Token, error) {
	if !r.started {
		if err := r.start(); err != nil {
			return Token{}, err
		}
	}
	if r.pendingEnd {
		r.pendingEnd = false
		return r.popElement(), nil
	}
	r.text = r.text[:0]
	inText := false
	ws := true
	var tl, tc int
	textToken := func() Token {
		return Token{Kind: KindText, Data: r.text, Whitespace: ws, Line: tl, Column: tc}
	}
	beginText := func() {
		if !inText {
			inText = true
			ws = true
			tl, tc = r.position()
		}
	}
	for {
		if !r.fill(1) {
			if err := r.inputErr(); err != nil {
				return Token{}, err
			}
			if !r.cur.doc {
				if r.depth != r.cur.depth {
					return Token{}, r.errorf(CodeEntityStructure, "elements in entity &"+r.cur.ent.name+"; are not balanced")
				}
				r.pop()
				continue
			}
			if inText && r.depth > 0 {
				return textToken(), nil
			}
			if r.depth > 0 {
				return Token{}, r.errorf(CodeEOF, "element <"+r.stack[len(r.stack)-1].raw+"> not closed")
			}
			if !r.seenRoot {
				return Token{}, r.errorf(CodeNoRoot, "document has no root element")
			}
			return Token{}, io.EOF
		}
		b := r.cur.buf[r.cur.pos]
		switch b {
		case '<':
			if inText && r.depth > 0 {
				return textToken(), nil
			}
			inText = false
			t, ok, err := r.markup()
			if err != nil || ok {
				return t, err
			}
			continue
		case '&':
			if r.depth == 0 {
				return Token{}, r.errorf(CodeOutsideRoot, "reference outside the root element")
			}
			beginText()
			_, err := r.contentReference(&ws)
			if err != nil {
				return Token{}, err
			}
			if len(r.text) > r.lim.MaxTextSize {
				return Token{}, r.errorf(CodeLimitTextSize, "text longer than the limit")
			}
			continue
		}
		beginText()
		if err := r.charData(&ws); err != nil {
			return Token{}, err
		}
		if r.depth == 0 && !ws {
			return Token{}, errorAt(CodeOutsideRoot, tl, tc, "text outside the root element")
		}
		if len(r.text) > r.lim.MaxTextSize {
			return Token{}, r.errorf(CodeLimitTextSize, "text longer than the limit")
		}
	}
}

// charData czyta ciąg danych znakowych do najbliższego '<' albo '&' ([14] CharData).
func (r *Reader) charData(ws *bool) error {
	for {
		f := r.cur
		buf := f.buf[f.pos:]
		i := 0
		for i < len(buf) && textByte[buf[i]] {
			if *ws && !isSpace(buf[i]) {
				*ws = false
			}
			i++
		}
		if i > 0 {
			r.text = append(r.text, buf[:i]...)
			r.advance(i)
			if len(r.text) > r.lim.MaxTextSize {
				return r.errorf(CodeLimitTextSize, "text longer than the limit")
			}
		}
		if !r.fill(1) {
			return nil
		}
		b := r.cur.buf[r.cur.pos]
		switch {
		case textByte[b]:
			// Bufor skończył się w środku tekstu; po doczytaniu wracamy do szybkiej pętli. Bez tego biały znak
			// za granicą bufora trafiał do gałęzi default i kasował flagę Whitespace.
			continue
		case b == '<' || b == '&':
			return nil
		case b == ']':
			if r.hasPrefix("]]>") {
				return r.errorf(CodeCDATAEnd, `"]]>" not allowed in character data`)
			}
			*ws = false
			r.text = append(r.text, ']')
			r.advance(1)
		default:
			c, size, err := r.peekRune()
			if err != nil {
				return err
			}
			if c != '\r' {
				*ws = false
			}
			r.text = append(r.text, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
			r.advance(size)
		}
		if len(r.text) > r.lim.MaxTextSize {
			return r.errorf(CodeLimitTextSize, "text longer than the limit")
		}
	}
}

// charRef czyta odwołanie znakowe po "&#" ([66] CharRef) i zwraca znak.
func (r *Reader) charRef() (rune, error) {
	hex := false
	if r.fill(1) && r.cur.buf[r.cur.pos] == 'x' {
		hex = true
		r.advance(1)
	}
	var v rune
	digits := 0
	for {
		if !r.fill(1) {
			return 0, r.eofErr("character reference")
		}
		b := r.cur.buf[r.cur.pos]
		var d rune
		switch {
		case '0' <= b && b <= '9':
			d = rune(b - '0')
		case hex && 'a' <= b && b <= 'f':
			d = rune(b-'a') + 10
		case hex && 'A' <= b && b <= 'F':
			d = rune(b-'A') + 10
		case b == ';' && digits > 0:
			r.advance(1)
			if !isChar(v) {
				return 0, r.errorf(CodeLegalCharacter, "character reference to a character not allowed in XML")
			}
			return v, nil
		default:
			return 0, r.errorf(CodeSyntax, "malformed character reference")
		}
		base := rune(10)
		if hex {
			base = 16
		}
		if v < 0x110000 {
			v = v*base + d
		}
		digits++
		r.advance(1)
	}
}

func predefined(name string) (byte, bool) {
	switch name {
	case "lt":
		return '<', true
	case "gt":
		return '>', true
	case "amp":
		return '&', true
	case "apos":
		return '\'', true
	case "quot":
		return '"', true
	}
	return 0, false
}

// contentReference obsługuje odwołanie w treści elementu ([67] Reference). Zwraca true, gdy weszło w encję.
func (r *Reader) contentReference(ws *bool) (bool, error) {
	l, c := r.position()
	r.advance(1) // '&'
	if r.fill(1) && r.cur.buf[r.cur.pos] == '#' {
		r.advance(1)
		v, err := r.charRef()
		if err != nil {
			return false, err
		}
		if !(v == ' ' || v == '\t' || v == '\n' || v == '\r') {
			*ws = false
		}
		r.text = utf8.AppendRune(r.text, v)
		return false, nil
	}
	name, err := r.readName("entity name")
	if err != nil {
		return false, err
	}
	if err := r.expect(";", `";" after the entity name`); err != nil {
		return false, err
	}
	if !r.noNS && bytes.IndexByte([]byte(name), ':') >= 0 {
		return false, errorAt(CodeQName, l, c, "entity name contains a colon")
	}
	// Encje predefiniowane (4.6) mają zawsze swój znak, także gdy podzbiór je deklaruje.
	if ch, ok := predefined(name); ok {
		*ws = false
		r.text = append(r.text, ch)
		return false, nil
	}
	e := r.dtd.general[name]
	if e == nil {
		return false, r.undeclared(name, l, c)
	}
	switch {
	case e.ndata != "":
		return false, errorAt(CodeParsedEntity, l, c, "reference to unparsed entity &"+name+";")
	case e.external:
		return false, errorAt(CodeExternalEntity, l, c, "reference to external entity &"+name+"; (external entities are never read)")
	}
	if err := r.push(e, l, c); err != nil {
		return false, err
	}
	return true, nil
}

// undeclared zwraca błąd dla odwołania do niezadeklarowanej encji: naruszenie WFC Entity Declared, gdy to
// ograniczenie obowiązuje (dokument bez podzbioru zewnętrznego i bez odwołań do encji parametrycznych albo
// standalone="yes"), inaczej błąd „encja mogła być w nieczytanym podzbiorze” (ADR 0004).
func (r *Reader) undeclared(name string, l, c int) error {
	if r.dtd.entityDeclaredApplies(r.standalone) {
		return errorAt(CodeEntityDeclared, l, c, "entity &"+name+"; is not declared")
	}
	return errorAt(CodeEntityUnresolved, l, c, "entity &"+name+"; is not declared in the internal subset (the external subset is never read)")
}

// markup rozpoznaje konstrukcję zaczynającą się od '<'. ok=false, gdy konstrukcja nie daje tokenu.
func (r *Reader) markup() (Token, bool, error) {
	l, c := r.position()
	r.fill(9)
	buf := r.cur.buf[r.cur.pos:]
	switch {
	case bytes.HasPrefix(buf, []byte("</")):
		if r.depth == 0 {
			return Token{}, false, r.errorf(CodeOutsideRoot, "end tag outside the root element")
		}
		t, err := r.endTag(l, c)
		return t, err == nil, err
	case bytes.HasPrefix(buf, []byte("<?")):
		t, err := r.procInst(l, c)
		return t, err == nil, err
	case bytes.HasPrefix(buf, []byte("<!--")):
		t, err := r.comment(l, c)
		return t, err == nil, err
	case bytes.HasPrefix(buf, []byte("<![CDATA[")):
		if r.depth == 0 {
			return Token{}, false, r.errorf(CodeOutsideRoot, "CDATA section outside the root element")
		}
		t, err := r.cdata(l, c)
		return t, err == nil, err
	case bytes.HasPrefix(buf, []byte("<!DOCTYPE")):
		if r.seenDoctype || r.seenRoot || !r.cur.doc {
			return Token{}, false, r.errorf(CodeSyntax, "document type declaration must precede the root element and appear once")
		}
		t, err := r.doctype(l, c)
		return t, err == nil, err
	case len(buf) >= 2 && buf[1] == '!':
		return Token{}, false, r.errorf(CodeSyntax, "unknown markup declaration")
	}
	if len(buf) < 2 {
		return Token{}, false, r.eofErr("start tag")
	}
	if r.depth == 0 && r.seenRoot {
		return Token{}, false, r.errorf(CodeOutsideRoot, "second root element")
	}
	t, err := r.startTag(l, c)
	return t, err == nil, err
}

func (r *Reader) comment(l, c int) (Token, error) {
	r.advance(4)
	r.text = r.text[:0]
	for {
		if !r.fill(1) {
			return Token{}, r.eofErr("comment")
		}
		if r.cur.buf[r.cur.pos] == '-' && r.hasPrefix("--") {
			if !r.hasPrefix("-->") {
				return Token{}, r.errorf(CodeComment, `"--" not allowed in a comment`)
			}
			r.advance(3)
			return Token{Kind: KindComment, Data: r.text, Line: l, Column: c}, nil
		}
		ch, size, err := r.peekRune()
		if err != nil {
			return Token{}, err
		}
		_ = ch
		r.text = append(r.text, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
		r.advance(size)
		if len(r.text) > r.lim.MaxTextSize {
			return Token{}, r.errorf(CodeLimitTextSize, "comment longer than the limit")
		}
	}
}

func (r *Reader) procInst(l, c int) (Token, error) {
	r.advance(2)
	target, err := r.readName("processing instruction target")
	if err != nil {
		return Token{}, err
	}
	if len(target) == 3 && (target[0]|0x20) == 'x' && (target[1]|0x20) == 'm' && (target[2]|0x20) == 'l' {
		return Token{}, errorAt(CodeReservedPITarget, l, c, "processing instruction target "+target+" is reserved")
	}
	if !r.noNS && bytes.IndexByte([]byte(target), ':') >= 0 {
		return Token{}, errorAt(CodeQName, l, c, "processing instruction target contains a colon")
	}
	r.text = r.text[:0]
	if !r.hasPrefix("?>") {
		if !r.skipSpace() {
			if !r.fill(1) {
				return Token{}, r.eofErr("processing instruction")
			}
			return Token{}, r.errorf(CodeSyntax, "white space required after the processing instruction target")
		}
		for {
			if !r.fill(1) {
				return Token{}, r.eofErr("processing instruction")
			}
			if r.cur.buf[r.cur.pos] == '?' && r.hasPrefix("?>") {
				break
			}
			_, size, err := r.peekRune()
			if err != nil {
				return Token{}, err
			}
			r.text = append(r.text, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
			r.advance(size)
			if len(r.text) > r.lim.MaxTextSize {
				return Token{}, r.errorf(CodeLimitTextSize, "processing instruction longer than the limit")
			}
		}
	}
	r.advance(2)
	return Token{Kind: KindProcInst, Name: Name{Local: target}, Data: r.text, Line: l, Column: c}, nil
}

func (r *Reader) cdata(l, c int) (Token, error) {
	r.advance(9)
	r.text = r.text[:0]
	ws := true
	for {
		if !r.fill(1) {
			return Token{}, r.eofErr("CDATA section")
		}
		b := r.cur.buf[r.cur.pos]
		if b == ']' && r.hasPrefix("]]>") {
			r.advance(3)
			return Token{Kind: KindText, CDATA: true, Data: r.text, Whitespace: ws, Line: l, Column: c}, nil
		}
		_, size, err := r.peekRune()
		if err != nil {
			return Token{}, err
		}
		if !isSpace(b) {
			ws = false
		}
		r.text = append(r.text, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
		r.advance(size)
		if len(r.text) > r.lim.MaxTextSize {
			return Token{}, r.errorf(CodeLimitTextSize, "CDATA section longer than the limit")
		}
	}
}

func (r *Reader) endTag(l, c int) (Token, error) {
	r.advance(2)
	name, err := r.readName("element name")
	if err != nil {
		return Token{}, err
	}
	r.skipSpace()
	if err := r.expect(">", `">" at the end of the end tag`); err != nil {
		return Token{}, err
	}
	top := r.stack[len(r.stack)-1]
	if name != top.raw {
		return Token{}, errorAt(CodeElementTypeMatch, l, c, "end tag </"+name+"> does not match start tag <"+top.raw+">")
	}
	if !r.cur.doc && r.depth == r.cur.depth {
		return Token{}, errorAt(CodeEntityStructure, l, c, "end tag in entity &"+r.cur.ent.name+"; closes an element opened outside it")
	}
	t := r.popElement()
	t.Line, t.Column = l, c
	return t, nil
}

func (r *Reader) popElement() Token {
	top := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.ns = r.ns[:top.nsMark]
	r.depth--
	// Pozycja znacznika początku; endTag nadpisuje ją pozycją znacznika końca.
	return Token{Kind: KindEndElement, Name: top.name, Line: top.line, Column: top.column}
}

// startTag czyta [40] STag albo [44] EmptyElemTag.
func (r *Reader) startTag(l, c int) (Token, error) {
	r.advance(1)
	name, err := r.readName("element name")
	if err != nil {
		return Token{}, err
	}
	r.attrs = r.attrs[:0]
	clear(r.attrSet)
	clear(r.expSet)
	selfClosing := false
	for {
		space := r.skipSpace()
		if !r.fill(1) {
			return Token{}, r.eofErr("start tag")
		}
		b := r.cur.buf[r.cur.pos]
		if b == '>' {
			r.advance(1)
			break
		}
		if b == '/' {
			if err := r.expect("/>", `"/>"`); err != nil {
				return Token{}, err
			}
			selfClosing = true
			break
		}
		if !space {
			return Token{}, r.errorf(CodeSyntax, "white space required before an attribute")
		}
		al, ac := r.position()
		an, err := r.readName("attribute name")
		if err != nil {
			return Token{}, err
		}
		r.skipSpace()
		if err := r.expect("=", `"=" after the attribute name`); err != nil {
			return Token{}, err
		}
		r.skipSpace()
		v, err := r.attValue()
		if err != nil {
			return Token{}, err
		}
		if r.duplicateAttr(an) {
			return Token{}, errorAt(CodeUniqueAttSpec, al, ac, "attribute "+an+" appears twice")
		}
		if len(r.attrs) >= r.lim.MaxAttributes {
			return Token{}, errorAt(CodeLimitAttributes, al, ac, "more attributes than the limit")
		}
		r.attrs = append(r.attrs, rawAttr{name: an, value: v, specified: true, line: al, column: ac})
	}
	if r.depth >= r.lim.MaxDepth {
		return Token{}, errorAt(CodeLimitDepth, l, c, "elements nested deeper than the limit")
	}
	r.dtd.applyAttlist(name, &r.attrs, l, c)
	el := element{raw: name, nsMark: len(r.ns), line: l, column: c}
	r.out = r.out[:0]
	if r.noNS {
		el.name = Name{Local: name}
		for _, a := range r.attrs {
			r.out = append(r.out, Attr{Name: Name{Local: a.name}, Value: a.value, Type: a.typ, Specified: a.specified, Line: a.line, Column: a.column})
		}
	} else {
		var err error
		el.name, err = r.namespaces(name, l, c)
		if err != nil {
			return Token{}, err
		}
	}
	r.stack = append(r.stack, el)
	r.depth++
	r.seenRoot = true
	r.pendingEnd = selfClosing
	endL, endC := r.position()
	return Token{Kind: KindStartElement, Name: el.name, Attrs: r.out, SelfClosing: selfClosing, Line: l, Column: c,
		EndLine: endL, EndColumn: endC}, nil
}

// duplicateAttr mówi, czy nazwa an już wystąpiła w bieżącym znaczniku.
func (r *Reader) duplicateAttr(an string) bool {
	if len(r.attrs) < 32 {
		for _, a := range r.attrs {
			if a.name == an {
				return true
			}
		}
		return false
	}
	if len(r.attrSet) == 0 {
		if r.attrSet == nil {
			r.attrSet = map[string]bool{}
		}
		for _, a := range r.attrs {
			r.attrSet[a.name] = true
		}
	}
	if r.attrSet[an] {
		return true
	}
	r.attrSet[an] = true
	return false
}

// attValue czyta [10] AttValue i normalizuje je według 3.3.3 (bez kroku dla typów innych niż CDATA, który
// wykonuje applyAttlist).
func (r *Reader) attValue() (string, error) {
	if !r.fill(1) {
		return "", r.eofErr("attribute value")
	}
	q := r.cur.buf[r.cur.pos]
	if q != '"' && q != '\'' {
		return "", r.errorf(CodeSyntax, "attribute value must be quoted")
	}
	r.advance(1)
	r.vbuf = r.vbuf[:0]
	for {
		if !r.fill(1) {
			return "", r.eofErr("attribute value")
		}
		b := r.cur.buf[r.cur.pos]
		switch {
		case b == q:
			r.advance(1)
			return string(r.vbuf), nil
		case b == '<':
			return "", r.errorf(CodeSyntax, `"<" not allowed in an attribute value`)
		case b == '&':
			l, c := r.position()
			r.advance(1)
			if err := r.attrReference(l, c, 0); err != nil {
				return "", err
			}
		case b == '\t' || b == '\n' || b == '\r':
			r.vbuf = append(r.vbuf, ' ')
			r.advance(1)
		default:
			_, size, err := r.peekRune()
			if err != nil {
				return "", err
			}
			r.vbuf = append(r.vbuf, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
			r.advance(size)
		}
		if len(r.vbuf) > r.lim.MaxTextSize {
			return "", r.errorf(CodeLimitTextSize, "attribute value longer than the limit")
		}
	}
}

// attrReference obsługuje odwołanie w wartości atrybutu po '&' (czytanym ze źródła).
func (r *Reader) attrReference(l, c, depth int) error {
	if r.fill(1) && r.cur.buf[r.cur.pos] == '#' {
		r.advance(1)
		v, err := r.charRef()
		if err != nil {
			return err
		}
		r.vbuf = utf8.AppendRune(r.vbuf, v)
		return nil
	}
	name, err := r.readName("entity name")
	if err != nil {
		return err
	}
	if err := r.expect(";", `";" after the entity name`); err != nil {
		return err
	}
	return r.expandInAttr(name, l, c, depth, nil)
}

// expandInAttr dopisuje do vbuf znormalizowany tekst zastępczy encji name (3.3.3, krok 3; 4.4.5).
func (r *Reader) expandInAttr(name string, l, c, depth int, chain []*entity) error {
	if !r.noNS && bytes.IndexByte([]byte(name), ':') >= 0 {
		return errorAt(CodeQName, l, c, "entity name contains a colon")
	}
	if ch, ok := predefined(name); ok {
		r.vbuf = append(r.vbuf, ch)
		return nil
	}
	e := r.dtd.general[name]
	if e == nil {
		return r.undeclared(name, l, c)
	}
	switch {
	case e.ndata != "":
		return errorAt(CodeParsedEntity, l, c, "reference to unparsed entity &"+name+"; in an attribute value")
	case e.external:
		return errorAt(CodeNoExternalEntityRef, l, c, "reference to external entity &"+name+"; in an attribute value")
	}
	for _, x := range chain {
		if x == e {
			return errorAt(CodeNoRecursion, l, c, "entity &"+name+"; refers to itself")
		}
	}
	if depth+1 > r.lim.MaxEntityDepth {
		return errorAt(CodeLimitEntityDepth, l, c, "entity references nested deeper than the limit")
	}
	r.expanded += len(e.value)
	if r.expanded > r.lim.MaxEntityExpansion {
		return errorAt(CodeLimitEntityExpand, l, c, "entity expansion larger than the limit")
	}
	chain = append(chain, e)
	v := e.value
	for i := 0; i < len(v); {
		b := v[i]
		switch {
		case b == '<':
			return errorAt(CodeNoLtInAttValue, l, c, "replacement text of &"+name+"; contains \"<\"")
		case b == '&':
			j := bytes.IndexByte(v[i:], ';')
			if j < 0 {
				return errorAt(CodeSyntax, l, c, "malformed reference in the replacement text of &"+name+";")
			}
			ref := string(v[i+1 : i+j])
			i += j + 1
			if len(ref) > 0 && ref[0] == '#' {
				ch, ok := parseCharRef(ref[1:])
				if !ok {
					return errorAt(CodeSyntax, l, c, "malformed character reference in &"+name+";")
				}
				if !isChar(ch) {
					return errorAt(CodeLegalCharacter, l, c, "character reference to a character not allowed in XML")
				}
				r.vbuf = utf8.AppendRune(r.vbuf, ch)
				continue
			}
			if !validName(ref) {
				return errorAt(CodeSyntax, l, c, "malformed entity reference in &"+name+";")
			}
			if err := r.expandInAttr(ref, l, c, depth+1, chain); err != nil {
				return err
			}
		case b == '\t' || b == '\n' || b == '\r':
			r.vbuf = append(r.vbuf, ' ')
			i++
		default:
			r.vbuf = append(r.vbuf, b)
			i++
		}
		if len(r.vbuf) > r.lim.MaxTextSize {
			return errorAt(CodeLimitTextSize, l, c, "attribute value longer than the limit")
		}
	}
	return nil
}

func parseCharRef(s string) (rune, bool) {
	base := rune(10)
	if len(s) > 0 && s[0] == 'x' {
		base, s = 16, s[1:]
	}
	if s == "" {
		return 0, false
	}
	var v rune
	for i := 0; i < len(s); i++ {
		b := s[i]
		var d rune
		switch {
		case '0' <= b && b <= '9':
			d = rune(b - '0')
		case base == 16 && 'a' <= b && b <= 'f':
			d = rune(b-'a') + 10
		case base == 16 && 'A' <= b && b <= 'F':
			d = rune(b-'A') + 10
		default:
			return 0, false
		}
		if v < 0x110000 {
			v = v*base + d
		}
	}
	return v, true
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if (i == 0 && !isNameStart(c)) || !isNameChar(c) {
			return false
		}
	}
	return true
}

// namespaces przetwarza deklaracje i nazwy elementu według Namespaces in XML 1.0 (rozdziały 3–6) i wypełnia
// r.out atrybutami z rozwiązanymi nazwami.
func (r *Reader) namespaces(name string, l, c int) (Name, error) {
	for _, a := range r.attrs {
		var prefix string
		switch {
		case a.name == "xmlns":
		case len(a.name) > 6 && a.name[:6] == "xmlns:":
			prefix = a.name[6:]
			if !isNCName(prefix) {
				return Name{}, errorAt(CodeQName, a.line, a.column, "attribute name "+a.name+" is not a QName")
			}
		default:
			continue
		}
		switch {
		case prefix == "xmlns":
			return Name{}, errorAt(CodeReservedPrefixes, a.line, a.column, "prefix xmlns must not be declared")
		case prefix == "xml" && a.value != nsXML:
			return Name{}, errorAt(CodeReservedPrefixes, a.line, a.column, "prefix xml must not be bound to another namespace")
		case prefix != "xml" && a.value == nsXML:
			return Name{}, errorAt(CodeReservedPrefixes, a.line, a.column, "the xml namespace must not be bound to another prefix")
		case a.value == nsXMLNS:
			return Name{}, errorAt(CodeReservedPrefixes, a.line, a.column, "the xmlns namespace must not be declared")
		case prefix != "" && a.value == "":
			return Name{}, errorAt(CodeNoPrefixUndeclaring, a.line, a.column, "prefix "+prefix+" must not be undeclared")
		}
		if prefix != "xml" {
			r.ns = append(r.ns, binding{prefix: prefix, uri: a.value})
		}
	}
	en, err := r.resolve(name, true, l, c)
	if err != nil {
		return Name{}, err
	}
	for _, a := range r.attrs {
		if a.name == "xmlns" || (len(a.name) > 6 && a.name[:6] == "xmlns:") {
			continue
		}
		an, err := r.resolve(a.name, false, a.line, a.column)
		if err != nil {
			return Name{}, err
		}
		if r.duplicateExpanded(an) {
			return Name{}, errorAt(CodeAttributesUnique, a.line, a.column, "attribute "+a.name+" has the same expanded name as another attribute")
		}
		r.out = append(r.out, Attr{Name: an, Value: a.value, Type: a.typ, Specified: a.specified, Line: a.line, Column: a.column})
	}
	return en, nil
}

// duplicateExpanded mówi, czy w r.out jest już atrybut o nazwie rozwiniętej n (NSC Attributes Unique).
func (r *Reader) duplicateExpanded(n Name) bool {
	if n.Space == "" {
		// atrybut bez przestrzeni nazw: powtórzenie wykrył już WFC Unique Att Spec
		return false
	}
	if len(r.out) < 32 {
		for _, o := range r.out {
			if o.Name.Space == n.Space && o.Name.Local == n.Local {
				return true
			}
		}
		return false
	}
	// Przy wielu atrybutach zbiór zamiast przeglądania listy: inaczej czas byłby kwadratowy (TestPathological).
	if len(r.expSet) == 0 {
		if r.expSet == nil {
			r.expSet = map[expandedName]bool{}
		}
		for _, o := range r.out {
			if o.Name.Space != "" {
				r.expSet[expandedName{o.Name.Space, o.Name.Local}] = true
			}
		}
	}
	k := expandedName{n.Space, n.Local}
	if r.expSet[k] {
		return true
	}
	r.expSet[k] = true // wywołujący zaraz dopisze n do r.out
	return false
}

// expandedName to nazwa rozwinięta (Namespaces in XML 1.0, 2.1): przestrzeń i nazwa lokalna, bez prefiksu.
type expandedName struct{ space, local string }

type qsplit struct {
	prefix, local string
	colon, ok     bool
}

// split dzieli QName na prefiks i nazwę lokalną i sprawdza ich składnię; wynik zapamiętuje dla powtarzających się
// nazw.
func (r *Reader) split(qname string) qsplit {
	if q, ok := r.qnames[qname]; ok {
		return q
	}
	var q qsplit
	if i := strings.IndexByte(qname, ':'); i < 0 {
		q = qsplit{local: qname, ok: isNCName(qname)}
	} else {
		q = qsplit{prefix: qname[:i], local: qname[i+1:], colon: true}
		q.ok = isNCName(q.prefix) && isNCName(q.local)
	}
	if r.qnames == nil {
		r.qnames = map[string]qsplit{}
	}
	if len(r.qnames) < 8192 && len(qname) <= 256 {
		r.qnames[qname] = q
	}
	return q
}

// resolve rozwiązuje QName elementu albo atrybutu.
func (r *Reader) resolve(qname string, elem bool, l, c int) (Name, error) {
	q := r.split(qname)
	if !q.colon {
		if !q.ok {
			return Name{}, errorAt(CodeQName, l, c, "name "+qname+" is not a QName")
		}
		if !elem {
			return Name{Local: qname}, nil
		}
		uri, _ := r.LookupNamespace("")
		return Name{Space: uri, Local: qname}, nil
	}
	prefix, local := q.prefix, q.local
	if !q.ok {
		return Name{}, errorAt(CodeQName, l, c, "name "+qname+" is not a QName")
	}
	if prefix == "xmlns" {
		return Name{}, errorAt(CodeReservedPrefixes, l, c, "prefix xmlns must not be used in a name")
	}
	uri, ok := r.LookupNamespace(prefix)
	if !ok || uri == "" {
		return Name{}, errorAt(CodePrefixDeclared, l, c, "prefix "+prefix+" is not declared")
	}
	return Name{Space: uri, Local: local, Prefix: prefix}, nil
}

// xmlDecl czyta [23] XMLDecl na początku dokumentu.
func (r *Reader) xmlDecl() error {
	r.advance(5)
	bad := func(msg string) error { return r.errorf(CodeXMLDecl, msg) }
	attr := func(want string) (string, bool, error) {
		save := r.cur.pos
		saveL, saveC, saveN := r.line, r.col, r.consumed
		if !r.skipSpace() {
			return "", false, nil
		}
		if !r.hasPrefix(want) {
			r.cur.pos, r.line, r.col, r.consumed = save, saveL, saveC, saveN
			return "", false, nil
		}
		r.advance(len(want))
		r.skipSpace()
		if err := r.expect("=", `"=" in the XML declaration`); err != nil {
			return "", false, err
		}
		r.skipSpace()
		if !r.fill(1) {
			return "", false, r.eofErr("XML declaration")
		}
		q := r.cur.buf[r.cur.pos]
		if q != '"' && q != '\'' {
			return "", false, bad(want + " value must be quoted")
		}
		r.advance(1)
		start := r.cur.pos
		for r.fill(1) && r.cur.buf[r.cur.pos] != q {
			if r.cur.buf[r.cur.pos] >= 0x80 || r.cur.buf[r.cur.pos] < 0x20 {
				return "", false, bad("invalid character in " + want)
			}
			r.advance(1)
			if r.cur.pos-start > 64 {
				return "", false, bad(want + " value too long")
			}
		}
		if !r.fill(1) {
			return "", false, r.eofErr("XML declaration")
		}
		v := string(r.cur.buf[start:r.cur.pos])
		r.advance(1)
		return v, true, nil
	}
	version, ok, err := attr("version")
	if err != nil {
		return err
	}
	if !ok {
		return bad("version is required in the XML declaration")
	}
	if len(version) < 3 || version[:2] != "1." || !allDigits(version[2:]) {
		return bad("version must match '1.' [0-9]+")
	}
	enc, hasEnc, err := attr("encoding")
	if err != nil {
		return err
	}
	if hasEnc {
		if !validEncName(enc) {
			return bad("malformed encoding name")
		}
		if err := r.setEncoding(enc); err != nil {
			return err
		}
	}
	sd, hasSD, err := attr("standalone")
	if err != nil {
		return err
	}
	if hasSD {
		if sd != "yes" && sd != "no" {
			return bad(`standalone must be "yes" or "no"`)
		}
		r.standalone = sd == "yes"
	}
	r.skipSpace()
	if !r.hasPrefix("?>") {
		if !r.fill(2) {
			return r.eofErr("XML declaration")
		}
		return bad(`malformed XML declaration (expected "?>")`)
	}
	r.advance(2)
	return nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// validEncName odpowiada produkcji [81] EncName.
func validEncName(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		letter := ('A' <= b && b <= 'Z') || ('a' <= b && b <= 'z')
		if i == 0 && !letter {
			return false
		}
		if !letter && !('0' <= b && b <= '9') && b != '.' && b != '_' && b != '-' {
			return false
		}
	}
	return s != ""
}

func (r *Reader) setEncoding(name string) error {
	n := []byte(name)
	for i := range n {
		if 'A' <= n[i] && n[i] <= 'Z' {
			n[i] += 'a' - 'A'
		}
	}
	lower := string(n)
	utf16 := r.in.enc == encUTF16LE || r.in.enc == encUTF16BE
	switch lower {
	case "utf-8":
		if utf16 {
			return r.errorf(CodeEncodingMismatch, "document is UTF-16 but declares "+name)
		}
	case "utf-16", "utf-16le", "utf-16be":
		if !utf16 {
			return r.errorf(CodeEncodingMismatch, "document declares "+name+" but has no UTF-16 byte order mark")
		}
	case "iso-8859-1", "iso_8859-1", "latin1", "l1", "iso-ir-100", "cp819", "ibm819", "csisolatin1":
		if r.bomUTF8 {
			return r.errorf(CodeEncodingMismatch, "document has a UTF-8 byte order mark but declares "+name)
		}
		if utf16 {
			return r.errorf(CodeEncodingMismatch, "document is UTF-16 but declares "+name)
		}
		r.docF.buf = r.in.switchEncoding(encLatin1, r.docF.buf, r.docF.pos)
	case "us-ascii", "ascii", "iso646-us", "ansi_x3.4-1968", "csascii":
		if r.bomUTF8 {
			return r.errorf(CodeEncodingMismatch, "document has a UTF-8 byte order mark but declares "+name)
		}
		if utf16 {
			return r.errorf(CodeEncodingMismatch, "document is UTF-16 but declares "+name)
		}
		r.docF.buf = r.in.switchEncoding(encASCII, r.docF.buf, r.docF.pos)
	default:
		return r.errorf(CodeEncodingUnsupported, "encoding "+name+" is not supported")
	}
	return nil
}
