package validate

import (
	"errors"
	"fmt"
)

// Kody błędów dokumentu. Nazwy to identyfikatory reguł oceny z części 1 rekomendacji (Validation Rules, dodatek
// C.1), z numerem punktu. Błędy wartości typów prostych mają kody pakietu datatypes (cvc-datatype-valid.*,
// cvc-pattern-valid, cvc-length-valid, …), błędy dobrej formy kody czytnika (wfc-*, nsc-*, xml-*).
const (
	CodeEltDecl        = "cvc-elt.1"     // brak deklaracji elementu przy ocenie ścisłej (korzeń, symbol strict)
	CodeEltAbstract    = "cvc-elt.2"     // deklaracja abstrakcyjna
	CodeEltNotNillable = "cvc-elt.3.1"   // xsi:nil przy deklaracji bez nillable
	CodeEltNilContent  = "cvc-elt.3.2.1" // element z xsi:nil="true" ma treść
	CodeEltNilFixed    = "cvc-elt.3.2.2" // xsi:nil="true" przy wartości stałej
	CodeEltTypeQName   = "cvc-elt.4.1"   // xsi:type nie jest poprawnym QName
	CodeEltTypeResolve = "cvc-elt.4.2"   // xsi:type nie wskazuje typu
	CodeEltTypeDerived = "cvc-elt.4.3"   // typ z xsi:type nie jest poprawnie wyprowadzony z typu deklaracji
	CodeEltDefault     = "cvc-elt.5.1.1" // wartość domyślna niepoprawna dla typu z xsi:type
	CodeEltFixedChild  = "cvc-elt.5.2.2.1"
	CodeEltFixedMixed  = "cvc-elt.5.2.2.2.1"
	CodeEltFixedValue  = "cvc-elt.5.2.2.2.2"

	CodeTypeAbstract     = "cvc-type.2"     // typ abstrakcyjny
	CodeTypeSimpleAttrs  = "cvc-type.3.1.1" // element o typie prostym ma atrybuty
	CodeTypeSimpleChild  = "cvc-type.3.1.2" // element o typie prostym ma elementy dzieci
	CodeComplexEmpty     = "cvc-complex-type.2.1"
	CodeComplexSimple    = "cvc-complex-type.2.2"
	CodeComplexText      = "cvc-complex-type.2.3"
	CodeComplexContent   = "cvc-complex-type.2.4"
	CodeComplexAttr      = "cvc-complex-type.3.2.1" // atrybut niezadeklarowany i bez symbolu wieloznacznego
	CodeComplexAttrWild  = "cvc-complex-type.3.2.2" // atrybut spoza przestrzeni symbolu wieloznacznego
	CodeComplexRequired  = "cvc-complex-type.4"
	CodeComplexWildIDs   = "cvc-complex-type.5.1"
	CodeComplexWildIDUse = "cvc-complex-type.5.2"
	CodeAttrDecl         = "cvc-attribute.1" // brak deklaracji atrybutu przy symbolu strict
	CodeAttrFixed        = "cvc-attribute.4"
	CodeAUFixed          = "cvc-au"

	CodeIDUnresolved = "cvc-id.1"
	CodeIDDuplicate  = "cvc-id.2"
	// CodeEntity: wartość typu ENTITY albo ENTITIES nie jest nazwą zadeklarowanej encji nieparsowanej (część 2,
	// 3.3.11 i 3.3.12). Rekomendacja nie nazywa osobno tej reguły; kod jest nasz.
	CodeEntity = "cvc-entity"
	// CodeNotation: wartość typu NOTATION nie jest nazwą notacji ze schematu (część 2, 3.2.19). Kod nasz.
	CodeNotation = "cvc-notation"

	CodeICField     = "cvc-identity-constraint.3"
	CodeICUnique    = "cvc-identity-constraint.4.1"
	CodeICKeyMiss   = "cvc-identity-constraint.4.2.1"
	CodeICKeyDup    = "cvc-identity-constraint.4.2.2"
	CodeICKeyNil    = "cvc-identity-constraint.4.2.3"
	CodeICKeyRef    = "cvc-identity-constraint.4.3"
	CodeSchemaHints = "schema-location-hint" // schemat wskazany przez xsi:schemaLocation nie jest poprawny (kod nasz)
)

// ErrLimit oznacza przekroczenie limitu walidatora albo czytnika (głębokość, rozmiar tekstu, liczba wpisów
// ograniczeń tożsamości, MaxConfigurations). To ograniczenie biblioteki, nie werdykt o dokumencie.
var ErrLimit = errors.New("validation limit exceeded")

// ErrTooManyErrors oznacza, że walidację przerwano po MaxErrors błędach; zebrane błędy są zwracane razem z nim.
var ErrTooManyErrors = errors.New("too many errors, validation stopped")

// Error to jeden błąd dokumentu.
type Error struct {
	Code         string
	Line, Column int
	// Path to ścieżka elementu (ADR 0008): nazwy lokalne bez prefiksów, [n] od drugiego wystąpienia nazwy wśród
	// rodzeństwa, "/@nazwa" dla atrybutu.
	Path string
	// Name to nazwa lokalna elementu albo atrybutu, którego dotyczy błąd.
	Name string
	// Value to wartość, której dotyczy błąd (np. tekst niezgodny z typem); Facet to naruszony aspekt i jego
	// wartość (np. "maxLength 5").
	Value, Facet string
	// Expected to elementy dozwolone w tym miejscu, dla cvc-complex-type.2.4: "nazwa" albo "{przestrzeń}nazwa",
	// symbol wieloznaczny jako "*" albo "{przestrzeń}*".
	Expected []string
	Message  string

	// Namespace to przestrzeń nazw elementu albo atrybutu Name (pusta dla nazwy bez przestrzeni).
	Namespace string
	// Reason rozróżnia błędy cvc-complex-type.2.4 (ReasonMissing, ReasonUnexpected, ReasonWrongNamespace); dla
	// innych kodów jest pusty. Kod reguły się nie zmienia (ADR 0010).
	Reason string
	// ExpectedNamespace to przestrzeń nazw, w której schemat oczekuje elementu o tej samej nazwie lokalnej
	// (tylko ReasonWrongNamespace).
	ExpectedNamespace string
	// Type i TypeNamespace to typ ze schematu, któremu nie odpowiada wartość (błędy typów prostych); Type jest
	// pusty dla typu anonimowego. Builtin to najbliższy typ wbudowany, z którego typ pochodzi (np. "date").
	Type, TypeNamespace, Builtin string
	// InsertLine i InsertColumn wskazują miejsce, w którym powinien zacząć się oczekiwany element (błędy
	// cvc-complex-type.2.4): przed niedozwolonym elementem albo przed znacznikiem końcowym rodzica. Zero, gdy
	// miejsca nie ma.
	InsertLine, InsertColumn int
}

// Wartości Error.Reason dla cvc-complex-type.2.4.
const (
	// ReasonMissing: zawartość elementu się skończyła, a brakuje wymaganego dziecka (Expected).
	ReasonMissing = "missing"
	// ReasonUnexpected: dziecko Name nie jest dozwolone w tym miejscu.
	ReasonUnexpected = "unexpected"
	// ReasonWrongNamespace: dziecko ma nazwę lokalną oczekiwanego elementu, ale inną przestrzeń nazw.
	ReasonWrongNamespace = "wrong-namespace"
)

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
