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
  które nie zmieniają kodu biblioteki poza testami (sprawdza to `git diff --stat <kandydat> -- . ':!*_test.go'
  ':!*testdata/*' ':!docs' ':!*.md' ':!tools' ':!.github' ':!.claude'` — pusty wynik; od 2026-11-26 także dane
  testowe w każdym katalogu `testdata`, pliki `.md`, CI i ustawienia Claude).
- Wiersz z oznaczeniem „(zmiany)” to przebieg na kodzie z niezatwierdzonymi zmianami: nie liczy się do K5.
- Przebieg zakończony błędem zeruje licznik dla kodu, którego dotyczy: błąd naprawiamy, wejście zostaje w
  `testdata/fuzz/<Fuzzer>/` jako test regresji, a godziny liczymy od nowego commitu.
- Wejścia startowe pochodzą z pobranych danych testowych (schematy i przykłady MF, próbka zestawu W3C i XML
  Conformance); bez nich fuzzery startują z wejść wpisanych w kod. Danych prywatnych nie używamy.

## Fuzzery

| Fuzzer | Pakiet | Niezmienniki |
|---|---|---|
| `FuzzReader` | `xmlreader` (do 2026-11-26 `internal/xmlreader`) | brak paniki; czytanie się kończy; pozycje tokenów i błędów w granicach wejścia; błąd zawsze typu `*Error` |
| `FuzzRegexTranslate` | `internal/regex` | brak paniki; wynik tłumaczenia kompiluje się w RE2 albo daje `regex-too-large`; czas tłumaczenia ograniczony |
| `FuzzLexical` | `internal/datatypes` | postać kanoniczna należy do przestrzeni leksykalnej, oznacza tę samą wartość i jest stała |
| `FuzzDecimal` | `internal/datatypes` | `Decimal` zgodny z niezależnym wzorcem na `big.Rat`: przyjęcie literału, porządek, znak, postać kanoniczna, liczba cyfr, jednoznaczna reprezentacja |
| `FuzzContentModel` | `internal/schema` | automat z licznikami daje te same wyniki co niezależne dopasowanie wzorcowe |
| `FuzzCompile` | `xsd` | brak paniki; powtarzalność; każdy błąd schematu z kodem i pozycją w granicach dokumentu; czas ograniczony |
| `FuzzValidate` | `xsd` | brak paniki; powtarzalność; każdy błąd z kodem, komunikatem i pozycją w granicach dokumentu; czas ograniczony |

## Przebiegi

| Data | Commit | Pakiet | Fuzzer | Czas | Wykonania | Wynik |
|---|---|---|---|---|---|---|
| 2026-11-25 15:59 | `51b65ae` | ./internal/datatypes | `FuzzDecimal` | 15m3s | 107809772 | ok; przebieg przerwany po 15 min (decyzja właściciela), kampania przeniesiona na Synology |
| 2026-11-25 15:59 | `51b65ae` | ./internal/datatypes | `FuzzLexical` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `51b65ae` | ./internal/regex | `FuzzRegexTranslate` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `51b65ae` | ./internal/schema | `FuzzContentModel` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `51b65ae` | ./internal/xmlreader | `FuzzReader` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `51b65ae` | ./xsd | `FuzzCompile` | — | — | nieuruchomiony (kampania przerwana) |
| 2026-11-25 15:59 | `51b65ae` | ./xsd | `FuzzValidate` | — | — | nieuruchomiony (kampania przerwana) |

Stan K5 (2026-11-25): na kandydacie `51b65ae` zaliczone 15 min (`FuzzDecimal`, bez błędów); do 24 h brakuje
23 h 45 min. Wcześniejsze przebiegi z etapów 1–5 (łącznie ok. 30 min, na innych commitach, poza dziennikiem) i 90 s `FuzzDecimal` na kodzie
przed commitem nie liczą się do K5.

## Zlecenie na Synology (2026-11-25)

