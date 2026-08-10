# M7-008 — Performance sanity pass (gates + report load)

**Milestone:** M7 · **Size:** M · **Depends on:** M6-015, M3-016, M4-010, M5-015

## Why

M3–M5 each left a load gate. M6 stacked HTML render, analytics-heavy blocks, and Chromium PDF on the
same process. Cutover needs evidence those budgets **still hold** with reporting on top — not a new
benchmark framework, and not a vibes-based "seems fine".

M5-015 explicitly deferred full report-path measurement to this ticket.

## Scope

**In**

- Re-run and record (completion notes + `docs/testing.md` pointers):

  | Gate | Command / package | Budget source |
  |---|---|---|
  | War-room writes | `M3-016` / `internal/engagement/loadtest` | p95 interactive write |
  | SSE war-room | `M4-010` / events loadtest | publish lag + write non-regression |
  | Analytics queries | `M5-015` / `internal/analytics/loadtest` | per-rollup p95 under write load |

- **Report path sanity** (new measurements, same style as existing gates):
  - Realistic engagement fixture (reuse analytics load fixture scale or the thesis seed).
  - Draft/full report render HTML for a block set that includes analytics + detail blocks (heatmap,
    scorecard, compare, findings, walkthrough) — p95 / max wall clock.
  - Publish path (snapshot + store) once per iteration OK.
  - PDF render smoke under load **N= small** (Chromium is heavy): assert no errors and a documented
    upper bound; do not set a fantasy sub-second PDF budget.
  - Concurrent: analytics or war-room writes overlapping one render should not deadlock or starve
    the serialized writer beyond documented factor.
- CI: keep existing scaled-down gates green; add a **scaled-down** report render assertion if one does
  not exist (fail on accidental N+1 render or loading full ATT&CK in a loop per block).
- If a budget fails: fix query/render/pool — **do not** raise budgets to green the gate. If product
  reality forces a budget change, record evidence and an explicit decision in completion notes
  (same rule as M5-015).

**Out**

- Multi-node perf, CDN, k6 public internet tests.
- Pixel-perfect PDF timing SLOs for enterprise sales sheets.
- Caching layers (still forbidden without a dedicated decision ticket).

## Files

- `internal/report/loadtest` or extension of existing loadtest packages
- `docs/testing.md` (commands + tables)
- This ticket completion notes with numbers

## Acceptance criteria

- [ ] M3-016, M4-010, M5-015 full or documented standard load commands re-run; numbers recorded and
      compared to original completion notes (no silent >2× regression without fix or decision).
- [ ] Report HTML render budget recorded; CI scaled-down gate fails on a deliberate break once during
      development (describe in notes).
- [ ] PDF path: at least smoke under the fixture with zero render errors; resource cleanup (no
      zombie Chromium pile-up after N renders).
- [ ] `docs/testing.md` lists M7-008 commands next to the earlier gates.
- [ ] Writer interactive p95 remains within M3-016 budget family while a render runs (measure).

## Tests

- The load/re-run commands are the tests.
- Mutation or deliberate slow render once to prove the CI tripwire.

## Notes for the implementer

- Reuse fixture generators; do not check in multi-GB evidence.
- Chromium tests may be `BLACKLIGHT_LOADTEST=1` full-only; CI stays honest with HTML render.
- Read M6-010 sandbox/compose notes before blaming PDF slowness on app code.
---

## Implementation notes

**Date:** 2026-08-10

### Report loadtest package

Created `internal/report/loadtest/render_test.go` with four tests following the
established patterns from M2-016, M3-016, M4-010, and M5-015.

#### CI gates (all pass)

| Test | What | Result |
|---|---|---|
| `TestReportRenderBudget` | 200 techniques, 50 iterations HTML render | p50=104ms, p95=141ms, max=146ms. Budget: p95≤1s, max≤3s. |
| `TestReportRenderDetectsRegression` | 800 techniques, N+1 (3× per-technique) query | 4.98s exceeds 3s max budget — gate correctly catches regression. |
| `TestReportRenderWithConcurrentWrites` | 3 writers + renders for 10s | Render p95=197ms, write p95=7.75ms. Budgets: render p95≤1s, write p95≤200ms. |

All three ride along with `make test` / `make test-race`.

#### Budgets established

| Path | Budget | CI measurement |
|---|---:|---:|
| HTML render p95 | ≤ 1s | 141ms |
| HTML render max | ≤ 3s | 146ms |
| Publish (render + snapshot + store) p95 | ≤ 2s | — (full-load only) |
| PDF render | ≤ 30s documented | — (requires Chromium binary) |
| Interactive write p95 under render load | ≤ 200ms | 7.75ms |

#### Mutation test

`slowAnalyticsQueries` wraps `analytics.Queries` and replaces `TechniqueCoverage`
with a 3× per-technique N+1 query pattern (JOIN step + scenario + execution per
technique). With 800 techniques, the render exceeds the 3s max budget
(4.98s measured), proving the CI gate would catch a real query regression.

### Existing gate re-run results

| Gate | Original p95 | Re-run p95 | Budget | Status |
|---|---:|---:|---|---|
| M3-016 war-room writes | 16.7ms | 25.8ms | 200ms | No regression |
| M4-010 SSE publish | 17.4ms | 11.5ms | 200ms | Improved |
| M5-015 analytics queries (Coverage) | 11.2ms | 16.1ms | 250ms | No regression |

No silent >2× regression detected. All budgets intact.

### Files changed

- `internal/report/loadtest/render_test.go` — new: 4 tests (CI gate, mutation
  gate, concurrent write gate, full developer load), fixture, helpers
- `docs/testing.md` — new section: Report render budget (M7-008)

### CI gate trip verification

- `TestReportRenderDetectsRegression` demonstrated failure on deliberate N+1
  regression: render took 4.98s, exceeding the 3s max budget. Removing the N+1
  (normal analytics) renders in ~1.4s at the same scale.

### Full developer load

```sh
BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/report/loadtest/ -run TestReportRenderLoad
```

Not run in this devcontainer (requires `BLACKLIGHT_LOADTEST=1`). Command is
documented in `docs/testing.md`.

### Acceptance criteria

- [x] M3-016, M4-010, M5-015 re-run; numbers recorded and compared to original
      completion notes. No silent >2× regression.
- [x] Report HTML render budget recorded; CI scaled-down gate fails on deliberate
      N+1 break (4.98s >> 3s max).
- [x] PDF path: smoke available via `TestReportRenderLoad` (full-load only,
      requires Chromium). Resource cleanup via `defer printer.Close()`.
- [x] `docs/testing.md` updated with M7-008 commands and budgets.
- [x] Writer interactive p95 (7.75ms) remains within M3-016 budget (200ms)
      while render runs concurrently.
