# xmlreader (deprecated)

The XML reader goxsdgo validates with has moved to its own module, **[goxmlgo](https://github.com/polestorm/goxmlgo)**,
package [`github.com/polestorm/goxmlgo/xmlreader`](https://pkg.go.dev/github.com/polestorm/goxmlgo/xmlreader).
Use that import path in new code.

This package keeps programs written for goxsdgo v0.2.0 to v0.4.1 compiling unchanged: every type is an alias of the
type in goxmlgo, and the constants and functions of v0.3.0 forward to it. Values pass freely between the two import
paths (an error matches `*xmlreader.Error` of either one in `errors.As`), and what goxmlgo added (`Skip`, `Reset`,
`Limits.MaxElements`, `Token.Namespaces`, `NamespaceScope`, `TokenReader`, `Declaration`) is reached through the
aliased types. The package will be removed no earlier than goxsdgo 1.0.

The reader's documentation, requirements, W3C XML Conformance results, fuzzing and the comparison with
`encoding/xml` are in goxmlgo: [README](https://github.com/polestorm/goxmlgo#readme),
[error codes](https://github.com/polestorm/goxmlgo/blob/v0.2.0/docs/errors.md),
[differences from goxsdgo's reader](https://github.com/polestorm/goxmlgo/blob/v0.2.0/docs/differences.md).
