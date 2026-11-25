# 0008 Walidacja dokumentu: strumień, ścieżki, pola błędów, limity, wskazówki lokalizacji

Status: przyjęta (etap 5) · 2026-11-25

## Kontekst

Etapy 1–4 dały czytnik XML, typy proste, wczytanie schematu i automat modeli zawartości. Etap 5 ma wykonać ocenę
dokumentu według reguł z części 1 (rozdział 3, Validation Rules, i rozdział 5) strumieniowo, w pamięci niezależnej
od rozmiaru pliku (K6), z błędami dla ludzi (ADR 0001 p. 6), powtarzalnie i współbieżnie, oraz dać pierwsze liczby
K1 dla 25 055 testów dokumentów zestawu W3C, K2 dla korpusu MF i K3 względem libxml2.

## Decyzja

1. **Architektura.** Pakiet `internal/validate` czyta dokument raz, tokenami `internal/xmlreader`, i trzyma stos
   ramek otwartych elementów: deklaracja, typ faktyczny, dopasowanie modelu (`schema.Matcher`), tekst tylko dla
   treści prostej (i dla treści mieszanej z wartością stałą), liczniki rodzeństwa do ścieżek. Ramki wracają do
   puli, więc nie ma alokacji na element. Pamięć zależy od głębokości, tabeli ID/IDREF i tabel ograniczeń
   tożsamości. `xsd.Set` jest tylko do odczytu; stan walidacji powstaje przy każdym wywołaniu, więc jeden `Set`
   waliduje wiele dokumentów równocześnie.
2. **Ocena elementu (3.3.4).** Korzeń: deklaracja globalna, a bez niej poprawny `xsi:type`, inaczej `cvc-elt.1`
   i ocena luźna. Dziecko: deklaracja z modelu zawartości (z członkami grup podstawienia, etap 4); symbol
   wieloznaczny `skip` pomija poddrzewo, `lax` używa deklaracji globalnej, jeśli jest, `strict` jej wymaga
   (`cvc-elt.1`). Po pierwszym błędzie modelu w elemencie (`cvc-complex-type.2.4`) dalsze dzieci są oceniane
   luźno, bez kolejnych błędów modelu: jeden błąd w złym miejscu nie daje setek pochodnych.
3. **`xsi:type`** zastępuje typ deklaracji, jeśli jest poprawnie wyprowadzony przy sumie `block` deklaracji
   i `block` typu (typ złożony) albo samym `block` deklaracji (typ prosty), według Type Derivation OK
   (`schema.TypeDerivationOK`). Inaczej `cvc-elt.4.3` i ocena typem deklaracji.
4. **Wartości stałe** elementów porównujemy w przestrzeni wartości typu faktycznego: gdy `xsi:type` zmienił typ,
   postać kanoniczną wartości stałej (według typu deklaracji) odczytujemy typem faktycznym (5.2.2.2.2: „match the
   canonical lexical representation”). Wartość stała atrybutu lokalnego należy do użycia atrybutu (3.2.2), więc jej
   naruszenie to `cvc-au`, a atrybutu globalnego `cvc-attribute.4`.
5. **Ograniczenia tożsamości (3.11.4, 3.11.5)** strumieniowo: podzbiór XPath z 3.11.6 ma tylko osie child,
   attribute i „.//” na początku, więc selektor i pola sprawdzamy po nazwach elementów od węzła kontekstu w dół,
   bez drzewa. Pole liczy węzeł raz, także gdy pasuje do kilku alternatyw (`@id|@id`). Atrybuty `xsi:*`
   i atrybuty z wartości domyślnych są węzłami dla pól. Ciąg kluczowy porównujemy po rodzaju pierwotnym i postaci
   kanonicznej (równość w przestrzeni wartości). Tabele węzłów przechodzą do rodzica (3.11.5, z usuwaniem
   konfliktów) tylko dla ograniczeń, do których odwołuje się jakiś `keyref`.
6. **ID/IDREF (3.15.5)**: ID z atrybutów, treści i wartości domyślnych; powtórzenie `cvc-id.2` od razu, IDREF bez ID
   `cvc-id.1` na końcu dokumentu, w miejscu odwołania. ENTITY i NOTATION sprawdzamy względem encji nieparsowanych
   dokumentu i notacji schematu; reguły nie mają nazw w rekomendacji, kody `cvc-entity` i `cvc-notation` są nasze.
7. **Wskazówki `xsi:schemaLocation` i `xsi:noNamespaceSchemaLocation`** są domyślnie pomijane (ADR 0001).
   `Config.SchemaLocationHints` je włącza: dokument wskazówki jest czytany tylko z `fs.FS` (wpis w `Locations`
   albo ścieżka względna wobec nazwy dokumentu z `ValidateDocument`), nigdy z sieci; adres bezwzględny bez wpisu
   jest pomijany (strategia 1 z 4.3.2). Schemat poszerzony o wskazane dokumenty jest kompilowany raz na zbiór
   ścieżek i trzymany w `Set`. Wskazówka, która dodaje dokument po pierwszym wystąpieniu elementu albo atrybutu jej
   przestrzeni, to błąd `schema-location-hint` (4.3.2: „it is an error if such an attribute occurs after the first
   appearance…”); powtórzenie tej samej wskazówki nie jest błędem. Z `Compile` bez dokumentów i z wskazówkami
   schemat wskażą dopiero dokumenty. Test zestawu W3C włącza wskazówki dla wszystkich przypadków, bo dokumenty
   zestawu z nich korzystają także w grupach ze schematem w metadanych.
