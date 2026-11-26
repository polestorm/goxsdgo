# goxsdgo

Validates XML documents against XML Schema 1.0 (XSD) in pure Go: no cgo, no libxml2, no dependencies outside the
Go standard library. Written from the W3C recommendations. Documents are validated as a stream, in bounded memory,
and every error has a stable code named after the rule of the recommendation, with line, column and element path.

```go
import "github.com/polestorm/goxsdgo/xsd"

set, err := xsd.Config{Locations: catalog}.Compile(os.DirFS("schemas"), "jpk_v7m_3.xsd") // imports resolved offline
errs, err := set.Validate(f)                                                              // streaming, bounded memory
for _, e := range errs {
	fmt.Println(e.Code, e.Line, e.Column, e.Path) // cvc-pattern-valid 12 21 /JPK/Podmiot1/OsobaNiefizyczna/NIP
	fmt.Println(e.Value, e.Facet, e.Message)      // the value, the facet it broke, an English sentence
}
```

## Why goxsdgo

Compared with libxml2 and three other XML Schema validators in pure Go,
[knroy/go-xml](https://github.com/knroy/go-xml), [jacoelho/xsd](https://github.com/jacoelho/xsd) and
[cB-Sigma/xsd](https://github.com/cB-Sigma/xsd) (details in [Comparison](#comparison)):

- **Follows the recommendation, measured**: 39,369 of the 39,439 W3C test-suite cases for XSD 1.0 pass; every one
  of the other 70 is explained in [docs/differences.md](docs/differences.md). All 170 published schemas of the
  Polish Ministry of Finance (JPK, KSeF, e-Sprawozdania) compile, offline.
- **A drop-in for libxml2 where it matters**: on the W3C instance tests it gives libxml2's verdict in 99.69% of
  the cases and libxml2's line of the first error in 99.7%; every verdict difference (530) is categorised, and
  in 521 of them the test suite agrees with goxsdgo.
- **Streaming**: a 1 GB document validates with a peak of 6.3 MB of heap; 200 MB take about as long as libxml2.
- **Errors made for people**: a stable code named after the rule (`cvc-complex-type.2.4`), line, column, element
  path, the elements expected at that point, the type and its built-in base, where a missing element should go,
  so a program can write its own messages in any language without parsing text; a table of libxml2 codes
  helps consumers in transition.
- **Safe by default, linear time**: no network, no external entities, limits on depth, size and entity
  expansion; `pattern` facets and content models with large counters run in linear time; fuzzed.

Where it is weaker: knroy/go-xml passes 17 more W3C cases (goxsdgo follows the text of the recommendation where it
reads it differently from the suite, see [docs/differences.md](docs/differences.md)) and also implements XSD 1.1;
libxml2 needs less memory (5 MB of process memory against 25 MB); jacoelho/xsd is slightly faster (3.75 s against
4.00 s on 200 MB) with a slightly smaller footprint. goxsdgo is new and has a single consumer so far.

## Contents

- [Why goxsdgo](#why-goxsdgo) · [Comparison](#comparison) · [Status](#status) · [Documentation](#documentation) ·
  [Goals](#goals) · [License](#license)

## Comparison

libxml2 2.15.4 is what most tools use today (xmllint, lxml, PHP, Nokogiri); in Go it needs cgo. The other three
are validators in pure Go published in 2026: [knroy/go-xml](https://github.com/knroy/go-xml) 2.3.1,
[jacoelho/xsd](https://github.com/jacoelho/xsd) 0.6.15 and [cB-Sigma/xsd](https://github.com/cB-Sigma/xsd) 1.0.0.
All of them were run as programs through their public API, on the same cases and data.

| | goxsdgo | libxml2 | knroy | jacoelho | cB-Sigma |
|---|:---:|:---:|:---:|:---:|:---:|
| W3C XSD 1.0 test suite[^w3c] | 99.82% | 98.03% | 99.87% 🥇 | 99.10% | 99.72% |
| — without contested cases[^contested] | 99.97% 🥇 | 98.18% | 99.94% | 99.26% | 99.88% |
| Dependencies | 0 | C, cgo | 1 | 0 | 0 |
| Added to a program | 1.7 MB | 1.0 MB shared library | 6.4 MB | 3.2 MB | 1.6 MB 🥇 |
| Time, 200 MB XML[^perf] | 4.00 s | 3.90 s | —[^tree] | 3.75 s 🥇 | —[^tree] |
| Memory, 200 MB XML | 25 MB | 5 MB 🥇 | — | 22 MB | — |
| XSD 1.1 | no | no | yes 🥇 | no | no |
| Error code | W3C rule 🥇 | number | W3C rule 🥇 | own | category |
| Line, column, path of an error | all | line, name | all | all | line, path |
| Error details[^details] | fields 🥇 | in text | in text | in text | in text |
| License | Apache 2.0 | MIT | MIT | MIT | Apache 2.0 |

🥇 marks the best value in a row, when no more than two candidates share it. The full comparison (schema and
instance tests, the MF corpus, safety probes, 20 MB and 1 GB documents, pathological patterns, `xs:redefine`,
`xsi:schemaLocation` hints, every footnote):
[docs/comparison.md](docs/comparison.md); the contested cases, each with its reason:
[docs/contested-cases.md](docs/contested-cases.md); inputs, settings and commands to repeat every measurement:
[docs/methodology.md](docs/methodology.md), raw results: [docs/results/](docs/results/).

[^w3c]: The W3C XML Schema test suite, all 39,439 cases that apply to XSD 1.0 Second Edition, run by one harness
    with the same expected verdicts for every candidate.
[^contested]: 133 cases without one right answer (queried or disputed by the working group, dependent on the XML
    edition, or with opposite expectations in the suite), chosen by fixed criteria, the same for every candidate:
    [docs/contested-cases.md](https://github.com/polestorm/goxsdgo/blob/main/docs/contested-cases.md).
[^details]: What the validator knows about an error beyond its position. goxsdgo gives it as separate fields of
    `xsd.Error`, so a program can use it without parsing the message: `Expected` (the elements allowed at that
    point), `Reason` (missing, unexpected or in the wrong namespace), `Value` and `Facet` (the value and the facet
    it broke), `Type` and `Builtin` (the type and its built-in base), `InsertLine` and `InsertColumn` (where a
    missing element should go). In the other validators the expected elements appear only in the text of the
    message.
[^perf]: A synthetic file in the JPK_V7M(3) format (a Polish VAT report), generated from a published sample;
    schema compiled and file validated in a fresh process; median of 5 runs, wall time and process memory; one
    Apple M4 laptop ([docs/methodology.md](docs/methodology.md)).
[^tree]: Builds a tree of the whole document (11× and 22× the file size in memory), so not run on 200 MB.

## Status

Feature complete for XSD 1.0; the API may still change before 1.0. Requires Go 1.22 or newer. The replacement
criteria of [ADR 0001](docs/adr/0001-scope-and-acceptance.md), from the evidence package of 2026-11-25
([docs/evidence.md](docs/evidence.md)):

- K1 W3C test suite: 39,369 of 39,439; the 70 others explained. K2 MF corpus: 170 of 170 schemas, 15 of 15
  documents. K3 agreement with libxml2: 99.69% verdicts, 99.7% first-error lines, every difference described.
  K4 requirements: 129, all tested. K6 memory: 1 GB in 6.3 MB. K7 speed: 1.05× libxml2 on the synthetic
  200 MB file (1.19× on a private corpus of 1.6 GB of client files, which cannot be published). K8 consumer:
  xml.pro passes its tests with goxsdgo as its validator (branch, not yet deployed). K9 evidence package: built.
- K5 fuzzing: 24 hours on the release candidate in progress (15 minutes done).

## Documentation

- [`docs/comparison.md`](docs/comparison.md): the full comparison with other validators.
- [`docs/contested-cases.md`](docs/contested-cases.md): the contested W3C cases, the criteria and every case.
- [`docs/methodology.md`](docs/methodology.md): inputs, settings and commands behind every number of the
  comparison; raw results in [`docs/results/`](docs/results/).
- [`docs/errors.md`](docs/errors.md): every error code, its rule and its fields.
- [`docs/migrating-from-libxml2.md`](docs/migrating-from-libxml2.md): moving from libxml2 to goxsdgo, error fields
  and codes.
- [`docs/provenance.md`](docs/provenance.md): sources and clean-room rules.
- In Polish: [`docs/testing.md`](docs/testing.md) (tests, measurements), [`docs/differences.md`](docs/differences.md)
  (differences from the test suite and from libxml2), [`docs/differences-libxml2.md`](docs/differences-libxml2.md)
  (every case), [`docs/evidence.md`](docs/evidence.md) (evidence package), [`docs/fuzzing.md`](docs/fuzzing.md),
  [`requirements/`](requirements/) (numbered requirements), [`docs/adr/`](docs/adr/) (decisions),
  [`docs/roadmap.md`](docs/roadmap.md).

## Goals

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

## License

Apache License 2.0, see `LICENSE` and `NOTICE`. goxsdgo contains no code by other authors. Test data by other
authors (the W3C test suites, published schemas of public bodies) is not part of the repository; it is fetched by
`tools/fetch` and described in `testdata/README.md`.
