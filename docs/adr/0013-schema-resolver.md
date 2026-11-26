# 0013 Dokumenty schematu spoza fs.FS: Config.Resolve, Set.Documents, pakiet resolve

Status: przyjęta · 2026-11-26 (decyzja właściciela)

## Kontekst

Do v0.3.0 dokumenty schematu pochodzą wyłącznie z `fs.FS`; adres bezwzględny w `schemaLocation` trzeba
przypisać plikowi w `Config.Locations`, inaczej zostaje nierozwiązany (ADR 0006). Odbiorcy (QMS, schematy MF)
chcą folderu lokalnego, pamięci podręcznej i, na życzenie, pobierania z sieci.

Odrzucony wariant: osobne narzędzie, które przed kompilacją przechodzi po include i import, pobiera pliki i buduje
`Locations`. Czyta każdy dokument dwa razy, a przede wszystkim powtarza reguły rozwiązywania (adres bazowy,
kameleony, redefine, import bez adresu, `Namespaces`, cykle, limity); dwie implementacje tych reguł z czasem się
rozjadą i pobiorą co innego, niż kompilator potem czyta (uwaga właściciela).

## Decyzja

1. **`Config.Resolve func(SchemaRef) (io.ReadCloser, error)`**: kompilator pyta tę funkcję w tym samym przebiegu
   wczytywania, w miejscu, w którym dotąd kończył na „adres bezwzględny bez wpisu”. Kolejność: `Locations`, ścieżka
   względna w `fs.FS`, dla importu `Namespaces`, na końcu `Resolve`. Dokument z `Resolve` ma za nazwę i adres
   bazowy swój URL, więc jego adresy względne też idą do `Resolve`; limity `MaxDocuments` i `MaxDepth` i kontrola
   cykli działają bez zmian. Każdy adres pada najwyżej raz w kompilacji (wynik jest zapamiętany, także „nie ma”).
2. **Błędy**: `fs.ErrNotExist` znaczy „nie ma”, jak bez `Resolve` (nierozwiązany include i import nie są błędem
   schematu, 4.2.1 i 4.2.3). Inny błąd to błąd środowiska, nie schematu: `Compile` kończy się błędem opakowanym, nie
   `*SchemaError`, bez nowego kodu błędu. Dokument z `Resolve` ma najwyżej `MaxResolvedSize` (64 MiB).
3. **Biblioteka nadal nie używa sieci.** Pakiet `xsd` nie importuje `net/http`; decyzję o źródle podejmuje
   wywołujący. Wskazówki `xsi:schemaLocation` z dokumentu (niezaufanego) nie trafiają do `Resolve`; trafiają tam
   tylko importy schematów, które wskazują (te leżą w `fs.FS`).
4. **`Set.Documents()`**: nazwa, adres, źródło, rozmiar i SHA-256 każdego wczytanego dokumentu, w kolejności
   wczytania. To zapis, z jakich dokładnie bajtów powstał schemat (dowód dla walidacji u odbiorcy).
5. **Pakiet `xsd/resolve`** (opcjonalny, importuje go wywołujący): układ `host/ścieżka`, kolejno `Dir` (tylko
   odczyt, np. `embed.FS`), `Cache` (katalog), sieć tylko z `Network`, tylko HTTP i HTTPS, z `AllowHosts`
   (także przekierowania), limitem czasu i rozmiaru; zapis do `Cache` atomowy. Błąd pobierania (`ErrDownload`)
   przerywa kompilację: przy włączonej sieci brak odpowiedzi nie może po cichu zmienić schematu.

## Skutki

- Moduł dalej bez zależności (`net/http` jest w bibliotece standardowej); `CGO_ENABLED=0` bez zmian.
- Wynik kompilacji z siecią zależy od serwera; dla powtarzalności odbiorca trzyma `Cache` (albo `Dir`) w swoim
  repozytorium lub archiwum i sprawdza `Set.Documents()`.
- Testy: `TestResolveRecursive`, `TestResolveOrder`, `TestResolveErrors` (xsd), `TestLocalPath`,
  `TestDirCacheNetwork` (xsd/resolve, lokalny serwer `httptest`, bez prawdziwej sieci). Wymagania SCH-022–SCH-024.
