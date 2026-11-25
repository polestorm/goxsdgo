// Package datatypes realizuje typy proste XML Schema 1.0 Second Edition, część 2 (Datatypes): 44 typy
// wbudowane, typy pochodne przez restrykcję, listę i unię, wszystkie aspekty ograniczające z ich regułami
// walidacji i ograniczeniami poprawności przy restrykcji.
//
// Typ buduje się z typu bazowego i listy aspektów (Restrict), z typu elementu (List) albo z typów
// składowych (Union) — tak, jak etap 3 zbuduje go z komponentów schematu. Validate sprawdza tekst według
// reguły Datatype Valid (4.1.4) i zwraca wartość z przestrzeni wartości. Pakiet służy bibliotece wewnętrznie.
package datatypes

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/polestorm/goxsdgo/internal/regex"
	"github.com/polestorm/goxsdgo/internal/xmlchars"
)

// Variety to odmiana typu prostego (2.5.1).
type Variety uint8

const (
	Atomic Variety = iota
	List
	Union
)

// WhiteSpace to wartość aspektu whiteSpace (4.3.6).
type WhiteSpace uint8

const (
	Preserve WhiteSpace = iota
	Replace
	Collapse
)

var whiteSpaceNames = [...]string{"preserve", "replace", "collapse"}

func (w WhiteSpace) String() string { return whiteSpaceNames[w] }

// FacetKind to rodzaj aspektu ograniczającego (4.3).
type FacetKind uint8

const (
	FacetLength FacetKind = iota
	FacetMinLength
	FacetMaxLength
	FacetPattern
	FacetEnumeration
	FacetWhiteSpace
	FacetMaxInclusive
	FacetMaxExclusive
	FacetMinExclusive
	FacetMinInclusive
	FacetTotalDigits
	FacetFractionDigits
	numFacets
)

var facetNames = [...]string{"length", "minLength", "maxLength", "pattern", "enumeration", "whiteSpace",
	"maxInclusive", "maxExclusive", "minExclusive", "minInclusive", "totalDigits", "fractionDigits"}

func (f FacetKind) String() string { return facetNames[f] }

// FacetByName zwraca rodzaj aspektu dla nazwy elementu schematu (np. "maxLength").
func FacetByName(name string) (FacetKind, bool) {
	for i, n := range facetNames {
		if n == name {
			return FacetKind(i), true
		}
	}
	return 0, false
}

type facetSet uint16

func (s facetSet) has(f FacetKind) bool { return s&(1<<f) != 0 }

func setOf(fs ...FacetKind) facetSet {
	var s facetSet
	for _, f := range fs {
		s |= 1 << f
	}
	return s
}

// FacetSpec to jeden aspekt z kroku restrykcji, tak jak w dokumencie schematu: literał wartości i atrybut fixed.
type FacetSpec struct {
	Kind  FacetKind
	Value string
	Fixed bool
}

// Type to definicja typu prostego.
type Type struct {
	// Name to nazwa typu wbudowanego (np. "decimal"); dla typów ze schematu pusta, nazwę zna etap 3.
	Name      string
	variety   Variety
	primitive Kind
	base      *Type
	item      *Type
	members   []*Type
	builtin   bool
	f         facets
}

// facets to aspekty obowiązujące typ: z ostatniego kroku i odziedziczone z typów bazowych.
type facets struct {
	present facetSet // aspekty należące do {facets}
	fixed   facetSet

	whiteSpace                   WhiteSpace
	length, minLength, maxLength uint64
	// minLengthAlone i maxLengthAlone: aspekt pochodzi z kroku, w którym typ nie miał length (warunek 1.2
	// i 2.2 reguły length-minLength-maxLength).
	minLengthAlone, maxLengthAlone bool
	// patterns: każdy krok restrykcji to jedna grupa; w grupie wzorce łączy „lub”, grupy łączy „i” (4.3.4.3).
	patterns [][]*regex.Regexp
	// lexicalGroups: pierwsze grupy wzorców pochodzą z typów wbudowanych (dodatek A) i definiują przestrzeń
	// leksykalną; ich naruszenie zgłaszamy jako cvc-datatype-valid.1.2.1, nie cvc-pattern-valid.
	lexicalGroups                  int
	enumeration                    []Value
	enumLexical                    []string
	minInc, minExc, maxInc, maxExc Value
	totalDigits, fractionDigits    uint64
}

