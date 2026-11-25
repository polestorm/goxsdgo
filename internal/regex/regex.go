// Package regex tłumaczy wyrażenia regularne XSD 1.0 (część 2, dodatek F) na składnię RE2 pakietu regexp
// i dopasowuje nimi napisy w czasie liniowym.
//
// Wyrażenie XSD jest zawsze kotwiczone na całym napisie. Klasy znaków liczymy jawnie jako zbiory przedziałów
// (odejmowanie klas, dopełnienia względem znaków XML, kategorie i bloki Unicode), więc do RE2 trafiają
// wyłącznie literały, klasy z przedziałami, grupy bez przechwytywania i liczniki. RE2 nie robi nawrotów,
// więc czas dopasowania jest liniowy względem długości napisu i rozmiaru wyrażenia (ADR 0005).
package regex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/polestorm/goxsdgo/internal/xmlchars"
)

// CodeSyntax to kod błędu wyrażenia, które nie jest poprawnym wyrażeniem regularnym XSD (część 2, 4.3.4.2:
// wartość pattern musi być wyrażeniem regularnym w rozumieniu dodatku F). Rekomendacja nie nadaje tej
// regule nazwy; kod jest nasz.
const CodeSyntax = "regex-syntax"

// CodeTooLarge to kod błędu wyrażenia poprawnego, którego RE2 nie przyjmuje z powodu rozmiaru po rozwinięciu
// liczników (ograniczenie implementacji, ADR 0005).
const CodeTooLarge = "regex-too-large"

// maxRepeat to największy licznik {n,m}, jaki przyjmuje regexp/syntax z biblioteki standardowej.
const maxRepeat = 1000

// maxExpansion ogranicza długość wyrażenia RE2 po rozwinięciu dużych liczników (1 MiB tekstu). Większe
// wyrażenie dostaje błąd regex-too-large zamiast zajmować pamięć i czas kompilacji.
const maxExpansion = 1 << 20

// Error to błąd kompilacji wyrażenia.
type Error struct {
	Code    string
	Pattern string
	Offset  int // przesunięcie w bajtach w Pattern, przy którym wykryto błąd
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s at offset %d in %q", e.Code, e.Message, e.Offset, e.Pattern)
}

// Regexp to skompilowane wyrażenie XSD.
type Regexp struct {
	src string
	re  *regexp.Regexp
	// re5 to wariant z klasami \i i \c według XML 1.0 Fifth Edition; nil, gdy wyrażenie ich nie używa
	// (wtedy oba warianty są tym samym językiem). ADR 0012.
	re5 *regexp.Regexp
}

// Compile kompiluje wyrażenie XSD. Klasy \i i \c (oraz \I, \C) mają domyślnie znaczenie z XML 1.0 Second
// Edition (dodatek F); wariant według Fifth Edition wybiera MatchStringNames.
func Compile(pattern string) (*Regexp, error) {
	re2, usesNames, err := translate(pattern, false)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(re2)
	if err != nil {
		return nil, &Error{Code: CodeTooLarge, Pattern: pattern, Message: err.Error()}
	}
	r := &Regexp{src: pattern, re: re}
	if usesNames {
		re25, _, err := translate(pattern, true)
		if err != nil {
			return nil, err
		}
		if r.re5, err = regexp.Compile(re25); err != nil {
			return nil, &Error{Code: CodeTooLarge, Pattern: pattern, Message: err.Error()}
		}
	}
	return r, nil
}

// MatchString mówi, czy cały napis s należy do języka wyrażenia (klasy nazw według XML 1.0 Second Edition).
func (r *Regexp) MatchString(s string) bool { return r.re.MatchString(s) }

// MatchStringNames jest jak MatchString, z klasami \i i \c według wydania ed.
func (r *Regexp) MatchStringNames(s string, ed xmlchars.Edition) bool {
	if ed == xmlchars.Edition5 && r.re5 != nil {
		return r.re5.MatchString(s)
	}
	return r.re.MatchString(s)
}

// String zwraca wyrażenie XSD, z którego powstał r.
func (r *Regexp) String() string { return r.src }

// Translate zwraca wyrażenie RE2 równoważne wyrażeniu XSD, kotwiczone na początku i końcu napisu.
func Translate(pattern string) (string, error) {
	re2, _, err := translate(pattern, false)
	return re2, err
}

