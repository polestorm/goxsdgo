package xmlreader

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// Deklaracja typu dokumentu (XML 1.0, 2.8) i podzbiór wewnętrzny. Czytnik nie waliduje według DTD. Zapamiętuje
// to, czego rekomendacja wymaga od procesora bez walidacji (5.1): encje wewnętrzne, wartości domyślne i typy
// atrybutów oraz notacje. Podzbioru zewnętrznego i encji zewnętrznych nigdy nie czyta.

type entity struct {
	name     string
	value    []byte
	external bool
	ndata    string
}

type attDecl struct {
	name   string
	cdata  bool
	def    string
	hasDef bool
}

type dtd struct {
	general   map[string]*entity
	params    map[string]*entity
	attlists  map[string][]attDecl
	notations []Notation
	notation  map[string]bool
	// hasExternal: DOCTYPE wskazuje podzbiór zewnętrzny. peRefs: podzbiór wewnętrzny odwołuje się do encji
	// parametrycznych. unreadPE: wystąpiło odwołanie do encji parametrycznej, której nie przeczytano; dalszych
	// deklaracji encji i list atrybutów nie przetwarzamy, chyba że standalone="yes" (5.1).
	hasExternal, peRefs, unreadPE bool
}

func (d *dtd) init() {
	d.general = map[string]*entity{}
	d.params = map[string]*entity{}
	d.attlists = map[string][]attDecl{}
	d.notation = map[string]bool{}
}

// entityDeclaredApplies mówi, czy obowiązuje WFC Entity Declared (4.1).
func (d *dtd) entityDeclaredApplies(standalone bool) bool {
	return standalone || (!d.hasExternal && !d.peRefs)
}

func (r *Reader) processDecls() bool { return !r.dtd.unreadPE || r.standalone }

// doctype czyta [28] doctypedecl.
func (r *Reader) doctype(l, c int) (Token, error) {
	start := r.consumed
	r.advance(9)
	if !r.skipSpace() {
		return Token{}, r.syntaxOrEOF("white space after <!DOCTYPE", "document type declaration")
	}
	name, err := r.readName("document type name")
	if err != nil {
		return Token{}, err
	}
	space := r.skipSpace()
	if space && (r.hasPrefix("SYSTEM") || r.hasPrefix("PUBLIC")) {
		if _, _, _, _, err := r.externalID(false); err != nil {
			return Token{}, err
		}
		r.dtd.hasExternal = true
		r.skipSpace()
	}
	if r.fill(1) && r.cur.buf[r.cur.pos] == '[' {
		r.advance(1)
		if err := r.intSubset(start); err != nil {
			return Token{}, err
		}
		r.advance(1) // ']'
		r.skipSpace()
	}
	if err := r.expect(">", `">" at the end of the document type declaration`); err != nil {
		return Token{}, err
	}
	r.seenDoctype = true
	return Token{Kind: KindDoctype, Name: Name{Local: name}, Line: l, Column: c}, nil
}

func (r *Reader) syntaxOrEOF(msg, what string) error {
	if !r.fill(1) {
		return r.eofErr(what)
	}
	return r.errorf(CodeSyntax, "expected "+msg)
}

// intSubset czyta [28b] intSubset do ']' (nie zużywa go).
func (r *Reader) intSubset(start int) error {
	for {
		r.skipSpace()
		if r.consumed-start > r.lim.MaxDTDSize {
			return r.errorf(CodeLimitDTDSize, "document type declaration larger than the limit")
		}
		if !r.fill(1) {
			if !r.cur.doc {
				r.pop()
				continue
			}
			return r.eofErr("document type declaration")
		}
		b := r.cur.buf[r.cur.pos]
		switch {
		case b == ']':
			if !r.cur.doc {
				return r.errorf(CodeSyntax, `"]" in the replacement text of a parameter entity`)
			}
			return nil
		case b == '%':
			if err := r.peReference(); err != nil {
				return err
			}
		case r.hasPrefix("<!ELEMENT"):
			if err := r.elementDecl(); err != nil {
				return err
			}
		case r.hasPrefix("<!ATTLIST"):
			if err := r.attlistDecl(); err != nil {
				return err
			}
		case r.hasPrefix("<!ENTITY"):
			if err := r.entityDecl(); err != nil {
				return err
			}
		case r.hasPrefix("<!NOTATION"):
			if err := r.notationDecl(); err != nil {
				return err
			}
		case r.hasPrefix("<!--"):
			l, c := r.position()
			if _, err := r.comment(l, c); err != nil {
				return err
			}
		case r.hasPrefix("<?"):
			l, c := r.position()
			if _, err := r.procInst(l, c); err != nil {
				return err
			}
		case r.hasPrefix("<!["):
			if r.cur.doc {
				return r.errorf(CodeSyntax, "conditional sections are allowed only in the external subset")
			}
			return r.errorf(CodeConditionalSection, "conditional sections in parameter entities are not supported")
		default:
			if !r.fill(2) && b == '<' {
				return r.eofErr("document type declaration")
			}
			return r.errorf(CodeSyntax, "markup declaration expected in the internal subset")
		}
	}
}

