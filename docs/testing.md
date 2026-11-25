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
| różnicowe | `tools/compare` (osobny moduł, cgo) | ten sam zestaw W3C i korpus MF przez libxml2 (`goxsdval`); werdykty dla każdego przypadku w `out/compare/*.tsv`, od etapu 3 także rozbieżności testów schematów goxsdgo i libxml2 (`out/compare/schemas-goxsdgo-libxml2.tsv`), od etapu 5 testów dokumentów i korpusu z wierszem pierwszego błędu (`out/compare/instances-goxsdgo-libxml2.tsv`), od etapu 6 pary kodów (`code-pairs-*.tsv`, polecenia `probe` i `compat`, `compat.tsv`) i wygenerowana lista rozbieżności `docs/differences-libxml2.md` (`-docs w3c`); od etapu 3 raport różnic względem `goxsdgo` i miejsca pierwszego błędu; każda różnica werdyktu w `differences.md` |
| katalog i compat | `TestCatalogMatchesSource`, `TestCompatTable`, `TestErrorsDoc` (od etapu 6) | każdy kod emitowany w źródłach jest w katalogu `xsd/codes.go` i odwrotnie; każdy zwrócony błąd ma kod z katalogu i właściwe pola; każdy kod ma odpowiednik w kodach libxml2 (K8); `docs/errors.md` zgodny z katalogiem |
| komunikaty | `messages`: `TestCatalogComplete`, `TestGoldenAgainstXMLPro`, `TestObservedErrorsRender` (od etapu 6b) | każdy kod ma tytuł i podpowiedź w każdym języku; polskie teksty dla przypadków z diagnozy xml.pro zgodne z xml.pro (`messages/testdata/golden.txt`, `-update`); każdy błąd zaobserwowany na W3C i korpusie MF opisany w EN i PL bez nierozwiniętych parametrów |
| porównanie z innymi walidatorami | `tools/bench` (od 2026-11-25, osobny moduł; wynik w README, „Comparison”) | goxsdgo, knroy/go-xml v2.3.1, jacoelho/xsd v0.6.15 i cB-Sigma/xsd v1.0.0 jako czarne skrzynki przez publiczne API (bez czytania ich kodu, ADR 0002); ten sam harness i reguły zestawu W3C co `TestW3CSuite`, korpus MF, rozmiar programu i liczba linii, czas i pamięć na syntetycznym JPK (osobne procesy), próby bezpieczeństwa z ustawieniami domyślnymi; libxml2 z `tools/compare`; wyniki w `out/bench/` |
| pakiet dowodów | `tools/evidence` (od etapu 8), `docs/evidence.md`, `docs/measurements.json` | K9: wynik ostatniego przebiegu testów, macierz wymaganie → test → wynik, tabela K1–K9 ze źródłami, sumy danych testowych; `go -C tools/evidence run . -root ../..` |
| bezpieczeństwo | `TestSafeByDefault` (od etapu 5) | brak sieci i encji zewnętrznych, limity głębokości i rozmiaru, schemat spoza `fs.FS` odrzucony |
| pamięć | `TestStreamingMemory` (od etapu 5) | syntetyczny JPK generowany w teście, nie plik: 200 MB w zwykłym przebiegu, 1 GB z `GOXSDGO_STREAM_MB=1024`; szczyt pamięci < 64 MB |
| powtarzalność | `TestDeterministic` (od etapu 5, także walidacja współbieżna) | wielokrotna walidacja daje te same błędy w tej samej kolejności |
| odporność | `FuzzCompile`, `FuzzValidate`, `FuzzContentModel` (automat porównywany z niezależnym, prostym dopasowaniem), `FuzzReader`, `FuzzRegexTranslate`, `FuzzLexical`, `FuzzDecimal`; kampania `tools/fuzz` (paczka dla serwera bez Go: `-pack`, dziennik: `-import`), dziennik godzin w `docs/fuzzing.md`; fuzzing według reguły z globalnego `CLAUDE.md` właściciela (Synology, lokalnie tylko w nocy na zasilaniu sieciowym) | dowolny schemat i dokument: bez paniki, czas ograniczony |
| patologiczne | `TestPathological` (`internal/schema`: modele zawartości; `xsd`: wzorce, liczności, zagnieżdżenia, długie atrybuty i teksty, klucze, ID, wyliczenia, listy, symbole wieloznaczne, encje; iloraz czasów 2n/n ≤ 3,2; wejścia ponad limitem kończą się `ErrLimit`) | wzorce z zagnieżdżonymi powtórzeniami, `maxOccurs` w setkach tysięcy, głębokie zagnieżdżenia, tysiące kluczy: czas rośnie liniowo |
| wydajność | `BenchmarkValidateJPK`, `BenchmarkCorpus`, `tools/compare private` i `timefile` | porównanie z libxml2 przez `goxsdval` (K7); korpus prywatny z `GOXSDGO_PRIVATE_CORPUS`, wyłącznie liczby zbiorcze |
| bez zmian wyników | `TestDumpErrors` (`GOXSDGO_DUMP_ERRORS`) | zrzut wszystkich błędów zestawu W3C i korpusu przed i po optymalizacji; różnica musi być pusta |

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
| 2026-11-25 | `355ec91` (`tools/bench`) | W3C z 39 439: goxsdgo 39 367, knroy/go-xml 39 386, cB-Sigma/xsd 39 329, jacoelho/xsd 39 083, libxml2 38 662 | MF: 170 / 170 (jacoelho/xsd 168) | — | 200 MB JPK: goxsdgo 3,96 s, libxml2 3,85 s, jacoelho/xsd 3,79 s; knroy/go-xml i cB-Sigma/xsd budują drzewo (11× i 22× rozmiaru pliku w pamięci); pojedyncze przebiegi przy obciążeniu 2–3 |
| 2026-11-25 | `50eaebc` + xml.pro `4c07f5b5` | bez zmian | bez zmian | xml.pro przez silnik goxsdgo: 172 pliki klientów (1,61 GB), ten sam status i werdykt 172 / 172, wiersz pierwszego błędu 50 / 51 | etap 8, K8: testy xml.pro 2029 pass / 1 fail (ta sama porażka niezwiązana z walidacją co na `origin/main`) w obu silnikach, `-tags libxml2` 1103 / 0; czas 1,03× (200 MB) i 1,09× (korpus) względem gomfxml |
| 2026-11-25 | `2e813e9` | 39 367 / 39 439 (bez zmian) | 170 / 170, 15 / 15 | korpus prywatny (151 plików z schematem, 1,59 GB): werdykt 151 / 151, wiersz pierwszego błędu 50 / 51 | etap 7; K6: 1 GB, szczyt 6,3 MB; K7: 200 MB 1,05×, korpus prywatny 1,19× (największy plik 227 MB: 1,33×); `BenchmarkValidateJPK` 47–48 MB/s (było 41); K5: 15 min / 24 h (`docs/fuzzing.md`) |
| 2026-11-25 | `12a5436` | bez zmian | bez zmian | bez zmian | etap 6b; 184 kombinacje kod/wariant zaobserwowane na W3C i korpusie MF opisane w EN i PL; `BenchmarkValidateJPK` ok. 41 MB/s, bez zmian |
| 2026-11-25 | `a497d5b` | 39 367 / 39 439 (bez zmian) | 170 / 170, 15 / 15 | bez zmian; 528 rozbieżności werdyktu z libxml2 opisanych w `differences-libxml2.md` (rację ma goxsdgo w 519, libxml2 w 9) | etap 6: compat 223 / 223 kodów; 52 ze 117 kodów odbiorcy są odpowiednikiem kodu dokumentu; `BenchmarkValidateJPK` bez `OnValue` 41 MB/s (bez zmian), z `OnValue` 33 MB/s |
| 2026-11-25 | `8f02159` | 39 367 / 39 439 (testy schematów 14 370 / 14 384; testy dokumentów 24 997 / 25 055: poprawne 14 016 / 14 040, błędne 10 981 / 11 015) | schematy 170 / 170, dokumenty 15 / 15 (8 MF i 7 syntetycznych, z kodem, wierszem i ścieżką) | testy dokumentów: ten sam werdykt w 99,69% z 24 903 porównywalnych, ten sam wiersz pierwszego błędu w 99,7% z 10 912; korpus: werdykt 15 / 15, wiersz 6 / 6 | etap 5; K6: plik 1 GB, szczyt pamięci 5,8 MB; K7: 200 MB w 5,05 s wobec 3,87 s `xmllint --stream` (1,30×); `BenchmarkValidateJPK` 41,9 MB/s |
| 2026-11-25 | `0a44da1` | 14 370 / 39 439 (testy schematów 14 370 / 14 384: poprawne 11 036 / 11 044, błędne 3 334 / 3 340; testy dokumentów 0 / 25 055) | schematy 170 / 170, dokumenty 0 / 8 | testy schematów: ten sam werdykt co libxml2 w 96,8% z 14 307 porównywalnych; z 452 rozbieżności rację ma goxsdgo w 447, libxml2 w 5 | etap 4; `particlesZ001` przestaje przechodzić (ADR 0007, `differences.md`) |
| 2026-11-25 | wyłączenie 9 grup XSD 1.1 bez żetonu | 14 048 / 39 439 (testy schematów 14 048 / 14 384) | bez zmian | bez zmian | 22 przypadki wyłączone z powodem (`internal/w3c.ExcludedGroups`, `differences.md`) |
| 2026-11-25 | `0546663` | 14 048 / 39 461 (testy schematów 14 048 / 14 393: poprawne 11 024 / 11 044, błędne 3 024 / 3 349; testy dokumentów 0 / 25 068, etap 5) | schematy 170 / 170, dokumenty 0 / 8 | testy schematów: ten sam werdykt co libxml2 w 98,6% z 14 316 porównywalnych; z 196 rozbieżności rację ma goxsdgo w 158, libxml2 w 38 | etap 3; 324 z 325 niezaliczonych „błędnych” czeka na kontrole z etapu 4 (265 MS-Particles, 24 MS-ModelGroups, reszta UPA, EDC, redefinicje grup) |
| 2026-11-25 | `892eade` | 0 / 39 461 | — | — | etap 2, pomiar pośredni (adapter, nie K1): NIST 22 678 / 22 678 odczytanych z 23 170 w zakresie (nieodczytane: 492 przypadki ID z typem złożonym), MS-Regex 2 772 / 2 789 z 3 998, MS-DataTypes 29 / 29 z 3 430; 17 rozbieżności opisanych w `differences.md`; `go test ./internal/datatypes -run W3CDatatypes -v` |
| 2026-11-25 | `b502002` | 0 / 39 461 | — | — | etap 1: XML Conformance 1725 / 1727 (xmltest 299/299, sun 101/101, oasis 323/323, ibm 526/527, eduni 476/477); `encoding/xml` na tym samym zestawie 1146 / 1727; werdykt dobrej formy jak w libxml2 dla 42 018 plików `.xml` i `.xsd` z zestawu W3C XSD i korpusu MF poza 3; `go test ./internal/xmlreader -run XMLConformance -v` |
| 2026-11-25 | `bb585b7` | 0 / 39 461 | schematy 0 / 170, dokumenty 0 / 8 | — | etap 0: infrastruktura bez implementacji; `go test ./xsd -run 'W3C\|Corpus' -v` |

Pomiary walidatora odniesienia (libxml2 2.15.4 przez `goxsdval` `2f45df6`, `tools/compare`):

| Data | W3C: zgodne z oczekiwanym / porównywalne / w zakresie | Korpus MF | Uwagi |
|---|---|---|---|
| 2026-11-25 | 38 669 / 39 252 / 39 461 (98,5% porównywalnych) | schematy 170 / 170, dokumenty 8 / 8 | według dostawców: NIST 23 170 / 23 170, Microsoft 13 675 / 14 198, Sun 1 587 / 1 610, Saxonica 98 / 130, W3C 92 / 92, IBM 44 / 49, Boeing 3 / 3 |
