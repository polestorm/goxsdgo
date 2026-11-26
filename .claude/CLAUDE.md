# goxsdgo — instrukcje dla Claude Code

Projekt: walidator XML według XSD 1.0 w czystym Go. Cele: zgodność z rekomendacjami W3C XML Schema 1.0 (Second
Edition, części 1 i 2), zero cgo i zero zależności poza biblioteką standardową, walidacja strumieniowa w ograniczonej
pamięci, bezpieczny z konstrukcji (bez sieci, bez encji zewnętrznych), błędy ze stałym kodem, wierszem, kolumną
i ścieżką, powtarzalny wynik, czas liniowy także dla danych złośliwych. Projekt jest agnostyczny wobec odbiorców.

Zacznij od `README.md`, potem `docs/roadmap.md`, `docs/adr/0001-scope-and-acceptance.md`, `docs/provenance.md`
i `docs/adr/open-questions.md`.

## Po co i w jakim układzie (stan 2026-11-24)

- Dziś odbiorcy walidują przez `goxsdval` (`../controls/goxsdval`): cgo nad libxml2, z kodami błędów libxml2 i polskimi
  komunikatami po stronie odbiorcy (`../controls/goxsdval/messages/messages.json`, 117 kodów). cgo i libxml2 to
  dodatkowa pozycja SBOM i IQ, budowanie z tagiem, brak kompilacji krzyżowej.
- Odbiorcy: walidacja plików JPK, KSeF i e-Sprawozdań (duże pliki, schematy MF z importami), walidacja plików
  konfiguracyjnych opisanych własnym XSD (np. profil w QMS.Studio: walidacja XSD 1.0, a reguły semantyczne osobno).
- `goxsdgo` zastąpi `goxsdval` za interfejsem odbiorcy dopiero po spełnieniu **wszystkich** kryteriów K1–K9 (ADR 0001).
  Nie ma terminu, jest próg jakości.
- Rodzina: `../gopdfgo`, `../gomermaidgo`, `../gomdgo` powstają tak samo (od zera ze specyfikacji, czarna skrzynka jako
  odniesienie, korpus, wymagania, datowane pomiary). **Najbliższy wzór to `../gomdgo`**: jego `docs/testing.md`,
  `requirements/`, `tools/compare` jako osobny moduł, test pokrycia wymagań, pakiet dowodów. Kodu innych walidatorów
  XSD nie czytaj (`docs/provenance.md`).

## Kryteria zastąpienia (skrót ADR 0001)

| # | Kryterium | Gdzie mierzone |
|---|---|---|
| K1 | zestaw testów W3C XSD 1.0: 100% przypadków objętych zakresem, każde wyłączenie opisane w `docs/differences.md` | `TestW3CSuite` |
| K2 | korpus schematów MF: wszystkie kompilują się; werdykt (poprawny/błędny) dla każdego dokumentu korpusu zgodny z oczekiwanym | `TestSchemaCorpus`, `TestInstanceCorpus` |
| K3 | zgodność z libxml2: ten sam werdykt dla ≥ 99,5% par schemat–dokument; to samo miejsce pierwszego błędu (wiersz, element) dla ≥ 95% dokumentów błędnych; każda różnica w `docs/differences.md` | `tools/compare` (osobny moduł, cgo) |
| K4 | każde wymaganie z `requirements/` ma test; testy przechodzą | `TestRequirementsCovered` |
| K5 | fuzzing ≥ 24 h łącznie bez paniki; przypadki patologiczne (wzorce, liczności, zagnieżdżenia) w czasie liniowym | `FuzzValidate`, `FuzzCompile`, `TestPathological` |
| K6 | domyślnie bez sieci, bez encji zewnętrznych, z limitami głębokości i rozmiaru; pamięć przy walidacji strumieniowej ograniczona (plik 1 GB w < 64 MB) | `TestSafeByDefault`, `TestStreamingMemory` |
| K7 | nie wolniej niż 3× libxml2 na korpusie (duże pliki JPK) | `BenchmarkCorpus` |
| K8 | testy odbiorcy przechodzą po przepięciu; każdy kod `goxsdgo` ma odpowiednik w kodach libxml2 używanych przez odbiorcę (tabela `compat`) | u odbiorcy, etap 8 |
| K9 | pakiet dowodów: wymagania → testy → wynik ostatniego przebiegu | etap 8 |

Po każdym etapie dopisz wiersz do tabeli pomiarów w `docs/testing.md` (data, commit, liczby K1–K3) i zaktualizuj
kolumnę „Stan” w `docs/roadmap.md`.

## Jak pracujemy

- etapy w kolejności z `docs/roadmap.md`; **pierwszy krok: etap 0** (pobranie i przypięcie zestawu W3C, czytnik jego
  metadanych, test pokrycia wymagań, szkielet fuzzingu, `tools/compare` z własnym `go.mod` i zależnością od `goxsdval`
  z tagiem `libxml2`);
- przed etapem 0 decyzje właściciela w pytaniach 1 i 2 (`docs/adr/open-questions.md`); pozostałe pytania rozstrzygamy
  w etapach, w których wypadają, z rekomendacją;
- wymagania piszemy razem z etapem, w plikach według prefiksów z `requirements/README.md`;
- najpierw test (przypadek W3C albo plik korpusu), potem kod; liczba zaliczonych przypadków W3C nie spada między
  commitami bez wpisu, dlaczego;
