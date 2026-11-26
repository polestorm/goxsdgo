# Changelog

Every release lists its changes with an impact class and the requirements they touch (IDs from
[`requirements/`](requirements/)). Impact classes:

| Class | Meaning | What a consumer should do |
|---|---|---|
| 0 | no change of behaviour (refactoring, documentation, tests) | nothing; regression run |
| 1 | change of behaviour within a requirement (a bug fix) | regression run; read the linked known issue |
| 2 | new or changed requirement (a new function, another criterion) | review the requirement and its tests |
| 3 | incompatible change (API, data format) | as 2, and follow the migration note |

Tags up to v0.3.0 are annotated, not signed; from v0.4.0 on, tags are signed. Known limitations:
[docs/known-issues.md](docs/known-issues.md).

## v0.4.1 (2026-11-26)

- **Class 1.** `xmlreader` (and so `xsd.Set.Validate`, which reads documents with it): input that arrives in small
  reads (`iotest.OneByteReader`, a small first network packet) made the XML declaration panic ("slice bounds out of
  range") or fail with a false `xml-decl`; a very short document with white space before `?>`, such as
  `<?xml version="1.0" ?><a/>`, was rejected with `xml-eof` even in one read; a disallowed ASCII character right
  after a name gave `xml-syntax` or `xml-char` depending on where a read ended. Affected: v0.1.0 to v0.4.0 (the
  reader was internal before v0.2.0). Known issue KI-13, closed. New requirement XML-019: the result does not depend
  on how the input is split into reads (`TestChunkedInput`, `TestXMLDeclSmallDocument`, `TestValidateChunkedInput`,
  and a new `FuzzReader` invariant). On the W3C XML Conformance suite read in one piece the token stream and errors
  are unchanged (1725 / 1727); the only change in the whole suite is the out-of-scope external entity
  `ibm/xml-1.1/valid/P03/ibm03v09.ent` (`xml-eof 1:20` becomes `xml-decl 1:21`). Validation errors on the W3C XSD
  suite and the MF corpus are byte for byte the same. Found while extracting the reader to goxmlgo.
- **Class 0.** Two fuzz-test invariants corrected after the Synology campaign (docs/fuzzing.md): `FuzzLexical`
  accepts the XSD 1.0 2e inconsistency for `time` across midnight (new known issue KI-12, behaviour unchanged),
  `FuzzRegexTranslate` allows 60 s instead of 2 s of wall time on slow or busy hardware. Library code unchanged.

## v0.4.0 (2026-11-26)

- **Class 2.** `Config.Resolve`, `SchemaRef`, `MaxResolvedSize`: schema documents the library cannot find in
  `Locations` or `fsys` are asked from a function the caller supplies, in the same loading pass, recursively.
  SCH-022; SCH-004 changed (documents come from `fsys` or `Config.Resolve`). Without `Resolve` nothing changes.
- **Class 2.** `Set.Documents()`: every schema document a set was compiled from, with its name, URL, source, size
  and SHA-256. SCH-023.
- **Class 2.** New package `xsd/resolve`: a resolver with a local directory, a cache directory and downloads only
  with `Options.Network`, from allowed hosts. SCH-024. The package `xsd` does not import it and has no network code.
- **Class 0.** Unused code removed and test literals escaped (staticcheck); no change of behaviour.
- **Class 0.** Validation dossier: this changelog, [docs/known-issues.md](docs/known-issues.md),
  [docs/design.md](docs/design.md), [docs/risk.md](docs/risk.md); release artifacts (test report, traceability,
  coverage, staticcheck, govulncheck, SBOM, environment) attached to every release by CI.

## v0.3.0 (2026-11-26)

- **Class 2.** `xmlreader.Attr.Type`: the attribute type declared in the internal DTD subset (`CDATA`, `ID`,
  `IDREF`, …, the [attribute type] values of the XML Information Set), also for attributes added from a default
  value; empty without a declaration. XML-018. Existing code is not affected.

## v0.2.0 (2026-11-26)

- **Class 2.** The XML reader is a public package, `xmlreader` (before: `internal/xmlreader`), with English API
  documentation and examples; the reader's behaviour is unchanged. XML-001 to XML-017. Comparison with
  `encoding/xml`: [xmlreader/README.md](xmlreader/README.md).

## v0.1.1 (2026-11-26)

- **Class 0.** Timing tests for linear growth are stable on CI machines (no garbage collection during the timed
  runs, up to three tries). CI comment on MF schemas updated. The library code is the same as in v0.1.0.

## v0.1.0 (2026-11-26)

- First public release: XSD 1.0 Second Edition, Structures and Datatypes. State of the acceptance criteria:
  [docs/evidence.md](docs/evidence.md).
