# M5-004 — Coverage rollups: technique and tactic

**Milestone:** M5 · **Size:** M · **Depends on:** M5-001, M5-002

## Why

Coverage is the number clients quote, so it is the number most worth being pedantic about. v1's
hexagon graphic implied a denominator it never named. This ticket names both of them and returns them
in the same payload, so no consumer can print a percentage whose meaning it did not choose.

Feeds the heatmap (`M5-013`), the Navigator layer (`M5-010`) and M6's coverage and per-tactic
scorecard blocks.

## Scope

**In**

- `internal/analytics/coverage.go` — two rollups, each a **single statement**:

  | Rollup | Returns |
  |---|---|
  | `TechniqueCoverage(ctx, Scope)` | One row per distinct technique (and sub-technique) in the engagement: external id, name, whether attempted, best detection category and its ordinal, best protection, step count |
  | `TacticCoverage(ctx, Scope)` | One row per tactic in the pinned ATT&CK version: tactic external id, name, techniques attempted, techniques in the matrix under that tactic, and the category distribution beneath it |

- **Both denominators, always** (epic decision). Each response carries:
  - `attemptedTechniques` — distinct techniques with an execution in `('complete','blocked')`,
  - `notAttemptedTechniques` — distinct techniques whose executions are all `pending` / `running` /
    `skipped`,
  - `matrixTechniques` — distinct techniques in `content.content_technique` for
    `storecontent.SourceIDAttack` at `engagement.attack_version`.
  - Counts, not percentages. The consumer divides, and `docs/analytics.md` says which over which.
- **Sub-techniques are their own cells.** `T1059.001` does not make `T1059` covered. Roll parent and
  child separately and let the heatmap nest them.
- **Tactic membership via `content.content_technique_tactic`**, at the engagement's pinned version.
  A technique in two tactics counts in both; the sum of tactic counts therefore exceeds the technique
  count, which is correct and is stated in `docs/analytics.md` and in the response's own field doc.
- **Best-of aggregation:** where a technique has several steps, the technique's category is the
  **maximum ordinal** across its attempted executions, and its protection is the best achieved.
  Document the choice — "did we ever detect this" is the question a coverage cell answers.
- Blind fence via `Scope` (`M5-001`), in the `WHERE` clause.
- Techniques in the workbook that are **absent from the pinned version** get their own bucket
  (`unmatchedTechniques`) rather than being dropped. A step naming a technique the pin does not
  contain is a data problem the report must show, not hide.

**Out**

- HTTP and OpenAPI (`M5-009`).
- Navigator layer shaping (`M5-010`).
- Heatmap rendering (`M5-013`).
- Any per-technique trend over time.

## Acceptance criteria

- [ ] Both rollups are one statement each. A `for` loop over rows that computes an aggregate is a
      review rejection (`PLAN.md` §5).
- [ ] Expected values are **hand-computed** from `analyticstest`'s tables, including the
      technique-in-two-tactics case and the sub-technique case.
- [ ] `skipped` and `pending` executions are in `notAttempted`, never in `attempted`. A workbook
      where nothing has run reports zero coverage and a full `notAttempted` count — not an empty
      result.
- [ ] `attempted + notAttempted` equals the distinct technique count in the workbook, asserted
      arithmetically rather than by eye.
- [ ] A `complete` but **unscored** execution counts as attempted, with category `unscored` — not
      `none`.
- [ ] Blue in the blind fixture engagement gets strictly smaller counts than lead, and the missing
      techniques are exactly the unrevealed ones. Asserted as a set difference, not a count.
- [ ] A technique absent from the pinned ATT&CK version appears in `unmatchedTechniques` and is
      excluded from `matrixTechniques`.
- [ ] `matrixTechniques` changes when the engagement's pin changes, proven by a second engagement
      pinned to a different seeded version.

## Tests

- Fixture-based, hand-computed, per acceptance criteria above.
- Seat sweep: lead, red, blue, observer, and platform admin with no seat — five scopes, one table.
- Empty engagement: no scenarios, no steps. Returns zeroed counts and no error.
- An engagement pinned to a version with **no** content installed: `matrixTechniques` is zero and the
  attempted numbers still compute. A missing pin must not take the dashboard down.

## Notes for the implementer

- Join `app.step` → `content.content_technique` on `(source_id, version, external_id)` — the natural
  key `0011_content.sql` declares. There is deliberately **no FK** from `app` to `content`
  (`docs/content-copy-on-use.md`), so this join can miss, which is what `unmatchedTechniques` is for.
- `app.step.technique_id` and `subtechnique_id` are ATT&CK external ids as text and are nullable.
  Decide once whether a sub-technique row carries its parent in `technique_id` — the fixture must
  cover whichever the M3 writers actually do, so read `M3-005` before assuming.
- Prefer ANSI over DuckDB shorthand where both exist and read the same. Where you use a DuckDB-only
  construct, add it to the package doc's list (`M5-001`).

## Implementation notes

- `TechniqueCoverage` uses a single CTE chain: `workbook_techniques → technique_execs → best → per_technique → SELECT` with subquery columns for summary counts. The blind fence (`stepPredicate`) is injected via `%%s` into the CTE's WHERE clause.
- `TacticCoverage` uses a separate `fillCategoryDistribution` query because combining tactic-level aggregation with per-category counts in a single statement would require JSON aggregation — and with ~14 tactics, two queries trade one round trip for significantly simpler scan logic. Each query is still one statement.
- Category ordinals (`CASE te.detection_category WHEN ...`) and protection ordinals (`CASE te.protection WHEN ...`) are inlined in SQL rather than a lookup table because the scoring vocabulary is closed (CHECK constraints on `app.execution`) and DuckDB MAX over CASE expressions is the natural way to do "best of" for a small closed set.
- `BestCategoryOrdinal` is `*int` — nil means unscored (no attempted + scored execution for this technique).
- Sub-techniques are separate cells: `T1059.001` does not make `T1059` covered. The fixture has both parent and child steps to prove this.
- DuckDB-specific: `||` for string concatenation, `BOOL_OR` instead of `MAX`, CASE expressions for ordinal mapping. All named in the package doc exception list.
