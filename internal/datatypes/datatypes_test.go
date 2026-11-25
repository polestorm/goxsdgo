package datatypes

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func bt(t *testing.T, name string) *Type {
	t.Helper()
	ty := Builtin(name)
	if ty == nil {
		t.Fatalf("no built-in %s", name)
	}
	return ty
}

type lexCase struct {
	typ     string
	valid   []string
	invalid []string
}

func runLex(t *testing.T, cases []lexCase) {
	t.Helper()
	for _, c := range cases {
		ty := bt(t, c.typ)
		for _, s := range c.valid {
			if _, err := ty.Validate(s, nil); err != nil {
				t.Errorf("%s: %q should be valid: %v", c.typ, s, err)
			}
		}
		for _, s := range c.invalid {
			if _, err := ty.Validate(s, nil); err == nil {
				t.Errorf("%s: %q should be invalid", c.typ, s)
			}
		}
	}
}

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// REQ: DT-001
func TestBuiltinHierarchy(t *testing.T) {
	names := BuiltinNames()
	if len(names) != 44 {
		t.Fatalf("%d built-in types, want 44 (19 primitive + 25 derived): %v", len(names), names)
	}
	parents := map[string]string{
		"string": "anySimpleType", "normalizedString": "string", "token": "normalizedString", "language": "token",
		"NMTOKEN": "token", "Name": "token", "NCName": "Name", "ID": "NCName", "IDREF": "NCName", "ENTITY": "NCName",
		"integer": "decimal", "nonPositiveInteger": "integer", "negativeInteger": "nonPositiveInteger",
		"long": "integer", "int": "long", "short": "int", "byte": "short", "nonNegativeInteger": "integer",
		"unsignedLong": "nonNegativeInteger", "unsignedInt": "unsignedLong", "unsignedShort": "unsignedInt",
		"unsignedByte": "unsignedShort", "positiveInteger": "nonNegativeInteger", "duration": "anySimpleType",
	}
	for n, p := range parents {
		if got := bt(t, n).Base(); got == nil || got.Name != p {
			t.Errorf("base of %s is %v, want %s", n, got, p)
		}
	}
	for _, l := range []struct{ list, item string }{{"NMTOKENS", "NMTOKEN"}, {"IDREFS", "IDREF"}, {"ENTITIES", "ENTITY"}} {
		ty := bt(t, l.list)
		if ty.Variety() != List || ty.ItemType().Name != l.item {
			t.Errorf("%s: variety %v item %v", l.list, ty.Variety(), ty.ItemType())
		}
	}
	if !bt(t, "byte").IsDerivedFrom(bt(t, "decimal")) || bt(t, "byte").IsDerivedFrom(bt(t, "string")) {
		t.Error("IsDerivedFrom")
	}
	if bt(t, "unsignedShort").Primitive() != KindDecimal || bt(t, "ID").Primitive() != KindString {
		t.Error("primitive")
	}
}

// REQ: DT-002
func TestWhiteSpace(t *testing.T) {
	in := "  a \t b\n\r c  "
	for _, c := range []struct {
		typ, want string
	}{{"string", in}, {"normalizedString", "  a   b   c  "}, {"token", "a b c"}, {"NMTOKENS", "a b c"}} {
		if got := bt(t, c.typ).Normalize(in); got != c.want {
			t.Errorf("%s: %q, want %q", c.typ, got, c.want)
		}
	}
	if v, err := bt(t, "decimal").Validate("\n  12.50 \t", nil); err != nil || Canonical(v) != "12.5" {
		t.Errorf("decimal with white space: %v %v", v, err)
	}
	// whiteSpace jest ustalony (fixed) jako collapse dla typów innych niż string.
	if _, err := Restrict(bt(t, "decimal"), []FacetSpec{{Kind: FacetWhiteSpace, Value: "preserve"}}, nil); code(err) != CodeFacetFixed {
		t.Errorf("decimal whiteSpace preserve: %v", err)
	}
	if _, err := Restrict(bt(t, "token"), []FacetSpec{{Kind: FacetWhiteSpace, Value: "replace"}}, nil); code(err) != CodeWhiteSpaceRestriction {
		t.Errorf("token whiteSpace replace: %v", err)
	}
	if _, err := Restrict(bt(t, "normalizedString"), []FacetSpec{{Kind: FacetWhiteSpace, Value: "preserve"}}, nil); code(err) != CodeWhiteSpaceRestriction {
		t.Errorf("normalizedString whiteSpace preserve: %v", err)
	}
	if _, err := Restrict(bt(t, "string"), []FacetSpec{{Kind: FacetWhiteSpace, Value: "nope"}}, nil); code(err) != CodeFacetValue {
		t.Errorf("bad whiteSpace: %v", err)
	}
}

