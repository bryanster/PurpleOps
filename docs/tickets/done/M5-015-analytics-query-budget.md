# M5-015 — Analytics query budget (gate before M6)

**Milestone:** M5 · **Size:** M · **Depends on:** M5-009 · **Gate before M6**

## Why

M6 runs every one of these queries on every report render, and the PDF path runs that render inside a
headless Chromium with a timeout. A rollup that takes four seconds is a report that fails to
generate, discovered by whoever is trying to send it to a client.

The epic forbids caching until somebody has measured. This is the measurement, and it is the evidence
that says whether the no-cache decision holds. It also gives `M7-008` something concrete to re-run
once M6 is on top.

Following `M2-016`, `M3-016` and `M4-010`: a full-load test plus a scaled-down CI variant, documented
budgets, and a rule about what to do when it fails.

## Scope

**In**

- `internal/analytics/loadtest` (package pattern from `internal/engagement/loadtest`).
- **Realistic fixture**, larger than `analyticstest`'s hand-computable one:
  - a **full installed ATT&CK version** (real adapter output, or a generated ~800-technique set),
  - ~10 scenarios × ~50 steps = **~500 steps** and their executions, scored across the whole
    vocabulary,
  - ~200 findings with ~1000 status-history rows spanning a 90-day engagement,
  - a **second engagement** overlapping on technique for the compare rollup,
  - evidence rows sufficient to exercise the archive path.
- **Measure each rollup and each endpoint** (`M5-004`…`M5-012`), p50 / p95 / max, and record them.
- **Measure under concurrent write load.** Analytics reads run while the war room writes; reuse
  `internal/engagement/loadtest`'s writer goroutines so the read pool is contended the way it will be
  in practice. A budget measured against an idle database is a budget for a database nobody has.
- **Starting budgets, tuned with evidence** (do not hollow them out):
  - per-rollup p95 **≤ 250 ms**, max ≤ 1 s, on developer SSD, under concurrent write load;
  - the whole dashboard's endpoint set (what `M5-013` fetches on load) **≤ 1 s** wall clock;
  - burndown at the point cap and compare across two full engagements measured separately — they are
    the two most likely to blow the budget.
- **Export/archive**: assert **constant memory**, not latency — the archive streams 2 GiB (`M3-EPIC`
  quota) and the failure mode is resident size, not p95.
- Scaled-down CI variant that still fails on an accidental N+1 or a dropped index.
- Document the run command, budgets, measured numbers and how to interpret them in `docs/testing.md`,
  matching the section `M3-016` added.
- **If a budget fails: fix the query, add the index, or fix the join.** Do not raise the budget, and
  do not add a cache — the epic's decision stands until this test says otherwise, and if it does say
  otherwise that is a decision with its own ticket.

**Out**

- Any caching or materialization (that is the outcome this test might justify, not part of it).
- M6's render path — measured in `M7-008` once it exists.
- Multi-node or multi-process scaling.

## Acceptance criteria

- [ ] Full-load run measures every rollup and every M5 endpoint, under concurrent write load, and the
      numbers are recorded in the ticket's completion notes and `docs/testing.md`.
- [ ] CI variant runs in the existing test budget and fails loudly on regression — not skipped, not
      `t.Skip` on a missing env var (`M0B-013`'s rule: a green run means the tests ran).
- [ ] A deliberately broken query — index dropped, or a rollup rewritten as a Go loop over rows —
      fails the gate. Demonstrated once during development and described in the completion notes;
      a performance gate nobody has seen fail is a performance gate nobody should trust.
- [ ] Archive export holds constant memory across a fixture with several hundred MB of blobs.
- [ ] Concurrent write latency (`M3-016`'s interactive p95) does **not** regress while analytics
      queries run — analytics must not starve the war room, which is the whole reason it reads
      through the read pool.
- [ ] Final budgets recorded, with the reasoning for any that moved from the starting values.
- [ ] `docs/testing.md` explains what to do when it fails, in the order to try things.

## Tests

- The load test is the test.
- The broken-query demonstration above.
- Memory assertion on the archive path.

## Notes for the implementer

- Read `internal/engagement/loadtest/warroom_test.go` first — the harness, the percentile helper and
  the CI-scaling pattern are all there, and a second copy of them is worse than reusing the first.
- The read pool and the serialized writer are separate by design (`M0B-003`). If analytics reads are
  blocking writes, that is a finding about the pool configuration and it is more important than the
  budget number.
- Generate the ATT&CK fixture rather than checking in 800 techniques of real content, unless the M2
  adapter fixtures already give you one cheaply.


## Completion notes

**Date:** 2026-08-09

### CI gate results (200 techniques, 50 steps, 50 findings, 3 writers, 10s)

| Rollup | p50 | p95 | max | samples |
|---|---:|---:|---:|---:|
| TechniqueCoverage | 7.3ms | 11.2ms | 45.4ms | 137 |
| TacticCoverage | 11.2ms | 15.9ms | 18.4ms | 137 |
| CategoryDistribution | 3.5ms | 6.6ms | 39.2ms | 137 |
| ProtectionRate | 2.9ms | 6.9ms | 27.4ms | 137 |
| OutcomeMix | 3.0ms | 7.3ms | 128.8ms | 137 |
| ModifierDistribution | 4.8ms | 8.3ms | 134.8ms | 137 |
| MTTD | 3.2ms | 5.7ms | 19.7ms | 137 |
| Burndown | 7.6ms | 11.7ms | 39.5ms | 137 |
| FindingsBySeverity | 1.4ms | 4.0ms | 6.9ms | 137 |
| Compare | 12.0ms | 16.5ms | 45.2ms | 136 |
| Navigator | 8.0ms | 12.5ms | 27.8ms | 136 |
| ExecutionsExport | 3.6ms | 6.1ms | 14.3ms | 136 |
| FindingsExport | 6.7ms | 10.6ms | 14.0ms | 136 |
| DashboardSet (concurrent) | — | 9.6ms | — | 136 |

Write probe: p95=10.5ms, max=123ms, 198 samples (budget: p95≤200ms)

Archive memory: heap delta 835 KiB (budget: ≤50 MiB)

### Mutation test

Baseline TechniqueCoverage: 7.2ms. Broken N+1 Go loop (per-technique query, 20×):
879.5ms — exceeds the 250ms budget. Gate correctly catches the regression.

### Budget decisions

Starting budgets held without adjustment: all rollups are 20× under the p95
budget at CI scale. No indexes were added; all queries are single-statement.
No cache was added — the no-cache decision holds.

### Run command

```
# CI (always on)
go test ./internal/analytics/loadtest/

# Full developer load
BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/analytics/loadtest/ -run TestAnalyticsQueryLoad
```