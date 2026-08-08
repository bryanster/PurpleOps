# M5-005 — Detection-category distribution, protection rate, outcome mix

**Milestone:** M5 · **Size:** M · **Depends on:** M5-001

## Why

The three headline distributions: how well things were detected, how often they were stopped, and the
derived outcome that combines the two. `M3-008` made the vocabulary ordinal and made outcome derived
precisely so these are `GROUP BY`s rather than the read-every-row-and-count loop v1 used.

This is also where the two implementations of the outcome matrix meet — Go's `scoring.DeriveOutcome`
and this package's SQL. The drift test from `M5-001` is what keeps them one matrix.

## Scope

**In**

- `internal/analytics/distribution.go` — each a single statement:

  | Rollup | Returns |
  |---|---|
  | `CategoryDistribution(ctx, Scope)` | Count per `none`\|`telemetry`\|`general`\|`tactic`\|`technique`, **plus `unscored`**, plus `notAttempted` |
  | `ProtectionRate(ctx, Scope)` | Count per `blocked`\|`partial`\|`not_blocked`\|`n/a`, plus `unscored` |
  | `OutcomeMix(ctx, Scope)` | Count per derived outcome (`prevented`\|`detected`\|`not_detected`\|`not_applicable`), plus `unscored`, using `outcomeCase` from `M5-001` |
  | `ModifierDistribution(ctx, Scope)` | Count per modifier across `detection_modifiers`, which is a JSON array — one execution contributes to several buckets |

- **Every bucket present, including zeroes.** A distribution that omits empty categories makes a
  chart with missing bars and a reader who thinks the axis changed. Return all five categories, all
  four protections, all five modifiers, every time.
- `unscored` is its own bucket everywhere and is **never** folded into `none` (epic decision). Each
  response carries `attempted` so a consumer can state "31 of 40 attempted steps scored".
- Modifier counts are explicitly **non-exclusive** and do not sum to the execution count — stated in
  the response's field doc and in `docs/analytics.md`, because a pie chart of them would be wrong.
- Blind fence via `Scope`.

**Out**

- MTTD (`M5-006`).
- HTTP and OpenAPI (`M5-009`).
- Chart rendering (`M5-013`).
- Any weighting or scoring formula that combines the distributions into a single grade. Nobody has
  agreed what that would mean, and inventing one here would put an unagreed number in a client
  report.

## Acceptance criteria

- [x] All four rollups are one statement each.
- [x] Expected values hand-computed from `analyticstest`.
- [x] Zero-count buckets are present in every response, asserted against the full enum from
      `scoring.CategoryStrings()` / `ProtectionStrings()` / `ModifierStrings()` rather than a literal
      list a test maintains separately.
- [x] `unscored` never appears as `none`: a fixture execution that is `complete` with NULL category
      lands in `unscored` in all three of category, protection and outcome.
- [x] `OutcomeMix` agrees with `scoring.DeriveOutcome` applied row-by-row in Go over the same
      fixture — the drift test, run against real rows and not just against the enum cross-product.
- [x] Modifier counts handle: empty array, one modifier, all five, and a duplicate in the stored JSON
      (`M3-008` collapses duplicates on write, so the SQL must agree rather than double-count if one
      ever survives).
- [x] Category counts sum to `attempted`; modifier counts deliberately do not, and a test asserts
      that non-equality so nobody "fixes" it later.
- [x] Blue in the blind fixture gets smaller counts than lead, differing by exactly the unrevealed
      steps.

## Tests

- Fixture-based with hand-computed values, per above.
- Row-by-row Go/SQL outcome agreement across the whole fixture.
- Seat sweep across lead/red/blue/observer/admin.
- Empty engagement: every bucket zero, no error, no missing keys.

## Notes for the implementer

- `detection_modifiers` is a JSON array column (`0016_app_domain.sql`). Unnesting it is the clearest
  case for the epic's DuckDB-syntax exception — use the JSON functions, and add them to the package
  doc's list of non-portable constructs with a note on what an ANSI fallback would cost.
- `0016_app_domain.sql` deliberately does not CHECK the array contents so the vocabulary can grow
  without a migration. That means this query can encounter a modifier `scoring` does not know. Count
  it in an `other` bucket rather than dropping it silently, and say so in the field doc.
- Do not reimplement the outcome matrix. Compose `outcomeCase` from `M5-001`; if it needs a change,
  change it there and let the drift test judge.
