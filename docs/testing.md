# Testy i porównania

## Warstwy

| Warstwa | Gdzie | Co sprawdza |
|---|---|---|
| zestaw W3C | `testdata/w3c/xsdtests/` (ADR 0003), `TestW3CSuite`, czytnik `internal/w3c` | werdykt dla każdego schematu i dokumentu z zestawu XSD 1.0 2e; raport w podziale na dostawców (NIST, Microsoft, Sun, Boeing, IBM, Saxonica, W3C); wyłączenia z powodem; zapadka `testdata/w3c/xsdtests.ratchet` |
| zgodność XML | `internal/xmlreader`, `TestXMLConformance` | czytnik XML na zestawie W3C XML Conformance (dobra forma, przestrzenie nazw, druga postać kanoniczna tam, gdzie zestaw ją podaje); zapadka `testdata/w3c/xmlconf.ratchet`, `go test ./internal/xmlreader -update` |
| typy proste (pośrednio) | `internal/datatypes`, `TestW3CDatatypes` | przypadki NIST i Microsoft (DataTypes, Regex) z zestawu W3C przez adapter w testach, który czyta schematy o prostym kształcie; nie jest to K1, bo bez etapu 3 brak prawdziwego wczytania schematu; zapadki `testdata/w3c/nist-datatypes.ratchet`, `ms-datatypes.ratchet` |
| wymagania | `requirements/*.md`, `TestRequirementsCovered` | każde wymaganie ma test albo plik korpusu; każde odwołanie `REQ:` wskazuje istniejące wymaganie |
| korpus schematów | `testdata/schemas/mf/`, `TestSchemaCorpus` | każdy ze 170 schematów MF (JPK, KSeF, e-Sprawozdania) kompiluje się bez błędów; importy rozwiązuje katalog adresów z manifestu, bez sieci; zapadki `testdata/mf-schemas.ratchet`, `testdata/mf-instances.ratchet` |
| korpus dokumentów | `testdata/instances/` + `expected.json`, `TestInstanceCorpus` | werdykt (później też kody i miejsca błędów) zgodny z oczekiwanym; zapadka jak wyżej |
| korpus własny | `testdata/corpus/` | małe schematy i dokumenty dla każdego wymagania, w tym polskie znaki, duże liczności, typy z ograniczeniami |
| różnicowe | `tools/compare` (osobny moduł, cgo) | ten sam zestaw W3C i korpus MF przez libxml2 (`goxsdval`); werdykty dla każdego przypadku w `out/compare/*.tsv`, od etapu 3 także rozbieżności testów schematów goxsdgo i libxml2 (`out/compare/schemas-goxsdgo-libxml2.tsv`); od etapu 3 raport różnic względem `goxsdgo` i miejsca pierwszego błędu; każda różnica werdyktu w `differences.md` |
| bezpieczeństwo | `TestSafeByDefault` | brak sieci i encji zewnętrznych, limity głębokości i rozmiaru, schemat spoza `fs.FS` odrzucony |
| pamięć | `TestStreamingMemory` | syntetyczny JPK 1 GB (generator w teście, nie plik): szczyt pamięci < 64 MB |
| powtarzalność | `TestDeterministic` | wielokrotna walidacja daje te same błędy w tej samej kolejności |
| odporność | `FuzzCompile`, `FuzzValidate` | dowolny schemat i dokument: bez paniki, czas ograniczony |
| patologiczne | `TestPathological` | wzorce z zagnieżdżonymi powtórzeniami, `maxOccurs` w setkach tysięcy, głębokie zagnieżdżenia, tysiące kluczy: czas rośnie liniowo |
| wydajność | `BenchmarkCorpus` | porównanie z `goxsdval` (kryterium K7) |

## CI i dane testowe

CI pobiera dane według manifestu (ADR 0003) i trzyma je w pamięci podręcznej według sumy manifestu. Część schematów MF
(22 pliki e-Sprawozdań i słowników, stan 2026-11-25) jest pod adresami crd.gov.pl, które przekierowują na
epuap.gov.pl, a ten serwer nie odpowiada z maszyn GitHuba. Wtedy `TestSchemaCorpus` i `TestInstanceCorpus` są
w CI pomijane z komunikatem, a kryterium K2 mierzymy lokalnie. Zestaw W3C i przykłady MF CI pobiera bez przeszkód.

## Zapadki

Pliki `*.ratchet` (`testdata/w3c/xsdtests.ratchet`, `xmlconf.ratchet`, `nist-datatypes.ratchet`,
`ms-datatypes.ratchet`, `testdata/mf-schemas.ratchet`, `testdata/mf-instances.ratchet`) mówią, które przypadki
w zakresie już przechodzą. Plik zawiera krótszą z dwóch list: przypadki zaliczone (`# mode: passing`) albo
niezaliczone (`# mode: failing`); nagłówek podaje, ile przypadków jest w zakresie i ile przechodzi
(`internal/testkit.Ratchet`). Test kończy się błędem, gdy przypadek, który przechodził, przestaje przechodzić. Nowe
zaliczenia zapisuje `go test ./xsd ./internal/... -update`, a zmiana pliku jest widoczna w przeglądzie. Przypadek
przestaje przechodzić tylko z wyjaśnieniem w opisie commitu. Przypadek, którego `goxsdgo` jeszcze nie umie
rozstrzygnąć (`errors.ErrUnsupported`), nie jest nigdy zaliczony, także gdy oczekiwany werdykt to „błędny”.

