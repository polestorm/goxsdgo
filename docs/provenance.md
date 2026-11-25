# Code provenance

`goxsdgo` is written from scratch, from the W3C recommendations. This file says what we may use and what we may not.

## Sources we write from

- W3C XML Schema Part 1: Structures Second Edition and Part 2: Datatypes Second Edition (2004-10-28), with errata;
- Extensible Markup Language (XML) 1.0 Fifth Edition and Namespaces in XML 1.0 Third Edition;
- the standards the recommendations refer to: Unicode (character categories and blocks for regular expressions),
  ISO 8601 as far as Part 2 describes it, RFC 3986 (`xs:anyURI`), IEEE 754 (`xs:float`, `xs:double`).

The recommendations state the validation rules and the schema component constraints. They are part of the
specification and may be applied directly, including the algorithms given in the text (for example the check that a
particle is a valid restriction).

General knowledge about automata (building an automaton from an expression, automata with counters), as described in
scientific publications, may be used, with the source cited in a comment.

## What we do not do

We do not read, copy or port the code of XSD validators or of validating XML parsers: libxml2 (including
`xmlschemas.c` and `xmlregexp.c`), Xerces (C and Java), Saxon, xmlschema (Python), lxml, go-libxml2, helium,
xsd-validator, or their ports. This also covers [goxsdval](https://github.com/polestorm/goxsdval) beyond its public
API: it wraps libxml2, so it may be called, but its `*_libxml2.go` files are not ported.

The list is not closed: it also covers newer XSD validators, including those written in Go (`knroy/go-xml`,
`jacoelho/xsd`, `cB-Sigma/xsd`, `moolekkari/validatexml-go`), and local copies of libxml2 wrappers.

The reason is not the licence (libxml2 is MIT, Xerces Apache-2.0) but three goals: code that is entirely ours and
easy to show to be so, an evidence package that ties the code to the rules of the recommendation, and the
recommendation, not someone else's implementation, as the reference. The rule holds while the library is being
built. Once criteria K1 and K2 are met, competitors' code under a permissive licence may be read under the rules of
[ADR 0002](adr/0002-competitor-code-review.md): separate roles, a review log, no copying.

## What we may do

- run libxml2 (through `goxsdval` or `xmllint`) and other validators as black boxes and compare verdicts and error
  locations (`tools/compare`, `tools/bench`; the comparison is in [comparison.md](comparison.md));
- read the user documentation of validators (descriptions of options, lists of known deviations);
- read `encoding/xml` and `regexp` from the Go standard library (BSD, the same licence as the rest of a program's
  dependencies); the library uses them only through their public API.

## Test data

The W3C test suite, the MF schemas and the samples are test data, not code. Sources, licences and checksums:
[`../testdata/README.md`](../testdata/README.md).
