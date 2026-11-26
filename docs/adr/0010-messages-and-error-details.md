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
2. **Komunikaty dla ludzi** (tytuł i podpowiedź według kodu i wariantu, w wielu językach) powstały w etapie 6b
   jako pakiet `messages` w tym repozytorium. 2026-11-26 właściciel wydzielił go razem z historią do osobnego,
   prywatnego modułu; z historii goxsdgo został usunięty. Biblioteka `xsd` zostaje przy komunikatach po angielsku,
   stałych kodach i polach błędu, z których wywołujący buduje własne teksty.
3. **Tabela `compat` jest przejściowa.** Służy odbiorcy, który chce najpierw podmienić walidator, a dopiero potem
   teksty. Docelowo odbiorca tłumaczy według kodu goxsdgo i pola `Reason` (`docs/migrating-from-libxml2.md`). Przy przejściu
   tylko przez `compat` rozszerzenia xml.pro (`Diagnose()`) przestaną działać, bo rozpoznają zdania libxml2.

## Konsekwencje

+ odbiorca może usunąć wyrażenia regularne na tekstach libxml2, heurystyki położenia i przypisanie tekstów do kodów
  libxml2;
+ nowe pola nie zmieniają szybkości walidacji (BenchmarkValidateJPK 41 MB/s przed i po, te same alokacje);
− element nadrzędny przy błędach innych niż `missing` trzeba wziąć ze ścieżki (nazwa lokalna rodzica, bez przestrzeni
  nazw).
