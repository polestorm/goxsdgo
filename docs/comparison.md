# Comparison with other XSD validators

The full comparison behind the short table in the [README](../README.md#comparison).

libxml2[^ref] is what most tools use today (xmllint, lxml, PHP, Nokogiri); in Go it needs cgo. The other three
are validators in pure Go, published in 2026.

**Correctness**

| | goxsdgo | libxml2[^ref] | knroy[^names] | jacoelho[^names] | cB-Sigma[^names] |
|---|:---:|:---:|:---:|:---:|:---:|
| W3C XSD 1.0, all cases[^w3c] | 99.82%[^names5] | 98.03%[^libxml2run] | 99.87% 🥇 | 99.10% | 99.72% |
| — schema tests | 99.92% | 96.81% | 99.98% 🥇 | 98.71% | 99.88% |
| — instance tests | 99.77% | 99.57% | 99.80% 🥇 | 99.32% | 99.63% |
| W3C, without 133 contested cases[^contested] | 99.97% 🥇 | 98.18% | 99.94% | 99.26% | 99.88% |
| Same verdict as libxml2 (instances) | 99.69% | reference | — | — | — |
| MF schemas compile[^mf] | all 170 🥇 | all 170 🥇 | all 170 🥇 | 168 | all 170 🥇 |
| MF and synthetic documents | all 15 🥇 | all 15 🥇 | all 15 🥇 | all 15 🥇 | all 15 🥇 |

**Size and speed**

| | goxsdgo | libxml2[^ref] | knroy[^names] | jacoelho[^names] | cB-Sigma[^names] |
|---|:---:|:---:|:---:|:---:|:---:|
| Dependencies[^deps] | 0 🥇 | C, cgo | 1 | 0 🥇 | 0 🥇 |
| Minimum Go | 1.22 🥇 | — | 1.25 | 1.27 | 1.24 |
| Lines of Go code[^size] | 15,293 | (C) | 203,186 | 55,395 | 8,679 🥇 |
| Added to a program | 1.7 MB | 1.0 MB[^dylib] | 6.4 MB | 3.2 MB | 1.6 MB 🥇 |
| Time, 20 MB XML[^perf] | 0.41 s | 0.40 s | 0.59 s | 0.38 s 🥇 | 3.67 s |
| Time, 200 MB XML | 4.00 s | 3.90 s | —[^tree] | 3.75 s 🥇[^jlimit] | —[^tree] |
| Memory, 20 MB XML | 24 MB | 5 MB 🥇 | 219 MB | 21 MB | 433 MB |
| Memory, 200 MB XML | 25 MB | 5 MB 🥇 | — | 22 MB | — |
| Streaming | yes 🥇 | yes 🥇 | no, 11×[^tree] | yes 🥇 | no, 22×[^tree] |
| 1 GB document | 6.3 MB[^gb] 🥇 | streams | — | limit[^jlimit] | — |

**Safety, features, errors**

| | goxsdgo | libxml2[^ref] | knroy[^names] | jacoelho[^names] | cB-Sigma[^names] |
|---|:---:|:---:|:---:|:---:|:---:|
| Network for `schemaLocation`[^safe] | never 🥇 | none seen | never 🥇 | never 🥇 | never 🥇 |
| External entity `file:` | refused 🥇 | read[^xmllint] | refused 🥇 | refused 🥇 | refused 🥇 |
| Billion laughs | limit 🥇 | limit 🥇 | refused 🥇 | refused 🥇 | refused 🥇 |
| 100,000 nested elements | limit 🥇 | limit 🥇 | limit 🥇 | limit 🥇 | limit 🥇 |
| `(a\|aa)*b`, 40,000 chars | 1.5 ms 🥇 | no verdict | 1.3 ms 🥇 | 0.6 ms 🥇 | 1.4 ms 🥇 |
| `xs:redefine`[^feat] | yes 🥇 | yes 🥇 | yes 🥇 | no | yes 🥇 |
| `key`, `xsi:type`, subst. groups | yes 🥇 | yes 🥇 | yes 🥇 | yes 🥇 | yes 🥇 |
| `xsi:schemaLocation` hints | opt-in | yes | opt-in | no | opt-in |
| XSD 1.1 | no | no | yes 🥇 | no | no |
| Error code[^err] | W3C rule 🥇 | number | W3C rule 🥇 | own | category |
| Line, column, path | all 🥇 | line, name | all 🥇 | all 🥇 | line, path |
| Error details[^details] | fields 🥇 | in text | in text | in text | in text |
| License | Apache 2.0 | MIT | MIT | MIT | Apache 2.0 |
| GitHub stars[^pop] | new | 771 (mirror) | 8 | 3 | 0 |

[^names]: knroy is [knroy/go-xml](https://github.com/knroy/go-xml) 2.3.1, jacoelho is
    [jacoelho/xsd](https://github.com/jacoelho/xsd) 0.6.15, cB-Sigma is [cB-Sigma/xsd](https://github.com/cB-Sigma/xsd)
    1.0.0; libxml2 is 2.15.4.
[^ref]: libxml2 is the reference validator throughout this project's tests (`tools/compare`, through
    [goxsdval](https://github.com/polestorm/goxsdval), and `xmllint`). It and the other validators are only run
    as programs, through their public API; goxsdgo is written from the recommendations, not from the source code of
    other validators ([docs/provenance.md](provenance.md)).
[^w3c]: The W3C XML Schema test suite at commit `7bc3365c`, every schema and instance test that applies to
    XSD 1.0 Second Edition: 39,439 cases, read by `internal/w3c` and run by the same harness for every candidate,
    with the same expected verdicts and the same rules as `TestW3CSuite`. Passed cases, all / schema / instance:
    goxsdgo 39,369 / 14,372 / 24,997; libxml2 38,662 / 13,851 of 14,307 / 24,797 of 24,903 comparable; knroy
    39,386 / 14,381 / 25,005; jacoelho 39,083 / 14,198 / 24,885; cB-Sigma 39,329 / 14,367 / 24,962 (of 39,439 /
    14,384 / 25,055). Without the contested cases (of 39,306): goxsdgo 39,295, libxml2 38,590, knroy 39,281,
    jacoelho 39,015, cB-Sigma 39,257. A case without a verdict (a library
    limit, an unsupported construct, a timeout of 30 s) counts as failed; so does an instance test whose valid
    schema was rejected. `xml.xsd` and `xlink.xsd`, which some cases import from w3.org, are served from local
    copies. Each candidate gets its own mechanism for `xsi:schemaLocation` hints where it has one; jacoelho/xsd has
    none, so 55 cases that name their schema only in the document cannot run on it. Self-reported figures, not
    measured here: knroy/go-xml 99.90% of instance and 99.98% of schema tests, cB-Sigma/xsd 99.6% of 39,369
    tests (their READMEs).
[^details]: What the validator knows about an error beyond its position. goxsdgo gives it as separate fields of
    `xsd.Error`, so a program can use it without parsing the message: `Expected` (the elements allowed at that
    point), `Reason` (missing, unexpected or in the wrong namespace), `Value` and `Facet` (the value and the facet
    it broke), `Type` and `Builtin` (the type and its built-in base), `InsertLine` and `InsertColumn` (where a
    missing element should go). In the other validators the expected elements appear only in the text of the
    message.
[^names5]: With `Config.NameRules: xsd.NamesXML10Edition5` (name characters of XML 1.0 Fifth Edition instead of
    the Second Edition that XSD 1.0 2e refers to; ADR 0012) goxsdgo passes 39,385 of 39,439; the default stays the
    Second Edition.
[^contested]: Chosen in advance by fixed criteria, the same for every candidate, before looking at who passes them:
    cases marked queried or disputed in the suite's own metadata (63), cases whose outcome depends on the XML edition
    of name characters (53), and cases where the suite expects opposite verdicts for the same situation (17, both
    sides); Unicode-version cases are all already marked queried. Every case, its reason and each candidate's result
    are listed in [docs/contested-cases.md](contested-cases.md). On the contested cases themselves goxsdgo passes
    74 of 133 (90 with Fifth Edition names), knroy/go-xml 105, libxml2 72, cB-Sigma/xsd 72, jacoelho/xsd 68.
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
    and 200 MB; compile the schema and validate the file in a fresh process; median of 5 interleaved runs (the
    spread of every row was under 5%), wall time and maximum resident set size; libxml2 2.15.4 as
    `xmllint --stream`. Validators that build a tree of the document were not run on 200 MB; on 60 MB they used
    612 MB (knroy/go-xml) and 1,334 MB (cB-Sigma/xsd). Measured on 2026-11-26 on one Apple M4 laptop (macOS 27.0.1,
    load average about 2.5); every run and the machine are in [results/perf.json](results/perf.json) and
    [results/environment-perf.json](results/environment-perf.json). 🥇 marks the best value in a row.
[^tree]: knroy/go-xml and cB-Sigma/xsd build a tree of the whole document: process memory 11× and 22× the file
    size, so they were not run on 200 MB or 1 GB.
[^dylib]: Size of the shared library `libxml2.16.dylib`, plus the cgo binding.
[^xmllint]: `xmllint` with default options reads it; a program using the library may set other options. The
    other validators refuse the DOCTYPE, the DTD, or report the document as not well-formed.
[^jlimit]: jacoelho/xsd stops at 64 MiB of input by default (`MaxInstanceBytes`); the time is with the limit
    raised.
[^gb]: `TestStreamingMemory` with `GOXSDGO_STREAM_MB=1024`, peak Go heap ([docs/testing.md](testing.md)).
[^safe]: Each library with its default settings on small inputs: a schema that imports from a local HTTP server
    (requests counted), a document with an external entity on a local file, nine levels of nested entities, 100,000
    nested elements, and the pattern `(a|aa)*b` against 20,000 and 40,000 `a`s (times grow linearly for the four Go
    libraries). libxml2 is `xmllint` with default options; a program using the library may set other options.
[^feat]: Small schemas and documents with a known verdict; "yes" means the verdict was right. For XSD 1.1, a
    schema with `xs:assert`: knroy/go-xml supports XSD 1.1 through an option.
[^err]: The same invalid document (a missing element and a wrong value) through each public API.
[^pop]: Stars on 2026-11-25; libxml2 is developed on GNOME's GitLab, the count is of its GitHub mirror.

Where the suite's expectation is contested (queried or disputed in its own metadata, dependent on the XML edition
or the Unicode version, or contradictory within the suite), goxsdgo follows the text of the recommendation; each
such case is listed with the rule in [docs/contested-cases.md](contested-cases.md) and
[docs/differences.md](differences.md). The second W3C row compares all validators on the same uncontested
set. On all in-scope cases knroy/go-xml passes 17 more than goxsdgo; on the uncontested set goxsdgo passes 14 more
than knroy/go-xml. Of goxsdgo's 11 uncontested failures, 4 end with a library limit on counter-ambiguous content
models, 5 are instance tests of groups whose schema test is contested (the instance tests themselves are not, so they stay in),
and 2 (`attP031`, `anyURI_b006`) were considered and left in, because the suite deliberately splits their XSD 1.0
and 1.1 outcomes.

## How the comparison was measured

Inputs, the settings of each candidate, how each row is computed, the commands to run it again and the known
limits of the numbers: [methodology.md](methodology.md). The raw results of the published run (every candidate's
verdict for every W3C case, times of every run, the machine and versions): [results/](results/).
