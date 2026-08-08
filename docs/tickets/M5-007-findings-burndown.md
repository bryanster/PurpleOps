# M5-007 — Findings burndown

**Milestone:** M5 · **Size:** M · **Depends on:** M5-001, M5-003

## Why

Findings are what make the retest loop meaningful (`PLAN.md` §2). The burndown is the chart that
shows remediation happening — open findings falling over the life of the engagement — and it is the
one M6 block that answers "did anything actually get fixed".

`M5-003` gave it a history table to read. This is the query.

## Scope

**In**

- `internal/analytics/burndown.go`:

  | Rollup | Returns |
  |---|---|
  | `FindingsBurndown(ctx, Scope, Interval)` | A daily (or weekly) series: date, `open`, `inProgress`, `resolved`, `acceptedRisk`, and `totalOpen` = open + inProgress |
  | `FindingsBySeverity(ctx, Scope)` | Current counts by severity × status — the snapshot beside the trend |

- **State at each point in time**, reconstructed from `app.finding_status_history` — the count of
  findings whose most recent transition on or before that date put them in each status. Not a count
  of transitions.
- **Terminal statuses:** `resolved` and `accepted_risk` are both closed for the purposes of
  `totalOpen`. Accepted risk is a decision, not an outstanding item. Stated in `docs/analytics.md`,
  because a client reading the chart will ask.
- **Date spine** from `engagement.starts_on` to `min(engagement.ends_on, today)`, so the series has a
  point for every day including days nothing happened — a burndown with gaps invites the reader to
  interpolate. Cap the point count (config, documented default) and fall back to weekly buckets past
  the cap so a two-year engagement does not return 700 points.
- `Interval` is `daily` | `weekly`, chosen by the caller, defaulting per the cap above.
- Findings created before the engagement's `starts_on`, or after `ends_on`, still appear — clamp them
  onto the first/last point rather than dropping them.

**Out**

- Burndown per owner, per severity trend, or forecast/velocity lines.
- Cross-engagement burndown (`M5-008` covers the comparison story).
- HTTP (`M5-009`), charting (`M5-013`).

## Blind scoping

This rollup counts **findings only** and never joins `finding_step`, so it introduces no new blind
surface: `finding.read` is held by all members (`internal/authz/policy.go`), and findings are not
seat-filtered. It still takes a `Scope` — every rollup does (`M5-001`), and a signature that made the
seat optional is the one somebody would later forget.

Whether a finding's **step links** are blind-filtered for blue is `M3-011`'s question, not this
ticket's. If the answer turns out to be no, that is a finding to raise, not to fix here.

## Acceptance criteria

- [ ] The series is derived from `finding_status_history`, and a test proves it by mutating the
      activity log and asserting the chart does not move.
- [ ] Counts at each point are the **state** at that date. A finding that went open → in_progress →
      resolved on three different days appears once per day, in the right bucket, never twice.
- [ ] Every day between the bounds has a point, including days with no transitions.
- [ ] A finding resolved and then reopened is open again from the reopen date — the fixture includes
      this, because it is the case a naive "count of resolutions" query gets wrong.
- [ ] `accepted_risk` is excluded from `totalOpen` and still reported in its own bucket.
- [ ] An engagement whose `ends_on` is in the future stops the series at today, not at `ends_on`.
- [ ] Past the configured point cap the interval switches to weekly, and the response says which
      interval it used — the consumer must not have to infer it from the spacing.
- [ ] An engagement with no findings returns a full spine of zeroes, not an empty array.

## Tests

- Hand-computed series over `analyticstest`'s findings and their history rows, asserted day by day
  across a short fixture window.
- The reopen case, the accepted-risk case, the created-before-start case, the ends-in-future case.
- Activity-log independence, per the first criterion.
- Cap/interval switch at the boundary and one past it.

## Notes for the implementer

- The date spine is generated in SQL, not in Go — a loop that builds dates and runs a query per day
  is exactly the N×M pattern `PLAN.md` §5 exists to remove. Whatever series-generating function you
  use goes in `M5-001`'s non-portable-construct list.
- `engagement.starts_on` / `ends_on` are `DATE`; history is `TIMESTAMP`. Be explicit about the
  boundary — a transition at 23:59 belongs to that day — and say which in `docs/analytics.md`.
- Timezone: everything is UTC (README § Conventions). Day boundaries are UTC day boundaries, and a
  client in Sydney will see a transition on what they call the next day. Document it; do not add a
  timezone parameter.
