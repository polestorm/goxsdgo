# Katalog kodów błędów

Plik generowany z `xsd/codes.go` i `xsd/compat.go` (`go test ./xsd -run TestErrorsDoc -update`); nie edytuj go ręcznie.
Decyzje: `docs/adr/0009-error-catalog-and-compat.md`. Kody są stałe; komunikat (`Error.Message`) jest po angielsku
i może się zmieniać, a do tłumaczenia służy kod i parametry.

- **Rodzaj**: `schema` — błąd schematu (`SchemaError.Errors`); `document` — błąd dokumentu (wynik `Validate`);
  `limit` — limit biblioteki albo konstrukcja nieobsługiwana, kod tylko w treści błędu zwracanego drugim wynikiem
  (`ErrLimit`, `errors.ErrUnsupported`).
- **Parametry**: pola `Error`, które błąd może ustawić poza `Code`, `Line`, `Column`, `Message`.
- **libxml2**: odpowiednik z tabeli compat (kryterium K8); „observed n/m” — ustalony na tych samych danych przez
  `tools/compare`, „assigned” — przypisany, z podaną podstawą.
- **Własny**: kod nadany przez goxsdgo, bo rekomendacja nie nazywa tej reguły.

Kodów: 234 (schema 143, document 80, limit 11); własnych 41; z odpowiednikiem libxml2 223.

## Błędy dokumentu