// REQ: DT-003
func TestStringFamily(t *testing.T) {
	runLex(t, []lexCase{
		{"string", []string{"", " x ", "zażółć\n"}, nil},
		{"language", []string{"pl", "en-US", "x-klingon", "i-default"}, []string{"", "toolonglang", "pl_PL", "pl-", "-pl"}},
		{"NMTOKEN", []string{"a", "1.5", "-x:y", " abc "}, []string{"", "a b", "a!"}},
		{"NMTOKENS", []string{"a b c", " x "}, []string{"", "a !"}},
		{"Name", []string{"a", "_a", ":a", "a:b", "ł"}, []string{"1a", "-a", "a b", ""}},
		{"NCName", []string{"a", "a.b-c_d", "ś1"}, []string{"a:b", ":a", "1a"}},
		{"ID", []string{"id1"}, []string{"1id", "a:b"}},
		{"IDREFS", []string{"a b"}, []string{"", "a 1"}},
		{"ENTITIES", []string{"e1 e2"}, []string{""}},
	})
}

// REQ: DT-004
func TestBoolean(t *testing.T) {
	runLex(t, []lexCase{{"boolean", []string{"true", "false", "1", "0", " true "}, []string{"TRUE", "yes", "2", ""}}})
	v1, _ := bt(t, "boolean").Validate("1", nil)
	v2, _ := bt(t, "boolean").Validate("true", nil)
	if !Identical(v1, v2) || Canonical(v1) != "true" {
		t.Error("1 and true are the same value, canonical true")
	}
}

// REQ: DT-005
func TestDecimalAndIntegers(t *testing.T) {
	runLex(t, []lexCase{
		{"decimal", []string{"-1.23", "12678967.543233", "+100000.00", "210", ".5", "5.", "0", "-0.0",
			"123456789012345678901234567890.123456789012345678901234567890"}, []string{"", ".", "+", "1e5", "1,5", "1.2.3", "--1", "INF"}},
		{"integer", []string{"-1", "0", "12678967543233", "+100000", "007"}, []string{"1.0", "1.", "", "+"}},
		{"nonPositiveInteger", []string{"0", "-0", "+0", "-5"}, []string{"1"}},
		{"negativeInteger", []string{"-1", "-99999999999999999999"}, []string{"0", "1"}},
		{"long", []string{"9223372036854775807", "-9223372036854775808"}, []string{"9223372036854775808", "-9223372036854775809"}},
		{"int", []string{"2147483647", "-2147483648"}, []string{"2147483648"}},
		{"short", []string{"32767", "-32768"}, []string{"32768"}},
		{"byte", []string{"127", "-128", "+1"}, []string{"128", "-129"}},
		{"nonNegativeInteger", []string{"0", "-0", "+0", "5"}, []string{"-1"}},
		{"unsignedLong", []string{"18446744073709551615", "0"}, []string{"18446744073709551616", "-1"}},
		{"unsignedInt", []string{"4294967295"}, []string{"4294967296"}},
		{"unsignedShort", []string{"65535"}, []string{"65536"}},
		{"unsignedByte", []string{"255"}, []string{"256"}},
		{"positiveInteger", []string{"1", "+1"}, []string{"0", "-1"}},
	})
	d := bt(t, "decimal")
	a, _ := d.Validate("2.0", nil)
	b, _ := d.Validate("2.00", nil)
	c, _ := d.Validate("2.000001", nil)
	if !Identical(a, b) || Compare(a, c) != Less || Compare(c, a) != Greater {
		t.Error("decimal value space: 2.0 = 2.00 < 2.000001")
	}
	for in, want := range map[string]string{"+100000.00": "100000.0", "-0.0": "0.0", ".5": "0.5", "-01.230": "-1.23"} {
		v, _ := d.Validate(in, nil)
		if got := Canonical(v); got != want {
			t.Errorf("canonical %q = %q, want %q", in, got, want)
		}
	}
}

