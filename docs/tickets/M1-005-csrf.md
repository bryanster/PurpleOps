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

- [ ] A cookie-authenticated `POST` with no `X-CSRF-Token` is **403** with `code: "forbidden"`, and
      the handler is not entered.
- [ ] Mismatched token → 403. Matching → success.
- [ ] A `GET` needs no token.
- [ ] A request authenticated by service token succeeds with no CSRF token, and **cannot** be
      spoofed into exemption by a client that merely sends an `Authorization` header with an invalid
      value (that should be 401, not an exemption).
- [ ] Comparison is constant-time.
- [ ] The CSRF cookie is not `HttpOnly` (deliberately) but **is** `Secure` + `SameSite=Strict`, and
      the *session* cookie remains `HttpOnly`. A test asserts both, because getting these backwards
      is the classic mistake.
- [ ] Every state-changing route is covered — add a test that enumerates the routes from the
      generated router and fails if a mutating route isn't behind the middleware. This is what stops
      the protection quietly decaying again.
- [ ] The SPA works end to end with no manual header handling in any component.

## Tests

- Middleware tests for each acceptance bullet.
- The route-enumeration coverage test described above (the important one).
- A frontend test asserting the client middleware attaches the header.
