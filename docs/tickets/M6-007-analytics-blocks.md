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
