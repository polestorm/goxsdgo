# Projekt: interfejs publiczny, założenia, ograniczenia, zależności

Dokument teczki walidacyjnej modułu (dla odbiorców, którzy walidują goxsdgo jako klocek, np. QMS). Szczegóły decyzji
są w `docs/adr/`, wymagania w `requirements/`, znane ograniczenia w [known-issues.md](known-issues.md), zmiany
w [CHANGELOG.md](../CHANGELOG.md).

## Przeznaczenie i granice

goxsdgo kompiluje schematy XML Schema 1.0 (Second Edition, części 1 i 2) i waliduje według nich dokumenty XML,
strumieniowo. **Nie robi**: XSD 1.1, walidacji według DTD, XSLT, XPath poza ograniczeniami tożsamości,
generowania typów Go, reguł semantycznych spoza schematu (te są po stronie odbiorcy), tłumaczenia komunikatów
(odbiorca tłumaczy po kodzie błędu).

## Pakiety publiczne

| Pakiet | Co daje | Główne elementy |
|---|---|---|
| `xsd` | kompilacja schematu i walidacja dokumentów | `Compile`, `Config` (`Locations`, `Namespaces`, `Resolve`, limity, `SchemaLocationHints`, `OnValue`, `NameRules`), `Set.Validate`, `Set.ValidateDocument`, `Set.Documents`, `Error`, `SchemaError`, `ErrLimit`, `ErrTooManyErrors`, katalog kodów (`Codes`, `LookupCode`, `LibXML2Code`) |
| `xsd/resolve` | opcjonalne źródło dokumentów schematu dla `Config.Resolve` | `New(Options)`: `Dir`, `Cache`, `Network`, `AllowHosts`, `LocalPath`, `ErrDownload` |
| `xmlreader` | przestarzała warstwa zgodności: aliasy typów czytnika `goxmlgo/xmlreader` z API v0.3.0 (ADR 0014) | `New`, `DefaultLimits`, aliasy `Reader`, `Token`, `Attr`, `Error`, `Limits`, … |

Pakiety `internal/` nie są interfejsem. API przed 1.0 może się zmienić (pytanie 5 w `docs/adr/open-questions.md`);
każda zmiana ma w CHANGELOG klasę wpływu, zmiana niezgodna wstecz (klasa 3) ma notatkę migracji.

## Założenia i własności

- **Wzorcem jest rekomendacja W3C**, nie zachowanie libxml2; różnice są opisane (`docs/differences.md`).
- **Bezpieczny domyślnie**: pakiet `xsd` nie sięga do sieci (nie importuje `net/http`), nie czyta encji
  zewnętrznych ani podzbioru zewnętrznego DTD; limity głębokości, rozmiaru, liczby atrybutów, rozwinięć encji,
  liczby dokumentów schematu i zagnieżdżenia importów (ADR 0004, 0011). Dokumenty spoza `fs.FS` wyłącznie przez
  funkcję wywołującego `Config.Resolve` (ADR 0013); sieć tylko w `xsd/resolve` z `Options.Network`.
- **Strumieniowo, w ograniczonej pamięci**: pamięć zależy od głębokości i największego tokenu, nie od rozmiaru
  dokumentu (plik 1 GB: szczyt 6,3 MB sterty, K6).
- **Czas liniowy** względem dokumentu, także dla wzorców i liczników (K5, `TestPathological`); wyjątek
  w [known-issues.md](known-issues.md) KI-3 kończy się `ErrLimit`, nigdy werdyktem.
- **Deterministycznie**: ten sam schemat i dokument dają te same błędy w tej samej kolejności.
- **Błędy dla ludzi i programów**: stały kod nazwany od reguły rekomendacji, wiersz, kolumna, ścieżka elementu,
  parametry w polach (`docs/errors.md`).
- **Współbieżnie**: `Set` jest bezpieczny dla wielu gorutyn; `Config.Resolve` jest wołane tylko podczas `Compile`
  (i przy wczytaniu schematu ze wskazówek), z jednej gorutyny na kompilację.
- **Ślad wejścia**: `Set.Documents()` podaje nazwę, adres, źródło, rozmiar i SHA-256 każdego dokumentu schematu.

## Zależności

| Zależność | Uzasadnienie |
|---|---|
| biblioteka standardowa Go (≥ 1.22) | `CGO_ENABLED=0` (sprawdza CI) |
| `github.com/polestorm/goxmlgo` (v0.2.0) | jedyny moduł spoza biblioteki standardowej (ADR 0014): czytnik XML (`xmlreader`) i klasy znaków nazw (`xmlchars`), wydzielone z goxsdgo; sam bez zależności i bez cgo, tagi podpisane, CHANGELOG z klasą wpływu, własny pakiet dowodów; CI sprawdza, że `go list -m all` podaje tylko ten moduł |
| `net/http`, `crypto/tls` (tylko `xsd/resolve`) | pobieranie dokumentów, gdy wywołujący włączy `Network`; programy bez `xsd/resolve` ich nie linkują przez goxsdgo |

Narzędzia w `tools/` (porównania, fuzzing, pakiet dowodów, pobieranie danych testowych) są osobnymi modułami i nie
wchodzą do biblioteki; `tools/compare` używa cgo i libxml2 wyłącznie do porównań.

## Weryfikacja

Wymagania → testy → wynik: pakiet dowodów (`tools/evidence`, `docs/evidence.md`), dołączany do każdego wydania
przez CI razem z raportem testów, pokryciem, `staticcheck`, `govulncheck`, SBOM i zapisem środowiska
(`.github/workflows/release.yml`). Pomiary porównawcze: `docs/methodology.md`, surowe wyniki `docs/results/`.
