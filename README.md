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

- **Follows the recommendation, measured**: 39,367 of the 39,439 W3C test-suite cases for XSD 1.0 pass; every one
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

Where it is weaker: knroy/go-xml passes 19 more W3C cases (goxsdgo follows the text of the recommendation where it
reads it differently from the suite, see [docs/differences.md](docs/differences.md)) and also implements XSD 1.1;
libxml2 needs less memory (5 MB of process memory against 25 MB); jacoelho/xsd is equally fast with a slightly
smaller footprint. goxsdgo is new and has a single consumer so far.

## Contents

- [Why goxsdgo](#why-goxsdgo) · [Comparison](#comparison) · [Status](#status) · [Documentation](#documentation) ·
  [How the comparison was measured](#how-the-comparison-was-measured) · [Goals](#goals) · [License](#license)

## Comparison

libxml2[^ref] is what most tools use today (xmllint, lxml, PHP, Nokogiri); in Go it needs cgo. The other three
are validators in pure Go, published in 2026.

| | goxsdgo | libxml2 2.15.4[^ref] | knroy/go-xml 2.3.1 | jacoelho/xsd 0.6.15 | cB-Sigma/xsd 1.0.0 |
|---|:---:|:---:|:---:|:---:|:---:|
| W3C XSD 1.0 test suite, all cases[^w3c] | 39,367 of 39,439 | 38,662 of 39,439[^libxml2run] | 39,386 of 39,439 🥇 | 39,083 of 39,439 | 39,329 of 39,439 |
| — schema tests | 14,370 of 14,384 | 13,851 of 14,307 comparable | 14,381 of 14,384 🥇 | 14,198 of 14,384 | 14,367 of 14,384 |
| — instance tests | 24,997 of 25,055 | 24,797 of 24,903 comparable | 25,005 of 25,055 🥇 | 24,885 of 25,055 | 24,962 of 25,055 |
| Same verdict as libxml2, instance tests | 99.69% | reference | not measured | not measured | not measured |
| MF schemas compile (JPK, KSeF, e-SF)[^mf] | 170 of 170 🥇 | 170 of 170 🥇 | 170 of 170 🥇 | 168 of 170 | 170 of 170 🥇 |
| MF and synthetic documents, expected verdict | 15 of 15 🥇 | 15 of 15 🥇 | 15 of 15 🥇 | 15 of 15 🥇 | 15 of 15 🥇 |
| Dependencies[^deps] | 0 🥇 | C library, cgo | 1 (`golang.org/x/text`) | 0 🥇 | 0 🥇 |
| Minimum Go | 1.22 🥇 | — | 1.25 | 1.27 | 1.24 |
| Lines of Go code[^size] | 15,119 | — (C) | 203,186 | 55,395 | 8,679 🥇 |
| Added to a program | 1,675 kB | 1,032 kB shared library + binding | 6,434 kB | 3,167 kB | 1,574 kB 🥇 |
| Time, 20 MB JPK[^perf] | 0.39 s 🥇 | 0.39 s 🥇 | 0.57 s | 0.39 s 🥇 | 3.62 s |
| Time, 200 MB JPK | 3.96 s | 3.85 s | not run (tree) | 3.79 s 🥇[^jlimit] | not run (tree) |
| Process memory, 20 MB | 24 MB | 5 MB 🥇 | 219 MB | 21 MB | 442 MB |
| Process memory, 200 MB | 26 MB | 5 MB 🥇 | — | 22 MB | — |
| Streaming validation | yes 🥇 | yes 🥇 | no, builds a tree (11× the file) | yes 🥇 | no, reads the whole file (22×) |
| 1 GB document | 6.3 MB heap 🥇[^gb] | streaming | not run | needs a raised limit[^jlimit] | not run |
| Network access for `schemaLocation`, by default[^safe] | never 🥇 | no request seen | never 🥇 | never 🥇 | never 🥇 |
| External entity `file:`, by default | refused, no verdict 🥇 | read (xmllint) | DOCTYPE refused 🥇 | DTD refused 🥇 | not well-formed 🥇 |
| Billion laughs | entity limit, 10 ms 🥇 | amplification limit 🥇 | DOCTYPE refused 🥇 | DTD refused 🥇 | not well-formed 🥇 |
| 100,000 nested elements | depth limit 🥇 | depth limit 🥇 | depth limit 🥇 | depth limit 🥇 | depth limit 🥇 |
| `pattern="(a\|aa)*b"`, 40,000 characters | 1.5 ms 🥇 | internal error, no verdict | 1.3 ms 🥇 | 0.6 ms 🥇 | 1.4 ms 🥇 |
| `xs:redefine`[^feat] | yes 🥇 | yes 🥇 | yes 🥇 | no | yes 🥇 |
| `key`/`keyref`, `xsi:type`, substitution groups | yes 🥇 | yes 🥇 | yes 🥇 | yes 🥇 | yes 🥇 |
| `xsi:schemaLocation` hints | opt-in | yes | opt-in | no | opt-in |
| XSD 1.1 | no | no | yes 🥇 | no | no |
| Error code[^err] | rule name (`cvc-complex-type.2.4`) 🥇 | number (1871) | rule name 🥇 | own (`validation.content`) | category only |
| Line, column, path of an error | all three 🥇 | line, element name | all three 🥇 | all three 🥇 | line, path |
| Elements expected at the error | a field 🥇 | in the message | in the message | in the message | in the message |
| Messages in other languages | English and Polish 🥇 | English | English | English | English |
| License | Apache 2.0 | MIT | MIT | MIT | Apache 2.0 |
| GitHub popularity[^pop] | new | 771 stars (mirror) | 8 stars | 3 stars | 0 stars |

[^ref]: libxml2 is the reference validator throughout this project's tests (`tools/compare`, through
    [goxsdval](https://github.com/polestorm/goxsdval), and `xmllint`). It and the other validators are only run
    as programs, through their public API; goxsdgo is written from the recommendations, not from the source code of
    other validators ([docs/provenance.md](docs/provenance.md)).
[^w3c]: The W3C XML Schema test suite at commit `7bc3365c`, every schema and instance test that applies to
    XSD 1.0 Second Edition: 39,439 cases, read by `internal/w3c` and run by the same harness for every candidate,
    with the same expected verdicts and the same rules as `TestW3CSuite`. A case without a verdict (a library
    limit, an unsupported construct, a timeout of 30 s) counts as failed; so does an instance test whose valid
    schema was rejected. `xml.xsd` and `xlink.xsd`, which some cases import from w3.org, are served from local
    copies. Each candidate gets its own mechanism for `xsi:schemaLocation` hints where it has one; jacoelho/xsd has
    none, so 55 cases that name their schema only in the document cannot run on it. Self-reported figures, not
    measured here: knroy/go-xml 99.90% of instance and 99.98% of schema tests, cB-Sigma/xsd 99.6% of 39,369
    tests (their READMEs).
[^libxml2run]: Counted by `tools/compare` through goxsdval, which compiles a schema from one file and does not
    follow hints: 209 cases (154 with several schema documents, 55 with hints only) cannot run and count as failed.
[^mf]: The 170 schemas of `testdata/sources.json` with their imports, resolved from a local catalogue; the 8
    published MF samples and 7 synthetic documents of `testdata/instances/expected.json`. jacoelho/xsd stops at its
    content-model state limit on the two KSeF invoice schemas, FA(2) and FA(3).
[^deps]: Modules required by a program that validates a file. libxml2 is a C library reached through cgo.
[^size]: Lines of the non-test Go files of the packages that a "compile a schema, validate a file" program pulls
    in (`tools/bench/sizeprog`), without the program itself. "Added to a program" is the size of that program minus
    the same program without a validator (`CGO_ENABLED=0`, `-ldflags="-s -w"`, macOS arm64); for libxml2 the size
    of `libxml2.16.dylib`.
[^perf]: A synthetic JPK_V7M(3) file: the published MF sample with its `SprzedazWiersz` row repeated to 20, 60
    and 200 MB; compile the schema and validate the file in a fresh process, wall time and maximum resident set
    size (`/usr/bin/time -l`); libxml2 as `xmllint --stream`. Validators that build a tree of the document were not
    run on 200 MB; on 60 MB they used 613 MB (knroy/go-xml) and 1,324 MB (cB-Sigma/xsd). Measured on 2026-11-25 on
    one Apple M4 laptop shared with other work (load average 2–3). 🥇 marks the best value in a row.
[^jlimit]: jacoelho/xsd stops at 64 MiB of input by default (`MaxInstanceBytes`); the time is with the limit
    raised.
[^gb]: `TestStreamingMemory` with `GOXSDGO_STREAM_MB=1024`, peak Go heap ([docs/testing.md](docs/testing.md)).
[^safe]: Each library with its default settings on small inputs: a schema that imports from a local HTTP server
    (requests counted), a document with an external entity on a local file, nine levels of nested entities, 100,000
    nested elements, and the pattern `(a|aa)*b` against 20,000 and 40,000 `a`s (times grow linearly for the four Go
    libraries). libxml2 is `xmllint` with default options; a program using the library may set other options.
[^feat]: Small schemas and documents with a known verdict; "yes" means the verdict was right. For XSD 1.1, a
    schema with `xs:assert`: knroy/go-xml supports XSD 1.1 through an option.
[^err]: The same invalid document (a missing element and a wrong value) through each public API.
[^pop]: Stars on 2026-11-25; libxml2 is developed on GNOME's GitLab, the count is of its GitHub mirror.

## Status

Feature complete for XSD 1.0; the API may still change before 1.0. Requires Go 1.22 or newer. The replacement
criteria of [ADR 0001](docs/adr/0001-scope-and-acceptance.md), from the evidence package of 2026-11-25
([docs/evidence.md](docs/evidence.md)):

- K1 W3C test suite: 39,367 of 39,439; the 72 others explained. K2 MF corpus: 170 of 170 schemas, 15 of 15
  documents. K3 agreement with libxml2: 99.69% verdicts, 99.7% first-error lines, every difference described.
  K4 requirements: 136, all tested. K6 memory: 1 GB in 6.3 MB. K7 speed: 1.05× libxml2 on 200 MB, 1.19× on a
  private corpus of 1.6 GB. K8 consumer: xml.pro passes its tests with goxsdgo as its validator (branch, not yet
  deployed). K9 evidence package: built.
- K5 fuzzing: 24 hours on the release candidate in progress (15 minutes done).

## Documentation

- [`docs/errors.md`](docs/errors.md): every error code, its rule and its fields.
- [`docs/consumer-migration.md`](docs/consumer-migration.md): moving a libxml2-based consumer to goxsdgo.
- In Polish: [`docs/provenance.md`](docs/provenance.md) (sources and clean-room rules),
  [`docs/testing.md`](docs/testing.md) (tests, measurements), [`docs/differences.md`](docs/differences.md)
  (differences from the test suite and from libxml2), [`docs/differences-libxml2.md`](docs/differences-libxml2.md)
  (every case), [`docs/evidence.md`](docs/evidence.md) (evidence package), [`docs/fuzzing.md`](docs/fuzzing.md),
  [`requirements/`](requirements/) (numbered requirements), [`docs/adr/`](docs/adr/) (decisions),
  [`docs/roadmap.md`](docs/roadmap.md).

## How the comparison was measured

All five validators run on the same inputs through their public API, by `tools/bench` (a separate module, so the
library itself has no dependencies; libxml2 through `tools/compare` and `xmllint`):

```sh
go -C tools/fetch run . -root ../..                       # test data (W3C suite, MF schemas and samples)
go -C tools/bench run . -root ../.. w3c                   # W3C suite, every candidate
go -C tools/bench run . -root ../.. corpus                # MF corpus
go -C tools/bench run . -root ../.. probe                 # safety, features, errors, patterns
go -C tools/bench run . -root ../.. size                  # program size and lines of code
go -C tools/bench build -o ../../out/bench/bench . && (cd tools/bench && ../../out/bench/bench -root ../.. perf 20 60 200)
cd tools/compare && PKG_CONFIG_PATH=/opt/homebrew/opt/libxml2/lib/pkgconfig go run -tags libxml2 . w3c
```

The versions are the latest tags on 2026-11-25 (knroy/go-xml v2.3.1, jacoelho/xsd v0.6.15, cB-Sigma/xsd v1.0.0,
libxml2 2.15.4 from Homebrew); `tools/bench` builds with Go 1.27 because jacoelho/xsd requires it.

What the numbers do not say: the W3C rows count verdicts only, not error codes or positions. knroy/go-xml is also
an XSD 1.1, XPath, XQuery and XSLT processor, which explains most of its size; it is configured here for XSD 1.0
and allowed to read the DOCTYPE of suite documents (its default refuses it). A schema rejected by a candidate for a
limit or an unsupported construct counts as a failure, as it does for goxsdgo. The timing rows are single runs on
a shared machine; differences under 10% are within the noise.

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