// translate tłumaczy wyrażenie; names5 wybiera klasy \i i \c według XML 1.0 Fifth Edition. Zwraca też, czy
// wyrażenie używa tych klas.
func translate(pattern string, names5 bool) (string, bool, error) {
	p := &parser{src: pattern, names5: names5}
	var b strings.Builder
	b.WriteString(`\A(?:`)
	if _, err := p.regExp(&b); err != nil {
		return "", false, err
	}
	if p.pos < len(p.src) {
		return "", false, p.fail("unexpected %q", p.peek())
	}
	if b.Len() > maxExpansion {
		return "", false, &Error{Code: CodeTooLarge, Pattern: pattern, Message: "expression too large after expanding quantifiers"}
	}
	b.WriteString(`)\z`)
	return b.String(), p.usesNames, nil
}

type parser struct {
	src   string
	pos   int
	depth int
	// names5: klasy \i i \c według XML 1.0 Fifth Edition; usesNames: wyrażenie ich używa.
	names5, usesNames bool
}

// nameSets zwraca zbiory \i i \c dla wydania wybranego w parserze i zapamiętuje, że wyrażenie ich używa.
func (p *parser) nameSets() (start, char rangeSet) {
	p.usesNames = true
	if p.names5 {
		return xmlNameStart5, xmlNameChar5
	}
	return xmlNameStart, xmlNameChar
}

func (p *parser) fail(format string, args ...any) error {
	return &Error{Code: CodeSyntax, Pattern: p.src, Offset: p.pos, Message: fmt.Sprintf(format, args...)}
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) peek() rune {
	c, _ := p.decode(p.pos)
	return c
}

func (p *parser) peekAt(n int) rune {
	pos := p.pos
	for ; n > 0 && pos < len(p.src); n-- {
		_, w := p.decode(pos)
		pos += w
	}
	c, _ := p.decode(pos)
	return c
}

func (p *parser) next() rune {
	c, w := p.decode(p.pos)
	p.pos += w
	return c
}

// decode czyta znak na pozycji pos; zwraca -1 na końcu. Bajt spoza UTF-8 daje utf8.RuneError o szerokości
// 1 i trafia do kontroli znaków XML jako błąd (invalidUTF8).
func (p *parser) decode(pos int) (rune, int) {
	if pos >= len(p.src) {
		return -1, 0
	}
	c, w := utf8.DecodeRuneInString(p.src[pos:])
	if c == utf8.RuneError && w == 1 {
		return invalidUTF8, 1
	}
	return c, w
}

// invalidUTF8 zastępuje bajt spoza UTF-8; nie jest znakiem XML, więc wzorzec zostaje odrzucony.
const invalidUTF8 = -2

// regExp ::= branch ( '|' branch )*. Zwraca największy iloczyn liczników zagnieżdżonych w wyrażeniu
// (regexp/syntax odrzuca iloczyn większy niż 1000).
func (p *parser) regExp(b *strings.Builder) (int, error) {
	prod, err := p.branch(b)
	if err != nil {
		return 0, err
	}
	for !p.eof() && p.peek() == '|' {
		p.next()
		b.WriteByte('|')
		q, err := p.branch(b)
		if err != nil {
			return 0, err
		}
		prod = max(prod, q)
	}
	return prod, nil
}

// branch ::= piece*
func (p *parser) branch(b *strings.Builder) (int, error) {
	prod := 1
	for !p.eof() {
		c := p.peek()
		if c == '|' || c == ')' {
			break
		}
		q, err := p.piece(b)
		if err != nil {
			return 0, err
		}
		prod = max(prod, q)
	}
	return prod, nil
}

// piece ::= atom quantifier?
func (p *parser) piece(b *strings.Builder) (int, error) {
	var atom strings.Builder
	prod, err := p.atom(&atom)
	if err != nil {
		return 0, err
	}
	x := atom.String()
	if p.eof() {
		b.WriteString(x)
		return prod, nil
	}
	switch p.peek() {
	case '?', '*', '+':
		b.WriteString("(?:" + x + ")" + string(p.next()))
		return prod, nil
	case '{':
		start := p.pos
		min, max, err := p.quantity()
		if err != nil {
			return 0, err
		}
		if !repeatFits(len(x), prod, min, max) || b.Len() > maxExpansion {
			return 0, &Error{Code: CodeTooLarge, Pattern: p.src, Offset: start, Message: "quantifier too large to expand"}
		}
		return emitRepeat(b, x, prod, min, max), nil
	}
	b.WriteString(x)
	return prod, nil
}

