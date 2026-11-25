# 0012 Reguły znaków nazw jako opcja; odwołania nierozwiązane bez zmian

Status: przyjęta · 2026-11-25 (decyzje właściciela po porównaniu z knroy/go-xml)

## Kontekst

Porównanie z knroy/go-xml jako czarną skrzynką (`tools/bench`, README, „Comparison”) pokazało 42 przypadki W3C, które
knroy przechodzi, a goxsdgo nie. Dwie grupy mają po stronie zestawu mocne argumenty:

- **XmlVersions** (Saxonica) i `reZ006i` (Microsoft): wartości typów nazw i klasy `\i`, `\c` według znaków nazw
  z XML 1.0 Fifth Edition. XSD 1.0 2e odsyła do XML 1.0 Second Edition (część 2, 3.3.x; dodatek F), ale nasz czytnik
  czyta nazwy elementów i atrybutów według Fifth Edition (ADR 0004): ta sama nazwa bywała dobra jako nazwa elementu,
  a zła jako wartość `xs:NCName`.
- **Missing** (Saxonica): nierozwiązane odwołanie w nieużywanej deklaracji nie czyni schematu błędnym (część 1,
  §4.2.3: „implementations must retain QName values for such references, in case an appropriately-named component
  becomes available to discharge the reference by the time it is actually needed”; §5.3 Missing Sub-components).

## Decyzja

1. **Reguły znaków nazw są opcją** `xsd.Config.NameRules`:
   - `NamesXML10Edition2` (wartość zerowa, domyślna): XML 1.0 Second Edition, dodatek B. Dosłowny odsyłacz XSD 1.0
     2e; zgodne z libxml2 dla wartości typów nazw (K3). Wynik jest taki sam jak przed opcją: zrzut wszystkich błędów
     W3C i korpusu (`TestDumpErrors`, 56 524 wiersze) identyczny przed i po.
   - `NamesXML10Edition5`: XML 1.0 Fifth Edition, produkcje [4] NameStartChar i [4a] NameChar.

   Opcja obejmuje typy Name, NCName, NMTOKEN(S), ID, IDREF(S), ENTITY(IES), QName i NOTATION (część lokalna
   i prefiks), klasy `\i`, `\c`, `\I`, `\C` we wszystkich wzorcach (wbudowanych i ze schematu), nazwy komponentów,
   wartości QName i nazwy w selector/field w dokumentach schematu oraz wartości w walidowanym dokumencie. Nazwy
   elementów i atrybutów dokumentu czytnik zawsze czyta według Fifth Edition (bez zmian, ADR 0004).

   Budowa: tablica Fifth Edition ma jedno źródło, `internal/xmlchars` (czytnik i tłumacz wyrażeń z niej korzystają;
   zachowanie czytnika bez zmian). Typy wbudowane zostają wspólne: wzorzec, który używa `\i`, `\c`, `\I` albo `\C`,
   jest kompilowany w dwóch wariantach (`regex.Regexp.MatchStringNames`), a reguła płynie w `datatypes.Context.Names`
   (schemat: `document.names` z `schema.Config.NameRules`; dokument: `validate.Options.Names`). Wzorce bez tych klas
   kompilują się raz, jak dotąd.

   Pomiar (2026-11-25, `TestW3CSuiteNameRules5`): domyślnie 39 367 / 39 439, z Fifth Edition 39 383 / 39 439:
   +16 (15 z XmlVersions, `reZ006i`), 0 utraconych. W XmlVersions zostają `xv003.v01`, `xv006.v01`, `xv008.v01`:
   odwołania do znaków dozwolonych tylko w XML 1.1, dokument nie jest dobrze uformowany według XML 1.0 — to nie
   sprawa reguł nazw. K1 liczymy w ustawieniu domyślnym; test pilnuje tylko, że opcja zmienia wyłącznie przypadki
   zależne od znaków nazw (bez osobnej zapadki: drugi plik zapadki dublowałby pierwszy na 39 400 przypadkach).

2. **Odwołania nierozwiązane: bez zmian** (zostaje `src-resolve` przy kompilacji, ADR 0006). Zgoda właściciela
   dotyczyła wdrożenia pod warunkiem, że da się pogodzić grupę Missing z ok. 43 przypadkami Sun i Microsoft, które
   oczekują `src-resolve`. Sprawdziliśmy rodzaje odwołań w testach schematów W3C, w których `src-resolve` jest
   jedynym błędem (2026-11-25): 46 przypadków, 42 oczekuje schematu błędnego, 4 (missing001, 002, 003, 006)
   poprawnego. Według rodzaju odwołania (przypadek z dwoma rodzajami liczony w obu): `attributeGroup/@ref` 11,
   `element/@type` 6 (w tym missing001 i missing003), `attribute/@ref` 6, `keyref/@refer` 5, `restriction/@base` 5
   (w tym missing004 i missing005, które oczekują błędu), `element/@ref` 3, `group/@ref` 3, `attribute/@type` 2,
   `element/@substitutionGroup` 2 (missing002, missing003), `list/@itemType` 1 (missing006), `union/@memberTypes` 1,
   inne 2. Kolejne 55 testów oczekujących błędu ma `src-resolve` obok innych błędów. Rodzaj odwołania nie rozdziela
   grup: identyczna konstrukcja ma sprzeczne oczekiwania —
   `elemM002` (`<xsd:element name="myElem" type="foo"/>`, bez przestrzeni docelowej, nieużywany) oczekuje błędu,
   `missing001` (`<xs:element name="bad" type="absent"/>`, ta sama sytuacja) poprawnego schematu; tak samo
   `xsd015`, `xsd016` i `idB005` wobec `missing001` i `missing003`. Zasady, która godzi obie grupy, tekst rekomendacji
   nie daje (`src-resolve` jest ograniczeniem reprezentacji, a §4.2.3 i §5.3 dopuszczają odroczenie), więc jej nie
   wymyślamy. Grupa Missing zostaje różnicą w `differences.md`; decyzję może zmienić tylko właściciel (np. przyjąć
   odroczenie jak w XSD 1.1 kosztem przypadków Sun i Microsoft).

## Konsekwencje

+ odbiorca, który chce nazw jak w dokumentach (i jak w XSD 1.1), włącza jedną opcję; domyślnie nic się nie zmienia;
+ jedno źródło tablicy Fifth Edition dla czytnika i typów;
− wzorce z `\i`/`\c` mają dwa skompilowane warianty (pamięć tylko dla tych wzorców);
− grupa Missing zostaje niezaliczona (12 przypadków).
