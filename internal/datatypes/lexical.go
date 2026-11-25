package datatypes

import (
	"encoding/base64"
	"encoding/hex"
	"math"
	"strconv"
	"strings"

	"github.com/polestorm/goxsdgo/internal/regex"
	"github.com/polestorm/goxsdgo/internal/xmlchars"
)

// NamespaceResolver rozwiązuje prefiks przestrzeni nazw w miejscu, w którym stoi wartość QName albo NOTATION
// (3.2.18: odwzorowanie literału na wartość wymaga deklaracji przestrzeni nazw w zasięgu). Pusty prefiks
// oznacza domyślną przestrzeń nazw; ok == false oznacza prefiks niezadeklarowany.
type NamespaceResolver interface {
	LookupPrefix(prefix string) (namespace string, ok bool)
}

// Context to otoczenie walidacji tekstu, potrzebne tylko niektórym typom.
type Context struct {
	Namespaces NamespaceResolver
	// Names wybiera reguły znaków nazw dla typów nazw (Name, NCName, NMTOKEN, ID, QName…) i klas \i, \c we
	// wzorcach: domyślnie XML 1.0 Second Edition (ADR 0012).
	Names xmlchars.Edition
}

// namesOf zwraca reguły znaków nazw z kontekstu (domyślne dla nil).
func namesOf(ctx *Context) xmlchars.Edition {
	if ctx == nil {
		return xmlchars.Edition2
	}
	return ctx.Names
}

// parsePrimitive odwzorowuje literał (po normalizacji białych znaków) na wartość typu pierwotnego kind.
// Zwraca false dla literału spoza przestrzeni leksykalnej.
func parsePrimitive(kind Kind, s string, ctx *Context) (Value, bool) {
	switch kind {
	case KindAnySimpleType, KindString:
		return StringValue{kind: kind, S: s}, true
	case KindAnyURI:
		if !validAnyURI(s) {
			return nil, false
		}
		return StringValue{kind: KindAnyURI, S: s}, true
	case KindBoolean:
		switch s {
		case "true", "1":
			return BooleanValue(true), true
		case "false", "0":
			return BooleanValue(false), true
		}
		return nil, false
	case KindDecimal:
		d, ok := parseDecimal(s)
		if !ok {
			return nil, false
		}
		return d, true
	case KindFloat, KindDouble:
		f, ok := parseFloat(s, kind == KindFloat)
		if !ok {
			return nil, false
		}
		return FloatValue{F: f, Single: kind == KindFloat}, true
	case KindDuration:
		d, ok := parseDuration(s)
		if !ok {
			return nil, false
		}
		return d, true
	case KindDateTime, KindTime, KindDate, KindGYearMonth, KindGYear, KindGMonthDay, KindGDay, KindGMonth:
		v, ok := parseDateTime(kind, s)
		if !ok {
			return nil, false
		}
		return v, true
	case KindHexBinary:
		if len(s)%2 != 0 {
			return nil, false
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, false
		}
		return BinaryValue{kind: KindHexBinary, Bytes: b}, true
	case KindBase64Binary:
		b, ok := parseBase64(s)
		if !ok {
			return nil, false
		}
		return BinaryValue{kind: KindBase64Binary, Bytes: b}, true
	case KindQName, KindNOTATION:
		prefix, local, ok := splitQName(s, namesOf(ctx))
		if !ok {
			return nil, false
		}
		ns := ""
		if ctx != nil && ctx.Namespaces != nil {
			var found bool
			ns, found = ctx.Namespaces.LookupPrefix(prefix)
			if !found && prefix != "" {
				return nil, false
			}
		} else if prefix != "" && prefix != "xml" {
			// Bez kontekstu przestrzeni nazw prefiksu nie da się rozwiązać.
			return nil, false
		}
		if prefix == "xml" {
			ns = "http://www.w3.org/XML/1998/namespace"
		}
		return QNameValue{kind: kind, Namespace: ns, Local: local}, true
	}
	return nil, false
}