// quantity czyta '{' quantity '}': {n}, {n,} albo {n,m} z n <= m. max == -1 oznacza brak górnej granicy.
func (p *parser) quantity() (int, int, error) {
	p.next() // '{'
	num := func() (int, error) {
		start := p.pos
		for !p.eof() && p.peek() >= '0' && p.peek() <= '9' {
			p.next()
		}
		if start == p.pos {
			return 0, p.fail("quantifier needs a number")
		}
		v, err := strconv.Atoi(p.src[start:p.pos])
		if err != nil {
			return 0, &Error{Code: CodeTooLarge, Pattern: p.src, Offset: start, Message: "quantifier too large"}
		}
		return v, nil
	}
	n, err := num()
	if err != nil {
		return 0, 0, err
	}
	m := n
	if !p.eof() && p.peek() == ',' {
		p.next()
		if !p.eof() && p.peek() == '}' {
			m = -1
		} else if m, err = num(); err != nil {
			return 0, 0, err
		}
	}
	if p.eof() || p.next() != '}' {
		return 0, 0, p.fail("unterminated quantifier")
	}
	if m != -1 && m < n {
		return 0, 0, p.fail("quantifier {%d,%d} has max below min", n, m)
	}
	return n, m, nil
}

// repeatFits mówi, czy rozwinięcie X{n,m} przez emitRepeat zmieści się w maxExpansion.
func repeatFits(size, inner, n, m int) bool {
	count := max(n, m)
	if count == 0 || count*inner <= maxRepeat {
		return true
	}
	var copies int
	switch {
	case inner == 1 && m == -1:
		copies = (n+maxRepeat-1)/maxRepeat + 1
	case inner == 1:
		copies = (n+maxRepeat-1)/maxRepeat + (m-n+maxRepeat-1)/maxRepeat
	case m == -1:
		copies = n + 1
	default:
		copies = m
	}
	return copies <= maxExpansion/(size+8)
}

// emitRepeat zapisuje X{n,m} w składni przyjmowanej przez RE2. Liczniki do 1000 przechodzą wprost. Większe
// dzielimy na odcinki: X{0,2500} ≡ X{0,1000}X{0,1000}X{0,500} (ten sam język, bo liczba powtórzeń rozkłada
// się na odcinki). Gdy X sam zawiera licznik i iloczyn przekroczyłby 1000, rozwijamy zewnętrzny licznik na
// konkatenację kopii X. Czas dopasowania zostaje liniowy; rośnie rozmiar programu RE2.
func emitRepeat(b *strings.Builder, x string, inner, n, m int) int {
	g := "(?:" + x + ")"
	count := n
	if m > n {
		count = m
	}
	if count == 0 {
		// X{0} i X{0,0}: tylko pusty napis.
		b.WriteString("(?:)")
		return 1
	}
	if count*inner <= maxRepeat {
		switch {
		case m == -1:
			fmt.Fprintf(b, "%s{%d,}", g, n)
		case m == n:
			fmt.Fprintf(b, "%s{%d}", g, n)
		default:
			fmt.Fprintf(b, "%s{%d,%d}", g, n, m)
		}
		return count * inner
	}
	if inner == 1 {
		limit := maxRepeat
		for left := n; left > 0; left -= limit {
			fmt.Fprintf(b, "%s{%d}", g, min(left, limit))
		}
		if m == -1 {
			b.WriteString(g + "*")
			return limit
		}
		for left := m - n; left > 0; left -= limit {
			fmt.Fprintf(b, "%s{0,%d}", g, min(left, limit))
		}
		return limit
	}
	for i := 0; i < n; i++ {
		b.WriteString(g)
	}
	if m == -1 {
		b.WriteString(g + "*")
		return inner
	}
	for i := n; i < m; i++ {
		b.WriteString(g + "?")
	}
	return inner
}

// atom ::= Char | charClass | '(' regExp ')'
func (p *parser) atom(b *strings.Builder) (int, error) {
	c := p.peek()
	switch c {
	case '(':
		p.next()
		p.depth++
		if p.depth > maxRepeat {
			return 0, &Error{Code: CodeTooLarge, Pattern: p.src, Offset: p.pos, Message: "groups nested too deeply"}
		}
		b.WriteString("(?:")
		prod, err := p.regExp(b)
		if err != nil {
			return 0, err
		}
		if p.eof() || p.next() != ')' {
			return 0, p.fail("missing )")
		}
		p.depth--
		b.WriteString(")")
		return prod, nil
	case '[':
		p.next()
		set, err := p.charClassExpr()
		if err != nil {
			return 0, err
		}
		writeSet(b, set)
		return 1, nil
	case '.':
		p.next()
		writeSet(b, rangeSet{{'\n', '\n'}, {'\r', '\r'}}.complement())
		return 1, nil
	case '\\':
		set, single, err := p.escape()
		if err != nil {
			return 0, err
		}
		if single >= 0 {
			b.WriteString(regexp.QuoteMeta(string(single)))
		} else {
			writeSet(b, set)
		}
		return 1, nil
	case '?', '*', '+', ')', '{', '}', ']', '|':
		return 0, p.fail("unexpected metacharacter %q", c)
	}
	p.next()
	if !xmlChars.contains(c) {
		return 0, p.fail("character %U is not an XML character", c)
	}
	b.WriteString(regexp.QuoteMeta(string(c)))
	return 1, nil
}

