# 0011 Utwardzanie: fuzzing, przypadki patologiczne, limity, wydajność

Status: przyjęta (etap 7) · 2026-11-25

## Kontekst

Etapy 0–6b dały zgodność (K1–K3), błędy dla ludzi i komunikaty. Etap 7 domyka odporność (K5), bezpieczeństwo
i pamięć (K6) oraz wydajność (K7): długi fuzzing, testy wejść patologicznych dla całej biblioteki, limity
zebrane w jednym miejscu i pomiary względem libxml2, także na prywatnym korpusie plików klientów.

## Decyzje

1. **Kampania fuzzingu (K5).** Prowadzi ją `tools/fuzz`: osobny moduł bez zależności, który znajduje wszystkie
   fuzzery w module biblioteki, uruchamia je po kolei przez `go test -fuzz` z równym podziałem czasu i dopisuje wynik
   każdego przebiegu do `docs/fuzzing.md` (data, commit, czas, wykonania, wynik). Do K5 liczy się czas przebiegów
   zakończonych wynikiem „ok” na commicie kandydata albo na późniejszych commitach bez zmian kodu biblioteki poza
   testami; przebieg z błędem zeruje licznik dla kodu, którego dotyczy (reguła w `docs/fuzzing.md`).
   Decyzja właściciela z 2026-11-25: fuzzing nie działa lokalnie w dzień (blokuje inne sesje i przekłamuje ich
   czasy). `tools/fuzz -pack` buduje paczkę dla serwera bez Go (Synology): binarki testów z `go test -c` dla
   linux/amd64 i linux/arm64, wejścia regresji i dane wejść startowych w układzie repozytorium, skrypt `run.sh` w czystym
   sh (BusyBox) z dziennikiem `journal.tsv`; `tools/fuzz -import` dopisuje dziennik serwera do `docs/fuzzing.md`.
   Pierwsza kampania lokalna na `2e813e9` została przerwana po 15 min (`FuzzDecimal`, 107,8 mln wykonań, bez błędów).
2. **Wejścia startowe z pobranych danych.** Fuzzery dostają próbkę schematów i przykładów MF, przypadków W3C,
   dokumentów XML Conformance i wzorców ze schematów MF (`testkit.SampleFiles`), czytaną w czasie testu. Danych nie
   commitujemy (ADR 0003), a bez nich fuzzery startują z wejść wpisanych w kod. Danych prywatnych nie używamy.
3. **Nowy fuzzer `FuzzDecimal`** porównuje `Decimal` z niezależnym wzorcem na `big.Rat` (przyjęcie literału,
   porządek, znak, postać kanoniczna, liczba cyfr, jednoznaczna reprezentacja), bo decyzja 6 zmienia reprezentację.
4. **`TestPathological` dla całej biblioteki** (pakiet `xsd`): każdy przypadek mierzy dwa rozmiary wejścia (n i 2n),
   z najlepszego z trzech przebiegów; iloraz czasów ponad 3,2 jest błędem (2 to wzrost liniowy, 4 kwadratowy).
   Przypadki ponad limitem kończą się `ErrLimit` w czasie poniżej 2 s. Modele zawartości z setkami tysięcy powtórzeń
   bada dalej `internal/schema` `TestPathological`, a wzorce `internal/regex` `TestPathologicalLinear`.
   - Test wykrył jedyne kwadratowe miejsce: sprawdzenie NSC Attributes Unique dla atrybutów z prefiksem w czytniku
     (iloraz 3,29). Od 32 atrybutów czytnik używa zbioru nazw rozwiniętych (przestrzeń i nazwa lokalna, bez prefiksu);
     iloraz spadł do ok. 2,0. Test `TestManyPrefixedAttributes` pilnuje, że powtórzenie jest dalej wykrywane, także
     przez dwa prefiksy tej samej przestrzeni (pierwsza wersja zbioru z kluczem z prefiksem go nie wykrywała).
   - Znane ograniczenie (z ADR 0007): model zgodny z UPA, ale niejednoznaczny co do liczników, np.
     `(a{2,3}){1,50000}`, ma po każdym dziecku coraz więcej konfiguracji liczników; ponad `MaxConfigurations` (1024)
     walidacja kończy się `ErrLimit`, nigdy werdyktem. Ścisłe łączenie zbiorów wartości liczników zostawiamy na
     później; w schematach MF takich modeli nie ma.
