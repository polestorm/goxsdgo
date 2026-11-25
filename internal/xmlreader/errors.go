package xmlreader

import "fmt"

// Kody błędów czytnika. Są stałe: wywołujący tłumaczy komunikat po kodzie. Kody "wfc-*" i "nsc-*" to nazwy
// ograniczeń dobrej formy z XML 1.0 i ograniczeń przestrzeni nazw z Namespaces in XML 1.0; "xml-*" to naruszenia
// gramatyki i innych reguł rekomendacji; "ns-*" to reguły przestrzeni nazw spoza listy NSC; "limit-*" to limity
// czytnika (ADR 0004); "unsupported-*" to konstrukcje poprawne, których czytnik świadomie nie wykonuje.
const (
	CodeEncodingInvalid     = "xml-encoding-invalid"     // bajty niezgodne z kodowaniem (4.3.3)
	CodeEncodingUnsupported = "xml-encoding-unsupported" // kodowanie spoza obsługiwanych (4.3.3, ADR 0004)
	CodeEncodingMismatch    = "xml-encoding-mismatch"    // deklaracja kodowania niezgodna z wykrytym (4.3.3)
	CodeChar                = "xml-char"                 // znak spoza [2] Char
	CodeSyntax              = "xml-syntax"               // naruszenie produkcji gramatyki
	CodeEOF                 = "xml-eof"                  // dokument kończy się w środku konstrukcji
	CodeName                = "xml-name"                 // nazwa niezgodna z [5] Name
	CodeNoRoot              = "xml-no-root"              // brak elementu głównego ([1] document)
	CodeOutsideRoot         = "xml-content-outside-root" // treść inna niż Misc poza elementem głównym ([1], [27])
	CodeCDATAEnd            = "xml-cdata-end"            // "]]>" w danych znakowych ([14] CharData)
	CodeComment             = "xml-comment"              // "--" w komentarzu ([15] Comment)
	CodeReservedPITarget    = "xml-reserved-pi-target"   // cel PI pasujący do [Xx][Mm][Ll] ([17] PITarget)
	CodeXMLDecl             = "xml-decl"                 // błędna deklaracja XML ([23]–[26], [32], [80], [81])
	CodeEntityStructure     = "xml-entity-structure"     // tekst encji nie jest poprawną treścią (4.3.2)
	CodeElementTypeMatch    = "wfc-element-type-match"
	CodeUniqueAttSpec       = "wfc-unique-att-spec"
	CodeNoExternalEntityRef = "wfc-no-external-entity-references"
	CodeNoLtInAttValue      = "wfc-no-lt-in-attribute-values"
	CodeLegalCharacter      = "wfc-legal-character"
	CodeEntityDeclared      = "wfc-entity-declared"
	CodeParsedEntity        = "wfc-parsed-entity"
	CodeNoRecursion         = "wfc-no-recursion"
	CodePEsInInternalSubset = "wfc-pes-in-internal-subset"
	CodePrefixDeclared      = "nsc-prefix-declared"
	CodeNoPrefixUndeclaring = "nsc-no-prefix-undeclaring"
	CodeAttributesUnique    = "nsc-attributes-unique"
	CodeReservedPrefixes    = "nsc-reserved-prefixes"
	CodeQName               = "ns-qname" // nazwa niezgodna z QName albo dwukropek w nazwie encji, PI, notacji (Namespaces 7)
	CodeLimitDepth          = "limit-depth"
	CodeLimitNameLength     = "limit-name-length"
	CodeLimitAttributes     = "limit-attributes"
	CodeLimitTextSize       = "limit-text-size"
	CodeLimitEntityExpand   = "limit-entity-expansion"
	CodeLimitEntityDepth    = "limit-entity-depth"
	CodeLimitDTDSize        = "limit-dtd-size"
	CodeExternalEntity      = "unsupported-external-entity"   // odwołanie do encji zewnętrznej w treści: nigdy nie czytamy
	CodeEntityUnresolved    = "unsupported-entity-unresolved" // encja mogła być zadeklarowana w nieczytanym podzbiorze zewnętrznym
	CodeConditionalSection  = "unsupported-conditional-section"
)

// Error to błąd krytyczny (fatal error, rozdział 1.2 XML 1.0): po nim czytnik nie podaje dalszych tokenów.
type Error struct {
	Code string
	// Line i Column wskazują znak, przy którym wykryto błąd; liczone od 1, kolumna w znakach (ADR 0004).
	// Błąd w tekście zastępczym encji wskazuje miejsce odwołania do niej w dokumencie.
	Line, Column int
	Message      string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
