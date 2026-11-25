package xsd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/polestorm/goxsdgo/internal/schema"
)

// errNotImplemented oznacza część rekomendacji, której biblioteka jeszcze nie obsługuje. Testy zgodności liczą
// taki wynik jako porażkę, nigdy jako poprawny werdykt „schemat błędny” albo „dokument błędny”.
var errNotImplemented = fmt.Errorf("xsd: %w", errors.ErrUnsupported)

// Config ustawia kompilację schematu. Wartość zerowa to ustawienia domyślne: bez sieci, schematy tylko z fsys.
//
// Kształt API jest wstępny (pytanie 5 w docs/adr/open-questions.md); ustalą go testy kolejnych etapów.
type Config struct {
	// Locations przypisuje adresom ze schemaLocation (w include, import i redefine) ścieżki w fsys: adresowi
	// dosłownie jak w dokumencie albo po rozwiązaniu względem adresu dokumentu, który go zawiera. Schematy MF
	// importują pliki pod adresami http://crd.gov.pl/..., a biblioteka nigdy nie sięga do sieci. Adres względny
	// bez wpisu jest rozwiązywany względem ścieżki dokumentu w fsys.
	Locations map[string]string
	// Namespaces przypisuje przestrzeniom nazw ścieżki dokumentów schematu w fsys; używa ich import bez
	// schemaLocation albo z adresem, którego nie da się rozwiązać (ADR 0006).
	Namespaces map[string]string
	// MaxDocuments i MaxDepth ograniczają liczbę wczytanych dokumentów schematu (domyślnie 1000) i zagnieżdżenie
	// include, import i redefine (domyślnie 64). Zero oznacza wartość domyślną.
	MaxDocuments, MaxDepth int
}

// Compile kompiluje schemat złożony z dokumentów names z fsys, z ustawieniami domyślnymi.
func Compile(fsys fs.FS, names ...string) (*Set, error) {
	return Config{}.Compile(fsys, names...)
}

// Compile kompiluje schemat złożony z dokumentów names z fsys. Gdy schemat nie jest poprawny (rozdziały 3 i 4
// części 1 rekomendacji), zwraca *SchemaError z listą błędów. Błąd opakowujący errors.ErrUnsupported oznacza
// konstrukcję, której biblioteka jeszcze nie obsługuje; nie mówi nic o poprawności schematu.
func (c Config) Compile(fsys fs.FS, names ...string) (*Set, error) {
	sch, err := schema.Load(fsys, schema.Config{Locations: c.Locations, Namespaces: c.Namespaces,
		MaxDocuments: c.MaxDocuments, MaxDepth: c.MaxDepth}, names...)
	if err != nil {
		var list schema.ErrorList
		if errors.As(err, &list) {
			se := &SchemaError{}
			for _, e := range list {
				se.Errors = append(se.Errors, Error{Code: e.Code, Document: e.Pos.Document, Line: e.Pos.Line,
					Column: e.Pos.Column, Message: e.Message})
			}
			return nil, se
		}
		return nil, err
	}
	return &Set{schema: sch}, nil
}

// SchemaError to błąd kompilacji: schemat nie jest poprawny. Errors są w powtarzalnej kolejności.
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

// Set to skompilowany schemat, gotowy do walidacji wielu dokumentów, także równocześnie.
type Set struct {
	schema *schema.Schema
}

// Validate waliduje dokument z r strumieniowo. Pusta lista błędów oznacza dokument poprawny. Błąd zwracany jako
// drugi wynik dotyczy wejścia albo ograniczeń biblioteki, nie treści dokumentu.
func (s *Set) Validate(r io.Reader) ([]Error, error) {
	return nil, errNotImplemented
}

// Error to jeden błąd walidacji dokumentu albo schematu.
type Error struct {
	// Code to stały kod błędu, nazwany według reguły rekomendacji (np. "cvc-complex-type.2.4", "src-resolve").
	Code string
	// Document to ścieżka dokumentu schematu w fsys dla błędów schematu; pusta dla błędów dokumentu.
	Document string
	// Line i Column to pozycja w dokumencie, liczone od 1.
	Line, Column int
	// Path to ścieżka elementu, np. "/JPK/Ewidencja/SprzedazWiersz[3]".
	Path string
	// Message to komunikat po angielsku; do tłumaczenia służy Code.
	Message string
}

func (e Error) Error() string {
	if e.Document != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.Document, e.Line, e.Column, e.Code, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