func (f facets) clone() facets {
	g := f
	g.patterns = append([][]*regex.Regexp(nil), f.patterns...)
	g.enumeration = append([]Value(nil), f.enumeration...)
	g.enumLexical = append([]string(nil), f.enumLexical...)
	return g
}

// Variety zwraca odmianę typu.
func (t *Type) Variety() Variety { return t.variety }

// Primitive zwraca typ pierwotny typu atomowego (KindList dla list, KindAnySimpleType dla unii
// i anySimpleType).
func (t *Type) Primitive() Kind { return t.primitive }

// Base zwraca typ bazowy (nil dla anySimpleType).
func (t *Type) Base() *Type { return t.base }

// ItemType zwraca typ elementu listy.
func (t *Type) ItemType() *Type { return t.item }

// MemberTypes zwraca typy składowe unii.
func (t *Type) MemberTypes() []*Type { return t.members }

// IsBuiltin mówi, czy typ jest typem wbudowanym.
func (t *Type) IsBuiltin() bool { return t.builtin }

// WhiteSpace zwraca obowiązującą normalizację białych znaków. Dla unii nie ma ona zastosowania (4.3.6):
// normalizuje typ składowy, według którego tekst przechodzi walidację.
func (t *Type) WhiteSpace() WhiteSpace { return t.f.whiteSpace }

// HasFacet mówi, czy aspekt należy do {facets} typu.
func (t *Type) HasFacet(k FacetKind) bool { return t.f.present.has(k) }

// Enumeration zwraca literały wartości aspektu enumeration (nil, gdy go nie ma).
func (t *Type) Enumeration() []string { return t.f.enumLexical }

// NeedsNotationEnumeration mówi, czy typ jest NOTATION albo pochodzi od niego bez aspektu enumeration.
// Takiego typu nie wolno użyć w schemacie wprost (3.2.19, reguła enumeration-required-notation); sprawdza to
// etap 3 przy deklaracjach.
func (t *Type) NeedsNotationEnumeration() bool {
	return t.variety == Atomic && t.primitive == KindNOTATION && !t.f.present.has(FacetEnumeration)
}

// IsDerivedFrom mówi, czy t to b albo typ pochodzący od b przez łańcuch typów bazowych.
func (t *Type) IsDerivedFrom(b *Type) bool {
	for x := t; x != nil; x = x.base {
		if x == b {
			return true
		}
	}
	return false
}

// applicable zwraca aspekty dozwolone dla typu bazowego (reguła cos-applicable-facets, 4.1.5).
func applicable(base *Type) facetSet {
	switch base.variety {
	case List:
		return setOf(FacetLength, FacetMinLength, FacetMaxLength, FacetPattern, FacetEnumeration, FacetWhiteSpace)
	case Union:
		return setOf(FacetPattern, FacetEnumeration)
	}
	ordered := setOf(FacetPattern, FacetEnumeration, FacetWhiteSpace, FacetMaxInclusive, FacetMaxExclusive,
		FacetMinInclusive, FacetMinExclusive)
	lengths := setOf(FacetLength, FacetMinLength, FacetMaxLength, FacetPattern, FacetEnumeration, FacetWhiteSpace)
	switch base.primitive {
	case KindString, KindHexBinary, KindBase64Binary, KindAnyURI, KindQName, KindNOTATION:
		return lengths
	case KindBoolean:
		return setOf(FacetPattern, FacetWhiteSpace)
	case KindDecimal:
		return ordered | setOf(FacetTotalDigits, FacetFractionDigits)
	case KindFloat, KindDouble, KindDuration, KindDateTime, KindTime, KindDate, KindGYearMonth, KindGYear,
		KindGMonthDay, KindGDay, KindGMonth:
		return ordered
	}
	return 0 // anySimpleType
}

