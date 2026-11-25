package datatypes

import (
	"bytes"
	"math"
	"strconv"
	"strings"
)

// Kind to typ pierwotny wartości (część 2, 3.2), a dla anySimpleType KindAnySimpleType.
type Kind uint8

const (
	KindAnySimpleType Kind = iota
	KindString
	KindBoolean
	KindDecimal
	KindFloat
	KindDouble
	KindDuration
	KindDateTime
	KindTime
	KindDate
	KindGYearMonth
	KindGYear
	KindGMonthDay
	KindGDay
	KindGMonth
	KindHexBinary
	KindBase64Binary
	KindAnyURI
	KindQName
	KindNOTATION
	// KindList oznacza wartość typu listowego.
	KindList
)

var kindNames = [...]string{"anySimpleType", "string", "boolean", "decimal", "float", "double", "duration",
	"dateTime", "time", "date", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth", "hexBinary",
	"base64Binary", "anyURI", "QName", "NOTATION", "list"}

func (k Kind) String() string { return kindNames[k] }

// Value to wartość z przestrzeni wartości typu prostego.
type Value interface {
	Kind() Kind
}

// Order to wynik porównania w przestrzeni wartości. Porządek bywa częściowy (duration, typy daty i czasu,
// NaN), stąd Incomparable.
type Order int8

const (
	Less         Order = -1
	Equal        Order = 0
	Greater      Order = 1
	Incomparable Order = 2
)

// StringValue to wartość typów string i anyURI oraz anySimpleType (napis po normalizacji).
type StringValue struct {
	kind Kind
	S    string
}

func (v StringValue) Kind() Kind { return v.kind }

// BooleanValue to wartość boolean.
type BooleanValue bool

func (BooleanValue) Kind() Kind { return KindBoolean }

// FloatValue to wartość float albo double. W przestrzeni wartości jest jedno zero i jeden NaN, a NaN jest
// równy samemu sobie (3.2.4); float trzyma wartość zaokrągloną do float32.
type FloatValue struct {
	F      float64
	Single bool
}

func (v FloatValue) Kind() Kind {
	if v.Single {
		return KindFloat
	}
	return KindDouble
}

// BinaryValue to wartość hexBinary albo base64Binary: ciąg oktetów.
type BinaryValue struct {
	kind  Kind
	Bytes []byte
}

func (v BinaryValue) Kind() Kind { return v.kind }

// QNameValue to wartość QName albo NOTATION: {przestrzeń nazw, nazwa lokalna} (3.2.18).
type QNameValue struct {
	kind      Kind
	Namespace string
	Local     string
}

func (v QNameValue) Kind() Kind { return v.kind }

// ListValue to wartość typu listowego: ciąg wartości typu elementu.
type ListValue []Value

func (ListValue) Kind() Kind { return KindList }

// Compare porównuje dwie wartości. Wartości różnych typów pierwotnych są nieporównywalne; dla typów bez
// porządku wynik to Equal albo Incomparable.
func Compare(a, b Value) Order {
	if a == nil || b == nil || a.Kind() != b.Kind() {
		return Incomparable
	}
	switch x := a.(type) {
	case *Decimal:
		return order(x.Cmp(b.(*Decimal)))
	case FloatValue:
		y := b.(FloatValue)
		switch {
		case math.IsNaN(x.F) && math.IsNaN(y.F):
			return Equal
		case math.IsNaN(x.F) || math.IsNaN(y.F):
			return Incomparable
		case x.F < y.F:
			return Less
		case x.F > y.F:
			return Greater
		}
		return Equal // także 0 i -0: jedno zero w przestrzeni wartości
	case *DurationValue:
		return x.compare(b.(*DurationValue))
	case *DateTimeValue:
		return x.compare(b.(*DateTimeValue))
	case StringValue:
		return eq(x.S == b.(StringValue).S)
	case BooleanValue:
		return eq(x == b.(BooleanValue))
	case BinaryValue:
		return eq(bytes.Equal(x.Bytes, b.(BinaryValue).Bytes))
	case QNameValue:
		y := b.(QNameValue)
		return eq(x.Namespace == y.Namespace && x.Local == y.Local)
	case ListValue:
		y := b.(ListValue)
		if len(x) != len(y) {
			return Incomparable
		}
		for i := range x {
			if Compare(x[i], y[i]) != Equal {
				return Incomparable
			}
		}
		return Equal
	}
	return Incomparable
}

// Identical mówi, czy dwie wartości są tą samą wartością (równość z rekomendacji, np. dla enumeration).
func Identical(a, b Value) bool { return Compare(a, b) == Equal }

func order(c int) Order {
	switch {
	case c < 0:
		return Less
	case c > 0:
		return Greater
	}
	return Equal
}

func eq(b bool) Order {
	if b {
		return Equal
	}
	return Incomparable
}

// Canonical zwraca postać kanoniczną wartości (część 2, 2.3.1 i podpunkty „Canonical representation”).
func Canonical(v Value) string {
	switch x := v.(type) {
	case *Decimal:
		return x.String()
	case FloatValue:
		return canonicalFloat(x)
	case BooleanValue:
		if x {
			return "true"
		}
		return "false"
	case StringValue:
		return x.S
	case BinaryValue:
		if x.kind == KindHexBinary {
			return strings.ToUpper(hexString(x.Bytes))
		}
		return base64String(x.Bytes)
	case QNameValue:
		if x.Namespace == "" {
			return x.Local
		}
		return "{" + x.Namespace + "}" + x.Local
	case *DurationValue:
		return x.String()
	case *DateTimeValue:
		return x.String()
	case ListValue:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = Canonical(item)
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// canonicalFloat zapisuje float i double w postaci kanonicznej (3.2.4.2): mantysa z jedną niezerową cyfrą
// przed kropką, wykładnik po „E”, zero jako 0.0E0.
func canonicalFloat(v FloatValue) string {
	switch {
	case math.IsNaN(v.F):
		return "NaN"
	case math.IsInf(v.F, 1):
		return "INF"
	case math.IsInf(v.F, -1):
		return "-INF"
	case v.F == 0:
		return "0.0E0"
	}
	bits := 64
	if v.Single {
		bits = 32
	}
	s := strconv.FormatFloat(v.F, 'E', -1, bits) // np. 1.2345E+06, 5E-01
	mant, exp, _ := strings.Cut(s, "E")
	if !strings.Contains(mant, ".") {
		mant += ".0"
	}
	e, _ := strconv.Atoi(exp)
	return mant + "E" + strconv.Itoa(e)
}