| Kod | Reguła | Parametry | libxml2 | Opis |
|---|---|---|---|---|
| `cvc-attribute.1` | [XSD 1 3.2.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-attribute) | Path, Name | 1878 (observed 14/14) | No global attribute declaration where a strict wildcard requires one. |
| `cvc-attribute.4` | [XSD 1 3.2.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-attribute) | Path, Name, Value | 1874 (observed 5/5) | The attribute value does not match the fixed value of its declaration. |
| `cvc-au` | [XSD 1 3.5.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-au) | Path, Name, Value | 1874 (observed 48/48) | The attribute value does not match the fixed value of its attribute use. |
| `cvc-complex-type.2.1` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1841 (observed 23/23) | An element with empty content has text or child elements. |
| `cvc-complex-type.2.2` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1842 (observed 2/2) | An element with simple content has child elements. |
| `cvc-complex-type.2.3` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name, Value | 1843 (observed 23/23) | An element with element-only content has text other than white space. |
| `cvc-complex-type.2.4` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name, Expected | 1871 (observed 794/805) | Child elements do not match the content model: an unexpected element or a missing one (Expected lists what is allowed). |
| `cvc-complex-type.3.2.1` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1866 (observed 76/78) | An attribute that is not declared and not allowed by an attribute wildcard. |
| `cvc-complex-type.3.2.2` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1867 (observed 76/76) | An attribute whose namespace the attribute wildcard does not allow. |
| `cvc-complex-type.4` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1868 (observed 17/17) | A required attribute is missing. |
| `cvc-complex-type.5.1` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1869 (assigned by meaning; libxml2 accepts the probe document) | More than one attribute of type ID through the attribute wildcard. |
| `cvc-complex-type.5.2` | [XSD 1 3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type) | Path, Name | 1869 (assigned by meaning; libxml2 accepts the probe document) | An attribute of type ID through the wildcard on a type that already declares one. |
| `cvc-datatype-valid.1.2.1` | [XSD 2 4.1.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-datatype-valid) | Path, Name, Value | 1824 (observed 731/731) | The value is not in the lexical space of the type. |
| `cvc-datatype-valid.1.2.2` | [XSD 2 4.1.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-datatype-valid) | Path, Name, Value | 1824 (observed 4/4) | An item of a list value is not valid for the item type. |
| `cvc-datatype-valid.1.2.3` | [XSD 2 4.1.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-datatype-valid) | Path, Name, Value | 1826 (observed 34/34) | The value matches no member type of the union. |
| `cvc-elt.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1845 (observed 42/44) | No element declaration where strict assessment requires one (validation root or strict wildcard). |
| `cvc-elt.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1846 (observed 2/2) | The element declaration is abstract. |
| `cvc-elt.3.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1847 (observed 12/12) | xsi:nil on an element whose declaration is not nillable. |
| `cvc-elt.3.2.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1848 (observed 30/30) | An element with xsi:nil="true" has content. |
| `cvc-elt.3.2.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1849 (observed 2/2) | An element with xsi:nil="true" has a fixed value constraint. |
| `cvc-elt.4.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name, Value | 1824 (observed 2/2) | The xsi:type value is not a valid QName. |
| `cvc-elt.4.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name, Value | 1851 (observed 8/9) | The xsi:type value does not resolve to a type definition. |
| `cvc-elt.4.3` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1852 (observed 355/355) | The xsi:type type is not validly derived from the declared type, or the derivation is blocked. |
| `cvc-elt.5.1.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name, Value, Facet | 1833 (observed 4/8) | The default value is not valid for the type given by xsi:type. |
| `cvc-elt.5.2.2.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name | 1857 (observed 2/2) | An element with a fixed value has child elements. |
| `cvc-elt.5.2.2.2.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name, Value | 1857 (observed 8/8) | The content of a mixed element does not match its fixed value. |
| `cvc-elt.5.2.2.2.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt) | Path, Name, Value | 1858 (observed 64/64) | The value of an element does not match its fixed value. |
| `cvc-entity` | [XSD 2 3.3.11](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#ENTITY), własny | Path, Name, Value | 1824 (assigned by meaning (value not valid for its type)) | An ENTITY or ENTITIES value is not the name of an unparsed entity declared in the document. |
| `cvc-enumeration-valid` | [XSD 2 4.3.5.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-enumeration-valid) | Path, Name, Value, Facet | 1840 (observed 2850/2852) | The value is not one of the enumerated values. |
| `cvc-fractionDigits-valid` | [XSD 2 4.3.12.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-fractionDigits-valid) | Path, Name, Value, Facet | 1838 (observed 50/50) | The value has more fraction digits than fractionDigits. |
| `cvc-id.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id) | Path, Value | 1824 (assigned like cvc-id.2; libxml2 does not check IDREF values) | An IDREF value does not match any ID in the document. |
| `cvc-id.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id) | Path, Name, Value | 1824 (observed 5/5) | An ID value is not unique in the document. |
| `cvc-identity-constraint.3` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path | 1877 (observed 24/24) | A field of an identity constraint selects more than one node or a node without a simple value. |
| `cvc-identity-constraint.4.1` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path, Value | 1877 (observed 66/66) | Duplicate value for a unique constraint. |
| `cvc-identity-constraint.4.2.1` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path | 1877 (observed 31/31) | A field of a key is missing. |
| `cvc-identity-constraint.4.2.2` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path, Value | 1877 (observed 44/44) | Duplicate value for a key. |
| `cvc-identity-constraint.4.2.3` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path | 1877 (observed 2/2) | A field of a key selects an element with xsi:nil="true". |
| `cvc-identity-constraint.4.3` | [XSD 1 3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-identity-constraint) | Path, Value | 1877 (observed 67/67) | A keyref value has no matching key or unique value. |
| `cvc-length-valid` | [XSD 2 4.3.1.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-length-valid) | Path, Name, Value, Facet | 1830 (observed 2304/2304) | The value does not have the required length. |
| `cvc-maxExclusive-valid` | [XSD 2 4.3.8.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-maxExclusive-valid) | Path, Name, Value, Facet | 1836 (observed 1094/1094) | The value is not less than maxExclusive. |
| `cvc-maxInclusive-valid` | [XSD 2 4.3.7.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-maxInclusive-valid) | Path, Name, Value, Facet | 1834 (observed 1242/1275) | The value is greater than maxInclusive. |
| `cvc-maxLength-valid` | [XSD 2 4.3.3.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-maxLength-valid) | Path, Name, Value, Facet | 1832 (observed 2288/2288) | The value is longer than maxLength. |
| `cvc-minExclusive-valid` | [XSD 2 4.3.9.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-minExclusive-valid) | Path, Name, Value, Facet | 1835 (observed 1058/1058) | The value is not greater than minExclusive. |
| `cvc-minInclusive-valid` | [XSD 2 4.3.10.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-minInclusive-valid) | Path, Name, Value, Facet | 1833 (observed 1192/1225) | The value is less than minInclusive. |
| `cvc-minLength-valid` | [XSD 2 4.3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-minLength-valid) | Path, Name, Value, Facet | 1831 (observed 2276/2284) | The value is shorter than minLength. |
| `cvc-notation` | [XSD 2 3.2.19](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#NOTATION), własny | Path, Name, Value | 1824 (assigned by meaning (value not valid for its type)) | A NOTATION value is not the name of a notation in the schema. |
| `cvc-pattern-valid` | [XSD 2 4.3.4.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-pattern-valid) | Path, Name, Value, Facet | 1839 (observed 5396/5568) | The value does not match a pattern facet. |
| `cvc-totalDigits-valid` | [XSD 2 4.3.11.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cvc-totalDigits-valid) | Path, Name, Value, Facet | 1837 (observed 648/648) | The value has more digits than totalDigits. |
| `cvc-type.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-type) | Path, Name | 1876 (observed 10/10) | The type definition is abstract. |
| `cvc-type.3.1.1` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-type) | Path, Name | 1827 (observed 2/2) | An element of simple type has attributes other than xsi attributes. |
| `cvc-type.3.1.2` | [XSD 1 3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-type) | Path, Name | 1828 (observed 6/6) | An element of simple type has child elements. |
| `ns-qname` | [Namespaces 1.0 7](https://www.w3.org/TR/2009/REC-xml-names-20091208/#Conformance), własny | Path | 202 (observed 2/2) | A name is not a valid QName, or a colon in an entity, notation or processing instruction name. |
| `nsc-attributes-unique` | [Namespaces 1.0 6.3](https://www.w3.org/TR/2009/REC-xml-names-20091208/#nsc-AttrsUnique) | Path | 203 (observed 2/2) | Two attributes with the same expanded name on one element. |
| `nsc-no-prefix-undeclaring` | [Namespaces 1.0 5](https://www.w3.org/TR/2009/REC-xml-names-20091208/#nsc-NoPrefixUndecl) | Path | 200 (observed 2/2) | A prefixed namespace declaration with an empty value (xmlns:p=""). |
| `nsc-prefix-declared` | [Namespaces 1.0 5](https://www.w3.org/TR/2009/REC-xml-names-20091208/#nsc-NSDeclared) | Path | 201 (observed 2/2) | A namespace prefix is used without a declaration. |
| `nsc-reserved-prefixes` | [Namespaces 1.0 3](https://www.w3.org/TR/2009/REC-xml-names-20091208/#xmlReserved) | Path | 200 (observed 2/2) | Misuse of the reserved prefixes xml or xmlns or of their namespace names. |
| `schema-location-hint` | [XSD 1 4.3.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema_reference), własny | Path, Name | 1839 (observed 1/5) | The schema located by xsi:schemaLocation or xsi:noNamespaceSchemaLocation is not valid (only with SchemaLocationHints). |
| `wfc-element-type-match` | [XML 1.0 3](https://www.w3.org/TR/2008/REC-xml-20081126/#GIMatch) | Path | 76 (observed 6/6) | The end tag does not match the start tag. |
| `wfc-entity-declared` | [XML 1.0 4.1](https://www.w3.org/TR/2008/REC-xml-20081126/#wf-entdeclared) | Path | 26 (observed 2/2) | A reference to an entity that is not declared. |
| `wfc-legal-character` | [XML 1.0 4.1](https://www.w3.org/TR/2008/REC-xml-20081126/#wf-Legalchar) | Path | 9 (observed 10/10) | A character reference to a character not allowed in XML. |
| `wfc-no-external-entity-references` | [XML 1.0 3.1](https://www.w3.org/TR/2008/REC-xml-20081126/#NoExternalRefs) | Path | 29 (observed 2/2) | An attribute value refers to an external entity. |
| `wfc-no-lt-in-attribute-values` | [XML 1.0 3.1](https://www.w3.org/TR/2008/REC-xml-20081126/#CleanAttrVals) | Path | 38 (observed 2/2) | The character "<" in an attribute value. |
| `wfc-no-recursion` | [XML 1.0 4.1](https://www.w3.org/TR/2008/REC-xml-20081126/#norecursion) | Path | 89 (observed 2/2) | An entity refers to itself, directly or indirectly. |
| `wfc-parsed-entity` | [XML 1.0 4.1](https://www.w3.org/TR/2008/REC-xml-20081126/#textent) | Path | 28 (observed 2/2) | A reference to an unparsed entity in content. |
| `wfc-pes-in-internal-subset` | [XML 1.0 2.8](https://www.w3.org/TR/2008/REC-xml-20081126/#wfc-PEinInternalSubset) | Path | 88 (observed 2/2) | A parameter entity reference inside a markup declaration in the internal subset. |
| `wfc-unique-att-spec` | [XML 1.0 3.1](https://www.w3.org/TR/2008/REC-xml-20081126/#uniqattspec) | Path | 42 (observed 2/2) | The same attribute appears twice in a start tag. |
| `xml-cdata-end` | [XML 1.0 2.4](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-CharData), własny | Path | 62 (observed 2/2) | The sequence "]]>" in character data. |
| `xml-char` | [XML 1.0 2.2](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-Char), własny | Path | 9 (observed 2/2) | A character outside the XML character range [2] Char. |
| `xml-comment` | [XML 1.0 2.5](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-Comment), własny | Path | 80 (observed 2/2) | The sequence "--" inside a comment. |
| `xml-content-outside-root` | [XML 1.0 2.1](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-document), własny | Path | 5 (observed 2/2) | Content other than comments, processing instructions and white space outside the root element. |
| `xml-decl` | [XML 1.0 2.8](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-XMLDecl), własny | Path | 96 (observed 2/2) | A malformed XML declaration. |
| `xml-encoding-invalid` | [XML 1.0 4.3.3](https://www.w3.org/TR/2008/REC-xml-20081126/#charencoding), własny | Path | 81 (observed 2/2) | The bytes are not valid in the document encoding. |
| `xml-encoding-mismatch` | [XML 1.0 4.3.3](https://www.w3.org/TR/2008/REC-xml-20081126/#charencoding), własny | Path | 32 (assigned by meaning; libxml2 accepts the probe document) | The encoding declaration does not match the detected encoding. |
| `xml-encoding-unsupported` | [XML 1.0 4.3.3](https://www.w3.org/TR/2008/REC-xml-20081126/#charencoding), własny | Path | 32 (observed 2/2) | The document uses an encoding the reader does not support (UTF-8, UTF-16, ISO-8859-1 and US-ASCII are supported). |
| `xml-entity-structure` | [XML 1.0 4.3.2](https://www.w3.org/TR/2008/REC-xml-20081126/#wf-entities), własny | Path | 77 (observed 2/2) | The replacement text of an entity is not well-formed content. |
| `xml-eof` | [XML 1.0 2.1](https://www.w3.org/TR/2008/REC-xml-20081126/#sec-well-formed), własny | Path | 77 (observed 2/2) | The document ends in the middle of a construct. |
| `xml-name` | [XML 1.0 2.3](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-Name), własny | Path | 68 (observed 2/2) | A name does not match [5] Name. |
| `xml-no-root` | [XML 1.0 2.1](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-document), własny | Path | 4 (observed 2/2) | The document has no root element. |
| `xml-reserved-pi-target` | [XML 1.0 2.6](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-PITarget), własny | Path | 64 (observed 2/2) | A processing instruction target reserved for XML ([Xx][Mm][Ll]). |
| `xml-syntax` | [XML 1.0 2](https://www.w3.org/TR/2008/REC-xml-20081126/#sec-documents), własny | Path | 41 (observed 2/4) | The text does not match the XML grammar at this point. |

## Błędy schematu

| Kod | Reguła | Parametry | libxml2 | Opis |
|---|---|---|---|---|
| `a-props-correct.2` | [XSD 1 3.2.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#a-props-correct) | Document | 1824 (observed 5/6) | The default or fixed value of an attribute is not valid for its type. |
| `a-props-correct.3` | [XSD 1 3.2.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#a-props-correct) | Document | 3090 (observed 7/7) | An attribute of type ID has a default or fixed value. |
| `ag-props-correct.2` | [XSD 1 3.6.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ag-props-correct) | Document | 3087 (observed 7/7) | An attribute is declared twice in an attribute group. |
| `ag-props-correct.3` | [XSD 1 3.6.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ag-props-correct) | Document | 3087 (observed 1/1) | An attribute group has more than one attribute of type ID. |
| `au-props-correct.2` | [XSD 1 3.5.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#au-props-correct) | Document | 1824 (assigned like a-props-correct.2) | An attribute use changes the fixed value of its declaration. |
| `c-fields-xpaths` | [XSD 1 3.11.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#c-fields-xpaths) | Document | 3037 (observed 26/26) | A field XPath is not in the restricted XPath subset. |
| `c-props-correct.1` | [XSD 1 3.11.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#c-props-correct) | Document | 3080 (observed 2/2) | A keyref refers to a keyref instead of a key or unique. |
| `c-props-correct.2` | [XSD 1 3.11.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#c-props-correct) | Document | 3080 (observed 3/3) | A keyref has a different number of fields than the key it refers to. |
| `c-selector-xpath` | [XSD 1 3.11.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#c-selector-xpath) | Document | 3037 (observed 25/25) | A selector XPath is not in the restricted XPath subset. |
| `cos-all-limited.1.2` | [XSD 1 3.8.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-all-limited) | Document | 3091 (observed 13/13) | An all group in a place where it is not allowed, or with maxOccurs other than 1. |
| `cos-applicable-facets` | [XSD 2 4.1.5](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cos-applicable-facets) | Document | 3029 (observed 15/25) | A facet that does not apply to the base type. |
| `cos-aw-union` | [XSD 1 3.10.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-aw-union) | Document | 1794 (observed 1/1) | The union of attribute wildcards is not expressible. |
| `cos-ct-extends.1.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (observed 8/8) | The base type of an extension has extension in its final. |
| `cos-ct-extends.1.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (assigned like cos-ct-extends.1.1) | The attribute wildcard of an extension does not include that of its base. |
| `cos-ct-extends.1.4` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (observed 2/2) | Complex content extends a type with simple content. |
| `cos-ct-extends.1.4.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (assigned like cos-ct-extends.1.1) | An extension changes the simple content type of its base. |
| `cos-ct-extends.1.4.3.2.2.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (observed 6/6) | An extension changes mixed or element-only content of its base. |
| `cos-ct-extends.2.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-ct-extends) | Document | 3063 (assigned like cos-ct-extends.1.1) | The simple base type of an extension has extension in its final. |
| `cos-element-consistent` | [XSD 1 3.8.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-element-consistent) | Document | 3070 (assigned like cos-nonambig) | A content model declares the same element name with different types (Element Declarations Consistent). |
| `cos-list-of-atomic` | [XSD 2 4.1.5](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#cos-list-of-atomic) | Document | 3015 (assigned like cos-st-restricts.2.1) | The item type of a list is not atomic or a union of atomic types. |
| `cos-nonambig` | [XSD 1 3.8.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-nonambig) | Document | 3070 (observed 30/30) | The content model is ambiguous (Unique Particle Attribution). |
| `cos-st-restricts.1.1` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-st-restricts) | Document | 3008 (observed 8/8) | anySimpleType is used as the base of a restriction. |
| `cos-st-restricts.2.1` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-st-restricts) | Document | 3015 (observed 3/3) | The item type of a list is a list or a union containing a list. |
| `cos-st-restricts.2.3.1.1` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-st-restricts) | Document | 3016 (observed 3/3) | The item type of a list has list in its final. |
| `cos-st-restricts.3.3.1.1` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-st-restricts) | Document | 3024 (observed 2/2) | A member type of a union has union in its final. |
| `cos-valid-default.2.1` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-valid-default) | Document | 3059 (observed 3/3) | An element value constraint needs simple or mixed content. |
| `cos-valid-default.2.2.2` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-valid-default) | Document | 3059 (observed 1/1) | An element value constraint on mixed content needs an emptiable particle. |
| `ct-props-correct.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ct-props-correct) | Document | 3009 (assigned like ct-props-correct.3) | A complex type with a simple base type is not an extension. |
| `ct-props-correct.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ct-props-correct) | Document | 3009 (observed 2/2) | Circular complex type definition. |
| `ct-props-correct.4` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ct-props-correct) | Document | 3087 (observed 5/5) | An attribute is declared twice in a complex type. |
| `ct-props-correct.5` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#ct-props-correct) | Document | 3087 (observed 1/1) | A complex type has more than one attribute of type ID. |
| `derivation-ok-restriction.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (observed 8/8) | The base type of a restriction has restriction in its final. |
| `derivation-ok-restriction.2.1.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1788 (observed 4/4) | An attribute required in the base is not required in the restriction. |
| `derivation-ok-restriction.2.1.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1789 (observed 1/1) | The type of an attribute is not derived from its type in the base. |
| `derivation-ok-restriction.2.1.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1789 (assigned like derivation-ok-restriction.2.1.2) | An attribute fixed in the base loses its fixed value. |
| `derivation-ok-restriction.2.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1790 (observed 1/1) | An attribute not allowed by the base type. |
| `derivation-ok-restriction.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1791 (observed 1/1) | A required attribute of the base is missing or optional. |
| `derivation-ok-restriction.4.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1797 (observed 1/1) | An attribute wildcard in a restriction of a type without one. |
| `derivation-ok-restriction.4.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1798 (observed 1/1) | The attribute wildcard is not a subset of the base wildcard. |
| `derivation-ok-restriction.4.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1799 (observed 1/1) | The attribute wildcard processes contents less strictly than the base. |
| `derivation-ok-restriction.5.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (assigned like derivation-ok-restriction.5.3) | Simple content cannot restrict the content of the base type. |
| `derivation-ok-restriction.5.2.2.1` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (observed 2/2) | The simple content is not derived from the simple content of the base. |
| `derivation-ok-restriction.5.3` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (observed 1/1) | Empty content cannot restrict the content of the base type. |
| `derivation-ok-restriction.5.4` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (assigned like derivation-ok-restriction.5.3) | Element content cannot restrict empty or simple content. |
| `derivation-ok-restriction.5.4.1.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 1787 (observed 3/3) | Mixed content cannot restrict element-only content. |
| `derivation-ok-restriction.5.4.2` | [XSD 1 3.4.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#derivation-ok-restriction) | Document | 3070 (observed 3/4) | The content model is not a valid restriction of the base content model (Particle Valid (Restriction)). |
| `e-props-correct.2` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#e-props-correct) | Document | 1824 (observed 11/14) | The default or fixed value of an element is not valid for its type. |
| `e-props-correct.4` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#e-props-correct) | Document | 3047 (observed 11/12) | The type of an element is not derived from the type of its substitution group head. |
| `e-props-correct.5` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#e-props-correct) | Document | 3048 (observed 4/4) | An element of type ID has a default or fixed value. |
| `e-props-correct.6` | [XSD 1 3.3.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#e-props-correct) | Document | 3049 (observed 3/3) | Circular substitution group. |
| `enumeration-required-notation` | [XSD 2 3.2.19](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#enumeration-required-notation) | Document | 1717 (assigned like facet-fixed; libxml2 accepts the probe schema) | A NOTATION type without an enumeration facet. |
| `enumeration-valid-restriction` | [XSD 2 4.3.5.5](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#enumeration-valid-restriction) | Document | 1824 (observed 42/44) | An enumeration value is not in the value space of the base type. |
| `facet-fixed` | [XSD 2 4.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-facets), własny | Document | 1717 (observed 13/13) | A facet that the base type fixes gets a different value. |
| `facet-value` | [XSD 2 4.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-facets), własny | Document | 1824 (observed 123/123) | A facet value is not a literal of the required type. |
| `fractionDigits-totalDigits` | [XSD 2 4.3.12.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#fractionDigits-totalDigits) | Document | 1717 (observed 1/1) | fractionDigits is greater than totalDigits. |
| `fractionDigits-valid-restriction` | [XSD 2 4.3.12.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#fractionDigits-valid-restriction) | Document | 1717 (observed 1/1) | fractionDigits is greater than in the base type. |
| `length-minLength-maxLength` | [XSD 2 4.3.1.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#length-minLength-maxLength) | Document | 1717 (observed 32/32) | length together with minLength or maxLength in a way the recommendation forbids. |
| `length-valid-restriction` | [XSD 2 4.3.1.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#length-valid-restriction) | Document | 1717 (observed 1/1) | length differs from the length of the base type. |
| `maxExclusive-valid-restriction` | [XSD 2 4.3.8.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#maxExclusive-valid-restriction) | Document | 1824 (observed 9/11) | maxExclusive is outside the range of the base type. |
| `maxInclusive-maxExclusive` | [XSD 2 4.3.8.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#maxInclusive-maxExclusive) | Document | 1717 (observed 24/24) | maxInclusive and maxExclusive in the same restriction step. |
| `maxInclusive-valid-restriction` | [XSD 2 4.3.7.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#maxInclusive-valid-restriction) | Document | 1824 (observed 10/16) | maxInclusive is outside the range of the base type. |
| `maxLength-valid-restriction` | [XSD 2 4.3.3.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#maxLength-valid-restriction) | Document | 1717 (observed 1/1) | maxLength is greater than in the base type. |
| `mg-props-correct.2` | [XSD 1 3.8.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#mg-props-correct) | Document | 3075 (observed 4/4) | Circular model group. |
| `minExclusive-less-than-equal-to-maxExclusive` | [XSD 2 4.3.8.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minExclusive-less-than-equal-to-maxExclusive) | Document | 1717 (observed 25/27) | minExclusive is greater than maxExclusive. |
| `minExclusive-less-than-maxInclusive` | [XSD 2 4.3.9.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minExclusive-less-than-maxInclusive) | Document | 1824 (observed 10/11) | minExclusive is not less than maxInclusive. |
| `minExclusive-valid-restriction` | [XSD 2 4.3.9.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minExclusive-valid-restriction) | Document | 1824 (observed 10/12) | minExclusive is outside the range of the base type. |
| `minInclusive-less-than-equal-to-maxInclusive` | [XSD 2 4.3.7.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minInclusive-less-than-equal-to-maxInclusive) | Document | 1717 (observed 25/47) | minInclusive is greater than maxInclusive. |
| `minInclusive-less-than-maxExclusive` | [XSD 2 4.3.10.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minInclusive-less-than-maxExclusive) | Document | 1824 (observed 10/14) | minInclusive is not less than maxExclusive. |
| `minInclusive-minExclusive` | [XSD 2 4.3.9.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minInclusive-minExclusive) | Document | 1717 (observed 1/1) | minInclusive and minExclusive in the same restriction step. |
| `minInclusive-valid-restriction` | [XSD 2 4.3.10.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minInclusive-valid-restriction) | Document | 1824 (observed 10/12) | minInclusive is outside the range of the base type. |
| `minLength-less-than-equal-to-maxLength` | [XSD 2 4.3.2.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minLength-less-than-equal-to-maxLength) | Document | 1717 (assigned like maxLength-valid-restriction) | minLength is greater than maxLength. |
| `minLength-valid-restriction` | [XSD 2 4.3.2.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#minLength-valid-restriction) | Document | 1717 (assigned like maxLength-valid-restriction) | minLength is less than in the base type. |
| `n-props-correct` | [XSD 1 3.12.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#n-props-correct) | Document | 3004 (observed 1/1) | A notation has neither a public nor a system identifier. |
| `no-xmlns` | [XSD 1 3.2.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#no-xmlns) | Document | 3056 (observed 2/2) | An attribute is named xmlns. |
| `no-xsi` | [XSD 1 3.2.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#no-xsi) | Document | 3057 (observed 2/2) | An attribute is declared in the XML Schema instance namespace. |
| `p-props-correct.2.1` | [XSD 1 3.9.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#p-props-correct) | Document | 3043 (observed 22/34) | minOccurs is greater than maxOccurs. |
| `p-props-correct.2.2` | [XSD 1 3.9.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#p-props-correct) | Document | 3044 (assigned as the second code observed with p-props-correct.2.1) | maxOccurs is less than 1. |
| `regex-syntax` | [XSD 2 F](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#regexs), własny | Document | 1756 (observed 574/574) | A pattern facet is not a valid XML Schema regular expression. |
| `regex-too-large` | [XSD 2 F](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#regexs), własny | Document | 1756 (assigned like regex-syntax) | A pattern is valid but too large after expanding quantifiers. |
| `sch-props-correct.1` | [XSD 1 3.15.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sch-props-correct) | Document | 3050 (observed 1/1) | targetNamespace is empty. |
| `sch-props-correct.2` | [XSD 1 3.15.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sch-props-correct) | Document | 1761 (observed 21/22) | Two components with the same name and kind. |
| `sfs-attribute` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 3035 (observed 86/112) | An attribute that is not allowed on this schema element. |
| `sfs-attribute-required` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 3036 (observed 9/12) | A required attribute of a schema element is missing. |
| `sfs-attribute-value` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 1717 (observed 317/668) | An attribute value of a schema element is outside its type. |
| `sfs-content` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 3033 (observed 601/620) | Children of a schema element do not match its content model in the schema for schemas. |
| `sfs-duplicate-id` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 3037 (observed 46/46) | Two id attributes with the same value in one schema document. |
| `sfs-not-schema` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 1772 (observed 1/1) | The root element of a schema document given to Compile is not xs:schema. |
| `sfs-not-well-formed` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 77 (observed 3/4) | A schema document is not well-formed XML with namespaces. |
| `sfs-text` | [XSD 1 A](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#normative-schemaSchema), własny | Document | 3033 (observed 2/2) | Text in a schema element that allows only elements. |
| `src-attribute.1` | [XSD 1 3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute) | Document | 3051 (observed 5/5) | xs:attribute has both default and fixed. |
| `src-attribute.2` | [XSD 1 3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute) | Document | 3052 (observed 6/6) | An attribute with a default does not have use="optional". |
| `src-attribute.3.1` | [XSD 1 3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute) | Document | 3035 (observed 2/2) | A local xs:attribute has both or neither of name and ref. |
| `src-attribute.3.2` | [XSD 1 3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute) | Document | 3035 (observed 5/7) | xs:attribute with ref has attributes or children that are not allowed. |
| `src-attribute.4` | [XSD 1 3.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute) | Document | 3055 (observed 4/4) | xs:attribute has both a type attribute and a simpleType child. |
| `src-attribute_group.2` | [XSD 1 3.6.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute_group) | Document | 3087 (assigned like ag-props-correct.2) | Two attribute uses for the same attribute in an attribute group. |
| `src-attribute_group.3` | [XSD 1 3.6.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-attribute_group) | Document | 3073 (observed 4/4) | Circular attribute group reference. |
| `src-ct.1` | [XSD 1 3.4.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-ct) | Document | 3076 (observed 2/2) | Complex content with a simple base type. |
| `src-ct.2.1` | [XSD 1 3.4.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-ct) | Document | 3076 (observed 8/8) | Simple content with a base type that has neither simple content nor emptiable mixed content. |
| `src-ct.2.2` | [XSD 1 3.4.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-ct) | Document | 3076 (observed 1/1) | Restricting mixed content to simple content without a simpleType child. |
| `src-ct.4` | [XSD 1 3.4.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-ct) | Document | 3087 (assigned like ct-props-correct.4) | Two attribute uses for the same attribute in a complex type. |
| `src-element.1` | [XSD 1 3.3.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-element) | Document | 3038 (observed 6/6) | xs:element has both default and fixed. |
| `src-element.2.1` | [XSD 1 3.3.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-element) | Document | 3039 (observed 5/6) | A local xs:element has both or neither of name and ref. |
| `src-element.2.2` | [XSD 1 3.3.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-element) | Document | 3040 (observed 11/13) | xs:element with ref has attributes or children that are not allowed. |
| `src-element.3` | [XSD 1 3.3.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-element) | Document | 3041 (observed 2/2) | xs:element has both a type attribute and an anonymous type. |
| `src-import.1.1` | [XSD 1 4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-import) | Document | 3064 (observed 3/3) | A schema document imports its own target namespace. |
| `src-import.1.2` | [XSD 1 4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-import) | Document | 3065 (observed 4/4) | An import without a namespace in a schema document without a target namespace. |
| `src-import.2` | [XSD 1 4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-import) | Document | 1772 (observed 2/2) | The imported document is not a valid schema document. |
| `src-import.3.1` | [XSD 1 4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-import) | Document | 3069 (observed 2/2) | The imported document has a target namespace different from the import namespace. |
| `src-import.3.2` | [XSD 1 4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-import) | Document | 3069 (assigned like src-import.3.1) | An import without a namespace refers to a document with a target namespace. |
| `src-include.1` | [XSD 1 4.2.1](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-include) | Document | 1772 (observed 1/1) | The included document is not a valid schema document. |
| `src-include.2.1` | [XSD 1 4.2.1](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-include) | Document | 3050 (observed 2/2) | The included document has a different target namespace. |
| `src-qname` | [XSD 1 3.15.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-qname) | Document | 3037 (observed 5/5) | The prefix of a QName value in a schema document is not declared. |
| `src-redefine` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (observed 5/9) | A redefinition refers to a component the redefined document does not define. |
| `src-redefine.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (assigned like src-redefine.6.1.2) | xs:redefine with components has a schemaLocation that does not resolve. |
| `src-redefine.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1772 (observed 1/1) | The redefined document is not a valid schema document. |
| `src-redefine.3.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3050 (observed 2/2) | The redefined document has a different target namespace. |
| `src-redefine.5` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (observed 5/6) | A redefined type is not derived from its original. |
| `src-redefine.6.1.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (assigned like src-redefine.6.1.2) | A redefined group refers to itself more than once. |
| `src-redefine.6.1.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (observed 2/2) | The self-reference in a redefined group does not have minOccurs and maxOccurs 1. |
| `src-redefine.6.2.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (assigned like src-redefine.6.1.2) | A redefined group is not a valid restriction of the original. |
| `src-redefine.7.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 3081 (observed 1/1) | A redefined attribute group refers to itself more than once. |
| `src-redefine.7.2.2.2.1.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1788 (assigned like derivation-ok-restriction.2.1.1) | Redefined attribute group: an attribute required in the original is not required (derivation-ok-restriction.2.1.1). |
| `src-redefine.7.2.2.2.1.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1789 (observed 3/3) | Redefined attribute group: an attribute type is not derived from the original (derivation-ok-restriction.2.1.2). |
| `src-redefine.7.2.2.2.1.3` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1789 (assigned like derivation-ok-restriction.2.1.2) | Redefined attribute group: an attribute fixed in the original loses its fixed value (derivation-ok-restriction.2.1.3). |
| `src-redefine.7.2.2.2.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1790 (observed 2/2) | Redefined attribute group: an attribute not allowed by the original (derivation-ok-restriction.2.2). |
| `src-redefine.7.2.2.3` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1791 (observed 1/1) | Redefined attribute group: a required attribute of the original is missing (derivation-ok-restriction.3). |
| `src-redefine.7.2.2.4.1` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1797 (assigned like derivation-ok-restriction.4.1) | Redefined attribute group: an attribute wildcard without one in the original (derivation-ok-restriction.4.1). |
| `src-redefine.7.2.2.4.2` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1798 (assigned like derivation-ok-restriction.4.2) | Redefined attribute group: the attribute wildcard is not a subset of the original (derivation-ok-restriction.4.2). |
| `src-redefine.7.2.2.4.3` | [XSD 1 4.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-redefine) | Document | 1799 (assigned like derivation-ok-restriction.4.3) | Redefined attribute group: the attribute wildcard is less strict than the original (derivation-ok-restriction.4.3). |
| `src-resolve` | [XSD 1 3.15.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-resolve) | Document | 3004 (observed 53/55) | A QName does not resolve to a component of the required kind. |
| `src-resolve.4.1` | [XSD 1 3.15.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-resolve) | Document | 3004 (observed 4/4) | A QName without a namespace is used, but the schema document neither has no target namespace nor imports the absent namespace. |
| `src-resolve.4.2` | [XSD 1 3.15.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-resolve) | Document | 3004 (observed 7/7) | A QName refers to a namespace the schema document neither targets nor imports. |
| `src-simple-type` | [XSD 1 3.14.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-simple-type) | Document | 3007 (observed 1/2) | Simple type definition representation constraint violated. |
| `src-simple-type.2` | [XSD 1 3.14.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-simple-type) | Document | 3005 (observed 1/1) | xs:restriction has both or neither of a base attribute and a simpleType child. |
| `src-simple-type.3` | [XSD 1 3.14.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-simple-type) | Document | 3000 (observed 1/1) | xs:list has both or neither of an itemType attribute and a simpleType child. |
| `src-simple-type.4` | [XSD 1 3.14.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#src-simple-type) | Document | 3003 (observed 1/1) | Circular union definition, or a union member type that cannot be resolved. |
| `src-single-facet-value` | [XSD 2 4.1.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#src-single-facet-value) | Document | 1717 (assigned like facet-fixed; libxml2 accepts the probe schema) | The same facet is specified twice in one restriction step. |
| `st-props-correct` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#st-props-correct) | Document | 3003 (assigned like st-props-correct.2) | Simple type definition properties are not correct. |
| `st-props-correct.1` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#st-props-correct) | Document | 3004 (observed 4/7) | The base, item or member type of a simple type is not simple. |
| `st-props-correct.2` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#st-props-correct) | Document | 3003 (observed 1/2) | Circular simple type definition. |
| `st-props-correct.3` | [XSD 1 3.14.6](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#st-props-correct) | Document | 3010 (observed 2/2) | The base type of a restriction has restriction in its final. |
| `totalDigits-valid-restriction` | [XSD 2 4.3.11.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#totalDigits-valid-restriction) | Document | 1717 (observed 1/1) | totalDigits is greater than in the base type. |
| `whiteSpace-valid-restriction` | [XSD 2 4.3.6.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#whiteSpace-valid-restriction) | Document | 1717 (observed 7/7) | whiteSpace is weaker than in the base type. |

## Limity i konstrukcje nieobsługiwane

| Kod | Reguła | Parametry | libxml2 | Opis |
|---|---|---|---|---|
| `limit-attributes` | —, własny | — | — | An element has more attributes than the reader limit (returned with ErrLimit). |
| `limit-depth` | —, własny | — | — | Element nesting exceeds the reader limit (returned with ErrLimit). |
| `limit-dtd-size` | —, własny | — | — | The internal DTD subset exceeds the reader limit (returned with ErrLimit). |
| `limit-entity-depth` | —, własny | — | — | Entity references are nested deeper than the reader limit (returned with ErrLimit). |
| `limit-entity-expansion` | —, własny | — | — | Entity expansion exceeds the reader limit (returned with ErrLimit). |
| `limit-name-length` | —, własny | — | — | A name exceeds the reader limit (returned with ErrLimit). |
| `limit-text-size` | —, własny | — | — | A text node exceeds the reader limit (returned with ErrLimit). |
| `schema-limit` | —, własny | — | — | Loading exceeded the limit on schema documents or nesting (returned with ErrLimit). |
| `unsupported-conditional-section` | [XML 1.0 3.4](https://www.w3.org/TR/2008/REC-xml-20081126/#sec-condition-sect), własny | — | — | A conditional section in a parameter entity (errors.ErrUnsupported). |
| `unsupported-entity-unresolved` | [XML 1.0 5.1](https://www.w3.org/TR/2008/REC-xml-20081126/#proc-types), własny | — | — | A reference to an entity that may be declared in the unread external subset (errors.ErrUnsupported). |
| `unsupported-external-entity` | [XML 1.0 5.1](https://www.w3.org/TR/2008/REC-xml-20081126/#proc-types), własny | — | — | A reference to an external entity in content; external entities are never read (errors.ErrUnsupported). |

