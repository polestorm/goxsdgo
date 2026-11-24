# Dane testowe

Dane testowe nie są kodem i nie są objęte licencją `goxsdgo` (`../NOTICE`). Każdy zestaw ma tu źródło, licencję,
datę pobrania i sumę kontrolną.

| Katalog | Zawartość | Źródło | Licencja | W repozytorium |
|---|---|---|---|---|
| `w3c/` | zestaw testów W3C XML Schema 1.0 (`xsdtests`), przypięty commit; wyciąg werdyktów i lista wyłączeń | `github.com/w3c/xsdtests` | W3C Test Suite License | tylko wyciąg i lista; zestaw pobierany (pytanie 1) |
| `w3c/xmlconf/` | zestaw W3C XML Conformance (czytnik XML, etap 1) | W3C | W3C | pobierany jak wyżej |
| `schemas/` | schematy MF: JPK (V7M, V7K, FA, KR, KR_PD, WB, MAG, EWP, PKPIR, VAT), KSeF FA(2) i FA(3), e-Sprawozdania z plikami pomocniczymi (`auxiliary/`, ok. 150 plików) | publikacje MF i KAS; pierwsza kopia z `../xml.pro/internal/importing/schemas/` (169 plików, 18 MB) | materiały urzędowe (pytanie 2) | tak, niezmienione, z SHA-256 w tabeli poniżej |
| `instances/` | dokumenty przykładowe opublikowane przez MF (`../xml.pro/tests/mf_official/*.xml`) i dokumenty syntetyczne; `expected.json`: werdykt, kody, miejsca błędów | MF; własne | urzędowe; nasze | tak |
| `corpus/` | małe schematy i dokumenty pod wymagania | własne | Apache 2.0 | tak |

## Dane prywatne: nigdy w repozytorium

Pliki od klientów (np. katalogi `tests/client_raw/` i `tests/temp/` w xml.pro) zawierają prawdziwe dane podatników.
**Nie kopiujemy ich tutaj, nie cytujemy ich treści w dokumentach, testach ani opisach commitów.** Wolno je użyć lokalnie
do porównania z libxml2 (K3): `tools/compare` czyta katalog wskazany zmienną `GOXSDGO_PRIVATE_CORPUS`, a do
`docs/testing.md` trafiają tylko liczby (ile par, ile zgodnych) i kody różnic, bez nazw plików i wartości.

Dokumenty syntetyczne budujemy generatorem z danymi wymyślonymi (NIP z poprawną cyfrą kontrolną, ale nieistniejący).

## Sumy kontrolne

| Plik | SHA-256 | Pobrano |
|---|---|---|
| — | — | uzupełnia etap 0 |