// REQ: DT-006
func TestFloatDouble(t *testing.T) {
	runLex(t, []lexCase{
		{"float", []string{"-1E4", "1267.43233E12", "12.78e-2", "12", "-0", "0", "INF", "-INF", "NaN", "1e+3", ".5e1", "1e40"},
			[]string{"", "inf", "+INF", "nan", "1e", "e5", "1.5e2.5", "0x1p3", "1_0", "Infinity"}},
		{"double", []string{"1.7976931348623157E308", "4.9E-324", "1E-400"}, []string{"1E"}},
	})
	f := bt(t, "float")
	a, _ := f.Validate("0", nil)
	b, _ := f.Validate("-0", nil)
	n1, _ := f.Validate("NaN", nil)
	n2, _ := f.Validate("NaN", nil)
	one, _ := f.Validate("1", nil)
	if !Identical(a, b) || !Identical(n1, n2) || Compare(n1, one) != Incomparable {
		t.Error("one zero, NaN = NaN, NaN incomparable")
	}
	// float zaokrągla do najbliższej wartości float32: 0.1 jako float to nie 0.1 jako double.
	fv, _ := f.Validate("0.1", nil)
	if fv.(FloatValue).F != float64(float32(0.1)) {
		t.Error("float rounding")
	}
	inf, _ := f.Validate("1e40", nil)
	if !math.IsInf(inf.(FloatValue).F, 1) {
		t.Error("1e40 as float should be INF")
	}
	// NaN wypada z typu z jakąkolwiek inną granicą (3.2.4).
	ty, err := Restrict(bt(t, "double"), []FacetSpec{{Kind: FacetMinInclusive, Value: "0"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ty.Validate("NaN", nil); code(err) != CodeMinInclusive {
		t.Errorf("NaN with minInclusive: %v", err)
	}
	for in, want := range map[string]string{"100": "1.0E2", "0.00125": "1.25E-3", "-0": "0.0E0", "INF": "INF", "1": "1.0E0"} {
		v, _ := bt(t, "double").Validate(in, nil)
		if got := Canonical(v); got != want {
			t.Errorf("canonical double %q = %q, want %q", in, got, want)
		}
	}
}

func val(t *testing.T, typ, s string) Value {
	t.Helper()
	v, err := bt(t, typ).Validate(s, nil)
	if err != nil {
		t.Fatalf("%s %q: %v", typ, s, err)
	}
	return v
}

// REQ: DT-007
func TestDuration(t *testing.T) {
	runLex(t, []lexCase{{"duration",
		[]string{"P1Y2M3DT10H30M", "-P120D", "P1347Y", "P1347M", "P1Y2MT2H", "P0Y1347M", "P0Y1347M0D", "PT1.5S", "PT0S", "P0D"},
		[]string{"", "P", "-P", "P-1347M", "P1Y2MT", "PT", "P1.5Y", "PT1.S", "PT.5S", "P1D2Y", "1Y", "P1M1M", "PT1H1D", "P1Y2M3D4H"}}})
	cmp := func(a, b string) Order { return Compare(val(t, "duration", a), val(t, "duration", b)) }
	// Tabela z 3.2.6.2.
	for _, c := range []struct {
		a, b string
		want Order
	}{
		{"P1Y", "P364D", Greater}, {"P1Y", "P365D", Incomparable}, {"P1Y", "P366D", Incomparable}, {"P1Y", "P367D", Less},
		{"P1M", "P27D", Greater}, {"P1M", "P28D", Incomparable}, {"P1M", "P31D", Incomparable}, {"P1M", "P32D", Less},
		{"P5M", "P149D", Greater}, {"P5M", "P150D", Incomparable}, {"P5M", "P153D", Incomparable}, {"P5M", "P154D", Less},
		{"P1Y", "P12M", Equal}, {"P1D", "PT24H", Equal}, {"PT1M", "PT60S", Equal}, {"-P1D", "P0D", Less},
		{"PT1.5S", "PT1.25S", Greater},
	} {
		if got := cmp(c.a, c.b); got != c.want {
			t.Errorf("%s vs %s: %v, want %v", c.a, c.b, got, c.want)
		}
	}
	// Porównanie nieokreślone przy aspektach granicznych jest fałszem (3.2.6.3).
	ty, _ := Restrict(bt(t, "duration"), []FacetSpec{{Kind: FacetMaxInclusive, Value: "P30D"}}, nil)
	if _, err := ty.Validate("P1M", nil); code(err) != CodeMaxInclusive {
		t.Errorf("P1M <= P30D is indeterminate, must fail: %v", err)
	}
	if _, err := ty.Validate("P29D", nil); err != nil {
		t.Error(err)
	}
}

// REQ: DT-008
func TestDateTimeFamily(t *testing.T) {
	runLex(t, []lexCase{
		{"dateTime", []string{"2002-10-10T12:00:00-05:00", "2002-10-10T12:00:00Z", "2002-10-10T12:00:00.5", "-0001-01-01T00:00:00",
			"12345-01-01T00:00:00", "2000-02-29T24:00:00", "2002-10-10T12:00:00+14:00"},
			[]string{"", "2002-10-10", "0000-01-01T00:00:00", "02002-01-01T00:00:00", "+2002-01-01T00:00:00", "2002-02-29T00:00:00",
				"2002-10-10T24:00:01", "2002-10-10T12:00:60", "2002-10-10T12:00:00+15:00", "2002-10-10T12:00:00+14:30",
				"2002-10-10T12:00:00.", "2002-10-10 12:00:00", "2002-1-10T12:00:00", "2002-10-10T12:00"}},
		{"time", []string{"13:20:00-05:00", "00:00:00", "24:00:00", "23:59:59.999999"}, []string{"25:00:00", "13:20", "13:20:00+5:00"}},
		{"date", []string{"2002-10-10", "2002-10-10+13:00", "-2002-10-10Z", "2000-02-29"}, []string{"2002-10-10T00:00:00", "1900-02-29", "2002-13-01"}},
		{"gYearMonth", []string{"1999-05", "1999-05Z", "-0045-01"}, []string{"1999", "1999-5", "1999-13"}},
		{"gYear", []string{"1999", "-0001", "12345", "1999+01:00"}, []string{"99", "0000", "01999", "+1999"}},
		{"gMonthDay", []string{"--09-14", "--02-29", "--12-31Z"}, []string{"--02-30", "-09-14", "09-14", "--09-14--"}},
		{"gDay", []string{"---15", "---31", "---01-05:00"}, []string{"---32", "---00", "--15"}},
		{"gMonth", []string{"--11", "--12Z"}, []string{"--13", "--11--", "-11"}},
	})
	cmp := func(typ, a, b string) Order { return Compare(val(t, typ, a), val(t, typ, b)) }
	for _, c := range []struct {
		typ, a, b string
		want      Order
	}{
		// Przykłady z 3.2.7.4.
		{"dateTime", "2000-01-15T00:00:00", "2000-02-15T00:00:00", Less},
		{"dateTime", "2000-01-15T12:00:00", "2000-01-16T12:00:00Z", Less},
		{"dateTime", "2000-01-01T12:00:00", "1999-12-31T23:00:00Z", Incomparable},
		{"dateTime", "2000-01-16T12:00:00", "2000-01-16T12:00:00Z", Incomparable},
		{"dateTime", "2000-01-16T00:00:00", "2000-01-16T12:00:00Z", Incomparable},
		{"dateTime", "2002-10-10T12:00:00-05:00", "2002-10-10T17:00:00Z", Equal},
		{"dateTime", "2000-03-04T23:00:00+03:00", "2000-03-04T20:00:00Z", Equal},
		{"dateTime", "2000-01-01T24:00:00", "2000-01-02T00:00:00", Equal},
		{"dateTime", "2000-01-01T00:00:00.5", "2000-01-01T00:00:00.50", Equal},
		// 3.2.9: 2002-10-10+13:00 i 2002-10-09-11:00 to ten sam przedział.
		{"date", "2002-10-10+13:00", "2002-10-09-11:00", Equal},
		{"date", "2002-10-10", "2002-10-11", Less},
		{"time", "13:20:00-05:00", "18:20:00Z", Equal},
		// time na stałej dacie: 12:00:00-14:00 to 02:00Z następnego dnia (zestaw W3C, time_maxInclusive007).
		{"time", "12:00:00-14:00", "12:00:00-10:00", Greater},
		{"time", "23:00:00-05:00", "04:00:00Z", Greater},
		{"gYear", "1999", "2000", Less},
		{"gMonthDay", "--02-29", "--03-01", Less},
		{"gDay", "---31", "---01", Greater},
	} {
		if got := cmp(c.typ, c.a, c.b); got != c.want {
			t.Errorf("%s %s vs %s: %v, want %v", c.typ, c.a, c.b, got, c.want)
		}
	}
	for _, c := range []struct{ typ, in, want string }{
		{"dateTime", "2002-10-10T12:00:00-05:00", "2002-10-10T17:00:00Z"},
		{"dateTime", "2002-10-10T24:00:00", "2002-10-11T00:00:00"},
		{"dateTime", "2002-10-10T12:00:00.500", "2002-10-10T12:00:00.5"},
		{"time", "24:00:00", "00:00:00"},
		{"time", "13:20:00-05:00", "18:20:00Z"},
		{"date", "2002-10-10+13:00", "2002-10-09-11:00"},
		{"date", "2002-10-10-12:00", "2002-10-11+12:00"},
		{"date", "2002-10-10+05:00", "2002-10-10+05:00"},
		{"gYear", "-0001", "-0001"},
	} {
		if got := Canonical(val(t, c.typ, c.in)); got != c.want {
			t.Errorf("canonical %s %q = %q, want %q", c.typ, c.in, got, c.want)
		}
	}
}

// REQ: DT-009
func TestBinary(t *testing.T) {
	runLex(t, []lexCase{
		{"hexBinary", []string{"", "0FB7", "0fb7", " ab "}, []string{"0", "0G", "0F B7"}},
		{"base64Binary", []string{"", "QQ==", "QUI=", "QUJD", "QUJD RA==", "Q Q = =", " QUJD "},
			[]string{"Q", "QQ=", "QR==", "QUJ=", "=QQ=", "QQ==QQ==", "QU*D", "QQ=A"}},
	})
	v := val(t, "base64Binary", "QUJD RA==")
	if string(v.(BinaryValue).Bytes) != "ABCD" {
		t.Errorf("decoded %q", v.(BinaryValue).Bytes)
	}
	ty, _ := Restrict(bt(t, "hexBinary"), []FacetSpec{{Kind: FacetLength, Value: "2"}}, nil)
	if _, err := ty.Validate("0FB7", nil); err != nil {
		t.Error("hexBinary length counts octets")
	}
	if Canonical(val(t, "hexBinary", "0fb7")) != "0FB7" {
		t.Error("hexBinary canonical is upper case")
	}
}

// REQ: DT-010
func TestAnyURI(t *testing.T) {
	runLex(t, []lexCase{{"anyURI",
		[]string{"", "http://www.w3.org/2001/XMLSchema", "../a/b?c#d", "urn:isbn:123", "mailto:a@b.pl", "a%20b", "http://zażółć.pl/ą", "#frag"},
		[]string{"a%2", "a%zz", "a#b#c", "1http://x", ":abc"}}})
	ty, _ := Restrict(bt(t, "anyURI"), []FacetSpec{{Kind: FacetMaxLength, Value: "3"}}, nil)
	if _, err := ty.Validate("ąść", nil); err != nil {
		t.Error("anyURI length counts characters")
	}
}

type nsMap map[string]string

func (m nsMap) LookupPrefix(p string) (string, bool) { ns, ok := m[p]; return ns, ok }

// REQ: DT-011
func TestQNameAndNotation(t *testing.T) {
	ctx := &Context{Namespaces: nsMap{"": "urn:default", "p": "urn:p"}}
	q := bt(t, "QName")
	v, err := q.Validate("p:local", ctx)
	if err != nil || v.(QNameValue).Namespace != "urn:p" || v.(QNameValue).Local != "local" {
		t.Fatalf("p:local: %v %v", v, err)
	}
	if v, _ := q.Validate("local", ctx); v.(QNameValue).Namespace != "urn:default" {
		t.Error("unprefixed QName takes the default namespace")
	}
	for _, bad := range []string{"x:local", "a:b:c", ":a", "1a", ""} {
		if _, err := q.Validate(bad, ctx); err == nil {
			t.Errorf("QName %q accepted", bad)
		}
	}
	// length na QName: każda wartość spełnia aspekt (4.3.1.3, punkt 1.3).
	ty, err := Restrict(q, []FacetSpec{{Kind: FacetLength, Value: "1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ty.Validate("p:longname", ctx); err != nil {
		t.Errorf("length on QName must not constrain: %v", err)
	}
	if !bt(t, "NOTATION").NeedsNotationEnumeration() {
		t.Error("NOTATION without enumeration")
	}
	n, err := Restrict(bt(t, "NOTATION"), []FacetSpec{{Kind: FacetEnumeration, Value: "p:gif"}}, ctx)
	if err != nil || n.NeedsNotationEnumeration() {
		t.Errorf("NOTATION with enumeration: %v", err)
	}
}

// REQ: DT-012
func TestLists(t *testing.T) {
	list, err := NewList(bt(t, "integer"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := list.Validate(" 8 10   12 ", nil)
	if err != nil || len(v.(ListValue)) != 3 {
		t.Fatalf("list: %v %v", v, err)
	}
	if _, err := list.Validate("8 x", nil); code(err) != CodeDatatypeList {
		t.Errorf("bad item: %v", err)
	}
	// Przykład z 2.5.1.2: wzorzec dotyczy całego literału listy.
	r, err := Restrict(list, []FacetSpec{{Kind: FacetPattern, Value: `123 (\d+\s)*456`}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for s, ok := range map[string]bool{"123 456": true, "123 987 456": true, "123 987 567 456": true, "456 123": false} {
		if _, err := r.Validate(s, nil); (err == nil) != ok {
			t.Errorf("myRestrictedList %q: %v", s, err)
		}
	}
	l3, _ := Restrict(list, []FacetSpec{{Kind: FacetLength, Value: "3"}}, nil)
	if _, err := l3.Validate("1 2", nil); code(err) != CodeLength {
		t.Errorf("list length counts items: %v", err)
	}
	// Lista typu string: elementy dzieli spacja, więc 18 elementów (2.5.1.2).
	ls, _ := NewList(bt(t, "string"))
	v, _ = ls.Validate("this is not list item 1\nthis is not list item 2\nthis is not list item 3", nil)
	if len(v.(ListValue)) != 18 {
		t.Errorf("list of string: %d items", len(v.(ListValue)))
	}
	if _, err := NewList(bt(t, "NMTOKENS")); code(err) != CodeListOfAtomic {
		t.Errorf("list of list: %v", err)
	}
	if _, err := Restrict(list, []FacetSpec{{Kind: FacetMaxInclusive, Value: "5"}}, nil); code(err) != CodeApplicableFacets {
		t.Errorf("maxInclusive on list: %v", err)
	}
	enum, err := Restrict(list, []FacetSpec{{Kind: FacetEnumeration, Value: "1 2"}, {Kind: FacetEnumeration, Value: "3"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enum.Validate("01  +2", nil); err != nil {
		t.Errorf("list enumeration compares values: %v", err)
	}
	if _, err := enum.Validate("1", nil); code(err) != CodeEnumeration {
		t.Errorf("list enumeration: %v", err)
	}
}

// REQ: DT-013
func TestUnions(t *testing.T) {
	str, _ := Restrict(bt(t, "string"), []FacetSpec{{Kind: FacetEnumeration, Value: "unbounded"}}, nil)
	u, err := NewUnion(bt(t, "nonNegativeInteger"), str)
	if err != nil {
		t.Fatal(err)
	}
	for s, ok := range map[string]bool{"5": true, "unbounded": true, " 7 ": true, "-1": false, "many": false} {
		if _, err := u.Validate(s, nil); (err == nil) != ok {
			t.Errorf("maxOccurs union %q: %v", s, err)
		}
	}
	if _, err := u.Validate("many", nil); code(err) != CodeDatatypeUnion {
		t.Errorf("union error code: %v", err)
	}
	// Kolejność składowych: "1" jest liczbą, a nie napisem (2.5.1.3).
	u2, _ := NewUnion(bt(t, "integer"), bt(t, "string"))
	if v, _ := u2.Validate("1", nil); v.Kind() != KindDecimal {
		t.Errorf("first member wins: %v", v.Kind())
	}
	// Wyliczenie na unii porównuje wartości: "01" to ta sama liczba co "1".
	e, err := Restrict(u2, []FacetSpec{{Kind: FacetEnumeration, Value: "1"}, {Kind: FacetEnumeration, Value: "x"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for s, ok := range map[string]bool{"01": true, "x": true, "2": false, "y": false} {
		if _, err := e.Validate(s, nil); (err == nil) != ok {
			t.Errorf("union enumeration %q: %v", s, err)
		}
	}
	if _, err := Restrict(u2, []FacetSpec{{Kind: FacetLength, Value: "1"}}, nil); code(err) != CodeApplicableFacets {
		t.Errorf("length on union: %v", err)
	}
}

func restrict(t *testing.T, base string, specs ...FacetSpec) *Type {
	t.Helper()
	ty, err := Restrict(bt(t, base), specs, nil)
	if err != nil {
		t.Fatalf("restrict %s: %v", base, err)
	}
	return ty
}

func fs(k FacetKind, v string) FacetSpec { return FacetSpec{Kind: k, Value: v} }

// REQ: DT-014
func TestFacetValidation(t *testing.T) {
	cases := []struct {
		ty      *Type
		valid   []string
		invalid map[string]string
	}{
		{restrict(t, "string", fs(FacetLength, "3")), []string{"abc", "żół"}, map[string]string{"ab": CodeLength}},
		{restrict(t, "string", fs(FacetMinLength, "2"), fs(FacetMaxLength, "3")), []string{"ab", "abc"},
			map[string]string{"a": CodeMinLength, "abcd": CodeMaxLength}},
		{restrict(t, "string", fs(FacetPattern, "[0-9]{2}"), fs(FacetPattern, "[a-z]")), []string{"12", "x"},
			map[string]string{"1x": CodePattern}},
		{restrict(t, "integer", fs(FacetEnumeration, "1"), fs(FacetEnumeration, "10")), []string{"1", "+10", "010"},
			map[string]string{"2": CodeEnumeration}},
		{restrict(t, "decimal", fs(FacetMinInclusive, "1.5"), fs(FacetMaxExclusive, "10")), []string{"1.5", "9.999"},
			map[string]string{"1.4": CodeMinInclusive, "10": CodeMaxExclusive}},
		{restrict(t, "decimal", fs(FacetMinExclusive, "0"), fs(FacetMaxInclusive, "1")), []string{"0.0001", "1"},
			map[string]string{"0": CodeMinExclusive, "1.01": CodeMaxInclusive}},
		{restrict(t, "decimal", fs(FacetTotalDigits, "3")), []string{"999", "0.12", "-1.23", "00012.300", "0.001"},
			map[string]string{"1000": CodeTotalDigits, "1.234": CodeTotalDigits, "0.0001": CodeTotalDigits}},
		{restrict(t, "decimal", fs(FacetFractionDigits, "2")), []string{"1.23", "1.230000", "100"},
			map[string]string{"1.234": CodeFractionDigits}},
		{restrict(t, "date", fs(FacetMaxInclusive, "2002-12-31")), []string{"2002-12-31", "1999-01-01"},
			map[string]string{"2003-01-01": CodeMaxInclusive}},
	}
	for i, c := range cases {
		for _, s := range c.valid {
			if _, err := c.ty.Validate(s, nil); err != nil {
				t.Errorf("case %d: %q: %v", i, s, err)
			}
		}
		for s, want := range c.invalid {
			if _, err := c.ty.Validate(s, nil); code(err) != want {
				t.Errorf("case %d: %q: %v, want %s", i, s, err, want)
			}
		}
	}
	// Wzorce z różnych kroków łączy „i” (4.3.4.3).
	two := restrict(t, "string", fs(FacetPattern, "[a-z]+"))
	two, _ = Restrict(two, []FacetSpec{fs(FacetPattern, ".{3}")}, nil)
	if _, err := two.Validate("abcd", nil); code(err) != CodePattern {
		t.Error("patterns from different steps are ANDed")
	}
	if _, err := two.Validate("abc", nil); err != nil {
		t.Error(err)
	}
	// Wzorzec sprawdzamy na tekście po normalizacji białych znaków.
	tok := restrict(t, "token", fs(FacetPattern, "a b"))
	if _, err := tok.Validate("  a   b ", nil); err != nil {
		t.Errorf("pattern after collapse: %v", err)
	}
}

// REQ: DT-015
func TestApplicableFacets(t *testing.T) {
	for _, c := range []struct {
		base string
		f    FacetKind
		ok   bool
	}{
		{"string", FacetLength, true}, {"string", FacetMaxInclusive, false}, {"string", FacetTotalDigits, false},
		{"boolean", FacetPattern, true}, {"boolean", FacetEnumeration, false}, {"boolean", FacetLength, false},
		{"float", FacetMinExclusive, true}, {"float", FacetTotalDigits, false},
		{"decimal", FacetTotalDigits, true}, {"integer", FacetMaxLength, false},
		{"dateTime", FacetMaxInclusive, true}, {"duration", FacetFractionDigits, false},
		{"hexBinary", FacetMaxLength, true}, {"anyURI", FacetMinLength, true}, {"QName", FacetLength, true},
		{"gYear", FacetLength, false},
	} {
		_, err := Restrict(bt(t, c.base), []FacetSpec{fs(c.f, "1")}, nil)
		if bad := code(err) == CodeApplicableFacets; bad == c.ok {
			t.Errorf("%s on %s: %v", c.f, c.base, err)
		}
	}
	if _, err := Restrict(AnySimpleType(), []FacetSpec{fs(FacetPattern, "a")}, nil); code(err) != CodeApplicableFacets {
		t.Errorf("facet on anySimpleType: %v", err)
	}
}

// REQ: DT-016
func TestRestrictionConstraints(t *testing.T) {
	cases := []struct {
		base  string
		steps [][]FacetSpec
		want  string
	}{
		{"string", [][]FacetSpec{{fs(FacetMinLength, "5"), fs(FacetMaxLength, "3")}}, CodeMinLengthMaxLength},
		{"string", [][]FacetSpec{{fs(FacetLength, "5"), fs(FacetMinLength, "3")}}, CodeLengthMinMax},
		{"string", [][]FacetSpec{{fs(FacetLength, "5")}, {fs(FacetLength, "6")}}, CodeLengthRestriction},
		{"string", [][]FacetSpec{{fs(FacetMinLength, "5")}, {fs(FacetMinLength, "4")}}, CodeMinLengthRestriction},
		{"string", [][]FacetSpec{{fs(FacetMaxLength, "5")}, {fs(FacetMaxLength, "6")}}, CodeMaxLengthRestriction},
		{"string", [][]FacetSpec{{fs(FacetLength, "1"), fs(FacetLength, "1")}}, CodeSingleFacetValue},
		{"integer", [][]FacetSpec{{fs(FacetEnumeration, "1.5")}}, CodeEnumerationRestriction},
		{"byte", [][]FacetSpec{{fs(FacetEnumeration, "200")}}, CodeEnumerationRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMinInclusive, "5"), fs(FacetMaxInclusive, "4")}}, CodeMinInclusiveMaxIncl},
		{"integer", [][]FacetSpec{{fs(FacetMaxInclusive, "5"), fs(FacetMaxExclusive, "6")}}, CodeMaxInclusiveMaxExcl},
		{"integer", [][]FacetSpec{{fs(FacetMinInclusive, "5"), fs(FacetMinExclusive, "4")}}, CodeMinInclusiveMinExcl},
		{"integer", [][]FacetSpec{{fs(FacetMinExclusive, "5"), fs(FacetMaxInclusive, "5")}}, CodeMinExclusiveMaxIncl},
		{"integer", [][]FacetSpec{{fs(FacetMinInclusive, "5"), fs(FacetMaxExclusive, "5")}}, CodeMinInclusiveMaxExclStep},
		{"integer", [][]FacetSpec{{fs(FacetMinExclusive, "6"), fs(FacetMaxExclusive, "5")}}, CodeMinExclusiveMaxExcl},
		{"integer", [][]FacetSpec{{fs(FacetMaxInclusive, "10")}, {fs(FacetMaxInclusive, "11")}}, CodeMaxInclusiveRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMaxExclusive, "10")}, {fs(FacetMaxInclusive, "10")}}, CodeMaxInclusiveRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMaxExclusive, "10")}, {fs(FacetMaxExclusive, "11")}}, CodeMaxExclusiveRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMinInclusive, "10")}, {fs(FacetMinInclusive, "9")}}, CodeMinInclusiveRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMinExclusive, "10")}, {fs(FacetMinExclusive, "9")}}, CodeMinExclusiveRestriction},
		{"integer", [][]FacetSpec{{fs(FacetMinExclusive, "10")}, {fs(FacetMinInclusive, "10")}}, CodeMinInclusiveRestriction},
		{"byte", [][]FacetSpec{{fs(FacetMaxInclusive, "200")}}, CodeMaxInclusiveRestriction},
		{"decimal", [][]FacetSpec{{fs(FacetTotalDigits, "5")}, {fs(FacetTotalDigits, "6")}}, CodeTotalDigitsRestriction},
		{"decimal", [][]FacetSpec{{fs(FacetFractionDigits, "2")}, {fs(FacetFractionDigits, "3")}}, CodeFractionRestriction},
		{"decimal", [][]FacetSpec{{fs(FacetTotalDigits, "2"), fs(FacetFractionDigits, "3")}}, CodeFractionTotal},
		{"integer", [][]FacetSpec{{fs(FacetFractionDigits, "1")}}, CodeFacetFixed},
		{"string", [][]FacetSpec{{{Kind: FacetMaxLength, Value: "5", Fixed: true}}, {fs(FacetMaxLength, "4")}}, CodeFacetFixed},
		{"string", [][]FacetSpec{{fs(FacetLength, "-1")}}, CodeFacetValue},
		{"decimal", [][]FacetSpec{{fs(FacetTotalDigits, "0")}}, CodeFacetValue},
		{"string", [][]FacetSpec{{fs(FacetPattern, "[a")}}, "regex-syntax"},
		{"integer", [][]FacetSpec{{fs(FacetMaxInclusive, "abc")}}, CodeFacetValue},
	}
	for i, c := range cases {
		ty := bt(t, c.base)
		var err error
		for _, step := range c.steps {
			if ty, err = Restrict(ty, step, nil); err != nil {
				break
			}
		}
		if code(err) != c.want {
			t.Errorf("case %d (%s): %v, want %s", i, c.base, err, c.want)
		}
	}
	// Dozwolone: maxExclusive równy maxExclusive bazy; minLength z kroku bez length obok length.
	ok := [][][]FacetSpec{
		{{fs(FacetMaxExclusive, "10")}, {fs(FacetMaxExclusive, "10")}},
		{{fs(FacetMinLength, "2")}, {fs(FacetLength, "3")}},
		{{fs(FacetMaxInclusive, "10")}, {fs(FacetMaxExclusive, "10")}},
		{{fs(FacetMinInclusive, "1"), fs(FacetMaxInclusive, "1")}},
		{{{Kind: FacetMaxLength, Value: "5", Fixed: true}}, {fs(FacetMaxLength, "5")}},
	}
	for i, steps := range ok {
		base := "integer"
		if i == 1 || i == 4 {
			base = "string"
		}
		ty := bt(t, base)
		for _, step := range steps {
			var err error
			if ty, err = Restrict(ty, step, nil); err != nil {
				t.Errorf("allowed case %d: %v", i, err)
				break
			}
		}
	}
}

