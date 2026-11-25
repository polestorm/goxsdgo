# 0009 Katalog kodów błędów, komunikaty i tabela compat do kodów libxml2

Status: przyjęta (etap 6) · 2026-11-25 · rozstrzyga pytanie 6 z `open-questions.md`

## Kontekst

Odbiorca tłumaczy błędy walidacji po kodzie: `goxsdval/messages/messages.json` ma polskie komunikaty dla 117 kodów
libxml2 (XML_ERR_* i XML_SCHEMAV_*), a błędy schematu (XML_SCHEMAP_*) tłumaczy jednym komunikatem ogólnym
(`generic.schema`). Kryterium K8 wymaga, żeby każdy kod `goxsdgo` miał odpowiednik w tych kodach. Po etapach 3–5
biblioteka emitowała kody nazwane według reguł rekomendacji, ale bez katalogu: nie było wiadomo, ile ich jest, co
znaczą, jakie pola ustawiają ani jak je przełożyć na kody libxml2. Część kodów powstawała przez doklejenie numeru
punktu do zmiennej, więc z kodu źródłowego nie dało się ich wyliczyć. Komunikaty powtarzały kod i surowe szczegóły,
a nazwy podawały w postaci `{http://crd.gov.pl/…}KodFormularza`.

## Decyzja

1. **Katalog w kodzie.** `xsd/codes.go` jest jedynym źródłem prawdy: dla każdego kodu rodzaj (`schema`, `document`,
   `limit`), rekomendacja, rozdział, odnośnik do kotwicy reguły, oznaczenie kodu własnego, parametry (pola `Error`,
   które błąd może ustawić) i opis po angielsku. API: `xsd.Codes()`, `xsd.LookupCode()`. `docs/errors.md` powstaje
   z katalogu (`go test ./xsd -run TestErrorsDoc -update`). Stan: 234 kody (schema 143, document 80, limit 11), 41
   własnych (`sfs-*`, `facet-fixed`, `facet-value`, `regex-*`, `cvc-entity`, `cvc-notation`,
   `schema-location-hint`, `schema-limit`, kody czytnika `xml-*`, `ns-qname`, `limit-*`, `unsupported-*`). Opisy są
   po angielsku, jak komunikaty, bo są częścią API.
2. **Katalog sprawdzany w obie strony.** `TestCatalogMatchesSource` czyta drzewo składni pakietów `internal/*` i zbiera
   kody, które mogą zostać wyemitowane: wartości stałych `Code*` i złożenia „stała + literał punktu”. Zbiór musi być
   równy katalogowi. Kod składany w czasie działania (zmienna + punkt) jest błędem testu, więc trzy takie miejsca
   zamieniliśmy na złożenia ze stałych (`role.notSchemaCode`, `role.namespaceCode`, tablice `attrRestrictionCodes`).
   Dodatkowo testy zgodności (`TestW3CSuite`, `TestInstanceCorpus`) sprawdzają każdy zwrócony błąd: kod jest
   w katalogu, rodzaj się zgadza, a ustawione pola są wśród zadeklarowanych parametrów.
3. **Poprawione kody** (werdykty bez zmian; zapadki bez zmian):
   - dokument wskazany przez `redefine` albo `import`, który nie jest poprawnym dokumentem schematu: `src-redefine.2`
     i `src-import.2` zamiast `src-redefine.1` i `src-import.1` (te punkty znaczą co innego: 4.2.2 punkt 1, 4.2.3
     punkt 1); `include` zostaje `src-include.1`;
   - dokument podany wprost, który nie jest schematem: `sfs-not-schema` zamiast przypadkowego `sfs-not-schema.1`;
   - usunięte nieużywane stałe: `cos-particle-restrict`, `cos-aw-intersect`, `wfc-in-dtd`,
     `src-identity-constraint`, `src-model_group` (restrykcję cząstek zgłasza `derivation-ok-restriction.5.4.2`
     albo `src-redefine.6.2.2`, ADR 0007).
