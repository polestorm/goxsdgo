# Testy i porównania

## Warstwy

| Warstwa | Gdzie | Co sprawdza |
|---|---|---|
| zestaw W3C | `testdata/w3c/xsdtests/` (ADR 0003), `TestW3CSuite`, czytnik `internal/w3c` | werdykt dla każdego schematu i dokumentu z zestawu XSD 1.0 2e; raport w podziale na dostawców (NIST, Microsoft, Sun, Boeing, IBM, Saxonica, W3C); wyłączenia z powodem; zapadka `testdata/w3c/passing.txt` |
| zgodność XML | `TestXMLConformance` | czytnik XML na zestawie W3C XML Conformance (dobra forma, przestrzenie nazw) |
| wymagania | `requirements/*.md`, `TestRequirementsCovered` | każde wymaganie ma test albo plik korpusu; każde odwołanie `REQ:` wskazuje istniejące wymaganie |
| korpus schematów | `testdata/schemas/mf/`, `TestSchemaCorpus` | każdy ze 170 schematów MF (JPK, KSeF, e-Sprawozdania) kompiluje się bez błędów; importy rozwiązuje katalog adresów z manifestu, bez sieci; zapadka `testdata/passing-corpus.txt` |
| korpus dokumentów | `testdata/instances/` + `expected.json`, `TestInstanceCorpus` | werdykt (później też kody i miejsca błędów) zgodny z oczekiwanym; zapadka jak wyżej |
| korpus własny | `testdata/corpus/` | małe schematy i dokumenty dla każdego wymagania, w tym polskie znaki, duże liczności, typy z ograniczeniami |
| różnicowe | `tools/compare` (osobny moduł, cgo) | ten sam zestaw W3C i korpus MF przez libxml2 (`goxsdval`); werdykty dla każdego przypadku w `out/compare/*.tsv`; od etapu 3 raport różnic względem `goxsdgo` i miejsca pierwszego błędu; każda różnica werdyktu w `differences.md` |
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

Pliki `testdata/w3c/passing.txt` i `testdata/passing-corpus.txt` zawierają identyfikatory przypadków, które już
przechodzą. Test kończy się błędem, gdy któryś z nich przestaje przechodzić. Nowe zaliczenia zapisuje
`go test ./xsd -update`, a zmiana pliku jest widoczna w przeglądzie. Przypadek usuwamy z listy tylko z wyjaśnieniem
w opisie commitu. Przypadek, którego `goxsdgo` jeszcze nie umie rozstrzygnąć (`errors.ErrUnsupported`), nie jest nigdy
zaliczony, także gdy oczekiwany werdykt to „błędny”.

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
| 2026-11-25 | `bb585b7` | 0 / 39 461 | schematy 0 / 170, dokumenty 0 / 8 | — | etap 0: infrastruktura bez implementacji; `go test ./xsd -run 'W3C\|Corpus' -v` |

Pomiary walidatora odniesienia (libxml2 2.15.4 przez `goxsdval` `2f45df6`, `tools/compare`):

| Data | W3C: zgodne z oczekiwanym / porównywalne / w zakresie | Korpus MF | Uwagi |
|---|---|---|---|
| 2026-11-25 | 38 669 / 39 252 / 39 461 (98,5% porównywalnych) | schematy 170 / 170, dokumenty 8 / 8 | według dostawców: NIST 23 170 / 23 170, Microsoft 13 675 / 14 198, Sun 1 587 / 1 610, Saxonica 98 / 130, W3C 92 / 92, IBM 44 / 49, Boeing 3 / 3 |
