# Pakiet dowodów

Kryterium K9 (ADR 0001): pakiet dowodów do walidacji u odbiorcy, czyli wymagania → testy → wynik ostatniego
przebiegu, z tabelą K1–K9 i źródłem każdej liczby. Buduje go `tools/evidence` (osobny moduł bez zależności, bo
uruchamia `go test`, `git` i `python3`; biblioteka go nie importuje). Wymagania: `requirements/evidence.md` (EVD).

## Jak zbudować

Z katalogu głównego repozytorium, na czystym commicie (pakiet z niezatwierdzonymi zmianami ma w nazwie `-dirty`):

```sh
go -C tools/fetch run . -root ../.. -from ../xml.pro/internal/importing/schemas   # dane testowe (ADR 0003)
go -C tools/evidence run . -root ../..                    # out/evidence/<data>-<commit>/
go -C tools/evidence run . -root ../.. -tar               # także out/evidence/<data>-<commit>.tar.gz
go -C tools/evidence run . -root ../.. -stream-mb 1024    # K6 zmierzone w tym przebiegu na pliku 1 GB (ok. 1 min)
```

Przebieg trwa tyle co `go test ./...` (ok. 1 min). Narzędzie nie sięga do sieci, nie uruchamia fuzzingu ani
libxml2 i nie czyta korpusu prywatnego: przebiegi testów dostają środowisko bez `GOXSDGO_PRIVATE_CORPUS`,
`GOXSDGO_DUMP_ERRORS` i (poza `-stream-mb`) `GOXSDGO_STREAM_MB`. Kończy się kodem 1, gdy K4 albo K9 nie są
spełnione; pakiet zapisuje także wtedy.

## Co zawiera

| Plik | Treść |
|---|---|
| `report.md` | raport po polsku: tabela K1–K9 (wartość, cel, stan, źródło), przebiegi testów, macierz śledzenia, zestaw W3C według dostawców i niezaliczone przypadki, zapadki, dziennik fuzzingu, dane testowe z sumami, ADR, sekcje `differences.md` |
| `evidence.json` | to samo w postaci maszynowej |
| `go-test-module.json`, `go-test-fetch.json`, `go-test-evidence.json` | surowy wynik `go test -count=1 -json ./...` modułu głównego, `tools/fetch` i `tools/evidence` |
| `python-fetch.txt` | wynik testów `tools/fetch` w Pythonie |
| `fetch-verify.txt` | wynik `tools/fetch -verify`: czy pobrane dane mają sumy z `testdata/sources.json` |

`report.md` i `evidence.json` są powtarzalne: dla tego samego commitu i tych samych danych różnią się tylko datą
wygenerowania. Czasy przebiegów są wyłącznie w plikach surowych.

## Skąd liczby

| Kryterium | Źródło w pakiecie |
|---|---|
| K1 | wyjście `TestW3CSuite`; niezaliczone przypadki z `testdata/w3c/xsdtests.ratchet` zestawione z ich wzmianką w `docs/differences.md`; wyłączenia z powodami (`internal/w3c.ExcludedGroups`) |
| K2 | wyjście `TestSchemaCorpus` i `TestInstanceCorpus` (bez kompletu danych MF: „oczekuje”) |
| K3 | `docs/measurements.json` (pomiar `tools/compare` z datą i commitem) i nagłówek `docs/differences-libxml2.md` (liczba rozbieżności, ile bez kategorii) |
| K4 | macierz śledzenia z `requirements/*.md` i odwołań `REQ:` w testach, wynik `TestRequirementsCovered` |
| K5 | dziennik `docs/fuzzing.md`: przebiegi „ok” bez „(zmiany)” na kandydacie z `docs/measurements.json` albo na commitach, które nie zmieniają kodu biblioteki (`git diff --quiet <kandydat> <commit> -- . ':!*_test.go' ':!*testdata/*' ':!docs' ':!*.md' ':!tools' ':!.github' ':!.claude'`); wynik `TestPathological` w `xsd` i `internal/schema` |
| K6 | wynik `TestSafeByDefault` i `TestStreamingMemory`; szczyt pamięci dla 1 GB z `docs/measurements.json` albo z tego przebiegu (`-stream-mb 1024`) |
| K7 | `docs/measurements.json` (ilorazy czasu względem libxml2) |
| K8 | `docs/measurements.json` (`k8`: testy odbiorcy po przepięciu) i wynik `TestCompatTable` |
| K9 | sam pakiet: wszystkie przebiegi zielone, macierz bez braków, `tools/fetch -verify` bez błędów |

Stan: **spełnione**, **niespełnione** albo **oczekuje** (brak danych, pomiar w toku, decyzja do podjęcia).

## Jak czytać macierz śledzenia

Dla każdego wymagania: testy, które się na nie powołują (`pakiet.Test`), i wynik.

- **przeszedł**: wszystkie testy przeszły;
- **na żądanie**: dowodem jest benchmark albo test uruchamiany zmienną `GOXSDGO_*` (np. `TestDumpErrors`,
  `BenchmarkCorpus`); wynik takiego pomiaru jest w `docs/testing.md` albo w `docs/measurements.json`;
- **pominięty**: test pominięty z braku danych (np. bez pobranego zestawu W3C);
- **nieprzypisany**: wymaganie ma tylko odwołanie przy funkcji pomocniczej albo test nie wystąpił w przebiegu;
- **brak testu**, **nie przeszedł**: niezgodność.

Odwołanie przy funkcji pomocniczej testów (np. `checkObservedCodes`, wołanej przez testy zgodności) jest dowodem
wspierającym: wypisanym, ale niewystarczającym bez prawdziwego testu.

## Jak uzupełnić później

- **K5, po kampanii fuzzingu na Synology** (`docs/fuzzing.md`): skopiować `journal.tsv` z serwera, dopisać go
  poleceniem `go -C tools/fuzz run . -root ../.. -import journal.tsv`, zacommitować i zbudować pakiet od nowa. Godziny
  przeliczą się same.
- **K8, po testach odbiorcy**: w `docs/measurements.json`, w sekcji `k8`, wpisać gałąź, commit, datę, wynik (np.
  `go test ./...: 1234 testów, 0 błędów`) i `status` (`met` albo `not_met`), zacommitować i zbudować pakiet od nowa.
- **K3, K6, K7, po nowym pomiarze**: zmienić wartości, datę i commit w `docs/measurements.json` (i wiersz w
  `docs/testing.md`).
- **K1**: każdy niezaliczony przypadek W3C musi być nazwany w `docs/differences.md` (grupa, nazwa testu albo
  zakres, np. `xv002–xv100`); pakiet wypisuje te, których nie znalazł.