// Restrict tworzy typ pochodny przez restrykcję base aspektami specs z jednego kroku (2.5.2.1, 4.1.2.1).
// Sprawdza ograniczenia poprawności aspektów (4.1.3, 4.1.5, 4.3.x.4); błąd ma kod reguły. ctx rozwiązuje
// prefiksy w wartościach aspektów typów QName i NOTATION.
func Restrict(base *Type, specs []FacetSpec, ctx *Context) (*Type, error) {
	t := &Type{variety: base.variety, primitive: base.primitive, base: base, item: base.item, members: base.members}
	t.f = base.f.clone()
	allowed := applicable(base)
	var step facetSet
	var stepPatterns []*regex.Regexp
	var enumValues []Value
	var enumLexical []string
	for _, s := range specs {
		if !allowed.has(s.Kind) {
			return nil, errorf(CodeApplicableFacets, s.Value, s.Kind.String(), "facet not applicable to the base type")
		}
		if step.has(s.Kind) && s.Kind != FacetPattern && s.Kind != FacetEnumeration {
			return nil, errorf(CodeSingleFacetValue, s.Value, s.Kind.String(), "facet specified twice in one step")
		}
		step |= 1 << s.Kind
		if s.Fixed {
			t.f.fixed |= 1 << s.Kind
		}
	}
	for _, s := range specs {
		switch s.Kind {
		case FacetPattern:
			re, err := regex.Compile(s.Value)
			if err != nil {
				code := regex.CodeSyntax
				if e, ok := err.(*regex.Error); ok {
					code = e.Code
				}
				return nil, &Error{Code: code, Value: s.Value, Facet: "pattern", Cause: err}
			}
			stepPatterns = append(stepPatterns, re)
		case FacetEnumeration:
			v, err := base.Validate(s.Value, ctx)
			if err != nil {
				return nil, &Error{Code: CodeEnumerationRestriction, Value: s.Value, Facet: "enumeration",
					Detail: "value is not in the value space of the base type", Cause: err}
			}
			enumValues = append(enumValues, v)
			enumLexical = append(enumLexical, s.Value)
		case FacetWhiteSpace:
			w, ok := parseWhiteSpace(s.Value)
			if !ok {
				return nil, errorf(CodeFacetValue, s.Value, "whiteSpace", "must be preserve, replace or collapse")
			}
			if base.f.present.has(FacetWhiteSpace) {
				if base.f.fixed.has(FacetWhiteSpace) && w != base.f.whiteSpace {
					return nil, errorf(CodeFacetFixed, s.Value, "whiteSpace", "base type fixes whiteSpace to %s", base.f.whiteSpace)
				}
				if w < base.f.whiteSpace {
					return nil, errorf(CodeWhiteSpaceRestriction, s.Value, "whiteSpace", "base type has %s", base.f.whiteSpace)
				}
			}
			t.f.whiteSpace = w
		case FacetLength, FacetMinLength, FacetMaxLength, FacetTotalDigits, FacetFractionDigits:
			n, ok := parseCount(s.Value, s.Kind == FacetTotalDigits)
			if !ok {
				what := "a nonNegativeInteger"
				if s.Kind == FacetTotalDigits {
					what = "a positiveInteger"
				}
				return nil, errorf(CodeFacetValue, s.Value, s.Kind.String(), "must be %s", what)
			}
			if err := t.f.setCount(base, s.Kind, n, s.Value); err != nil {
				return nil, err
			}
		case FacetMinInclusive, FacetMinExclusive, FacetMaxInclusive, FacetMaxExclusive:
			v, err := boundValue(base, s, namesOf(ctx))
			if err != nil {
				return nil, err
			}
			if base.f.fixed.has(s.Kind) {
				if Compare(v, *base.f.bound(s.Kind)) != Equal {
					return nil, errorf(CodeFacetFixed, s.Value, s.Kind.String(), "base type fixes %s", s.Kind)
				}
			}
			*t.f.bound(s.Kind) = v
		}
	}
	if len(stepPatterns) > 0 {
		t.f.patterns = append(t.f.patterns, stepPatterns)
	}
	if step.has(FacetEnumeration) {
		t.f.enumeration, t.f.enumLexical = enumValues, enumLexical
	}
	t.f.present |= step
	if err := t.checkLengths(base, step); err != nil {
		return nil, err
	}
	if err := t.checkBounds(base, step); err != nil {
		return nil, err
	}
	if t.f.present.has(FacetFractionDigits) && t.f.present.has(FacetTotalDigits) && t.f.fractionDigits > t.f.totalDigits {
		return nil, errorf(CodeFractionTotal, fmt.Sprint(t.f.fractionDigits), "fractionDigits", "totalDigits is %d", t.f.totalDigits)
	}
	return t, nil
}

