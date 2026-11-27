package xsd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/polestorm/goxmlgo/xmlchars"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxsdgo/internal/schema"
	"github.com/polestorm/goxsdgo/internal/validate"
)

// DocumentLimits are the limits for reading a validated document (Config.Document). They guard against hostile
// documents: each limit bounds memory or time regardless of the schema (ADR 0004, ADR 0011).
type DocumentLimits struct {
	MaxDepth           int // nesting depth of elements
	MaxNameLength      int // length of an element or attribute name, in UTF-8 bytes
	MaxAttributes      int // attributes of one element, namespace declarations included
	MaxTextSize        int // text of one element, attribute, comment or PI, in bytes
	MaxEntityExpansion int // total text produced by expanding internal entities in a document, in bytes
	MaxEntityDepth     int // nesting of entity references
	MaxDTDSize         int // length of the DOCTYPE declaration with its internal subset, in bytes
}

// DefaultDocumentLimits returns the default document limits: depth 512, name 16 KiB, 4096 attributes,
// text 16 MiB, entity expansion 4 MiB, entity nesting 32, DOCTYPE 1 MiB.
func DefaultDocumentLimits() DocumentLimits {
	l := xmlreader.DefaultLimits()
	return DocumentLimits{MaxDepth: l.MaxDepth, MaxNameLength: l.MaxNameLength, MaxAttributes: l.MaxAttributes,
		MaxTextSize: l.MaxTextSize, MaxEntityExpansion: l.MaxEntityExpansion, MaxEntityDepth: l.MaxEntityDepth,
		MaxDTDSize: l.MaxDTDSize}
}

// ErrLimit reports that validation exceeded a library limit (depth, text size, number of ID and identity-constraint
// entries, complexity of matching a content model). It is not a verdict on the document.
var ErrLimit = validate.ErrLimit

// ErrTooManyErrors reports that validation stopped after Config.MaxErrors errors. Validate returns it together
// with the errors collected so far; the document is then certainly invalid.
var ErrTooManyErrors = validate.ErrTooManyErrors

// Config configures schema compilation and validation. The zero value is the default: no network access, schema
// documents only from fsys, xsi:schemaLocation hints off.
//
// The shape of the API is provisional (question 5 in docs/adr/open-questions.md).
type Config struct {
	// Locations maps schemaLocation addresses (in include, import and redefine) to paths in fsys: the address as
	// written in the document, or resolved against the address of the document that contains it. MF schemas
	// import files from http://crd.gov.pl/..., and the library never goes to the network. A relative address
	// without an entry is resolved against the path of the document in fsys.
	Locations map[string]string
	// Namespaces maps namespaces to paths of schema documents in fsys; an import without schemaLocation, or with
	// an address that cannot be resolved, uses them (ADR 0006).
	Namespaces map[string]string
	// MaxDocuments and MaxDepth limit the number of schema documents read (default 1000) and the nesting of
	// include, import and redefine (default 64). Zero means the default.
	MaxDocuments, MaxDepth int
	// Resolve, if not nil, supplies the schema documents the library cannot find itself: an include, import or
	// redefine whose schemaLocation is an absolute URL without an entry in Locations, or a relative address in a
	// document Resolve supplied (it is resolved against that document's URL). For an import, Namespaces is tried
	// first. Resolve is called at most once per URL during a compilation, and its documents are read in the same
	// pass, so their own includes and imports go through the same rules, limits and cycle checks.
	//
	// The library itself never uses the network; Resolve decides where documents come from (a directory, a
	// cache, the network). Returning an error for which errors.Is(err, fs.ErrNotExist) holds means "not found",
	// as if there were no Resolve; any other error stops Compile and is returned wrapped. A document may be at
	// most MaxResolvedSize bytes. xsi:schemaLocation hints never go to Resolve, only the imports of the schema
	// documents they name.
	Resolve func(SchemaRef) (io.ReadCloser, error)

	// SchemaLocationHints turns on xsi:schemaLocation and xsi:noNamespaceSchemaLocation hints in documents
	// (4.3.2). The schema documents they name are read only from fsys: through Locations, or as a path relative
	// to the document name given to ValidateDocument. Off by default (ADR 0001, ADR 0008).
	SchemaLocationHints bool
	// MaxErrors stops validating a document after this many errors (ErrTooManyErrors). 0: 1000; negative: no limit.
	MaxErrors int
	// MaxIdentityEntries limits the total number of ID and IDREF values and identity-constraint table entries
	// kept while validating one document. 0: 4,194,304.
	MaxIdentityEntries int

	// OnValue, if not nil, receives every valid simple-type value of the document: the content of an element with
	// a simple type or simple content, and attribute values, with the type and the canonical form (ADR 0009).
	// Calls are synchronous, in document order, on the goroutine that called Validate. Without a function,
	// validation computes nothing beyond what it needs itself.
	OnValue func(TypedValue)

	// Document holds the limits for reading the validated document (depth, names, attributes, text, entities,
	// DOCTYPE). A zero field means the default from DefaultDocumentLimits, a negative one no limit. Exceeding a
	// limit ends validation with ErrLimit, not with a document error (ADR 0011).
	Document DocumentLimits

	// NameRules selects the name-character rules for the types Name, NCName, NMTOKEN(S), ID, IDREF(S), ENTITY(IES),
	// QName and NOTATION and for the classes \i, \c (and \I, \C) in patterns, in the schema and in the validated
	// document. The zero value NamesXML10Edition2 follows the reference in XSD 1.0 2e literally; NamesXML10Edition5
	// is the rule set by which the reader reads element and attribute names (ADR 0012).
	NameRules NameRules
}

