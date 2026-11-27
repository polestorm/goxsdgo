# 0004 Własny czytnik XML

Status: przyjęta · 2026-11-25 · rozstrzyga pytanie 3 z `open-questions.md`; od 2026-11-27 czytnik jest w module goxmlgo (ADR 0014), decyzje tej ADR obowiązują tam

## Kontekst

Walidacja XSD działa na zbiorze informacji dokumentu (infoset): nazwach rozwiniętych, atrybutach po normalizacji,
wartościach domyślnych z podzbioru wewnętrznego DTD i deklaracjach przestrzeni nazw w zasięgu. Błędy mają mieć
wiersz i kolumnę (ADR 0001, kryterium K3), czytanie ma być strumieniowe i bezpieczne (K6), a odrzucenie dokumentu
źle uformowanego jest częścią werdyktu.

`encoding/xml` z biblioteki standardowej Go nie spełnia tych wymagań. Na zestawie W3C XML Conformance (te same
1 727 przypadków co niżej, pomiar 2026-11-25, `xml.Decoder` w trybie ścisłym) daje właściwy werdykt dla 1 146,
przyjmuje 517 dokumentów źle uformowanych i odrzuca 64 poprawne. Nie przetwarza podzbioru wewnętrznego (encje,
wartości domyślne, typy atrybutów), nie sprawdza ograniczeń przestrzeni nazw i podaje kolumnę w bajtach.

## Decyzja

1. **Własny czytnik `internal/xmlreader`**, napisany z XML 1.0 Fifth Edition i Namespaces in XML 1.0 Third Edition.
   Pull API: `New(r, Options)`, `Next()` zwraca tokeny (początek i koniec elementu z nazwą rozwiniętą i atrybutami,
   tekst z oznaczeniem CDATA i „same białe znaki”, PI, komentarz, DOCTYPE). `LookupNamespace` podaje przestrzenie
   nazw w zasięgu (potrzebne dla wartości typu QName i `xsi:type`), `UnparsedEntity` encje nieparsowane (typy ENTITY).
   `encoding/xml` zostaje tylko w testach, do czytania metadanych zestawów.
2. **Pełna dobra forma i przestrzenie nazw.** Każde naruszenie jest błędem krytycznym (1.2): `Next` zwraca `*Error`
   ze stałym kodem (`wfc-*`, `nsc-*` według nazw ograniczeń rekomendacji, `xml-*`, `ns-*`, `limit-*`,
   `unsupported-*`), wierszem i kolumną, a potem stale ten sam błąd. Przestrzenie nazw są zawsze przetwarzane;
   `Options.NoNamespaces` służy tylko testom zgodności czystego XML 1.0.
3. **Pozycja.** Wiersz i kolumna liczone od 1, po normalizacji końców wierszy (CR LF to jeden koniec wiersza).
   **Kolumna liczy znaki** (punkty kodowe Unicode), nie bajty: polskie litery w nazwach i wartościach nie przesuwają
   kolumny, a ten sam dokument w UTF-8 i UTF-16 ma te same pozycje. Tabulator to jeden znak. Błąd w tekście
   zastępczym encji wskazuje miejsce odwołania do niej w dokumencie. libxml2 (przez `goxsdval`) podaje dla błędów
   schematu kolumnę 0, więc porównanie K3 dotyczy wiersza i elementu.
4. **Kodowania:** UTF-8 (z BOM i bez), UTF-16 LE i BE z BOM, ISO-8859-1 i US-ASCII według deklaracji. Inne dają
   `xml-encoding-unsupported`. Pliki MF są w UTF-8. Rekomendacja na później: ISO-8859-2 i windows-1250 dodać, gdy
   odbiorca ich potrzebuje, z tablicami z danych Unicode (mapping tables), nie z cudzych bibliotek. UTF-16 bez BOM
   jest błędem (4.3.3), deklaracja sprzeczna z BOM też.
5. **DTD.** Podzbiór wewnętrzny jest sprawdzany składniowo w całości. Czytnik robi to, czego rekomendacja wymaga od
   procesora bez walidacji (5.1): rozwija encje wewnętrzne, dodaje atrybuty z wartości domyślnych (`Specified=false`),
   normalizuje atrybuty według typów z ATTLIST i zapamiętuje notacje. Domyślne deklaracje przestrzeni nazw z ATTLIST
   działają jak podane w znaczniku. Po odwołaniu do encji parametrycznej, której nie czyta, nie przetwarza dalszych
   deklaracji encji i list atrybutów, chyba że `standalone="yes"`. Sekcje warunkowe w tekście encji parametrycznej:
   `unsupported-conditional-section`. Instrukcji przetwarzania z wnętrza DOCTYPE nie podaje jako tokenów (nie należą
   do treści dokumentu; XSD ich nie używa).
