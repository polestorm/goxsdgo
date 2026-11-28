# 0014 Czytnik XML z modułu goxmlgo; jedyna zależność

Status: przyjęta · 2026-11-27 (decyzja właściciela; etap 6a planu `docs/migration.md` w goxmlgo)
Zmiana: goxmlgo v0.5.0 i Go 1.26 — ADR 0015.

## Kontekst

Czytnik XML powstał w goxsdgo (ADR 0004, `internal/xmlreader`, publiczny `xmlreader` od v0.2.0). Korzystają z niego
też inne projekty (godocxgo, gopdfgo dla SVG), więc właściciel wydzielił go 2026-11-26 do osobnego publicznego modułu
`github.com/polestorm/goxmlgo` (pakiety `xmlreader` i `xmlchars`, tagi podpisane), gdzie jest rozwijany dalej (`Skip`,
`Reset`, `Limits.MaxElements`, `Token.Namespaces`, `NamespaceScope`, `TokenReader`, `Declaration`). Dwie kopie
czytnika z czasem by się rozjechały; poprawka deklaracji XML z v0.4.1 (KI-13) powstała w obu repozytoriach osobno.

Do v0.4.1 moduł goxsdgo nie miał żadnej zależności (`go.mod` bez `require`, sprawdzane w CI).

## Decyzja

1. **Jedyna zależność modułu biblioteki to `github.com/polestorm/goxmlgo`**, od v0.2.0 (v0.1.0 nie ma poprawki
   KI-13, którą goxsdgo ma od v0.4.1). goxmlgo sam nie ma zależności ani cgo, więc nadal: zero cgo, nic spoza
   biblioteki standardowej poza tym jednym modułem. CI sprawdza, że `go list -m all` podaje tylko goxmlgo.
   Podbicie wersji goxmlgo to zmiana goxsdgo z wpisem w `CHANGELOG.md` (klasa według wpływu na walidację) i tym
   samym zestawem pomiarów co przy tym przełączeniu (zrzut błędów `TestDumpErrors`, K1–K3, szybkość).
2. **Kod czytnika i jego testy są tylko w goxmlgo**: `internal/xmlchars` i kod `xmlreader` usunięte z goxsdgo;
   biblioteka importuje `goxmlgo/xmlreader` i `goxmlgo/xmlchars`. W goxmlgo są też: wymagania XML-001…018 (te same
   identyfikatory i treść), `TestXMLConformance` z zapadką zestawu W3C XML Conformance, `FuzzReader`, porównanie
   z `encoding/xml` i pomiary szybkości czytnika. W goxsdgo te wymagania są **delegowane** (odsyłacz z wersją,
   sprawdzany przez `TestRequirementsCovered`), a zestaw XML Conformance nie jest już pobierany.
3. **`goxsdgo/xmlreader` zostaje jako warstwa zgodności**, oznaczona `Deprecated`: aliasy typów, stałe i funkcje
   przekazujące z v0.3.0 (XML-020). Typy są tożsame z goxmlgo, więc program napisany dla v0.3.0 kompiluje się bez
   zmian, a nowości goxmlgo są dostępne przez aliasy; nowych funkcji przekazujących nie dopisujemy. Pakiet usuniemy
   najwcześniej w 1.0, z wpisem klasy 3.
4. **Kody błędów czytnika** (`xml-*`, `wfc-*`, `nsc-*`, `ns-qname`, `limit-*` czytnika, `unsupported-*`) zostają
   w katalogu goxsdgo (`xsd/codes.go`, `docs/errors.md`), bo użytkownik walidatora je dostaje, ze źródłem
   „goxmlgo/xmlreader”. `TestCatalogMatchesSource` porównuje tę część z kodami eksportowanymi przez goxmlgo w wersji
   z `go.mod`; kody, których walidator nie może dostać (`limit-elements`: goxsdgo nie ustawia `MaxElements`;
   `usage-skip`: goxsdgo nie woła `Skip`), są wymienione jawnie i nie trafiają do katalogu.
5. **Pomiar przełączenia**: przed i po zmianie ten sam zrzut błędów (`TestDumpErrors`), te same zapadki K1–K3,
   ten sam pomiar szybkości (`tools/bench perf`); wynik w `docs/results/goxmlgo-switch/`. Różnica w zachowaniu
   walidatora jest błędem do naprawy w goxmlgo, nie do obejścia w goxsdgo.

## Konsekwencje

- Fuzzing K5 goxsdgo obejmuje fuzzery walidatora i kompilatora; godziny `FuzzReader` liczy goxmlgo. Czytnik jest
  dalej fuzzowany pośrednio przez `FuzzValidate` i `FuzzCompile`.
- Pakiet dowodów goxsdgo podaje wersję goxmlgo i odsyła do jego pakietu dowodów dla wymagań delegowanych.
- Odbiorca walidujący goxsdgo (np. QMS) ma w SBOM jedną pozycję więcej: goxmlgo, z tym samym właścicielem,
  licencją i zasadami wydań (podpisane tagi, CHANGELOG z klasą wpływu).
