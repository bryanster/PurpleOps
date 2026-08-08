# M1-005 — CSRF protection for cookie sessions

**Milestone:** M1 · **Size:** M · **Depends on:** M1-003, M0B-009

## Why

`PLAN.md` §4: CSRF was "added then removed" in v1, leaving "vestigial header plumbing" — the worst
outcome, because the code looks protected. The plan specifies `SameSite=Strict` **plus** a
double-submit token on state-changing routes, "properly wired this time".

`SameSite=Strict` alone is close to sufficient in modern browsers, but the double-submit layer costs
little and covers older clients and same-site subdomain scenarios. Both, or neither — and neither
isn't an option.

## Scope

**In**

- Token issuance: on session creation, a random CSRF token set in a **non-`HttpOnly`** cookie
  (the JS must read it) alongside the session cookie, and returned by `GET /auth/me`.
- Verification middleware: for every `POST`/`PUT`/`PATCH`/`DELETE` authenticated **by cookie**, the
  `X-CSRF-Token` header must match the cookie value (constant-time compare).
- **Exempt requests authenticated by service token** (`Authorization: Bearer`) — CSRF does not apply
  to token auth (`PLAN.md` §4). The exemption must key off *how the request authenticated*, not off
  the presence of a header an attacker could add.
- Exempt: `GET`/`HEAD`/`OPTIONS`, login (no session yet), and the SSO callback endpoints — each
  exemption listed explicitly in code with a comment, no wildcard patterns.
- Frontend: `openapi-fetch` middleware (`M0B-009`) attaches the header automatically. No component
  ever thinks about CSRF.
- Token rotates with the session (`M1-003`).
- `docs/security.md` section explaining the model.

**Out**

- Per-form/per-request one-time tokens. Overkill here.

## Acceptance criteria

- [x] A cookie-authenticated `POST` with no `X-CSRF-Token` is **403** with `code: "forbidden"`, and
      the handler is not entered.
- [x] Mismatched token → 403. Matching → success.
- [x] A `GET` needs no token.
- [x] A request authenticated by service token succeeds with no CSRF token, and **cannot** be
      spoofed into exemption by a client that merely sends an `Authorization` header with an invalid
      value (that should be 401, not an exemption).
- [x] Comparison is constant-time.
- [x] The CSRF cookie is not `HttpOnly` (deliberately) but **is** `Secure` + `SameSite=Strict`, and
      the *session* cookie remains `HttpOnly`. A test asserts both, because getting these backwards
      is the classic mistake.
- [x] Every state-changing route is covered — add a test that enumerates the routes from the
      generated router and fails if a mutating route isn't behind the middleware. This is what stops
      the protection quietly decaying again.
- [x] The SPA works end to end with no manual header handling in any component.

## Tests

- Middleware tests for each acceptance bullet.
- The route-enumeration coverage test described above (the important one).
- A frontend test asserting the client middleware attaches the header.

---

## Implementation notes

**`api/openapi.yaml` · `internal/authn/session/csrf.go` · `internal/httpapi/csrf.go` ·
`internal/authn/subject.go` · `web/src/api/client.ts` · `docs/security.md`**

### The token is derived from the session token, not stored

`HMAC-SHA256(BLACKLIGHT_SESSION_SECRET, "blacklight/csrf\0" || token)`. No column, no migration, and
it rotates with the session for free — rotation replaces the token it is derived from, so there is
nothing to remember to update.

The reason it is not simply a second random string is the ticket's own *Why*: the double-submit
layer is there to cover "same-site subdomain scenarios", and **naive double-submit does not cover
them**. An attacker who can write a cookie for this host — a neighbouring subdomain, an `http`
sibling of an `https` origin — can make the header and the cookie agree with each other. Deriving
the value means the server can recompute what the header *should* be, so both comparisons happen:
header against cookie (the double-submit the ticket specifies) and header against the derivation
(what an injected cookie fails). `TestOnlyTheRightTokenIsAccepted` has a case for each attacker.

The domain separator is load-bearing and looks redundant, so it has its own test. Without it the
derivation would equal `session.token_hash` — and that value would then be sitting in a cookie
deliberately readable by script, which is the database's lookup key for a live session.
`TestTheCSRFTokenIsNotTheStoredHash`.

