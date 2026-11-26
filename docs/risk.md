# Analiza ryzyka funkcji

Ryzyka goxsdgo jako biblioteki, niezależnie od odbiorcy: co może pójść źle, jak to ograniczamy i czym to sprawdzamy.
**Klasy ryzyka (wysoka, średnia, niska) nadaje odbiorca** według swojego przeznaczenia: ten sam błąd walidatora ma
inną wagę przy walidacji profilu sterującego blokadami niż przy podpowiedzi w edytorze. Odbiorca przenosi poniższe
wiersze do swojej analizy (np. QMS: `reuse-inventory.md`, `module-validation.md` pkt 12) i dopisuje klasę.

Przegląd analizy: przy każdej zmianie klasy 2 i 3 w [CHANGELOG.md](../CHANGELOG.md) i raz na wydanie.

| # | Ryzyko | Skutek | Ograniczenie | Weryfikacja |
|---|---|---|---|---|
| R1 | **Fałszywe przyjęcie**: dokument niezgodny ze schematem uznany za poprawny | błędne dane przechodzą dalej | implementacja z rekomendacji, każda reguła z wymaganiem; zestaw W3C (39 369 / 39 439); porównanie z libxml2 z opisem każdej różnicy | `TestW3CSuite`, `TestInstanceCorpus`, `tools/compare`; VAL-*, CM-*, DT-*, RE-* |
| R2 | **Fałszywe odrzucenie**: dokument poprawny odrzucony | zatrzymanie procesu, obejścia u użytkownika | jak R1; różnice z libxml2 i zestawem opisane (KI-1, KI-2) | jak R1; `docs/differences.md` |
| R3 | **Schemat błędny przyjęty albo poprawny odrzucony** | walidacja według innych reguł, niż autor schematu chciał | ograniczenia komponentów i reprezentacji z rekomendacji; korpus MF | testy schematów W3C, `TestSchemaCorpus`; SCH-* |
| R4 | **Złe miejsce lub opis błędu** (wiersz, kolumna, ścieżka, kod) | użytkownik poprawia nie to miejsce | pozycje z czytnika, kody stałe; zgodność wiersza pierwszego błędu z libxml2 99,7% | `tools/compare`, testy błędów; ERR-*, XML-014 |
| R5 | **Odmowa usługi**: złośliwy schemat albo dokument wyczerpuje czas lub pamięć | zawieszenie usługi | limity domyślne, czas liniowy wzorców i liczników, walidacja strumieniowa | `TestPathological`, `TestSafeByDefault`, `TestStreamingMemory`, fuzzing (K5, KI-4); SAFE-*, XML-016, XML-017 |
| R6 | **Dostęp do sieci lub plików** wymuszony przez dokument (XXE, SSRF) | wyciek danych, ataki z serwera | brak sieci i encji zewnętrznych w `xsd` i `xmlreader`; wskazówki `xsi:schemaLocation` domyślnie wyłączone i tylko z `fs.FS`; sieć wyłącznie w `xsd/resolve` z `Network` i `AllowHosts` | `TestSafeByDefault`, `TestResolveOrder`, `TestDirCacheNetwork`; SAFE-*, SCH-004, SCH-022, SCH-024, VAL-014 |
| R7 | **Schemat z innego źródła, niż zakładano** (podmieniony plik w pamięci podręcznej, zmiana na serwerze) | walidacja według innego schematu | `Set.Documents()` z SHA-256 każdego dokumentu; `Cache`/`Dir` pod kontrolą wersji; błąd pobierania przerywa kompilację | `TestResolveRecursive`, `TestDirCacheNetwork`; SCH-023, SCH-024; KI-9 |
| R8 | **Niepowtarzalny wynik** (kolejność błędów, zależność od mapy, równoległości) | różne wyniki tego samego pliku, trudny audyt | deterministyczna kolejność, testy powtarzalności | DET-*; testy determinizmu |
| R9 | **Brak werdyktu potraktowany jak werdykt** (`ErrLimit`, `errors.ErrUnsupported`) | dokument niezwalidowany uznany za poprawny | te błędy są drugim wynikiem, nigdy `Error.Code`; dokumentacja API | testy limitów; KI-3; ERR-* |
| R10 | **Zmiana zachowania przy aktualizacji wersji** | regresja u odbiorcy | CHANGELOG z klasą wpływu i ID wymagań; zapadki liczby zaliczonych przypadków; podpisane tagi od v0.4.0 | zapadki `testdata/**/*.ratchet`, CI, artefakty wydania |
| R11 | **Panika w bibliotece** na nietypowym wejściu | przerwanie programu odbiorcy | fuzzing czytnika, kompilacji, walidacji, wzorców | `Fuzz*`, K5 (KI-4) |
| R12 | **Podatność w zależnościach** | luka bezpieczeństwa | brak zależności poza biblioteką standardową; `govulncheck` przy każdym wydaniu | artefakt `govulncheck` w wydaniu |

Prefiksy wymagań: `requirements/README.md`. Stan kryteriów K1–K9: [evidence.md](evidence.md).
