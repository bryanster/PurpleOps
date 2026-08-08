# M5-006 — MTTD percentiles with detected/undetected counts

**Milestone:** M5 · **Size:** M · **Depends on:** M5-001

## Why

Mean time to detect is the most quotable and most misleading number in the product. A mean over three
fast detections, with thirty misses excluded, reads as excellent performance and describes a failure.

The epic's decision is to make that impossible to print: percentiles over detected executions only,
with the detected and undetected counts as **required** fields in the same payload. The API shape
does the enforcing, because a UI convention would be forgotten exactly once, in the client-facing
place.

## Scope

**In**

- `internal/analytics/mttd.go` — `MTTD(ctx, Scope)`, one statement, returning:

  | Field | Meaning |
  |---|---|
  | `p50`, `p90`, `max` | Over `detected_at − started_at` for executions where **both** are set |
  | `detectedCount` | Executions with a computable MTTD — the percentile denominator |
  | `undetectedCount` | Attempted executions with `detection_category` set and no `detected_at`, or category `none` |
  | `unscoredCount` | Attempted executions blue has not scored at all |
  | `unmeasurableCount` | Detected, but `started_at` is NULL so no duration exists |
  | `attemptedCount` | The four above must sum to this |

- All count fields are **required, non-nullable** in the Go struct and later in the OpenAPI schema
  (`M5-009`). A consumer cannot construct a response with percentiles and no denominator.
- Durations serialize as **seconds** (integer), not a formatted string — `PLAN.md`'s "never format a
  time for display on the server" applies to durations too. `docs/analytics.md` states the unit.
- Percentiles use the ANSI ordered-set aggregate form where DuckDB supports it; whichever form is
  used goes in `M5-001`'s non-portable-construct list.
- **No censored or infinite figure** in v1 (epic decision). If somebody wants a worst-case MTTD
  treating misses as detected at engagement end, that is a new ticket with a product decision
  attached, not an extra field added quietly.
- Blind fence via `Scope`.

**Out**

- Per-technique or per-tactic MTTD breakdowns. Useful, and not until somebody asks — one number with
  honest denominators first.
- MTTD trend over time.
- HTTP (`M5-009`), charting (`M5-013`).

## Acceptance criteria

- [ ] One statement.
- [ ] Percentiles hand-computed from `analyticstest`'s timestamps. With a small fixed sample this is
      arithmetic a reviewer can check on paper, which is the point of choosing the values by hand.
- [ ] Every count field is required in the struct; a test asserts the four component counts sum to
      `attemptedCount`.
- [ ] When nothing was detected: percentiles are **absent** (nil/omitted), not zero. Zero MTTD means
      "detected instantly" and is a lie the moment it renders.
- [ ] `detected_at` set with `started_at` NULL lands in `unmeasurableCount` and is excluded from the
      percentiles — it does not silently become a zero-length duration.
- [ ] Category `none` counts as undetected even where a stray `detected_at` exists.
- [ ] `detected_at` before `started_at` cannot occur (`M3-007` enforces it on write) but does not
      produce a negative percentile if it ever does — guarded and tested, mirroring
      `scoring.MTTD`'s own guard.
- [ ] Single-sample and two-sample fixtures produce defensible percentiles; whichever interpolation
      the chosen function uses is stated in `docs/analytics.md` so a report can be reproduced.
- [ ] Blue in the blind fixture sees a smaller `attemptedCount` and correspondingly different
      percentiles.

## Tests

- Hand-computed p50/p90/max over the fixture's detected executions.
- Degenerate cases: zero detections, one detection, two detections, all detections.
- The NULL `started_at` case, the category-`none`-with-timestamp case, and the inverted-timestamp
  guard.
- Seat sweep.

## Notes for the implementer

- `scoring.MTTD` is the Go definition of this duration and already guards the inverted case. The SQL
  must agree with it; add the agreement to the drift tests rather than assuming.
- Resist returning a mean. The epic asked for percentiles because the mean is the misleading one, and
  a `mean` field would be quoted the day it appeared.
- Seconds, integer, in the payload. Formatting belongs to the client (`PLAN.md` § Time).