func parseWhiteSpace(s string) (WhiteSpace, bool) {
	switch collapse(s) {
	case "preserve":
		return Preserve, true
	case "replace":
		return Replace, true
	case "collapse":
		return Collapse, true
	}
	return 0, false
}

// parseCount czyta wartość aspektu typu nonNegativeInteger (albo positiveInteger). Wartości większe niż
// mieści uint64 zapisujemy jako MaxUint64: żaden tekst w pamięci nie jest dłuższy.
func parseCount(s string, positive bool) (uint64, bool) {
	d, ok := parseDecimal(collapse(s))
	if !ok || !d.IsInteger() || strings.Contains(s, ".") || d.Sign() < 0 || (positive && d.Sign() == 0) {
		return 0, false
	}
	u, ok := d.uint64Value()
	if !ok {
		return math.MaxUint64, true
	}
	return u, true
}

func (f *facets) count(k FacetKind) uint64 {
	switch k {
	case FacetLength:
		return f.length
	case FacetMinLength:
		return f.minLength
	case FacetMaxLength:
		return f.maxLength
	case FacetTotalDigits:
		return f.totalDigits
	}
	return f.fractionDigits
}

// setCount ustawia aspekt liczbowy i sprawdza reguły restrykcji względem typu bazowego.
func (f *facets) setCount(base *Type, k FacetKind, n uint64, lexical string) *Error {
	if base.f.present.has(k) {
		old := base.f.count(k)
		if base.f.fixed.has(k) && n != old {
			return errorf(CodeFacetFixed, lexical, k.String(), "base type fixes %s to %d", k, old)
		}
		switch k {
		case FacetLength:
			if n != old {
				return errorf(CodeLengthRestriction, lexical, "length", "base type has length %d", old)
			}
		case FacetMinLength:
			if n < old {
				return errorf(CodeMinLengthRestriction, lexical, "minLength", "base type has minLength %d", old)
			}
		case FacetMaxLength:
			if n > old {
				return errorf(CodeMaxLengthRestriction, lexical, "maxLength", "base type has maxLength %d", old)
			}
		case FacetTotalDigits:
			if n > old {
				return errorf(CodeTotalDigitsRestriction, lexical, "totalDigits", "base type has totalDigits %d", old)
			}
		case FacetFractionDigits:
			if n > old {
				return errorf(CodeFractionRestriction, lexical, "fractionDigits", "base type has fractionDigits %d", old)
			}
		}
	}
	switch k {
	case FacetLength:
		f.length = n
	case FacetMinLength:
		f.minLength = n
		f.minLengthAlone = !f.present.has(FacetLength)
	case FacetMaxLength:
		f.maxLength = n
		f.maxLengthAlone = !f.present.has(FacetLength)
	case FacetTotalDigits:
		f.totalDigits = n
	case FacetFractionDigits:
		f.fractionDigits = n
	}
	return nil
}

// checkLengths sprawdza length-minLength-maxLength i minLength-less-than-equal-to-maxLength (4.3.1.4, 4.3.2.4).
func (t *Type) checkLengths(base *Type, step facetSet) *Error {
	f := &t.f
	if f.present.has(FacetLength) {
		// minLength (maxLength) może współistnieć z length tylko wtedy, gdy nie przeczy mu (1.1, 2.1) i istnieje
		// typ-przodek, w którym minLength (maxLength) ma tę samą wartość, a length nie było (1.2, 2.2). Wartość może
		// być powtórzona w bieżącym kroku: tekst 1.2 wymaga tylko takiego przodka (np. restrykcja xsd:IDREFS, który
		// ma minLength 1, z length 5 i minLength 1 w jednym kroku jest poprawna: MS IDREFS_length006).
		if f.present.has(FacetMinLength) && !(f.minLength <= f.length && base != nil &&
			base.f.present.has(FacetMinLength) && base.f.minLength == f.minLength && base.f.minLengthAlone) {
			return errorf(CodeLengthMinMax, fmt.Sprint(f.length), "length", "minLength %d", f.minLength)
		}
		if f.present.has(FacetMaxLength) && !(f.length <= f.maxLength && base != nil &&
			base.f.present.has(FacetMaxLength) && base.f.maxLength == f.maxLength && base.f.maxLengthAlone) {
			return errorf(CodeLengthMinMax, fmt.Sprint(f.length), "length", "maxLength %d", f.maxLength)
		}
	}
	if f.present.has(FacetMinLength) && f.present.has(FacetMaxLength) && f.minLength > f.maxLength {
		return errorf(CodeMinLengthMaxLength, fmt.Sprint(f.minLength), "minLength", "maxLength %d", f.maxLength)
	}
	return nil
}

