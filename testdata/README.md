# Dane testowe

Dane testowe nie są kodem i nie są objęte licencją `goxsdgo` (`../NOTICE`). **Cudzych danych nie przechowujemy
w repozytorium** (ADR 0003). Manifest `sources.json` opisuje, skąd je pobrać, a narzędzie `tools/fetch` pobiera je
i sprawdza sumy SHA-256.

Z Go (z katalogu głównego repozytorium):

```sh
go -C tools/fetch run . -root ../..          # pobiera brakujące pliki (ok. 15 s, ok. 250 MB na dysku)
go -C tools/fetch run . -root ../.. -verify  # sprawdza pobrane pliki bez sieci
```

Z Pythonem 3.9+ (bez Go, np. na Windows; z katalogu głównego repozytorium):

```bat
py tools\fetch\fetch.py            :: pobiera brakujące pliki
py tools\fetch\fetch.py --verify   :: sprawdza pobrane pliki bez sieci
py tools\fetch\fetch.py --from ..\xml.pro\internal\importing\schemas   :: także jpk_kr_pd_1.xsd z lokalnej kopii
```

Obie wersje dają te same pliki. Na Windows repozytorium nie powinno leżeć głębiej niż ok. 120 znaków od dysku, bo
najdłuższa ścieżka w zestawie W3C ma 136 znaków licząc od katalogu repozytorium, a Windows domyślnie ogranicza ścieżki
do 260 znaków.

| Katalog | Zawartość | Źródło | Licencja | W repozytorium |
|---|---|---|---|---|
| `w3c/xsdtests/` | zestaw testów W3C XML Schema: 42 270 plików, 78 plików `*.testSet`, 27 408 przypadków schematów i dokumentów | `github.com/w3c/xsdtests`, commit `7bc3365c` (2026-04-01) | W3C Document Notice and License (1999), plik `00COPYRIGHT` | nie, pobierany |
| `w3c/xmlconf/` | zestaw W3C XML Conformance 20130923: 3 386 plików (czytnik XML, etap 1) | `www.w3.org/XML/Test/xmlts20130923.tar.gz` | W3C; części od Sun, OASIS, IBM, Edinburgh (noty w plikach) | nie, pobierany |
| `w3c/schemas/` | `xml.xsd` i `xlink.xsd`, które `xsts.xsd` i trzy przypadki zestawu importują spod adresów w3.org | `www.w3.org` | W3C | nie, pobierane |
| `w3c/passing.txt` | zapadka: przypadki W3C, które `goxsdgo` przechodzi (`docs/testing.md`) | własne | Apache 2.0 | tak |
| `schemas/mf/` | schematy MF i KAS: JPK (V7M, V7K w wersjach 1–3, FA, KR, KR_PD, WB, MAG, EWP, PKPIR, VAT), KSeF FA(2) i FA(3), e-Sprawozdania z plikami pomocniczymi (`auxiliary/`): 170 plików | gov.pl, crd.gov.pl; lista i adresy z katalogu `../xml.pro/internal/importing/schemas/auxiliary.json`, uzupełnione o JPK_V7M(1) i V7K(1) z importami | materiały urzędowe (art. 4 ustawy o prawie autorskim) | nie, pobierane |
| `instances/mf/` | przykłady JPK_VAT (V7M i V7K w wersjach 1–3, InitUpload): 14 plików, dane fikcyjne MF | podatki.gov.pl, strona „Pliki do pobrania” JPK_VAT z deklaracją | materiały urzędowe | nie, pobierane |
| `instances/expected.json` | oczekiwany werdykt każdego dokumentu korpusu i jego schemat; 8 przykładów MF ma schemat, 6 dokumentów InitUpload nie ma (brak schematu metadanych przesyłki) | własne | Apache 2.0 | tak |
| `instances/` (poza `mf/`) | dokumenty syntetyczne (od etapu 5) | własne | Apache 2.0 | tak |
| `passing-corpus.txt` | zapadka: schematy i dokumenty korpusu, które `goxsdgo` przechodzi | własne | Apache 2.0 | tak |
| `corpus/` | małe schematy i dokumenty pod wymagania | własne | Apache 2.0 | tak |

Sumy kontrolne, adresy, przestrzenie nazw i importy każdego pliku są w `sources.json`. Zbiór schematów jest domknięty:
każdy `import` i `include` wskazuje plik z manifestu (sprawdzone 2026-11-25). Lista adresów służy też jako katalog:
testy i `tools/compare` rozwiązują przez nią `schemaLocation` bez sieci.

## Uwagi o źródłach (stan 2026-11-25)

- **169 ze 170 schematów** pobierają się spod adresów MF z oczekiwaną sumą.
- **`jpk_kr_pd_1.xsd`** nie ma znanego adresu pliku u MF (w katalogu xml.pro zapisano adres przestrzeni nazw).
  W manifeście ma znacznik `local_only`: narzędzie bierze go tylko z lokalnej kopii
  (`-from ../xml.pro/internal/importing/schemas`) i sprawdza sumę, a bez niej pomija go z komunikatem. Testy korpusu
  też go wtedy pomijają. Do uzupełnienia, gdy znajdziemy adres.
- **Nazwy plików pomocniczych się powtarzają**: np. `StrukturyDanych_v6-0E.xsd` z gov.pl i z CRD z 2020 r. mają inną
  treść. Plik dodany później, którego nazwa jest już zajęta, trafia do `auxiliary/<data z adresu>/`.
- **Kopie w xml.pro różnią się od oryginałów MF** w 7 schematach JPK w wersji 1 (EWP, FA, KR, MAG, PKPIR, VAT, WB).
  W sześciu różnica dotyczy tylko końców wierszy (MF: CRLF, xml.pro: LF). W `jpk_vat_1.xsd` różni się treść (m.in.
  inne importy). Manifest zapisuje sumy oryginałów MF.

## Dane prywatne: nigdy w repozytorium

Pliki od klientów (np. katalogi `tests/client_raw/` i `tests/temp/` w xml.pro) zawierają prawdziwe dane podatników.
**Nie kopiujemy ich tutaj, nie cytujemy ich treści w dokumentach, testach ani opisach commitów.** Wolno je użyć lokalnie
do porównania z libxml2 (K3): `tools/compare` czyta katalog wskazany zmienną `GOXSDGO_PRIVATE_CORPUS`, a do
`docs/testing.md` trafiają tylko liczby (ile par, ile zgodnych) i kody różnic, bez nazw plików i wartości.

Dokumenty syntetyczne budujemy generatorem z danymi wymyślonymi (NIP z poprawną cyfrą kontrolną, ale nieistniejący).
