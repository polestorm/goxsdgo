package xsd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// errNotImplemented oznacza część rekomendacji, której biblioteka jeszcze nie obsługuje. Testy zgodności liczą
// taki wynik jako porażkę, nigdy jako poprawny werdykt „schemat błędny” albo „dokument błędny”.
var errNotImplemented = fmt.Errorf("xsd: %w", errors.ErrUnsupported)

// Config ustawia kompilację schematu. Wartość zerowa to ustawienia domyślne: bez sieci, schematy tylko z fsys.
//
// Kształt API jest wstępny (pytanie 5 w docs/adr/open-questions.md); ustalą go testy kolejnych etapów.
type Config struct {
	// Locations przypisuje bezwzględnym adresom ze schemaLocation (w include, import i redefine) ścieżki w fsys.
	// Schematy MF importują pliki pod adresami http://crd.gov.pl/..., a biblioteka nigdy nie sięga do sieci.
	Locations map[string]string
}

// Compile kompiluje schemat złożony z dokumentów names z fsys, z ustawieniami domyślnymi.
func Compile(fsys fs.FS, names ...string) (*Set, error) {
	return Config{}.Compile(fsys, names...)
}

// Compile kompiluje schemat złożony z dokumentów names z fsys. Zwraca błąd, gdy schemat nie jest poprawny
// (rozdział 3 części 1 rekomendacji).
func (c Config) Compile(fsys fs.FS, names ...string) (*Set, error) {
	return nil, errNotImplemented
}

// Set to skompilowany schemat, gotowy do walidacji wielu dokumentów, także równocześnie.
type Set struct{}

// Validate waliduje dokument z r strumieniowo. Pusta lista błędów oznacza dokument poprawny. Błąd zwracany jako
// drugi wynik dotyczy wejścia albo ograniczeń biblioteki, nie treści dokumentu.
func (s *Set) Validate(r io.Reader) ([]Error, error) {
	return nil, errNotImplemented
}

// Error to jeden błąd walidacji dokumentu.
type Error struct {
	// Code to stały kod błędu, nazwany według reguły rekomendacji (np. "cvc-complex-type.2.4").
	Code string
	// Line i Column to pozycja w dokumencie, liczone od 1.
	Line, Column int
	// Path to ścieżka elementu, np. "/JPK/Ewidencja/SprzedazWiersz[3]".
	Path string
	// Message to komunikat po angielsku; do tłumaczenia służy Code.
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", e.Line, e.Column, e.Code, e.Message)
}
