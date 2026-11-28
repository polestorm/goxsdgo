# Wymagania: czytnik XML

Czytnik XML jest od goxsdgo v0.5.0 w osobnym module `github.com/polestorm/goxmlgo` (pakiet `xmlreader`; ADR 0014).
Wymagania czytnika i ich testy są tam: wersja z `go.mod` goxsdgo, plik `requirements/xml.md` w goxmlgo. Tu zostają
odsyłacze (wymagania delegowane) i wymagania goxsdgo, które dotyczą czytnika widzianego przez walidator.

## Wymagania delegowane do goxmlgo

Identyfikatory i treść bez zmian w goxmlgo (przeniesione stamtąd z commitu źródłowego, `docs/provenance.md` w goxmlgo).
`TestRequirementsCovered` sprawdza, że wersja w odsyłaczu jest wersją z `go.mod`, a wskazane wymaganie istnieje
w goxmlgo; testu w goxsdgo te wymagania nie mają.

| Id | Wymaganie |
|---|---|
| XML-001 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-001. |
| XML-002 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-002. |
| XML-003 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-003. |
| XML-004 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-004. |
| XML-005 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-005. |
| XML-006 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-006. |
| XML-007 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-007. |
| XML-008 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-008. |
| XML-009 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-009. |
| XML-010 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-010. |
| XML-011 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-011. |
| XML-012 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-012. |
| XML-013 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-013. |
| XML-014 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-014. |
| XML-015 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-015. |
| XML-016 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-016. |
| XML-017 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-017. |
| XML-018 | Delegowane: goxmlgo v0.5.0, `requirements/xml.md` XML-018. |

## Wymagania goxsdgo

| Id | Wymaganie |
|---|---|
| XML-019 | Wynik walidacji (`xsd.Set.Validate`: błędy z kodem, wierszem, kolumną i ścieżką) nie zależy od podziału wejścia na odczyty `io.Reader`: jeden odczyt, po jednym bajcie i małymi porcjami dają to samo (poprawka 2026-11-26, wersje dotknięte v0.1.0–v0.4.0). Identyfikator goxsdgo: XML-019 w goxmlgo to inne wymaganie (`Skip`); tę samą własność samego czytnika goxmlgo sprawdza w DET-003. |
| XML-020 | Pakiet `goxsdgo/xmlreader` jest warstwą zgodności (przestarzały): aliasy typów `goxmlgo/xmlreader`, stałe i funkcje przekazujące z v0.3.0. Jego API jest źródłowo zgodne z v0.3.0 (wzorzec `xmlreader/testdata/api-v0.3.0.txt`), program napisany dla v0.3.0 kompiluje się i działa bez zmian, a typy z obu ścieżek importu są tożsame (`errors.As` z `*xmlreader.Error` z dowolnej ścieżki odbiera błąd z `xsd`). Identyfikator goxsdgo (XML-020 w goxmlgo to inne wymaganie). |
