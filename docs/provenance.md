# Rodowód kodu

`goxsdgo` powstaje od zera, z rekomendacji W3C. Ten plik mówi, z czego wolno korzystać, a z czego nie.

## Źródła, z których piszemy

- W3C XML Schema Part 1: Structures Second Edition i Part 2: Datatypes Second Edition (2004-10-28), z erratą;
- Extensible Markup Language (XML) 1.0 Fifth Edition i Namespaces in XML 1.0 Third Edition;
- normy, do których rekomendacje odsyłają: Unicode (kategorie i bloki znaków dla wyrażeń regularnych), ISO 8601
  w zakresie opisanym w części 2, RFC 3986 (`xs:anyURI`), IEEE 754 (`xs:float`, `xs:double`).

Rekomendacje opisują ograniczenia poprawności i reguły oceny (Validation Rules, Schema Component Constraints). To jest
część specyfikacji i wolno je stosować wprost, łącznie z algorytmami podanymi w tekście (np. sprawdzenie poprawnego
ograniczenia cząstek).

Wolno korzystać z ogólnej wiedzy o automatach (konstrukcja automatu z wyrażenia, automaty z licznikami), opisanej
w publikacjach naukowych, z podaniem źródła w komentarzu.

## Czego nie robimy

Nie czytamy, nie kopiujemy i nie przenosimy kodu walidatorów XSD ani parserów XML z walidacją: libxml2 (także
`xmlschemas.c`, `xmlregexp.c`), Xerces (C i Java), Saxon, xmlschema (Python), lxml, go-libxml2, helium, xsd-validator
ani ich portów. Dotyczy to też `../controls/goxsdval` poza jego publicznym API: to opakowanie libxml2, więc wolno go
wywoływać, a jego plików `*_libxml2.go` nie przenosimy.

## Co wolno

- uruchamiać libxml2 (przez `goxsdval` albo `xmllint`) i inne walidatory jako czarną skrzynkę i porównywać werdykty
  oraz miejsca błędów (`tools/compare`);
- czytać publiczną dokumentację użytkową walidatorów (opisy opcji, listy znanych odstępstw);
- czytać `encoding/xml` i `regexp` z biblioteki standardowej Go (BSD, ta sama licencja co reszta zależności programu),
  ale biblioteka używa ich tylko przez publiczne API.

## Dane testowe

Zestaw W3C, schematy MF i przykłady są danymi testowymi, nie kodem. Źródła, licencje i sumy kontrolne:
`../testdata/README.md`.
