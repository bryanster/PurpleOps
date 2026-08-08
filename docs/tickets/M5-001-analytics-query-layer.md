# M5-001 — Analytics query layer + seeded fixture

**Milestone:** M5 · **Size:** L · **Depends on:** M3 complete, M4-010 gate

## Why

`internal/analytics/doc.go` has been a one-paragraph promise since M0b. This ticket makes it a
package: the construction pattern every rollup uses, the shared SQL vocabulary they compose, and the
fixture database their expectations are hand-computed against.

Nothing else in M5 is reviewable until the fixture exists, because "is this number right" is not a
question you can answer against ad-hoc test data. The fixture *is* the deliverable — the rollups that
follow are comparatively mechanical.

## Scope

**In**

- **Package `internal/analytics`** — replace the stub `doc.go` with a real package doc covering the
  M5 decision table, the DuckDB-syntax exception, and the one-statement rule.

  | API | Behavior |
  |---|---|
  | `type Queries struct` | Holds a read-side handle. Constructor-injected (`NewQueries(db *store.DB) *Queries`); no package-level global (DoD 5) |
  | `type Scope struct` | `EngagementID string` + `Blind blind.Scope`. **Every** rollup takes exactly one `Scope`, so no rollup can be written that forgot the seat |
  | `func (s Scope) stepPredicate() string` | The blind fence as SQL: `blind.Scope.Where("revealed_at IS NOT NULL")`. One helper, so fifteen queries cannot each get it slightly wrong |
  | `attemptedPredicate` | `execution.status IN ('complete','blocked')` as a named constant with the decision quoted above it |
  | `outcomeCase` | The SQL `CASE` deriving outcome from `detection_category` × `protection`, matching `scoring.DeriveOutcome` |

- **Fixture package `internal/analytics/analyticstest`** over `storetest.Migrated`:
  - A **small synthetic ATT&CK version** in `content` (`storecontent.SourceIDAttack`, version
    `"99.0"`): ~10 techniques including two sub-techniques, ~3 tactics, `content_technique_tactic`
    rows including **one technique in two tactics** (the case that makes tactic rollups sum to more
    than the technique count — see `docs/analytics.md`). Small enough to add up on paper; do not
    seed real ATT&CK.
  - A **baseline engagement** in `blind` mode: 2 scenarios, ~8 steps spanning those techniques, with
    deliberately covered edge cases — at least one `skipped` execution, one still `pending`, one
    `complete` but **unscored**, one `blocked`, one detected with `detected_at` giving a known MTTD,
    one detected with **no** `started_at`, and **at least two unrevealed steps**.
  - A **retest engagement** overlapping the baseline on some techniques, with better scores, plus one
    technique the baseline never touched and one the retest dropped — so `M5-008`'s compare has
    added, removed and improved rows to find.
  - Findings across all four statuses, with the history rows `M5-003` adds.
  - `Seed(t testing.TB) Fixture` returns a struct of ids **and the expectation tables** — the
    hand-computed answers live beside the data that produces them, not scattered across test files.
- **`docs/analytics.md`** — normative definitions of every term the API and reports use: attempted,
  not-attempted, unscored, covered, the two coverage denominators, tactic double-counting, seat
  scoping. `M5-013` and M6 take their UI copy from here.

**Out**

- Any actual rollup (`M5-004`…`M5-008`).
- HTTP, OpenAPI, authz (`M5-009`).
- Exports (`M5-010`…`M5-012`).
- Making `Service.ListSteps` honour its `blind.Scope` argument (`M5-002`) — this ticket only
  *provides* the SQL predicate.

## Files

- `internal/analytics/analytics.go`, `doc.go`, `scope.go`, `sqlfragments.go`
- `internal/analytics/analyticstest/fixture.go`
- `docs/analytics.md`

## Acceptance criteria

- [ ] `Queries` takes its handle through the constructor; `go vet` and the DoD-5 review find no
      package-level DB global.
- [ ] Every exported rollup signature added later is forced through `Scope` — documented in the
      package doc as the rule, with the reason (a rollup that took a bare engagement id would compile
      while leaking a blind engagement's totals to blue).
- [ ] `outcomeCase` agrees with `scoring.DeriveOutcome` for **every** category × protection pair,
      asserted by a test that enumerates `scoring.AllCategories()` × `scoring.AllProtections()` and
      runs the SQL against the fixture — not by two lists a human compared.
- [ ] The nil cases agree too: `detection_category IS NULL` or `protection IS NULL` yields the same
      "unscored" answer in SQL as `scoring.DeriveOutcomePtr` yields in Go.
- [ ] `analyticstest.Seed` is deterministic — fixed UUIDv7 values and fixed timestamps, no
      `time.Now()`, no random ids. A fixture that changes between runs cannot have hand-computed
      expectations.
- [ ] The fixture's expectation tables are exhaustive enough that `M5-004`…`M5-008` add no new seed
      rows. If a later rollup needs new data, that is a signal this fixture missed a case — amend it
      here rather than growing a second fixture.
- [ ] `docs/analytics.md` defines every term before any endpoint uses it.
- [ ] Package doc names each DuckDB-specific construct used and what porting it would cost.

## Tests

- Fixture self-test: seeds, then asserts the row counts and the blind/unrevealed split it claims,
  so a broken fixture fails in its own package rather than as a confusing failure in a rollup test.
- Outcome drift test as above, in both directions: every Go pair reachable in SQL, every SQL result
  reachable in Go.
- `Scope.stepPredicate()` produces `TRUE` for lead/red/observer and the revealed predicate for blue
  in a blind engagement — checked against `blind.Scope.Permits` so the two fences cannot disagree
  (the property `internal/store/blind`'s own tests already hold themselves to).

## Notes for the implementer

- Read `internal/store/blind`'s package doc before writing `stepPredicate`. `Where` takes a **boolean
  column expression** and `app.step` stores `revealed_at` as a nullable timestamp, so the argument is
  a constant expression owned by this package — never a value from a request.
- `storecontent.SourceIDAttack` is the constant for the ATT&CK source id; the pinned version string
  is `engagement.attack_version` and equals `content_source_version.version` (`attackpin` package doc).
- Resist a `Service` layer. This package answers questions and returns structs; who may ask is
  `authz.Can`'s business in the one middleware that asks it.
- Reads use the read pool. Analytics never calls `store.Write` — it takes no locks and blocks no
  war room.
