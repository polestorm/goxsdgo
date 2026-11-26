# Moving from libxml2 to goxsdgo

For programs that validate with libxml2 today (directly, or through cgo in Go) and translate or post-process its errors.

## What an error carries

libxml2 gives an error number, a line, a node name and an English sentence; anything else (the elements that were
expected, the kind of content-model error, the column) has to be recovered from the sentence. goxsdgo gives it in
the fields of `xsd.Error`:

| Need | goxsdgo field |
|---|---|
| node name | `Name`, its namespace in `Namespace`, the element path in `Path` |
| value | `Value` |
| facet that was broken | `Facet` (for example `maxInclusive 12`) |
| missing element, unexpected element, wrong namespace | `Reason`: `missing`, `unexpected`, `wrong-namespace`; `Expected`, `ExpectedNamespace` |
| parent element | `Name` for `missing`; otherwise the last element of `Path` before the node |
| type of the value | `Type`, `TypeNamespace`; the built-in base in `Builtin` |
| position in the line | `Line`, `Column` (column in characters) |
| where a missing element should go | `InsertLine`, `InsertColumn` |

So a consumer can drop regular expressions over libxml2's sentences and heuristics that guess the column: translate
by `Code` (and `Reason` for `cvc-complex-type.2.4`), fill the text from the fields. `Message` is an English sentence
for logs and developers; its wording is not stable, the code and the fields are.

## Error codes: libxml2 → goxsdgo

The `compat` table (`xsd.LibXML2Code`, ADR 0009) maps goxsdgo codes to libxml2 numbers, for consumers that want to
switch the validator first and their texts later. It is transitional: code that recognises libxml2's English
sentences stops working either way, because goxsdgo writes different ones. Reversed, it gives for the libxml2
numbers below the goxsdgo codes that appear in their place:

| libxml2 | goxsdgo |
|---|---|
| 4 | `xml-no-root` |
| 5 | `xml-content-outside-root` |
| 9 | `wfc-legal-character`, `xml-char` |
| 26 | `wfc-entity-declared` |
| 32 | `xml-encoding-mismatch`, `xml-encoding-unsupported` |
| 41 | `xml-syntax` |
| 42 | `wfc-unique-att-spec` |
| 62 | `xml-cdata-end` |
| 64 | `xml-reserved-pi-target` |
| 68 | `xml-name` |
| 76 | `wfc-element-type-match` |
| 77 | `xml-entity-structure`, `xml-eof` |
| 200 | `nsc-no-prefix-undeclaring`, `nsc-reserved-prefixes` |
| 201 | `nsc-prefix-declared` |
| 202 | `ns-qname` |
| 203 | `nsc-attributes-unique` |
| 1824 | `cvc-datatype-valid.1.2.1`, `cvc-datatype-valid.1.2.2`, `cvc-elt.4.1`, `cvc-entity`, `cvc-id.1`, `cvc-id.2`, `cvc-notation` |
| 1826 | `cvc-datatype-valid.1.2.3` |
| 1827 | `cvc-type.3.1.1` |
| 1828 | `cvc-type.3.1.2` |
| 1830 | `cvc-length-valid` |
| 1831 | `cvc-minLength-valid` |
| 1832 | `cvc-maxLength-valid` |
| 1833 | `cvc-elt.5.1.1`, `cvc-minInclusive-valid` |
| 1834 | `cvc-maxInclusive-valid` |
| 1835 | `cvc-minExclusive-valid` |
| 1836 | `cvc-maxExclusive-valid` |
| 1837 | `cvc-totalDigits-valid` |
| 1838 | `cvc-fractionDigits-valid` |
| 1839 | `cvc-pattern-valid`, `schema-location-hint` |
| 1840 | `cvc-enumeration-valid` |
| 1841 | `cvc-complex-type.2.1` |
| 1842 | `cvc-complex-type.2.2` |
| 1843 | `cvc-complex-type.2.3` |
| 1845 | `cvc-elt.1` |
| 1846 | `cvc-elt.2` |
| 1847 | `cvc-elt.3.1` |
| 1848 | `cvc-elt.3.2.1` |
| 1849 | `cvc-elt.3.2.2` |
| 1851 | `cvc-elt.4.2` |
| 1852 | `cvc-elt.4.3` |
| 1857 | `cvc-elt.5.2.2.1`, `cvc-elt.5.2.2.2.1` |
| 1858 | `cvc-elt.5.2.2.2.2` |
| 1866 | `cvc-complex-type.3.2.1` |
| 1867 | `cvc-complex-type.3.2.2` |
| 1868 | `cvc-complex-type.4` |
| 1869 | `cvc-complex-type.5.1`, `cvc-complex-type.5.2` |
| 1871 | `cvc-complex-type.2.4` |
| 1874 | `cvc-attribute.4`, `cvc-au` |
| 1876 | `cvc-type.2` |
| 1877 | `cvc-identity-constraint.3`, `cvc-identity-constraint.4.1`, `cvc-identity-constraint.4.2.1`, `cvc-identity-constraint.4.2.2`, `cvc-identity-constraint.4.2.3`, `cvc-identity-constraint.4.3` |
| 1878 | `cvc-attribute.1` |

