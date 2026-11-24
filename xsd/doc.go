// Package xsd kompiluje schematy XSD 1.0 i waliduje według nich dokumenty XML.
//
// Plan API (etap 0, do potwierdzenia testami):
//
//	set, err := xsd.Compile(fsys, "jpk_v7m_3.xsd", opts...) // schemat z importami, bez sieci
//	errs, err := set.Validate(r)                           // strumieniowo, pamięć ograniczona
//
// Każdy błąd ma stały kod, wiersz, kolumnę i ścieżkę elementu.
package xsd
