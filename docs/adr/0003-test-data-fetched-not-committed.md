# 0003 Dane testowe pobierane narzędziem, nie przechowywane w repozytorium

Status: przyjęta · 2026-11-25 · rozstrzyga pytania 1 i 2 z `open-questions.md`

## Kontekst

Zestaw W3C XML Schema (`w3c/xsdtests`) ma 42 270 plików (ok. 84 MB, 214 MB na dysku). Zestaw W3C XML Conformance
ma 3 386 plików. Schematy MF (JPK, KSeF, e-Sprawozdania z importami) to 164 pliki (18 MB). Wszystkie te pliki są
opublikowane i dają się pobrać spod stałych adresów. Wpisanie ich do repozytorium rozdęłoby historię i przeglądy, a nie
daje niczego, czego nie da przypięcie wersji i sum kontrolnych.

## Decyzja

1. **Żadne cudze dane testowe nie trafiają do repozytorium.** Dotyczy to zestawów W3C, schematów MF i przykładów MF.
   Katalogi docelowe są w `.gitignore`.
2. **Do repozytorium trafiają manifest i narzędzie.** Manifest `testdata/sources.json` podaje dla każdego pliku
   adresy, sumę SHA-256, przestrzeń nazw i importy, a dla archiwów przypięty adres, sumę drzewa plików i ich liczbę.
   Narzędzie `tools/fetch` pobiera brakujące pliki, a w trybie `-verify` sprawdza to, co jest na dysku.
3. **Narzędzie ma dwie równoważne wersje: Go i Python.** Wersja w Go (`tools/fetch/main.go`) jest dla osób pracujących
   w Go. Jest **osobnym modułem** (`tools/fetch/go.mod` bez `require`), więc kod sieciowy nie należy do modułu
   biblioteki. Wersja w Pythonie (`tools/fetch/fetch.py`, Python 3.9+, tylko biblioteka standardowa) jest dla osób bez
   Go, m.in. na Windows. Obie czytają ten sam manifest i dają te same pliki. Testy obu wersji mają tę samą wartość
   wzorcową sumy drzewa, a CI uruchamia testy Pythona 3.10 i najnowszego na Linuksie i Windows (3.9 sprawdzony lokalnie, 3.9.6). Basha nie używamy: nie ma go
   w standardzie Windows, a narzędzia, których by potrzebował (`jq`, `sha256sum`), różnią się między systemami.
   Zmiana zachowania jednej wersji wymaga tej samej zmiany w drugiej.
4. **Plik przyjmujemy tylko z sumą z manifestu.** Przy archiwach W3C sprawdzamy sumę drzewa plików, bo GitHub generuje
   archiwa sam i ich bajty nie są gwarantowane. Odrzucamy „miękkie 404”, czyli strony HTML, które gov.pl i crd.gov.pl
   zwracają zamiast brakującego pliku.
5. **Wersja oryginalna wygrywa.** Schematy zapisujemy bajt w bajt tak, jak publikuje je MF, łącznie z BOM i końcami
   wierszy CRLF. Kopie z innych projektów (np. xml.pro) służą tylko jako zapasowe źródło (`-from`) i muszą mieć tę samą
   sumę.
6. **Testy bez pobranych danych są pomijane z komunikatem**, który podaje polecenie pobierające. Nie są uznawane za
   zaliczone. Gdy pojawią się pierwsze testy na tych danych (`TestW3CSuite`, etap 0), CI zacznie je pobierać
   i przechowywać w pamięci podręcznej według sumy manifestu.

## Konsekwencje

+ repozytorium zostaje małe, a dane są powtarzalne: ta sama suma oznacza te same pliki u każdego;
+ zmiana wersji danych to zmiana jednego wiersza manifestu, widoczna w przeglądzie;
− do pierwszego pełnego przebiegu testów potrzebna jest sieć (ok. 15 s);
− adresy MF się zmieniają. Gdy plik zniknie spod wszystkich adresów, trzeba znaleźć nowy adres albo użyć `-from`
  z lokalnej kopii. Narzędzie podaje wtedy, które pliki i dlaczego.
