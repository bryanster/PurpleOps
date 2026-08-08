# M5 — Analytics (epic)

**State:** refined · **Depends on:** M3 complete, M4 complete (including the **M4-010** gate)

## Goal

Turn the workbook into the numbers a purple-team programme is judged on — coverage, detection-category
distribution, protection rate, MTTD, findings burndown, and the baseline-vs-retest delta — computed in
**SQL, not application loops** (`PLAN.md` §5). This is the payoff for choosing DuckDB: v1's N×M read
pattern becomes single statements.

One source, two consumers. Every number the dashboard shows is the same number an M6 report block
prints, from the same query. There is no second aggregation path.

## Decisions (locked)

| Topic | Decision |
|---|---|
| **Coverage denominator** | **Both, attempted is the headline.** Every coverage response carries `attempted` (distinct techniques with a non-skipped execution in this engagement) **and** `matrix` (distinct techniques in `content.content_technique` for `storecontent.SourceIDAttack` at `engagement.attack_version`). Both are named fields with named denominators — no endpoint returns a bare percentage, so no consumer can print one unlabelled. |
| **What "attempted" means** | Red `status` ∈ {`complete`, `blocked`}. `pending`, `running` and `skipped` are **not** attempted and are reported as `notAttempted` alongside, so the buckets sum to the workbook size. A skipped step is not evidence of anything. |
| **Scored vs unscored** | `detection_category IS NULL` is **`unscored`**, never `none`. Every distribution carries an explicit `unscored` bucket. Folding unscored into `none` would report "we tested and saw nothing" when the truth is "nobody looked". |
| **Sub-technique rollup** | Sub-techniques are their own cells for coverage, heatmap and Navigator. A parent technique counts as covered **only** if a step names the parent. Tactic rollups join `content.content_technique_tactic`, so a technique in two tactics counts in **both** — documented in `docs/analytics.md`, not silently deduplicated. |
| **Blind mode** | **Seat-scoped rollups.** Every analytics query takes a `blind.Scope`; the blue seat of a blind engagement aggregates over **revealed steps only**. Totals legitimately differ per seat — aggregates leak existence just as row ids do, so the filter is in the `WHERE` clause, not in a post-pass. Every response echoes the scope it was computed under, and the UI must label it (`M5-013`). |
| **Outcome in SQL** | Outcome is derived in SQL, and a test asserts the SQL agrees with `scoring.DeriveOutcome` for **every** category × protection pair including the nil cases. Two implementations of one matrix are permitted only with a drift test binding them — the same rule M3-008 applied to the OpenAPI enums. |
| **MTTD** | p50 / p90 / max over **detected executions only**. `detectedCount` and `undetectedCount` are **required** response fields, so no consumer can render MTTD without its denominator. No censored or infinite figure in v1. |
| **Rounds → compare** | Rounds stay dropped (`M3-EPIC`). Their replacement is **ad-hoc cross-engagement compare**: `baseline` and `current` engagement ids as query parameters, caller must hold `report.read` on **both**, steps matched on `(technique_id, subtechnique_id)` then `template_id`. **No `baseline_engagement_id` column** — any two readable engagements can be compared. This closes the re-scope that `M3-EPIC` deferred to M5. |
| **PLAN.md §9 steps 6–7** | Rewritten by this epic: step 6 becomes "create the retest engagement from the open findings' steps, score them higher"; step 7's round-comparison block becomes the **engagement-comparison** block. `M6-018` owns the spec; `M5-008` owns the query it rests on. |
| **Findings burndown** | New append-only **`app.finding_status_history`**, written in the same transaction as the status change (`M5-003`). **Not** reconstructed from `app.activity`: `0009_activity.sql` already plans a `blctl` prune, and a burndown that silently rewrites itself after a retention run is exactly the class of error clients notice. |
| **Navigator** | One layer schema version **pinned in code**; the layer's ATT&CK version comes from `engagement.attack_version`. Scores from the detection-category ordinal (0–4) with a colour ramp documented in `docs/analytics.md`. A layer that re-maps itself when ATT&CK updates is the hazard the per-engagement pin exists to prevent. |
| **Archive** | **Export only.** One versioned format — a zip of `manifest.json` plus evidence files — carrying an explicit `formatVersion`. The round-trip test is export → re-parse → compare object graph, which is what actually catches v1's `export.csv` / `export.json` mismatch. **No import endpoint in v1**; restore remains the DuckDB file backup (`M7-005`). |
| **Authz** | Analytics and every export use the existing **`report.read`** action (all members, token scope `reports:read`). **No new action.** Analytics *is* the data behind reports; a token that can fetch a rendered report and not the numbers under it is a distinction nobody asked for. The archive is `report.read` too — it contains nothing a member cannot already read one row at a time. |
| **Caching** | **None.** No materialized views, no rollup tables, no in-process cache. `M5-015` sets the budget; a query that misses it gets fixed or indexed. Adding staleness to solve a problem nobody has measured is how analytics starts lying. |
| **SQL portability** | `internal/analytics/` is the **documented exception** to the ANSI-only rule (README § Conventions). DuckDB-specific constructs are allowed **inside that package only**, and each one is named in the package doc with what porting it would cost. Percentiles and JSON unnesting are where this bites. |
| **Query style** | One rollup per file, each a **single statement**. No application-side loop over rows to compute an aggregate (`PLAN.md` §5). Reads go through the read pool; analytics never touches `store.Write`. |
| **Test values** | Every rollup test asserts **hand-computed** expected values against the seeded fixture from `M5-001`. Values captured from the implementation's own output only prove the code has not changed, and are a review rejection. |
| **Exit gate** | **M5-015** — query budget on a realistic fixture, measured under concurrent write load. M6 runs every one of these queries per report render, inside a Chromium PDF timeout. |

