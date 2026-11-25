package datatypes

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Ograniczenia implementacji (część 2, 3.2.6 i 3.2.7 wymagają ich udokumentowania; minimum to rok
// czterocyfrowy i trzy cyfry ułamka sekundy):
//   - rok w typach daty i czasu: najwyżej 12 cyfr (|rok| < 10^12);
//   - pola duration (lata, miesiące, dni, godziny, minuty): najwyżej 15 cyfr każde;
//   - ułamek sekundy i liczba sekund w duration: bez ograniczenia.
const (
	maxYearDigits     = 12
	maxDurationDigits = 15
)

const (
	secondsPerDay = 86400
	// maxTZ to największe przesunięcie strefy w minutach (3.2.7.3) i zakres nieokreśloności porównania
	// wartości ze strefą i bez niej (3.2.7.4): 14 godzin.
	maxTZ = 14 * 60
)

// DateTimeValue to wartość jednego z typów dateTime, time, date, gYearMonth, gYear, gMonthDay, gDay, gMonth.
// Pola nieobecne w danym typie mają wartość zerową. Wartości dateTime i time ze strefą są sprowadzone do UTC
// (3.2.7, 3.2.8); pozostałe typy zachowują strefę z literału, a porównanie i równość liczymy na chwili
// początkowej (3.2.9–3.2.14).
type DateTimeValue struct {
	kind                 Kind
	Year                 int64
	Month, Day           int
	Hour, Minute, Second int
	// Frac to cyfry ułamka sekundy bez zer na końcu ("" dla zera).
	Frac string
	// HasTZ mówi, czy wartość ma strefę; TZ to przesunięcie w minutach.
	HasTZ bool
	TZ    int
}

func (v *DateTimeValue) Kind() Kind { return v.kind }

// fields mówi, które pola ma dany typ.
type fields struct{ year, month, day, time bool }

var kindFields = map[Kind]fields{
	KindDateTime:   {true, true, true, true},
	KindTime:       {false, false, false, true},
	KindDate:       {true, true, true, false},
	KindGYearMonth: {true, true, false, false},
	KindGYear:      {true, false, false, false},
	KindGMonthDay:  {false, true, true, false},
	KindGDay:       {false, false, true, false},
	KindGMonth:     {false, true, false, false},
}

// parseDateTime czyta literał typu kind (3.2.7.1, 3.2.8.1, 3.2.9.1, 3.2.10.1–3.2.14.1).
func parseDateTime(kind Kind, s string) (*DateTimeValue, bool) {
	v := &DateTimeValue{kind: kind}
	f := kindFields[kind]
	p := s
	switch kind {
	case KindGMonthDay, KindGMonth:
		if !strings.HasPrefix(p, "--") {
			return nil, false
		}
		p = p[2:]
	case KindGDay:
		if !strings.HasPrefix(p, "---") {
			return nil, false
		}
		p = p[3:]
	}
	var ok bool
	if f.year {
		if p, ok = v.parseYear(p); !ok {
			return nil, false
		}
		if f.month {
			if p, ok = expect(p, '-'); !ok {
				return nil, false
			}
		}
	}
	if f.month {
		if v.Month, p, ok = twoDigits(p); !ok || v.Month < 1 || v.Month > 12 {
			return nil, false
		}
		if f.day {
			if p, ok = expect(p, '-'); !ok {
				return nil, false
			}
		}
	}
	if f.day {
		if v.Day, p, ok = twoDigits(p); !ok || v.Day < 1 {
			return nil, false
		}
		switch {
		case f.year && v.Day > daysInMonth(v.Year, v.Month):
			return nil, false
		case !f.year && f.month && v.Day > daysInMonth(2000, v.Month): // gMonthDay: dowolny rok przestępny
			return nil, false
		case v.Day > 31:
			return nil, false
		}
	}
	if f.time {
		if f.day {
			if p, ok = expect(p, 'T'); !ok {
				return nil, false
			}
		}
		if p, ok = v.parseTime(p); !ok {
			return nil, false
		}
	}
	if p != "" {
		tz, ok := parseTZ(p)
		if !ok {
			return nil, false
		}
		v.HasTZ, v.TZ = true, tz
	}
	if f.time && v.Hour == 24 {
		// 24:00:00 to pierwsza chwila następnego dnia (3.2.7.1); dla time po prostu 00:00:00.
		v.Hour = 0
		if f.day {
			v.addDays(1)
		}
	}
	if v.HasTZ && kind == KindDateTime {
		v.toUTC()
	}
	// time zachowuje strefę: porządek liczymy na dowolnej, stałej dacie (3.2.8), więc 12:00:00-14:00 to
	// 02:00:00Z następnego dnia, a nie 02:00:00Z tej samej doby. Postać kanoniczna sprowadza do UTC.
	return v, true
}

func expect(p string, c byte) (string, bool) {
	if p == "" || p[0] != c {
		return p, false
	}
	return p[1:], true
}

func twoDigits(p string) (int, string, bool) {
	if len(p) < 2 || !isDigit(p[0]) || !isDigit(p[1]) {
		return 0, p, false
	}
	return int(p[0]-'0')*10 + int(p[1]-'0'), p[2:], true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// parseYear czyta '-'? yyyy: co najmniej cztery cyfry, bez zer wiodących przy więcej niż czterech, bez 0000
// (3.2.7.1, przypis o roku 0).
func (v *DateTimeValue) parseYear(p string) (string, bool) {
	neg := false
	if strings.HasPrefix(p, "-") {
		neg = true
		p = p[1:]
	}
	n := 0
	for n < len(p) && isDigit(p[n]) {
		n++
	}
	digits := p[:n]
	if n < 4 || (n > 4 && digits[0] == '0') || n > maxYearDigits {
		return p, false
	}
	y, _ := strconv.ParseInt(digits, 10, 64)
	if y == 0 {
		return p, false
	}
	if neg {
		y = -y
	}
	v.Year = y
	return p[n:], true
}

// parseTime czyta hh ':' mm ':' ss ('.' s+)?.
func (v *DateTimeValue) parseTime(p string) (string, bool) {
	var ok bool
	if v.Hour, p, ok = twoDigits(p); !ok || v.Hour > 24 {
		return p, false
	}
	if p, ok = expect(p, ':'); !ok {
		return p, false
	}
	if v.Minute, p, ok = twoDigits(p); !ok || v.Minute > 59 {
		return p, false
	}
	if p, ok = expect(p, ':'); !ok {
		return p, false
	}
	if v.Second, p, ok = twoDigits(p); !ok || v.Second > 59 {
		return p, false
	}
	if strings.HasPrefix(p, ".") {
		n := 1
		for n < len(p) && isDigit(p[n]) {
			n++
		}
		if n == 1 {
			return p, false
		}
		v.Frac = strings.TrimRight(p[1:n], "0")
		p = p[n:]
	}
	if v.Hour == 24 && (v.Minute != 0 || v.Second != 0 || v.Frac != "") {
		return p, false
	}
	return p, true
}

// parseTZ czyta strefę (3.2.7.3): Z albo ±hh:mm z |hh| <= 14, mm <= 59, a przy 14 minuty 00.
func parseTZ(p string) (int, bool) {
	if p == "Z" {
		return 0, true
	}
	if len(p) != 6 || (p[0] != '+' && p[0] != '-') || p[3] != ':' {
		return 0, false
	}
	h, _, ok1 := twoDigits(p[1:3])
	m, _, ok2 := twoDigits(p[4:6])
	if !ok1 || !ok2 || h > 14 || m > 59 || (h == 14 && m != 0) {
		return 0, false
	}
	tz := h*60 + m
	if p[0] == '-' {
		tz = -tz
	}
	return tz, true
}

// daysInMonth to maximumDayInMonthFor z dodatku E dla miesiąca 1–12.
func daysInMonth(year int64, month int) int {
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		if mod(year, 400) == 0 || (mod(year, 100) != 0 && mod(year, 4) == 0) {
			return 29
		}
		return 28
	}
	return 31
}

func mod(a, b int64) int64 {
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

func fquot(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// addDays przesuwa datę o n dni (pętla dni z dodatku E, w krokach miesięcznych).
func (v *DateTimeValue) addDays(n int64) {
	day := int64(v.Day) + n
	for day < 1 {
		v.Month--
		if v.Month < 1 {
			v.Month, v.Year = 12, v.Year-1
		}
		day += int64(daysInMonth(v.Year, v.Month))
	}
	for day > int64(daysInMonth(v.Year, v.Month)) {
		day -= int64(daysInMonth(v.Year, v.Month))
		v.Month++
		if v.Month > 12 {
			v.Month, v.Year = 1, v.Year+1
		}
	}
	v.Day = int(day)
}

// toUTC sprowadza dateTime albo time ze strefą do UTC (3.2.7.3); dla time dzień jest pomijany.
func (v *DateTimeValue) toUTC() {
	mins := v.Hour*60 + v.Minute - v.TZ
	days := fquot(int64(mins), 1440)
	mins = int(mod(int64(mins), 1440))
	v.Hour, v.Minute = mins/60, mins%60
	if v.kind == KindDateTime {
		v.addDays(days)
	}
	v.TZ = 0
}

// daysFromCivil zwraca numer dnia w kalendarzu gregoriańskim proleptycznym (rok liczony jak w dodatku E,
// wprost z literału). Służy tylko do porównań, więc wystarcza monotoniczność.
func daysFromCivil(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := fquot(y, 400)
	yoe := y - era*400
	mm := int64(m)
	if m > 2 {
		mm -= 3
	} else {
		mm += 9
	}
	doy := (153*mm+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe
}

// instant zwraca chwilę początkową wartości w sekundach (bez ułamka) i ułamek sekundy. Pola nieobecne
// w typie wypełniamy tak, jak opisują 3.2.8–3.2.14: rok przestępny 1972, grudzień (31 dni), dzień 1, 00:00:00.
func (v *DateTimeValue) instant() (int64, string) {
	f := kindFields[v.kind]
	y, m, d := int64(1972), 12, 1
	if f.year {
		y = v.Year
		m = 1
	}
	if f.month {
		m = v.Month
	}
	if f.day {
		d = v.Day
	}
	if !f.year && f.month && f.day {
		y = 1972 // gMonthDay: dowolny rok przestępny
	}
	secs := daysFromCivil(y, m, d)*secondsPerDay + int64(v.Hour*3600+v.Minute*60+v.Second)
	if v.HasTZ {
		secs -= int64(v.TZ) * 60
	}
	return secs, v.Frac
}

func compareInstant(as int64, af string, bs int64, bf string) Order {
	switch {
	case as < bs:
		return Less
	case as > bs:
		return Greater
	}
	return order(compareFrac(af, bf))
}

func compareFrac(a, b string) int {
	for len(a) < len(b) {
		a += "0"
	}
	for len(b) < len(a) {
		b += "0"
	}
	return strings.Compare(a, b)
}

// compare to porządek z 3.2.7.4: na tej samej osi (obie wartości ze strefą albo obie bez) porządek pełny;
// między osiami wartości różniące się o 14 godzin lub mniej są nieporównywalne.
func (v *DateTimeValue) compare(w *DateTimeValue) Order {
	as, af := v.instant()
	bs, bf := w.instant()
	if v.HasTZ == w.HasTZ {
		return compareInstant(as, af, bs, bf)
	}
	const shift = maxTZ * 60
	if v.HasTZ {
		// P ze strefą, Q bez: P < Q, gdy P < (Q z +14:00); P > Q, gdy P > (Q z -14:00).
		if compareInstant(as, af, bs-shift, bf) == Less {
			return Less
		}
		if compareInstant(as, af, bs+shift, bf) == Greater {
			return Greater
		}
		return Incomparable
	}
	// P bez strefy, Q ze strefą: P < Q, gdy (P z -14:00) < Q; P > Q, gdy (P z +14:00) > Q.
	if compareInstant(as+shift, af, bs, bf) == Less {
		return Less
	}
	if compareInstant(as-shift, af, bs, bf) == Greater {
		return Greater
	}
	return Incomparable
}

// String zwraca postać kanoniczną (3.2.7.2, 3.2.8.2, 3.2.9.2); dla typów g* literał ze strefą w postaci
// kanonicznej.
func (v *DateTimeValue) String() string {
	w := *v
	if w.kind == KindTime && w.HasTZ {
		w.toUTC()
	}
	if w.kind == KindDate && w.HasTZ {
		// Strefa odzyskiwalna leży w przedziale [-11:59, +12:00]; data przesuwa się o tyle dni, o ile
		// przesunęła się strefa (3.2.9).
		switch {
		case w.TZ > 12*60:
			w.TZ -= 24 * 60
			w.addDays(-1)
		case w.TZ < -(11*60 + 59):
			w.TZ += 24 * 60
			w.addDays(1)
		}
	}
	var b strings.Builder
	f := kindFields[w.kind]
	switch w.kind {
	case KindGMonthDay, KindGMonth:
		b.WriteString("--")
	case KindGDay:
		b.WriteString("---")
	}
	if f.year {
		if w.Year < 0 {
			b.WriteByte('-')
		}
		y := w.Year
		if y < 0 {
			y = -y
		}
		fmt.Fprintf(&b, "%04d", y)
	}
	if f.month {
		if f.year {
			b.WriteByte('-')
		}
		fmt.Fprintf(&b, "%02d", w.Month)
	}
	if f.day {
		if f.month {
			b.WriteByte('-')
		}
		fmt.Fprintf(&b, "%02d", w.Day)
	}
	if f.time {
		if f.day {
			b.WriteByte('T')
		}
		fmt.Fprintf(&b, "%02d:%02d:%02d", w.Hour, w.Minute, w.Second)
		if w.Frac != "" {
			b.WriteString("." + w.Frac)
		}
	}
	if w.HasTZ {
		b.WriteString(formatTZ(w.TZ))
	}
	return b.String()
}

func formatTZ(tz int) string {
	if tz == 0 {
		return "Z"
	}
	sign := '+'
	if tz < 0 {
		sign, tz = '-', -tz
	}
	return fmt.Sprintf("%c%02d:%02d", sign, tz/60, tz%60)
}

// DurationValue to wartość duration. Trzymamy ją jako liczbę miesięcy i liczbę sekund (z ułamkiem), ze
// wspólnym znakiem: P1Y i P12M to ta sama wartość, tak samo P1D i PT24H (ADR 0005). Porządek liczymy według
// 3.2.6.2: przez dodanie do czterech dat odniesienia.
type DurationValue struct {
	Negative bool
	Months   int64
	Seconds  Decimal // nieujemne
}

func (*DurationValue) Kind() Kind { return KindDuration }

// parseDuration czyta literał duration (3.2.6.1): -?P(nY)?(nM)?(nD)?(T(nH)?(nM)?(n(.n)?S)?)?, co najmniej
// jedna część, T tylko przed częściami czasu i tylko wtedy, gdy są.
func parseDuration(s string) (*DurationValue, bool) {
	v := &DurationValue{}
	p := s
	if strings.HasPrefix(p, "-") {
		v.Negative = true
		p = p[1:]
	}
	if !strings.HasPrefix(p, "P") {
		return nil, false
	}
	p = p[1:]
	datePart, timePart, hasT := strings.Cut(p, "T")
	if hasT && timePart == "" {
		return nil, false // P1YT
	}
	var vals [5]int64
	var secs *Decimal
	found := false
	read := func(part string, designators string, offset int, allowFrac bool) bool {
		last := -1
		for part != "" {
			n := 0
			for n < len(part) && (isDigit(part[n]) || (allowFrac && part[n] == '.')) {
				n++
			}
			if n == 0 || n >= len(part) {
				return false
			}
			idx := strings.IndexByte(designators, part[n])
			if idx < 0 || idx <= last {
				return false
			}
			last = idx
			num := part[:n]
			found = true
			if allowFrac && designators[idx] == 'S' {
				// [0-9]+(\.[0-9]+)?
				ip, fp, dot := strings.Cut(num, ".")
				if ip == "" || (dot && fp == "") || strings.Contains(fp, ".") {
					return false
				}
				d, ok := parseDecimal(num)
				if !ok {
					return false
				}
				secs = d
			} else {
				if strings.Contains(num, ".") || len(num) > maxDurationDigits {
					return false
				}
				vals[offset+idx], _ = strconv.ParseInt(num, 10, 64)
			}
			part = part[n+1:]
		}
		return true
	}
	if !read(datePart, "YMD", 0, false) || !read(timePart, "HMS", 3, true) || !found {
		return nil, false
	}
	// vals: 0=Y 1=M 2=D 3=H 4=M; sekundy w secs
	v.Months = vals[0]*12 + vals[1]
	// ((D*24 + H)*60 + M)*60 w big.Int: pola mają do 15 cyfr, iloczyn nie mieści się w int64.
	total := big.NewInt(vals[2])
	total.Mul(total, big.NewInt(24)).Add(total, big.NewInt(vals[3]))
	total.Mul(total, big.NewInt(60)).Add(total, big.NewInt(vals[4]))
	total.Mul(total, big.NewInt(60))
	sec := Decimal{}
	sec.unscaled.Set(total)
	if secs != nil {
		sec = addDecimal(&sec, secs)
	}
	v.Seconds = sec
	if v.Months == 0 && v.Seconds.Sign() == 0 {
		v.Negative = false // -P0D to zero
	}
	return v, true
}

func addDecimal(a, b *Decimal) Decimal {
	scale := max(a.scale, b.scale)
	x := new(big.Int).Mul(&a.unscaled, pow10(scale-a.scale))
	y := new(big.Int).Mul(&b.unscaled, pow10(scale-b.scale))
	var r Decimal
	r.unscaled.Add(x, y)
	r.scale = scale
	r.normalize()
	return r
}

func (d *Decimal) normalize() {
	if d.unscaled.Sign() == 0 {
		d.scale = 0
		return
	}
	ten := big.NewInt(10)
	q, m := new(big.Int), new(big.Int)
	for d.scale > 0 {
		q.QuoRem(&d.unscaled, ten, m)
		if m.Sign() != 0 {
			break
		}
		d.unscaled.Set(q)
		d.scale--
	}
}

// durationRefs to daty odniesienia z 3.2.6.2.
var durationRefs = [4][3]int64{{1696, 9, 1}, {1697, 2, 1}, {1903, 3, 1}, {1903, 7, 1}}

// endInstant zwraca s + d dla daty odniesienia s (dodatek E): najpierw miesiące z przypięciem dnia do końca
// miesiąca, potem sekundy (dni, godziny, minuty i sekundy dodają się liniowo). Wynik w sekundach od początku
// osi, jako Decimal.
func (d *DurationValue) endInstant(ref [3]int64) Decimal {
	months, secs := d.Months, d.Seconds
	if d.Negative {
		months = -months
		secs.unscaled.Neg(&secs.unscaled)
	}
	temp := ref[1] + months
	m := mod(temp-1, 12) + 1
	y := ref[0] + fquot(temp-1, 12)
	day := ref[2]
	if dim := int64(daysInMonth(y, int(m))); day > dim {
		day = dim
	}
	var start Decimal
	start.unscaled.SetInt64(daysFromCivil(y, int(m), int(day)) * secondsPerDay)
	return addDecimal(&start, &secs)
}

func (d *DurationValue) signedMonths() int64 {
	if d.Negative {
		return -d.Months
	}
	return d.Months
}

func (d *DurationValue) signedSeconds() *Decimal {
	s := d.Seconds
	if d.Negative {
		s.unscaled.Neg(&s.unscaled)
	}
	return &s
}

func (d *DurationValue) compare(e *DurationValue) Order {
	if d.signedMonths() == e.signedMonths() {
		return order(d.signedSeconds().Cmp(e.signedSeconds()))
	}
	result := Incomparable
	for i, ref := range durationRefs {
		a, b := d.endInstant(ref), e.endInstant(ref)
		o := order(a.Cmp(&b))
		if i == 0 {
			result = o
		} else if o != result {
			return Incomparable
		}
	}
	return result
}

// String zwraca znormalizowany zapis duration (rekomendacja 1.0 nie definiuje postaci kanonicznej): lata
// i miesiące z liczby miesięcy, dni, godziny, minuty i sekundy z liczby sekund.
func (d *DurationValue) String() string {
	var b strings.Builder
	if d.Negative {
		b.WriteByte('-')
	}
	b.WriteByte('P')
	if y, m := d.Months/12, d.Months%12; y != 0 || m != 0 {
		if y != 0 {
			fmt.Fprintf(&b, "%dY", y)
		}
		if m != 0 {
			fmt.Fprintf(&b, "%dM", m)
		}
	}
	whole := new(big.Int).Set(&d.Seconds.unscaled)
	frac := ""
	if d.Seconds.scale > 0 {
		s := d.Seconds.String()
		ip, fp, _ := strings.Cut(s, ".")
		whole.SetString(ip, 10)
		frac = fp
	}
	day := big.NewInt(secondsPerDay)
	days, rest := new(big.Int).QuoRem(whole, day, new(big.Int))
	r := rest.Int64()
	h, mi, sec := r/3600, r%3600/60, r%60
	if days.Sign() != 0 {
		fmt.Fprintf(&b, "%sD", days)
	}
	if h != 0 || mi != 0 || sec != 0 || frac != "" {
		b.WriteByte('T')
		if h != 0 {
			fmt.Fprintf(&b, "%dH", h)
		}
		if mi != 0 {
			fmt.Fprintf(&b, "%dM", mi)
		}
		if sec != 0 || frac != "" {
			fmt.Fprintf(&b, "%d", sec)
			if frac != "" {
				b.WriteString("." + frac)
			}
			b.WriteByte('S')
		}
	}
	if d.Months == 0 && d.Seconds.Sign() == 0 {
		return "PT0S"
	}
	return b.String()
}