// peReference obsługuje [69] PEReference między deklaracjami ([28a] DeclSep).
func (r *Reader) peReference() error {
	l, c := r.position()
	r.advance(1)
	name, err := r.readName("parameter entity name")
	if err != nil {
		return err
	}
	if err := r.expect(";", `";" after the parameter entity name`); err != nil {
		return err
	}
	r.dtd.peRefs = true
	e := r.dtd.params[name]
	switch {
	case e == nil:
		if r.standalone {
			return errorAt(CodeEntityDeclared, l, c, "parameter entity %"+name+"; is not declared")
		}
		r.dtd.unreadPE = true
		return nil
	case e.external:
		r.dtd.unreadPE = true
		return nil
	}
	return r.push(e, l, c)
}

// declSpace pomija białe znaki wewnątrz deklaracji; odwołanie do encji parametrycznej w tym miejscu narusza
// WFC PEs in Internal Subset.
func (r *Reader) declSpace() (bool, error) {
	sp := r.skipSpace()
	if r.fill(1) && r.cur.buf[r.cur.pos] == '%' {
		return sp, r.errorf(CodePEsInInternalSubset, "parameter entity reference inside a markup declaration in the internal subset")
	}
	return sp, nil
}

func (r *Reader) needSpace(what string) error {
	sp, err := r.declSpace()
	if err != nil {
		return err
	}
	if !sp {
		return r.syntaxOrEOF("white space "+what, "markup declaration")
	}
	return nil
}

func (r *Reader) endDecl() error {
	if _, err := r.declSpace(); err != nil {
		return err
	}
	return r.expect(">", `">" at the end of the markup declaration`)
}

// keyword sprawdza słowo kluczowe, po którym nie może być znaku nazwy.
func (r *Reader) keyword(k string) bool {
	if !r.hasPrefix(k) {
		return false
	}
	if r.fill(len(k) + 1) {
		nb := r.cur.buf[r.cur.pos+len(k)]
		if nb < 0x80 && isNameChar(rune(nb)) {
			return false
		}
	}
	r.advance(len(k))
	return true
}

func (r *Reader) noColon(name, what string) error {
	if !r.noNS && strings.IndexByte(name, ':') >= 0 {
		return r.errorf(CodeQName, what+" "+name+" contains a colon")
	}
	return nil
}

// elementDecl czyta [45] elementdecl (tylko składnia; walidacji nie ma).
func (r *Reader) elementDecl() error {
	r.advance(9)
	if err := r.needSpace("after <!ELEMENT"); err != nil {
		return err
	}
	if _, err := r.readName("element type name"); err != nil {
		return err
	}
	if err := r.needSpace("after the element type name"); err != nil {
		return err
	}
	switch {
	case r.keyword("EMPTY"), r.keyword("ANY"):
	case r.fill(1) && r.cur.buf[r.cur.pos] == '(':
		r.advance(1)
		if _, err := r.declSpace(); err != nil {
			return err
		}
		if r.hasPrefix("#PCDATA") {
			if err := r.mixed(); err != nil {
				return err
			}
		} else if err := r.children(); err != nil {
			return err
		}
	default:
		return r.syntaxOrEOF("content specification", "element declaration")
	}
	return r.endDecl()
}

// mixed czyta resztę [51] Mixed po "(".
func (r *Reader) mixed() error {
	r.advance(7)
	names := 0
	for {
		if _, err := r.declSpace(); err != nil {
			return err
		}
		if !r.fill(1) {
			return r.eofErr("element declaration")
		}
		switch r.cur.buf[r.cur.pos] {
		case ')':
			r.advance(1)
			if r.fill(1) && r.cur.buf[r.cur.pos] == '*' {
				r.advance(1)
			} else if names > 0 {
				return r.errorf(CodeSyntax, `mixed content with element types must end with ")*"`)
			}
			return nil
		case '|':
			r.advance(1)
			if _, err := r.declSpace(); err != nil {
				return err
			}
			if _, err := r.readName("element type name"); err != nil {
				return err
			}
			names++
		default:
			return r.errorf(CodeSyntax, `expected "|" or ")" in mixed content`)
		}
	}
}