// escape czyta charClassEsc po '\'. Dla pojedynczego znaku (SingleCharEsc) zwraca go w single, dla klas
// zwraca zbiór i single == -1.
func (p *parser) escape() (rangeSet, rune, error) {
	p.next() // '\'
	if p.eof() {
		return nil, 0, p.fail("trailing backslash")
	}
	c := p.next()
	switch c {
	case 'n':
		return nil, '\n', nil
	case 'r':
		return nil, '\r', nil
	case 't':
		return nil, '\t', nil
	case '\\', '|', '.', '?', '*', '+', '(', ')', '{', '}', '-', '[', ']', '^':
		return nil, c, nil
	case 's':
		return spaceSet, -1, nil
	case 'S':
		return spaceSet.complement(), -1, nil
	case 'i':
		start, _ := p.nameSets()
		return start, -1, nil
	case 'I':
		start, _ := p.nameSets()
		return start.complement(), -1, nil
	case 'c':
		_, char := p.nameSets()
		return char, -1, nil
	case 'C':
		_, char := p.nameSets()
		return char.complement(), -1, nil
	case 'd':
		return category("Nd"), -1, nil
	case 'D':
		return category("Nd").complement(), -1, nil
	case 'w':
		return wordSet(), -1, nil
	case 'W':
		return wordSet().complement(), -1, nil
	case 'p', 'P':
		if p.eof() || p.next() != '{' {
			return nil, 0, p.fail(`\%c needs {`, c)
		}
		end := strings.IndexByte(p.src[p.pos:], '}')
		if end < 0 {
			return nil, 0, p.fail("unterminated \\%c{", c)
		}
		name := p.src[p.pos : p.pos+end]
		set, ok := property(name)
		if !ok {
			return nil, 0, p.fail("unknown character property %q", name)
		}
		p.pos += end + 1
		if c == 'P' {
			set = set.complement()
		}
		return set, -1, nil
	}
	return nil, 0, p.fail("unknown escape \\%c", c)
}

// charClassExpr ::= '[' charGroup ']'; '[' już przeczytany.
// charGroup ::= posCharGroup | negCharGroup | charClassSub
func (p *parser) charClassExpr() (rangeSet, error) {
	negative := false
	if p.peek() == '^' {
		p.next()
		negative = true
	}
	var set rangeSet
	first := true
	for {
		if p.eof() {
			return nil, p.fail("unterminated character class")
		}
		c := p.peek()
		if c == ']' {
			if first {
				return nil, p.fail("empty character group")
			}
			p.next()
			break
		}
		if c == '-' {
			switch next := p.peekAt(1); {
			case next == '[' && !first:
				// charClassSub: grupa '-' charClassExpr ']'
				p.next()
				p.next()
				sub, err := p.charClassExpr()
				if err != nil {
					return nil, err
				}
				if p.eof() || p.next() != ']' {
					return nil, p.fail("character class subtraction must end the group")
				}
				set = set.normalize().intersect(xmlChars)
				if negative {
					set = set.complement()
				}
				return set.subtract(sub), nil
			case first || next == ']' || (next == '-' && p.peekAt(2) == '['):
				// także „-” kończące grupę tuż przed odejmowaniem: [a-z--[b-z]] (produkcje [14], [16]).
				// '-' jest zwykłym znakiem tylko na początku albo na końcu grupy.
				p.next()
				set = append(set, struct{ lo, hi rune }{'-', '-'})
				first = false
				continue
			default:
				return nil, p.fail("unescaped - inside character group")
			}
		}
		var lo rune
		switch {
		case c == '\\':
			esc, single, err := p.escape()
			if err != nil {
				return nil, err
			}
			if single < 0 {
				set = set.union(esc)
				first = false
				continue
			}
			lo = single
		case c == '[':
			return nil, p.fail("unescaped [ inside character group")
		default:
			p.next()
			if !xmlChars.contains(c) {
				return nil, p.fail("character %U is not an XML character", c)
			}
			lo = c
		}
		hi := lo
		// seRange ::= charOrEsc '-' charOrEsc, chyba że '-' kończy grupę albo zaczyna odejmowanie.
		if p.peek() == '-' && p.peekAt(1) != ']' && p.peekAt(1) != '[' && p.peekAt(1) != -1 {
			p.next()
			e := p.peek()
			switch e {
			case '\\':
				_, single, err := p.escape()
				if err != nil {
					return nil, err
				}
				if single < 0 {
					return nil, p.fail("range end must be a single character")
				}
				hi = single
			case '[':
				return nil, p.fail("range end must not be [")
			default:
				p.next()
				if !xmlChars.contains(e) {
					return nil, p.fail("character %U is not an XML character", e)
				}
				hi = e
			}
			if hi < lo {
				return nil, p.fail("range %q-%q is reversed", lo, hi)
			}
		}
		set = append(set, struct{ lo, hi rune }{lo, hi})
		first = false
	}
	set = set.normalize().intersect(xmlChars)
	if negative {
		set = set.complement()
	}
	return set, nil
}

