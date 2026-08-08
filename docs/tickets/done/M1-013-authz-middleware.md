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

- [x] **The server refuses to start** if any non-public operation lacks an authz mapping. Add an
      operation without one and show the startup failure in the PR — this is the mechanism that
      prevents a future unprotected `/manage/access`.
- [x] Public operations (login, SSO callbacks, healthz, version) are an explicit allowlist in the
      spec (`x-authz-public: true`), not an absence of configuration. Absence must never mean open.
- [x] A denied request never enters the handler (assert via a handler that would panic).
- [x] Denials return 403 with `code: "forbidden"`, or 404 where documented, and leak nothing about
      the resource.
- [x] The decision (subject, action, resource, allow/deny, reason) is available for the activity log
      (`M1-015`).
- [x] No handler imports `authz` or compares roles. Enforced by an automated test.
- [x] Blind mode: a blue member's repository read returns no unrevealed steps, verified at the store
      layer independently of any HTTP test.
- [x] Adding a new endpoint without an action mapping fails CI, not production.

## Tests

- Startup-validation test: a spec with an unmapped operation fails to build the server.
- Middleware tests: allow, deny-403, deny-404, unauthenticated, handler-never-reached.
- Import-boundary test over the handler packages.
- Store-level blind-mode filter test.

## Notes for the implementer

- Getting from a chi request to its `operationId` is the fiddly part. `kin-openapi`'s router
  (already loaded for request validation in `M0B-006`) can resolve a request to its spec operation —
  reuse it rather than maintaining a second route table.

---

## Implementation notes

### Where the mapping lives, and what it looks like

Each operation in `api/openapi.yaml` declares exactly one of three things. `api/authz.go`
(`Requirements`) reads them; `go test ./api` fails on a gap and so does `NewServer`.

```yaml
x-authz-action: settings.read              # the action, by its wire name in internal/authz
x-authz-resource: {type: platform}         # and what it acts on

x-authz-self: true                         # signed in, acting on your own account
x-authz-because: your own profile, and the only account this operation can name is the one that asked.

x-authz-public: true                       # no credential, no permission
x-authz-because: this endpoint issues the credential; requiring one to reach it would be a closed door with the key inside.
```

Three deviations from the ticket's sketch, each deliberate:

- **`from: path` is gone.** The ticket's example is
  `{type: engagement, from: path, param: engagementId}`. `from` had exactly one legal value, and a
  key that can only say one thing is a key nobody reads and a branch nobody exercises. What remains
  is `param` (the resource's own identifier) and `engagement` (the owning engagement's), both path
  parameter *names*, both checked against the parameters the operation actually declares.
- **`x-authz-because` is required on every exemption.** The ticket asks for `x-authz-public: true`,
  which it is. The reason beside it follows this codebase's two existing exemption lists —
  `csrfExemptRoutes` and `enrolmentOnlyRoutes` — which hold every entry to a written argument. An
  exemption nobody had to justify is one nobody reviewed.
- **`x-authz-self` is a third category the ticket does not name.** Six of the fourteen operations
  today act on the caller's own account: `/auth/me`, `/auth/password`, and the four MFA
  self-service endpoints. Mapping them to an action would have meant adding `account.read` /
  `account.manage` to `internal/authz`, and with them a service-token scope answer that belongs to
  M1-011 — the ticket puts "the policy itself" out of scope, so that was the wrong ticket to decide
  it in.

  It is not a hole. The policy question for these is a constant (everyone signed in holds them, over
  their own account) and a constant is not a decision. What would be a hole is an operation
  *acquiring* the exemption by omission, so: it is a declaration the document has to carry, the
  server checks it at boot, it needs a written reason, and
  `TestASelfOperationCannotNameAnotherResource` refuses the claim to any operation with a **path
  parameter**. `/users/{userId}` cannot claim it however the description is worded.

### `logout` is public

Not because it is unimportant, but because `POST /auth/logout` is documented to answer 204 to a
request carrying no usable session — "the caller wanted to be signed out, and they are". Requiring a
permission would turn that into a 401 and leave a browser holding a dead cookie nobody told it to
drop. Ending a session grants nothing. CSRF still applies to it.

`verifyTotp` and `verifyRecoveryCode` are public for a related reason: they carry the pending-MFA
cookie rather than a session, so there is no subject for a permission to attach to. `security` and
`x-authz-*` are separate axes, and the spec's header comment now says so.

### Resolving a request to its operation

Via the kin-openapi router the request validator already builds, as the ticket suggests. `validate.go`
now records the resolved `operationId` and path parameters on the context (`specRoute`), and the
authorization middleware reads them. There is no second route table, so there is nothing for a
second table to disagree with. A request that reaches the middleware without one is answered 500,
not allowed: on the chain this server builds it cannot happen, and the safe thing to do with a
decision you cannot make is not to serve it.

### The loader, and a second fail-closed check

`Ownership` (two methods: `Facts` and `Seat`) is the "small loader interface". It has no
implementation yet, because engagements arrive with M3 — so rather than shipping a loader that
returns "not implemented" on a reachable path, `newServer` **refuses to start** when the
specification maps any operation to an engagement-scoped resource and `Deps.Ownership` is nil. Same
discipline as a missing mapping, one level down, and it keeps the not-implemented branch from
existing at all. `TestTheServerRefusesToStartWithNothingToLoadAnEngagementFrom` covers it.

