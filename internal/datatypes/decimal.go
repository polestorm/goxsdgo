package datatypes

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Decimal to wartość z przestrzeni wartości decimal: unscaled × 10^-scale (część 2, 3.2.3). Postać jest
// znormalizowana: scale >= 0 i najmniejsze możliwe (bez zer na końcu części ułamkowej), zero ma scale 0.
// Precyzja nie należy do wartości: 2.0 i 2.00 to ta sama wartość.
//
// Reprezentacja (etap 7, ADR 0011): unscaled mieszczące się w int64 trzymamy w polu small bez alokacji; tylko
// większe w big. Niezmiennik: big != nil wtedy i tylko wtedy, gdy unscaled nie mieści się w int64, więc każda
// wartość ma dokładnie jedną reprezentację i porównanie pól nie zależy od historii obliczeń.
//
// Ograniczenie implementacji (wymagane do udokumentowania przez 3.2.3): brak; cyfr może być dowolnie wiele,
// duże wartości liczy math/big.
type Decimal struct {
	small int64
	big   *big.Int // tylko do odczytu po utworzeniu: wartości współdzielą go przy kopiowaniu
	scale int
}

func (Decimal) Kind() Kind { return KindDecimal }

// maxSmallDigits to liczba cyfr, która zawsze mieści się w int64 (10^18 - 1 < 2^63).
const maxSmallDigits = 18

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
	intPart = strings.TrimLeft(intPart, "0")
	d := &Decimal{scale: len(frac)}
	if intPart == "" && frac == "" {
		return d, true // zero: scale 0
	}
	if intPart == "" {
		frac = strings.TrimLeft(frac, "0") // 0.05: cyfry „5”, scale 2
	}
	if len(intPart)+len(frac) <= maxSmallDigits {
		var v int64
		for i := 0; i < len(intPart); i++ {
			v = v*10 + int64(intPart[i]-'0')
		}
		for i := 0; i < len(frac); i++ {
			v = v*10 + int64(frac[i]-'0')
		}
		if neg {
			v = -v
		}
		d.small = v
		return d, true
	}
	b, _ := new(big.Int).SetString(intPart+frac, 10)
	if neg {
		b.Neg(b)
	}
	d.setUnscaled(b)
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

// setUnscaled ustawia unscaled z x, z zachowaniem niezmiennika reprezentacji. Nie zatrzymuje x.
func (d *Decimal) setUnscaled(x *big.Int) {
	if x.IsInt64() {
		d.small, d.big = x.Int64(), nil
		return
	}
	d.small, d.big = 0, new(big.Int).Set(x)
}

// unscaledBig zwraca unscaled jako nowy big.Int (wolno go zmieniać).
func (d *Decimal) unscaledBig() *big.Int {
	if d.big != nil {
		return new(big.Int).Set(d.big)
	}
	return big.NewInt(d.small)
}

// negated zwraca wartość przeciwną.
func (d Decimal) negated() Decimal {
	if d.big == nil {
		if d.small == math.MinInt64 {
			d.setUnscaled(new(big.Int).Neg(big.NewInt(d.small)))
			return d
		}
		d.small = -d.small
		return d
	}
	d.setUnscaled(new(big.Int).Neg(d.big))
	return d
}

// pow10s to potęgi dziesięciu mieszczące się w int64.
var pow10s = func() [19]int64 {
	var p [19]int64
	p[0] = 1
	for i := 1; i < len(p); i++ {
		p[i] = p[i-1] * 10
	}
	return p
}()

// mulPow10Small zwraca v × 10^k, jeśli wynik mieści się w int64.
func mulPow10Small(v int64, k int) (int64, bool) {
	if k >= len(pow10s) {
		return 0, v == 0
	}
	p := pow10s[k]
	if v > math.MaxInt64/p || v < math.MinInt64/p {
		return 0, false
	}
	return v * p, true
}

// Cmp porównuje dwie wartości decimal (porządek liczb rzeczywistych).
func (d *Decimal) Cmp(e *Decimal) int {
	if d.big == nil && e.big == nil {
		a, b := d.small, e.small
		ok := true
		switch {
		case d.scale < e.scale:
			a, ok = mulPow10Small(a, e.scale-d.scale)
		case d.scale > e.scale:
			b, ok = mulPow10Small(b, d.scale-e.scale)
		}
		if ok {
			switch {
			case a < b:
				return -1
			case a > b:
				return 1
			}
			return 0
		}
	}
	a, b := d.unscaledBig(), e.unscaledBig()
	if d.scale < e.scale {
		a.Mul(a, pow10(e.scale-d.scale))
	} else if d.scale > e.scale {
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
	if d.big != nil {
		return len(new(big.Int).Abs(d.big).String()), d.scale
	}
	if d.small == 0 {
		return 1, 0
	}
	u := uint64(d.small)
	if d.small < 0 {
		u = uint64(-(d.small + 1)) + 1 // także dla MinInt64
	}
	n := 0
	for u > 0 {
		u /= 10
		n++
	}
	return n, d.scale
}

// Sign zwraca -1, 0 albo 1.
func (d *Decimal) Sign() int {
	if d.big != nil {
		return d.big.Sign()
	}
	switch {
	case d.small < 0:
		return -1
	case d.small > 0:
		return 1
	}
	return 0
}

// IsInteger mówi, czy wartość jest liczbą całkowitą.
func (d *Decimal) IsInteger() bool { return d.scale == 0 }

// Int64 zwraca wartość całkowitą, jeśli mieści się w int64.
func (d *Decimal) Int64() (int64, bool) {
	if d.scale != 0 || d.big != nil {
		return 0, false
	}
	return d.small, true
}

// uint64Value zwraca nieujemną wartość całkowitą jako uint64; ok=false, gdy się nie mieści.
func (d *Decimal) uint64Value() (uint64, bool) {
	if d.big != nil {
		if d.big.IsUint64() {
			return d.big.Uint64(), true
		}
		return 0, false
	}
	if d.small < 0 {
		return 0, false
	}
	return uint64(d.small), true
}

// absDigits zwraca cyfry |unscaled| w zapisie dziesiętnym.
func (d *Decimal) absDigits() string {
	if d.big != nil {
		return new(big.Int).Abs(d.big).String()
	}
	if d.small < 0 {
		s := strconv.FormatInt(d.small, 10)
		return s[1:]
	}
	return strconv.FormatInt(d.small, 10)
}

// String zwraca postać kanoniczną decimal (3.2.3.2): bez „+”, z kropką i co najmniej jedną cyfrą po obu
// stronach, bez zbędnych zer.
func (d *Decimal) String() string {
	s := d.absDigits()
	if d.scale > 0 {
		if len(s) <= d.scale {
			s = strings.Repeat("0", d.scale-len(s)+1) + s
		}
		s = s[:len(s)-d.scale] + "." + s[len(s)-d.scale:]
	} else {
		s += ".0"
	}
	if d.Sign() < 0 {
		s = "-" + s
	}
	return s
}

// integerString zwraca postać kanoniczną integer (3.3.13.2): bez „+” i zer wiodących.
func (d *Decimal) integerString() string {
	if d.big != nil {
		return d.big.String()
	}
	return strconv.FormatInt(d.small, 10)
}
