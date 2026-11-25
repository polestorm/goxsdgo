# 0005 Typy proste i wyrażenia regularne

Status: proponowana (etap 2) · 2026-11-25

## Kontekst

Etap 2 realizuje część 2 rekomendacji (Datatypes Second Edition): 44 typy wbudowane, aspekty, listy, unie
i wyrażenia regularne z dodatku F. Etap 3 zbuduje z tego typy ze schematu, a etap 5 będzie walidował nimi tekst
dokumentu. Część zachowań rekomendacja zostawia implementacji (limity cyfr, wersja Unicode) albo opisuje
niejednoznacznie; ten ADR zapisuje rozstrzygnięcia.

## Decyzja

1. **Pakiety.** `internal/datatypes` (typy, wartości, aspekty) i `internal/regex` (tłumacz wyrażeń XSD na RE2).
   Oba korzystają tylko z biblioteki standardowej (`math/big`, `regexp`, `unicode`, `encoding/base64`,
   `encoding/hex`, `strconv`) przez publiczne API.
2. **Budowa typów.** `Restrict(base, []FacetSpec, ctx)`, `NewList(item)`, `NewUnion(members...)`. Typy wbudowane
   pochodne powstają z dodatku A tym samym mechanizmem, więc każdy typ wbudowany jest testem mechanizmu restrykcji.
   Obowiązujące aspekty są kumulowane w typie: kontrola tekstu nie przechodzi łańcucha typów bazowych.
3. **Wartości.** decimal: `big.Int` × 10^-scale w postaci znormalizowanej, bez limitu cyfr. float i double:
   `float64` (float zaokrąglony do `float32` przez `strconv.ParseFloat(s, 32)`), jedno zero, NaN równy sobie.
   duration: liczba miesięcy i liczba sekund (decimal) ze wspólnym znakiem; P1Y = P12M i P1D = PT24H są tą samą
   wartością. Porządek liczymy przez dodanie do czterech dat odniesienia z 3.2.6.2: po normalizacji do (miesiące,
   sekundy) algorytm z dodatku E sprowadza się do dodania miesięcy z przypięciem dnia, a potem sekund. Typy daty
   i czasu: pola i strefa; dateTime sprowadzamy do UTC. Porządek i równość liczymy na chwili początkowej
   (3.2.7.4, „starting instants” w 3.2.10–3.2.14), z polami nieobecnymi wypełnionymi stałymi: rok przestępny
   1972, grudzień (31 dni), dzień 1, 00:00:00.
4. **time na stałej dacie.** time ze strefą nie jest zawijany do tej samej doby: 12:00:00-14:00 to 02:00Z dnia
   następnego, później niż 12:00:00-10:00 (22:00Z). Tak czytamy 3.2.8 („porządek dateTime na dowolnej dacie”).
   Zestaw W3C tego oczekuje (MS-DataTypes, time_maxInclusive007). Postać kanoniczna sprowadza time do UTC
   z zawinięciem.
5. **Rok bez zera.** Rok liczymy wprost z literału (-0001 to 1 p.n.e.), także w funkcji przestępności
   z dodatku E. -0001-02-29 jest więc niepoprawne. W XSD 1.1 (rok 0000 jako 1 p.n.e.) byłoby inaczej.
6. **Ograniczenia implementacji** (część 2 wymaga ich udokumentowania): rok do 12 cyfr, pola duration do 15 cyfr,
   decimal, ułamek sekundy i liczba sekund w duration bez ograniczenia. Przekroczenie limitu oznacza literał
   spoza przestrzeni leksykalnej (cvc-datatype-valid.1.2.1).
7. **Kody błędów.** Walidacja tekstu: identyfikatory reguł z rekomendacji (`cvc-datatype-valid.1.2.1`, `.1.2.2`,
   `.1.2.3`, `cvc-pattern-valid`, `cvc-enumeration-valid`, `cvc-length-valid`, `cvc-minLength-valid`,
   `cvc-maxLength-valid`, `cvc-minInclusive-valid`, `cvc-maxInclusive-valid`, `cvc-minExclusive-valid`,
   `cvc-maxExclusive-valid`, `cvc-totalDigits-valid`, `cvc-fractionDigits-valid`). Definicja typu: identyfikatory
   ograniczeń z części 2 (`cos-applicable-facets`, `cos-list-of-atomic`, `src-single-facet-value`,
   `length-minLength-maxLength`, `*-valid-restriction`, `minInclusive-less-than-equal-to-maxInclusive` itd.).
   Trzy kody są nasze, bo rekomendacja nie nazywa tych reguł: `facet-fixed` (zmiana aspektu z fixed),
   `facet-value` (wartość aspektu spoza jego typu) i `regex-syntax` (błędne wyrażenie). Do tego `regex-too-large`
   (wyrażenie za duże po rozwinięciu, ograniczenie implementacji). Naruszenie wzorca typu wbudowanego (np. integer,
   NCName) zgłaszamy jako `cvc-datatype-valid.1.2.1`, bo ten wzorzec definiuje przestrzeń leksykalną.
