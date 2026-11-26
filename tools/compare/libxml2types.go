package main

import (
	"errors"
	"fmt"
)

// Własne, małe wiązanie do libxml2 (pliki libxml2*.go): tylko to, czego potrzebuje porównanie, napisane
// z deklaracji publicznego API libxml2 (xmlschemas.h, xmlerror.h, xmlreader.h, catalog.h, parser.h). Bez tagu
// libxml2 każda funkcja zwraca errNotEnabled, żeby moduł kompilował się bez cgo.

// libSeverity odpowiada xmlErrorLevel z xmlerror.h.
type libSeverity int

const (
	sevNone    libSeverity = 0
	sevWarning libSeverity = 1
	sevError   libSeverity = 2
	sevFatal   libSeverity = 3
)

// Domeny błędów (xmlErrorDomain) potrzebne do rozpoznania błędu składni dokumentu.
const (
	domainParser    = 1  // XML_FROM_PARSER
	domainNamespace = 3  // XML_FROM_NAMESPACE
	domainSchemasV  = 17 // XML_FROM_SCHEMASV
)

// libError to jeden błąd albo ostrzeżenie libxml2 (pola xmlError).
type libError struct {
	Code     int
	Domain   int
	Severity libSeverity
	Line     int
	NodeName string
	Message  string
}

var (
	errNotEnabled    = errors.New("libxml2 not enabled in this build (build with -tags libxml2)")
	errTooManyErrors = errors.New("libxml2: too many errors, validation stopped")
)

// schemaParseError: libxml2 nie skompilował schematu; Errors to zgłoszone przyczyny (z ostrzeżeniami).
type schemaParseError struct {
	Errors []libError
}

func (e *schemaParseError) Error() string {
	for _, x := range e.Errors {
		if x.Severity >= sevError {
			return fmt.Sprintf("libxml2: schema does not compile: %d %s", x.Code, oneLine(x.Message))
		}
	}
	return "libxml2: schema does not compile"
}

// parseFailure zostawia z listy błędów walidacji dokumentu, który nie jest poprawnie uformowany, błędy
// znalezione przed błędem składni i pierwszy błąd składni (z poziomem fatal); dalsze błędy składni mogą być
// tylko skutkiem pierwszego.
func parseFailure(errs []libError) []libError {
	for i, e := range errs {
		if (e.Domain == domainParser || e.Domain == domainNamespace) && e.Severity >= sevError {
			var out []libError
			for _, p := range errs[:i] {
				if p.Domain != domainParser && p.Domain != domainNamespace {
					out = append(out, p)
				}
			}
			e.Severity = sevFatal
			return append(out, e)
		}
	}
	return errs
}