func (f *facets) bound(k FacetKind) *Value {
	switch k {
	case FacetMinInclusive:
		return &f.minInc
	case FacetMinExclusive:
		return &f.minExc
	case FacetMaxInclusive:
		return &f.maxInc
	}
	return &f.maxExc
}

// boundValue czyta wartość aspektu granicznego: musi należeć do przestrzeni wartości typu bazowego, a dla
// maxExclusive i minExclusive może też równać się tej samej granicy typu bazowego (4.3.8, 4.3.9).
func boundValue(base *Type, s FacetSpec, ed xmlchars.Edition) (Value, *Error) {
	lexical := normalize(s.Value, base.f.whiteSpace)
	v, ok := parsePrimitive(base.primitive, lexical, nil)
	if !ok {
		return nil, errorf(CodeFacetValue, s.Value, s.Kind.String(), "not a valid %s literal", base.primitive)
	}
	if s.Kind == FacetMaxExclusive || s.Kind == FacetMinExclusive {
		if base.f.present.has(s.Kind) && Compare(v, *base.f.bound(s.Kind)) == Equal {
			return v, nil
		}
	}
	// Pozostałe granice typu bazowego sprawdzają reguły *-valid-restriction w checkBounds; tu tylko wzorce,
	// wyliczenie i cyfry.
	if err := base.checkPatterns(lexical, ed); err != nil {
		return nil, errorf(boundRestrictionCode(s.Kind), s.Value, s.Kind.String(), "not in the value space of the base type")
	}
	if err := base.checkValueFacets(v, lexical, false); err != nil {
		return nil, errorf(boundRestrictionCode(s.Kind), s.Value, s.Kind.String(), "not in the value space of the base type")
	}
	return v, nil
}

func boundRestrictionCode(k FacetKind) string {
	switch k {
	case FacetMinInclusive:
		return CodeMinInclusiveRestriction
	case FacetMinExclusive:
		return CodeMinExclusiveRestriction
	case FacetMaxInclusive:
		return CodeMaxInclusiveRestriction
	}
	return CodeMaxExclusiveRestriction
}

