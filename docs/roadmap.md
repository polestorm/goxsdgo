# Plan

Szacunki z 2026-11-24, w godzinach pracy z asystentem. Kryteria zakończenia całości: ADR 0001, kryteria K1–K9.

| Etap | Zakres | Szacunek | Stan |
|---|---|---|---|
| 0 | infrastruktura testów: pobranie i przypięcie zestawu W3C (pytanie 1), czytnik metadanych zestawu (`*.testSet`, oczekiwane werdykty), raport zaliczeń według grup (Microsoft, Sun, NIST, Boeing, IBM), test pokrycia wymagań, szkielet fuzzingu, `tools/compare` z `goxsdval` (`-tags libxml2`), korpus MF z plikiem oczekiwań | 6–10 h | zrobione 2026-11-25: `tools/fetch` (Go i Python, ADR 0003), `internal/w3c`, `TestW3CSuite`, `TestSchemaCorpus`, `TestInstanceCorpus`, `TestRequirementsCovered`, `FuzzCompile`, `FuzzValidate`, zapadki, `tools/compare` (libxml2: W3C 98,5%, MF 170/170 i 8/8) |
| 1 | czytnik XML: XML 1.0 Fifth Edition i przestrzenie nazw, strumieniowo, wiersz i kolumna, dobra forma, DOCTYPE bez rozwijania encji zewnętrznych, limity (pytanie 3) | 12–18 h | zrobione 2026-11-25: `internal/xmlreader` (ADR 0004), XML Conformance 1725/1727, XML-001…017, `FuzzReader`, ok. 195 MB/s; do podłączenia w etapach 3 i 5 |
| 2 | typy proste: 44 typy wbudowane, przestrzeń wartości i leksykalna, normalizacja białych znaków, wszystkie aspekty (`length`, `pattern`, `enumeration`, `totalDigits`, `fractionDigits`, zakresy…), listy i unie, tłumacz wyrażeń regularnych XSD → RE2 | 20–30 h | zrobione 2026-11-25: `internal/datatypes`, `internal/regex` (ADR 0005), DT-001…018, RE-001…011; pomiar pośredni NIST 22 678 / 22 678 odczytanych, MS-Regex 2 772 / 2 789; K1 dla typów po etapie 3 |
| 3 | wczytanie schematu: komponenty z dokumentu schematu, `include`, `import`, `redefine`, rozwiązywanie nazw i przestrzeni, `fs.FS` bez sieci, katalog przestrzeń → plik, ograniczenia poprawności schematu (rozdział 3 części 1) z kodami błędów | 20–30 h | zrobione 2026-11-25: `internal/schema` (ADR 0006), testy schematów W3C 14 048 / 14 393, schematy MF 170 / 170, SCH-001…020; UPA, EDC i restrykcja cząstek w etapie 4 |
| 4 | typy złożone i modele zawartości: rozszerzenie i ograniczenie (z kontrolą poprawnego ograniczenia), `sequence`/`choice`/`all`, liczności jako liczniki, UPA, EDC, symbole wieloznaczne, grupy podstawienia, `abstract`, `block`, `final`, `mixed`, `nillable` | 20–30 h | zrobione 2026-11-25: `internal/schema` (`model.go`, `restrict.go`, `subst.go`), ADR 0007, testy schematów W3C 14 370 / 14 384, MF 170 / 170, CM-001…015, `TestPathological`, `FuzzContentModel` |
| 5 | walidacja dokumentu: strumieniowa ocena elementów i atrybutów, `xsi:type`, `xsi:nil`, wartości domyślne i stałe, ograniczenia tożsamości z podzbiorem XPath, zbieranie błędów z limitem, ścieżka elementu | 15–20 h | zrobione 2026-11-25: `internal/validate` (ADR 0008), W3C 39 367 / 39 439 (testy dokumentów 24 997 / 25 055), korpus MF i syntetyczny 15 / 15, VAL-001…022; K6: 1 GB w 5,8 MB; K7: 1,30× libxml2 |
| 6 | błędy dla ludzi i zgodność z odbiorcą: katalog kodów, parametry błędów, tabela `compat` do kodów libxml2, pierwszy błąd w tym samym miejscu co libxml2, `differences.md`, opcjonalna funkcja zwrotna z wartością typowaną (pytanie 6) | 8–12 h | zrobione 2026-11-25: katalog 234 kodów (`xsd/codes.go`, `docs/errors.md`), compat 223 / 223 (189 zaobserwowanych, 34 przypisanych), `docs/differences-libxml2.md` (528 rozbieżności, wszystkie skategoryzowane), `Config.OnValue`, ERR-001…008, ADR 0009 |
| 7 | utwardzanie: przypadki patologiczne, fuzzing, pamięć przy pliku 1 GB, testy wydajności na dużych JPK | 8–12 h | — |
| 8 | odbiór: pomiar K1–K9, pakiet dowodów, próba przepięcia u odbiorcy na jego testach | 4–6 h | — |
| | **razem** | **115–170 h** | |

Walidator odniesienia w `tools/compare`: `goxsdval` z libxml2 2.15.3 lub nowszą (wersja przypięta w pomiarach).

Rozmiar docelowy: 10–14 tys. linii kodu bez testów (dla porównania: moduł schematów libxml2 w C ma ok. 29 tys. linii,
same walidacje bez parsera XML).

Kolejność jest zależnością: etap 3 potrzebuje czytnika z etapu 1 i typów z etapu 2; etap 5 potrzebuje 3 i 4. Etapy 1 i 2
można prowadzić równolegle. Liczba zaliczonych przypadków W3C, w podziale na grupy, jest datowaną miarą postępu po każdym
etapie; pierwsze liczby (typy proste z grupy NIST) pojawią się po etapie 2.
