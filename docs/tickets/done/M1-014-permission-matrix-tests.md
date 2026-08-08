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

- [x] Every `Action` constant appears in the matrix; adding one without an expectation fails the
      test with a message naming the missing action.
- [x] All five named regression tests exist, pass, and each fails if its rule is removed from the
      policy. **Verify each by temporarily breaking the rule** and record the results in the PR —
      a regression test that passes against the bug is worthless.
- [x] The matrix runs in under a second (it's pure functions — `M1-012` made this possible).
- [x] The HTTP sweep covers at least: read engagement, write red field, write blue field, manage
      members, manage users, sync content.
- [x] Where the answer is 404-instead-of-403, the matrix records that explicitly rather than
      treating them as interchangeable.
- [x] A short comment at the top of the file explains how to add a row, so the next person extends
      it instead of writing a one-off test.

## Tests

This ticket *is* tests. The deliverable is the file plus the demonstrated break-and-fail evidence.

## Notes for the implementer

- Keep the expectations table readable — a wall of `true`/`false` is unreviewable. Prefer named
  constants (`allow`, `deny403`, `deny404`) and group rows by action.
- If a combination is genuinely nonsensical (an engagement role without an engagement), encode it as
  an explicit `n/a` entry rather than omitting the row.

---

## Implementation notes

Three files, no production code. `docs/authz.md` already carried the rendered matrix — `M1-012`
generated it from the rule table — so nothing was needed there; `make generate` leaves the tree
clean.

| File | What it is |
|---|---|
| `internal/authz/matrix_test.go` | The checked-in expectations table and the exhaustiveness gates |
| `internal/authz/regressions_test.go` | The five named cases |
| `internal/httpapi/authzsweep_test.go` | The HTTP sweep |

### The shape of the table, and why the auth method is not a column

The full cross product is platform role × engagement seat × auth method × action × resource state.
Written out flat that is a wall of `true`/`false` nobody reviews, which the ticket warns against. So
it is split by what varies per action and what does not:

- **Per action, written out literally**: a `seats` grid — two platform roles × five seats, where the
  fifth is "holds no seat here" — for each resource state the action can be in. A platform-scoped
  action has one state; an engagement-scoped one has three (standard, blind + revealed, blind +
  unrevealed), all three written out even where identical, because "blind mode changes nothing for a
  revealed step" is a claim worth asserting per action. 46 grids, 460 cells.
- **Uniform across actions, stated once**: the auth method, as four `lenses` — a session, a token
  carrying the right scope, a token carrying a real but wrong one, and a token bound to another
  engagement. Each projects the session answer, and the projection is the invariant M1-011 is made
  of: *a fence can turn an allow into a refusal; it can never turn a refusal into an allow, and
  never softens one.* 1,840 assertions in ~10 ms, under `-race`.

Nothing in the table is computed from `authz.Rules()`. A table that derived its answers the way
`Can` does would agree with a bug. What *is* compared against the rule table is the four restated
columns (action, resource type, token scope, session-only reach) — restating cannot mean disagreeing
— and which grids a row is required to fill.

### `n/a`, used once and checked rather than skipped

The ticket asks that a nonsensical combination be an explicit `n/a` rather than an omitted row, and
names the case: an engagement role without an engagement. That is exactly the seat columns of a
platform-scoped action — `Can` cannot see a membership when the resource names no engagement. Those
40 cells are `na`, and `na` is asserted, not skipped: the answer must equal the same platform role's
no-seat answer. `TestNAIsOnlyUsedWhereTheSeatIsInvisible` confines it to that case, so it cannot
become a hole.

The caller in every cell is a *lead of a different engagement*, always. That makes the "no seat"
column mean "a member of something else" rather than "a member of nothing" — a matrix built from
somebody who belongs to no engagement at all would not notice a rule that let a seat travel.

### `active / disabled user` is not a policy dimension

The ticket lists it among the resource states. It is not one, and encoding it would have been
misleading: a disabled account never produces a `Subject` at all. Every one of the three
authentication paths refuses it first — `Service.Login`, `Service.Authenticate` (so a live session
dies at the next request, not at its expiry) and `Service.AuthenticateToken` — so the answer for a
disabled user is 401 and the policy is never asked. Covered where it belongs, at that layer, by
`TestDisablingAnAccountEndsItsSessionsNow` and `TestDisablingTheOwnerDisablesTheirTokensImmediately`.

### The HTTP sweep

Six operations as six callers, each twice — signed in, and on a service token carrying every scope.
Real accounts in a real DuckDB, real sign-ins, real session and CSRF cookies, real tokens minted
through the API. The expectations are shared between the two arrivals on purpose: none of the six
actions is session-only, so automation must reach exactly what its owner reaches.

The endpoints do not exist yet — engagements are M3, user management M1-016, content M2 — so they
are declared in a fixture specification merged into the real document, following the precedent in
`authorize_test.go`. What is under test is present today: the `x-authz-*` mapping, the middleware
reading the engagement out of the right path segment, and 403 vs 404. The user-management path is
spelled `/manage/access`, after the v1 hole. `TestTheSweepCoversTheOperationsTheTicketNames` asserts
the six actions are still driven, so the sweep cannot quietly stop sweeping.

Each row records the status literally, so `404` and `403` are different expectations:

| | admin | lead | red | blue | observer | non-member |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| `GET /engagements/{id}` | 200 | 200 | 200 | 200 | 200 | **404** |
| `PUT …/executions/{id}/red` | 200 | 200 | 200 | 403 | 403 | **404** |
| `PUT …/executions/{id}/blue` | 200 | 200 | 403 | 200 | 403 | **404** |
| `POST …/members` | 200 | 200 | 403 | 403 | 403 | **404** |
| `PATCH /manage/access/{id}` | 200 | 403 | 403 | 403 | 403 | 403 |
| `POST /content/sync` | 200 | 403 | 403 | 403 | 403 | 403 |

Beyond the status: the refusal is the documented problem shape with the right code, its body names
neither the engagement nor the rule, and the handler is proved not to have run — it sets a response
header, and a refused request must not carry it.

### Break-and-fail evidence

Each rule was removed from `internal/authz/policy.go`, the tests run, and the rule restored. Every
regression failed against its bug, and none passed.

| # | The break | Test | Failed with |
|---|---|---|---|
| 1 | `user.manage` given `Platform: everyone` | `TestRegression_NonAdminCannotReachUserManagement` | `a platform member in the lead seat, on cookie, holds user.manage … This is v1's ungated /manage/access` |
| 2 | `EngagementRoleObserver` added to `writers` | `TestRegression_ObserverCannotWrite` | `an observer holds finding.write … This is v1's Spectator, which fell through to write access` |
| 3 | `EngagementRoleBlue` added to `leadAndRed` | `TestRegression_BlueCannotWriteRedFields` | `the blue seat holds execution.write_red … This is v1's two definitions of "blue"` |
| 4 | `Guard: GuardBlindMode` deleted from both execution rules | `TestRegression_BlueCannotSeeUnrevealedStepsInBlindMode` | `blue holds execution.read on an unrevealed step of a blind engagement … This is the blind-mode leak` |
| 5 | `Can` returns allow for any token carrying the scope, before `grant` | `TestRegression_ServiceTokenCannotExceedOwnerPermissions` | `the same token still manages users after its owner was demoted … This is v1's API key: a credential that outlived the authority it was issued under` |

Breaks 1, 3 and 5 were also confirmed through HTTP — `the lead, signed in, tried to manage the
users: 200, want 403`; `a blue analyst … tried to write a red field: 200, want 403`; `somebody in no
engagement at all, with a service token, tried to read the engagement: 200, want 404` — so the sweep
is checking the wiring rather than passing alongside it. `TestPermissionMatrix` failed on all five,
which is the point of having it.

The exhaustiveness gate was verified the same way. Adding an `Action` with no matrix row fails with
`the matrix decides nothing about evidence.upload. Add an entry for it to matrix in
internal/authz/matrix_test.go`; adding one with no rule either names its position, since an action
with no rule has no wire name to print. Emptying a single cell fails with `the matrix leaves
finding.write in a standard engagement undecided for a member in the observer seat`, and dropping
one fails to compile — the grids are fixed-arity struct literals.

### Tested

`make lint test build` green, `make generate && git diff --exit-code` clean, and
`go test -race ./internal/authz/... ./internal/httpapi/` green. `TestPermissionMatrix` decides 1,840
cases in 10 ms under `-race`, against a one-second budget that is itself asserted — if `authz.Can`
ever starts doing I/O, that assertion is what says so.