## Walidator odniesienia

```sh
cd tools/compare
PKG_CONFIG_PATH=/opt/homebrew/opt/libxml2/lib/pkgconfig go run -tags libxml2 . w3c     # zestaw W3C
PKG_CONFIG_PATH=/opt/homebrew/opt/libxml2/lib/pkgconfig go run -tags libxml2 . corpus  # korpus MF
```

Na Linuksie wystarczy `go run -tags libxml2 . w3c` (pakiet `libxml2-dev`). Moduł zależy od prywatnego
`github.com/polestorm/goxsdval`, więc potrzebny jest dostęp do niego i `GOPRIVATE=github.com/polestorm/*`. Importy
spod adresów w sieci (MF: crd.gov.pl, gov.pl; W3C: `xml.xsd`, `xlink.xsd`) rozwiązuje katalog XML zbudowany
z manifestu.

Przypadki W3C, których libxml2 przez `goxsdval` nie potrafi wykonać, są oznaczone jako nieporównywalne: 55 przypadków
bez schematu w metadanych (schemat z `xsi:schemaLocation`) i 154 przypadki ze schematem z kilku dokumentów
(`goxsdval` kompiluje schemat z jednego pliku).

## Pomiary

Wyniki zapisujemy tutaj z datą, wersją i sposobem pomiaru.

| Data | Commit | W3C (zaliczone / w zakresie) | Korpus MF | Zgodność z libxml2 | Uwagi |
|---|---|---|---|---|---|
| 2026-11-25 | wyłączenie 9 grup XSD 1.1 bez żetonu | 14 048 / 39 439 (testy schematów 14 048 / 14 384) | bez zmian | bez zmian | 22 przypadki wyłączone z powodem (`internal/w3c.ExcludedGroups`, `differences.md`) |
| 2026-11-25 | `0546663` | 14 048 / 39 461 (testy schematów 14 048 / 14 393: poprawne 11 024 / 11 044, błędne 3 024 / 3 349; testy dokumentów 0 / 25 068, etap 5) | schematy 170 / 170, dokumenty 0 / 8 | testy schematów: ten sam werdykt co libxml2 w 98,6% z 14 316 porównywalnych; z 196 rozbieżności rację ma goxsdgo w 158, libxml2 w 38 | etap 3; 324 z 325 niezaliczonych „błędnych” czeka na kontrole z etapu 4 (265 MS-Particles, 24 MS-ModelGroups, reszta UPA, EDC, redefinicje grup) |
| 2026-11-25 | `892eade` | 0 / 39 461 | — | — | etap 2, pomiar pośredni (adapter, nie K1): NIST 22 678 / 22 678 odczytanych z 23 170 w zakresie (nieodczytane: 492 przypadki ID z typem złożonym), MS-Regex 2 772 / 2 789 z 3 998, MS-DataTypes 29 / 29 z 3 430; 17 rozbieżności opisanych w `differences.md`; `go test ./internal/datatypes -run W3CDatatypes -v` |
| 2026-11-25 | `b502002` | 0 / 39 461 | — | — | etap 1: XML Conformance 1725 / 1727 (xmltest 299/299, sun 101/101, oasis 323/323, ibm 526/527, eduni 476/477); `encoding/xml` na tym samym zestawie 1146 / 1727; werdykt dobrej formy jak w libxml2 dla 42 018 plików `.xml` i `.xsd` z zestawu W3C XSD i korpusu MF poza 3; `go test ./internal/xmlreader -run XMLConformance -v` |
| 2026-11-25 | `bb585b7` | 0 / 39 461 | schematy 0 / 170, dokumenty 0 / 8 | — | etap 0: infrastruktura bez implementacji; `go test ./xsd -run 'W3C\|Corpus' -v` |

Pomiary walidatora odniesienia (libxml2 2.15.4 przez `goxsdval` `2f45df6`, `tools/compare`):

| Data | W3C: zgodne z oczekiwanym / porównywalne / w zakresie | Korpus MF | Uwagi |
|---|---|---|---|
| 2026-11-25 | 38 669 / 39 252 / 39 461 (98,5% porównywalnych) | schematy 170 / 170, dokumenty 8 / 8 | według dostawców: NIST 23 170 / 23 170, Microsoft 13 675 / 14 198, Sun 1 587 / 1 610, Saxonica 98 / 130, W3C 92 / 92, IBM 44 / 49, Boeing 3 / 3 |
