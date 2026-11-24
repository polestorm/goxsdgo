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
  go-libxml2, helium i podobne). Wolno uruchamiać je jako czarną skrzynkę, żeby porównać wynik;
- **zero zależności i zero cgo**: `go.mod` w katalogu głównym nie ma sekcji `require`; `CGO_ENABLED=0 go build ./...`
  musi przechodzić; porównanie z libxml2 żyje w osobnym module `tools/compare` i nie jest importowane przez bibliotekę;
- wzorcem są rekomendacje i oczekiwane wyniki zestawu W3C, nie zachowanie libxml2: gdy libxml2 różni się od
  rekomendacji, wygrywa rekomendacja, a różnicę wpisujemy do `docs/differences.md` (odbiorca musi wiedzieć, że plik
  dotąd przyjmowany teraz zostanie odrzucony, albo odwrotnie);
- **żadnych prawdziwych danych podatników ani klientów w korpusie**: tylko przykłady opublikowane przez MF i dokumenty
  syntetyczne (`testdata/README.md`);
- nazwy plików, pakietów i identyfikatorów po angielsku; komentarze i dokumentacja po polsku; `README.md` po angielsku;
  komunikaty błędów biblioteki po angielsku, ze stałym kodem do tłumaczenia przez wywołującego;
- każdy obsługiwany element rekomendacji ma wymaganie w `requirements/` i test, który je wskazuje;
- każda decyzja = plik w `docs/adr/`; pytania bez decyzji w `docs/adr/open-questions.md`;
- liczby (zaliczone przypadki, zgodność z odniesieniem) są datowane i podpisane sposobem pomiaru;
- opisy commitów po angielsku; przed commitem muszą przechodzić `gofmt`, `go vet ./...` i `go test ./...`.
