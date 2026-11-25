# 0006 Wczytanie schematu

Status: proponowana (etap 3) · 2026-11-25

## Kontekst

Etap 3 zamienia dokumenty schematu w komponenty (XSD 1.0 2e, część 1, rozdziały 3 i 4) i sprawdza ograniczenia
reprezentacji (src-*) oraz ograniczenia komponentów. Modele zawartości (UPA, EDC, restrykcja cząstek) należą do
etapu 4; walidacja dokumentów do etapu 5. Rekomendacja zostawia kilka miejsc do interpretacji, a zestaw W3C bywa
w nich niespójny; ten ADR zapisuje, jak je czytamy.

## Decyzja

1. **Pakiet i model.** `internal/schema` buduje komponenty jako struktury Go odpowiadające tablicom właściwości
   z rozdziału 3 (`SimpleType`, `ComplexType`, `ElementDecl`, `AttributeDecl`, `AttributeUse`, `AttributeGroup`,
   `ModelGroupDef`, `ModelGroup`, `Particle`, `Wildcard`, `IdentityConstraint`, `Notation`). Typy proste trzymają
   typ z pakietu `datatypes` (etap 2). `Schema` ma indeksy komponentów globalnych oraz listy wszystkich typów
   złożonych i deklaracji elementów (także lokalnych) dla etapu 4. `xsd.Config.Compile` wywołuje `schema.Load`.
2. **Dokument schematu w pamięci.** Dokumenty schematu są małe, więc czytamy je czytnikiem z etapu 1 do drzewa,
   z deklaracjami przestrzeni nazw każdego elementu (nowa metoda `xmlreader.Reader.DeclaredNamespaces`), bo
   wartości QName w atrybutach rozwiązuje się według [in-scope namespaces] (3.15.3). Walidacja dokumentów (etap 5)
   pozostaje strumieniowa.
3. **Schemat dla schematów.** Warunki „imposed by the schema for schemas” sprawdzamy z tablicy zapisanej ręcznie
   z dodatku A części 1 i 2: kontekst elementu wynika z rodzica (np. `topLevelElement`, `localElement`,
   `narrowMaxMin`, `simpleExplicitGroup`), model treści jest wyrażeniem nad nazwami dzieci, atrybuty mają typy
   sprawdzane typami wbudowanymi z etapu 2. Atrybuty z innych przestrzeni są dozwolone; `xml:lang`, `xml:space`,
   `xml:base`, `xml:id` sprawdzamy według schematu przestrzeni xml, który schemat dla schematów importuje (lax).
   Wartości `id` muszą być niepowtarzalne w dokumencie. Rekomendacja nie nazywa tych reguł; kody są nasze, z
   przedrostkiem `sfs-`. Wyrażeń selector i field nie sprawdzamy wzorcem ze schematu dla schematów (nie dopuszcza
   białych znaków), tylko gramatyką z 3.11.6, która je dopuszcza.
4. **Lokalizacja dokumentów, bez sieci.** `schemaLocation` rozwiązujemy w kolejności: `Config.Locations` dla adresu
   dosłownego; `Config.Locations` dla adresu rozwiązanego względem adresu bazowego dokumentu (adres bazowy ma
   dokument wczytany przez `Locations`); ścieżka względna w `fs.FS` z dokładną wielkością liter na każdym poziomie
   (wynik nie zależy od systemu plików) i bez wyjścia poza `fs.FS`. Adres bezwzględny bez wpisu nie jest czytany.
   Import bez `schemaLocation` albo z adresem nierozwiązywalnym korzysta z nowego `Config.Namespaces`
   (przestrzeń → ścieżka; strategia 3 z 4.3.2). Nierozwiązane include i import nie są błędem (4.2.1, 4.2.3);
   redefine z dziećmi musi się rozwiązać (src-redefine.1). Import przestrzeni XSD pomijamy, bo jej komponenty są
   wbudowane; dla przestrzeni xml i xsi mamy wbudowane deklaracje atrybutów, używane, gdy żaden dokument ich nie
   dostarcza (strategia 1 z 4.3.2).
5. **Tożsamość dokumentu i kameleon.** Klucz dokumentu to ścieżka i docelowa przestrzeń po zamianie kameleona. Ten
   sam plik podany wprost i włączony daje te same komponenty; plik bez przestrzeni włączony przez dokumenty
   o różnych przestrzeniach daje osobne komponenty. W kameleonie brak przestrzeni zastępujemy przestrzenią
   włączającego w nazwach komponentów, nazwach lokalnych kwalifikowanych, odwołaniach QName i symbolach
   wieloznacznych `##targetNamespace` i `##other` (4.2.1, punkt 3.2); `##local` zostaje brakiem przestrzeni.
6. **Rozwiązywanie nazw.** Przestrzeń XSD jest dostępna bez importu (punkt 4 z 3.15.3 wymienia tylko docelową
   przestrzeń i importy; tak czytają to wszystkie przypadki zestawu). Nierozwiązane odwołanie jest błędem
   `src-resolve`, mimo że 5.3 opisuje brakujące komponenty jako problem dopiero walidacji: tak oczekują przypadki
   Sun i Microsoft (43), przeciwnie tylko 4 przypadki grupy Missing (Saxonica), opisane w `differences.md`.
