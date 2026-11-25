package datatypes

import (
	"math/big"
	"strings"
)

// Decimal to wartość z przestrzeni wartości decimal: unscaled × 10^-scale (część 2, 3.2.3). Postać jest
// znormalizowana: scale >= 0 i najmniejsze możliwe (bez zer na końcu części ułamkowej), zero ma scale 0.
// Precyzja nie należy do wartości: 2.0 i 2.00 to ta sama wartość.
//
// Ograniczenie implementacji (wymagane do udokumentowania przez 3.2.3): brak; cyfr może być dowolnie wiele,
// liczy je math/big.
type Decimal struct {
	unscaled big.Int
	scale    int
}

func (Decimal) Kind() Kind { return KindDecimal }

// parseDecimal czyta literał decimal (3.2.3.1): opcjonalny znak, cyfry z opcjonalną kropką dziesiętną,
// przynajmniej jedna cyfra. Zwraca false dla literału spoza przestrzeni leksykalnej.
func parseDecimal(s string) (*Decimal, bool) {
	if s == "" {
		return nil, false
	}
	neg := false
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		neg = true
		s = s[1:]
	}
	intPart, frac, hasDot := strings.Cut(s, ".")
	if intPart == "" && frac == "" {
		return nil, false
	}
	if !allDigits(intPart) || !allDigits(frac) || (hasDot && strings.Contains(frac, ".")) {
		return nil, false
	}
	frac = strings.TrimRight(frac, "0")
	d := &Decimal{scale: len(frac)}
	digits := strings.TrimLeft(intPart+frac, "0")
	if digits == "" {
		return &Decimal{}, true
	}
	d.unscaled.SetString(digits, 10)
	if neg {
		d.unscaled.Neg(&d.unscaled)
	}
	return d, true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Cmp porównuje dwie wartości decimal (porządek liczb rzeczywistych).
func (d *Decimal) Cmp(e *Decimal) int {
	if d.scale == e.scale {
		return d.unscaled.Cmp(&e.unscaled)
	}
	a, b := new(big.Int).Set(&d.unscaled), new(big.Int).Set(&e.unscaled)
	if d.scale < e.scale {
		a.Mul(a, pow10(e.scale-d.scale))
	} else {
		b.Mul(b, pow10(d.scale-e.scale))
	}
	return a.Cmp(b)
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// digitCounts zwraca liczbę cyfr potrzebnych do zapisu wartości (dla totalDigits: |i| < 10^td) i liczbę
// cyfr po przecinku (dla fractionDigits i warunku n <= td), według 4.3.11.3 i 4.3.12.3.
func (d *Decimal) digitCounts() (total, fraction int) {
	if d.unscaled.Sign() == 0 {
		return 1, 0
	}
	abs := new(big.Int).Abs(&d.unscaled)
	return len(abs.String()), d.scale
}

// Sign zwraca -1, 0 albo 1.
func (d *Decimal) Sign() int { return d.unscaled.Sign() }

// IsInteger mówi, czy wartość jest liczbą całkowitą.
func (d *Decimal) IsInteger() bool { return d.scale == 0 }

// Int64 zwraca wartość całkowitą, jeśli mieści się w int64.
func (d *Decimal) Int64() (int64, bool) {
	if d.scale != 0 || !d.unscaled.IsInt64() {
		return 0, false
	}
	return d.unscaled.Int64(), true
}

// String zwraca postać kanoniczną decimal (3.2.3.2): bez „+”, z kropką i co najmniej jedną cyfrą po obu
// stronach, bez zbędnych zer.
func (d *Decimal) String() string {
	s := new(big.Int).Abs(&d.unscaled).String()
	if d.scale > 0 {
		for len(s) <= d.scale {
			s = "0" + s
		}
		s = s[:len(s)-d.scale] + "." + s[len(s)-d.scale:]
	} else {
		s += ".0"
	}
	if d.unscaled.Sign() < 0 {
		s = "-" + s
	}
	return s
}

// integerString zwraca postać kanoniczną integer (3.3.13.2): bez „+” i zer wiodących.
func (d *Decimal) integerString() string {
	return d.unscaled.String()
}