// checkBounds sprawdza reguły granic: w jednym typie (4.3.7.4–4.3.10.4) i względem typu bazowego
// (*-valid-restriction). Wynik Incomparable (np. data ze strefą i bez) nie jest błędem: reguła mówi
// o wartości „większej” albo „mniejszej”, a tej relacji wtedy nie ma.
func (t *Type) checkBounds(base *Type, step facetSet) *Error {
	f, bf := &t.f, &base.f
	gt := func(a, b Value) bool { return Compare(a, b) == Greater }
	ge := func(a, b Value) bool { o := Compare(a, b); return o == Greater || o == Equal }
	lexOf := func(v Value) string { return Canonical(v) }
	if step.has(FacetMaxInclusive) && step.has(FacetMaxExclusive) {
		return errorf(CodeMaxInclusiveMaxExcl, lexOf(f.maxInc), "maxInclusive", "maxExclusive in the same step")
	}
	if step.has(FacetMinInclusive) && step.has(FacetMinExclusive) {
		return errorf(CodeMinInclusiveMinExcl, lexOf(f.minInc), "minInclusive", "minExclusive in the same step")
	}
	has := f.present.has
	if has(FacetMinInclusive) && has(FacetMaxInclusive) && gt(f.minInc, f.maxInc) {
		return errorf(CodeMinInclusiveMaxIncl, lexOf(f.minInc), "minInclusive", "maxInclusive %s", lexOf(f.maxInc))
	}
	if has(FacetMinExclusive) && has(FacetMaxExclusive) && gt(f.minExc, f.maxExc) {
		return errorf(CodeMinExclusiveMaxExcl, lexOf(f.minExc), "minExclusive", "maxExclusive %s", lexOf(f.maxExc))
	}
	if has(FacetMinExclusive) && has(FacetMaxInclusive) && ge(f.minExc, f.maxInc) {
		return errorf(CodeMinExclusiveMaxIncl, lexOf(f.minExc), "minExclusive", "maxInclusive %s", lexOf(f.maxInc))
	}
	if has(FacetMinInclusive) && has(FacetMaxExclusive) && ge(f.minInc, f.maxExc) {
		return errorf(CodeMinInclusiveMaxExclStep, lexOf(f.minInc), "minInclusive", "maxExclusive %s", lexOf(f.maxExc))
	}
	bh := bf.present.has
	type rule struct {
		ok   bool
		what string
	}
	check := func(k FacetKind, v Value, code string, rules []rule) *Error {
		for _, r := range rules {
			if !r.ok {
				return errorf(code, lexOf(v), k.String(), "base type has %s", r.what)
			}
		}
		return nil
	}
	if step.has(FacetMaxInclusive) {
		v := f.maxInc
		if err := check(FacetMaxInclusive, v, CodeMaxInclusiveRestriction, []rule{
			{!bh(FacetMaxInclusive) || !gt(v, bf.maxInc), "maxInclusive " + lexOf(bf.maxInc)},
			{!bh(FacetMaxExclusive) || !ge(v, bf.maxExc), "maxExclusive " + lexOf(bf.maxExc)},
			{!bh(FacetMinInclusive) || !gt(bf.minInc, v), "minInclusive " + lexOf(bf.minInc)},
			{!bh(FacetMinExclusive) || !ge(bf.minExc, v), "minExclusive " + lexOf(bf.minExc)},
		}); err != nil {
			return err
		}
	}
	if step.has(FacetMaxExclusive) {
		v := f.maxExc
		if err := check(FacetMaxExclusive, v, CodeMaxExclusiveRestriction, []rule{
			{!bh(FacetMaxExclusive) || !gt(v, bf.maxExc), "maxExclusive " + lexOf(bf.maxExc)},
			{!bh(FacetMaxInclusive) || !gt(v, bf.maxInc), "maxInclusive " + lexOf(bf.maxInc)},
			{!bh(FacetMinInclusive) || !ge(bf.minInc, v), "minInclusive " + lexOf(bf.minInc)},
			{!bh(FacetMinExclusive) || !ge(bf.minExc, v), "minExclusive " + lexOf(bf.minExc)},
		}); err != nil {
			return err
		}
	}
	if step.has(FacetMinExclusive) {
		v := f.minExc
		if err := check(FacetMinExclusive, v, CodeMinExclusiveRestriction, []rule{
			{!bh(FacetMinExclusive) || !gt(bf.minExc, v), "minExclusive " + lexOf(bf.minExc)},
			{!bh(FacetMaxInclusive) || !gt(v, bf.maxInc), "maxInclusive " + lexOf(bf.maxInc)},
			{!bh(FacetMinInclusive) || !gt(bf.minInc, v), "minInclusive " + lexOf(bf.minInc)},
			{!bh(FacetMaxExclusive) || !ge(v, bf.maxExc), "maxExclusive " + lexOf(bf.maxExc)},
		}); err != nil {
			return err
		}
	}
	if step.has(FacetMinInclusive) {
		v := f.minInc
		if err := check(FacetMinInclusive, v, CodeMinInclusiveRestriction, []rule{
			{!bh(FacetMinInclusive) || !gt(bf.minInc, v), "minInclusive " + lexOf(bf.minInc)},
			{!bh(FacetMaxInclusive) || !gt(v, bf.maxInc), "maxInclusive " + lexOf(bf.maxInc)},
			{!bh(FacetMinExclusive) || !ge(bf.minExc, v), "minExclusive " + lexOf(bf.minExc)},
			{!bh(FacetMaxExclusive) || !ge(v, bf.maxExc), "maxExclusive " + lexOf(bf.maxExc)},
		}); err != nil {
			return err
		}
	}
	return nil
}

// NewList tworzy typ listowy z typem elementu item (2.5.1.2, 4.1.2.2). Typ elementu musi być atomowy albo
// być unią (cos-list-of-atomic).
func NewList(item *Type) (*Type, error) {
	if item.variety == List {
		return nil, errorf(CodeListOfAtomic, item.Name, "", "item type of a list must be atomic or a union")
	}
	return &Type{variety: List, primitive: KindList, base: anySimpleType, item: item,
		f: facets{present: setOf(FacetWhiteSpace), fixed: setOf(FacetWhiteSpace), whiteSpace: Collapse}}, nil
}

