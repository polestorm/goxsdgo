# goxsdgo

Validates XML documents against XML Schema 1.0 (XSD) in pure Go: no cgo, no libxml2, no dependencies outside the
Go standard library. Written from the W3C recommendations. Documents are validated as a stream, in bounded memory,
and every error has a stable code named after the rule of the recommendation, with line, column and element path.

```go
import (
	"github.com/polestorm/goxsdgo/messages"
	"github.com/polestorm/goxsdgo/xsd"
)

set, err := xsd.Config{Locations: catalog}.Compile(os.DirFS("schemas"), "jpk_v7m_3.xsd") // imports resolved offline
errs, err := set.Validate(f)                                                              // streaming, bounded memory
for _, e := range errs {
	fmt.Println(e.Code, e.Line, e.Column, e.Path) // cvc-pattern-valid 12 21 /JPK/Podmiot1/OsobaNiefizyczna/NIP
	m := messages.Describe("pl", e, nil)          // title and hint in Polish (or "en")
	fmt.Println(m.Title, "—", m.Hint)
}
```

## Why goxsdgo

Compared with libxml2 and three other XML Schema validators in pure Go,
[knroy/go-xml](https://github.com/knroy/go-xml), [jacoelho/xsd](https://github.com/jacoelho/xsd) and
[cB-Sigma/xsd](https://github.com/cB-Sigma/xsd) (details in [Comparison](#comparison)):

- **Follows the recommendation, measured**: 39,369 of the 39,439 W3C test-suite cases for XSD 1.0 pass; every one
  of the other 72 is explained in [docs/differences.md](docs/differences.md). All 170 published schemas of the
  Polish Ministry of Finance (JPK, KSeF, e-Sprawozdania) compile, offline.
- **A drop-in for libxml2 where it matters**: on the W3C instance tests it gives libxml2's verdict in 99.69% of
  the cases and libxml2's line of the first error in 99.7%; every verdict difference (528) is categorised, and
  in 519 of them the test suite agrees with goxsdgo. On 151 real tax files (1.6 GB) the verdicts are the same in
  151 of 151.
- **Streaming**: a 1 GB document validates with a peak of 6.3 MB of heap; 200 MB take about as long as libxml2.
- **Errors made for people**: a stable code named after the rule (`cvc-complex-type.2.4`), line, column, element
  path, the elements expected at that point, the type and its built-in base, where a missing element should go,
  and messages in English and Polish (`messages`), with a table of libxml2 codes for consumers in transition.
- **Safe by default, linear time**: no network, no external entities, limits on depth, size and entity
  expansion; `pattern` facets and content models with large counters run in linear time; fuzzed.

Where it is weaker: knroy/go-xml passes 17 more W3C cases (goxsdgo follows the text of the recommendation where it
reads it differently from the suite, see [docs/differences.md](docs/differences.md)) and also implements XSD 1.1;
libxml2 needs less memory (5 MB of process memory against 25 MB); jacoelho/xsd is equally fast with a slightly
smaller footprint. goxsdgo is new and has a single consumer so far.

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
| Time, 200 MB JPK[^perf] | 3.96 s | 3.85 s | —[^tree] | 3.79 s 🥇 | —[^tree] |
| Memory, 200 MB JPK | 26 MB | 5 MB 🥇 | — | 22 MB | — |
| `(a\|aa)*b`, 40,000 chars | 1.5 ms | no verdict | 1.3 ms | 0.6 ms | 1.4 ms |
| `xs:redefine` | yes | yes | yes | no | yes |
| `xsi:schemaLocation` hints | opt-in | yes | opt-in | no | opt-in |
| XSD 1.1 | no | no | yes 🥇 | no | no |
| Error code | W3C rule 🥇 | number | W3C rule 🥇 | own | category |
| Line, column, path of an error | all | line, name | all | all | line, path |
| Expected elements at an error | a field 🥇 | in text | in text | in text | in text |
| Message languages | EN, PL 🥇 | EN | EN | EN | EN |
| License | Apache 2.0 | MIT | MIT | MIT | Apache 2.0 |

🥇 marks the best value in a row, when no more than two candidates share it. The full comparison (schema and
instance tests, the MF corpus, safety probes, 20 MB and 1 GB documents, every footnote and how it was measured):
[docs/comparison.md](docs/comparison.md); the contested cases, each with its reason:
[docs/contested-cases.md](docs/contested-cases.md).

[^w3c]: The W3C XML Schema test suite, all 39,439 cases that apply to XSD 1.0 Second Edition, run by one harness
    with the same expected verdicts for every candidate.
[^contested]: 133 cases chosen in advance by fixed criteria, the same for every candidate: queried or disputed in
    the suite's own metadata, dependent on the XML edition of name characters, or with opposite expectations in the
    suite for the same situation. In those cases goxsdgo follows the text of the recommendation; each one is listed
    in [docs/contested-cases.md](https://github.com/polestorm/goxsdgo/blob/main/docs/contested-cases.md).
[^perf]: A synthetic JPK_V7M(3) file; schema compiled and file validated in a fresh process; wall time and
    process memory; one Apple M4 laptop shared with other work, so differences under about 10% are noise.
[^tree]: Builds a tree of the whole document (11× and 22× the file size in memory), so not run on 200 MB.

## Status

Feature complete for XSD 1.0; the API may still change before 1.0. Requires Go 1.22 or newer. The replacement
criteria of [ADR 0001](docs/adr/0001-scope-and-acceptance.md), from the evidence package of 2026-11-25
([docs/evidence.md](docs/evidence.md)):

- K1 W3C test suite: 39,369 of 39,439; the 70 others explained. K2 MF corpus: 170 of 170 schemas, 15 of 15
  documents. K3 agreement with libxml2: 99.69% verdicts, 99.7% first-error lines, every difference described.
  K4 requirements: 136, all tested. K6 memory: 1 GB in 6.3 MB. K7 speed: 1.05× libxml2 on 200 MB, 1.19× on a
  private corpus of 1.6 GB. K8 consumer: xml.pro passes its tests with goxsdgo as its validator (branch, not yet
  deployed). K9 evidence package: built.
- K5 fuzzing: 24 hours on the release candidate in progress (15 minutes done).

## Documentation

- [`docs/comparison.md`](docs/comparison.md): the full comparison with other validators.
- [`docs/errors.md`](docs/errors.md): every error code, its rule and its fields.
- [`docs/consumer-migration.md`](docs/consumer-migration.md): moving a libxml2-based consumer to goxsdgo.
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