Paczka z `d8c4e60` (kod biblioteki jak w kandydacie `51b65ae`) wgrana do `~/fuzz/goxsdgo/7aa29c4/` na Synology (katalog nazwany pierwotnym identyfikatorem)
„StormSyn” (DS224+, x86_64, 4 rdzenie Celeron J4125) i ustawiona 2026-11-25 16:22 we wspólnej kolejce `~/fuzz/queue.sh`:
1425 min (23 h 45 min brakujące do K5), 3 workery, `nice -n 19`. Ziarna wszystkich 7 fuzzerów przechodzą na serwerze.
Kampania ruszy po zakończeniu trwającego fuzzingu `gomdgo`. Po zakończeniu: skopiować
`~/fuzz/goxsdgo/7aa29c4/fuzzpack/work/journal.tsv` i dopisać `go -C tools/fuzz run . -root ../.. -import journal.tsv`;
wejścia łamiące niezmiennik leżą w `fuzzpack/tree/<pakiet>/testdata/fuzz/`.

## Identyfikatory po przepisaniu historii (2026-11-25)

Tego dnia przepisano opisy wszystkich commitów (jedna linia `Made with the help of Claude Code`), więc ich
identyfikatory się zmieniły; zawartość plików bez zmian. Kandydat K5 `2e813e9` to teraz `59d6910`, a paczka
`7aa29c4` to teraz `4106d3f`. Paczka na Synology ma wpisane stare identyfikatory (katalog `~/fuzz/goxsdgo/7aa29c4/`,
kolumna commit w `journal.tsv`): przed `-import` zamień w dzienniku `7aa29c4` na `4106d3f`
(`sed -i '' 's/\t7aa29c4\t/\t4106d3f\t/' journal.tsv`), żeby pakiet dowodów zaliczył godziny do kandydata.
Stara historia: kopia lustrzana `../goxsdgo-backup-2026-11-25.git`.

## Identyfikatory po przepisaniu historii (2026-11-26)

Tego dnia z historii usunięto pakiet `messages` (wydzielony do osobnego modułu, ADR 0010), więc identyfikatory
commitów od etapu 6b znów się zmieniły; kod pakietów objętych fuzzingiem bez zmian. Kandydat K5 `59d6910` to teraz
`51b65ae`, paczka `4106d3f` to teraz `d8c4e60`. Przed `-import` zamień w dzienniku z Synology `7aa29c4` (albo
`4106d3f`) na `d8c4e60`: `sed -i '' 's/\t7aa29c4\t/\td8c4e60\t/; s/\t4106d3f\t/\td8c4e60\t/' journal.tsv`.
Pełne przypisanie starych identyfikatorów do nowych: `git log` w kopii `goxsdgo-backup-2026-11-26.git` obok
repozytorium (te same opisy commitów, w tej samej kolejności).

## Wynik kampanii na Synology (2026-11-25 19:57 – 2026-11-26 13:04)

Paczka z `d8c4e60` (kod biblioteki jak w `51b65ae`), 7 fuzzerów po 203 min, 3 workery, Synology „StormSyn”
(Celeron J4125). Dziennik zaimportowany 2026-11-26 (`-import`, identyfikator paczki `7aa29c4` zamieniony na `d8c4e60`).

| Data | Commit | Pakiet | Fuzzer | Czas | Wykonania | Wynik |
|---|---|---|---|---|---|---|
| 2026-11-25 19:57 | `d8c4e60` | ./internal/datatypes | `FuzzDecimal` | 3h23m0s | 1256022899 | ok (Synology) |
| 2026-11-25 23:20 | `d8c4e60` | ./internal/datatypes | `FuzzLexical` | 1s | 0 | **błąd**: FAIL rc=1 input=internal/datatypes/testdata/fuzz/FuzzLexical/f16e7cb754284fef log=~/fuzz/goxsdgo/7aa29c4/fuzzpack/work/logs/20261125-232016-FuzzLexical.log (Synology) |
| 2026-11-25 23:20 | `d8c4e60` | ./internal/regex | `FuzzRegexTranslate` | 11m50s | 47289214 | **błąd**: FAIL rc=1 input=internal/regex/testdata/fuzz/FuzzRegexTranslate/91dd650195e5d5ae log=~/fuzz/goxsdgo/7aa29c4/fuzzpack/work/logs/20261125-232017-FuzzRegexTranslate.log (Synology) |
| 2026-11-25 23:32 | `d8c4e60` | ./internal/schema | `FuzzContentModel` | 3h23m0s | 918296274 | ok (Synology) |
| 2026-11-26 02:55 | `d8c4e60` | ./internal/xmlreader | `FuzzReader` | 3h23m0s | 358040267 | ok (Synology) |
| 2026-11-26 06:18 | `d8c4e60` | ./xsd | `FuzzCompile` | 3h23m1s | 136133422 | ok (Synology) |
| 2026-11-26 09:41 | `d8c4e60` | ./xsd | `FuzzValidate` | 3h23m1s | 176876533 | ok (Synology) |

