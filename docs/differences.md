# Różnice względem rekomendacji i walidatora odniesienia

Każda różnica werdyktu między `goxsdgo` a libxml2 (przez `goxsdval`) i każde wyłączenie przypadku z zestawu W3C ma tu
wpis: przypadek, oba wyniki, co mówi rekomendacja (rozdział i reguła), kto ma rację, skutek dla odbiorcy.

| Przypadek | `goxsdgo` | libxml2 | Rekomendacja | Skutek dla odbiorcy |
|---|---|---|---|---|
| — | — | — | — | jeszcze bez kodu |

## Wyłączenia z zestawu W3C

| Grupa / przypadek | Powód |
|---|---|
| przypadki XSD 1.1 (`version="1.1"` w metadanych) | poza zakresem (ADR 0001) |
| przypadki wymagające walidacji DTD | poza zakresem (ADR 0001) |