6. **Encje zewnętrzne nigdy nie są czytane**, tak samo podzbiór zewnętrzny: czytnik nie ma dostępu do plików ani
   sieci. Odwołanie do zewnętrznej encji ogólnej w treści daje błąd `unsupported-external-entity`, zamiast pominięcia
   treści, które rekomendacja dopuszcza: pominięta treść zmieniłaby wynik walidacji XSD bez śladu. W wartości
   atrybutu takie odwołanie narusza WFC No External Entity References. Odwołanie do niezadeklarowanej encji, gdy WFC
   Entity Declared nie obowiązuje (podzbiór zewnętrzny albo odwołania do encji parametrycznych), daje
   `unsupported-entity-unresolved`, z tego samego powodu.
7. **Limity z bezpiecznymi wartościami domyślnymi** (`DefaultLimits`; zero oznacza domyślną, wartość ujemna brak
   limitu): głębokość 512, nazwa 16 KiB, 4 096 atrybutów w elemencie (zestaw W3C XSD ma przypadek z 2 000), token
   tekstu, komentarza, PI albo wartość atrybutu 16 MiB, łączne rozwinięcia encji 4 MiB (chroni przed „billion
   laughs” w treści i w wartościach atrybutów), zagnieżdżenie encji 32, deklaracja DOCTYPE 1 MiB.
8. **Strumieniowo.** Bufor wejścia 64 KiB; pamięć zależy od najdłuższego tokenu, głębokości i podzbioru wewnętrznego,
   nie od rozmiaru dokumentu. Kolejne dane znakowe (także z encji) łączą się w jeden token tekstu, a sekcja CDATA jest
   osobnym tokenem.

## Pomiary (2026-11-25, macOS arm64, Go 1.26)

- **Zestaw W3C XML Conformance 20130923:** 1 725 z 1 727 przypadków w zakresie (XML 1.0 piątego wydania
  i Namespaces 1.0, bez encji zewnętrznych), według grup: xmltest 299/299, sun 101/101, oasis 323/323, ibm 526/527,
  eduni 476/477. Przypadki z OUTPUT dają tę samą drugą postać kanoniczną. Wyłączone z powodem: 274 XML 1.1
  i Namespaces 1.1, 310 tylko dla wydań 1–4, 265 wymaga encji zewnętrznych, 9 błędów opcjonalnych. Dwie świadome
  porażki: `ibm-valid-P29-ibm29v01.xml` (postać kanoniczna zawiera PI z wnętrza DOCTYPE, punkt 5)
  i `rmt-e3e-13` (dokument „invalid” z odwołaniem do niezadeklarowanej encji, punkt 6).
- **Wszystkie 42 018 plików `.xml` i `.xsd`** z zestawu W3C XSD i korpusu MF: werdykt dobrej formy zgodny z libxml2
  2.15.4 (`xmllint --noout --nonet`) poza 3 plikami. libxml2 odrzuca w nich deklaracje przestrzeni nazw z nazwami
  `a&#x0400;` i `あ` jako „nie URI”. Namespaces in XML 1.0 nie robi z tego błędu dobrej formy, a `あ` jest poprawnym
  IRI, więc czytnik je przyjmuje.
- **Wydajność** (syntetyczny JPK_V7M, 20 000 wierszy, 8,8 MB, `BenchmarkJPK`): ok. 195 MB/s, 68 alokacji na cały
  dokument (stała liczba). `encoding/xml` na tym samym dokumencie: ok. 77 MB/s i 1,26 mln alokacji. Plik 88 MB
  z tego samego generatora, obok siebie na tej samej maszynie: czytnik 0,75 s czasu procesora, libxml2
  (`xmllint --noout --stream`) 0,65 s; pamięć procesu czytnika ok. 6,5 MB (RSS). Pomiar czasu z równolegle
  obciążoną maszyną, do powtórzenia w etapie 7.

## Konsekwencje

+ odrzucamy dokumenty źle uformowane tak jak rekomendacja, a nie tak jak `encoding/xml`;
+ czytnik daje dokładnie te informacje, których potrzebuje walidacja XSD, z pozycją w znakach;
+ zero zależności, czas liniowy (powtórzenia atrybutów sprawdza mapa, nie porównania każdy z każdym);
− ok. 2,7 tys. linii własnego kodu (bez testów) do utrzymania;
− dokumenty z encjami zewnętrznymi są odrzucane; dla JPK, KSeF i e-Sprawozdań to bez znaczenia, ale trzeba to
  opisać w `differences.md`.

**Uzupełnienie 2026-11-26.** Czytnik jest publicznym pakietem `xmlreader` (decyzja właściciela): bez zmian w kodzie,
z angielską dokumentacją API i porównaniem z `encoding/xml` w `xmlreader/README.md`. Moduł nadal nie ma zależności.
