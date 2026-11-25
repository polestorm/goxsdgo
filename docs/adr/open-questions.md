# Pytania otwarte

1. ~~**Zestaw testów W3C w repozytorium.**~~ Rozstrzygnięte w ADR 0003 (2026-11-25): pobierany narzędziem
   `tools/fetch`, przypięty commit i suma drzewa w `testdata/sources.json`, poza repozytorium.
2. ~~**Schematy i przykłady MF w repozytorium.**~~ Rozstrzygnięte w ADR 0003 (2026-11-25): pobierane tak samo, poza
   repozytorium, z sumą każdego pliku w manifeście.
3. ~~**Własny czytnik XML czy `encoding/xml`.**~~ Rozstrzygnięte w ADR 0004 (2026-11-25): własny czytnik
   (`internal/xmlreader`); `encoding/xml` daje na zestawie XML Conformance 1146 / 1727, własny 1725 / 1727.
   Pierwotny opis: `encoding/xml` nie zgłasza części błędów dobrej formy, nie daje kolumny,
   inaczej traktuje przestrzenie nazw i DOCTYPE. Rekomendacja: własny czytnik strumieniowy XML 1.0 + przestrzenie nazw
   (ok. 1,5–2 tys. linii), z testami z zestawu W3C XML Conformance (pobierany jak w pytaniu 1). Do rozstrzygnięcia
   w etapie 1.
4. **Kody błędów.** Własna numeracja z tabelą `compat` do kodów libxml2 (rekomendacja), czy wprost kody libxml2?
   Własne kody są stabilne niezależnie od libxml2; tabela pozwala odbiorcy użyć istniejących tłumaczeń.
5. **API schematu.** Czy wystawiamy skompilowany model schematu (typy, deklaracje) do odczytu, np. dla edytorów
   z podpowiedziami i dla generatorów formularzy, czy tylko `Compile` i `Validate`? Rekomendacja na start: tylko
   walidacja, model po pierwszym konsumencie.
6. **Wartości typowane w wyniku.** Czy walidator ma oddawać wartości po normalizacji (np. `xs:decimal` jako tekst
   kanoniczny, `xs:date`), żeby odbiorca nie parsował drugi raz? Rekomendacja: punkt rozszerzenia (funkcja zwrotna dla
   elementu z typem prostym) w etapie 6, nie w API rdzenia.
7. **Wersja Unicode w zestawie W3C.** Propozycja w ADR 0005 (2026-11-25): kategorie znaków z biblioteki standardowej
   Go (Unicode 15, żeton `Unicode_6.0.0`), bloki według Unicode 3.1 jak w dodatku F; 16 przypadków MS-Regex
   zakładających Unicode 3.0 do `differences.md`. Czeka na decyzję właściciela. Pierwotny opis: 16 oczekiwanych wyników w zestawie zależy od wersji Unicode (żetony
   `Unicode_4.0.0` i `Unicode_6.0.0`), a jeden przypadek dotyczy tylko Unicode 4.0.0. Dotyczy to klas znaków
   w wyrażeniach regularnych i znaków nazw. Biblioteka standardowa Go ma tablice Unicode 15. Rekomendacja: przyjąć
   `Unicode_6.0.0` jako bliższy temu, co da `unicode` z Go, i opisać różnice w `differences.md`. Do rozstrzygnięcia
   w etapie 2, razem z tłumaczem wyrażeń regularnych. Do tego czasu przypadki są wyłączone z powodem.
