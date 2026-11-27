# Wymagania

Każde wymaganie ma identyfikator `PREFIKS-nnn`. Testy i pliki korpusu odwołują się do nich przez `REQ: PREFIKS-nnn`;
test `TestRequirementsCovered` sprawdza pokrycie w obie strony.
Wymaganie delegowane („Delegowane: goxmlgo vX.Y.Z, `requirements/plik.md` ID…”) jest spełnione w module goxmlgo; test
sprawdza, że wersja zgadza się z `go.mod`, a wskazane wymaganie istnieje w tej wersji goxmlgo.

| Prefiks | Plik | Zakres |
|---|---|---|
| `XML` | `xml.md` | czytnik XML: wymagania delegowane do goxmlgo i wymagania goxsdgo wobec czytnika |
| `DT` | `datatypes.md` | typy wbudowane, aspekty, listy, unie (część 2) |
| `RE` | `regex.md` | wyrażenia regularne XSD (część 2, dodatek F) |
| `SCH` | `schema.md` | wczytanie schematu, komponenty, `include`/`import`/`redefine`, ograniczenia poprawności schematu |
| `CM` | `content.md` | typy złożone, modele zawartości, UPA, EDC, wyprowadzenia, grupy podstawienia |
| `VAL` | `validation.md` | ocena dokumentu, `xsi:*`, wartości domyślne, ograniczenia tożsamości |
| `ERR` | `errors.md` | kody, pozycje, ścieżki, parametry, tabela `compat`, kolejność |
| `SAFE` | `safety.md` | brak sieci i encji zewnętrznych, limity |
| `DET` | `determinism.md` | powtarzalność, czas liniowy, pamięć przy strumieniu |
| `EVD` | `evidence.md` | pakiet dowodów (K9): przebiegi testów, macierz śledzenia, tabela K1–K9, powtarzalność (`tools/evidence`) |

Wymaganie dla rekomendacji wskazuje rozdział i regułę (np. „cvc-complex-type.2.4”, „src-resolve”), a nie pojedynczy
przypadek testowy; przypadki W3C są testami tego wymagania. Nazwy reguł z rekomendacji są też podstawą kodów błędów.
Pliki powstają razem z etapami w `../docs/roadmap.md`.
