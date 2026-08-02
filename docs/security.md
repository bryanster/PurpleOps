# Security model

What this file covers is the parts of the design a reader has to understand *before* changing them,
because the failure mode is silent. Sessions, sign-in throttling and the middleware chain are in
[`docs/http.md`](http.md); this is the cross-site request forgery model, and it will grow as the
rest of M1 lands.

## CSRF

`PLAN.md` §4 records what v1 did: CSRF protection was added, then removed, and the header plumbing
was left behind. That is the worst of the three possible states, because the code still looks
protected. v2 has two layers, and both, or neither, is the choice — neither is not an option.

### The two layers

**`SameSite=Strict` on the session cookie.** A modern browser does not attach it to a request
another site caused, which on its own defeats almost every version of this attack. It is set in
`internal/authn/session/cookie.go` alongside `HttpOnly` and `Secure`.

**A double-submit token.** The server issues a second cookie, `pops_csrf`, and a state-changing
request must echo its value in the `X-CSRF-Token` header. It covers what `SameSite` does not: older
clients, and a request from a sibling origin on the same site — a neighbouring subdomain, or an
`http` sibling of an `https` origin — which is same-site as far as the cookie is concerned.

### The token is derived, not stored

`pops_csrf` carries `HMAC-SHA256(PURPLEOPS_SESSION_SECRET, "purpleops/csrf\0" || session token)`,
base64url. There is no column and no migration, and three things follow from it:

- **It rotates with the session.** Rotation replaces the session token (`docs/http.md`, *Sessions*),
  so the CSRF token changes with it and nothing has to remember to update anything.
- **The server can recompute what the header should be**, rather than only checking the header
  against the cookie. This is the difference from naive double-submit: an attacker who can *write* a
  cookie for this host can make the header and the cookie agree with each other, but not with a
  value keyed by a secret they do not have.
- **It is not the value stored in `session.token_hash`.** The domain separator is what keeps the two
  derivations apart. Without it, a cookie deliberately readable by script would carry the database's
  lookup key for a live session. `TestTheCSRFTokenIsNotTheStoredHash` is there because that line
  looks redundant.

The CSRF token is not a credential: on its own it authorizes nothing. That is why the cookie has no
expiry — a stale copy fails the check and is replaced on the way out, and a refusal carries a fresh
one, so a browser that lost the cookie recovers by retrying instead of by signing in again.

### The two cookies

| | `pops_session` | `pops_csrf` |
|---|---|---|
| `HttpOnly` | **yes** — script must never read the session token | **no** — script must read it, or no request could carry the header |
| `Secure` | yes, except `PURPLEOPS_ENV=development` | the same |
| `SameSite` | `Strict` | `Strict` |
| `Path` / `Domain` | `/` / none | the same |
| Expiry | the session's absolute expiry | none; the value is checked against a derivation |

Getting the `HttpOnly` flags the wrong way round is the classic version of this mistake, and it
would leave a test that checks either cookie alone still passing.
`TestTheCookieFlagsAreNotSwappedOver` asserts both.

### What is checked, and what is exempt

`requireCSRF` (`internal/httpapi/csrf.go`) is step 10 of the middleware chain. For a state-changing
request that authenticated **by cookie**, the `X-CSRF-Token` header must equal the `pops_csrf`
cookie *and* equal the value derived from the session token — both comparisons in constant time.
Anything else is a `403` with `code: "forbidden"`, written before the handler is entered.

| Exempt | Why |
|---|---|
| `GET`, `HEAD`, `OPTIONS` | They change nothing. A handler that mutates on `GET` is a bug in that handler |
| A request that did not authenticate by cookie | Nothing was attached on the caller's behalf, so there is nothing to forge |
| `POST /auth/login` | There is no session to protect yet; the credential in the body is the proof |

The second row is the one to be careful with. The exemption is read from `authn.Subject.Method`,
which the *authentication* step sets from what it actually resolved — so sending an
`Authorization: Bearer` header buys a caller nothing. An invalid one authenticates nobody, and a
request that still holds a session cookie is checked exactly as it would have been. Service tokens
(`M1-011`) are exempt because a browser never attaches one; `PLAN.md` §4 says the same.

Exemptions live in `csrfExemptRoutes`, one route at a time, each with its reason. There are no
patterns and no prefixes: a wildcard is how a route nobody meant to exempt becomes exempt.

### What stops this decaying again

`TestEveryMutatingRouteIsCoveredByCSRF` walks the real router, and for every state-changing route it
finds it requires an entry in `csrfCoverage` saying whether the route is protected or exempt — then
sends a real cookie-authenticated request with no token and checks the answer. Adding a mutating
endpoint fails that test until somebody decides which it is. Removing the middleware from the chain
fails eight tests.

### The client

`web/src/api/client.ts` attaches the header in `openapi-fetch` middleware. No component, hook or
mutation refers to CSRF, and there is no call site that could forget: the one thing that could go
wrong is forgetting, so there is nothing to forget.

The header is declared in `api/openapi.yaml` as the `CSRFToken` parameter, on every operation that
needs it — deliberately as `required: false`. Declaring it required would make an *absent* header a
`400` from the request validator and a *wrong* one a `403` from this middleware, which is one rule
answered two ways. It is enforced in one place instead, and the parameter is documentation.
