# M5-002 — Query-layer blind fence for step reads

**Milestone:** M5 · **Size:** S · **Depends on:** M3-005

## Why

M3 debt, surfaced by M5. `internal/store/blind`'s package doc describes two fences: the policy
(`authz.GuardBlindMode`) and a query-layer filter, "so that an endpoint added without the right
action, or a rule that forgot the guard, still cannot return a step blue has not been shown."

The second fence is not there for steps. `Service.ListSteps` and `Service.ListEngagementSteps`
(`internal/engagement/steps.go:145,151`) accept a `blind.Scope`, document themselves as
"blind-filtered through scope", and **ignore the argument** — the filtering actually happens in Go at
`stepsToWire` (`internal/httpapi/stephandlers.go:368`). Correct output today, one fence instead of
two, and a signature that lies to the next caller.

M5 is the first consumer that cannot rely on the wire-layer pass: analytics aggregates in SQL, where
there is no row list left to filter. Fix the fence before building on it.

## Scope

**In**

- Push the filter into the repository: `ListByScenario` / `ListByEngagement` in
  `internal/store/engagement/steps.go` take a `blind.Scope` and apply `scope.Where(...)` over the
  revealed expression in their `WHERE` clause.
- `Service.ListSteps` / `ListEngagementSteps` pass their `scope` through instead of dropping it.
- Keep `stepsToWire`'s filter. **Both fences, deliberately** — that is the design
  `internal/store/blind` argues for, and a single fence is one edit away from not being there.
- Audit sibling reads for the same gap: any other engagement-scoped list that takes a `blind.Scope`
  and does not reach SQL with it. Fix or record why not.

**Out**

- Changing the blind *policy*, reveal rules, or seat resolution (`M3-EPIC` owns those).
- Analytics queries (`M5-001` provides their predicate separately).
- Presence and SSE filtering — already correct (`M4-004`, `M4-006`).

## Files

- `internal/store/engagement/steps.go`
- `internal/engagement/steps.go`
- `internal/httpapi/stephandlers.go` (call sites only)

## Acceptance criteria

- [x] `ListByScenario` and `ListByEngagement` cannot be called without a scope — the argument is
      required, not optional, so the next repository method added is forced to think about it.
- [x] A blue caller in a blind engagement gets unrevealed steps excluded **by the SQL**, proven by a
      store-layer test that calls the repository directly with no HTTP layer above it.
- [x] `stepsToWire` is unchanged and still filters. A test asserts belt and braces: with the store
      filter disabled in a test double, the wire layer still withholds.
- [x] Existing blind tests (`internal/httpapi/blind_integration_test.go`, the M1-014 matrix, M4-009's
      Playwright spec) stay green with no assertion changes. If one needs changing, the behaviour
      changed and that needs saying out loud.
- [x] No doc comment in `internal/engagement/steps.go` claims filtering that the code does not do.

## Implementation notes

- `ReorderSteps` passes `blind.Scope{}` (zero value = widest scope) to `ListByScenario` for
  validation, since reorder is a lead-only write operation and needs all steps. The wire layer
  (`stepsToWire`) provides the second fence on the response path.
- Audit found no other engagement-scoped lists with the same gap. Scenarios, findings, executions,
  members, comments, and evidence either do not take a `blind.Scope` or already apply it correctly.
  Single-step reads (`GetStep`) check blind scope in the handler — correct for single-row reads.
- The `listStepsByEngagement` constant was split into base + order suffix to allow the blind
  predicate to be injected between the WHERE clause and ORDER BY.
- Store-layer: blue + blind sees revealed only; blue + standard sees all; lead/red/observer see all;
  non-member has no seat and the caller above decides, per `blind.Scope`'s zero-value contract.
- The `Where` / `Permits` agreement property extended to the step repository, matching
  `TestWhereAndPermitsAgree` in `internal/store/blind`.
- Regression: the M1-014 permission matrix and the blind integration tests, unchanged.

## Notes for the implementer

- This is a fence repair, not a refactor. Resist tidying the step repository while you are in there.
- `blind.Scope`'s zero value is the **widest** scope — a non-blind engagement read by somebody with
  no seat. That is deliberate (see its doc) and is the right default here; do not "fix" it into
  hiding everything.
- If the audit finds a third place with the same gap, open a follow-up ticket rather than widening
  this one.
