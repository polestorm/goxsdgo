# Move to goxmlgo v0.5.0: before and after

Correctness measured 2026-11-27, speed 2026-11-28 12:28–12:33 (load average 2.0–3.2 on 10 cores, `load.txt`), on an
Apple M4 (macOS, on AC power). **Before**: goxsdgo v0.5.0 (`b210e7d`, clean
tree) with goxmlgo v0.2.0. **After**: the same code with goxmlgo v0.5.0, `go 1.26`, the new limit
`DocumentLimits.MaxNamespaces` and three new tests (ADR 0015; commit `72166c9`, measured as the uncommitted tree on top of `b210e7d`). A third state,
**bump only** (goxmlgo v0.5.0 with no other change), separates the reader's effect from goxsdgo's own changes.
Method: as for the move to goxmlgo v0.2.0 ([../../methodology.md](../../methodology.md#switching-to-goxmlgo-validation-before-and-after)).
Library tests ran with Go 1.26.6, the new minimum; `tools/bench` and the performance binaries with Go 1.27.0 (its
`go.mod` asks for it).

## Results

| Check | Before (goxmlgo v0.2.0) | Bump only (v0.5.0) | After (v0.5.0 and changes) |
|---|---|---|---|
| Errors of every in-scope W3C case and corpus document (`TestDumpErrors`, 56,522 lines), SHA-256 | `c1abb66c…3c00a56` | identical | identical |
| W3C XSD suite (`TestW3CSuite`) | 39,369 / 39,439 (schema 14,372 / 14,384, instance 24,997 / 25,055) | same, every per-set line identical | same, every per-set line identical |
| MF schemas / documents | 170 / 170, 15 / 15 | same | same |
| Ratchets `testdata/**/*.ratchet` (5 files), SHA-256 | — | identical | identical |
| Tests, all modules (pass / fail / skip) | 1,144 / 0 / 1 | 1,142 / 2 / 1 | 1,147 / 0 / 1 |
| `TestStreamingMemory`, `TestDeterministic` | pass | pass | pass |
| 20 / 60 / 200 MB JPK, goxsdgo median of 5 (runs 1; 2) | 0.352; 0.362 / 1.059; 1.070 / 3.565; 3.538 s | — | 0.374; 0.361 / 1.086; 1.071 / 3.594; 3.554 s |
| Same, ratio of goxsdgo to xmllint in the same run | 1.24; 1.24 / 1.25; 1.26 / 1.26; 1.25 | — | 1.26; 1.25 / 1.27; 1.26 / 1.28; 1.27 |
| Maximum resident set, 200 MB | 25 MB; 25 MB | — | 25 MB; 25 MB |
| `BenchmarkValidateJPK` (`-count 5`, twice), median of 10 | 368 ms | — | 364 ms |
| `BenchmarkValidateJPK`, 15 pairs run back to back, median of after / before | — | — | 0.99 (range 0.79–1.01) |

The two failures of the bump alone are the checks that tie the documentation to the dependency:
`TestRequirementsCovered` (the delegated requirements still named goxmlgo v0.2.0) and `TestCatalogMatchesSource`
(goxmlgo exports `limit-namespaces`, missing from the catalogue). Both are fixed in the "after" state.

**Speed: no measurable difference.** On a quiet machine xmllint took 2.80–2.83 s for the 200 MB file in all four
runs. goxsdgo with goxmlgo v0.5.0 is within 1% of goxmlgo v0.2.0: up to 1% slower in `bench perf` (whole
program, file read from disk), 1% faster in `BenchmarkValidateJPK` (library only, in memory); the difference between
two runs of the same binary is of the same size.

A first measurement on 2026-11-27 ran while other sessions loaded the machine (load average up to 13, xmllint
3.4–6.8 s for the same file); it gave the same conclusion with wider spread and is kept in `loaded-2026-11-27/`,
not used for the numbers above.

## Files

- `perf-before-1.json`, `perf-after-1.json`, `perf-before-2.json`, `perf-after-2.json`: every run of
  `bench perf` (`-only goxsdgo -runs 5 perf 20 60 200`; xmllint 2.9.13 from `PATH` ran alongside as the load
  reference), in the order before 1, after 1, before 2, after 2;
- `environment-*.json`: machine, load, time of each run. `goxsdgo_commit`/`goxsdgo_dirty` and `modules`
  describe the working tree the tool ran in; the "before" binary was built earlier from the clean `b210e7d`;
- `validate-bench.txt`: `BenchmarkValidateJPK` (`-benchmem -count 5`) after each `bench perf` run, then 15 pairs
  `-benchtime 3x -count 1`, before then after;
- `load.txt`: load average every 10 s during the speed runs;
- `loaded-2026-11-27/`: the same files from the first, loaded measurement.
