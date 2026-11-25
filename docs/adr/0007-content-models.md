# 0007 Modele zawartości: automat z licznikami, UPA, EDC, restrykcja cząstek

Status: przyjęta (etap 4) · 2026-11-25

## Kontekst

Etap 3 zbudował komponenty schematu, ale zostawił kontrole, które wymagają modeli zawartości: Unique Particle
Attribution (`cos-nonambig`), Element Declarations Consistent (`cos-element-consistent`), Particle Valid
(Restriction) (`cos-particle-restrict`) w Derivation Valid (Restriction, Complex) 5.4.2 i w redefinicji grupy
(`src-redefine.6.2.2`). Etap 5 potrzebuje dopasowania dzieci elementu do modelu w czasie liniowym. ADR 0001 wymaga,
by `maxOccurs="100000"` nie rozwijało się w 100 000 stanów.

## Decyzja

1. **Automat pozycji z licznikami** (`internal/schema/model.go`). Pozycje to cząstki-liście (deklaracje elementów,
   symbole wieloznaczne), jak w konstrukcji Glushkova (Brüggemann-Klein i Wood, 1998). Cząstka z licznością inną niż
   0..1, 1..1, 0..unbounded, 1..unbounded ma licznik (Sperberg-McQueen 2004; Gelade, Gyssens, Martens 2012).
   Przejście z pozycji p do q zachodzi w węźle J: w sekwencji (q w kolejnych gałęziach po p, przez gałęzie
   dopuszczające pustkę) albo w powtórzeniu J (q zaczyna kolejną iterację). Warunki przejścia: węzły opuszczane pod J
   mają licznik co najmniej `minOccurs` (chyba że ich ciało dopuszcza pustkę), a przy powtórzeniu licznik J jest
   mniejszy od `maxOccurs`. Licznik dla `maxOccurs="unbounded"` nasyca się na `minOccurs`.
2. **Krawędzie do zbiorów zamiast par przejść.** Z pozycji prowadzi kilka krawędzi (do pierwszych pozycji węzła albo
   kolejnych dzieci sekwencji), a każdy węzeł ma indeks swoich pierwszych pozycji według nazwy elementu
   i przestrzeni nazw symbolu wieloznacznego. Dla `choice*` z n cząstkami nie powstaje n² przejść; krok dopasowania
   kosztuje tyle, ile krawędzi ma pozycja (stała schematu), więc dopasowanie jest liniowe w liczbie dzieci.
3. **Grupa `all`** ma osobny, prosty dopasowywacz (zbiór użytych cząstek); `cos-all-limited` wyklucza ją w innych
   miejscach.
4. **Grupy podstawienia** (`subst.go`): ·substitution group· głowy według 3.3.6 — bez deklaracji abstrakcyjnych, tylko
   członkowie dopuszczeni przez Substitution Group OK (Transitive) przy {disallowed substitutions} głowy, z metodami
   wyprowadzenia i {prohibited substitutions} typów pośrednich (punkt 2.3). Kolejność: głowa, potem członkowie
   w kolejności budowy deklaracji (powtarzalnie).
5. **UPA z licznikami, jak w dodatku H.** Liczności są rozwinięte, nie pominięte: dwa przejścia z tego samego źródła do
   różnych pozycji o wspólnej etykiecie (nazwa, członek grupy podstawienia, przestrzeń symbolu wieloznacznego) są
   w konflikcie, gdy ich warunki (przedziały liczników źródła) mają część wspólną. Stąd `sequence(a{2,2}, a)` jest
   poprawne, a `sequence(a{1,2}, a?)` nie. Wartości liczników źródła traktujemy jako dowolne z ich zakresów (każda
   jest osiągalna w każdej iteracji przodka).
6. **Niejednoznaczność liczników.** Gdy do tej samej pozycji prowadzą dwie krawędzie o różnym działaniu na liczniki
   i spełnialnych razem warunkach (np. `sequence{1,2}(a{2,3})`: po trzech `a` licznik to (1, 3) albo (2, 1)), model
   jest jednoznaczny według UPA (ta sama cząstka), ale dopasowanie trzyma zbiór konfiguracji. Taki model sprawdzamy
   ostrożniej: konflikt jest wtedy, gdy każdy z dwóch warunków osobno jest spełnialny. Zbiór konfiguracji ograniczamy
   do `MaxConfigurations` (1024); przekroczenie ustawia `Matcher.Overflow` i etap 5 zgłosi je jako przekroczenie
   limitu, nie jako błąd dokumentu. W modelach bez niejednoznaczności liczników konfiguracja jest zawsze jedna.
