# Różnice względem rekomendacji i walidatora odniesienia

Każda różnica werdyktu między `goxsdgo` a libxml2 (przez `goxsdval`) i każde wyłączenie przypadku z zestawu W3C ma tu
wpis: przypadek, oba wyniki, co mówi rekomendacja (rozdział i reguła), kto ma rację, skutek dla odbiorcy.

| Przypadek | `goxsdgo` | libxml2 | Rekomendacja | Skutek dla odbiorcy |
|---|---|---|---|---|
| `msData/additional/ns.xml`, `msData/datatypes/Facets/anyURI/anyURI_a002.xml` i `.xsd` (dobra forma) | przyjmuje | odrzuca nazwę przestrzeni nazw jako „nie URI” | Namespaces in XML 1.0, §2.2: niezgodność z RFC 3986 nie jest błędem dobrej formy | plik dotąd odrzucany zostanie przyjęty przez czytnik |
| dokumenty z odwołaniem do encji zewnętrznej, z deklaracjami potrzebnymi z podzbioru zewnętrznego albo z sekcjami warunkowymi w encjach parametrycznych | odrzuca (`unsupported-*`, ADR 0004) | czyta encje zewnętrzne, jeśli są dostępne | rekomendacja pozwala procesorowi bez walidacji DTD nie czytać encji zewnętrznych (XML 1.0, §5.1), ale wtedy treść jest niepełna | plik dotąd przyjmowany zostanie odrzucony; w korpusie MF takich nie ma |

| MS-Regex: reJ11, reJ13, reJ19, reJ21, reJ23, reJ25, reJ29, reJ31, reJ33, reJ35, reJ61, reJ69, reJ75, reJ77, reS21, reS42, reT63, reT84 | kategorie znaków spoza BMP według Unicode 15 (np. U+1D7A8 to `Lu`, U+1D7FF to `Nd`) | — (porównanie w etapie 3) | zestaw zakłada Unicode 3.0, w którym znaki spoza BMP nie mają kategorii; ADR 0005, pytanie 7 | wzorce z `\p{Lu}`, `\d` itd. przyjmą znaki spoza BMP; w korpusie MF bez znaczenia |
| MS-Regex: reZ006i | U+0346 poza `\c` | — | produkcja CombiningChar z XML 1.0 2e kończy pierwszy przedział na U+0345; zestaw oczekuje U+0346 w `\c` | bez znaczenia dla korpusu MF |

## Rozbieżności werdyktu z libxml2 (kategorie, etap 6)

Stan 2026-11-25, libxml2 2.15.4 przez `goxsdval`, zestaw W3C z commitu `7bc3365c`. Pełna lista 528 przypadków
(452 testy schematów, 76 testów dokumentów), każdy z kategorią: `differences-libxml2.md` (generowany przez
`tools/compare`). „Rację ma” według oczekiwanego werdyktu zestawu. Rację ma goxsdgo w 519 przypadkach, libxml2 w 9;
te 9 to odstępstwa opisane niżej.