// NameRules are the rules for name characters (ADR 0012).
type NameRules uint8

const (
	// NamesXML10Edition2: XML 1.0 Second Edition, Appendix B (the default). The same as libxml2 for values of the
	// name types.
	NamesXML10Edition2 NameRules = iota
	// NamesXML10Edition5: XML 1.0 Fifth Edition, productions [4] and [4a]; a wider set of characters, as for the
	// element and attribute names of a document and in XSD 1.1.
	NamesXML10Edition5
)

func (n NameRules) edition() xmlchars.Edition {
	if n == NamesXML10Edition5 {
		return xmlchars.Edition5
	}
	return xmlchars.Edition2
}

// TypedValue is a valid simple-type value of a document, passed to Config.OnValue.
type TypedValue struct {
	// Path is the path of the element or attribute, as in Error.Path; Name the local name; Attribute: an attribute.
	Path, Name string
	Attribute  bool
	// Type is the type name ("{namespace}name" or "name"), or "" for an anonymous type; Builtin is the name of the
	// nearest built-in type it is derived from (for example "decimal", "date", "anySimpleType").
	Type, Builtin string
	// Canonical is the value in the canonical form of its type (Part 2), for example "12.5" for " 012.50 ".
	Canonical string
	// Line and Column are the position as in Error: the end of the element's start tag, or the attribute.
	Line, Column int
}

// Compile compiles the schema made of the documents names in fsys, with the default settings.
func Compile(fsys fs.FS, names ...string) (*Set, error) {
	return Config{}.Compile(fsys, names...)
}

// Compile compiles the schema made of the documents names in fsys. When the schema is not valid (chapters 3 and
// 4 of Part 1), it returns a *SchemaError with the list of errors. An error wrapping errors.ErrUnsupported means
// a construct the library does not support yet; it says nothing about the validity of the schema. With hints
// on (SchemaLocationHints) names may be empty: the documents will name the schema.
func (c Config) Compile(fsys fs.FS, names ...string) (*Set, error) {
	s := &Set{cfg: c, fsys: fsys, names: append([]string(nil), names...)}
	if len(names) == 0 && c.SchemaLocationHints {
		s.schema = schema.Empty()
		return s, nil
	}
	sch, err := schema.Load(fsys, c.schemaConfig(), names...)
	if err != nil {
		var list schema.ErrorList
		if !errors.As(err, &list) && !errors.Is(err, errors.ErrUnsupported) {
			return nil, fmt.Errorf("xsd: %w", err)
		}
		return nil, schemaError(err)
	}
	s.schema = sch
	return s, nil
}

func (c Config) schemaConfig() schema.Config {
	sc := schema.Config{Locations: c.Locations, Namespaces: c.Namespaces, MaxDocuments: c.MaxDocuments,
		MaxDepth: c.MaxDepth, NameRules: c.NameRules.edition()}
	if f := c.Resolve; f != nil {
		sc.Resolve = func(r schema.Ref) (io.ReadCloser, error) {
			return f(SchemaRef{Location: r.Location, Namespace: r.Namespace, Kind: r.Kind, From: r.From})
		}
	}
	return sc
}

// SchemaRef is a reference to a schema document that Config.Resolve is asked for.
type SchemaRef struct {
	// Location is the absolute URL of the document: schemaLocation resolved against the referring document.
	Location string
	// Namespace is the namespace attribute of an import; empty for include and redefine.
	Namespace string
	// Kind is "include", "import" or "redefine".
	Kind string
	// From is the name of the referring document (see SchemaDocument.Name).
	From string
}

// MaxResolvedSize is the largest schema document accepted from Config.Resolve, in bytes.
const MaxResolvedSize = schema.MaxResolvedSize

