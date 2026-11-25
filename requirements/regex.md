# Wymagania: wyrażenia regularne XSD

Źródło: XML Schema Part 2: Datatypes Second Edition, dodatek F. Decyzje: `docs/adr/0005-datatypes-and-regex.md`.
Kod: `internal/regex`.

| Id | Wymaganie |
|---|---|
| RE-001 | Wyrażenie jest kotwiczone na całym napisie (dodatek F, uwaga na początku); puste wyrażenie i pusta gałąź oznaczają pusty napis. |
| RE-002 | Gałęzie, części i liczniki (produkcje [1]–[8]): `?`, `*`, `+`, `{n}`, `{n,}`, `{n,m}` z n <= m; `{0,0}` to tylko pusty napis; licznik bez atomu albo po liczniku to błąd. |
| RE-003 | Znaki zwykłe i metaznaki (produkcja [10] i definicja metaznaku): metaznaki `.\?*+{}()[]\|` trzeba poprzedzić `\`; `^` i `$` są znakami zwykłymi; znak spoza znaków XML to błąd. |
| RE-004 | Ucieczki jednoznakowe (produkcja [24]): `\n \r \t \\ \| \. \- \^ \? \* \+ \{ \} \( \) \[ \]`; inne ucieczki to błąd. |
| RE-005 | Ucieczki wieloznakowe (produkcje [37], [37a]): `.` (bez #xA i #xD), `\s \S`, `\i \I \c \C` według produkcji Letter i NameChar z XML 1.0 Second Edition (dodatek B), `\d \D` (kategoria Nd), `\w \W` (wszystko poza P, Z i C). |
| RE-006 | Ucieczki kategorii (produkcje [25]–[35]): `\p{X}` i `\P{X}` dla kategorii ogólnych Unicode z tablicy F.1.1; C obejmuje Cn i wyklucza Cs; nieznana kategoria to błąd. |
| RE-007 | Ucieczki bloków (produkcja [36]): `\p{IsX}` i `\P{IsX}` dla bloków z tablicy dodatku F (bloki BMP Unicode 3.1) i bloków Unicode 3.1 spoza BMP; PrivateUse tylko E000–F8FF; nazwy bloków surogatów przyjmujemy jako zbiory puste; inna nieznana nazwa to błąd. |
| RE-008 | Wyrażenia klas znaków (produkcje [12]–[22]): grupy pozytywne i negatywne (dopełnienie względem znaków XML), przedziały s-e (e >= s), `-` jako znak tylko na początku albo na końcu grupy (także tuż przed odejmowaniem), odejmowanie klas `[G-[C]]` z zagnieżdżaniem. |
| RE-009 | Błędne wyrażenie dostaje kod regex-syntax i pozycję w tekście wyrażenia. |
| RE-010 | Dopasowanie w czasie liniowym względem długości tekstu, bez nawrotów, także dla wyrażeń patologicznych (np. `(a*)*b`, `(a\|aa)*c`). |
| RE-011 | Liczniki większe niż 1000 (granica RE2) działają przez rozkład na odcinki albo rozwinięcie, z zachowaniem języka; wyrażenie zbyt duże po rozwinięciu (ponad 1 MiB tekstu RE2) dostaje kod regex-too-large. |
