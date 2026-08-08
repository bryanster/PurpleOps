# Analytics vocabulary

Every term here is normative for the dashboard (M5-013), the report builder
(M6), and every cross-engagement comparison (M5-008). The UI and report copy
derive from this document; a term that changes here must update the copy.

## Engagement scope

- **Engagement** — one assessment, identified by `engagement_id`.
- **Blind mode** — a blind engagement withholds unrevealed steps from the blue
  seat. Analytics queries filter on `blind.Scope` at the `WHERE`-clause level,
  so totals differ per seat. Every analytics response echoes the scope it was
  computed under; the UI labels it.
- **Seat** — the reader's role in the engagement: `lead`, `red`, `blue`,
  `observer`, or empty (platform administrator outside the engagement).

## Attempted vs not-attempted

- **Attempted** — an execution where `execution.status IN ('complete',
  'blocked')`. The red operator ran it (or tried and was blocked); it is
  evidence either way. This is the headline coverage denominator.
- **Not attempted** — `pending`, `running` and `skipped`. A `pending` step
  has not been touched; a `running` one has not concluded; a `skipped` one
  was deliberately omitted and is evidence of nothing.
- **Workbook size** — `attempted + notAttempted` equals the total step count
  in the engagement. Every rollup that reports `attempted` carries a
  `notAttempted` count alongside it, so the buckets sum to a known total.

## Scored vs unscored

- **Scored** — an execution where both `detection_category` and `protection`
  are non-null. The blue side has evaluated it.
- **Unscored** — either column is NULL. The blue side has not evaluated it.
  This is explicitly *not* `none`; folding unscored into `none` would report
  "we tested and saw nothing" when the truth is "nobody looked". Every
  distribution carries an explicit `unscored` bucket.

## Covered

A technique is **covered** when there is at least one attempted execution
for a step referencing that technique. The coverage set is the distinct
`technique_id` values from `app.step` joined to `app.execution` where the
execution is attempted.

`app.step.technique_id` can be NULL — a step without technique lineage is
not counted in coverage. `app.step.subtechnique_id`, when set, references
the ATT&CK external-id of a sub-technique, not a parent; it is treated as
its own cell.

## Coverage denominators

Every coverage response carries **two** denominators, both named fields:

| Denominator | Definition |
|---|---|
| `attempted` | Distinct techniques with a non-skipped execution in this engagement. The headline number. |
| `matrix` | Distinct techniques in `content.content_technique` for `storecontent.SourceIDAttack` at the engagement's `attack_version`. The total ATT&CK matrix size at that version. |

No endpoint returns a bare percentage — both numerators and both denominators
are present, so no consumer can print a number unlabelled.

## Tactic double-counting

Tactic rollups join through `content.content_technique_tactic`. A technique
in two tactics (e.g. T1059 in both `execution` and `lateral-movement`) counts
in **both**. The sum of tactic-covered counts can exceed the technique-covered
count. This is documented, not silently deduplicated.

## Sub-technique rollup

Sub-techniques are their own cells for coverage, heatmap and Navigator. A
parent technique counts as covered **only** if a step names the parent
directly. No inference up or down the sub-technique tree — the tree is for
navigation, not aggregation.

## Outcome in SQL

Outcome is derived in SQL by a `CASE` expression (`outcomeCase` in
`internal/analytics/sqlfragments.go`) matching `scoring.DeriveOutcome`.
The two are bound by a test that enumerates every `category × protection`
pair. The outcome labels:

| Label | Meaning |
|---|---|
| `prevented` | Attack was blocked or partially blocked. |
| `detected` | Detected but not prevented. |
| `not_detected` | Category is `none`, protection is `not_blocked`. |
| `not_applicable` | Blue did not report (protection is `n/a`). |
| `unscored` | Either column is NULL — blue has not evaluated yet. |

## MTTD

Mean Time To Detect = `detected_at − started_at` for **detected executions
only** (outcome is `detected` or `prevented`, both timestamps set). Reported
as p50 / p90 / max.

Every MTTD response carries `detectedCount` and `undetectedCount` so no
consumer can render MTTD without its denominator. No censored or infinite
figure in v1.

## Cross-engagement compare

Two engagements compared on `(technique_id, subtechnique_id)` then
`template_id`. The caller must hold `report.read` on both; there is no
`baseline_engagement_id` column. Steps that exist only in one engagement
are reported as added or removed; steps in both with different outcomes
are reported as improved, unchanged, or regressed.

## Findings burndown

Findings status changes are tracked in `app.finding_status_history`, written
in the same transaction as the status change. The burndown is reconstructed
from the history table, not from `app.activity` — a retention run that prunes
activity must not silently rewrite the burndown.
