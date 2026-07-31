# M1-014 — Full permission matrix tests

**Milestone:** M1 · **Size:** M · **Depends on:** M1-012, M1-013, M1-011

## Why

`PLAN.md` §9 asks for "the full (platform role × engagement role × action × resource) matrix
asserted in one table, with named regression cases". Every v1 authorization failure was individually
obvious in hindsight; what was missing was a single artifact that says *for every combination, this
is the answer*. That artifact is this ticket, and it is the thing that keeps the model honest as
M2–M6 add endpoints.

## Scope

**In**

- One table-driven test enumerating **every** combination of:
  - platform role: `admin`, `member`
  - engagement role: `lead`, `red`, `blue`, `observer`, *non-member*
  - auth method: session, service token (with and without the relevant scope)
  - every `Action` constant
  - relevant resource states: own engagement / other engagement, revealed / unrevealed step,
    standard / blind mode, active / disabled user
- **Exhaustiveness enforcement**: the test fails if a combination is unlisted, so adding an `Action`
  forces a decision instead of an accidental default. A generated skeleton plus a checked-in
  expectations table is fine — the point is that silence is impossible.
- The five **named regression cases** from `PLAN.md` §9, each its own test function named after the
  bug:
  1. `TestRegression_NonAdminCannotReachUserManagement` — the `/manage/access` hole.
  2. `TestRegression_ObserverCannotWrite` — the Spectator fall-through.
  3. `TestRegression_BlueCannotWriteRedFields` — and the reverse.
  4. `TestRegression_BlueCannotSeeUnrevealedStepsInBlindMode`.
  5. `TestRegression_ServiceTokenCannotExceedOwnerPermissions`.
- An **HTTP-level** sweep complementing the unit-level matrix: for a representative resource, drive
  real requests as each role and assert the status codes. The unit matrix proves the policy; this
  proves the wiring.
- `docs/authz.md` gains the rendered matrix (generated, per `M1-012`).

**Out**

- New policy rules. If the matrix reveals a rule is wrong or missing, fix it in `M1-012` and
  reference it here.

## Acceptance criteria

- [ ] Every `Action` constant appears in the matrix; adding one without an expectation fails the
      test with a message naming the missing action.
- [ ] All five named regression tests exist, pass, and each fails if its rule is removed from the
      policy. **Verify each by temporarily breaking the rule** and record the results in the PR —
      a regression test that passes against the bug is worthless.
- [ ] The matrix runs in under a second (it's pure functions — `M1-012` made this possible).
- [ ] The HTTP sweep covers at least: read engagement, write red field, write blue field, manage
      members, manage users, sync content.
- [ ] Where the answer is 404-instead-of-403, the matrix records that explicitly rather than
      treating them as interchangeable.
- [ ] A short comment at the top of the file explains how to add a row, so the next person extends
      it instead of writing a one-off test.

## Tests

This ticket *is* tests. The deliverable is the file plus the demonstrated break-and-fail evidence.

## Notes for the implementer

- Keep the expectations table readable — a wall of `true`/`false` is unreviewable. Prefer named
  constants (`allow`, `deny403`, `deny404`) and group rows by action.
- If a combination is genuinely nonsensical (an engagement role without an engagement), encode it as
  an explicit `n/a` entry rather than omitting the row.
