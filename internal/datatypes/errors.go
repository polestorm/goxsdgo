package datatypes

import "fmt"

// Kody błędów walidacji tekstu (część 2, 4.1.4 i 4.3.x: Validation Rules). Nazwy to identyfikatory reguł
// z rekomendacji, z numerem punktu tam, gdzie reguła ma punkty.
const (
	CodeDatatypeAtomic = "cvc-datatype-valid.1.2.1" // tekst spoza przestrzeni leksykalnej typu atomowego
	CodeDatatypeList   = "cvc-datatype-valid.1.2.2" // element listy spoza przestrzeni leksykalnej typu elementu
	CodeDatatypeUnion  = "cvc-datatype-valid.1.2.3" // tekst nie pasuje do żadnego typu składowego unii
	CodePattern        = "cvc-pattern-valid"
	CodeEnumeration    = "cvc-enumeration-valid"
	CodeLength         = "cvc-length-valid"
	CodeMinLength      = "cvc-minLength-valid"
	CodeMaxLength      = "cvc-maxLength-valid"
	CodeMinInclusive   = "cvc-minInclusive-valid"
	CodeMaxInclusive   = "cvc-maxInclusive-valid"
	CodeMinExclusive   = "cvc-minExclusive-valid"
	CodeMaxExclusive   = "cvc-maxExclusive-valid"
	CodeTotalDigits    = "cvc-totalDigits-valid"
	CodeFractionDigits = "cvc-fractionDigits-valid"
)

// Kody błędów definicji typu (część 2, 4.1.3, 4.1.5 i 4.3.x: Schema Component Constraints i Schema
// Representation Constraints). Etap 3 zgłasza je jako błędy schematu.
const (
	CodeApplicableFacets        = "cos-applicable-facets"
	CodeListOfAtomic            = "cos-list-of-atomic"
	CodeSingleFacetValue        = "src-single-facet-value"
	CodeLengthMinMax            = "length-minLength-maxLength"
	CodeLengthRestriction       = "length-valid-restriction"
	CodeMinLengthMaxLength      = "minLength-less-than-equal-to-maxLength"
	CodeMinLengthRestriction    = "minLength-valid-restriction"
	CodeMaxLengthRestriction    = "maxLength-valid-restriction"
	CodeEnumerationRestriction  = "enumeration-valid-restriction"
	CodeWhiteSpaceRestriction   = "whiteSpace-valid-restriction"
	CodeMinInclusiveMaxIncl     = "minInclusive-less-than-equal-to-maxInclusive"
	CodeMaxInclusiveRestriction = "maxInclusive-valid-restriction"
	CodeMaxInclusiveMaxExcl     = "maxInclusive-maxExclusive"
	CodeMinExclusiveMaxExcl     = "minExclusive-less-than-equal-to-maxExclusive"
	CodeMaxExclusiveRestriction = "maxExclusive-valid-restriction"
	CodeMinInclusiveMinExcl     = "minInclusive-minExclusive"
	CodeMinExclusiveMaxIncl     = "minExclusive-less-than-maxInclusive"
	CodeMinExclusiveRestriction = "minExclusive-valid-restriction"
	CodeMinInclusiveMaxExclStep = "minInclusive-less-than-maxExclusive"
	CodeMinInclusiveRestriction = "minInclusive-valid-restriction"
	CodeTotalDigitsRestriction  = "totalDigits-valid-restriction"
	CodeFractionTotal           = "fractionDigits-totalDigits"
	CodeFractionRestriction     = "fractionDigits-valid-restriction"
	CodeNotationEnumeration     = "enumeration-required-notation"
	// CodeFacetFixed: aspekt z {fixed} = true w typie bazowym nie może dostać innej wartości (część 2, opis
	// składnika każdego aspektu, np. 4.3.1.1). Rekomendacja nie nazywa tej reguły; kod jest nasz.
	CodeFacetFixed = "facet-fixed"
	// CodeFacetValue: wartość aspektu nie jest poprawnym literałem wymaganego typu (np. length="-1",
	// whiteSpace="none"); część 2, 4.3.x.2. Kod nasz.
	CodeFacetValue = "facet-value"
)

// Error to błąd walidacji tekstu albo definicji typu.
type Error struct {
	// Code to stały kod reguły (stałe Code* powyżej, albo kody pakietu regex dla błędnego wzorca).
	Code string
	// Value to sprawdzany tekst albo wartość aspektu.
	Value string
	// Facet to nazwa aspektu, którego dotyczy błąd, albo "".
	Facet string
	// Detail to wartość aspektu albo inne wyjaśnienie (np. "maxLength 5").
	Detail string
	// Cause to błąd zagnieżdżony, np. błąd elementu listy albo błąd składni wzorca.
	Cause error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("%s: %q", e.Code, e.Value)
	if e.Facet != "" {
		msg += " (" + e.Facet + ")"
	}
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Cause }

func errorf(code, value, facet, format string, args ...any) *Error {
	return &Error{Code: code, Value: value, Facet: facet, Detail: fmt.Sprintf(format, args...)}
}
