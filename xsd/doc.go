// Package xsd kompiluje schematy XSD 1.0 i waliduje według nich dokumenty XML.
//
//	cfg := xsd.Config{Locations: urls} // adresy importów → pliki w fsys; bez sieci
//	set, err := cfg.Compile(fsys, "jpk_v7m_3.xsd")
//	errs, err := set.Validate(r) // strumieniowo, pamięć ograniczona; Set jest bezpieczny dla wielu goroutines
//
// Każdy błąd ma stały kod nazwany według reguły rekomendacji, wiersz, kolumnę i ścieżkę elementu
// (ADR 0006 dla schematu, ADR 0008 dla dokumentu). Kształt API jest wstępny (pytanie 5).
package xsd
