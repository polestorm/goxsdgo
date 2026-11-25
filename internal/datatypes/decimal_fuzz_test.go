package datatypes

import (
	"math/big"
	"strings"
	"testing"
)

// ratOf czyta literał decimal niezależnie od parseDecimal: wzorzec dla FuzzDecimal.
func ratOf(s string) (*big.Rat, bool) {
	t := s
	if strings.HasPrefix(t, "+") || strings.HasPrefix(t, "-") {
		t = t[1:]
	}
	if t == "" || t == "." || strings.Count(t, ".") > 1 {
		return nil, false
	}
	for _, c := range t {
		if c != '.' && (c < '0' || c > '9') {
			return nil, false
		}
	}
	return new(big.Rat).SetString(strings.TrimPrefix(s, "+"))
}

// FuzzDecimal porównuje Decimal (szybka ścieżka int64 i math/big, ADR 0011) z niezależnym wzorcem na big.Rat:
// przyjęcie literału, porządek, znak, postać kanoniczna, liczbę cyfr i jednoznaczność reprezentacji.
// REQ: DET-005
func FuzzDecimal(f *testing.F) {
	for _, p := range [][2]string{
		{"1.5", "1.50"}, {"-0", "0.0"}, {"999999999999999999", "1000000000000000000"},
		{"9223372036854775807", "9223372036854775808"}, {"-9223372036854775808", "-9223372036854775809"},
		{"0.000000000000000001", "0.0000000000000000001"}, {"123456789012345678.9", "12345678901234567.89"},
		{"+.5", "0.5"}, {"5.", "005.000"}, {"-1e5", "1"}, {"-.0", "+0."},
	} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if len(a) > 200 || len(b) > 200 {
			return
		}
		da, okA := parseDecimal(a)
		ra, okRA := ratOf(a)
		if okA != okRA {
			t.Fatalf("%q: parseDecimal ok=%v, reference ok=%v", a, okA, okRA)
		}
		db, okB := parseDecimal(b)
		rb, okRB := ratOf(b)
		if okB != okRB {
			t.Fatalf("%q: parseDecimal ok=%v, reference ok=%v", b, okB, okRB)
		}
		if !okA || !okB {
			return
		}
		if got, want := da.Cmp(db), ra.Cmp(rb); got != want {
			t.Fatalf("Cmp(%q, %q) = %d, want %d", a, b, got, want)
		}
		if da.Sign() != ra.Sign() {
			t.Fatalf("Sign(%q) = %d, want %d", a, da.Sign(), ra.Sign())
		}
		c := da.String()
		rc, ok := new(big.Rat).SetString(c)
		if !ok || rc.Cmp(ra) != 0 {
			t.Fatalf("%q: canonical %q is a different value", a, c)
		}
		dc, ok := parseDecimal(c)
		if !ok || dc.String() != c || dc.Cmp(da) != 0 {
			t.Fatalf("%q: canonical %q not stable", a, c)
		}
		// Jednoznaczność reprezentacji: int64, gdy unscaled się mieści, inaczej big.
		for _, d := range []*Decimal{da, dc} {
			if d.big != nil && d.big.IsInt64() {
				t.Fatalf("%q: big representation for a value that fits int64", a)
			}
		}
		if (da.big == nil) != (dc.big == nil) || da.scale != dc.scale || da.small != dc.small {
			t.Fatalf("%q and its canonical %q have different representations", a, c)
		}
		// Liczba cyfr: unscaled bez znaku i zer wiodących; liczba cyfr ułamka to skala.
		total, frac := da.digitCounts()
		ip, fp, _ := strings.Cut(strings.TrimPrefix(c, "-"), ".")
		if fp == "0" {
			fp = ""
		}
		want := len(strings.TrimLeft(ip+fp, "0"))
		if want == 0 {
			want = 1
		}
		if total != want || frac != len(fp) {
			t.Fatalf("%q (canonical %q): digitCounts = %d,%d, want %d,%d", a, c, total, frac, want, len(fp))
		}
	})
}
