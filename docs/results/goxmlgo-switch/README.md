# Switch to the goxmlgo reader: before and after

Measured 2026-11-27 on the machine in `environment-*.json` (Apple M4, 10 cores, macOS 27.0.1, Go 1.27.0, on AC
power). **Before**: goxsdgo v0.4.1 with its own reader (commit `741210b`, clean tree). **After**: the same code with the
reader from `github.com/polestorm/goxmlgo` v0.2.0 (ADR 0014; uncommitted tree on top of `741210b`). Method:
[../../methodology.md](../../methodology.md#switching-to-goxmlgo-validation-before-and-after).

## Results

| Check | Before (v0.4.1) | After (goxmlgo v0.2.0) |
|---|---|---|
| Errors of every in-scope W3C case and corpus document (`TestDumpErrors`, 56,522 lines), SHA-256 | `c1abb66c…3c00a56` | `c1abb66c…3c00a56` (identical) |
| W3C XSD suite (`TestW3CSuite`) | 39,369 / 39,439 | 39,369 / 39,439 |
| MF schemas / documents | 170 / 170, 15 / 15 | 170 / 170, 15 / 15 |
| Ratchets `testdata/**/*.ratchet` (XSD and datatypes) | unchanged | unchanged |
| `TestStreamingMemory`, `TestDeterministic` | pass | pass |
| Validate 20 MB JPK, median of 5 (runs 1; 2) | 0.47 s; 0.41 s | 0.37 s; 0.37 s |
| Validate 60 MB JPK | 1.22 s; 1.31 s | 1.18 s; 1.08 s |
| Validate 200 MB JPK | 4.15 s; 4.84 s | 3.55 s; 3.55 s |
| 200 MB, ratio to xmllint in the same run | 1.41; 1.45 | 1.25; 1.24 |
| Maximum resident set, 200 MB | 25.7 MB; 25.6 MB | 25.4 MB; 25.9 MB |
| `BenchmarkValidateJPK` (in-memory JPK, 21 MB), median of 5 | 444 ms, 25.9 MB/op, 945,890 allocs/op | 368 ms, 25.8 MB/op, 945,891 allocs/op |

The runs went in the order before 1, after 1, before 2, after 2. xmllint is the one in `PATH` (`/usr/bin/xmllint`,
libxml2 2.9.13, not the 2.15.4 of the published comparison): here it only shows how loaded the machine was, so
the ratio to it is less sensitive to load than the time alone. Validation is 13–17% faster with the goxmlgo reader (ratio to xmllint, `BenchmarkValidateJPK`)
(goxmlgo measures its reader alone at about 1.2× the reader of goxsdgo v0.3.0); memory is the same.

## Files

- `perf-before-1.json`, `perf-before-2.json`, `perf-after-1.json`, `perf-after-2.json`: every run (`bench perf`);
- `environment-before.json`, `environment-after.json`: machine, Go, commit, load, module versions
  (the environment of run 2 was the same machine minutes later);
- `validate-bench.txt`: `go test ./xsd -run '^$' -bench 'BenchmarkValidateJPK$' -benchmem -count 5`, before and after.