// children czyta grupę [49] choice albo [50] seq po "(" z opcjonalnym wskaźnikiem powtórzeń.
func (r *Reader) children() error {
	var sep byte
	for {
		if err := r.cp(); err != nil {
			return err
		}
		if _, err := r.declSpace(); err != nil {
			return err
		}
		if !r.fill(1) {
			return r.eofErr("element declaration")
		}
		b := r.cur.buf[r.cur.pos]
		switch {
		case b == ')':
			r.advance(1)
			r.occurrence()
			return nil
		case (b == '|' || b == ',') && (sep == 0 || sep == b):
			sep = b
			r.advance(1)
			if _, err := r.declSpace(); err != nil {
				return err
			}
		default:
			return r.errorf(CodeSyntax, `expected ")" or a consistent separator in the content model`)
		}
	}
}

// cp czyta [48] cp.
func (r *Reader) cp() error {
	if !r.fill(1) {
		return r.eofErr("element declaration")
	}
	if r.cur.buf[r.cur.pos] == '(' {
		r.advance(1)
		if _, err := r.declSpace(); err != nil {
			return err
		}
		return r.children()
	}
	if _, err := r.readName("element type name"); err != nil {
		return err
	}
	r.occurrence()
	return nil
}

func (r *Reader) occurrence() {
	if r.fill(1) {
		switch r.cur.buf[r.cur.pos] {
		case '?', '*', '+':
			r.advance(1)
		}
	}
}

// attlistDecl czyta [52] AttlistDecl.
func (r *Reader) attlistDecl() error {
	r.advance(9)
	if err := r.needSpace("after <!ATTLIST"); err != nil {
		return err
	}
	elem, err := r.readName("element type name")
	if err != nil {
		return err
	}
	for {
		sp, err := r.declSpace()
		if err != nil {
			return err
		}
		if r.fill(1) && r.cur.buf[r.cur.pos] == '>' {
			r.advance(1)
			return nil
		}
		if !sp {
			return r.syntaxOrEOF(`white space or ">" in the attribute-list declaration`, "attribute-list declaration")
		}
		name, err := r.readName("attribute name")
		if err != nil {
			return err
		}
		if err := r.needSpace("after the attribute name"); err != nil {
			return err
		}
		cdata, err := r.attType()
		if err != nil {
			return err
		}
		if err := r.needSpace("before the default declaration"); err != nil {
			return err
		}
		d := attDecl{name: name, cdata: cdata}
		switch {
		case r.keyword("#REQUIRED"), r.keyword("#IMPLIED"):
		default:
			if r.keyword("#FIXED") {
				if err := r.needSpace("after #FIXED"); err != nil {
					return err
				}
			}
			v, err := r.attValue()
			if err != nil {
				return err
			}
			d.def, d.hasDef = v, true
		}
		if r.processDecls() {
			dup := false
			for _, x := range r.dtd.attlists[elem] {
				if x.name == name {
					dup = true
				}
			}
			if !dup {
				r.dtd.attlists[elem] = append(r.dtd.attlists[elem], d)
			}
		}
	}
}

// attType czyta [54] AttType; zwraca, czy to CDATA.
func (r *Reader) attType() (bool, error) {
	for _, k := range []string{"CDATA", "IDREFS", "IDREF", "ID", "ENTITIES", "ENTITY", "NMTOKENS", "NMTOKEN"} {
		if r.keyword(k) {
			return k == "CDATA", nil
		}
	}
	notation := r.keyword("NOTATION")
	if notation {
		if err := r.needSpace("after NOTATION"); err != nil {
			return false, err
		}
	}
	if !r.fill(1) || r.cur.buf[r.cur.pos] != '(' {
		return false, r.syntaxOrEOF("attribute type", "attribute-list declaration")
	}
	r.advance(1)
	for {
		if _, err := r.declSpace(); err != nil {
			return false, err
		}
		if notation {
			if _, err := r.readName("notation name"); err != nil {
				return false, err
			}
		} else if err := r.nmtoken(); err != nil {
			return false, err
		}
		if _, err := r.declSpace(); err != nil {
			return false, err
		}
		if !r.fill(1) {
			return false, r.eofErr("attribute-list declaration")
		}
		switch r.cur.buf[r.cur.pos] {
		case ')':
			r.advance(1)
			return false, nil
		case '|':
			r.advance(1)
		default:
			return false, r.errorf(CodeSyntax, `expected "|" or ")" in the enumeration`)
		}
	}
}