// SchemaDocument describes one schema document a Set was compiled from.
type SchemaDocument struct {
	// Name is the path of the document in fsys, or its URL when Config.Resolve supplied it. Schema errors use the
	// same name in Error.Document.
	Name string
	// Location is the absolute URL the document was found under (through Locations or Resolve); empty for a
	// document found by its path only.
	Location string
	// Resolved reports that Config.Resolve supplied the document.
	Resolved bool
	// Size is the length of the document in bytes; SHA256 the hexadecimal SHA-256 of those bytes.
	Size   int64
	SHA256 string
}

// Documents returns the schema documents the Set was compiled from, in the order they were read: the documents
// given to Compile and everything they include, import or redefine. Documents named only by xsi:schemaLocation
// hints are not in the list. The list is a copy.
func (s *Set) Documents() []SchemaDocument {
	out := make([]SchemaDocument, len(s.schema.Documents))
	for i, d := range s.schema.Documents {
		out[i] = SchemaDocument{Name: d.Name, Location: d.Location, Resolved: d.Resolved, Size: d.Size,
			SHA256: hex.EncodeToString(d.SHA256[:])}
	}
	return out
}

func schemaError(err error) error {
	var list schema.ErrorList
	if !errors.As(err, &list) {
		return err
	}
	se := &SchemaError{}
	for _, e := range list {
		se.Errors = append(se.Errors, Error{Code: e.Code, Document: e.Pos.Document, Line: e.Pos.Line,
			Column: e.Pos.Column, Message: e.Message})
	}
	return se
}

// SchemaError is a compilation error: the schema is not valid. Errors come in a deterministic order.
type SchemaError struct {
	Errors []Error
}

func (e *SchemaError) Error() string {
	if len(e.Errors) == 0 {
		return "xsd: invalid schema"
	}
	var b strings.Builder
	b.WriteString("xsd: invalid schema: ")
	b.WriteString(e.Errors[0].Error())
	if len(e.Errors) > 1 {
		fmt.Fprintf(&b, " (and %d more)", len(e.Errors)-1)
	}
	return b.String()
}

// Set is a compiled schema, ready to validate many documents, also concurrently.
type Set struct {
	schema *schema.Schema
	cfg    Config
	fsys   fs.FS
	names  []string

	// hinted: schematy poszerzone o dokumenty ze wskazówek, według posortowanej listy ścieżek.
	mu     sync.Mutex
	hinted map[string]hintedSchema
}

type hintedSchema struct {
	sch *schema.Schema
	err error
}

// Validate validates the document read from r, as a stream. An empty list of errors means a valid document. The
// second result is about the input or the library's limits (ErrLimit, errors.ErrUnsupported), not the content of
// the document; the exception is ErrTooManyErrors, which comes together with the document's errors.
func (s *Set) Validate(r io.Reader) ([]Error, error) {
	return s.ValidateDocument(r, "")
}

// ValidateDocument works like Validate; name is the path of the document in the schema's fsys, against which
// relative xsi:schemaLocation hints are resolved (with Config.SchemaLocationHints). Without hints name is used
// only in messages and may be empty.
func (s *Set) ValidateDocument(r io.Reader, name string) ([]Error, error) {
	d := s.cfg.Document
	opts := validate.Options{MaxErrors: s.cfg.MaxErrors, MaxIdentityEntries: s.cfg.MaxIdentityEntries,
		MaxTextSize: d.MaxTextSize, Reader: xmlreader.Limits{MaxDepth: d.MaxDepth, MaxNameLength: d.MaxNameLength,
			MaxAttributes: d.MaxAttributes, MaxTextSize: d.MaxTextSize, MaxEntityExpansion: d.MaxEntityExpansion,
			MaxEntityDepth: d.MaxEntityDepth, MaxDTDSize: d.MaxDTDSize}, Names: s.cfg.NameRules.edition()}
	if f := s.cfg.OnValue; f != nil {
		opts.OnValue = func(ev validate.ValueEvent) {
			f(TypedValue{Path: ev.Path, Name: ev.Name, Attribute: ev.Attribute, Type: ev.Type, Builtin: ev.Builtin,
				Canonical: ev.Canonical, Line: ev.Line, Column: ev.Column})
		}
	}
	if s.cfg.SchemaLocationHints {
		opts.Hints = &hintLoader{set: s, dir: path.Dir(name), loaded: append([]string(nil), s.names...)}
	}
	verrs, err := validate.Validate(s.schema, r, opts)
	var out []Error
	if len(verrs) > 0 {
		out = make([]Error, len(verrs))
		for i, e := range verrs {
			out[i] = Error{Code: e.Code, Line: e.Line, Column: e.Column, Path: e.Path, Name: e.Name, Value: e.Value,
				Facet: e.Facet, Expected: e.Expected, Message: e.Message, Namespace: e.Namespace, Reason: e.Reason,
				ExpectedNamespace: e.ExpectedNamespace, Type: e.Type, TypeNamespace: e.TypeNamespace,
				Builtin: e.Builtin, InsertLine: e.InsertLine, InsertColumn: e.InsertColumn}
		}
	}
	if err != nil && !errors.Is(err, ErrTooManyErrors) {
		err = fmt.Errorf("xsd: %w", err)
	}
	return out, err
}