4. **Komunikaty dla ludzi.** Kody są stałe, komunikaty mogą się zmieniać. Od tego etapu:
   - nazwy z przestrzeni nazw elementu głównego są podawane bez przestrzeni („unexpected element Miesiac in
     Naglowek; expected Rok”), nazwy z innych przestrzeni w postaci `{przestrzeń}nazwa`; pola `Name` i `Expected` bez
     zmian (ADR 0008);
   - błąd wartości to zdanie z wartością i aspektem („element NIP: value "11111111A1" does not match the pattern
     [1-9]((\d[1-9])|([1-9]\d))\d{7}”, „element Miesiac: value "13" is greater than the maximum 12”), bez kodu na
     początku; długie listy wartości wyliczenia są skracane, pełna wartość jest w `Value`;
   - wartość aspektu w `Facet` i w komunikacie jest w postaci kanonicznej typu: dla typów pochodnych od `integer`
     bez kropki („maxInclusive 12”, wcześniej „maxInclusive 12.0”).
5. **Tabela compat ustalona empirycznie.** `xsd/compat.go` przypisuje kodom odpowiednik w kodach libxml2;
   `xsd.LibXML2Code()` i pola `CodeInfo.LibXML2`, `LibXML2Basis`. Źródło: `tools/compare` uruchamia libxml2 2.15.4
   (przez `goxsdval`) i `goxsdgo` na tych samych danych i liczy pary kodów: pierwsze błędy obu w przypadku
   odrzuconym przez oba oraz pierwsze błędy obu na tym samym wierszu dokumentu (polecenia `w3c`, `corpus`, `probe`,
   `compat`). Polecenie `probe` ocenia małe, celowe dokumenty i schematy dla kodów, których zestaw W3C i korpus nie
   zestawiają (głównie kody czytnika XML). Numery kodów libxml2 pochodzą wyłącznie z wyników uruchomienia
   i z `messages.json`, nie ze źródeł ani nagłówków libxml2 (`docs/provenance.md`). Reguły:
   - wybieramy najczęstszy kod libxml2 dla danego kodu `goxsdgo` (42 kody mają w danych więcej niż jeden
     odpowiednik; alternatywy z liczebnościami są w `out/compare/compat.tsv`);
   - podstawa „observed n/m” (n par z wybranym kodem z m wszystkich) dla 189 kodów, „assigned …” z powodem dla 34
     kodów, których libxml2 nie zgłasza albo których dane nie zestawiły (odpowiednik reguły pokrewnej albo według
     znaczenia);
   - limity i konstrukcje nieobsługiwane (`KindLimit`, 11 kodów) nie mają odpowiednika;
   - odpowiednik błędu dokumentu musi być kodem z `messages.json` (lista w `testdata/libxml2-consumer-codes.tsv`),
     z jawnymi wyjątkami: 8 kodów czytnika XML ma zaobserwowany odpowiednik, którego odbiorca nie tłumaczy (28, 29,
     38, 80, 81, 88, 89, 96); propozycja dla odbiorcy: dopisać je do `messages.json`;
   - odpowiednik błędu schematu jest kodem XML_SCHEMAP_* (albo XML_SCHEMAV_* przy wartościach), który odbiorca
     tłumaczy komunikatem ogólnym dla schematu.
   Test `TestCompatTable` (K8) pilnuje tych reguł. Pokrycie: 223 z 223 kodów poza limitami ma odpowiednik; 52 ze 117
   kodów odbiorcy są odpowiednikiem któregoś kodu dokumentu `goxsdgo`.
6. **Rozbieżności werdyktu z libxml2 (K3).** `tools/compare -docs w3c` generuje `docs/differences-libxml2.md`: każdy
   przypadek W3C z innym werdyktem obu walidatorów, z kategorią, statusem w zestawie i tym, kto ma rację według
   oczekiwanego werdyktu. Kategorie wyznacza reguła zgłoszona przez stronę odrzucającą (kod `goxsdgo` albo grupa
   kodów libxml2). Stan 2026-11-25: 528 rozbieżności (452 testy schematów, 76 testów dokumentów), wszystkie
   skategoryzowane, w 519 rację ma `goxsdgo`, w 9 libxml2 (znane odstępstwa opisane w `differences.md`). Tabela
   kategorii trafia do `differences.md`.
