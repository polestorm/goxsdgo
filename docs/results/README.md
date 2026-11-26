# Raw results

Output of the last published comparison run, copied from `out/bench` and `out/compare` by
`go -C tools/bench run . -root ../.. publish`. What each file holds, the inputs and the commands to produce it
again: [../methodology.md](../methodology.md). Local paths are replaced with `<root>` and `<tmp>`.

`xmlreader-conformance.tsv` and `xmlreader-bench.txt` come from the tests of `xmlreader` (comparison with
`encoding/xml`); their commands are in [../methodology.md](../methodology.md#xml-reader).
