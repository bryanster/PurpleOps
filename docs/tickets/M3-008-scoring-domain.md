# M3-008 — Scoring domain logic

**Milestone:** M3 · **Size:** M · **Depends on:** M3-001 (types may live without HTTP)

## Why

ATT&CK Evaluations vocabulary is the product’s scoring spine (`PLAN.md` §2, §8). Category is
ordinal (`none`=0 … `technique`=4); **outcome is derived, never entered**; MTTD =
`detected_at − started_at`. Pure functions keep M5 SQL and UI honest.

## Scope

**In**

- Package `internal/domain/scoring` (no I/O, no store import):

  | API | Behavior |
  |---|---|
  | `Category` + constants | `none`, `telemetry`, `general`, `tactic`, `technique` |
  | `Ordinal(c Category) int` | 0..4; unknown → error |
  | `ParseCategory` / `ParseModifier` / `ParseProtection` | wire strings |
  | `Modifiers` set | `alert`, `correlated`, `delayed`, `config_change`, `residual_artifact` — multi-select, not exclusive |
  | `ValidateModifiers([]string) error` | unknown rejected; duplicates collapsed or rejected — pick one, test it |
  | `Outcome` derived type | e.g. labels used by UI/report: derive from category × protection (document matrix). **Not stored.** |
  | `DeriveOutcome(category, protection) (Outcome, error)` | pure; table-driven |
  | `MTTD(started, detected *time.Time) (time.Duration, bool)` | ok false if either nil; error/false if detected < started |
  | `CompareOrdinal(a, b) int` | for later deltas / tests |

- Document the outcome matrix in package doc + short `docs/scoring.md` (normative for UI copy).
- OpenAPI enums for category/modifiers/protection should be generated from the same source of truth
  **or** drift-tested against Go constants (prefer a `go:generate` string list or test that parses
  OpenAPI enum and compares). At minimum, a test fails if OpenAPI has a value Go doesn’t know once
  both exist (`M3-007` lands OpenAPI).

**Out**

- HTTP handlers (`M3-007`).
- SQL rollups (M5).
- UI (`M3-015`).

## Acceptance criteria

- [ ] Table-driven tests cover **all** category × protection pairs for `DeriveOutcome` (hand-written
      expected values, not captured from the function under test).
- [ ] Every modifier constant round-trips Parse; unknown fails.
- [ ] Ordinal monotonicity: none < telemetry < general < tactic < technique.
- [ ] MTTD: nil cases, positive duration, inverted timestamps.
- [ ] Package import boundary: no `store`, `http`, `sql`.

## Tests

- Pure unit tests only — the deliverable is the test table.

## Notes for the implementer

- Keep outcome vocabulary small and stable; M6 reports will print it.
- Do not fold modifiers into ordinal math — modifiers are descriptive only (`M3-EPIC`).
