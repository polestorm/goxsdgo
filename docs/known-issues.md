# Known issues and limitations

Known anomalies and deliberate limits of goxsdgo, with an assessment of the risk for a consumer and a workaround.
Every release updates this page ([CHANGELOG.md](../CHANGELOG.md)); a closed issue keeps its entry with the version
that closed it. Differences case by case: [differences.md](differences.md) (Polish) and
[differences-libxml2.md](differences-libxml2.md).

Risk: **low** — no effect on the MF corpus or on typical configuration schemas; **medium** — a consumer can meet
it and has to know; **high** — can change a verdict without a visible sign. No high entry is open.

## Open

| # | Issue | Risk | Workaround |
|---|---|---|---|
| KI-1 | **Different verdicts from libxml2** in 530 W3C cases (schemas mostly; in 521 of them the test suite agrees with goxsdgo). A schema or a document accepted by libxml2 can be rejected, or the other way round. On the MF corpus (170 schemas, 15 documents) both give the same verdicts. | medium | Before switching from libxml2, run both on your own schemas and documents; categories and their effect: [differences.md](differences.md). |
| KI-2 | **70 W3C cases** in scope fail where goxsdgo follows the text of the recommendation and the suite reads it differently (names by XML 1.0 2e, missing components, a few disputed or queried cases). | low | `NameRules: NamesXML10Edition5` for names by XML 1.0 5e; the rest is explained per case in [differences.md](differences.md). |
| KI-3 | **Content models that need more than 1024 configurations** (counters inside ambiguous nested repetitions) stop with `ErrLimit` instead of a verdict (4 W3C cases). | low | `ErrLimit` is never a verdict: treat it as "not validated". Such models are not in the MF schemas. |
| KI-4 | **24 h of fuzzing (K5) not yet complete for the current code.** The finished campaign (17.1 h, no panic) tested commit `d8c4e60`, before v0.3.0, v0.4.0 and v0.4.1 changed library code; a 24 h campaign on v0.4.1 is queued on the Synology. Short fuzz runs on the new code found nothing. | medium | Status in [fuzzing.md](fuzzing.md); a campaign on the release candidate follows. |
| KI-5 | **Compilation time grows a little faster than linearly with very large schemas** (a sequence of thousands of elements, many wildcards: about n^1.3–1.5 at 4n). Validation time stays linear in the size of the document. | low | Not visible on real schemas (MF schemas compile in milliseconds); compile once and reuse the `Set`. |
| KI-6 | **External entities and the external DTD subset are never read**; a document that needs them is rejected (`unsupported-*`). Conditional sections in parameter entities are not supported. | low | By design (safety). Expand such documents before validation. |
| KI-7 | **Encodings**: UTF-8, UTF-16, ISO-8859-1 and US-ASCII only; other encodings give `xml-encoding-unsupported`. | low | Convert to UTF-8 before validation. |
| KI-8 | **`xml:id` is an ID only when the DTD declares it** (`xmlreader.Attr.Type`); the xml:id recommendation is not applied. For XSD validation `xml:id` follows the schema, as the recommendation requires. | low | Declare it, or treat `xml:id` as an ID in the consumer. |
| KI-9 | **Schemas downloaded through `xsd/resolve` with `Network` on depend on the server**: the same program can compile a different schema when the server changes a file. | medium | Keep the cache (or a `Dir`) under version control; check `Set.Documents()` hashes; turn `Network` off in production. |
| KI-10 | **Not in scope**: XSD 1.1, DTD validation, XSLT, XPath beyond identity constraints, generating Go types. | — | Another tool. |
| KI-11 | **The API may change before 1.0** (open question 5: a read-only schema model is planned). | low | Pin the version; every change has an impact class in the changelog. |
| KI-12 | **`time` with a timezone across midnight**: the canonical form of `20:00:00-07:00` is `03:00:00Z` (Part 2, 3.2.8.2: UTC, the day dropped), but by the order of 3.2.8 (a reference date) the two values differ by one day. The recommendation itself is inconsistent here; goxsdgo applies both rules as written. So `OnValue` reports `03:00:00Z`, and an enumeration `03:00:00Z` does not accept `20:00:00-07:00`. | low | Write `time` values and enumerations in UTC. Found by fuzzing (docs/fuzzing.md). |

## Closed

| # | Issue | Affected | Fixed in |
|---|---|---|---|
| KI-13 | **Input in small reads**: the XML declaration read in pieces (`iotest.OneByteReader`, a small first network packet) panicked in the reader ("slice bounds out of range"), so also in `xsd.Set.Validate`, or gave a false `xml-decl`; `<?xml version="1.0" ?><a/>` was rejected with `xml-eof` even in one read; the error code for a disallowed ASCII character right after a name depended on where a read ended. Workaround for affected versions: read the first bytes in one piece, `head := make([]byte, 512); n, _ := io.ReadFull(r, head); r = io.MultiReader(bytes.NewReader(head[:n]), r)` (`bufio.Reader` does not help: it passes large reads straight to the source); a document of only a declaration with white space before `?>` and a short root element stays rejected. | v0.1.0 – v0.4.0 | v0.4.1; [CHANGELOG.md](../CHANGELOG.md) |
