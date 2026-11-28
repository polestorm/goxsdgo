# Raw results

Output of the last published comparison run, copied from `out/bench` and `out/compare` by
`go -C tools/bench run . -root ../.. publish`. What each file holds, the inputs and the commands to produce it
again: [../methodology.md](../methodology.md). Local paths are replaced with `<root>` and `<tmp>`.

`goxmlgo-switch/` holds the measurements of the switch to the goxmlgo reader in v0.5.0 (before and after, same
method); `goxmlgo-v0.5.0/` those of the move from goxmlgo v0.2.0 to v0.5.0. The comparison of the XML reader with `encoding/xml` is in goxmlgo since v0.5.0
([../methodology.md](../methodology.md#xml-reader)).
