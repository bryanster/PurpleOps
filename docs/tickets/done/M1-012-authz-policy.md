# M1-012 — Central `authz.Can` policy engine

**Milestone:** M1 · **Size:** L · **Depends on:** M1-001, M1-003

## Why

This is the most important ticket in M1. `PLAN.md` §4 is unambiguous:

> Every failure in §12 of `CURRENT.md` traces to permission logic scattered across handlers.

v1 shipped `/manage/access` without an admin gate (any user could make themselves Admin), let
Spectators fall through to write access, and had **two contradictory definitions of "blue"**. Every
one of those is a consequence of each handler deciding for itself. So: one function, one place,
and handlers that literally cannot make a role decision because they aren't given the information
to make one.

## Scope

**In**

- `internal/authz` with a single entry point:
  ```
  Can(ctx, subject Subject, action Action, resource Resource) Decision
  ```
  - `Subject` — user ID, platform role, engagement memberships, auth method, token scopes,
    `mfa_satisfied`. Built once by the authn middleware; never re-fetched inside `Can`.
  - `Action` — a closed enum of verbs (`engagement.read`, `execution.write_red`,
    `execution.write_blue`, `member.manage`, `user.manage`, `content.sync`, `report.publish`, …).
    Constants, not strings at call sites.
  - `Resource` — type + ID + owning engagement ID (+ any attribute a rule needs, e.g. whether a
    step is revealed).
  - `Decision` — allow/deny **plus a reason**, so denials are debuggable and loggable.
- The policy is **data**: a table of rules the code evaluates, not a tree of `if` statements. A
  reviewer must be able to read the permission model in one screen.
- Pure function: no I/O, no database, no `context` values beyond what's passed. This is what makes
  the exhaustive matrix test in `M1-014` fast and total.
- Default deny. An unknown action or a resource with no matching rule is denied, and that is tested.
- Rules to encode, at minimum:
  - Platform `admin` may manage users, platform settings, and content sources.
  - Platform `member` may not — **this is the `/manage/access` regression**.
  - Engagement `lead` manages membership and settings for **their** engagement only.
  - `red` writes red execution fields; `blue` writes blue detection fields; neither writes the
    other's — enforced structurally by separate actions (`PLAN.md` §4).
  - `observer` reads and comments, **never writes** — this is the Spectator regression.
  - Non-members get nothing on an engagement, including its existence.
  - Blind mode: `blue` cannot read an unrevealed step (also enforced in the query layer — see
    `M1-013`; belt and braces is correct here).
  - Service tokens: scope ∩ owner's permissions (`M1-011`).
- `docs/authz.md` — a table of role × action, generated from the rule data so it cannot drift.

**Out**

- Custom/user-defined roles. Fixed role sets in v1.
- The middleware that calls this (`M1-013`) and the matrix tests (`M1-014`).

## Acceptance criteria

- [x] `authz` imports no database, HTTP, or store package. Enforced by a lint rule or an
      import-check test.
- [x] Every denial carries a human-readable reason; a debug log line shows subject, action, resource
      and reason.
- [x] Unknown action → deny. Nil/zero subject → deny. Resource with no owning engagement where one
      is required → deny.
- [x] There is exactly **one** definition of each role in the codebase. A test greps for role string
      literals outside `authz` and fails if any exist — this is the direct fix for "two contradictory
      definitions of blue".
- [x] Adding a new `Action` constant without adding a rule causes a test failure (exhaustiveness).
      Design the rule table so this is checkable.
- [x] `docs/authz.md` regenerates via `make generate` and CI's drift gate covers it.

## Tests

The full matrix lives in `M1-014`. Here, cover: default deny, unknown action, reason text,
exhaustiveness, and the import boundary.

## Notes for the implementer

- Do not reach for a policy DSL (OPA, Casbin). The rule set is small and closed; a Go table is
  clearer, faster and testable to exhaustion. `PLAN.md` calls for one function, not a framework.
- If a rule needs data the `Subject`/`Resource` doesn't carry, **add it to the struct** and let the
  caller load it. Do not add I/O to `Can` — the moment it can hit the database, the exhaustive test
  becomes impossible and the whole design unwinds.

---

## Implementation notes

### The role vocabulary moved out of the store

`PlatformRole`, `EngagementRole` and their constants lived in `internal/store/identity`. Acceptance
criterion 1 forbids `authz` from importing a store package and criterion 4 requires exactly one
definition of each role, so the two could not both hold with the types where they were. They now
live in `internal/authz` (`role.go`) and `internal/store/identity` imports them. Where a role is
persisted is a detail; what it means is the policy's business.

