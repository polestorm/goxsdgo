# 0002 Przegląd kodu konkurencji po zbudowaniu własnej implementacji

Status: przyjęta · 2026-11-25

## Kontekst

`provenance.md` zakazuje czytania kodu innych walidatorów XSD. Zakaz nie wynika z licencji: libxml2 jest na MIT, Xerces
na Apache-2.0, nowe walidatory w Go (`knroy/go-xml`, `jacoelho/xsd`, `cB-Sigma/xsd`) na MIT albo Apache-2.0. Ma trzy
powody: (1) kod jest w całości nasz i łatwo to wykazać, (2) pakiet dowodów wiąże kod z regułami rekomendacji, nie
z cudzą implementacją, (3) wzorcem jest rekomendacja, więc nie przejmujemy cudzych odstępstw jako oczywistych.

Właściciel chce budować od zera, ale gdy program już działa, uczyć się od konkurencji, co można zrobić lepiej
(wydajność, komunikaty, przypadki brzegowe).

## Decyzja

1. **Faza budowy (etapy 0–6): bez zmian.** Obowiązuje pełny zakaz z `provenance.md`. Dozwolone jest porównanie
   czarnej skrzynki i dokumentacja użytkowa.
2. **Faza przeglądu** zaczyna się, gdy K1 i K2 są spełnione i zapisane w `docs/testing.md`. Wtedy poprawność rdzenia
   jest już wykazana niezależnie, a jej ślad prowadzi do rekomendacji.
3. **Kolejność w fazie przeglądu:** najpierw czarna skrzynka (pomiary, komunikaty, werdykty), a kod czytamy dopiero
   wtedy, gdy pomiar pokazuje konkretną przewagę konkurenta i nie wiemy, skąd się bierze.
4. **Tylko licencje liberalne:** MIT, BSD, Apache-2.0, ISC i podobne. Bez GPL, LGPL, AGPL, MPL, licencji niejasnych
   (np. `NOASSERTION`) i kodu komercyjnego.
5. **Rozdzielenie ról.** Kod konkurenta czyta osobna sesja albo osobny agent („czytelnik”). Oddaje notatkę prozą: na
   czym polega pomysł, której reguły rekomendacji dotyczy, jaki problem rozwiązuje. **Bez kodu, bez pseudokodu
   wiernego oryginałowi, bez nazw wewnętrznych.** Implementację pisze sesja, która kodu nie widziała, z notatki
   i z rekomendacji.
6. **Dziennik przeglądu** `docs/review/<projekt>-<data>.md`: repozytorium, commit, licencja, przeczytane pliki, notatka
   czytelnika, decyzja (przyjęte, odrzucone, z powodem) i commit, w którym zmiana weszła. Dziennik jest częścią pakietu
   dowodów (K9).
7. **Zmiana zachowania wymaga rekomendacji.** „Konkurent robi X” nie uzasadnia zmiany werdyktu ani kodu błędu. Taka
   zmiana potrzebuje reguły z rekomendacji i testu (przypadek W3C albo plik korpusu). Bez nich różnica trafia do
   `differences.md`. Pomysły na wydajność i ergonomię komunikatów nie zmieniają werdyktu i wymagają tylko pomiaru.
8. **Bez kopiowania.** Fragment przeniesiony dosłownie jest błędem procesu: usuwamy go i piszemy od nowa z notatki.
   Wyjątkiem są dane (np. tabele znaków), które bierzemy wyłącznie ze źródła normatywnego (Unicode), nie od konkurenta.

## Konsekwencje

+ korzystamy z doświadczeń konkurencji tam, gdzie daje to najwięcej: wydajność (K7) i jakość komunikatów;
+ rdzeń powstał i został zmierzony bez cudzego kodu, a dziennik pokazuje, skąd przyszedł każdy późniejszy pomysł;
− rozdzielenie ról kosztuje czas (dwie sesje na jedną zmianę);
− po fazie przeglądu nie możemy już twierdzić, że nikt w projekcie nie widział cudzego kodu. Twierdzimy, że widzieliśmy,
  w którym miejscu i co z tego wynikło, i że każda zmiana ma uzasadnienie w rekomendacji albo w pomiarze.
