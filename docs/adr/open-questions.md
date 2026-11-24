# Pytania otwarte

1. **Zestaw testów W3C w repozytorium.** Zestaw `w3c/xsdtests` (licencja W3C Test Suite / W3C Software and Document
   License, dopuszcza rozpowszechnianie z notą) ma kilkadziesiąt tysięcy plików. Rekomendacja: pobierać skryptem
   `tools/fetch-w3c` do `testdata/w3c/xsdtests/` (ignorowane), przypiąć commit i sumę kontrolną archiwum
   w `testdata/w3c/README.md`; do repozytorium trafia tylko wyciąg wyników oczekiwanych i lista wyłączeń. Testy bez
   pobranego zestawu są pomijane z komunikatem, a CI pobiera go z pamięci podręcznej. Alternatywa: całość w repozytorium
   (testy bez sieci, ale duże repozytorium).
2. **Schematy i przykłady MF w repozytorium.** Schematy JPK, KSeF i e-Sprawozdań są publikowane przez organy państwa
   (urzędowe materiały, art. 4 ustawy o prawie autorskim). Rekomendacja: trzymać w `testdata/schemas/` niezmienione,
   ze źródłem, datą pobrania i SHA-256 w `testdata/README.md`; przykładowe dokumenty tylko opublikowane przez MF
   i syntetyczne.
3. **Własny czytnik XML czy `encoding/xml`.** `encoding/xml` nie zgłasza części błędów dobrej formy, nie daje kolumny,
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
