package regex

import "github.com/polestorm/goxsdgo/internal/xmlchars"

// xmlNameStart5 i xmlNameChar5 to \i i \c według XML 1.0 Fifth Edition (produkcje [4] i [4a]), z tych samych
// przedziałów co czytnik dokumentów (internal/xmlchars). Używane tylko z xsd.Config.NameRules (ADR 0012).
var (
	xmlNameStart5 = fromRanges(xmlchars.NameStart5)
	xmlNameChar5  = fromRanges(xmlchars.NameStart5).union(fromRanges(xmlchars.NameCharExtra5))
)

func fromRanges(rs [][2]rune) rangeSet {
	s := make(rangeSet, len(rs))
	for i, r := range rs {
		s[i].lo, s[i].hi = r[0], r[1]
	}
	return s.normalize()
}