// hintLoader wczytuje dokumenty ze wskazówek jednego dokumentu; schematy są wspólne dla Set (pamięć podręczna).
type hintLoader struct {
	set    *Set
	dir    string
	loaded []string
}

func (h *hintLoader) LoadHints(hints []validate.Hint) (*schema.Schema, error) {
	added := false
	for _, hint := range hints {
		p, ok := h.resolve(hint.Location)
		if !ok {
			continue // wskazówka bez dokumentu w fsys: procesor może ją pominąć (4.3.2)
		}
		dup := false
		for _, q := range h.loaded {
			if q == p {
				dup = true
				break
			}
		}
		if !dup {
			h.loaded = append(h.loaded, p)
			added = true
		}
	}
	if !added {
		return nil, nil
	}
	return h.set.schemaFor(h.loaded)
}

func (h *hintLoader) resolve(loc string) (string, bool) {
	if p, ok := h.set.cfg.Locations[loc]; ok {
		return p, true
	}
	if strings.Contains(loc, ":") || strings.HasPrefix(loc, "/") {
		return "", false
	}
	p := path.Clean(path.Join(h.dir, loc))
	if !fs.ValidPath(p) {
		return "", false
	}
	if _, err := fs.Stat(h.set.fsys, p); err != nil {
		return "", false
	}
	return p, true
}

func (s *Set) schemaFor(names []string) (*schema.Schema, error) {
	key := append([]string(nil), names...)
	sort.Strings(key)
	k := strings.Join(key, "\x00")
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.hinted[k]; ok {
		return h.sch, h.err
	}
	sch, err := schema.Load(s.fsys, s.cfg.schemaConfig(), names...)
	if s.hinted == nil {
		s.hinted = map[string]hintedSchema{}
	}
	s.hinted[k] = hintedSchema{sch, err}
	return sch, err
}

// Error is one validation error of a document or a schema.
type Error struct {
	// Code is the stable error code, named after the rule of the recommendation ("cvc-complex-type.2.4").
	Code string
	// Document is the path of the schema document in fsys for schema errors; empty for document errors.
	Document string
	// Line and Column are the position in the document, counted from 1; the column counts characters.
	Line, Column int
	// Path is the element path, for example "/JPK/Ewidencja/SprzedazWiersz[3]": local names without prefixes, [n]
	// from the second occurrence of a name among siblings, "/@name" for an attribute (ADR 0008).
	Path string
	// Name is the local name of the element or attribute the error is about.
	Name string
	// Value is the value the error is about (for example text that does not match its type, a duplicate key).
	Value string
	// Facet is the violated facet and its value, for example "maxLength 5"; empty when no facet is involved.
	Facet string
	// Expected lists the elements allowed at the place of a cvc-complex-type.2.4 error: "name" or
	// "{namespace}name", a wildcard as "*" or "{namespace}*".
	Expected []string
	// Message is the message in English; translations key on Code, with the parameters in the other fields.
	Message string

	// Namespace is the namespace of the element or attribute Name; empty for a name in no namespace (ADR 0010).
	Namespace string
	// Reason tells cvc-complex-type.2.4 errors apart: ReasonMissing (a required element is missing),
	// ReasonUnexpected (an element not allowed at this place), ReasonWrongNamespace (the local name of an expected
	// element, in another namespace). Empty for other codes; the rule code is always the same.
	Reason string
	// ExpectedNamespace is the namespace in which the schema expects the element (ReasonWrongNamespace only).
	ExpectedNamespace string
	// Type and TypeNamespace are the schema type the value does not match (simple-type errors, e.g. "TData");
	// Type is empty for an anonymous type. Builtin is the nearest built-in type it is derived from
	// (for example "date", "decimal").
	Type, TypeNamespace, Builtin string
	// InsertLine and InsertColumn (cvc-complex-type.2.4) show where the expected element should start: before the
	// element that is not allowed (the start of its tag), or before the parent's end tag; for a parent written as
	// <x/>, the end of that tag. The main position (Line, Column) is unchanged (ADR 0008).
	InsertLine, InsertColumn int
}

// Values of Error.Reason for cvc-complex-type.2.4 (ADR 0010).
const (
	ReasonMissing        = validate.ReasonMissing
	ReasonUnexpected     = validate.ReasonUnexpected
	ReasonWrongNamespace = validate.ReasonWrongNamespace
)

func (e Error) Error() string {
	if e.Document != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.Document, e.Line, e.Column, e.Code, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