7. **Budowanie leniwe.** Komponenty globalne mają stan (nowy, w budowie, gotowy). Odwołanie do komponentu w budowie
   przez typ bazowy, element listy, typ składowy unii, grupę, grupę atrybutów albo głowę grupy podstawienia to cykl
   z kodem właściwej reguły (`st-props-correct.2`, `src-simple-type.4`, `ct-props-correct.3`, `mg-props-correct.2`,
   `src-attribute_group.3`, `e-props-correct.6`). Anonimowe typy złożone w deklaracjach elementów budujemy z kolejki
   po bieżącym komponencie, żeby rekurencja przez deklarację elementu (dozwolona) nie wyglądała jak cykl grupy.
   Kontrole wymagające kompletu (wartości domyślne i stałe, wyprowadzenia, keyref) działają na końcu.
8. **Redefine.** Redefinicja zastępuje wpis w indeksie, więc działa wszędzie, także w dokumencie redefiniowanym.
   Oryginał zostaje bez nazwy (Individual Component Redefinition 1.1); typ redefiniujący dostaje go jako typ
   bazowy (tylko przez atrybut `base`), a odwołanie grupy albo grupy atrybutów do siebie w poddrzewie redefinicji
   wskazuje oryginał. Komponent redefiniowany może pochodzić z dokumentu redefiniowanego albo z jego włączeń.
   Łańcuchy redefinicji (A redefiniuje B, które redefiniuje C) są dozwolone.
9. **Błędy.** `schema.Load` zbiera wszystkie wykryte błędy w kolejności dokumentów i elementów; `xsd.Compile`
   zwraca `*xsd.SchemaError` z listą `xsd.Error` (nowe pole `Document`, wiersz, kolumna, kod, komunikat). Kody to
   kotwice reguł z rekomendacji z numerem punktu (`src-resolve.4.2`, `derivation-ok-restriction.2.1.1`), dla aspektów
   kody części 2 z pakietu `datatypes`, a własne tylko tam, gdzie rekomendacja reguły nie nazywa (`sfs-*`,
   `schema-limit`).
10. **Uczciwość pomiaru.** `Compile` zwraca nil tylko po przetworzeniu wszystkich konstrukcji etapu 3. Jedyne dziś
    konstrukcje spoza zakresu to te, których nie obsługuje czytnik (np. encja zewnętrzna w dokumencie schematu):
    dają błąd opakowujący `errors.ErrUnsupported`. Kontrole etapu 4 nie są jeszcze zrobione, więc schemat
    naruszający tylko je przechodzi jako poprawny: w zestawie W3C to fałszywe przyjęcia, nigdy fałszywe zaliczenia.
    Dokument z grupy, której schemat odrzucamy, zaliczamy w `TestW3CSuite` tylko wtedy, gdy zestaw też uznaje
    schemat za błędny.
11. **Interpretacje.**
    - Wildcard Subset: not(x) jest podzbiorem not(absent) dla każdego x, bo not(x) zawsze wyklucza brak przestrzeni
      (Wildcard allows Namespace Name 2.3). Litera punktu 2.2 wymaga tej samej wartości; zestaw (wildZ013) oczekuje
      naszego odczytania.
    - Notacja musi mieć `public` albo `system`: erratum E1-16 uczyniło `public` opcjonalnym, a przypadek Sun
      `publicId00201m1` (bez obu) oczekuje błędu.
    - `targetNamespace=""` jest błędem (`sch-props-correct.1`): pusty napis nie jest nazwą przestrzeni (3.15.2).
    - anySimpleType nie jest typem bazowym restrykcji (`cos-st-restricts.1.1`).
    - Liczności większe niż int64 obcinamy do `math.MaxInt64`; porównania min ≤ max pozostają poprawne poza
      skrajnymi liczbami powyżej 9,2·10^18.
    - Pusta sekwencja albo `all` (i pusty `choice` z `minOccurs="0"`) nie tworzy cząstki (3.4.2, punkt 2.1), więc nie
      podlega Particle Correct.
12. **Bezpieczeństwo.** Limity `Config.MaxDocuments` (domyślnie 1000) i `Config.MaxDepth` (domyślnie 64) z kodem
    `schema-limit`; cykle include, import i redefine kończy klucz dokumentu.
13. **Zmiany API.** `xsd.Config` dostaje `Namespaces`, `MaxDocuments`, `MaxDepth`; `Compile` zwraca `*SchemaError`;
    `Error` ma pole `Document`. `Compile(fsys, names...)` i `Config{Locations}` działają jak dotąd.

## Co zostaje dla etapu 4

Unique Particle Attribution (`cos-nonambig`), Element Declarations Consistent (`cos-element-consistent`), Particle
Valid (Restriction) (`cos-particle-restrict` z podregułami NameAndTypeOK, NSCompat, Recurse, RecurseLax,
RecurseUnordered, MapAndSum, NSSubset, NSRecurseCheckCardinality), a przez nią `derivation-ok-restriction.5.4.2`
i `src-redefine.6.2.2`, oraz `cos-ct-extends.1.5`. Miejsca w kodzie są oznaczone (`checks.go`, `load.go`).
Według przeglądu z 2026-11-25 dotyczy to 324 z 325 testów schematów W3C oczekujących błędu, których dziś nie
odrzucamy (265 z grupy MS-Particles, 24 z MS-ModelGroups, reszta to UPA, EDC i redefinicje grup).

## Konsekwencje

+ testy schematów W3C: 14 048 / 14 393; schematy MF: 170 / 170;
+ komponenty są gotowe dla etapów 4 i 5, a kody błędów odpowiadają regułom rekomendacji;
− do etapu 4 schematy naruszające UPA albo restrykcję cząstek są przyjmowane;
− tablica schematu dla schematów jest zapisana ręcznie; zmiana dodatku A (np. w erracie) wymaga jej zmiany.
