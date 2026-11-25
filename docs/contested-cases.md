# Przypadki sporne zestawu W3C w porównaniu z innymi walidatorami

Plik generowany przez `tools/bench` (`go -C tools/bench run . -root ../.. -docs contested`, po `w3c`);
nie edytuj go ręcznie. Stan: 2026-11-25, zestaw W3C z commitu `7bc3365c`, zakres XSD 1.0 2e jak `TestW3CSuite`.

Decyzja właściciela z 2026-11-25: w porównaniu z innymi walidatorami przypadki sporne są wyłączane albo
opatrzone zastrzeżeniem. Kryteria są ustalone z góry, obiektywne i te same dla wszystkich kandydatów;
przypadek dostaje kategorię pierwszego pasującego kryterium. W przypadkach spornych goxsdgo idzie za
tekstem rekomendacji (`differences.md`).

## Kryteria

| Kategoria | Kryterium | Przypadki (pierwsza pasująca) | Pasuje też, liczone wcześniej |
|---|---|---|---|
| a | status queried, disputed-test albo disputed-spec w metadanych zestawu | 63 | 0 |
| b | wynik zależy od wydania XML 1.0 (reguły znaków nazw) | 53 | 0 |
| c | wynik zależy od wersji Unicode | 0 | 18 |
| d | sprzeczne oczekiwania wewnątrz zestawu dla tej samej sytuacji (oba końce sprzeczności) | 17 | 3 |
| e | oczekiwanie sprzeczne z jednoznacznym tekstem rekomendacji 2e | 0 | 0 |
| razem | | 133 z 39439 w zakresie | |

Kategoria e jest pusta: żaden rozważony przypadek nie miał jednoznacznego cytatu (niżej).

## Wyniki

| Kandydat | Wszystkie w zakresie | Bez spornych | Sporne zaliczone | a | b | c | d | e |
|---|---|---|---|---|---|---|---|---|
| goxsdgo | 39369 / 39439 | 39295 / 39306 | 74 / 133 | 37 | 34 | 0 | 3 | 0 |
| goxsdgo (XML 1.0 5e names) | 39385 / 39439 | 39295 / 39306 | 90 / 133 | 37 | 50 | 0 | 3 | 0 |
| libxml2 2.15.4 | 38662 / 39439 | 38590 / 39306 | 72 / 133 | 27 | 40 | 0 | 5 | 0 |
| knroy/go-xml | 39386 / 39439 | 39281 / 39306 | 105 / 133 | 37 | 53 | 0 | 15 | 0 |
| jacoelho/xsd | 39083 / 39439 | 39015 / 39306 | 68 / 133 | 32 | 22 | 0 | 14 | 0 |
| cB-Sigma/xsd | 39329 / 39439 | 39257 / 39306 | 72 / 133 | 25 | 45 | 0 | 2 | 0 |

libxml2 przez `goxsdval` (`tools/compare`): przypadki, których to narzędzie nie umie uruchomić (schemat z kilku
dokumentów, schemat tylko ze wskazówek), liczą się jako niezaliczone, jak w tabeli w README.

## Rozważone i niewłączone

| Przypadki | Powód |
|---|---|
| particlesZ035_a, particlesZ036_b1, particlesZ036_b2, particlesZ036_c (dokumenty) | goxsdgo kończy je błędem limitu (`ErrLimit`, modele niejednoznaczne co do liczników, ADR 0011, pytanie 9): to ograniczenie goxsdgo, nie spór o oczekiwanie |
| IDREFS_length006, NMTOKENS_length006 (testy schematów, stable) | po ponownym przeczytaniu 4.3.1.4 (length-minLength-maxLength, punkt 1.2) oczekiwanie zestawu jest zgodne z tekstem: typy wbudowane IDREFS i NMTOKENS mają już minLength 1 bez length, więc minLength 1 obok length 5 jest dozwolone; to błąd goxsdgo do poprawienia, nie spór |
| attP031 (schemat i dokument) | zestaw rozdziela oczekiwania dla XSD 1.0 (poprawny) i 1.1 (błędny); tekst 3.2.2 o `use="prohibited"` nie jest tu jednoznaczny wobec tej decyzji grupy roboczej |
| anyURI_b006 (test schematu) | zestaw rozdziela oczekiwania dla XSD 1.0 (błędny) i 1.1 (poprawny); odczyt 3.2.17 przez XLink §5.4 nie jest tu jednoznaczny wobec tej decyzji grupy roboczej |
| pozostałe przypadki, które goxsdgo oblewa | nie spełniają żadnego kryterium; samo to, że goxsdgo je oblewa, nie czyni ich spornymi |

## Lista przypadków spornych

Kolumny kandydatów: ✓ zaliczony, · niezaliczony.