// nmtoken czyta [7] Nmtoken.
func (r *Reader) nmtoken() error {
	n := 0
	for {
		if !r.fill(1) {
			return r.eofErr("name token")
		}
		c, size, err := r.peekRune()
		if err != nil {
			return err
		}
		if !isNameChar(c) {
			break
		}
		r.advance(size)
		n++
	}
	if n == 0 {
		return r.errorf(CodeName, "name token expected")
	}
	return nil
}

// entityDecl czyta [70] EntityDecl.
func (r *Reader) entityDecl() error {
	r.advance(8)
	if err := r.skipRequired("after <!ENTITY"); err != nil {
		return err
	}
	pe := false
	if r.fill(1) && r.cur.buf[r.cur.pos] == '%' {
		r.advance(1)
		pe = true
		if err := r.needSpace(`after "%" in the parameter entity declaration`); err != nil {
			return err
		}
	}
	name, err := r.readName("entity name")
	if err != nil {
		return err
	}
	if err := r.noColon(name, "entity name"); err != nil {
		return err
	}
	if err := r.needSpace("after the entity name"); err != nil {
		return err
	}
	e := &entity{name: name}
	if r.fill(1) && (r.cur.buf[r.cur.pos] == '"' || r.cur.buf[r.cur.pos] == '\'') {
		v, err := r.entityValue()
		if err != nil {
			return err
		}
		e.value = v
	} else {
		if _, _, _, _, err := r.externalID(false); err != nil {
			return err
		}
		e.external = true
		if !pe {
			sp, err := r.declSpace()
			if err != nil {
				return err
			}
			if sp && r.keyword("NDATA") {
				if err := r.needSpace("after NDATA"); err != nil {
					return err
				}
				n, err := r.readName("notation name")
				if err != nil {
					return err
				}
				e.ndata = n
			}
		}
	}
	if err := r.endDecl(); err != nil {
		return err
	}
	if !r.processDecls() {
		return nil
	}
	m := r.dtd.general
	if pe {
		m = r.dtd.params
	}
	if m[name] == nil {
		m[name] = e
	}
	return nil
}

// skipRequired: wymagane białe znaki zaraz po słowie kluczowym deklaracji ENTITY (tu "%" rozpoczyna deklarację
// encji parametrycznej, nie odwołanie).
func (r *Reader) skipRequired(what string) error {
	if !r.skipSpace() {
		return r.syntaxOrEOF("white space "+what, "entity declaration")
	}
	return nil
}