- repozytorium `github.com/polestorm/goxsdgo` (prywatne); commit i push tylko na prośbę właściciela.

Zasady:
- **czysty rodowód** (`docs/provenance.md`): piszemy z rekomendacji W3C. Nie czytaj, nie kopiuj i nie przenoś kodu
  innych walidatorów XSD ani parserów XML z walidacją (libxml2, Xerces, Saxon, xmlschema dla Pythona, lxml,
  go-libxml2, helium, walidatory XSD w Go i podobne). Wolno uruchamiać je jako czarną skrzynkę, żeby porównać wynik.
  Po spełnieniu K1 i K2 przegląd kodu konkurencji według ADR 0002: czyta osobny agent, oddaje notatkę prozą bez kodu,
  wpis w `docs/review/`;
- **zero zależności i zero cgo**: `go.mod` w katalogu głównym nie ma sekcji `require`; `CGO_ENABLED=0 go build ./...`
  musi przechodzić; porównanie z libxml2 żyje w osobnym module `tools/compare` i nie jest importowane przez bibliotekę;
- wzorcem są rekomendacje i oczekiwane wyniki zestawu W3C, nie zachowanie libxml2: gdy libxml2 różni się od
  rekomendacji, wygrywa rekomendacja, a różnicę wpisujemy do `docs/differences.md` (odbiorca musi wiedzieć, że plik
  dotąd przyjmowany teraz zostanie odrzucony, albo odwrotnie);
- nowy plik danych z serwera MF albo w3.org dodajemy też do publicznego lustra `polestorm/goxsdgo-testdata` (`../goxsdgo-testdata`,
  commit lustra i drugi adres w `sources.json`), żeby pomiary dało się powtórzyć spoza Polski;
- dane testowe (zestawy W3C, schematy i przykłady MF) pobiera `go -C tools/fetch run . -root ../..` albo
  `python tools/fetch/fetch.py` według `testdata/sources.json`; nie trafiają do repozytorium (ADR 0003). Obie wersje
  narzędzia zmieniamy razem;
- zapadki `testdata/**/*.ratchet` (`docs/testing.md`): po zmianie, która dodaje zaliczenia,
  `go test ./xsd ./internal/datatypes ./internal/xmlreader -update` (flagę `-update` mają tylko pakiety z zapadkami); walidator odniesienia: `tools/compare` (polecenia w `docs/testing.md`);
- **żadnych prawdziwych danych podatników ani klientów w korpusie**: tylko przykłady opublikowane przez MF i dokumenty
  syntetyczne (`testdata/README.md`);
- nazwy plików, pakietów i identyfikatorów po angielsku; komentarze i dokumentacja po polsku; po angielsku: `README.md`,
  `docs/comparison.md`, `docs/contested-cases.md` (generuje `tools/bench`), `docs/provenance.md`, `docs/migrating-from-libxml2.md`, `docs/methodology.md` i `docs/results/` (strony publiczne) oraz komentarze dokumentacyjne publicznego API
  (opis pakietu, eksportowane typy, funkcje, metody, stałe, zmienne i pola struktur w `xsd/`), bo
  pokazuje je pkg.go.dev (decyzja właściciela z 2026-11-25, jak w gomdgo); komentarze wewnętrzne i pakiety
  `internal/` zostają po polsku;
  komunikaty błędów biblioteki po angielsku, ze stałym kodem do tłumaczenia przez wywołującego;
- każdy obsługiwany element rekomendacji ma wymaganie w `requirements/` i test, który je wskazuje;
- każda decyzja = plik w `docs/adr/`; pytania bez decyzji w `docs/adr/open-questions.md`;
- liczby (zaliczone przypadki, zgodność z odniesieniem) są datowane i podpisane sposobem pomiaru;
- opisy commitów po angielsku; przed commitem muszą przechodzić `gofmt`, `go vet ./...` i `go test ./...`;
- opis commita kończy się jedną linią `Made with the help of Claude Code` (po pustym wierszu); nie dopisuj
  `Co-Authored-By` ani `Claude-Session` (decyzja właściciela z 2026-11-25, jak w gomdgo, gopdfgo i gomermaidgo;
  tego dnia przepisano opisy wszystkich commitów, więc ich identyfikatory się zmieniły);
- **porównania z konkurencją są powtarzalne** (reguła globalna): każda liczba w README i `docs/comparison.md` ma
  dane wejściowe z `tools/fetch` albo generatora, polecenie w `docs/methodology.md` i surowy wynik w `docs/results/`
  (`go -C tools/bench run . -root ../.. publish`); żadnej liczby z korpusu prywatnego w porównaniu;
- **komunikaty dla ludzi (EN/PL) są w prywatnym `../goxsdgo-messages`** (`github.com/polestorm/goxsdgo-messages`,
  wydzielone 2026-11-26 z historią); polskich tekstów komunikatów nie dopisujemy do tego repozytorium ani do jego
  dokumentów (decyzja właściciela). Nowy albo zmieniony kod w `xsd/codes.go` wymaga wpisu
  w `catalog.json` tamtego modułu w każdym języku, zanim odbiorca podbije zależność;
- w katalogu głównym trzymamy tylko `README.md`, `LICENSE`, `NOTICE`, `go.mod` (i `.gitignore`); ten plik leży
  w `.claude/CLAUDE.md`.