`Seat` loads **one** membership — the engagement this request named — which is what
`authn.Subject`'s comment about deliberately carrying none was reserving. Platform-scoped
operations, which is all of today's, cost no query at all.

### 403 or 404, per resource type

Decided by the policy (`authz.Decision.Conceal`, from M1-012) and merely rendered here — working it
out from the shape of the subject would be a second place making a permission decision. Documented
in `docs/http.md#authorization`:

| Resource | Refusal |
|---|---|
| Anything an engagement owns, asked about by a non-member | **404** — `PLAN.md` §4: non-members get nothing on an engagement, including its existence |
| An unrevealed step, asked about by the blue side of a blind engagement | **404** — learning a step exists is most of what blind mode withholds |
| Everything else | **403**, `code: forbidden` |

Neither body says anything about the resource or the rule; `assertRefusalLeaksNothing` checks that
against the live responses. The reason goes to the log and, when M1-015 lands, to the activity log —
it is on the context as `Authorization`, built from the same values the decision was made from.

### `authn.requireAdmin` is gone

M1-012's notes left it as the last hand-rolled role comparison in the tree, and this ticket owned
deleting it. `Service.MFAPolicy` no longer takes a subject at all; `SetMFAPolicy` still does, but
only to record who changed the policy. `settings.read` and `settings.manage` are now enforced in
front of the handler, and `TestAnOrdinaryRefusalIs403` / `TestAnAdministratorReachesTheSameEndpoint`
prove the move was a move rather than a removal.

### Blind mode in the query layer

`internal/store/blind` is the second fence: `Scope.Where("revealed")` returns the SQL predicate a
repository puts in its `WHERE` clause, and `Scope.Permits(revealed)` answers the same question for
callers that already have the row (M4's event stream). It ships with no production caller because
steps arrive with M3 — but it ships *tested*, against a real DuckDB table, rather than arriving with
them untested.

`TestTheFilterAgreesWithThePolicyAboutWhoBlueIs` walks every seat × blind combination and checks the
filter against `authz.Can`. That is the point of having two fences at all: the second one exists
because a rule might be missed, so it must not be a second, subtly different opinion about who blue
is — which is exactly the shape of v1's defining bug.

`Where` concatenates a column name into SQL. It is always a constant belonging to the repository
that owns the table; the doc comment says so, and says what it would mean if it ever were not.

### The import boundary

`TestNoHandlerDecidesForItself` finds handler files by looking for methods on `*handlers`, not by a
filename convention — so moving a handler into a new file does not move it out of the test's reach.
Go imports are per file, so this is checkable at the granularity that matters even though the
middleware and the handlers share a package. It also asserts the list is non-empty and that
`authorize.go` *does* import `internal/authz`, or the test would be satisfied by a build in which
nobody decides anything.

`TestOnlyOneFunctionInTheRepositoryAsksThePolicy` states the same rule over the whole tree: `authz.Can`
has one non-test call site, and it is the middleware.

### Evidence

Each enforcement mechanism was verified by breaking the thing it protects and confirming the
failure, then reverting.

**The startup failure, from the real binary.** `GET /manage/access` added to `api/openapi.yaml` with
no mapping — the v1 endpoint this whole mechanism is named after:

```
{"level":"INFO","msg":"applied migration","version":6,"file":"0006_platform_setting.sql"}
blacklight: httpapi: api: 1 operation(s) in openapi.yaml do not say what they require of their caller:
  - listAccess (GET /manage/access) declares no x-authz-action, and no x-authz-public or x-authz-self exemption. Absence is not permission: say which action it needs, or say why it needs none
```

It never opened a socket. The same document fails `go test ./api` first, so in practice the failure
lands on the machine of whoever added the endpoint.

| Broken | Failed with |
|---|---|
| An operation with no mapping | the startup refusal above, and `TestRequirementsRefusesAGapOrAMistake` |
| `x-authz-actoin: settings.manage` | `setMfaPolicy … declares x-authz-actoin, which is not one of x-authz-action, …` |
| `x-authz-action: settings.peek` | `requires the action "settings.peek", which internal/authz does not define` |
| `x-authz-public: false` | `Remove the key instead — an exemption that is written down and switched off reads as a decision somebody made and then hid` |
| An exemption with no `x-authz-because` | `claims an exemption with no x-authz-because` |
| `settings.read` mapped to `{type: engagement}` | `settings.read acts on a platform, and the request named a engagement` |
| `security: []` alongside an action | `an operation with no credential has no subject to authorize, so every request to it would be refused` |
| `x-authz-self` on an operation with `{engagementId}` | `an operation that can name another resource needs an action, not an exemption` |
| An engagement resource naming no engagement | `names no engagement path parameter` |
| An engagement-scoped operation with `Deps.Ownership` nil | `this server was built with no Ownership to load one from (Deps.Ownership)` |
| A denied request | never reached the handler — the handler behind that route panics, and the test sees 404 |

`make lint test build` green; `make generate` leaves the tree clean; the Playwright suite (4 tests,
including an administrator writing the MFA policy through the middleware) passes.

### What is deliberately still outstanding

- **Service tokens** (`M1-011`). `authz.Can`'s second fence is already in place and the middleware
  feeds it whatever `authn.Subject.Method` says; M1-011 supplies the tokens and the scopes.
- **The activity log** (`M1-015`). The decision is on the context as `Authorization`, with an
  unexported accessor, waiting for its one reader.
- **The permission matrix** (`M1-014`) and the `Ownership` implementation (`M3`).