// entityValue czyta [9] EntityValue i buduje tekst zastępczy (4.5): odwołania znakowe są rozwijane, odwołania do
// encji ogólnych pozostają bez zmian.
func (r *Reader) entityValue() ([]byte, error) {
	q := r.cur.buf[r.cur.pos]
	r.advance(1)
	var v []byte
	for {
		if !r.fill(1) {
			return nil, r.eofErr("entity value")
		}
		b := r.cur.buf[r.cur.pos]
		switch {
		case b == q:
			r.advance(1)
			return v, nil
		case b == '%':
			return nil, r.errorf(CodePEsInInternalSubset, "parameter entity reference in an entity value in the internal subset")
		case b == '&':
			r.advance(1)
			if r.fill(1) && r.cur.buf[r.cur.pos] == '#' {
				r.advance(1)
				ch, err := r.charRef()
				if err != nil {
					return nil, err
				}
				v = utf8.AppendRune(v, ch)
				continue
			}
			name, err := r.readName("entity name")
			if err != nil {
				return nil, err
			}
			if err := r.expect(";", `";" after the entity name`); err != nil {
				return nil, err
			}
			v = append(v, '&')
			v = append(v, name...)
			v = append(v, ';')
		default:
			_, size, err := r.peekRune()
			if err != nil {
				return nil, err
			}
			v = append(v, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
			r.advance(size)
		}
		if len(v) > r.lim.MaxDTDSize {
			return nil, r.errorf(CodeLimitDTDSize, "entity value larger than the limit")
		}
	}
}

// externalID czyta [75] ExternalID; dla notacji dopuszcza też [83] PublicID.
func (r *Reader) externalID(notation bool) (pub, sys string, hasPub, hasSys bool, err error) {
	switch {
	case r.keyword("SYSTEM"):
		if err = r.needSpace("after SYSTEM"); err != nil {
			return
		}
		sys, err = r.systemLiteral()
		hasSys = true
		return
	case r.keyword("PUBLIC"):
		if err = r.needSpace("after PUBLIC"); err != nil {
			return
		}
		if pub, err = r.pubidLiteral(); err != nil {
			return
		}
		hasPub = true
		var sp bool
		if sp, err = r.declSpace(); err != nil {
			return
		}
		if r.fill(1) && (r.cur.buf[r.cur.pos] == '"' || r.cur.buf[r.cur.pos] == '\'') {
			if !sp {
				err = r.errorf(CodeSyntax, "white space required between the public and system identifiers")
				return
			}
			sys, err = r.systemLiteral()
			hasSys = true
			return
		}
		if !notation {
			err = r.syntaxOrEOF("system literal after the public identifier", "external identifier")
		}
		return
	}
	err = r.syntaxOrEOF("SYSTEM or PUBLIC", "external identifier")
	return
}

func (r *Reader) systemLiteral() (string, error) {
	if !r.fill(1) {
		return "", r.eofErr("system literal")
	}
	q := r.cur.buf[r.cur.pos]
	if q != '"' && q != '\'' {
		return "", r.errorf(CodeSyntax, "system literal must be quoted")
	}
	r.advance(1)
	var v []byte
	for {
		if !r.fill(1) {
			return "", r.eofErr("system literal")
		}
		if r.cur.buf[r.cur.pos] == q {
			r.advance(1)
			return string(v), nil
		}
		_, size, err := r.peekRune()
		if err != nil {
			return "", err
		}
		v = append(v, r.cur.buf[r.cur.pos:r.cur.pos+size]...)
		r.advance(size)
		if len(v) > r.lim.MaxNameLength {
			return "", r.errorf(CodeLimitNameLength, "system literal longer than the limit")
		}
	}
}

func (r *Reader) pubidLiteral() (string, error) {
	if !r.fill(1) {
		return "", r.eofErr("public identifier")
	}
	q := r.cur.buf[r.cur.pos]
	if q != '"' && q != '\'' {
		return "", r.errorf(CodeSyntax, "public identifier must be quoted")
	}
	r.advance(1)
	var v []byte
	for {
		if !r.fill(1) {
			return "", r.eofErr("public identifier")
		}
		b := r.cur.buf[r.cur.pos]
		if b == q {
			r.advance(1)
			return string(v), nil
		}
		if !isPubidChar(b) {
			return "", r.errorf(CodeSyntax, "character not allowed in a public identifier")
		}
		v = append(v, b)
		r.advance(1)
		if len(v) > r.lim.MaxNameLength {
			return "", r.errorf(CodeLimitNameLength, "public identifier longer than the limit")
		}
	}
}

// notationDecl czyta [82] NotationDecl.
func (r *Reader) notationDecl() error {
	r.advance(10)
	if err := r.needSpace("after <!NOTATION"); err != nil {
		return err
	}
	name, err := r.readName("notation name")
	if err != nil {
		return err
	}
	if err := r.noColon(name, "notation name"); err != nil {
		return err
	}
	if err := r.needSpace("after the notation name"); err != nil {
		return err
	}
	pub, sys, hasPub, hasSys, err := r.externalID(true)
	if err != nil {
		return err
	}
	if err := r.endDecl(); err != nil {
		return err
	}
	if !r.dtd.notation[name] {
		r.dtd.notation[name] = true
		r.dtd.notations = append(r.dtd.notations, Notation{Name: name, PublicID: pub, SystemID: sys, HasPublic: hasPub, HasSystem: hasSys})
	}
	return nil
}

// applyAttlist dodaje atrybuty z wartości domyślnych i normalizuje wartości atrybutów typów innych niż CDATA
// (3.3.3, ostatni akapit; 5.1).
func (d *dtd) applyAttlist(elem string, attrs *[]rawAttr, l, c int) {
	decls := d.attlists[elem]
	if len(decls) == 0 {
		return
	}
	for _, decl := range decls {
		found := false
		for i := range *attrs {
			a := &(*attrs)[i]
			if a.name == decl.name {
				found = true
				if !decl.cdata {
					a.value = collapse(a.value)
				}
			}
		}
		if !found && decl.hasDef {
			v := decl.def
			if !decl.cdata {
				v = collapse(v)
			}
			*attrs = append(*attrs, rawAttr{name: decl.name, value: v, line: l, column: c})
		}
	}
}

// collapse usuwa spacje na początku i końcu i zastępuje ciągi spacji jedną spacją.
func collapse(s string) string {
	if !strings.Contains(s, "  ") && !strings.HasPrefix(s, " ") && !strings.HasSuffix(s, " ") {
		return s
	}
	var b bytes.Buffer
	for _, f := range strings.Split(s, " ") {
		if f == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(f)
	}
	return b.String()
}
