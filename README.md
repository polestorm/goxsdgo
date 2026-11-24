# goxsdgo

Validates XML documents against XML Schema 1.0 (XSD) in pure Go: no cgo, no libxml2, no dependencies outside the
Go standard library. Written from the W3C recommendations.

Status: planning (stage 0). Until the acceptance criteria in `docs/adr/0001-scope-and-acceptance.md` are met,
consumers use [goxsdval](https://github.com/polestorm/goxsdval) (cgo over libxml2).

Planned API:

```go
import "github.com/polestorm/goxsdgo/xsd"

set, err := xsd.Compile(os.DirFS("schemas"), "jpk_v7m_3.xsd") // imports resolved offline
errs, err := set.Validate(f)                                  // streaming, bounded memory
for _, e := range errs {
	fmt.Println(e.Code, e.Line, e.Column, e.Path, e.Message)
}
```

Goals:

- **XSD 1.0 (Second Edition)**, Structures and Datatypes, measured on the W3C XML Schema test suite;
- **pure Go**: `CGO_ENABLED=0`, single static binary, cross-compiles;
- **streaming**: multi-hundred-megabyte documents validate in bounded memory;
- **safe by default**: no network access, no external entities, limits on depth and size;
- **errors made for people**: a stable code, line, column and element path for every error, so callers can
  translate messages;
- **deterministic and linear**: the same input gives the same errors in the same order; `pattern` facets run
  in linear time.

Not in scope: XSD 1.1 (assertions, conditional types), DTD validation, XSLT, XPath beyond what identity
constraints need, generating Go types from schemas.

Licence: Apache 2.0.