7. **Trzy przypadki „do sprawdzenia w etapie 6”** — bez zmiany kodu:
   - `anyURI_b006` (stable, oczekiwany „błędny”): część 2, 3.2.17 każe przed sprawdzeniem z RFC 2396 zamienić znaki
     według XLink §5.4; ukośnik wsteczny należy do znaków „excluded” z RFC 2396 §2.4.3 i staje się `%5C`, a każda
     wartość wyliczenia jest po tej zamianie poprawnym odwołaniem URI (także „//” i „//a%5C%5Cb”, gdzie autorytet to
     reg_name). Oczekiwanie zestawu opiera się na orzeczeniu grupy roboczej (bug 4048), nie na tekście; zostaje różnicą.
   - `anyURI_a004.i` (queried, bug 4126, oczekiwany „błędny”): adnotacja tej grupy mówi, że nierozwiązywalne
     `schemaLocation` w `include`, `import` i `redefine` nie czynią błędnym ani schematu, ani dokumentu (4.2.1, 4.2.2
     punkt 1, 4.2.3); każda wartość dokumentu należy do wyliczenia. Werdykt „poprawny” zgadza się z adnotacją; zostaje
     różnicą.
   - `stZ073b.i` (disputed-spec, bug 4065): `xsi:type="xs:integer"` przy typie, który jest restrykcją unii
     `string integer` z wyliczeniem „a”. Type Derivation OK (Simple), 3.14.6 punkt 2.2.4: typ unii z członkiem
     `integer` dopuszcza `integer` jako `xsi:type`; dosłowny tekst daje „poprawny”, mimo że omija wyliczenie. Spór
     dotyczy samej rekomendacji; zostaje różnicą.
   - `attP031.i` (accepted, oczekiwany „poprawny”): według 3.2.2 deklaracja atrybutu z `use="prohibited"` nie daje
     żadnego komponentu, więc typ nie ma użycia atrybutu `att` ani symbolu wieloznacznego, a dokument ma `x:att`;
     to cvc-complex-type.3.2.1. Adnotacja zestawu zakłada dokument bez atrybutu, a plik go zawiera. Zostaje różnicą.
8. **Pytanie 6: wartości typowane.** `Config.OnValue func(TypedValue)` dostaje każdą poprawną wartość typu prostego
   (treść elementu, wartość atrybutu) ze ścieżką, nazwą, typem („{przestrzeń}nazwa” albo „” dla anonimowego),
   najbliższym typem wbudowanym i postacią kanoniczną, synchronicznie i w kolejności dokumentu. Bez funkcji ścieżka
   wykonania ma tylko sprawdzenie `nil`. Pomiar `BenchmarkValidateJPK` (syntetyczny JPK 20 MB, 3 przebiegi):
   przed zmianą 41,2–41,3 MB/s, po zmianie bez funkcji 40,9–41,2 MB/s (różnica w granicach szumu, alokacje bez
   zmian: 2,65 mln); z funkcją (`BenchmarkValidateJPKOnValue`) 33,2–33,5 MB/s i 8,7 mln alokacji, bo każda wartość
   ma ścieżkę i postać kanoniczną liczoną na żądanie. Dlatego punkt rozszerzenia jest w API, a nie w ścieżce
   domyślnej.

## Konsekwencje

+ odbiorca może przełożyć każdy kod `goxsdgo` na kod libxml2 i użyć istniejących tłumaczeń; katalog i tabela są
  sprawdzane automatycznie, więc nowy kod bez wpisu nie przejdzie testów;
+ komunikaty są czytelne bez tłumaczenia, a kody się nie zmieniają;
+ każda różnica werdyktu z libxml2 ma kategorię i uzasadnienie w jednym, generowanym pliku;
− przypisania „assigned” (34) są przybliżeniem; dla kodów dokumentu, których libxml2 nie zgłasza (`cvc-id.1`,
  `cvc-complex-type.5.1`, `cvc-complex-type.5.2`, `cvc-entity`, `cvc-notation`, `xml-encoding-mismatch`), komunikat
  odbiorcy dobrany po kodzie libxml2 może być mniej precyzyjny niż kod `goxsdgo`;
− przy 42 kodach libxml2 zgłaszał na tych samych danych różne kody; tabela wybiera najczęstszy, więc tłumaczenie
  odbiorcy może w pojedynczych przypadkach mówić co innego niż libxml2;
− tabela compat jest ustalona dla libxml2 2.15.4; przy zmianie wersji libxml2 po stronie odbiorcy trzeba ponowić
  `tools/compare` (`w3c`, `corpus`, `probe`, `compat`) i porównać.
