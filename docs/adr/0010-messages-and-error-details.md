# 0010 Szczegóły błędu i komunikaty w wielu językach

Status: przyjęta · 2026-11-25 (etap 6b)

## Kontekst

Odbiorca (xml.pro) pokazuje dziś polskie komunikaty z kodów libxml2 (`internal/i18n/errors.yaml`, 117 kodów) i je
rozszerza: `Diagnose()` wyciąga wyrażeniami regularnymi z angielskiego tekstu libxml2 nazwy elementów i rozpoznaje
brak wymaganego elementu, element niedozwolony, złą przestrzeń nazw i pustą albo złą wartość z nazwą typu, a
`snippet_format.go` zgaduje miejsce błędu, bo libxml2 nie podaje kolumny. Po etapie 6 goxsdgo ma katalog kodów,
pola `Name`, `Value`, `Facet`, `Expected`, `Path`, kolumnę i tabelę `compat`, ale sprawdzenie z 2026-11-25 na
schemacie w kształcie JPK wykazało cztery braki: oba rodzaje błędu modelu mają ten sam kod i rozróżnia je tylko
ścieżka, nie ma przestrzeni nazw węzła (jest tylko w tekście), nie ma typu ze schematu (komunikat podaje typ
wbudowany) i nie ma miejsca, w które wstawić brakujący element.

Odbiorca chce zachować swoje polskie teksty i ich rozszerzenia bez kodów libxml2 i bez parsowania tekstu, a w
przyszłości komunikaty w kolejnych językach.

## Decyzja

1. **Nowe pola `xsd.Error`**, zgodnie wstecz:
   - `Namespace`: przestrzeń nazw węzła `Name` (pusta dla nazwy bez przestrzeni);
   - `Reason` dla `cvc-complex-type.2.4`: `missing`, `unexpected` albo `wrong-namespace`, oraz `ExpectedNamespace`
     przy `wrong-namespace`. **Kod reguły zostaje jeden.** Rekomendacja ma jedną regułę (3.4.4, punkt 2.4); podkod
     byłby naszym wymysłem i zmieniłby kod dla odbiorców, którzy już go używają. Rodzaj to parametr błędu, tak jak
     `Facet` dla aspektów. `wrong-namespace` ustawiamy, gdy nazwa lokalna niedozwolonego elementu jest nazwą
     lokalną któregoś z oczekiwanych elementów, w innej przestrzeni nazw;
   - `Type`, `TypeNamespace`, `Builtin` dla błędów wartości typów prostych (`cvc-datatype-valid.*`, aspekty,
     `cvc-elt.5.1.1`): typ ze schematu (pusty `Type` dla typu anonimowego) i najbliższy typ wbudowany;
   - `InsertLine`, `InsertColumn` dla `cvc-complex-type.2.4`: początek znacznika niedozwolonego elementu albo
     początek znacznika końcowego rodzica; dla rodzica `<x/>` koniec tego znacznika, bo dziecko można wstawić dopiero
     po rozwinięciu go do `<x></x>`.
   Pozycja główna (`Line`, `Column`) zostaje końcem znacznika początkowego (ADR 0008, pytanie 8); werdykty, kody
   i pozycje główne się nie zmieniają. Katalog kodów (`xsd/codes.go`) deklaruje nowe parametry: `Reason`, `Type`,
   `Insert` (`Name` obejmuje `Namespace`).
