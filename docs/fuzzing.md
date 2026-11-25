# Dziennik fuzzingu

Kryterium K5 (ADR 0001): fuzzery bez paniki i bez naruszenia niezmienników przez co najmniej 24 h łącznie na
kandydacie do wydania. Kampanię prowadzi `tools/fuzz` (ADR 0011); każdy przebieg dopisuje tu jeden wiersz.

**Decyzja właściciela z 2026-11-25:** fuzzing nie działa lokalnie w dzień, bo zajmuje procesory i przekłamuje czasy
innych sesji. Kampania działa na serwerze Synology z paczki bez narzędzi Go (niżej). Lokalnie najwyżej w nocy:

```sh
go -C tools/fuzz run . -root ../.. -duration 24h            # pełna kampania: 7 fuzzerów po ok. 3,4 h
go -C tools/fuzz run . -root ../.. -duration 21h -rounds 3  # to samo w trzech rundach (np. noce)
go -C tools/fuzz run . -root ../.. -only Validate -duration 1h
```

## Kampania na serwerze bez Go (Synology)

Paczkę buduje się na komputerze z Go, z czystego commitu kandydata:

```sh
go -C tools/fuzz run . -root ../.. -pack ../../out/fuzzpack   # out/fuzzpack/ i out/fuzzpack.tar.gz (ok. 42 MB)
go -C tools/fuzz run . -root ../.. -pack ../../out/fuzzpack -targets linux/amd64   # tylko jedna architektura
go -C tools/fuzz run . -root ../.. -pack ../../out/fuzzpack -data=false            # bez danych testowych
```

Paczka zawiera binarki testów fuzzerów z `go test -c` (`CGO_ENABLED=0`, statyczne) dla linux/amd64 i linux/arm64,
wejścia regresji `testdata/fuzz/*` pakietów oraz pobrane dane testowe, które fuzzery czytają jako wejścia startowe
(schematy i przykłady MF, zestaw W3C XSD, XML Conformance), w tym samym układzie katalogów co repozytorium. Danych
prywatnych w niej nie ma. Na serwerze (sh, BusyBox ash albo dash; bez bash):

```sh
tar -xzf fuzzpack.tar.gz && cd fuzzpack
WORK=/volume1/fuzz/work ./run.sh 1425 3   # 1425 min (23 h 45 min) łącznie, 3 workery; domyślnie: CPU - 1
```

`run.sh` uruchamia kolejno każdy fuzzer (`-test.fuzz`, `-test.fuzztime`, `-test.parallel`, `-test.fuzzcachedir`,
`-test.timeout 0`) w katalogu `tree/<pakiet>`, z równym podziałem czasu; dziennik `$WORK/journal.tsv` (data, commit,
pakiet, fuzzer, sekundy, wykonania, wynik), pełne wyjście w `$WORK/logs/`, korpus fuzzerów rośnie w `$WORK/cache/`
między uruchomieniami. Kończy się podsumowaniem i kodem wyjścia 1, gdy któryś fuzzer zawiódł; znalezione wejście
leży wtedy w `tree/<pakiet>/testdata/fuzz/<Fuzzer>/` i trzeba je przenieść do repozytorium jako test regresji.
Wiersze dziennika z serwera dopisuje tutaj:

```sh
go -C tools/fuzz run . -root ../.. -import /ścieżka/do/journal.tsv   # wynik z dopiskiem „(Synology)”
```

## Jak liczymy godziny do K5

- Liczy się czas przebiegów zakończonych wynikiem „ok” na jednym commicie kandydata albo na commitach późniejszych,
  które nie zmieniają kodu biblioteki poza testami (sprawdza to `git diff --stat <kandydat> -- ':!*_test.go'
  ':!testdata' ':!docs' ':!tools'` — pusty wynik).
- Wiersz z oznaczeniem „(zmiany)” to przebieg na kodzie z niezatwierdzonymi zmianami: nie liczy się do K5.
- Przebieg zakończony błędem zeruje licznik dla kodu, którego dotyczy: błąd naprawiamy, wejście zostaje w
  `testdata/fuzz/<Fuzzer>/` jako test regresji, a godziny liczymy od nowego commitu.
- Wejścia startowe pochodzą z pobranych danych testowych (schematy i przykłady MF, próbka zestawu W3C i XML
  Conformance); bez nich fuzzery startują z wejść wpisanych w kod. Danych prywatnych nie używamy.

## Fuzzery

| Fuzzer | Pakiet | Niezmienniki |
|---|---|---|
| `FuzzReader` | `internal/xmlreader` | brak paniki; czytanie się kończy; pozycje tokenów i błędów w granicach wejścia; błąd zawsze typu `*Error` |
| `FuzzRegexTranslate` | `internal/regex` | brak paniki; wynik tłumaczenia kompiluje się w RE2 albo daje `regex-too-large`; czas tłumaczenia ograniczony |
| `FuzzLexical` | `internal/datatypes` | postać kanoniczna należy do przestrzeni leksykalnej, oznacza tę samą wartość i jest stała |
| `FuzzDecimal` | `internal/datatypes` | `Decimal` zgodny z niezależnym wzorcem na `big.Rat`: przyjęcie literału, porządek, znak, postać kanoniczna, liczba cyfr, jednoznaczna reprezentacja |
| `FuzzContentModel` | `internal/schema` | automat z licznikami daje te same wyniki co niezależne dopasowanie wzorcowe |
| `FuzzCompile` | `xsd` | brak paniki; powtarzalność; każdy błąd schematu z kodem i pozycją w granicach dokumentu; czas ograniczony |
| `FuzzValidate` | `xsd` | brak paniki; powtarzalność; każdy błąd z kodem, komunikatem i pozycją w granicach dokumentu; czas ograniczony |

## Przebiegi

| Data | Commit | Pakiet | Fuzzer | Czas | Wykonania | Wynik |
|---|---|---|---|---|---|---|
| 2026-11-25 15:59 | `2e813e9` | ./internal/datatypes | `FuzzDecimal` | 15m3s | 107809772 | ok; przebieg przerwany po 15 min (decyzja właściciela), kampania przeniesiona na Synology |
| 2026-11-25 15:59 | `2e813e9` | ./internal/datatypes | `FuzzLexical` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `2e813e9` | ./internal/regex | `FuzzRegexTranslate` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `2e813e9` | ./internal/schema | `FuzzContentModel` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `2e813e9` | ./internal/xmlreader | `FuzzReader` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `2e813e9` | ./xsd | `FuzzCompile` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `2e813e9` | ./xsd | `FuzzValidate` | — | — | nieuruchomiony (kampania przerwana) |

Stan K5 (2026-11-25): na kandydacie `2e813e9` zaliczone 15 min (`FuzzDecimal`, bez błędów); do 24 h brakuje
23 h 45 min. Wcześniejsze przebiegi z etapów 1–5 (łącznie ok. 30 min, na innych commitach, poza dziennikiem) i 90 s `FuzzDecimal` na kodzie
przed commitem nie liczą się do K5.
