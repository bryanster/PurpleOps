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

- [ ] `authz` imports no database, HTTP, or store package. Enforced by a lint rule or an
      import-check test.
- [ ] Every denial carries a human-readable reason; a debug log line shows subject, action, resource
      and reason.
- [ ] Unknown action → deny. Nil/zero subject → deny. Resource with no owning engagement where one
      is required → deny.
- [ ] There is exactly **one** definition of each role in the codebase. A test greps for role string
      literals outside `authz` and fails if any exist — this is the direct fix for "two contradictory
      definitions of blue".
- [ ] Adding a new `Action` constant without adding a rule causes a test failure (exhaustiveness).
      Design the rule table so this is checkable.
- [ ] `docs/authz.md` regenerates via `make generate` and CI's drift gate covers it.

## Tests

The full matrix lives in `M1-014`. Here, cover: default deny, unknown action, reason text,
exhaustiveness, and the import boundary.

## Notes for the implementer

- Do not reach for a policy DSL (OPA, Casbin). The rule set is small and closed; a Go table is
  clearer, faster and testable to exhaustion. `PLAN.md` calls for one function, not a framework.
- If a rule needs data the `Subject`/`Resource` doesn't carry, **add it to the struct** and let the
  caller load it. Do not add I/O to `Can` — the moment it can hit the database, the exhaustive test
  becomes impossible and the whole design unwinds.
