# Wymagania: bezpieczeństwo

Decyzje: `docs/adr/0004-xml-reader.md`, `docs/adr/0011-hardening.md`. Brak sieci i encji zewnętrznych przy
ustawieniach domyślnych opisują XML-015, SCH-020 i VAL-020; tu są limity i obchodzenie się z danymi prywatnymi.

| Id | Wymaganie |
|---|---|
| SAFE-001 | Limity czytania dokumentu są w jednym miejscu (`xsd.DocumentLimits`, `DefaultDocumentLimits`): głębokość 512, nazwa 16 KiB, 4096 atrybutów, tekst 16 MiB, rozwinięcia encji 4 MiB, zagnieżdżenie encji 32, DOCTYPE 1 MiB. `Config.Document` zmienia je dla jednego `Set` (zero: wartość domyślna, wartość ujemna: bez limitu); przekroczenie daje `ErrLimit`. |
| SAFE-002 | Kod testów i narzędzi, który czyta prywatny korpus (`GOXSDGO_PRIVATE_CORPUS`), jest pomijany bez tej zmiennej i wypisuje wyłącznie liczby zbiorcze i kody błędów: bez nazw, ścieżek, wartości i fragmentów plików (`testdata/README.md`). |
| SAFE-003 | Dokumenty złośliwe nie powodują paniki ani nieograniczonego zużycia czasu i pamięci: każdy przypadek z DET-002 kończy się w czasie poniżej 2 s na typowym komputerze (`TestPathological`). |
