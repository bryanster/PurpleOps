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
- **One exception** — [MTTD](#mttd) counts `running` executions too. It
  measures detection latency, which exists the moment red starts, not
  attempt coverage. Every other rollup uses the definition above.

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
only** (category is not `none` and not NULL, both timestamps set). Reported
as p50 / p90 / max over those executions, in integer seconds.

### Scope: one status wider than attempted

MTTD is the **only** rollup that does not use the `attempted` definition
above. Its scope is `status IN ('complete', 'blocked', 'running')`, because
MTTD measures detection latency rather than attempt coverage: a detection can
land while the attack is still in flight, and the step view's Start button
leaves an execution `running` from the press until Stop. Under the narrower
definition a step red had started and blue had detected was dropped from every
bucket — percentiles and denominator alike — so the analytics panel reported
nothing for an execution whose own step view was showing an MTTD.

`pending` and `skipped` stay out: neither has a `started_at` to measure from.

The consequence to expect: an engagement mid-run reports MTTD percentiles
while Coverage, Detection mix and Protection rate still show nothing, because
those count attempted executions and a running one is not attempted. The
`attemptedCount` in the MTTD payload therefore counts what red has *begun*,
which can exceed the `attempted` reported elsewhere. Widening the others is a
product decision, not an oversight.

Percentiles use DuckDB's `PERCENTILE_CONT` (continuous percentile) with linear
interpolation. For a single sample all percentiles return that value; for two
samples p50 is the midpoint. Interpolation is stated here so a report consumer
can reproduce the numbers independently.

| Field | Meaning |
|---|---|
| `p50`, `p90`, `max` | Percentile and maximum MTTD in integer seconds. Absent when nothing was detected (nil, not zero). |
| `detectedCount` | Executions with a computable MTTD — the percentile denominator. |
| `undetectedCount` | Attempted executions with `detection_category` set and no `detected_at`, or category `none` even when `detected_at` exists. |
| `unscoredCount` | Begun executions blue has not scored (`detection_category IS NULL`). A step red is running right now lands here until blue scores it. |
| `unmeasurableCount` | Detected (category ≠ `none`) but `started_at` is NULL, so no duration exists. |
| `attemptedCount` | The sum of the four component counts above — every execution red has begun, per the scope note above. |

Category `none` counts as undetected even where a stray `detected_at` exists.
`detected_at` before `started_at` cannot occur by product rules, but the SQL
guards it — a negative duration is excluded from percentiles and treated as
unmeasurable.

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

### Date spine

The burndown series has a point for every day (or week) between
`engagement.starts_on` and `min(engagement.ends_on, today)`. Days with no
transitions are included — a burndown with gaps invites the reader to
interpolate.

### Day boundaries

Everything is UTC (README § Conventions). Day boundaries are UTC day
boundaries: a transition at 23:59 belongs to that day. The SQL uses
`CAST(changed_at AS DATE)` so the boundary is at midnight UTC. A client
in a non-UTC timezone will see transitions on what they call the next day.

### Status definitions

The burndown counts findings by status at each point:

| Status | Meaning |
|---|---|
| `open` | New, unremediated, needs attention. |
| `in_progress` | Remediation is underway. |
| `resolved` | Remediation is complete and verified. |
| `accepted_risk` | The risk is acknowledged and will not be remediated at this time. |

### Terminal statuses

`resolved` and `accepted_risk` are both **closed** for the purposes of
`totalOpen`. Accepted risk is a decision, not an outstanding item. The
burndown reports `acceptedRisk` in its own bucket so a viewer can see
exactly how many findings closed through acceptance rather than remediation.

### totalOpen

`totalOpen = open + inProgress`. Reported in every burndown point alongside
the individual status counts so no consumer must compute it — the definition
is sealed in this document.

### Reopen

A finding that was resolved and later reopened appears as `open` from the
reopen date onward. The burndown does not count a "total resolutions" —
it counts the state at each point, so a finding that was resolved twice
and is now resolved contributes exactly one to `resolved` at the current
date, not two.

### Pre-start and post-end findings

Findings created before `engagement.starts_on` are clamped onto the first
point. Findings created after `min(engagement.ends_on, today)` are clamped
onto the last point. A finding never disappears from the series because its
creation date falls outside the engagement window.

### Point cap and weekly fallback

When the date range from `starts_on` to `min(ends_on, today)` exceeds
`DefaultBurndownPointCap` days (90 by default), the burndown switches to
weekly buckets. The interval actually used is reported in the response
field `interval` — the consumer must not infer it from point spacing.
The weekly bucket uses ISO weeks grouped by `DATE_TRUNC('week', d)`, with
the date label being the Monday of each week.

### Severity snapshot

`FindingsBySeverity` returns the current snapshot of finding counts by
severity × status. Unlike the burndown this is a point-in-time view of
`app.finding.status`, not a time series derived from history. Every
severity level present in the engagement appears as a bucket; absent
severity levels are not emitted as zero rows.

## Navigator colour ramp

The detection-category ordinal (0–4) maps to a fixed colour ramp used by
both the ATT&CK Navigator layer export (M5-010) and the dashboard heatmap
(M5-013). One ramp, two renderers — a report and a Navigator side by side
must not disagree about what a colour means.

| Ordinal | Category   | Hex       | Swatch                                                  |
|---------|------------|-----------|---------------------------------------------------------|
| 0       | None       | `#aeb3bf` | <span style="color:#aeb3bf">████████</span> grey        |
| 1       | Telemetry  | `#ffee58` | <span style="color:#ffee58">████████</span> amber       |
| 2       | General    | `#fca128` | <span style="color:#fca128">████████</span> orange      |
| 3       | Tactic     | `#d13c3c` | <span style="color:#d13c3c">████████</span> red         |
| 4       | Technique  | `#862121` | <span style="color:#862121">████████</span> dark red    |

The ramp is defined in `internal/analytics/navigator.go` as
`NavigatorColourRamp`. The Navigator layer's `gradient.colors` array and
`legendItems` are built from it, and `M5-013`'s heatmap reads the same
array.

## Detection gaps

A **detection gap** is a technique with either:

1. **Attempted but undetected** — the technique has at least one attempted
   execution, and its best detection category is `none`. The red operator
   tested it and blue did not detect it.
2. **Not attempted** — the technique exists in the pinned ATT&CK version
   (it is in-scope) but has no attempted execution. Nobody has tested it
   yet.

The detection gaps report block enumerates both categories separately, with
counts drawn from `TechniqueCoverage` so the labels match every other coverage
number in the engagement.

A technique that is attempted and has any detection category other than
`none` is not a gap — it was detected to some degree. Unscored techniques
are not gaps because the blue side has not evaluated them yet.