## Tickets

Build roughly in this order — the dependency chain is real. **M5-015 is a gate before M6.**

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M5-001](M5-001-analytics-query-layer.md) | Analytics query layer + seeded fixture | L | M3, M4-010 |
| [M5-002](M5-002-blind-query-fence.md) | Query-layer blind fence for step reads (M3 debt) | S | M3-005 |
| [M5-003](M5-003-finding-status-history.md) | `finding_status_history` migration + write path | M | M3-011 |
| [M5-004](M5-004-coverage-rollups.md) | Coverage rollups: technique and tactic, dual denominator | M | M5-001, M5-002 |
| [M5-005](M5-005-detection-distribution.md) | Detection-category distribution, protection rate, outcome mix | M | M5-001 |
| [M5-006](M5-006-mttd-analysis.md) | MTTD percentiles with detected/undetected counts | M | M5-001 |
| [M5-007](M5-007-findings-burndown.md) | Findings burndown | M | M5-001, M5-003 |
| [M5-008](M5-008-cross-engagement-compare.md) | Cross-engagement compare rollup | L | M5-004, M5-005 |
| [M5-009](M5-009-analytics-endpoints.md) | Analytics read endpoints + blind scoping + authz | L | M5-004…M5-008 |
| [M5-010](M5-010-navigator-layer-export.md) | ATT&CK Navigator layer export | M | M5-004, M5-009 |
| [M5-011](M5-011-json-csv-exports.md) | JSON and CSV exports | M | M5-009 |
| [M5-012](M5-012-engagement-archive-export.md) | Engagement archive export (versioned, round-tripped) | L | M5-011, M3-009 |
| [M5-013](M5-013-dashboard-ui.md) | Dashboard UI: heatmap and scorecards | L | M5-009, M3-014 |
| [M5-014](M5-014-compare-ui.md) | Cross-engagement compare UI | M | M5-008, M5-013 |
| [M5-015](M5-015-analytics-query-budget.md) | Analytics query budget (**gate before M6**) | M | M5-009 |

## Risks

- **Correctness errors land in a client report with a logo on it.** This is the milestone where a
  wrong number survives all the way to somebody's board pack. Hand-computed fixture expectations, no
  exceptions, and a reviewer who checks the arithmetic rather than the diff.
- **Two outcome matrices.** `scoring.DeriveOutcome` in Go and a `CASE` expression in SQL will drift
  the first time somebody adds a protection value. The binding test is mandatory, not a nice-to-have.
- **Seat-scoped totals will start an argument.** In a blind engagement red and blue see different
  numbers, correctly. If `M5-013` does not label the view, the first war room to notice will file it
  as a data-integrity bug.
- **Compare rests on stable technique identity.** `M3-EPIC`'s soft freeze is what makes
  `(technique_id, subtechnique_id)` trustworthy across two engagements. A compare that returns
  nonsense is more likely a step edited before freeze engaged than a bug in the join.
- **The archive can be 2 GiB** (the per-engagement evidence quota, `M3-EPIC`). Stream it to the
  response; a handler that buffers one in memory takes the process with it.
- **CSV injection.** A finding titled `=cmd|...` is a formula when the client opens it in Excel.
  Escaping is an acceptance criterion in `M5-011`, not an implementer's discretion.
- **Analytics reads run while the war room writes.** `M5-015` must measure under concurrent write
  load; a budget measured against a quiet database is a budget measured against the wrong database.
- **`internal/analytics` may not import `internal/httpapi`, and must not grow a service layer.**
  It answers questions; the handler decides who may ask.

## Out of milestone (do not pull in)

- Report builder, block registry, HTML/PDF rendering, share links (M6).
- Programme-level or cross-client analytics beyond the two-engagement compare.
- Retest **rounds** — still dropped (`M3-EPIC`).
- Archive **import** / engagement restore into another install.
- Materialized rollups, scheduled aggregation, any cache.
- Data-bound custom query blocks (`PLAN.md` §8 keeps these out of v1 entirely).
- Scheduled or emailed reports.
- Per-user or per-actor productivity metrics. Nobody asked, and it changes what the tool is for.
