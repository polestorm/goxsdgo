# Wymagania: powtarzalność, czas liniowy, wydajność

Decyzje: `docs/adr/0011-hardening.md`. Powtarzalność walidacji dokumentu opisuje też VAL-019, czas liniowy modeli
zawartości CM-013 i wzorców RE-010, pamięć przy strumieniu VAL-018 i XML-017; tu są wymagania dla całej biblioteki.

| Id | Wymaganie |
|---|---|
| DET-001 | Czas walidacji rośnie liniowo z rozmiarem wejścia także dla wejść patologicznych: wzorce z zagnieżdżonymi powtórzeniami na długich wartościach, liczności w setkach tysięcy, zagnieżdżone liczniki jednoznaczne, głębokie zagnieżdżenie do limitu, długie atrybuty i teksty, tysiące atrybutów (także z prefiksami), tysiące kluczy, ID i IDREFS, duże wyliczenia, długie listy, wiele symboli wieloznacznych, rozwinięcia encji wewnętrznych. Przy dwukrotnym zwiększeniu wejścia czas rośnie nie więcej niż 3,2 raza (`TestPathological`). |
| DET-002 | Wejście ponad limit kończy walidację błędem `ErrLimit` w ograniczonym czasie, bez paniki i bez zużycia pamięci proporcjonalnego do wejścia: zagnieżdżenie ponad `MaxDepth`, encje rozwijane wykładniczo („billion laughs”), atrybuty ponad `MaxAttributes`, tekst ponad `MaxTextSize`, deklaracje przestrzeni nazw w zasięgu ponad `MaxNamespaces`, wpisy tożsamości ponad `MaxIdentityEntries`, model niejednoznaczny co do liczników ponad `MaxConfigurations`. |
| DET-003 | Fuzzery biblioteki (`FuzzReader`, `FuzzRegexTranslate`, `FuzzLexical`, `FuzzDecimal`, `FuzzContentModel`, `FuzzCompile`, `FuzzValidate`) działają łącznie co najmniej 24 h na kandydacie do wydania bez paniki i bez naruszenia niezmienników (kryterium K5); kampanię prowadzi `tools/fuzz`, a każdy przebieg ma wiersz w `docs/fuzzing.md`. |
| DET-004 | Zmiana wydajnościowa nie zmienia żadnego błędu: kod, pola, pozycja i kolejność wszystkich błędów zestawu W3C i korpusu są takie same przed zmianą i po niej (zrzut `TestDumpErrors`, porównanie poleceniem diff). |
| DET-005 | Wartości decimal są dokładne niezależnie od reprezentacji: szybka ścieżka dla wartości mieszczących się w int64 i `math/big` dla większych dają ten sam porządek, znak, postać kanoniczną i liczbę cyfr co niezależny wzorzec na liczbach wymiernych, a każda wartość ma jedną reprezentację (`FuzzDecimal`). |
| DET-006 | Walidacja strumieniowa jest nie wolniejsza niż 3× libxml2 na dużych plikach JPK (kryterium K7): na syntetycznym JPK_V7M(3) (`BenchmarkValidateJPK`, `tools/compare timefile`) i na korpusie prywatnym (`BenchmarkCorpus`, `tools/compare private`). |