var spaceSet = rangeSet{{'\t', '\n'}, {'\r', '\r'}, {' ', ' '}}

func wordSet() rangeSet {
	// \w ≡ [#x0000-#x10FFFF]-[\p{P}\p{Z}\p{C}]
	return xmlChars.subtract(category("P").union(category("Z")).union(category("C")))
}

// category zwraca zbiór znaków kategorii ogólnej Unicode według tablic biblioteki standardowej Go. "C"
// i "Cn" liczymy sami: XSD wyłącza Cs (surogaty) i włącza Cn (znaki nieprzypisane), których Go nie ma
// w tablicach.
func category(name string) rangeSet {
	categoryOnce.Do(func() {
		categoryCache = map[string]rangeSet{}
		for n := range categoryNames {
			categoryCache[n] = computeCategory(n)
		}
	})
	return categoryCache[name]
}

var (
	categoryOnce  sync.Once
	categoryCache map[string]rangeSet
)

func computeCategory(name string) rangeSet {
	switch name {
	case "Cn":
		// Znaki przypisane to suma kategorii dwuliterowych bez Cn; nowsze Go mają tablicę "Cn" (i "C" ją
		// obejmuje), starsze nie, więc liczymy tak samo w obu przypadkach.
		var assigned rangeSet
		for n, t := range unicode.Categories {
			if len(n) == 2 && n != "Cn" {
				assigned = assigned.union(fromTable(t))
			}
		}
		return assigned.complement()
	case "C":
		return fromTable(unicode.Cc).union(fromTable(unicode.Cf)).union(fromTable(unicode.Co)).union(computeCategory("Cn")).intersect(xmlChars)
	}
	t, ok := unicode.Categories[name]
	if !ok {
		return nil
	}
	return fromTable(t).intersect(xmlChars)
}

// categoryNames to wartości IsCategory z produkcji [28]–[35].
var categoryNames = map[string]bool{
	"L": true, "Lu": true, "Ll": true, "Lt": true, "Lm": true, "Lo": true,
	"M": true, "Mn": true, "Mc": true, "Me": true,
	"N": true, "Nd": true, "Nl": true, "No": true,
	"P": true, "Pc": true, "Pd": true, "Ps": true, "Pe": true, "Pi": true, "Pf": true, "Po": true,
	"Z": true, "Zs": true, "Zl": true, "Zp": true,
	"S": true, "Sm": true, "Sc": true, "Sk": true, "So": true,
	"C": true, "Cc": true, "Cf": true, "Co": true, "Cn": true,
}

func property(name string) (rangeSet, bool) {
	if categoryNames[name] {
		return category(name), true
	}
	if block, ok := strings.CutPrefix(name, "Is"); ok {
		switch block {
		case "HighSurrogates", "LowSurrogates", "HighPrivateUseSurrogates":
			// F.1 wyłącza bloki surogatów, bo ich znaki nie występują w dokumentach XML. Zestaw W3C (MS-Regex
			// reN75) oczekuje jednak poprawnego schematu: nazwę przyjmujemy, a zbiór jest pusty (ADR 0005).
			return nil, true
		}
		set, ok := unicodeBlocks[block]
		return set.intersect(xmlChars), ok
	}
	return nil, false
}

// writeSet zapisuje zbiór jako klasę RE2 z jawnymi przedziałami. Pusty zbiór nie pasuje do niczego.
func writeSet(b *strings.Builder, s rangeSet) {
	if len(s) == 0 {
		b.WriteString(`[^\x00-\x{10FFFF}]`)
		return
	}
	b.WriteByte('[')
	for _, r := range s {
		fmt.Fprintf(b, `\x{%X}`, r.lo)
		if r.hi != r.lo {
			fmt.Fprintf(b, `-\x{%X}`, r.hi)
		}
	}
	b.WriteByte(']')
}
