package datatypes

import "sort"

// anySimpleType to prosta definicja ur-typu (4.1.6): bez aspektów, z nieograniczoną przestrzenią leksykalną.
var anySimpleType = &Type{Name: "anySimpleType", variety: Atomic, primitive: KindAnySimpleType, builtin: true}

var builtins = map[string]*Type{"anySimpleType": anySimpleType}

// Builtin zwraca typ wbudowany o nazwie lokalnej name z przestrzeni http://www.w3.org/2001/XMLSchema
// (np. "decimal", "NMTOKENS"), albo nil.
func Builtin(name string) *Type { return builtins[name] }

// AnySimpleType zwraca definicję anySimpleType.
func AnySimpleType() *Type { return anySimpleType }

// BuiltinNames zwraca nazwy wszystkich typów wbudowanych, posortowane (bez anySimpleType: 44 nazwy).
func BuiltinNames() []string {
	var names []string
	for n := range builtins {
		if n != "anySimpleType" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func init() {
	// Typy pierwotne (3.2, dodatek A): string z whiteSpace preserve, pozostałe z collapse ustalonym (fixed).
	primitives := []struct {
		name string
		kind Kind
	}{
		{"string", KindString}, {"boolean", KindBoolean}, {"decimal", KindDecimal}, {"float", KindFloat},
		{"double", KindDouble}, {"duration", KindDuration}, {"dateTime", KindDateTime}, {"time", KindTime},
		{"date", KindDate}, {"gYearMonth", KindGYearMonth}, {"gYear", KindGYear}, {"gMonthDay", KindGMonthDay},
		{"gDay", KindGDay}, {"gMonth", KindGMonth}, {"hexBinary", KindHexBinary},
		{"base64Binary", KindBase64Binary}, {"anyURI", KindAnyURI}, {"QName", KindQName},
		{"NOTATION", KindNOTATION},
	}
	for _, p := range primitives {
		t := &Type{Name: p.name, variety: Atomic, primitive: p.kind, base: anySimpleType, builtin: true}
		t.f.present = setOf(FacetWhiteSpace)
		if p.kind == KindString {
			t.f.whiteSpace = Preserve
		} else {
			t.f.whiteSpace = Collapse
			t.f.fixed = setOf(FacetWhiteSpace)
		}
		builtins[p.name] = t
	}

	// Typy pochodne (3.3) w kolejności z dodatku A: nazwa, typ bazowy, aspekty.
	derived := []struct {
		name, base string
		facets     []FacetSpec
	}{
		{"normalizedString", "string", []FacetSpec{{Kind: FacetWhiteSpace, Value: "replace"}}},
		{"token", "normalizedString", []FacetSpec{{Kind: FacetWhiteSpace, Value: "collapse"}}},
		{"language", "token", []FacetSpec{{Kind: FacetPattern, Value: "[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*"}}},
		{"NMTOKEN", "token", []FacetSpec{{Kind: FacetPattern, Value: `\c+`}}},
		{"Name", "token", []FacetSpec{{Kind: FacetPattern, Value: `\i\c*`}}},
		{"NCName", "Name", []FacetSpec{{Kind: FacetPattern, Value: `[\i-[:]][\c-[:]]*`}}},
		{"ID", "NCName", nil},
		{"IDREF", "NCName", nil},
		{"ENTITY", "NCName", nil},
		{"integer", "decimal", []FacetSpec{{Kind: FacetFractionDigits, Value: "0", Fixed: true},
			{Kind: FacetPattern, Value: `[\-+]?[0-9]+`}}},
		{"nonPositiveInteger", "integer", []FacetSpec{{Kind: FacetMaxInclusive, Value: "0"}}},
		{"negativeInteger", "nonPositiveInteger", []FacetSpec{{Kind: FacetMaxInclusive, Value: "-1"}}},
		{"long", "integer", []FacetSpec{{Kind: FacetMinInclusive, Value: "-9223372036854775808"},
			{Kind: FacetMaxInclusive, Value: "9223372036854775807"}}},
		{"int", "long", []FacetSpec{{Kind: FacetMinInclusive, Value: "-2147483648"},
			{Kind: FacetMaxInclusive, Value: "2147483647"}}},
		{"short", "int", []FacetSpec{{Kind: FacetMinInclusive, Value: "-32768"}, {Kind: FacetMaxInclusive, Value: "32767"}}},
		{"byte", "short", []FacetSpec{{Kind: FacetMinInclusive, Value: "-128"}, {Kind: FacetMaxInclusive, Value: "127"}}},
		{"nonNegativeInteger", "integer", []FacetSpec{{Kind: FacetMinInclusive, Value: "0"}}},
		{"unsignedLong", "nonNegativeInteger", []FacetSpec{{Kind: FacetMaxInclusive, Value: "18446744073709551615"}}},
		{"unsignedInt", "unsignedLong", []FacetSpec{{Kind: FacetMaxInclusive, Value: "4294967295"}}},
		{"unsignedShort", "unsignedInt", []FacetSpec{{Kind: FacetMaxInclusive, Value: "65535"}}},
		{"unsignedByte", "unsignedShort", []FacetSpec{{Kind: FacetMaxInclusive, Value: "255"}}},
		{"positiveInteger", "nonNegativeInteger", []FacetSpec{{Kind: FacetMinInclusive, Value: "1"}}},
	}
	for _, d := range derived {
		t, err := Restrict(builtins[d.base], d.facets, nil)
		if err != nil {
			panic("datatypes: built-in " + d.name + ": " + err.Error())
		}
		t.Name, t.builtin = d.name, true
		t.f.lexicalGroups = len(t.f.patterns)
		builtins[d.name] = t
	}

	// Listy wbudowane: lista typu elementu z minLength 1 (dodatek A).
	for _, l := range []struct{ name, item string }{{"NMTOKENS", "NMTOKEN"}, {"IDREFS", "IDREF"}, {"ENTITIES", "ENTITY"}} {
		list, err := NewList(builtins[l.item])
		if err != nil {
			panic(err)
		}
		t, err := Restrict(list, []FacetSpec{{Kind: FacetMinLength, Value: "1"}}, nil)
		if err != nil {
			panic("datatypes: built-in " + l.name + ": " + err.Error())
		}
		t.Name, t.builtin = l.name, true
		builtins[l.name] = t
	}
}