// REQ: DT-017
func TestErrorCodes(t *testing.T) {
	_, err := bt(t, "int").Validate("abc", nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeDatatypeAtomic || e.Value != "abc" || !strings.Contains(e.Error(), "cvc-datatype-valid.1.2.1") {
		t.Errorf("atomic error: %#v", err)
	}
	_, err = bt(t, "int").Validate("3000000000", nil)
	if !errors.As(err, &e) || e.Code != CodeMaxInclusive || e.Facet != "maxInclusive" {
		t.Errorf("bound error: %#v", err)
	}
}

// REQ: DT-018
func TestImplementationLimits(t *testing.T) {
	if _, err := bt(t, "gYear").Validate("999999999999", nil); err != nil {
		t.Errorf("12-digit year: %v", err)
	}
	if _, err := bt(t, "gYear").Validate("1000000000000", nil); err == nil {
		t.Error("13-digit year accepted beyond the documented limit")
	}
	if _, err := bt(t, "duration").Validate("P999999999999999D", nil); err != nil {
		t.Errorf("15-digit duration field: %v", err)
	}
	if _, err := bt(t, "duration").Validate("P1000000000000000D", nil); err == nil {
		t.Error("16-digit duration field accepted beyond the documented limit")
	}
	if _, err := bt(t, "decimal").Validate(strings.Repeat("9", 500)+"."+strings.Repeat("1", 500), nil); err != nil {
		t.Errorf("long decimal: %v", err)
	}
	if _, err := bt(t, "dateTime").Validate("2000-01-01T00:00:00."+strings.Repeat("1", 100), nil); err != nil {
		t.Errorf("long fraction: %v", err)
	}
}

func FuzzLexical(f *testing.F) {
	for _, s := range []string{"1.5", "-0", "1E5", "INF", "P1Y2M", "2002-10-10T12:00:00-05:00", "--02-29", "QUJD", "0FB7", "true"} {
		f.Add(s)
	}
	types := []string{"decimal", "integer", "double", "float", "duration", "dateTime", "date", "time", "gYear",
		"gMonthDay", "base64Binary", "hexBinary", "boolean", "anyURI"}
	f.Fuzz(func(t *testing.T, s string) {
		for _, name := range types {
			ty := Builtin(name)
			v, err := ty.Validate(s, nil)
			if err != nil {
				continue
			}
			// Postać kanoniczna należy do przestrzeni leksykalnej i oznacza tę samą wartość.
			c := ty.Canonical(v)
			w, err := ty.Validate(c, nil)
			if err != nil {
				t.Fatalf("%s: %q -> canonical %q invalid: %v", name, s, c, err)
			}
			if !Identical(v, w) && !(name == "float" || name == "double") {
				t.Fatalf("%s: %q -> %q is a different value", name, s, c)
			}
			if ty.Canonical(w) != c {
				t.Fatalf("%s: canonical not stable: %q -> %q", name, c, ty.Canonical(w))
			}
		}
	})
}