| Kategoria | Przypadki | Rację ma goxsdgo / libxml2 | Skutek dla odbiorcy |
|---|---|---|---|
| libxml2 przyjmuje restrykcję typu złożonego niezgodną z 3.4.6 i 3.9.6 | 264 | 263 / 1 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 przyjmuje wzorzec spoza dodatku F | 43 | 43 / 0 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 odrzuca schemat, który goxsdgo przyjmuje | 34 | 34 / 0 | schemat dotąd odrzucany zostanie przyjęty |
| libxml2 pozwala zmienić aspekt ustalony (`fixed`) | 26 | 26 / 0 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 nie sprawdza części ograniczeń spójności aspektów | 24 | 23 / 1 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 odrzuca dokument według reguły oceny, którą goxsdgo stosuje inaczej | 23 | 22 / 1 | dokument dotąd odrzucany zostanie przyjęty |
| libxml2 uznaje dokument za źle uformowany, a goxsdgo nie | 18 | 17 / 1 | dokument dotąd odrzucany zostanie przyjęty |
| libxml2 nie sprawdza innych ograniczeń komponentów | 15 | 15 / 0 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 przyjmuje niepoprawną redefinicję | 15 | 14 / 1 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 przyjmuje dokument niezgodny ze schematem dla schematów | 12 | 10 / 2 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 nie sprawdza IDREF (`cvc-id.1`) | 11 | 11 / 0 | dokument dotąd przyjmowany zostanie odrzucony |
| libxml2 nie sprawdza EDC | 10 | 10 / 0 | schemat dotąd przyjmowany zostanie odrzucony |
| goxsdgo odrzuca wartość, którą libxml2 przyjmuje | 10 | 8 / 2 | dokument dotąd przyjmowany zostanie odrzucony |
| goxsdgo odrzuca dokument według reguły oceny, której libxml2 nie stosuje | 9 | 9 / 0 | dokument dotąd przyjmowany zostanie odrzucony |
| libxml2 nie wykrywa części niejednoznaczności (UPA) | 9 | 9 / 0 | schemat dotąd przyjmowany zostanie odrzucony |
| libxml2 odrzuca wartość, którą goxsdgo przyjmuje | 5 | 5 / 0 | dokument dotąd odrzucany zostanie przyjęty |

Na korpusie MF (170 schematów, 15 dokumentów) werdykty obu walidatorów są takie same.

## Różnice względem oczekiwań zestawu W3C (testy schematów, etap 3)

| Przypadek | `goxsdgo` | Oczekiwane | Uzasadnienie | Propozycja |
|---|---|---|---|---|
| Saxonica Missing: missing001, missing002, missing003, missing006 | błąd `src-resolve` | poprawny | 43 przypadki Sun i Microsoft oczekują błędu dla nierozwiązanego odwołania; ADR 0006 | zostawić jako różnicę |
| Saxonica XmlVersions: xv001, xv004 | błąd | poprawny | „Dĳkstra” (U+0133) nie jest NCName według znaków nazw XML 1.0 2e, z których korzysta XSD 1.0 2e | zostawić jako różnicę |
| MS IDREFS_length006, NMTOKENS_length006 (stable), errF001 (queried) | błąd `length-minLength-maxLength` | poprawny | `length` i `minLength` w jednym kroku restrykcji są błędem według 4.3.1.4, punkty 1.2 i 2.2 | zostawić jako różnicę |
| MS particlesZ001 | błąd `cos-particle-restrict` | poprawny | RecurseAsIfGroup stosowany dosłownie (3.9.6), czego wymagają particlesHa161 i particlesK006; ADR 0007 | zostawić jako różnicę |
| MS elemZ026, elemZ027_c, particlesV020 (queried) | poprawny | błędny | restrykcja z grupami podstawienia według normalizacji 2.1 w 3.9.6; choice z grupy podstawienia nie wymaga zachowania kolejności członków (rekomendacja jej nie określa); ADR 0007 | zostawić jako różnicę |
| MS ste110 (queried) | błąd `src-simple-type.4` | poprawny | unia z cyklem w definicji | zostawić jako różnicę |
| MS schU1 (disputed-test) | według rekomendacji | sporny | status sporny w zestawie | zostawić jako różnicę |
| MS anyURI_b006 (stable) | poprawny | błędny | ukośnik wsteczny przyjmujemy: część 2, 3.2.17 każe przed sprawdzeniem z RFC 2396 zamienić znaki według XLink §5.4, a ukośnik wsteczny staje się wtedy `%5C`, czyli poprawnym URI | rozstrzygnięte w ADR 0009, punkt 7: zostaje |

## Różnice względem oczekiwań zestawu W3C (testy dokumentów, etap 5)

