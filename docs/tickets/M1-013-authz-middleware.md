# M1-013 — One authorization middleware, zero handler checks

**Milestone:** M1 · **Size:** M · **Depends on:** M1-012, M0B-005

## Why

`PLAN.md` §4: "One function … called from one middleware. **No handler makes its own role
decision.**" `M1-012` built the function; this ticket makes it structurally impossible to bypass.

## Scope

**In**

- Each operation in `api/openapi.yaml` declares its required action via a vendor extension, e.g.
  `x-authz-action: engagement.read`, plus how to locate the resource
  (`x-authz-resource: {type: engagement, from: path, param: engagementId}`).
- A generator or startup check builds the operation → action map from the spec.
- The middleware sits after authn, before handlers, in the single chain from `M0B-006`:
  1. Resolve the operation (chi route pattern → operationId).
  2. Look up the required action. **No mapping → refuse to start the server.** Fail closed at boot,
     not at request time.
  3. Load the resource's ownership facts (engagement ID, revealed state) via a small loader
     interface.
  4. Call `authz.Can`. Deny → 403, or **404 where confirming existence is itself a leak** (a
     non-member must not learn that an engagement exists). Decide per resource type and document it.
  5. Allow → handler, with the decision in the context for audit.
- **Blind mode in the query layer** (`PLAN.md` §4): repositories filter unrevealed steps for blue
  members at the repository boundary, so no endpoint can leak them even if a rule is missed. This
  ticket lands the mechanism; `M4` uses it for real.
- Deleting the per-handler check habit: a test asserting no handler package imports `authz` or
  references role constants.

**Out**

- The policy itself (`M1-012`), the matrix tests (`M1-014`).

## Acceptance criteria

- [ ] **The server refuses to start** if any non-public operation lacks an authz mapping. Add an
      operation without one and show the startup failure in the PR — this is the mechanism that
      prevents a future unprotected `/manage/access`.
- [ ] Public operations (login, SSO callbacks, healthz, version) are an explicit allowlist in the
      spec (`x-authz-public: true`), not an absence of configuration. Absence must never mean open.
- [ ] A denied request never enters the handler (assert via a handler that would panic).
- [ ] Denials return 403 with `code: "forbidden"`, or 404 where documented, and leak nothing about
      the resource.
- [ ] The decision (subject, action, resource, allow/deny, reason) is available for the activity log
      (`M1-015`).
- [ ] No handler imports `authz` or compares roles. Enforced by an automated test.
- [ ] Blind mode: a blue member's repository read returns no unrevealed steps, verified at the store
      layer independently of any HTTP test.
- [ ] Adding a new endpoint without an action mapping fails CI, not production.

## Tests

- Startup-validation test: a spec with an unmapped operation fails to build the server.
- Middleware tests: allow, deny-403, deny-404, unauthenticated, handler-never-reached.
- Import-boundary test over the handler packages.
- Store-level blind-mode filter test.

## Notes for the implementer

- Getting from a chi request to its `operationId` is the fiddly part. `kin-openapi`'s router
  (already loaded for request validation in `M0B-006`) can resolve a request to its spec operation —
  reuse it rather than maintaining a second route table.