`authn.Method` came with them, for the same reason at smaller scale: `Can` reads it (a service token
is fenced by its scopes as well as by its owner's role) and the CSRF middleware reads it, and one
distinction deserves one definition. `internal/authn` no longer declares it.

Call sites were updated mechanically — `identity.PlatformRoleAdmin` → `authz.PlatformRoleAdmin`, and
so on. No compatibility aliases were left behind.

### Action is an integer enum, not a string

Criterion 5 asks that adding an `Action` without a rule fail a test, and the design that makes this
airtight rather than best-effort is an `iota` block ending in an unexported `numActions` sentinel.
Adding a constant moves the sentinel, `Actions()` grows, and `TestEveryActionHasExactlyOneRule`
fails naming the uncovered value. `ActionUnknown` is the zero value, so a struct field nobody filled
in cannot mean the first real action.

The wire spelling (`engagement.read`) lives in the rule row, so an action has one name attached to
the rule that defines it, and `ParseAction` — which M1-013's `x-authz-action` resolves through — is
built from the same table.

### Action set

The ticket's named verbs plus the ones M1-013, M1-015 and M1-016 need immediately: 20 in total, 8
platform-scoped and 12 engagement-scoped. Later milestones add their own as they add endpoints,
which is a one-line table edit that the exhaustiveness test forces them to make.

### Two things the ticket did not ask for

- **`Decision.Conceal`.** M1-013 has to decide 403-vs-404, and the policy is what knows *why* it
  refused — a non-member, or blue facing an unrevealed step. Deciding it there would be a second
  place making a permission decision. The flag is set here and M1-013 reads it.
- **Token scopes are named in the rule.** Every rule declares the `TokenScope` a service token needs
  for it, so M1-011's "scope ∩ owner's permissions" is already the shape of `Can`: the role fence
  runs first, then the scope fence. `TokenScopes()` renders the list M1-011 puts in the spec. The
  vocabulary follows M1-011's sketch, with `reports:write` and `admin:read`/`admin:write` added
  because publishing and administration needed one; M1-011 owns the final wire list.

### `mfa_satisfied` is carried and read by no rule

`Subject` carries it because the ticket lists it and because the audit line for a decision should
record how strong the session that got it was. No rule *reads* it, deliberately: whether a factor is
*required* of this person is a fact the `Subject` does not carry, and enforcing a requirement
against half of it is precisely how v1 shipped an MFA setting that let anyone who skipped enrolment
in with a password alone. The requirement is enforced in one place — M1-008's gate, ahead of this.

### Blind mode binds to the seat, not to the grant

`GuardBlindMode` withholds an unrevealed step from anybody whose *engagement role* is `blue`,
including a platform administrator who is also a blue member of that engagement. An administrator
who is not a member is unaffected. Taking the blue chair is taking the blue chair; an administrator
who wants the unblinded view can have it by not sitting in it. The guard covers
`execution.write_blue` as well as `execution.read`, because a write that succeeds confirms the step
exists just as a read does.

### What is deliberately still outstanding

`authn.requireAdmin` (`internal/authn/mfapolicy.go`) is still a hand-rolled platform-role comparison
guarding the settings endpoints. It is the last one in the tree, its comment says so, and M1-013
owns deleting it — replacing it here would mean building M1-013's `authn.Subject` → `authz.Subject`
mapping in the wrong ticket. `settings.read` and `settings.manage` exist in the table waiting for it.

### Evidence

Each enforcement mechanism was verified by breaking the thing it protects and confirming the
failure, then reverting:

| Broken | Failed with |
|---|---|
| Added `ActionEvidenceUpload` with no rule | `no rule covers action(21) — add a row to the table in internal/authz/policy.go` |
| `const sneakyRole = "blue"` in `internal/cli/user.go` | `"blue" is the engagement role and must come from internal/authz, not from a literal` |
| `import "net/http"` in `authz` | `internal/authz imports "net/http" … if it is a database, a transport or a clock, it does not belong here` |
| Edited a cell of `docs/authz.md` by hand | `docs/authz.md is not what the rule table renders. Run 'make generate'` |
| Gave `finding.write` to `observer` | `finding.write grants to observer — an observer reads and comments and writes nothing else` |
| Made the non-member denial an ordinary 403 | `the denial is not concealed, so M1-013 would answer 403 and confirm … exists` |

Importing `internal/store/identity` into `authz` is additionally impossible rather than merely
tested: `identity` imports `authz` for the role types, so the compiler reports an import cycle.

`make lint test build` green; `make generate` leaves the tree clean.