Łącznie 17,1 h bez paniki; dwa przebiegi zakończone naruszeniem niezmiennika testu, żadne nie jest błędem biblioteki
(wejścia są w repozytorium jako regresje, w `internal/datatypes/testdata/fuzz/FuzzLexical/` i
`internal/regex/testdata/fuzz/FuzzRegexTranslate/`):

- `FuzzLexical`, `time` `20:00:00-07:00`: postać kanoniczna `03:00:00Z` (3.2.8.2: UTC, bez dnia) nie jest tą samą
  wartością według porządku z datą odniesienia (3.2.8, 3.2.7.4); różnica dokładnie jednej doby. Niespójność samej
  rekomendacji XSD 1.0 2e, biblioteka stosuje oba przepisy dosłownie; niezmiennik testu dopuszcza ten przypadek
  (`timeAcrossMidnight`), opis w `docs/known-issues.md` (KI-12).
- `FuzzRegexTranslate`, `(ab){0,999990}`: tłumaczenie i kompilacja RE2 2,2 s przy progu 2 s. Czas liniowy względem
  rozmiaru rozwinięcia (0,4 s na Apple M4); próg był za ciasny na wolny sprzęt (na Synology z `nice -n 19` obok innej kampanii 6,5–8,4 s). Próg 60 s.

Godziny nie liczą się do K5 dla v0.4.0: od `d8c4e60` zmienił się kod biblioteki (`xmlreader.Attr.Type`,
`Config.Resolve`). Nowa kampania na `v0.4.0` (z poprawionymi niezmiennikami, kod biblioteki bez zmian): niżej.

## Zlecenie na Synology (2026-11-26): kandydat v0.4.1

Paczka z `e715fa1` (kod v0.4.0), ustawiona w kolejce 2026-11-26 21:37, została wycofana przed startem
(23:36): poprawka KI-13 zmieniła kod biblioteki, więc jej godziny nie liczyłyby się dla wydania z poprawką.
Zamiast niej paczka z tagu `v0.4.1` (`985c49f`, kod biblioteki jak w `140b2de`) w `~/fuzz/goxsdgo/985c49f/`,
ustawiona 2026-11-26 23:37: 1440 min (24 h, 7 fuzzerów po ok. 205 min), 3 workery, `nice -n 19`, z nowym
niezmiennikiem `FuzzReader` (niżej). Ziarna wszystkich 7 fuzzerów przechodzą na serwerze. Ruszy po zadaniach
`gohighlightgo` i `gomermaidgo`. Po zakończeniu: skopiować `~/fuzz/goxsdgo/985c49f/fuzzpack/work/journal.tsv`
i `go -C tools/fuzz run . -root ../.. -import journal.tsv`, potem zbudować pakiet dowodów.

## Nowy niezmiennik FuzzReader (2026-11-26)

`FuzzReader` sprawdza też, że strumień tokenów i błędów nie zależy od podziału wejścia na odczyty (jeden odczyt,
po jednym bajcie, porcjami po 7 bajtów; XML-019). Dotąd fuzzer podawał wejście jednym odczytem i dlatego nie znalazł
błędu deklaracji XML czytanej porcjami (KI-13, znaleziony przy wydzielaniu czytnika do goxmlgo). Przebieg sprawdzający
na Macu (zasilanie sieciowe, `-parallel 2`): 100 s, bez błędu. Poprawka zmienia kod biblioteki, dlatego kampania
K5 biegnie na `v0.4.1` (wyżej).
