# Wymagania: typy złożone i modele zawartości

Decyzje: `docs/adr/0007-content-models.md`. Rekomendacja: XML Schema Part 1: Structures Second Edition (2004-10-28),
rozdziały 3.3, 3.8, 3.9, 3.10 i dodatek H. Pakiet: `internal/schema` (`model.go`, `restrict.go`, `subst.go`,
`cmchecks.go`).

| Id | Wymaganie |
|---|---|
| CM-001 | Model zawartości typu złożonego (treść pusta, wyłącznie elementowa albo mieszana) jest kompilowany do automatu pozycji z licznikami: liczności inne niż 0..1, 1..1 i 1..unbounded są licznikami, nie są rozwijane, więc `maxOccurs="100000"` nie zwiększa liczby stanów (3.8, 3.9, ADR 0007). |
| CM-002 | Dopasowanie ciągu dzieci odpowiada Element Sequence Locally Valid (Particle) i Element Sequence Valid (3.9.4, 3.8.4) dla `sequence` i `choice` z dowolnymi licznościami: ciąg jest przyjmowany wtedy i tylko wtedy, gdy należy do języka modelu; dziecko spoza modelu zatrzymuje dopasowanie bez zmiany stanu. |
| CM-003 | Grupa `all` przyjmuje każdy element najwyżej raz, w dowolnej kolejności; ciąg jest kompletny, gdy zawiera wszystkie elementy z `minOccurs="1"`, a grupa z `minOccurs="0"` przyjmuje także brak dzieci (3.8.4). |
| CM-004 | Symbol wieloznaczny w modelu przyjmuje element, którego przestrzeń nazw jest dopuszczona według Wildcard allows Namespace Name (3.10.4); wynik dopasowania wskazuje symbol (z jego {process contents}) i cząstkę. |
| CM-005 | Cząstka z globalną deklaracją elementu przyjmuje członków jej ·substitution group· (3.3.6, Substitution Group): bez deklaracji abstrakcyjnych, tylko członków dopuszczonych przez Substitution Group OK (Transitive) przy {disallowed substitutions} głowy; wynik dopasowania wskazuje deklarację członka. |
| CM-006 | Dopasowanie podaje, jakie elementy i symbole wieloznaczne mogą przyjść jako następne, w powtarzalnej kolejności, do komunikatu cvc-complex-type.2.4 w etapie 5. |
| CM-007 | Unique Particle Attribution (`cos-nonambig`, 3.8.6, dodatek H): model, w którym jedno dziecko mogą przyjąć dwie różne cząstki (także przez grupę podstawienia albo symbol wieloznaczny), jest błędem schematu. Liczności są uwzględnione jak przy rozwinięciu z dodatku H, więc np. `sequence(a{2,2}, a)` jest jednoznaczne, a `sequence(a{1,2}, a?)` nie. |
| CM-008 | Element Declarations Consistent (`cos-element-consistent`, 3.8.6): deklaracje o tej samej nazwie w jednym modelu zawartości, wprost, pośrednio albo ·implicitly· (przez grupę podstawienia), są tą samą deklaracją albo mają ten sam nazwany typ. |
| CM-009 | Particle Valid (Restriction) (3.9.6) stosuje punkt 2.1 (głowa grupy podstawienia jak `choice` swoich członków) i punkt 2.2 (pominięcie zbędnych `sequence`, `choice` i `all`) przed tabelą przypadków; ta sama cząstka jest zawsze poprawną restrykcją. |
| CM-010 | Przypadki z elementem albo symbolem wieloznacznym: NameAndTypeOK (nazwa, Occurrence Range OK, `nillable`, wartość stała, ograniczenia tożsamości, {disallowed substitutions}, typ wyprowadzony przez restrykcję), NSCompat, NSSubset (z wyjątkiem dla symbolu treści anyType); symbol wieloznaczny nie może zawęzić elementu ani grupy. |
| CM-011 | Przypadki z grupami: RecurseAsIfGroup, Recurse (odwzorowanie zachowujące kolejność, pominięte cząstki bazy ·emptiable·), RecurseLax, RecurseUnordered, MapAndSum, NSRecurseCheckCardinality (zakres łączny grupy, Effective Total Range); pozostałe pary są zabronione. Odwzorowania są szukane wyczerpująco (programowanie dynamiczne), nie zachłannie. |
| CM-012 | Restrykcję cząstek sprawdzają Derivation Valid (Restriction, Complex) punkt 5.4.2 (`derivation-ok-restriction.5.4.2`) i redefinicja grupy bez odwołania do siebie (`src-redefine.6.2.2`). |
| CM-013 | Kompilacja i dopasowanie mają czas liniowy w przypadkach patologicznych: liczności w setkach tysięcy, zagnieżdżone liczniki, długie ciągi dzieci, wiele symboli wieloznacznych, duże grupy podstawienia. |
| CM-014 | Model z niejednoznacznością liczników (kilka konfiguracji liczników po tym samym prefiksie) jest dopasowywany zbiorem konfiguracji ograniczonym przez `MaxConfigurations`; przekroczenie jest sygnalizowane (`Matcher.Overflow`), nigdy nie daje fałszywego werdyktu ani czasu wykładniczego. |
| CM-015 | Particle Emptiable i Effective Total Range (3.8.6, 3.9.6) liczą zakres łączny grup `all`, `sequence` i `choice`, także z licznościami bliskimi granicy int64 bez przepełnienia. |
