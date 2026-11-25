package regex

import (
	"sort"
	"unicode"
)

// rangeSet to zbiór znaków jako posortowane, rozłączne i niesąsiadujące przedziały kodów.
// Klasy znaków XSD liczymy jawnie na zbiorach (odejmowanie klas, dopełnienia względem znaków XML),
// a do RE2 trafia gotowa lista przedziałów.
type rangeSet []struct{ lo, hi rune }

// xmlChars to znaki XML 1.0: #x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF].
// Dopełnienie klasy w XSD jest liczone względem tego zbioru (F.1, negatywna grupa znaków).
var xmlChars = rangeSet{{0x9, 0xA}, {0xD, 0xD}, {0x20, 0xD7FF}, {0xE000, 0xFFFD}, {0x10000, 0x10FFFF}}

func (s rangeSet) normalize() rangeSet {
	if len(s) == 0 {
		return nil
	}
	sort.Slice(s, func(i, j int) bool { return s[i].lo < s[j].lo })
	out := s[:1]
	for _, r := range s[1:] {
		last := &out[len(out)-1]
		if r.lo <= last.hi+1 {
			if r.hi > last.hi {
				last.hi = r.hi
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

func (s rangeSet) union(t rangeSet) rangeSet {
	u := make(rangeSet, 0, len(s)+len(t))
	u = append(u, s...)
	u = append(u, t...)
	return u.normalize()
}

func (s rangeSet) intersect(t rangeSet) rangeSet {
	var out rangeSet
	i, j := 0, 0
	for i < len(s) && j < len(t) {
		lo, hi := max(s[i].lo, t[j].lo), min(s[i].hi, t[j].hi)
		if lo <= hi {
			out = append(out, struct{ lo, hi rune }{lo, hi})
		}
		if s[i].hi < t[j].hi {
			i++
		} else {
			j++
		}
	}
	return out
}

// invert zwraca dopełnienie s w całej przestrzeni kodów [0, MaxRune].
func (s rangeSet) invert() rangeSet {
	var gaps rangeSet
	next := rune(0)
	for _, r := range s {
		if r.lo > next {
			gaps = append(gaps, struct{ lo, hi rune }{next, r.lo - 1})
		}
		next = r.hi + 1
	}
	if next <= unicode.MaxRune {
		gaps = append(gaps, struct{ lo, hi rune }{next, unicode.MaxRune})
	}
	return gaps
}

// complement zwraca znaki XML spoza s.
func (s rangeSet) complement() rangeSet { return s.invert().intersect(xmlChars) }

// subtract zwraca znaki z s, których nie ma w t.
func (s rangeSet) subtract(t rangeSet) rangeSet { return s.intersect(t.invert()) }

func (s rangeSet) contains(c rune) bool {
	i := sort.Search(len(s), func(i int) bool { return s[i].hi >= c })
	return i < len(s) && s[i].lo <= c
}

// fromTable zamienia tablicę Unicode z biblioteki standardowej na rangeSet.
func fromTable(t *unicode.RangeTable) rangeSet {
	var s rangeSet
	for _, r := range t.R16 {
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			if r.Stride == 1 {
				s = append(s, struct{ lo, hi rune }{rune(r.Lo), rune(r.Hi)})
				break
			}
			s = append(s, struct{ lo, hi rune }{c, c})
		}
	}
	for _, r := range t.R32 {
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			if r.Stride == 1 {
				s = append(s, struct{ lo, hi rune }{rune(r.Lo), rune(r.Hi)})
				break
			}
			s = append(s, struct{ lo, hi rune }{c, c})
		}
	}
	return s.normalize()
}
