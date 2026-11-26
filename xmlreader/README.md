# xmlreader

A streaming XML 1.0 reader with namespaces, in pure Go, written from the W3C recommendations (XML 1.0 Fifth
Edition, Namespaces in XML 1.0 Third Edition). It is the reader [goxsdgo](../README.md) validates with; it has no
dependencies outside the standard library and can be used on its own.

```go
import "github.com/polestorm/goxsdgo/xmlreader"

r := xmlreader.New(f, xmlreader.Options{}) // default limits, namespaces on
for {
	t, err := r.Next() // io.EOF at the end; *xmlreader.Error with Code, Line, Column
	if err != nil {
		break
	}
	if t.Kind == xmlreader.KindStartElement {
		fmt.Println(t.Line, t.Column, t.Name.Space, t.Name.Local, len(t.Attrs))
	}
}
```

The API is documented on [pkg.go.dev](https://pkg.go.dev/github.com/polestorm/goxsdgo/xmlreader). It may change
before goxsdgo 1.0.

## Compared with encoding/xml

| | xmlreader | encoding/xml |
|---|---|---|
| W3C XML Conformance, in-scope cases[^conf] | **1725 / 1727** | 1146 / 1727 |
| … malformed documents rejected | **951 / 951** | 434 / 951 |
| … well-formed documents read | **774 / 776** | 712 / 776 |
| Speed, 8.8 MB JPK-like document[^perf] | **207 MB/s** | 90 MB/s (`Token`), 99 MB/s (`RawToken`) |
| Memory allocated reading it | **149 KB, 67 allocations** | 46 MB, 1.26 M allocations (`Token`) |
| Position | **line and column of every token and error**; end of each start tag | `InputPos` after a token; line in error text |
| Errors | **fixed code** (`wfc-unique-att-spec`, …), line, column, message | message text only |
| Internal DTD subset | **internal entities, attribute defaults and types, notations** | returned as raw text |
| External entities, network | never read | never read |
| Limits (depth, names, attributes, text, entity expansion) | **on by default** | none built in |
| Encodings | UTF-8, UTF-16, ISO-8859-1, US-ASCII | UTF-8; others through `CharsetReader` |
| Tokens | not copied, valid until the next `Next` | new values for every token |
| Decoding into structs (`Unmarshal`), writing XML (`Encoder`) | no | **yes** |
| XML 1.1 | no | no |

Some malformed documents encoding/xml accepts:

| Document | xmlreader | encoding/xml |
|---|---|---|
| `<a x='1' x='2'/>` | `1:10: wfc-unique-att-spec` | accepted, both attributes returned |
| `<a/><b/>` | `1:5: xml-content-outside-root` | accepted, two root elements |
| `<a>]]></a>` | `1:4: xml-cdata-end` | rejected |
| `<a>&undefined;</a>` | `1:4: wfc-entity-declared` | rejected |

Use encoding/xml when you need `Unmarshal`, `Marshal` or a writer, and the input comes from a trusted source.
Use xmlreader when you need to know that a document is well-formed, where an error is, or when you read large or
untrusted input.

[^conf]: The W3C XML Conformance suite cases that apply to XML 1.0 Fifth Edition and Namespaces 1.0 without
    external entities; the same cases for both readers. A well-formed case passes when the document is read
    without an error (xmlreader must also give the canonical form, where the suite has one), a malformed case when
    it is rejected. Per-case results: [docs/results/xmlreader-conformance.tsv](../docs/results/xmlreader-conformance.tsv).
[^perf]: Median of 5 runs of `go test -bench` on an Apple M4, one goroutine; a deterministic JPK-like document of
    20,000 rows (`jpkLike` in `reader_test.go`). Raw output:
    [docs/results/xmlreader-bench.txt](../docs/results/xmlreader-bench.txt); method and commands:
    [docs/methodology.md](../docs/methodology.md#xml-reader).
