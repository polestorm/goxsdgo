// Package xmlchars zawiera klasy znaków nazw z XML 1.0 wspólne dla czytnika XML, tłumacza wyrażeń regularnych
// i typów prostych.
//
// Dwa zestawy reguł znaków nazw (ADR 0012):
//   - XML 1.0 Second Edition, dodatek B (Letter, Digit, CombiningChar, Extender): do tego wydania odsyła XSD 1.0 2e
//     dla typów Name, NCName, NMTOKEN i pokrewnych oraz dla klas \i i \c we wzorcach (część 2, dodatek F). Tablice
//     tego wydania są w internal/regex (wygenerowane z tekstu rekomendacji).
//   - XML 1.0 Fifth Edition, produkcje [4] NameStartChar i [4a] NameChar: według nich czyta dokumenty czytnik
//     (ADR 0004); na życzenie także typy nazw i klasy \i, \c (xsd.Config.NameRules).
package xmlchars

// Edition wybiera reguły znaków nazw.
type Edition uint8

const (
	// Edition2: XML 1.0 Second Edition (domyślnie dla typów nazw i wzorców, dosłownie według odsyłacza XSD 1.0 2e).
	Edition2 Edition = iota
	// Edition5: XML 1.0 Fifth Edition (jak czytnik dokumentów).
	Edition5
)

// NameStart5 to przedziały produkcji [4] NameStartChar z XML 1.0 Fifth Edition, rosnąco.
var NameStart5 = [][2]rune{
	{':', ':'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'},
	{0xC0, 0xD6}, {0xD8, 0xF6}, {0xF8, 0x2FF}, {0x370, 0x37D}, {0x37F, 0x1FFF}, {0x200C, 0x200D},
	{0x2070, 0x218F}, {0x2C00, 0x2FEF}, {0x3001, 0xD7FF}, {0xF900, 0xFDCF}, {0xFDF0, 0xFFFD}, {0x10000, 0xEFFFF},
}

// NameCharExtra5 to przedziały, które produkcja [4a] NameChar dodaje do NameStartChar, rosnąco.
var NameCharExtra5 = [][2]rune{
	{'-', '.'}, {'0', '9'}, {0xB7, 0xB7}, {0x300, 0x36F}, {0x203F, 0x2040},
}

// IsNameStart5 odpowiada produkcji [4] NameStartChar z XML 1.0 Fifth Edition.
func IsNameStart5(r rune) bool {
	if r < 0x80 {
		return r == ':' || r == '_' || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
	}
	switch {
	case 0xC0 <= r && r <= 0xD6, 0xD8 <= r && r <= 0xF6, 0xF8 <= r && r <= 0x2FF,
		0x370 <= r && r <= 0x37D, 0x37F <= r && r <= 0x1FFF, 0x200C <= r && r <= 0x200D,
		0x2070 <= r && r <= 0x218F, 0x2C00 <= r && r <= 0x2FEF, 0x3001 <= r && r <= 0xD7FF,
		0xF900 <= r && r <= 0xFDCF, 0xFDF0 <= r && r <= 0xFFFD, 0x10000 <= r && r <= 0xEFFFF:
		return true
	}
	return false
}

// IsNameChar5 odpowiada produkcji [4a] NameChar z XML 1.0 Fifth Edition.
func IsNameChar5(r rune) bool {
	if r < 0x80 {
		return IsNameStart5(r) || r == '-' || r == '.' || ('0' <= r && r <= '9')
	}
	return IsNameStart5(r) || r == 0xB7 || (0x300 <= r && r <= 0x36F) || (0x203F <= r && r <= 0x2040)
}
