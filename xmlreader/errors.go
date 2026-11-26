package xmlreader

import "fmt"

// Error codes of the reader (Error.Code). They do not change, so a caller can translate messages by code.
// "wfc-*" and "nsc-*" are the names of the well-formedness constraints of XML 1.0 and the namespace constraints of
// Namespaces in XML 1.0; "xml-*" are violations of the grammar and other rules of the recommendation; "ns-*" are
// namespace rules outside the list of NSCs; "limit-*" are the limits of the reader (Limits); "unsupported-*" are
// well-formed constructs the reader deliberately does not process.
const (
	CodeEncodingInvalid     = "xml-encoding-invalid"     // bytes not valid in the encoding (4.3.3)
	CodeEncodingUnsupported = "xml-encoding-unsupported" // not UTF-8, UTF-16, ISO-8859-1 or US-ASCII (4.3.3)
	CodeEncodingMismatch    = "xml-encoding-mismatch"    // the encoding declaration contradicts the detected one (4.3.3)
	CodeChar                = "xml-char"                 // a character outside [2] Char
	CodeSyntax              = "xml-syntax"               // a violation of a grammar production
	CodeEOF                 = "xml-eof"                  // the document ends inside a construct
	CodeName                = "xml-name"                 // a name that does not match [5] Name
	CodeNoRoot              = "xml-no-root"              // no root element ([1] document)
	CodeOutsideRoot         = "xml-content-outside-root" // content other than Misc outside the root element ([1], [27])
	CodeCDATAEnd            = "xml-cdata-end"            // "]]>" in character data ([14] CharData)
	CodeComment             = "xml-comment"              // "--" in a comment ([15] Comment)
	CodeReservedPITarget    = "xml-reserved-pi-target"   // a PI target matching [Xx][Mm][Ll] ([17] PITarget)
	CodeXMLDecl             = "xml-decl"                 // a malformed XML declaration ([23]–[26], [32], [80], [81])
	CodeEntityStructure     = "xml-entity-structure"     // the replacement text of an entity is not well-formed content (4.3.2)
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
	CodeQName               = "ns-qname" // not a QName, or a colon in the name of an entity, PI or notation (Namespaces 7)
	CodeLimitDepth          = "limit-depth"
	CodeLimitNameLength     = "limit-name-length"
	CodeLimitAttributes     = "limit-attributes"
	CodeLimitTextSize       = "limit-text-size"
	CodeLimitEntityExpand   = "limit-entity-expansion"
	CodeLimitEntityDepth    = "limit-entity-depth"
	CodeLimitDTDSize        = "limit-dtd-size"
	CodeExternalEntity      = "unsupported-external-entity"   // a reference to an external entity in content: never read
	CodeEntityUnresolved    = "unsupported-entity-unresolved" // the entity may be declared in the external subset, which is never read
	CodeConditionalSection  = "unsupported-conditional-section"
)

// Error is a fatal error (XML 1.0, section 1.2): after it the reader returns no more tokens.
type Error struct {
	Code string // one of the Code* constants
	// Line and Column point at the character where the error was detected; from 1, the column in characters.
	// An error in the replacement text of an entity points at the reference to it in the document.
	Line, Column int
	Message      string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
