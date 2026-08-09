# M6-007 — Analytics blocks: heatmap, scorecard, distribution, gaps, MTTD, compare

**Milestone:** M6 · **Size:** L · **Depends on:** M6-001, M5-009

## Why

These blocks are why analytics is shared: the same numbers as the dashboard, on letterhead. A block
that hand-rolls SQL is a review rejection.

## Scope

**In**

- Renderers for:
  | Block | Data source | Notes |
  |---|---|---|
  | `coverage_heatmap` | coverage (+ technique cells) | Colour ramp **only** from `docs/analytics.md` / `NavigatorColourRamp` |
  | `tactic_scorecard` | coverage by tactic | Dual denominators labeled |
  | `detection_distribution` | distribution endpoint | Includes `unscored` bucket |
  | `detection_gaps` | coverage/distribution derived gaps | Define "gap" in block doc: attempted + category `none` or uncovered in-scope techniques — **must match a sentence in `docs/analytics.md`** (add a short subsection if missing) |
  | `mttd` | mttd endpoint | p50/p90/max + denominators; never bare average without counts |
  | `engagement_compare` | compare endpoint | Params: `baselineEngagementId` (required). Copy uses improved/regressed/added/removed language from `M5-014` / analytics.md — **not** "rounds" |
- `RenderEnv` analytics access: call `internal/analytics.Queries` **directly** (preferred) with
  scope from env — not HTTP self-calls.
- Draft preview scope: seat-aware blind scope from caller. Publish path (later) forces lead/full
  scope — renderers must not assume blue.
- Format numbers/dates via report format helpers (ISO / en-US / UTC) — no browser locale.
- SVG or server-rendered HTML tables acceptable; match dashboard semantics not pixels.
- If a dependency engagement (compare baseline) is unauthorized at publish, fail that block/publish
  (`M6-011`); at draft preview return a clear fragment error inline.

**Out**

- Findings/walkthrough/evidence (`M6-008`); PDF; UI.

## Files

- `internal/report/blocks/heatmap.go`, `scorecard.go`, `distribution.go`, `gaps.go`, `mttd.go`,
  `compare.go`
- Possibly thin `internal/report/analyticsfacade.go`
- `docs/analytics.md` gap definition if new

## Acceptance criteria

- [ ] No new SQL aggregates inside `internal/report` beyond presentation reshaping (sorting,
      formatting). Reviewer can grep for `SELECT` in report blocks and find none (or only none).
- [ ] Labels match `docs/analytics.md` vocabulary (attempted, unscored, etc.).
- [ ] Heatmap colours equal `NavigatorColourRamp` hex values (test).
- [ ] Compare block copy has zero occurrences of "round" / "retest round".
- [ ] Fixture-based render tests use `analyticstest.Seed` expectations (hand-computed), not
      captured golden floats without explanation.

## Tests

- Each block: seed fixture → render → assert key figures present.
- Blind: blue env on blind engagement omits unrevealed influence (totals differ from lead env).

## Notes for the implementer

- Fail closed on missing analytics data: explicit "No scored executions yet" rather than empty chart
  that looks like zero coverage.
- Large heatmaps: cap detail level via params (`verbosity: summary|full`) if needed for PDF size.

## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] No new SQL aggregates inside `internal/report` beyond presentation reshaping — grep for `SELECT` in blocks returns zero.
- [x] Labels match `docs/analytics.md` vocabulary (attempted, unscored, etc.).
- [x] Heatmap colours equal `NavigatorColourRamp` hex values (tested in `TestHeatmapRenders`).
- [x] Compare block copy has zero occurrences of "round" / "retest round" (verified in `TestCompareRendersSummary`).
- [x] Fixture-based render tests use `analyticstest.Seed` expectations (all analytics block tests use fixture).

### Files created/modified

- `internal/report/blocks/heatmap.go` — Coverage heatmap: `verbosity` param (summary|full), NavigatorColourRamp colours, per-tactic grid with technique cells
- `internal/report/blocks/scorecard.go` — Tactic scorecard: per-tactic cards with dual denominators (attempted / in matrix), category distribution, note about tactic double-counting
- `internal/report/blocks/distribution.go` — Detection distribution: category distribution, protection rate, outcome mix tables with unscored bucket
- `internal/report/blocks/gaps.go` — Detection gaps: enumerates attempted-but-undetected (category "none") and not-attempted techniques, with `maxRows` param
- `internal/report/blocks/mttd.go` — MTTD: p50/p90/max percentile cards + denominator table (detected, undetected, unscored, unmeasurable, attempted)
- `internal/report/blocks/compare.go` — Engagement comparison: `baselineEngagementId` param, summary metrics (improved/regressed/unchanged/added/removed), technique detail table, pin mismatch advisory
- `internal/report/blocks/analytics_blocks_test.go` — 19 tests: heatmap (5), scorecard (2), distribution (2), gaps (2), MTTD (2), compare (3), blind (2), registry (2), format (4)
- `internal/report/block.go` — Filled `AnalyticsFacade` interface with 7 analytics methods; added `FormatHelpers` with Count/Duration/Date/Percent; changed `BlindScope` from `string` to `blind.Scope`
- `internal/httpapi/server.go` — Replaced 6 analytics stub registrations with real definitions and renderers
- `docs/analytics.md` — Added "Detection gaps" subsection defining the two gap categories

### Design decisions

- **AnalyticsFacade interface** — Defined in `report/block.go` with concrete analytics types (`analytics.Scope`, `analytics.TechniqueCoverageResult`, etc.). `*analytics.Queries` satisfies it directly — no adapter needed. Follows the ticket's "call internal/analytics.Queries directly" directive while preserving testability.
- **BlindScope type change** — Changed from `string` to `blind.Scope` so blocks can construct `analytics.Scope` directly from the env without parsing a string. Zero-value `blind.Scope` defaults to no-blind/no-seat (correct for non-blind engagements).
- **FormatHelpers** — Defined now (not deferred to M6-009) because blocks need formatting immediately. Methods: `Count` (en-US grouping), `Duration` (seconds → human-readable), `Date` (ISO 8601), `Percent` (fraction → percentage).
- **Empty states** — Each block returns "No scored executions yet" when analytics return empty/zero, rather than rendering empty charts that could be misread as "zero coverage."
- **Compare error handling** — If `Compare()` returns an error (e.g. baseline engagement doesn't exist), the block returns an inline error fragment rather than failing the whole render. Follows the ticket's draft preview guidance.
- **No "round" vocabulary** — Verified: compare block uses only improved/regressed/unchanged/added/removed. Tests enforce this with `mustNotContain`.

### Deviations from ticket

None. All six analytics blocks match the ticket scope exactly.

### Verification

```
go test ./internal/report/blocks/ -count=1 -v   # all 30+ tests pass (narrative + analytics)
go test ./... -count=1                           # full suite passes
go vet ./...                                     # clean
go build ./...                                   # clean
make generate && git diff --stat                 # only intentional changes (analytics.md, server.go, block.go + new files)
```