| Przypadek | Kat. | Oczekiwane | goxsdgo | goxsdgo (XML 1.0 5e names) | libxml2 2.15.4 | knroy/go-xml | jacoelho/xsd | cB-Sigma/xsd | Powód |
|---|---|---|---|---|---|---|---|---|---|
| `ElemDecl/valueconstraint00501m1/Positive` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `ElemDecl/valueconstraint00501m2/Positive` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB159/addB159.i` | a | invalid | ✓ | ✓ | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB161/addB161.i` | a | invalid | ✓ | ✓ | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB162/addB162.i` | a | invalid | ✓ | ✓ | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB164/addB164.i` | a | invalid | ✓ | ✓ | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB166/addB166.i` | a | invalid | ✓ | ✓ | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB187/addB187` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB187/addB187.v` | a | valid | ✓ | ✓ | · | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB188/addB188` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Additional2006-07-15/addB188/addB188.i` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-DataTypes2006-07-15/anyURI_a004_1339/anyURI_a004_1339.i` | a | invalid | · | · | ✓ | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-DataTypes2006-07-15/gMonth002_2061/gMonth002_2061` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-DataTypes2006-07-15/gMonth002_2061/gMonth002_2061.v` | a | valid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-DataTypes2006-07-15/gMonth004_2063/gMonth004_2063` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-DataTypes2006-07-15/gMonth004_2063/gMonth004_2063.v` | a | valid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Element2006-07-15/elemM002/elemM002` | a (też d) | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Element2006-07-15/elemZ026/elemZ026` | a | invalid | ✓ | ✓ | · | ✓ | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Element2006-07-15/elemZ027_c/elemZ027_c` | a | invalid | · | · | · | ✓ | · | · | status w metadanych zestawu W3C: queried |
| `MS-Errata102006-07-15/errF001/errF001` | a | valid | · | · | ✓ | ✓ | · | ✓ | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idH031/idH031.v` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: disputed-spec |
| `MS-IdentityConstraint2006-07-15/idI014/idI014` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idI016/idI016` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idI022/idI022` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idJ015/idJ015` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idJ017/idJ017` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idJ030/idJ030` | a | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-IdentityConstraint2006-07-15/idK011/idK011.v` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: disputed-spec |
| `MS-IdentityConstraint2006-07-15/idL076/idL076.v` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: disputed-spec |
| `MS-IdentityConstraint2006-07-15/idL077/idL077.v` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: disputed-spec |
| `MS-Particles2006-07-15/particlesV020/particlesV020` | a | invalid | ✓ | ✓ | · | ✓ | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Particles2006-07-15/particlesW006/particlesW006` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ11/reJ11.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ13/reJ13.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ19/reJ19.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ21/reJ21.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ23/reJ23.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ25/reJ25.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ29/reJ29.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ31/reJ31.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ33/reJ33.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ35/reJ35.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ61/reJ61.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ69/reJ69.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ75/reJ75.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reJ77/reJ77.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reL98/reL98.i` | a | invalid | ✓ | ✓ | · | · | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reL99/reL99.i` | a | invalid | ✓ | ✓ | · | · | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reM98/reM98.i` | a | invalid | ✓ | ✓ | · | · | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reN99/reN99.i` | a | invalid | ✓ | ✓ | · | · | ✓ | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reS21/reS21.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reS42/reS42.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reT63/reT63.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reT84/reT84.i` | a (też c) | invalid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `MS-Schema2006-07-15/schG11/schG11` | a | invalid | ✓ | ✓ | · | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Schema2006-07-15/schG12/schG12` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Schema2006-07-15/schG3/schG3` | a | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Schema2006-07-15/schG6/schG6` | a | invalid | ✓ | ✓ | · | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Schema2006-07-15/schU1/schU1` | a | valid | · | · | ✓ | ✓ | · | · | status w metadanych zestawu W3C: disputed-test |
| `MS-SimpleType2006-07-15/stZ073b/stZ073b.i` | a | invalid | · | · | ✓ | ✓ | ✓ | · | status w metadanych zestawu W3C: disputed-spec |
| `MS-SimpleType2006-07-15/ste110/ste110` | a | valid | · | · | · | · | · | · | status w metadanych zestawu W3C: queried |
| `suntest/xsd015.e/xsd015.e` | a (też d) | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `suntest/xsd016.e/xsd016.e` | a (też d) | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | status w metadanych zestawu W3C: queried |
| `MS-Regex2006-07-15/reZ006i/reZ006i` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | klasa \c według produkcji CombiningChar XML 1.0 2e (koniec przedziału na U+0345) albo nowszego wydania; ADR 0005, ADR 0012 |
| `MS-Regex2006-07-15/reZ006i/reZ006i.i` | b | valid | · | ✓ | · | ✓ | ✓ | ✓ | klasa \c według produkcji CombiningChar XML 1.0 2e (koniec przedziału na U+0345) albo nowszego wydania; ADR 0005, ADR 0012 |
| `XmlVersions/xv001/xv001.v01.xml` | b | valid | · | ✓ | ✓ | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv001/xv001.xsd` | b | valid | · | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv002/xv002.v01.xml` | b | valid | · | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv002/xv002.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv003/xv003.v01.xml` | b | valid | · | · | · | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv003/xv003.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv004/xv004.v01.xml` | b | valid | · | ✓ | ✓ | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv004/xv004.xsd` | b | valid | · | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv005/xv005.n01.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv005/xv005.n02.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv005/xv005.v01.xml` | b | valid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv005/xv005.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv006/xv006.n01.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv006/xv006.n02.xml` | b | invalid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv006/xv006.n03.xml` | b | invalid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv006/xv006.v01.xml` | b | valid | · | · | · | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv006/xv006.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv007/xv007.n01.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv007/xv007.n02.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv007/xv007.v01.xml` | b | valid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv007/xv007.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv008/xv008.n01.xml` | b | invalid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv008/xv008.n02.xml` | b | invalid | · | ✓ | · | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv008/xv008.v01.xml` | b | valid | · | · | · | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv008/xv008.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.n01.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.n02.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.n03.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | · | · | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.v01.xml` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.v02.xml` | b | valid | · | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv009/xv009.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | · | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100c/xv100c.c.xml` | b | valid | · | ✓ | · | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100c/xv100c.i.xml` | b | valid | · | ✓ | · | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100c/xv100c.notc.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100c/xv100c.noti.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100c/xv100c.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100i/xv100i.c.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100i/xv100i.i.xml` | b | valid | · | ✓ | · | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100i/xv100i.notc.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100i/xv100i.noti.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100i/xv100i.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100notc/xv100notc.c.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100notc/xv100notc.i.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100notc/xv100notc.notc.xml` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100notc/xv100notc.noti.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100notc/xv100notc.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100noti/xv100noti.c.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100noti/xv100noti.i.xml` | b | invalid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100noti/xv100noti.notc.xml` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100noti/xv100noti.noti.xml` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `XmlVersions/xv100noti/xv100noti.xsd` | b | valid | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | znaki nazw według XML 1.0 2e (odsyłacz XSD 1.0 2e) albo 5e (oczekiwanie grupy); goxsdgo rozstrzyga opcją Config.NameRules, ADR 0012 |
| `MS-IdentityConstraint2006-07-15/idB005/idB005` | d | invalid | ✓ | ✓ | ✓ | ✓ | · | ✓ | przeciwny koniec sprzeczności z grupą Missing (błąd src-resolve dla nierozwiązanego odwołania w nieużywanej deklaracji); ADR 0012 |
| `MS-Particles2006-07-15/particlesHa161/particlesHa161` | d | invalid | ✓ | ✓ | · | ✓ | ✓ | · | RecurseAsIfGroup (3.9.6): particlesZ001 wymaga przeniesienia liczności elementu na grupę, particlesHa161 i particlesK006 — odczytu dosłownego; ADR 0007 |
| `MS-Particles2006-07-15/particlesK006/particlesK006` | d | invalid | ✓ | ✓ | · | ✓ | ✓ | · | RecurseAsIfGroup (3.9.6): particlesZ001 wymaga przeniesienia liczności elementu na grupę, particlesHa161 i particlesK006 — odczytu dosłownego; ADR 0007 |
| `MS-Particles2006-07-15/particlesZ001/particlesZ001` | d | valid | · | · | ✓ | · | · | ✓ | RecurseAsIfGroup (3.9.6): particlesZ001 wymaga przeniesienia liczności elementu na grupę, particlesHa161 i particlesK006 — odczytu dosłownego; ADR 0007 |
| `MS-Particles2006-07-15/particlesZ001/particlesZ001.i` | d | invalid | · | · | · | · | · | · | RecurseAsIfGroup (3.9.6): particlesZ001 wymaga przeniesienia liczności elementu na grupę, particlesHa161 i particlesK006 — odczytu dosłownego; ADR 0007 |
| `Missing/missing001/missing001.n1.xml` | d | invalid | · | · | ✓ | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing001/missing001.v1.xml` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing001/missing001.xsd` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing002/missing001.n1.xml` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing002/missing001.v1.xml` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing002/missing001.xsd` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing003/missing003.n1.xml` | d | invalid | · | · | ✓ | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing003/missing003.v1.xml` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing003/missing003.xsd` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing006/missing006.n1.xml` | d | invalid | · | · | ✓ | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing006/missing006.v1.xml` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
| `Missing/missing006/missing006.xsd` | d | valid | · | · | · | ✓ | ✓ | · | nierozwiązane odwołanie w nieużywanej deklaracji: grupa Missing oczekuje schematu poprawnego (§5.3), a elemM002, xsd015.e, xsd016.e i idB005 dla tej samej sytuacji błędu src-resolve; ADR 0012 |
