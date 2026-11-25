# Różnice względem rekomendacji i walidatora odniesienia

Każda różnica werdyktu między `goxsdgo` a libxml2 (przez `goxsdval`) i każde wyłączenie przypadku z zestawu W3C ma tu
wpis: przypadek, oba wyniki, co mówi rekomendacja (rozdział i reguła), kto ma rację, skutek dla odbiorcy.

| Przypadek | `goxsdgo` | libxml2 | Rekomendacja | Skutek dla odbiorcy |
|---|---|---|---|---|
| `msData/additional/ns.xml`, `msData/datatypes/Facets/anyURI/anyURI_a002.xml` i `.xsd` (dobra forma) | przyjmuje | odrzuca nazwę przestrzeni nazw jako „nie URI” | Namespaces in XML 1.0, §2.2: niezgodność z RFC 3986 nie jest błędem dobrej formy | plik dotąd odrzucany zostanie przyjęty przez czytnik |
| dokumenty z odwołaniem do encji zewnętrznej, z deklaracjami potrzebnymi z podzbioru zewnętrznego albo z sekcjami warunkowymi w encjach parametrycznych | odrzuca (`unsupported-*`, ADR 0004) | czyta encje zewnętrzne, jeśli są dostępne | rekomendacja pozwala procesorowi bez walidacji DTD nie czytać encji zewnętrznych (XML 1.0, §5.1), ale wtedy treść jest niepełna | plik dotąd przyjmowany zostanie odrzucony; w korpusie MF takich nie ma |

| MS-Regex: reJ11, reJ13, reJ19, reJ21, reJ23, reJ25, reJ29, reJ31, reJ33, reJ35, reJ61, reJ69, reJ75, reJ77, reS42, reT63 | kategorie znaków spoza BMP według Unicode 15 (np. U+1D7A8 to `Lu`, U+1D7FF to `Nd`) | — (porównanie w etapie 3) | zestaw zakłada Unicode 3.0, w którym znaki spoza BMP nie mają kategorii; ADR 0005, pytanie 7 | wzorce z `\p{Lu}`, `\d` itd. przyjmą znaki spoza BMP; w korpusie MF bez znaczenia |
| MS-Regex: reZ006i | U+0346 poza `\c` | — | produkcja CombiningChar z XML 1.0 2e kończy pierwszy przedział na U+0345; zestaw oczekuje U+0346 w `\c` | bez znaczenia dla korpusu MF |

## Wyłączenia z zestawu W3C

| Grupa / przypadek | Powód |
|---|---|
| przypadki XSD 1.1 (`version="1.1"` na zestawie, grupie albo przypadku): 2 250 | poza zakresem (ADR 0001) |
| oczekiwany wynik `indeterminate`: 16 | rekomendacja nie rozstrzyga wyniku (metadane zestawu, `common/xsts.xsd`) |
| brak oczekiwanego wyniku dla XSD 1.0 2e: 7 | oczekiwane wyniki podano tylko dla wersji Unicode albo XSD 1.1; pytanie 7 |
| przypadek tylko dla Unicode 4.0.0 (`version="Unicode_4.0.0"`): 1 | wersja Unicode do decyzji; pytanie 7 |
| przypadki wymagające walidacji DTD | poza zakresem (ADR 0001); lista powstanie w etapie 1 |

Liczby z 2026-11-25 dla zestawu z commitu `7bc3365c`; liczy je `TestW3CSuite`. W zakresie zostaje 39 461 przypadków:
14 393 testy schematów i 25 068 testów dokumentów. Przypadki ze statusem `queried`, `disputed-test` i `disputed-spec`
zostają w zakresie, dopóki nie rozstrzygniemy ich pojedynczo.