8. **Pozycja i ścieżka błędu.** `Line` i `Column` elementu to pozycja tuż za końcem jego znacznika początkowego
   (`xmlreader.Token.EndLine`, `EndColumn`), czyli tam, gdzie zaczyna się treść; tak samo wskazuje element libxml2,
   co daje to samo miejsce pierwszego błędu (K3). Błąd atrybutu ma pozycję atrybutu, błąd dobrej formy pozycję
   z czytnika. `Path` to nazwy lokalne bez prefiksów i przestrzeni, `[n]` od drugiego wystąpienia nazwy wśród
   rodzeństwa (pierwsze wystąpienie bez indeksu, bo w strumieniu nie wiadomo, czy będzie drugie), `/@nazwa` dla
   atrybutu: `/JPK/Ewidencja/SprzedazWiersz[3]/P_15`. Ścieżka jest liczona dopiero przy błędzie.
9. **Pola błędu** (`xsd.Error`, zgodnie wstecz): `Code`, `Line`, `Column`, `Path`, `Message` oraz nowe `Name`
   (nazwa lokalna węzła), `Value` (wartość, której dotyczy błąd), `Facet` (naruszony aspekt z wartością, np.
   „maxInclusive 12”), `Expected` (dozwolone elementy dla `cvc-complex-type.2.4`, „nazwa” albo „{przestrzeń}nazwa”,
   symbol wieloznaczny jako „*”). Kody wartości to kody reguł części 2 z `internal/datatypes`
   (`cvc-pattern-valid`, `cvc-datatype-valid.1.2.1`…), a nie ogólne `cvc-type.3.1.3`: odbiorca tłumaczy po
   najbardziej szczegółowej regule.
10. **Limity i wyniki.** Błędy w kolejności dokumentu do `Config.MaxErrors` (domyślnie 1000), potem
    `ErrTooManyErrors` razem z błędami. Błąd dobrej formy to błąd dokumentu z kodem czytnika i koniec walidacji.
    Encja zewnętrzna i inne konstrukcje `unsupported-*` czytnika dają `errors.ErrUnsupported`; limity czytnika
    (`limit-*`), tekst elementu ponad 16 MiB, ponad `MaxIdentityEntries` (domyślnie 4 194 304) wpisów ID
    i ograniczeń tożsamości oraz `schema.MaxConfigurations` dopasowania dają `ErrLimit`. To ograniczenia
    biblioteki, nie werdykt o dokumencie.
11. **Odstępstwa i rozstrzygnięcia** (przypadki w `differences.md`):
    - korzeń bez deklaracji jest błędem (`cvc-elt.1`), choć rekomendacja przewiduje dla niego ocenę luźną
      i wynik „notKnown”: walidator ma odpowiedzieć „tak” albo „nie” (jak libxml2 i zestaw, który oczekuje
      „invalid”);
    - restrykcja unii (`stZ073b`): typ pochodny przez restrykcję z unii ma {variety} union i {member type
      definitions} bazy, więc `xsi:type="xs:integer"` jest poprawnie wyprowadzony według 3.14.6 punkt 2.2.4;
      zestaw oczekuje „invalid” (XSD 1.1 zaostrzył tę regułę);
    - zabroniony atrybut z wartością stałą (`attP031`): `use="prohibited"` nie tworzy użycia atrybutu, więc atrybut
      jest niedozwolony (`cvc-complex-type.3.2.1`); zestaw oczekuje „valid”.
12. **Czego nie robimy** w etapie 5: PSVI jako API, wartości typowane w wyniku (pytanie 6), walidacja od elementu
    innego niż korzeń dokumentu, unie z typem ID w tabeli ID/IDREF (wartość unii nie wiąże ID).

## Konsekwencje

+ 39 367 z 39 439 przypadków W3C (testy dokumentów 24 997 z 25 055), korpus MF i dokumenty syntetyczne 15 z 15
  z kodem, wierszem i ścieżką; z libxml2 ten sam werdykt w 99,69% porównywalnych testów dokumentów i ten sam wiersz
  pierwszego błędu w 99,7% (pomiary 2026-11-25);
+ plik 1 GB walidowany przy szczycie pamięci 5,8 MB (K6); ok. 1,3× czasu `xmllint --stream` na pliku 200 MB (K7);
− pozycja elementu to koniec znacznika początkowego, nie jego początek: przy znaczniku na kilku wierszach wskazuje
  ostatni wiersz;
− dalsze przyspieszenie wymaga szybkich ścieżek typów liczbowych bez `math/big` (etap 7).