2. **Pakiet `github.com/polestorm/goxsdgo/messages`**, opcjonalny, tylko biblioteka standardowa, zależny od `xsd`
   (nie odwrotnie). Biblioteka `xsd` zostaje przy komunikatach po angielsku i stałych kodach (CLAUDE.md).
   - Dane: osadzony `messages/catalog.json`, według kodu z `xsd.Codes()` i wariantu, dla każdego języka tytuł
     i podpowiedź z parametrami. Warianty: `cvc-complex-type.2.4` ma `missing`, `missing_one_of`, `unexpected`,
     `unexpected_no_expected`, `wrong_namespace`; `cvc-datatype-valid.1.2.1`, `cvc-length-valid`,
     `cvc-minLength-valid`, `cvc-pattern-valid`, `cvc-enumeration-valid` mają `empty` (pusta wartość, najczęstszy
     błąd w JPK); pozostałe kody wariant podstawowy `""`. Razem 234 kody, 243 warianty, 486 tekstów (EN i PL).
   - API: `Describe(lang, e, Params) Message` (tytuł, podpowiedź jako tekst i jako segmenty `{Text, Param, Em}`),
     `Variant(e)`, `Lookup`, `Languages`, `Variants`, `Render`. Nieznany język daje angielski, kod spoza katalogu
     tekst zastępczy. Pusty parametr znika razem ze spacją i pustymi nawiasami lub cudzysłowami.
   - Parametry wyliczone z błędu: `{name}`, `{namespace}`, `{container}`, `{element}`, `{expected}` (nazwy lokalne,
     symbol wieloznaczny opisany słowami języka), `{expected_namespace}`, `{value}`, `{facet}`, `{limit}`
     (wartość aspektu), `{type}` (typ ze schematu albo wbudowany), `{builtin}`, `{where}` („elemencie”/„atrybucie”),
     `{path}`, `{line}`, `{column}`, `{insert_line}`, `{insert_column}`, `{code}`, `{document}`, `{message}`.
     Parametry odbiorcy przez `Params`, np. `{type_desc}` z opisem typu z `jpktypes`.
3. **Źródła tekstów.** Angielskie teksty są napisane w goxsdgo z opisów katalogu i rekomendacji. Polskie przeniesione
   z xml.pro (`errors.yaml`: pola `pl`, `hint_pl`; `chrome.yaml`: `diagnose.*`) są przypisane od nowa do kodów
   goxsdgo według znaczenia reguły, nie przez numer libxml2, i mają `source: "xml.pro"` (61 tekstów). Zdania
   o widoku podglądu zostały w xml.pro, bo dotyczą interfejsu, nie błędu. Pozostałe 182
   polskie teksty napisano w tym samym stylu (`source: "goxsdgo"`, `review: true`, do przeglądu przez właściciela).
   **Pól `en:` z `errors.yaml` nie przenosimy**: według nagłówka tego pliku pochodzą ze źródeł libxml2
   (`docs/provenance.md`).
4. **Błędy schematu** mają wspólny tytuł i zdanie według kodu
   w podpowiedzi, z dokumentem i wierszem. Odbiorca pokazuje dziś wszystkie błędy schematu jednym komunikatem; zdanie
   według kodu jest dokładniejsze, a tytuł wspólny wystarcza, bo schematy przygotowuje odbiorca, nie użytkownik.
5. **Nowy język** to kolumna w danych: kod w `languages`, słowa w `words` i tekst każdego wariantu; test kompletności
   wskaże braki. Kod pakietu się nie zmienia.
6. **Tabela `compat` jest przejściowa.** Służy odbiorcy, który chce najpierw podmienić walidator, a dopiero potem
   teksty. Docelowo odbiorca tłumaczy według kodu goxsdgo i wariantu (`docs/consumer-migration.md`). Przy przejściu
   tylko przez `compat` rozszerzenia xml.pro (`Diagnose()`) przestaną działać, bo rozpoznają zdania libxml2.

## Konsekwencje

+ odbiorca może usunąć `Diagnose()`, heurystyki położenia i przypisanie tekstów do kodów libxml2;
+ teksty są w jednym miejscu, sprawdzane testami (kompletność, parametry, wszystkie błędy z zestawu W3C i korpusu,
  porównanie z tekstami xml.pro dla czterech przypadków);
+ bez użycia pakietu `messages` koszt jest zerowy; nowe pola nie zmieniają szybkości walidacji (BenchmarkValidateJPK
  41 MB/s przed i po, te same alokacje);
− 182 polskie teksty czekają na przegląd właściciela;
− `{container}` przy błędach innych niż `missing` pochodzi ze ścieżki (nazwa lokalna rodzica, bez przestrzeni nazw);
− zmiana kodu w `xsd/codes.go` wymaga wpisu w `catalog.json` w każdym języku (test to wymusza).