7. **EDC** obejmuje deklaracje wprost, pośrednio i ·implicitly· (członków grup podstawienia). Ta sama deklaracja
   użyta dwa razy (np. dwa `ref` do jednego elementu globalnego) jest zgodna, także z typem anonimowym.
8. **Restrykcja cząstek** (`restrict.go`) według tabeli 3.9.6, po normalizacji punktów 2.1 (głowa z niepustą grupą
   podstawienia jak `choice` członków z licznością 1..1) i 2.2 (pominięcie zbędnych grup; sekwencja w sekwencji
   i choice w choice z licznością 1..1 wchodzą do rodzica, także choice z grupy podstawienia). Odwzorowania
   zachowujące kolejność (Recurse, RecurseLax) szukamy programowaniem dynamicznym, RecurseUnordered — skojarzeniem
   w grafie dwudzielnym.
9. **Dwa odczytania zgodne z zestawem W3C** (opisane w kodzie i w `differences.md`):
   - NSRecurseCheckCardinality, punkt 1: każdą cząstkę grupy porównujemy z symbolem wieloznacznym bez ograniczenia
     liczności (przestrzeń nazw i {process contents}), a liczność całej grupy sprawdza punkt 2. Dosłownie odrzucałoby
     to np. `all(e1, e2, e3)` jako restrykcję `any{3,3}`; zestaw (particlesHa070, Ha080, Ha084, Q013, R013) oczekuje
     „poprawny”, a nazwa reguły („CheckCardinality”) wskazuje to odczytanie.
   - RecurseLax dla `choice` z grupy podstawienia: kolejność członków grupy podstawienia nie jest w rekomendacji
     określona, więc dla takiego choice odwzorowanie nie musi zachowywać kolejności (różnowartościowe skojarzenie).
     Zestaw (elemZ027_e, elemZ027_f) tego wymaga.
10. **RecurseAsIfGroup dosłownie**: element staje się jedyną cząstką grupy rodzaju B z licznością 1..1 i zachowuje
    własną liczność. particlesHa161 i particlesK006 tego wymagają; particlesZ001 oczekuje odwrotnie (liczność elementu
    przeniesiona na grupę). Wygrywa tekst rekomendacji, a Z001 trafia do `differences.md`.
11. **API dla etapu 5**: `ComplexType.Model` (`*ContentModel`, tylko do odczytu, bezpieczny dla wielu goroutines),
    `Model.NewMatcher()`, `Matcher.Step(QName) (Match, bool)` — `Match` wskazuje cząstkę i deklarację (także członka
    grupy podstawienia) albo symbol wieloznaczny; `Matcher.Accepting()`; `Matcher.Expected()` — następne dopuszczalne
    elementy i symbole w powtarzalnej kolejności (do komunikatu `cvc-complex-type.2.4`); `Matcher.Overflow`.

## Czego nie robimy (jeszcze)

- `cos-ct-extends.1.5` (wyprowadzenie „w dwóch krokach”): brak przypadku W3C, który by od niej zależał; przy
  rozszerzeniu cząstka bazy jest zawsze pierwszą cząstką sekwencji, więc Particle Valid (Extension) zachodzi
  z konstrukcji. Do zrobienia, gdy pojawi się przypadek.
- Dopasowanie modeli z niejednoznacznością liczników ponad `MaxConfigurations` (np. `(a{1,1000}){1,1000}` po ponad
  tysiącu dzieci): zamiast czasu kwadratowego — sygnał przekroczenia limitu.

## Konsekwencje

+ testy schematów W3C: 14 370 / 14 384 (przed etapem 14 048); ten sam werdykt co libxml2 w 13 855 z 14 307
  porównywalnych, a z 452 rozbieżności rację ma goxsdgo w 447;
+ dopasowanie i kompilacja liniowe w przypadkach patologicznych (`TestPathological`);
− `particlesZ001` przestaje przechodzić (wygrywa rekomendacja, `differences.md`);
− UPA dla modeli z niejednoznacznością liczników jest ostrożniejszy niż dodatek H (może odrzucić model, który po
  pełnym rozwinięciu byłby jednoznaczny); w zestawie W3C nie ma takiego przypadku.
