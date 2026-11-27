// Package xmlreader is the former home of the XML reader goxsdgo validates with. The reader now lives in its own
// module, github.com/polestorm/goxmlgo/xmlreader; this package keeps programs written for goxsdgo v0.2.0 to v0.4.1
// compiling unchanged.
//
// Every type here is an alias of the type in goxmlgo/xmlreader, so values pass freely between the two import
// paths: an error from either one matches *Error of either one in errors.As, and the newer parts of goxmlgo
// (methods, fields, types such as TokenReader and NamespaceScope) are reached through the aliased types. The
// constants and functions are the ones this package had in v0.3.0.
//
// Deprecated: use github.com/polestorm/goxmlgo/xmlreader.
package xmlreader

import (
	"io"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// Types, as aliases of goxmlgo/xmlreader.
type (
	// Attr is an attribute of a start tag; see goxmlgo/xmlreader.Attr.
	Attr = xmlreader.Attr
	// Binding is a namespace declaration; see goxmlgo/xmlreader.Binding.
	Binding = xmlreader.Binding
	// Error is an error in the document, with a fixed code; see goxmlgo/xmlreader.Error.
	Error = xmlreader.Error
	// Kind is the kind of a token; see goxmlgo/xmlreader.Kind.
	Kind = xmlreader.Kind
	// Limits are the limits of the reader; see goxmlgo/xmlreader.Limits.
	Limits = xmlreader.Limits
	// Name is a name after namespace resolution; see goxmlgo/xmlreader.Name.
	Name = xmlreader.Name
	// Notation is a notation declared in the internal DTD subset; see goxmlgo/xmlreader.Notation.
	Notation = xmlreader.Notation
	// Options are the options of the reader; see goxmlgo/xmlreader.Options.
	Options = xmlreader.Options
	// Reader is a streaming XML reader; see goxmlgo/xmlreader.Reader.
	Reader = xmlreader.Reader
	// Token is a token of the document; see goxmlgo/xmlreader.Token.
	Token = xmlreader.Token
)

// Kinds of tokens.
const (
	KindStartElement = xmlreader.KindStartElement
	KindEndElement   = xmlreader.KindEndElement
	KindText         = xmlreader.KindText
	KindComment      = xmlreader.KindComment
	KindProcInst     = xmlreader.KindProcInst
	KindDoctype      = xmlreader.KindDoctype
)

// Attribute types (Attr.Type).
const (
	AttrTypeCDATA       = xmlreader.AttrTypeCDATA
	AttrTypeID          = xmlreader.AttrTypeID
	AttrTypeIDREF       = xmlreader.AttrTypeIDREF
	AttrTypeIDREFS      = xmlreader.AttrTypeIDREFS
	AttrTypeENTITY      = xmlreader.AttrTypeENTITY
	AttrTypeENTITIES    = xmlreader.AttrTypeENTITIES
	AttrTypeNMTOKEN     = xmlreader.AttrTypeNMTOKEN
	AttrTypeNMTOKENS    = xmlreader.AttrTypeNMTOKENS
	AttrTypeNotation    = xmlreader.AttrTypeNotation
	AttrTypeEnumeration = xmlreader.AttrTypeEnumeration
)

// Error codes (Error.Code); the list and meaning of every code: docs/errors.md in goxmlgo.
const (
	CodeEncodingInvalid     = xmlreader.CodeEncodingInvalid
	CodeEncodingUnsupported = xmlreader.CodeEncodingUnsupported
	CodeEncodingMismatch    = xmlreader.CodeEncodingMismatch
	CodeChar                = xmlreader.CodeChar
	CodeSyntax              = xmlreader.CodeSyntax
	CodeEOF                 = xmlreader.CodeEOF
	CodeName                = xmlreader.CodeName
	CodeNoRoot              = xmlreader.CodeNoRoot
	CodeOutsideRoot         = xmlreader.CodeOutsideRoot
	CodeCDATAEnd            = xmlreader.CodeCDATAEnd
	CodeComment             = xmlreader.CodeComment
	CodeReservedPITarget    = xmlreader.CodeReservedPITarget
	CodeXMLDecl             = xmlreader.CodeXMLDecl
	CodeEntityStructure     = xmlreader.CodeEntityStructure
	CodeElementTypeMatch    = xmlreader.CodeElementTypeMatch
	CodeUniqueAttSpec       = xmlreader.CodeUniqueAttSpec
	CodeNoExternalEntityRef = xmlreader.CodeNoExternalEntityRef
	CodeNoLtInAttValue      = xmlreader.CodeNoLtInAttValue
	CodeLegalCharacter      = xmlreader.CodeLegalCharacter
	CodeEntityDeclared      = xmlreader.CodeEntityDeclared
	CodeParsedEntity        = xmlreader.CodeParsedEntity
	CodeNoRecursion         = xmlreader.CodeNoRecursion
	CodePEsInInternalSubset = xmlreader.CodePEsInInternalSubset
	CodePrefixDeclared      = xmlreader.CodePrefixDeclared
	CodeNoPrefixUndeclaring = xmlreader.CodeNoPrefixUndeclaring
	CodeAttributesUnique    = xmlreader.CodeAttributesUnique
	CodeReservedPrefixes    = xmlreader.CodeReservedPrefixes
	CodeQName               = xmlreader.CodeQName
	CodeLimitDepth          = xmlreader.CodeLimitDepth
	CodeLimitNameLength     = xmlreader.CodeLimitNameLength
	CodeLimitAttributes     = xmlreader.CodeLimitAttributes
	CodeLimitTextSize       = xmlreader.CodeLimitTextSize
	CodeLimitEntityExpand   = xmlreader.CodeLimitEntityExpand
	CodeLimitEntityDepth    = xmlreader.CodeLimitEntityDepth
	CodeLimitDTDSize        = xmlreader.CodeLimitDTDSize
	CodeExternalEntity      = xmlreader.CodeExternalEntity
	CodeEntityUnresolved    = xmlreader.CodeEntityUnresolved
	CodeConditionalSection  = xmlreader.CodeConditionalSection
)

// DefaultLimits returns the default limits; see goxmlgo/xmlreader.DefaultLimits.
func DefaultLimits() Limits { return xmlreader.DefaultLimits() }

// New returns a reader of r; see goxmlgo/xmlreader.New.
func New(r io.Reader, opts Options) *Reader { return xmlreader.New(r, opts) }