libxml2 numbers without a counterpart: goxsdgo does not report that kind of error separately, or the error is about
libxml2's internals. The nearest goxsdgo codes by meaning (assigned by hand, not measured):

| libxml2 | Nearest goxsdgo code |
|---|---|
| 6 | wfc-legal-character, xml-syntax |
| 7 | wfc-legal-character, xml-syntax |
| 8 | wfc-legal-character, xml-syntax |
| 10 | xml-eof |
| 14 | xml-eof |
| 33 | xml-syntax |
| 34 | xml-syntax, xml-eof |
| 43 | xml-syntax |
| 44 | xml-syntax |
| 45 | xml-eof |
| 47 | xml-eof |
| 63 | xml-eof |
| 72 | xml-syntax |
| 73 | xml-syntax |
| 74 | xml-eof |
| 75 | xml-syntax |
| 84 | xml-syntax |
| 85 | xml-entity-structure |
| 86 | xml-content-outside-root |
| 110 | limit-name-length (returned as the second result, `ErrLimit`) |
| 204 | nsc-no-prefix-undeclaring |
| 205 | ns-qname |
| 1801 | xml-no-root |
| 1802 | cvc-elt.1 |
| 1803 | cvc-elt.1 |
| 1804 | cvc-complex-type.2.4 (missing) or cvc-complex-type.4 |
| 1805 | cvc-complex-type.2.4 (unexpected) |
| 1806 | — (goxsdgo reports the missing type when compiling the schema: src-resolve) |
| 1807 | — (internal state of libxml2) |
| 1808 | cvc-elt.2, cvc-type.2 |
| 1809 | cvc-complex-type.2.1 |
| 1810 | cvc-complex-type.2.4 |
| 1811 | cvc-elt.5.* |
| 1812 | cvc-elt.3.1 |
| 1813 | cvc-complex-type.2.4 (unexpected) |
| 1814 | cvc-complex-type.3.2.1 |
| 1815 | cvc-complex-type.2.4, cvc-elt.1 |
| 1816 | — (schema: cos-nonambig at compile time) |
| 1817 | — (schema: compile error) |
| 1818 | — (internal error of libxml2) |
| 1819 | cvc-type.3.1.1, cvc-type.3.1.2 |
| 1820 | cvc-complex-type.3.2.1 |
| 1821 | value codes (cvc-datatype-valid.*, facets) for an attribute |
| 1822 | value codes |
| 1823 | facet codes (cvc-*-valid) |
| 1825 | cvc-datatype-valid.1.2.2 |
| 1829 | facet codes (cvc-*-valid) |
| 1844 | cvc-complex-type.2.4 |
| 1850 | cvc-elt.4.1 |
| 1853 | cvc-elt.5.1.1 |
| 1854 | cvc-elt.5.* |
| 1855 | cvc-elt.5.* |
| 1856 | cvc-elt.5.2.2.1 |
| 1859 | cvc-identity-constraint.* |
| 1860 | — |
| 1861 | value codes for an attribute |
| 1862 | — (schema) |
| 1863 | value codes for an attribute |
| 1864 | cvc-attribute.4 |
| 1865 | cvc-complex-type.3.2.1 |
| 1870 | — (no such situation in XSD 1.0) |
| 1872 | xml-no-root |
| 1873 | — (schema) |
| 1875 | — (schema) |
| 1879 | — |

Differences in verdicts between libxml2 and goxsdgo, case by case: [differences.md](differences.md) and
[differences-libxml2.md](differences-libxml2.md) (in Polish).