// NewUnion tworzy unię typów members (2.5.1.3, 4.1.2.3). Kolejność typów składowych decyduje o tym, według
// którego typu tekst przechodzi walidację.
func NewUnion(members ...*Type) (*Type, error) {
	if len(members) == 0 {
		return nil, errorf(CodeFacetValue, "", "", "union needs member types")
	}
	return &Type{variety: Union, primitive: KindAnySimpleType, base: anySimpleType, members: members}, nil
}

// Validate sprawdza tekst według Datatype Valid (4.1.4): normalizacja białych znaków, wzorce, przestrzeń
// leksykalna, a potem aspekty w przestrzeni wartości. Zwraca wartość albo *Error.
func (t *Type) Validate(s string, ctx *Context) (Value, error) {
	v, _, err := t.validate(s, ctx)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Canonical zwraca postać kanoniczną wartości v tego typu. Różni się od funkcji Canonical tylko dla typów
// pochodnych od integer, które mają własną postać kanoniczną bez kropki (3.3.13.2).
func (t *Type) Canonical(v Value) string {
	if d, ok := v.(*Decimal); ok && t.IsDerivedFrom(builtins["integer"]) {
		return d.integerString()
	}
	if l, ok := v.(ListValue); ok && t.variety == List {
		parts := make([]string, len(l))
		for i, item := range l {
			parts[i] = t.item.Canonical(item)
		}
		return strings.Join(parts, " ")
	}
	return Canonical(v)
}

// Normalize stosuje normalizację białych znaków typu (dla unii: bez zmian).
func (t *Type) Normalize(s string) string {
	if t.variety == Union {
		return s
	}
	return normalize(s, t.f.whiteSpace)
}

func (t *Type) validate(s string, ctx *Context) (Value, string, *Error) {
	switch t.variety {
	case List:
		n := collapse(s)
		if err := t.checkPatterns(n, namesOf(ctx)); err != nil {
			return nil, n, err
		}
		var items []string
		if n != "" {
			items = strings.Split(n, " ")
		}
		list := make(ListValue, len(items))
		for i, it := range items {
			v, _, err := t.item.validate(it, ctx)
			if err != nil {
				return nil, n, &Error{Code: CodeDatatypeList, Value: n, Detail: fmt.Sprintf("item %d", i+1), Cause: err}
			}
			list[i] = v
		}
		if err := t.checkValueFacets(list, n, true); err != nil {
			return nil, n, err
		}
		return list, n, nil
	case Union:
		for _, m := range t.members {
			v, n, err := m.validate(s, ctx)
			if err != nil {
				continue
			}
			if err := t.checkPatterns(n, namesOf(ctx)); err != nil {
				return nil, n, err
			}
			if err := t.checkValueFacets(v, n, true); err != nil {
				return nil, n, err
			}
			return v, n, nil
		}
		return nil, s, &Error{Code: CodeDatatypeUnion, Value: s, Detail: "matches no member type"}
	}
	n := normalize(s, t.f.whiteSpace)
	if err := t.checkPatterns(n, namesOf(ctx)); err != nil {
		return nil, n, err
	}
	v, ok := parsePrimitive(t.primitive, n, ctx)
	if !ok {
		return nil, n, &Error{Code: CodeDatatypeAtomic, Value: n, Detail: "not a valid " + t.describe()}
	}
	if err := t.checkValueFacets(v, n, true); err != nil {
		return nil, n, err
	}
	return v, n, nil
}

func (t *Type) describe() string {
	for x := t; x != nil; x = x.base {
		if x.builtin {
			return x.Name
		}
	}
	return t.primitive.String()
}

func (t *Type) checkPatterns(n string, ed xmlchars.Edition) *Error {
	for gi, group := range t.f.patterns {
		ok := false
		for _, re := range group {
			if re.MatchStringNames(n, ed) {
				ok = true
				break
			}
		}
		if !ok && gi < t.f.lexicalGroups {
			return &Error{Code: CodeDatatypeAtomic, Value: n, Detail: "not a valid " + t.describe()}
		}
		if !ok {
			srcs := make([]string, len(group))
			for i, re := range group {
				srcs[i] = re.String()
			}
			return &Error{Code: CodePattern, Value: n, Facet: "pattern", Detail: strings.Join(srcs, " | ")}
		}
	}
	return nil
}

// checkValueFacets sprawdza aspekty w przestrzeni wartości (Facet Valid, 4.1.4); withEnum == false pomija
// enumeration i granice (przy sprawdzaniu wartości aspektu granicznego wobec typu bazowego).
func (t *Type) checkValueFacets(v Value, lexical string, withBounds bool) *Error {
	f := &t.f
	has := f.present.has
	if has(FacetLength) || has(FacetMinLength) || has(FacetMaxLength) {
		n, measured := t.lengthOf(v, lexical)
		if measured {
			if has(FacetLength) && n != f.length {
				return errorf(CodeLength, lexical, "length", "length %d, required %d", n, f.length)
			}
			if has(FacetMinLength) && n < f.minLength {
				return errorf(CodeMinLength, lexical, "minLength", "length %d, minimum %d", n, f.minLength)
			}
			if has(FacetMaxLength) && n > f.maxLength {
				return errorf(CodeMaxLength, lexical, "maxLength", "length %d, maximum %d", n, f.maxLength)
			}
		}
	}
	if has(FacetTotalDigits) || has(FacetFractionDigits) {
		if d, ok := v.(*Decimal); ok {
			total, frac := d.digitCounts()
			if has(FacetTotalDigits) && (uint64(total) > f.totalDigits || uint64(frac) > f.totalDigits) {
				return errorf(CodeTotalDigits, lexical, "totalDigits", "%d digits, maximum %d", total, f.totalDigits)
			}
			if has(FacetFractionDigits) && uint64(frac) > f.fractionDigits {
				return errorf(CodeFractionDigits, lexical, "fractionDigits", "%d fraction digits, maximum %d", frac, f.fractionDigits)
			}
		}
	}
	if !withBounds {
		return nil
	}
	if has(FacetEnumeration) {
		found := false
		for _, e := range f.enumeration {
			if Identical(v, e) {
				found = true
				break
			}
		}
		if !found {
			return errorf(CodeEnumeration, lexical, "enumeration", "allowed: %s", strings.Join(f.enumLexical, ", "))
		}
	}
	bound := func(k FacetKind, b Value, ok func(Order) bool, code string) *Error {
		if has(k) && !ok(Compare(v, b)) {
			return errorf(code, lexical, k.String(), "%s %s", k, t.Canonical(b))
		}
		return nil
	}
	if err := bound(FacetMinInclusive, f.minInc, func(o Order) bool { return o == Greater || o == Equal }, CodeMinInclusive); err != nil {
		return err
	}
	if err := bound(FacetMinExclusive, f.minExc, func(o Order) bool { return o == Greater }, CodeMinExclusive); err != nil {
		return err
	}
	if err := bound(FacetMaxInclusive, f.maxInc, func(o Order) bool { return o == Less || o == Equal }, CodeMaxInclusive); err != nil {
		return err
	}
	if err := bound(FacetMaxExclusive, f.maxExc, func(o Order) bool { return o == Less }, CodeMaxExclusive); err != nil {
		return err
	}
	return nil
}

// lengthOf mierzy długość wartości w jednostkach aspektu length (4.3.1.3): znaki dla string i anyURI, oktety
// dla typów binarnych, elementy dla list. Dla QName i NOTATION każda wartość spełnia aspekt (measured false).
func (t *Type) lengthOf(v Value, lexical string) (uint64, bool) {
	switch x := v.(type) {
	case ListValue:
		return uint64(len(x)), true
	case BinaryValue:
		return uint64(len(x.Bytes)), true
	case StringValue:
		return uint64(utf8.RuneCountInString(x.S)), true
	}
	return 0, false
}

// normalize stosuje whiteSpace (4.3.6): replace zamienia #x9, #xA i #xD na spację, collapse dodatkowo
// skleja ciągi spacji i obcina je na brzegach.
func normalize(s string, w WhiteSpace) string {
	switch w {
	case Replace:
		return replaceWS(s)
	case Collapse:
		return collapse(s)
	}
	return s
}

func replaceWS(s string) string {
	if !strings.ContainsAny(s, "\t\n\r") {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}

func collapse(s string) string {
	clean := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' || c == '\n' || c == '\r' || (c == ' ' && (i == 0 || i == len(s)-1 || s[i+1] == ' ')) {
			clean = false
			break
		}
	}
	if clean {
		return s // bez alokacji w typowym przypadku
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}), " ")
}
