# 0001 Zakres i kryteria zastąpienia walidatora odniesienia

Status: przyjęta (robocza) · 2026-11-24

## Kontekst

Odbiorcy walidują dziś XML według XSD przez `goxsdval`: cgo nad libxml2. Działa i jest sprawdzony na plikach JPK,
KSeF i e-Sprawozdań, ale wnosi cgo (budowanie z tagiem, bez kompilacji krzyżowej, biblioteka systemowa w wersji
co najmniej 2.15), libxml2 jako składnik zewnętrzny w SBOM i IQ oraz komunikaty libxml2, które odbiorca tłumaczy
po kodzie. `goxsdgo` ma go zastąpić dopiero wtedy, gdy będzie co najmniej tak samo poprawny na tych samych danych,
a przy tym w czystym Go i w całości nasz (rodowód: `../provenance.md`).

## Decyzja

1. **Zakres.**
   - W3C XML Schema **1.0 Second Edition** (2004-10-28): część 1 (Structures) i część 2 (Datatypes), w całości
     z punktu widzenia walidacji: typy proste i złożone, wszystkie typy wbudowane i aspekty (facets), grupy, grupy
     atrybutów, symbole wieloznaczne (`any`, `anyAttribute`, `processContents`), `xsi:type`, `xsi:nil`,
     `xsi:schemaLocation` (wyłączone domyślnie), grupy podstawienia, typy abstrakcyjne, `block` i `final`,
     ograniczenie i rozszerzenie z kontrolą poprawności wyprowadzenia, ograniczenia tożsamości (`unique`, `key`,
     `keyref` z podzbiorem XPath z rekomendacji), `include`, `import`, `redefine`, zasada jednoznaczności
     (Unique Particle Attribution) i zgodność deklaracji elementów (Element Declarations Consistent).
   - Własny czytnik XML 1.0 (Fifth Edition) z przestrzeniami nazw (Namespaces in XML 1.0), z dokładnym wierszem
     i kolumną (pytanie 3).
   - Poza zakresem: XSD 1.1, walidacja DTD (deklaracja DOCTYPE jest odrzucana albo pomijana według opcji, encje
     zewnętrzne nigdy nie są rozwijane), PSVI jako API, generowanie typów Go.
2. **Wyrażenia regularne XSD** (aspekt `pattern`) według dodatku F części 2: własny tłumacz na składnię RE2 tam, gdzie
   to możliwe (klasy `\i`, `\c`, bloki Unicode `\p{IsBasicLatin}`, odejmowanie klas), z gwarancją czasu liniowego.
3. **Modele zawartości** kompilowane do automatu z licznikami: `maxOccurs="100000"` nie rozwija się w 100 000 stanów.
4. **Bezpieczeństwo.** Schematy i dołączane pliki wyłącznie z `fs.FS` podanego przez wywołującego, bez sieci; limity
   głębokości, długości nazw, liczby atrybutów i rozmiaru tekstu konfigurowalne, z bezpiecznymi wartościami domyślnymi.
5. **Strumieniowo.** Walidacja dokumentu w jednym przejściu, bez budowania drzewa; pamięć zależy od głębokości
   i od ograniczeń tożsamości, nie od rozmiaru pliku.
6. **Błędy.** Każdy błąd ma stały kod (`goxsdgo`), wiersz, kolumnę, ścieżkę elementu, nazwę węzła i parametry (np.
   oczekiwane elementy, naruszony aspekt i jego wartość). Tabela `compat` przypisuje kodom `goxsdgo` kody libxml2, żeby
   odbiorca mógł użyć istniejących tłumaczeń. Walidacja zbiera wszystkie błędy z limitem liczby, w ustalonej kolejności.
7. **Determinizm.** To samo wejście i opcje dają te same błędy w tej samej kolejności.

## Kryteria zastąpienia `goxsdval`

Wszystkie muszą być spełnione jednocześnie, zmierzone i zapisane z datą w `../testing.md`:

| # | Kryterium | Miara |
|---|---|---|
| K1 | zgodność z rekomendacją | zestaw testów W3C XML Schema 1.0 (przypięty commit, pytanie 1): wszystkie przypadki schematów i dokumentów objęte zakresem dają oczekiwany werdykt; wyłączenia (XSD 1.1, DTD, przypadki ze sprzecznymi oczekiwaniami) wymienione w `../differences.md` z powodem |
| K2 | korpus odbiorców | wszystkie schematy z `testdata/schemas/` (JPK, KSeF, e-Sprawozdania z importami) kompilują się; każdy dokument z `testdata/instances/` daje werdykt z pliku oczekiwań |
| K3 | podobieństwo do libxml2 | ten sam werdykt dla ≥ 99,5% par schemat–dokument z korpusu i z wejść fuzzingu; to samo miejsce pierwszego błędu (wiersz i element) dla ≥ 95% dokumentów błędnych; **każda** różnica werdyktu opisana w `../differences.md` |
| K4 | wymagania | każde wymaganie z `../../requirements/` ma test; testy przechodzą |
| K5 | odporność | `FuzzCompile` i `FuzzValidate` bez paniki co najmniej 24 h łącznie na kandydacie do wydania; przypadki patologiczne (wzorce, liczności, zagnieżdżenia, długie atrybuty, wiele kluczy) w czasie liniowym |
| K6 | bezpieczeństwo i pamięć | testy: brak dostępu do sieci i encji zewnętrznych przy ustawieniach domyślnych; plik 1 GB waliduje się w mniej niż 64 MB pamięci |
| K7 | wydajność | nie wolniej niż 3× libxml2 (przez `goxsdval`) na korpusie, pomiar `go test -bench` |
| K8 | odbiorca | testy odbiorcy przechodzą po przepięciu; każdy kod `goxsdgo` występujący w korpusie ma odpowiednik w tabeli `compat` |
| K9 | dowód | pakiet dowodów do walidacji u odbiorcy: wymagania → testy → wynik ostatniego przebiegu |

Przepięcie odbiorcy to jego decyzja, podejmowana na podstawie tych wyników.

## Konsekwencje

+ odbiorcy nie czekają: używają `goxsdval` i zmieniają tylko implementację za własnym interfejsem;
+ po przepięciu znika cgo i libxml2 z budowania, SBOM i IQ;
+ kryteria są mierzalne i sprawdzane automatycznie;
− XSD 1.0 jest dużą rekomendacją; szacunek w `../roadmap.md` jest większy niż w pozostałych projektach rodziny;
− K3 wymaga modułu porównawczego z cgo i libxml2 poza biblioteką (`tools/compare`), budowanego tylko lokalnie i w osobnym
  zadaniu CI.