### The cookie is issued by the middleware, not by the handlers

`csrfWriter` wraps the response: when a handler sets or clears the session cookie, the matching CSRF
cookie is set or cleared beside it, derived from the very token going into the response.

Two reasons, one mechanical and one not. The mechanical one is that the generated response types
carry a single `Set-Cookie` **string** (`w.Header().Set`), and two cookies cannot be folded into one
header — a handler cannot emit both without hand-writing a response type outside the generator. The
other is that `M1-006`, `M1-009` and `M1-010` all issue sessions, and a handler that forgets leaves
a browser unable to make any state-changing request at all. Doing it here means every one of them is
correct without knowing this exists.

The same wrapper repairs a browser whose cookie is missing or stale, including on the 403 itself —
so a client that lost it retries successfully instead of being stuck until it signs in again. That
is also why the CSRF cookie has **no expiry**: it is not a credential, a stale copy fails the
derivation check rather than authorizing anything, and it is replaced on the way out.

### The exemption keys off `authn.Subject.Method`

`Subject` gains a `Method` (`none` / `cookie` / `service_token`), set by `Service.Authenticate`
where the cookie was actually resolved. The CSRF check applies when it is `cookie` — so an
`Authorization` header buys a caller nothing, which is the acceptance criterion. Nothing can produce
`service_token` until `M1-011`, so the exemption is unreachable today and fails closed; the
middleware is tested against the subject that step will produce, and the spoofing attempt is tested
through the real server.

### The spec parameter is deliberately `required: false`

The document already promised that `X-CSRF-Token` would be declared as a parameter on the operations
that need it, and it now is. It is **not** marked required, and the parameter's own description says
why: `kin-openapi` validates parameters at step 7 of the chain, so a required header would make an
*absent* token a `400 validation_failed` and a *wrong* one a `403 forbidden` — one rule answered two
ways, and not the answer the acceptance criteria ask for. The rule lives in one middleware.

### A dependency M1-003 declined

Declaring the first header parameter makes oapi-codegen emit binding code, which takes
`github.com/oapi-codegen/runtime` (and `nullable`, `go-jsonmerge` with it) — the dependency
`M1-003`'s notes turned down for `format: uuid`. It is taken here because the alternative is
dropping the parameter from the document, and because `components/parameters/Limit` and `Cursor` are
already declared and will take it in M2 regardless. The generated `LogoutParams.XCSRFToken` is never
read by a handler; enforcement is the middleware's.

### Testing notes

- **Constant time** is asserted by parsing `csrf.go` (`TestTheCSRFComparisonIsConstantTime`), not by
  timing. A 43-byte comparison is far too fast to time without flaking; what the test actually
  guards against is somebody simplifying `csrfMatches` into `header == expected`, and an AST check
  catches exactly that.
- **The route-enumeration test** walks the real router and requires every state-changing route to
  have an entry in `csrfCoverage` — so adding a mutating endpoint fails the test until its author
  says whether it is protected or exempt. Removing `requireCSRF` from the chain fails eight tests.
- `authServer.post` now attaches the CSRF cookie and header, because that is what a signed-in
  browser sends; `csrf_test.go` builds its requests by hand, which is the point of it.
- **The SPA half** is the `openapi-fetch` middleware plus four tests in `web/src/api/client.test.ts`.
  There is no end-to-end Playwright case because there is no login UI until `M1-017`; the flow was
  driven by hand instead — `blctl user create`, login (both cookies set, `csrfToken` in the body),
  `GET /auth/me` with no header, a password change refused twice and then accepted, the CSRF cookie
  rotating with the session, logout with the rotated pair, and the replay coming back 401.

### One judgement call worth flagging

`POST /auth/login` is exempt **even when the browser holds a session cookie**. The ticket lists login
as exempt "(no session yet)", which is the usual case; keying the exemption on the cookie's absence
instead would mean a login form that works or returns 403 depending on whether a stale cookie
happens to be there. What that leaves on the table is login CSRF — forcing a victim into the
attacker's account — which costs the attacker their own credentials and is not what this ticket is
about. It is a route in `csrfExemptRoutes` with the argument written next to it.

### Verified

`make lint test build` green, `make test-spa` green, `make generate` idempotent.
