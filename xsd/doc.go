// Package xsd compiles XML Schema 1.0 schemas and validates XML documents against them.
//
//	cfg := xsd.Config{Locations: urls} // import addresses → files in fsys; no network access
//	set, err := cfg.Compile(fsys, "jpk_v7m_3.xsd")
//	errs, err := set.Validate(r) // streaming, bounded memory; a Set is safe for concurrent use
//
// Schema documents come from fsys. Documents outside it (an import of http://crd.gov.pl/...) come only through
// Config.Resolve, a function the caller supplies; the package xsd/resolve provides one that reads a local
// directory and a cache and downloads only when told to. Set.Documents lists every document a schema was compiled
// from, with its SHA-256.
//
// Every error has a stable code named after the rule of the recommendation, a line, a column and an element path
// (ADR 0006 for schemas, ADR 0008 for documents). The shape of the API is provisional (open question 5).
package xsd