| Przypadek | `goxsdgo` | Oczekiwane | Uzasadnienie | Skutek dla odbiorcy |
|---|---|---|---|---|
| Saxonica XmlVersions xv002–xv100 | według znaków nazw XML 1.0 2e | według XML 1.0 5e albo XML 1.1 | typy nazw (`Name`, `NCName`, `NMTOKEN`) w XSD 1.0 2e opierają się na XML 1.0 2e | nazwy z nowszymi znakami odrzucane; w korpusie MF bez znaczenia |
| MS gMonth002, gMonth004 | „--MM” | „--MM--” | wydanie drugie (część 2, 3.2.14, z erratą) podaje postać „--MM”; zestaw zakłada pierwsze wydanie | wartości „--MM--” odrzucane |
| MS anyURI_a004 | według etapu 2 | inaczej | literał `anyURI`, jak `anyURI_b006` wyżej | rozstrzygnięte w ADR 0009, punkt 7: zostaje |
| MS stZ073b | poprawny | błędny | restrykcja unii przyjmuje `xsi:type` wskazujący typ członka (ADR 0008, punkt 11) | rozstrzygnięte w ADR 0009, punkt 7: zostaje |
| MS attP031 | błędny | poprawny | atrybut zabroniony (`use="prohibited"`) jest niedozwolony w dokumencie (ADR 0008, punkt 11) | rozstrzygnięte w ADR 0009, punkt 7: zostaje |
| MS particlesZ035_a, particlesZ036_b1, particlesZ036_b2, particlesZ036_c (dokumenty) | brak werdyktu: `ErrLimit` („content model … needs more than 1024 configurations”) | niepoprawny (Z035_a), poprawny (pozostałe) | modele zgodne z UPA, ale niejednoznaczne co do liczników (`maxOccurs` od 6 000 do 100 000 000 000 zagnieżdżone w powtórzeniach); dopasowanie przekracza `MaxConfigurations` (ADR 0007, ADR 0011); decyzja właściciela z 2026-11-25: zostaje `ErrLimit` (pytanie 9) | taki dokument kończy się błędem limitu biblioteki, nie werdyktem; w korpusie MF takich modeli nie ma |
| korzeń bez deklaracji globalnej | błąd `cvc-elt.1` | według zestawu | ocena korzenia jest ścisła; luźna ocena dotyczy tylko symboli wieloznacznych z `processContents="lax"` (ADR 0008) | dokument z nieznanym korzeniem jest odrzucany, jak w libxml2 |

Pozostałe niezaliczone testy dokumentów (Missing, errF001, ste110, schU3–schU5, particlesZ001 i pokrewne) wynikają ze
schematów odrzuconych z powodów opisanych wyżej.

## Wyłączenia z zestawu W3C

| Grupa / przypadek | Powód |
|---|---|
| przypadki XSD 1.1 (`version="1.1"` na zestawie, grupie albo przypadku): 2 250 | poza zakresem (ADR 0001) |
| 9 grup z konstrukcjami XSD 1.1 bez żetonu wersji w metadanych: Saxonica VC vc001, vc002, vc014, vc024-11 (`vc:`, `xs:error`); IBM anyAttribute s3_10_6ii01, s3_10_6ii02, s3_10_6ii04 i wildcard s3_10_1ii08, s3_10_1ii09 (`notQName`, `notNamespace`) | w XSD 1.0 te schematy są błędne, a zestaw oczekuje „poprawny”; lista `internal/w3c.ExcludedGroups`, decyzja właściciela z 2026-11-25 |
| oczekiwany wynik `indeterminate`: 16 | rekomendacja nie rozstrzyga wyniku (metadane zestawu, `common/xsts.xsd`) |
| brak oczekiwanego wyniku dla XSD 1.0 2e: 7 | oczekiwane wyniki podano tylko dla wersji Unicode albo XSD 1.1; pytanie 7 |
| przypadek tylko dla Unicode 4.0.0 (`version="Unicode_4.0.0"`): 1 | wersja Unicode do decyzji; pytanie 7 |
| przypadki wymagające walidacji DTD | poza zakresem (ADR 0001); lista powstanie w etapie 1 |

Liczby z 2026-11-25 dla zestawu z commitu `7bc3365c`; liczy je `TestW3CSuite`. W zakresie zostaje 39 461 przypadków:
14 393 testy schematów i 25 068 testów dokumentów. Przypadki ze statusem `queried`, `disputed-test` i `disputed-spec`
zostają w zakresie, dopóki nie rozstrzygniemy ich pojedynczo.
