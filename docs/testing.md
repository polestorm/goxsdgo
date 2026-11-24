# Testy i porównania

## Warstwy

| Warstwa | Gdzie | Co sprawdza |
|---|---|---|
| zestaw W3C | `testdata/w3c/` (pytanie 1), `TestW3CSuite` | werdykt dla każdego schematu i dokumentu z zestawu XSD 1.0; raport w podziale na grupy i na rozdziały rekomendacji; lista wyłączeń z powodem |
| zgodność XML | `TestXMLConformance` | czytnik XML na zestawie W3C XML Conformance (dobra forma, przestrzenie nazw) |
| wymagania | `requirements/*.md`, `TestRequirementsCovered` | każde wymaganie ma test albo plik korpusu; każde odwołanie `REQ:` wskazuje istniejące wymaganie |
| korpus schematów | `testdata/schemas/`, `TestSchemaCorpus` | schematy MF (JPK, KSeF, e-Sprawozdania) z importami kompilują się bez błędów |
| korpus dokumentów | `testdata/instances/` + `expected.json`, `TestInstanceCorpus` | werdykt, kody i miejsca błędów (wiersz, kolumna, ścieżka) zgodne z oczekiwanymi; zmiana wzorca przez `go test -update` i przegląd różnicy |
| korpus własny | `testdata/corpus/` | małe schematy i dokumenty dla każdego wymagania, w tym polskie znaki, duże liczności, typy z ograniczeniami |
| różnicowe | `tools/compare` (osobny moduł, cgo) | ten sam korpus przez `goxsdgo` i `goxsdval`; raport różnic werdyktu i miejsca pierwszego błędu; każda różnica werdyktu w `differences.md` |
| bezpieczeństwo | `TestSafeByDefault` | brak sieci i encji zewnętrznych, limity głębokości i rozmiaru, schemat spoza `fs.FS` odrzucony |
| pamięć | `TestStreamingMemory` | syntetyczny JPK 1 GB (generator w teście, nie plik): szczyt pamięci < 64 MB |
| powtarzalność | `TestDeterministic` | wielokrotna walidacja daje te same błędy w tej samej kolejności |
| odporność | `FuzzCompile`, `FuzzValidate` | dowolny schemat i dokument: bez paniki, czas ograniczony |
| patologiczne | `TestPathological` | wzorce z zagnieżdżonymi powtórzeniami, `maxOccurs` w setkach tysięcy, głębokie zagnieżdżenia, tysiące kluczy: czas rośnie liniowo |
| wydajność | `BenchmarkCorpus` | porównanie z `goxsdval` (kryterium K7) |

## Pomiary

Wyniki zapisujemy tutaj z datą, wersją i sposobem pomiaru.

| Data | Commit | W3C (zaliczone / w zakresie) | Korpus MF | Zgodność z libxml2 | Uwagi |
|---|---|---|---|---|---|
| — | — | — | — | — | jeszcze bez kodu |