8. **Wyrażenia regularne.** Parser według produkcji [1]–[37a]. Klasy znaków liczymy jawnie jako zbiory przedziałów
   (odejmowanie, dopełnienie względem znaków XML), a do RE2 trafiają tylko literały, klasy z przedziałami, grupy
   bez przechwytywania i liczniki, w `\A(?:…)\z`. RE2 nie robi nawrotów, więc dopasowanie jest liniowe.
   - **Liczniki powyżej 1000** (granica `regexp/syntax`): `X{n,m}` rozkładamy na odcinki po 1000
     (`X{0,2500}` ≡ `X{0,1000}X{0,1000}X{0,500}`, ten sam język). Gdy X sam zawiera licznik i iloczyn
     przekroczyłby 1000, rozwijamy zewnętrzny licznik na kopie X. Wyrażenie dłuższe po rozwinięciu niż 1 MiB
     dostaje `regex-too-large`.
   - **`\i` i `\c`** według produkcji Letter i NameChar z XML 1.0 Second Edition (dodatek B), bo do niej odsyła
     część 2. Tablice generuje `internal/regex/gentables.go` z tekstu rekomendacji, z sumą SHA-256 źródła.
   - **Bloki:** Unicode 3.1 Blocks.txt bez bloków surogatów. Generator sprawdza, że wszystkie 85 wierszy tablicy
     z dodatku F są w tym pliku. PrivateUse to tylko E000–F8FF, jak w tablicy dodatku F; obszary prywatne spoza
     BMP od Unicode 4.0 nazywają się inaczej. Nazwy bloków surogatów przyjmujemy jako zbiory puste: F.1 je
     wyłącza, ale zestaw W3C (MS-Regex reN75) oczekuje poprawnego schematu.
   - **Kategorie:** tablice `unicode` z biblioteki standardowej Go (Unicode 15.0). XSD dokłada dwie poprawki:
     C obejmuje Cn i nie obejmuje Cs, a Cn liczymy jako dopełnienie wszystkich kategorii przypisanych.
9. **Pytanie 7 (wersja Unicode), propozycja.** Kategorie według Unicode z biblioteki standardowej Go, czyli
   żeton `Unicode_6.0.0` w zestawie W3C (najbliższy dzisiejszym danym), a bloki według Unicode 3.1, jak w dodatku F.
   Zestaw w przypadkach bez żetonu wersji zakłada dane Unicode 3.0, w których znaki spoza BMP nie mają kategorii.
   Takich przypadków jest 16 w MS-Regex: reJ11, reJ13, reJ19, reJ21, reJ23, reJ25, reJ29, reJ31, reJ33, reJ35,
   reJ61, reJ69, reJ75, reJ77, reS42 i reT63. Każdy z nich dotyczy znaku spoza BMP, który od Unicode 3.1 ma
   kategorię (np. U+1D7A8 to Lu, U+1D7FF to Nd). Proponujemy uznać je za różnice wobec zestawu i opisać
   w `differences.md`, a nie wracać do danych Unicode 3.0, które odrzuciłyby znaki używane dziś.

## Czego nie robimy w etapie 2

- jednoznaczności ID, rozwiązywania IDREF i sprawdzania deklaracji ENTITY: to etap 5 (walidacja dokumentu);
- rozwiązywania przestrzeni nazw w dokumencie: QName i NOTATION dostają `NamespaceResolver` od wywołującego;
- pełnej składni URI dla anyURI: rekomendacja zastrzega, że reguł poszczególnych schematów adresów nie sprawdza się,
  a sprawdzamy tylko to, co nie zależy od schematu adresu (sekwencje %HH, jeden „#”, składnia nazwy schematu);
- postaci kanonicznej duration i typów g*: rekomendacja 1.0 jej nie definiuje; `Canonical` zwraca zapis
  znormalizowany;
- optymalizacji pod kryterium K7: wzorce typów wbudowanych (Name, NCName, integer) przechodzą przez RE2;
  szybkie ścieżki bez alokacji przyjdą w etapie 7, gdy pomiar pokaże potrzebę.

## Konsekwencje

+ typy wbudowane i aspekty przechodzą wszystkie przypadki NIST, które adapter odczytuje (pomiar w `docs/testing.md`);
+ czas dopasowania wzorców jest liniowy także dla danych złośliwych;
− dwie różnice wobec zestawu W3C wynikają z danych znaków: 16 przypadków kategorii Unicode spoza BMP (punkt 9)
  i U+0346, którego zestaw oczekuje w `\c`, choć produkcja CombiningChar z XML 1.0 2e kończy pierwszy przedział
  na U+0345 (MS-Regex reZ006i);
− wyrażenia z bardzo dużymi licznikami zagnieżdżonymi w licznikach dostają `regex-too-large` zamiast wyniku.