5. **Limity w jednym miejscu.** Wszystkie limity są konfigurowalne i mają bezpieczne wartości domyślne:

   | Limit | Gdzie | Domyślnie |
   |---|---|---|
   | głębokość elementów | `Config.Document.MaxDepth` | 512 |
   | długość nazwy | `Config.Document.MaxNameLength` | 16 KiB |
   | atrybuty jednego elementu | `Config.Document.MaxAttributes` | 4096 |
   | tekst elementu, atrybutu, komentarza, PI | `Config.Document.MaxTextSize` | 16 MiB |
   | rozwinięcia encji wewnętrznych w dokumencie | `Config.Document.MaxEntityExpansion` | 4 MiB |
   | zagnieżdżenie odwołań do encji | `Config.Document.MaxEntityDepth` | 32 |
   | DOCTYPE z podzbiorem wewnętrznym | `Config.Document.MaxDTDSize` | 1 MiB |
   | błędy jednego dokumentu | `Config.MaxErrors` | 1000 (`ErrTooManyErrors`) |
   | wpisy tabel tożsamości i ID | `Config.MaxIdentityEntries` | 4 mln |
   | dokumenty schematu | `Config.MaxDocuments` | 1000 |
   | zagnieżdżenie include, import, redefine | `Config.MaxDepth` | 64 |
   | konfiguracje liczników w jednym modelu | `schema.MaxConfigurations` | 1024 |
   | rozmiar wzorca po rozwinięciu liczników | `internal/regex` | 1 MiB tekstu RE2 (`regex-too-large`) |

   Nowe w API: `xsd.DocumentLimits`, `Config.Document` i `DefaultDocumentLimits()` (zgodnie wstecz: wartość zerowa
   to wartości domyślne, ujemna wyłącza limit). Wcześniej limity czytnika dokumentu nie były dostępne przez `xsd`.
6. **Optymalizacje bez zmiany wyników.**
   - `Decimal` trzyma wartości mieszczące się w int64 bez `math/big`; `big.Int` tylko dla większych, z niezmiennikiem
     jednej reprezentacji. Alokacje na 20 MB JPK spadły z 2,65 mln do 0,95 mln, pamięć przydzielana z 78,7 MB do
     25,8 MB.
   - Indeks pozycji modelu z najwyżej 8 nazwami przeszukuje listę zamiast mapy.
   - Krawędź sekwencji, która przegląda od 5 do 256 opcjonalnych dzieci, dostaje leniwie budowany indeks scalony
     (`sync.Once`, bo model jest współdzielony przez goroutines) z tym samym pierwszeństwem co przeglądanie po kolei;
     równoważność sprawdza `TestMergedIndexMatchesSequential` (58 501 porównań).
   - Każdą zmianę sprawdza zrzut wszystkich błędów zestawu W3C i korpusu (`TestDumpErrors`, 56 524 wiersze): zrzut
     po zmianach jest identyczny bajt w bajt z zrzutem kodu sprzed etapu.
7. **Pomiary (2026-11-25, Apple M-series, 10 rdzeni; w czasie pomiarów obcy proces zajmował 1–5 rdzeni).**
   - `BenchmarkValidateJPK` (20 MB, przemiennie z kodem bazowym): przed 37,9–39,6 MB/s, po 47,5–48,4 MB/s.
   - K7, syntetyczny JPK_V7M(3) 200 MB (`tools/compare timefile`, najlepszy z 3): goxsdgo 4,44 s, libxml2
     strumieniowo przez `goxsdval` 4,24 s, iloraz **1,05×**.
   - K7 i K3, prywatny korpus (`tools/compare private`): 172 pliki XML (1613,5 MB), 21 bez schematu w korpusie,
     151 porównanych (1594,7 MB); czas goxsdgo 19,7 s, libxml2 16,6 s, iloraz **1,19×**; największy plik 227,1 MB:
     1,33×. Ten sam werdykt w 151 / 151, ten sam wiersz pierwszego błędu w 50 / 51 plikach błędnych (jedna różnica
     wiersza przy tym samym rodzaju błędu: `cvc-complex-type.2.4` / 1871). Do dokumentów trafiają tylko liczby.
   - K6, syntetyczny JPK 1 GB (`TestStreamingMemory`, `GOXSDGO_STREAM_MB=1024`): szczyt pamięci 6,3 MB.
8. **Porównanie wydajności z libxml2 przez `goxsdval`**, nie przez `xmllint --stream`: to ta sama biblioteka i ten sam
   tryb strumieniowy, którego używa odbiorca, a import schematów MF rozwiązuje ten sam katalog XML co w `tools/compare`.

## Czego nie robimy

- Bez `unsafe`, asemblera i współbieżności wewnątrz walidacji jednego dokumentu: zapas do K7 (≤ 3×) jest duży.
- Bez przybliżonego łączenia wartości liczników (przedziały zamiast zbiorów): mogłoby przyjąć dokument błędny.
- Kampania fuzzingu nie działa w CI (CI jest wstrzymane do następnego okresu rozliczeniowego) ani lokalnie w dzień;
  działa na serwerze Synology z paczki `tools/fuzz -pack` (polecenia w `docs/fuzzing.md`).

## Konsekwencje

+ K6 i K7 spełnione z zapasem także na prawdziwych dużych plikach; K3 na korpusie prywatnym 100% werdyktów;
+ limity opisane w jednym miejscu i dostępne przez API;
+ każda optymalizacja ma dowód, że nie zmienia wyników;
− do K5 brakuje 23 h 45 min kampanii na kandydacie `2e813e9` (stan w `docs/fuzzing.md`);
− indeks scalony zwiększa pamięć używanego modelu o rozmiar sumy indeksów krawędzi (najwyżej 256 dzieci na krawędź).