// parseFloat czyta literał float albo double (3.2.4.1): mantysa decimal, opcjonalnie E|e i wykładnik integer,
// albo INF, -INF, NaN. Odwzorowanie na najbliższą wartość z zaokrągleniem do parzystej (3.2.4) wykonuje
// strconv.ParseFloat; wartości poza zakresem dają ±INF, a -0 to zero.
func parseFloat(s string, single bool) (float64, bool) {
	switch s {
	case "INF":
		return math.Inf(1), true
	case "-INF":
		return math.Inf(-1), true
	case "NaN":
		return math.NaN(), true
	}
	mant, exp := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant, exp = s[:i], s[i+1:]
		if exp == "" {
			return 0, false
		}
		e := exp
		if e[0] == '+' || e[0] == '-' {
			e = e[1:]
		}
		if e == "" || !allDigits(e) {
			return 0, false
		}
	}
	if _, ok := parseDecimal(mant); !ok {
		return 0, false
	}
	bits := 64
	if single {
		bits = 32
	}
	f, err := strconv.ParseFloat(s, bits)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
			return 0, false
		}
	}
	if f == 0 {
		f = 0 // jedno zero: -0 i 0 to ta sama wartość
	}
	return f, true
}

// parseBase64 sprawdza produkcję Base64Binary (3.2.16) na tekście po normalizacji collapse i dekoduje go.
// Po collapse spacje są pojedyncze, więc wystarczy je usunąć; warunki B04 i B16 dla końcowych znaków
// sprawdza dekodowanie ścisłe.
func parseBase64(s string) ([]byte, bool) {
	compact := strings.ReplaceAll(s, " ", "")
	if len(compact)%4 != 0 {
		return nil, false
	}
	for i := 0; i < len(compact); i++ {
		c := compact[i]
		isB64 := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/'
		if !isB64 && !(c == '=' && i >= len(compact)-2) {
			return nil, false
		}
	}
	if n := len(compact); n >= 2 && compact[n-2] == '=' && compact[n-1] != '=' {
		return nil, false
	}
	b, err := base64.StdEncoding.Strict().DecodeString(compact)
	if err != nil {
		return nil, false
	}
	return b, true
}

func hexString(b []byte) string { return hex.EncodeToString(b) }

func base64String(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// splitQName dzieli literał według produkcji QName z Namespaces in XML: (NCName ':')? NCName.
func splitQName(s string, ed xmlchars.Edition) (prefix, local string, ok bool) {
	prefix, local, found := strings.Cut(s, ":")
	if !found {
		prefix, local = "", s
	}
	if (found && !isNCName(prefix, ed)) || !isNCName(local, ed) {
		return "", "", false
	}
	return prefix, local, true
}

var ncNamePattern = mustRegex(`[\i-[:]][\c-[:]]*`)

func isNCName(s string, ed xmlchars.Edition) bool {
	return s != "" && ncNamePattern.MatchStringNames(s, ed)
}

func mustRegex(p string) *regex.Regexp {
	re, err := regex.Compile(p)
	if err != nil {
		panic(err)
	}
	return re
}

// validAnyURI sprawdza literał anyURI (3.2.17.1) w zakresie, który da się sprawdzić bez znajomości schematu
// adresu: po zastąpieniu znaków spoza ASCII i znaków niedozwolonych sekwencjami %HH (XLink 5.4) wynik musi
// być odwołaniem URI według RFC 2396 z poprawką RFC 2732. Sprawdzamy: każdy „%” poprzedza dwie cyfry
// szesnastkowe, najwyżej jeden „#”, a jeśli przed pierwszym „/”, „?” i „#” stoi „:”, to część przed nim jest
// poprawnym schematem. Rekomendacja zaznacza, że reguł poszczególnych schematów procesor nie sprawdza.
func validAnyURI(s string) bool {
	if strings.Count(s, "#") > 1 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			if i+2 >= len(s) || !isHex(s[i+1]) || !isHex(s[i+2]) {
				return false
			}
		}
	}
	end := strings.IndexAny(s, "/?#")
	head := s
	if end >= 0 {
		head = s[:end]
	}
	if colon := strings.IndexByte(head, ':'); colon >= 0 {
		scheme := head[:colon]
		if scheme == "" || !isAlpha(scheme[0]) {
			return false
		}
		for i := 1; i < len(scheme); i++ {
			c := scheme[i]
			if !isAlpha(c) && !isDigit(c) && c != '+' && c != '-' && c != '.' {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
