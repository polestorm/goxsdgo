# Dziennik fuzzingu

Kryterium K5 (ADR 0001): fuzzery bez paniki i bez naruszenia niezmienników przez co najmniej 24 h łącznie na
kandydacie do wydania. Kampanię prowadzi `tools/fuzz` (ADR 0011); każdy przebieg dopisuje tu jeden wiersz.

```sh
go -C tools/fuzz run . -root ../.. -duration 24h            # pełna kampania: 7 fuzzerów po ok. 3,4 h
go -C tools/fuzz run . -root ../.. -duration 21h -rounds 3  # to samo w trzech rundach (np. noce)
go -C tools/fuzz run . -root ../.. -only Validate -duration 1h
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
