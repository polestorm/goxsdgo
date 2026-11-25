package schema

import (
	"errors"
	"fmt"
	"strings"
)

// Kody błędów schematu. Nazwy to identyfikatory reguł z rekomendacji (część 1: Schema Representation Constraints
// i Schema Component Constraints), z numerem punktu tam, gdzie reguła go ma. Błędy definicji typów prostych mają
// kody pakietu datatypes (część 2), a błędy wzorców kody pakietu regex.
const (
	// Dokument schematu: dobra forma i schemat dla schematów (dodatek A). Rekomendacja nie nazywa tych reguł
	// osobno („conditions imposed by the schema for schemas”), kody są nasze, z przedrostkiem sfs-.
	CodeNotWellFormed    = "sfs-not-well-formed"    // dokument schematu nie jest poprawnym XML z przestrzeniami nazw
	CodeNotSchema        = "sfs-not-schema"         // element główny nie jest xs:schema
	CodeContent          = "sfs-content"            // dzieci elementu niezgodne z modelem treści ze schematu dla schematów
	CodeText             = "sfs-text"               // tekst w elemencie o treści wyłącznie elementowej
	CodeAttribute        = "sfs-attribute"          // atrybut niedozwolony na tym elemencie
	CodeAttributeMissing = "sfs-attribute-required" // brak wymaganego atrybutu
	CodeAttributeValue   = "sfs-attribute-value"    // wartość atrybutu spoza jego typu
	CodeDuplicateID      = "sfs-duplicate-id"       // dwa atrybuty typu ID o tej samej wartości w dokumencie

	CodeQName    = "src-qname"   // prefiks wartości QName bez deklaracji przestrzeni nazw (3.15.3)
	CodeResolve  = "src-resolve" // QName nie wskazuje komponentu wymaganego rodzaju (3.15.3)
	CodeResolve4 = "src-resolve.4"

	CodeInclude  = "src-include"
	CodeImport   = "src-import"
	CodeRedefine = "src-redefine"

	CodeAttributeRep      = "src-attribute"
	CodeElementRep        = "src-element"
	CodeComplexTypeRep    = "src-ct"
	CodeSimpleTypeRep     = "src-simple-type"
	CodeAttributeGroupRep = "src-attribute_group"
	CodeModelGroupRep     = "src-model_group"
	CodeIdentityRep       = "src-identity-constraint"

	CodeAttributeProps      = "a-props-correct"
	CodeNoXMLNS             = "no-xmlns"
	CodeNoXSI               = "no-xsi"
	CodeElementProps        = "e-props-correct"
	CodeValidDefault        = "cos-valid-default"
	CodeComplexTypeProps    = "ct-props-correct"
	CodeExtends             = "cos-ct-extends"
	CodeRestriction         = "derivation-ok-restriction"
	CodeAttributeUseProps   = "au-props-correct"
	CodeAttributeGroupProps = "ag-props-correct"
	CodeModelGroupProps     = "mg-props-correct"
	CodeAllLimited          = "cos-all-limited"
	CodeParticleProps       = "p-props-correct"
	CodeWildcardUnion       = "cos-aw-union"
	CodeWildcardIntersect   = "cos-aw-intersect"
	CodeIdentityProps       = "c-props-correct"
	CodeSelectorXPath       = "c-selector-xpath"
	CodeFieldsXPath         = "c-fields-xpaths"
	CodeSimpleTypeProps     = "st-props-correct"
	CodeSimpleRestricts     = "cos-st-restricts"
	CodeSchemaProps         = "sch-props-correct"
	CodeNotationProps       = "n-props-correct"
	// Ograniczenia modeli zawartości (etap 4, ADR 0007).
	CodeNonambig          = "cos-nonambig"
	CodeElementConsistent = "cos-element-consistent"
	CodeParticleRestrict  = "cos-particle-restrict"

	// CodeLimit: przekroczony limit wczytywania (liczba dokumentów, głębokość włączeń); kod nasz (ADR 0006).
	CodeLimit = "schema-limit"
)

// Error to błąd schematu: kod reguły, miejsce w dokumencie schematu i komunikat po angielsku.
type Error struct {
	Code    string
	Pos     Position
	Message string
	// Cause to błąd zagnieżdżony (np. z pakietu datatypes albo regex), gdy jest.
	Cause error
}

func (e *Error) Error() string {
	var b strings.Builder
	if e.Pos.Document != "" {
		b.WriteString(e.Pos.Document)
		if e.Pos.Line > 0 {
			fmt.Fprintf(&b, ":%d:%d", e.Pos.Line, e.Pos.Column)
		}
		b.WriteString(": ")
	}
	b.WriteString(e.Code)
	b.WriteString(": ")
	b.WriteString(e.Message)
	return b.String()
}

func (e *Error) Unwrap() error { return e.Cause }

// ErrorList to wszystkie błędy schematu w kolejności wykrycia. Kolejność jest powtarzalna (dokumenty i elementy
// w kolejności z dokumentów).
type ErrorList []*Error

func (l ErrorList) Error() string {
	switch len(l) {
	case 0:
		return "schema: no errors"
	case 1:
		return l[0].Error()
	}
	return fmt.Sprintf("%s (and %d more)", l[0].Error(), len(l)-1)
}

// errUnsupported oznacza konstrukcję spoza obsługiwanego zakresu; Load zwraca ją opakowaną, nigdy jako werdykt.
var errUnsupported = fmt.Errorf("schema: %w", errors.ErrUnsupported)
