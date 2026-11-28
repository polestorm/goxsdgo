# xmlreader (deprecated)

The XML reader goxsdgo validates with has moved to its own module, **[goxmlgo](https://github.com/polestorm/goxmlgo)**,
package [`github.com/polestorm/goxmlgo/xmlreader`](https://pkg.go.dev/github.com/polestorm/goxmlgo/xmlreader).
Use that import path in new code.

This package keeps programs written for goxsdgo v0.2.0 to v0.4.1 compiling unchanged: every type is an alias of the
type in goxmlgo, and the constants and functions of v0.3.0 forward to it. Values pass freely between the two import
paths (an error matches `*xmlreader.Error` of either one in `errors.As`), and what goxmlgo added (`Skip`, `Reset`,
`Limits.MaxElements`, `Limits.MaxNamespaces`, `Token.Namespaces`, `NamespaceScope`, `TokenReader`, `Declaration`,
`InScopeNamespaces`) is reached through the aliased types. `CodeLimitNamespaces` is here too, because the limit
`MaxNamespaces` is on by default since goxmlgo v0.4.0, so a program written for v0.3.0 can receive that code. The package will be removed no earlier than goxsdgo 1.0.

The reader's documentation, requirements, W3C XML Conformance results, fuzzing and the comparison with
`encoding/xml` are in goxmlgo: [README](https://github.com/polestorm/goxmlgo#readme),
[error codes](https://github.com/polestorm/goxmlgo/blob/v0.5.0/docs/errors.md),
[differences from goxsdgo's reader](https://github.com/polestorm/goxmlgo/blob/v0.5.0/docs/differences.md).
