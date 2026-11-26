package regex

import (
	"errors"
	"html"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/polestorm/goxsdgo/internal/testkit"
)

type matchCase struct {
	pattern string
	match   []string
	noMatch []string
}

func runMatch(t *testing.T, cases []matchCase) {
	t.Helper()
	for _, c := range cases {
		re, err := Compile(c.pattern)
		if err != nil {
			t.Errorf("%q: %v", c.pattern, err)
			continue
		}
		for _, s := range c.match {
			if !re.MatchString(s) {
				t.Errorf("%q should match %q", c.pattern, s)
			}
		}
		for _, s := range c.noMatch {
			if re.MatchString(s) {
				t.Errorf("%q should not match %q", c.pattern, s)
			}
		}
	}
}

// REQ: RE-001
func TestAnchored(t *testing.T) {
	runMatch(t, []matchCase{
		{"A.*Z", []string{"AZ", "AbcZ"}, []string{"xAZ", "AZx", "A\nZ"}},
		{".*AAA.*", []string{"AAA", "xAAAy"}, []string{"AA"}},
		{"", []string{""}, []string{"a"}},
		{"a|", []string{"a", ""}, []string{"b"}},
	})
}

// REQ: RE-002
func TestQuantifiers(t *testing.T) {
	runMatch(t, []matchCase{
		{"a?", []string{"", "a"}, []string{"aa"}},
		{"a*", []string{"", "aaaa"}, []string{"b"}},
		{"a+", []string{"a", "aa"}, []string{""}},
		{"a{3}", []string{"aaa"}, []string{"aa", "aaaa"}},
		{"a{2,}", []string{"aa", "aaaaa"}, []string{"a"}},
		{"a{1,2}", []string{"a", "aa"}, []string{"", "aaa"}},
		{"a{0,0}", []string{""}, []string{"a"}},
		{"(ab){2}c", []string{"ababc"}, []string{"abc"}},
	})
	for _, bad := range []string{"a{2,1}", "a{", "a{x}", "a{1", "a{,2}", "a**", "*a", "+", "a{1}{2}"} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-003
func TestNormalCharacters(t *testing.T) {
	runMatch(t, []matchCase{
		{"^a$", []string{"^a$"}, []string{"a"}},
		{`\.\?\*\+\(\)\{\}\[\]\|\\\-\^`, []string{`.?*+(){}[]|\-^`}, nil},
		{"zażółć", []string{"zażółć"}, []string{"zazolc"}},
		{"#$%&'", []string{"#$%&'"}, nil},
	})
	for _, bad := range []string{"a)", "(a", "a]", "{", "}", "a\\"} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-004
func TestSingleCharEscapes(t *testing.T) {
	runMatch(t, []matchCase{
		{`\n\r\t`, []string{"\n\r\t"}, []string{"nrt"}},
		{`[\n\t]`, []string{"\n", "\t"}, []string{"n"}},
	})
	for _, bad := range []string{`\a`, `\x41`, `\0`, `\b`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-005
func TestMultiCharEscapes(t *testing.T) {
	runMatch(t, []matchCase{
		{".", []string{"a", "ż", "\t", "\U00010000"}, []string{"\n", "\r", ""}},
		{`\s+`, []string{" \t\n\r"}, []string{" ", " "}},
		{`\S`, []string{"a"}, []string{" "}},
		{`\d+`, []string{"0123", "٣", "३"}, []string{"a", "²"}},
		{`\D`, []string{"a"}, []string{"5"}},
		{`\w+`, []string{"abcŻ9", "̀"}, []string{"-", " ", ".", "\u00ad"}},
		{`\W`, []string{"!", " "}, []string{"a"}},
		// \i i \c według XML 1.0 2e: litery z dodatku B, '_' i ':'; cyfry tylko w \c.
		{`\i\c*`, []string{"a", "_x", ":x", "a-b.c", "ł1", "à"}, []string{"1a", "-a", ".a"}},
		{`\I`, []string{"1", "-"}, []string{"a", "_"}},
		{`\C`, []string{" ", "!"}, []string{"a", "1"}},
	})
	// Znaki dodane do Unicode po wersji 2.0 nie są literami w XML 1.0 2e (np. U+0221, U+04CF).
	re, _ := Compile(`\i`)
	for _, c := range []string{"ȡ", "ӏ", "€"} {
		if re.MatchString(c) {
			t.Errorf(`\i matches %U`, []rune(c)[0])
		}
	}
}

// REQ: RE-006
func TestCategoryEscapes(t *testing.T) {
	runMatch(t, []matchCase{
		{`\p{Lu}`, []string{"A", "Ż"}, []string{"a"}},
		{`\p{Ll}`, []string{"a", "ż"}, []string{"A"}},
		{`\p{L}+`, []string{"Abż"}, []string{"1"}},
		{`\P{L}`, []string{"1", " "}, []string{"a"}},
		{`\p{Nd}`, []string{"7"}, []string{"Ⅶ"}},
		{`\p{Nl}`, []string{"Ⅶ"}, nil},
		{`\p{N}`, []string{"7", "Ⅶ", "½"}, []string{"a"}},
		{`\p{P}`, []string{"!", "-", "«"}, []string{"a"}},
		{`\p{Pi}`, []string{"«"}, []string{"»"}},
		{`\p{Zs}`, []string{" ", " "}, []string{"\t"}},
		{`\p{Sc}`, []string{"$", "€"}, []string{"z"}},
		{`\p{Cc}`, []string{"\t", "\x7f"}, []string{"a"}},
		{`\p{Co}`, []string{"\ue000"}, []string{"a"}},
		// Cn: znaki nieprzypisane (U+0378 jest nieprzypisany w każdej wersji Unicode).
		{`\p{Cn}`, []string{"͸"}, []string{"a"}},
		{`\p{C}`, []string{"\t", "\ue000", "͸", "\u200b"}, []string{"a"}},
	})
	for _, bad := range []string{`\p{Cs}`, `\p{Lx}`, `\p{}`, `\p{L`, `\pL`, `\p{l}`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-007
func TestBlockEscapes(t *testing.T) {
	runMatch(t, []matchCase{
		{`\p{IsBasicLatin}+`, []string{"abc~\t"}, []string{"ą"}},
		{`\p{IsLatinExtended-A}`, []string{"ą", "ł"}, []string{"a"}},
		{`\p{IsLatin-1Supplement}`, []string{"é"}, []string{"ą"}},
		{`\P{IsBasicLatin}`, []string{"ą"}, []string{"a"}},
		{`\p{IsSpecials}`, []string{"\ufeff", "�"}, []string{"a"}},
		{`\p{IsPrivateUse}`, []string{"\ue000", "\uf8ff"}, []string{"a", "\U000F0000", "\U00100000"}},
		{`\p{IsOldItalic}`, []string{"\U00010300"}, []string{"a"}},
		{`\p{IsCJKUnifiedIdeographsExtensionB}`, []string{"\U00020000"}, nil},
	})
	if len(unicodeBlocks) != 93 {
		t.Errorf("%d Unicode 3.1 blocks, want 93 (96 minus 3 surrogate blocks)", len(unicodeBlocks))
	}
	// Bloki surogatów: nazwa przyjęta, zbiór pusty (ADR 0005; zestaw W3C, MS-Regex reN75).
	for _, p := range []string{`\p{IsHighSurrogates}`, `\p{IsLowSurrogates}`, `\p{IsHighPrivateUseSurrogates}`} {
		re, err := Compile(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if re.MatchString("a") || re.MatchString("\ufffd") {
			t.Errorf("%s matches a character", p)
		}
	}
	for _, bad := range []string{`\p{IsBasic Latin}`,
		`\p{IsLatinExtended-C}`, `\p{Isbasiclatin}`, `\p{IsaA0-a9}`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-008
func TestCharacterClassExpressions(t *testing.T) {
	runMatch(t, []matchCase{
		{"[abc]", []string{"a", "c"}, []string{"d", "ab"}},
		{"[a-z]", []string{"m"}, []string{"A"}},
		{"[^a-z]", []string{"A", "1", "\n"}, []string{"m"}},
		{"[-a]", []string{"-", "a"}, []string{"b"}},
		{"[a-]", []string{"-", "a"}, []string{"b"}},
		{"[^-a]", []string{"b"}, []string{"-", "a"}},
		{"[a-z-[aeiou]]", []string{"b", "z"}, []string{"a", "e"}},
		{"[a-z-[d-w-[m]]]", []string{"a", "m", "z"}, []string{"e"}},
		{"[a-z--[b-z]]", []string{"a", "-"}, []string{"b", "z"}},
		{"[^a-z-[xyz]]", []string{"1"}, []string{"a", "x"}},
		{`[\p{L}-[\p{Lu}]]`, []string{"a"}, []string{"A", "1"}},
		{`[\d\s]`, []string{"5", " "}, []string{"a"}},
		{`[\--/]`, []string{"-", ".", "/"}, []string{","}},
		{`[\^a]`, []string{"^", "a"}, []string{"b"}},
		{`[a^]`, []string{"^", "a"}, []string{"b"}},
		{`[.]`, []string{"."}, []string{"a"}},
		{`[*+?(){}|$]`, []string{"*", "+", "?", "(", ")", "{", "}", "|", "$"}, []string{"a"}},
	})
	for _, bad := range []string{"[]", "[^]", "[a-b-c]", "[z-a]", "[a", "[[a]]", "a[b-[c]", "[a-[b]c]", `[\w-z]`,
		"[a-[]", `[a-\d]`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// REQ: RE-009
func TestSyntaxErrorCode(t *testing.T) {
	_, err := Compile("ab[c")
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeSyntax || e.Pattern != "ab[c" {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(e.Error(), CodeSyntax) {
		t.Errorf("message %q", e.Error())
	}
}

// REQ: RE-011
func TestLargeQuantifiers(t *testing.T) {
	re, err := Compile("a{2500}")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(strings.Repeat("a", 2500)) || re.MatchString(strings.Repeat("a", 2499)) ||
		re.MatchString(strings.Repeat("a", 2501)) {
		t.Error("a{2500}")
	}
	re, err = Compile("[0-9]{0,3000}x")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(strings.Repeat("1", 3000)+"x") || !re.MatchString("x") || re.MatchString(strings.Repeat("1", 3001)+"x") {
		t.Error("[0-9]{0,3000}x")
	}
	re, err = Compile("b{1500,}")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(strings.Repeat("b", 4000)) || re.MatchString(strings.Repeat("b", 1499)) {
		t.Error("b{1500,}")
	}
	// Licznik zagnieżdżony w liczniku: iloczyn 1200 > 1000, zewnętrzny licznik jest rozwijany.
	re, err = Compile(`(\d{3}){400}`)
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(strings.Repeat("123", 400)) || re.MatchString(strings.Repeat("123", 399)) {
		t.Error(`(\d{3}){400}`)
	}
	re, err = Compile(`(a{2}){0,600}`)
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(strings.Repeat("a", 1200)) || re.MatchString(strings.Repeat("a", 1201)) || !re.MatchString("") {
		t.Error(`(a{2}){0,600}`)
	}
	// Zbyt duże po rozwinięciu: jasny błąd, nie zawieszenie.
	_, err = Compile(`((a{1000}){1000}){1000}`)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeTooLarge {
		t.Errorf("huge nested quantifier: %v", err)
	}
	_, err = Compile("a{99999999999999999999}")
	if !errors.As(err, &e) || e.Code != CodeTooLarge {
		t.Errorf("overflowing quantifier: %v", err)
	}
}

// REQ: RE-010
func TestPathologicalLinear(t *testing.T) {
	for _, c := range []struct{ pattern, input string }{
		{"(a*)*b", strings.Repeat("a", 1<<16)},
		{"(a|aa)*c", strings.Repeat("a", 1<<16)},
		{"(a+)+$", strings.Repeat("a", 1<<16) + "!"},
		{`(\w|\d)*X`, strings.Repeat("1", 1<<16)},
		{"(.*)*(.*)*y", strings.Repeat("x", 1<<15)},
	} {
		re, err := Compile(c.pattern)
		if err != nil {
			t.Fatal(err)
		}
		measure := func(s string) time.Duration {
			start := time.Now()
			re.MatchString(s)
			return time.Since(start)
		}
		small := measure(c.input[:len(c.input)/8])
		big := measure(c.input)
		// Czas liniowy: 8 razy dłuższe wejście nie może trwać kilkaset razy dłużej (zapas na szum).
		if big > 200*small+50*time.Millisecond {
			t.Errorf("%q: %v for n, %v for n/8: not linear", c.pattern, big, small)
		}
	}
}

func TestTranslateIsValidRE2(t *testing.T) {
	for _, p := range []string{`\i\c*`, `[\i-[:]][\c-[:]]*`, `\w+@\p{L}+`, `[^\p{IsBasicLatin}]`} {
		s, err := Translate(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := regexp.Compile(s); err != nil {
			t.Errorf("%q -> %q: %v", p, s, err)
		}
	}
}

func FuzzRegexTranslate(f *testing.F) {
	for _, s := range []string{"a*b", "[a-z-[aeiou]]", `\p{IsBasicLatin}{2,5}`, `(\d{3}){400}`, `[^\i]`, "a{1,3000}", "((a)|b)*"} {
		f.Add(s)
	}
	// Wzorce ze schematów MF i z próbki zestawu W3C (ADR 0011), z pobranych danych, bez commitowania.
	pat := regexp.MustCompile(`<(?:xs|xsd):pattern\s+value="([^"]*)"`)
	for _, dir := range []string{"../../testdata/schemas/mf", "../../testdata/w3c/xsdtests/msData/regex"} {
		seen := map[string]bool{}
		for _, b := range testkit.SampleFiles(dir, []string{".xsd"}, 400, 256<<10) {
			for _, m := range pat.FindAllSubmatch(b, -1) {
				s := html.UnescapeString(string(m[1]))
				if !seen[s] && len(s) <= 256 {
					seen[s] = true
					f.Add(s)
				}
			}
		}
	}
	f.Fuzz(func(t *testing.T, pattern string) {
		if len(pattern) > 256 {
			return
		}
		start := time.Now()
		s, err := Translate(pattern)
		if err != nil {
			return
		}
		if _, err := regexp.Compile(s); err != nil {
			var e *Error
			// Compile zgłasza przekroczenie rozmiaru jako regex-too-large; inne błędy RE2 to błąd tłumacza.
			if _, cerr := Compile(pattern); !errors.As(cerr, &e) || e.Code != CodeTooLarge {
				t.Fatalf("%q -> %q: %v", pattern, s, err)
			}
			return
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("%q: translation took %v", pattern, d)
		}
	})
}
