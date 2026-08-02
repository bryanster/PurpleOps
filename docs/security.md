# Security model

What this file covers is the parts of the design a reader has to understand *before* changing them,
because the failure mode is silent. Sessions, sign-in throttling and the middleware chain are in
[`docs/http.md`](http.md); this is the cross-site request forgery model and the multi-factor one,
and it will grow as the rest of M1 lands.

## CSRF

`PLAN.md` §4 records what v1 did: CSRF protection was added, then removed, and the header plumbing
was left behind. That is the worst of the three possible states, because the code still looks
protected. v2 has two layers, and both, or neither, is the choice — neither is not an option.

### The two layers

**`SameSite=Strict` on the session cookie.** A modern browser does not attach it to a request
another site caused, which on its own defeats almost every version of this attack. It is set in
`internal/authn/session/cookie.go` alongside `HttpOnly` and `Secure`.

**A double-submit token.** The server issues a second cookie, `bl_csrf`, and a state-changing
request must echo its value in the `X-CSRF-Token` header. It covers what `SameSite` does not: older
clients, and a request from a sibling origin on the same site — a neighbouring subdomain, or an
`http` sibling of an `https` origin — which is same-site as far as the cookie is concerned.

### The token is derived, not stored

`bl_csrf` carries `HMAC-SHA256(BLACKLIGHT_SESSION_SECRET, "blacklight/csrf\0" || session token)`,
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

| | `bl_session` | `bl_csrf` |
|---|---|---|
| `HttpOnly` | **yes** — script must never read the session token | **no** — script must read it, or no request could carry the header |
| `Secure` | yes, except `BLACKLIGHT_ENV=development` | the same |
| `SameSite` | `Strict` | `Strict` |
| `Path` / `Domain` | `/` / none | the same |
| Expiry | the session's absolute expiry | none; the value is checked against a derivation |

Getting the `HttpOnly` flags the wrong way round is the classic version of this mistake, and it
would leave a test that checks either cookie alone still passing.
`TestTheCookieFlagsAreNotSwappedOver` asserts both.

### What is checked, and what is exempt

`requireCSRF` (`internal/httpapi/csrf.go`) is step 10 of the middleware chain. For a state-changing
request that authenticated **by cookie**, the `X-CSRF-Token` header must equal the `bl_csrf`
cookie *and* equal the value derived from the session token — both comparisons in constant time.
Anything else is a `403` with `code: "forbidden"`, written before the handler is entered.

| Exempt | Why |
|---|---|
| `GET`, `HEAD`, `OPTIONS` | They change nothing. A handler that mutates on `GET` is a bug in that handler |
| A request that did not authenticate by cookie | Nothing was attached on the caller's behalf, so there is nothing to forge |
| `POST /auth/login` | There is no session to protect yet; the credential in the body is the proof |
| `POST /auth/mfa/totp/verify` | The other half of the same sign-in; the credential is the pending cookie plus a code from a device |

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

---

## Multi-factor authentication

`PLAN.md` §4 records what v1 did with MFA: it could be turned on and then skipped, because "this
person is required to have a second factor" and "this person has one" were the same fact. v2 keeps
three facts apart, and they are three different rows:

| Fact | Where it lives | What it means |
|---|---|---|
| `enforced` | `app."user".mfa_enforced` | An administrator requires a second factor of this person |
| `enrolled` | `app.user_totp.confirmed_at` | This person has an authenticator that works |
| `satisfied` | `app.session.mfa_satisfied` | *This session* presented one |

`GET /auth/me` reports all three. Conflating any two of them is the hole; `M1-008` is the ticket
that turns `enforced` into a refusal, and it can only do that because the three are separate here.

### The pending state

A correct password against an account with a **confirmed** authenticator does not produce a session.
It produces a *challenge*: a row in `app.mfa_challenge` and a `bl_mfa` cookie, and nothing else.

The distinction is enforced in the plumbing rather than in the logic. `internal/authn/challenge` is
a separate package from `internal/authn/session`, with a separate cookie name, a separate table and
a separate HMAC domain; the authentication middleware does not resolve one, so a request carrying
only `bl_mfa` is anonymous everywhere. `POST /auth/mfa/totp/verify` is the one endpoint that
looks at it, and the only thing it can produce is a session.

Three properties, each enforced by a statement rather than by a caller remembering:

- **It expires** — five minutes by default (`BLACKLIGHT_MFA_PENDING_TTL`), checked against the row.
- **It is spent by use** — `UPDATE ... WHERE consumed_at IS NULL`, so one correct code buys exactly
  one session even if two requests arrive together.
- **It is superseded** — opening a challenge deletes whatever that person had pending, so an
  abandoned sign-in is not still answerable while its owner starts another.

An **unconfirmed** enrolment gates nothing at all. Somebody who scans the QR code and closes the tab
has changed nothing about their account, which is what stops a half-finished enrolment from being a
lockout.

### Codes, and using one twice

Thirty seconds, six digits, SHA-1 — the parameters every authenticator app implements, fixed rather
than configurable. The accepted window is the current step and one either side: thirty seconds of
tolerance in each direction, which covers a drifted phone clock and a slow typist. Two steps is
refused.

RFC 6238 says nothing about replay, and neither do the libraries, so `app.user_totp.last_used_step`
is ours: a code is accepted only when its step is *after* the last one accepted, and the check is
`UPDATE ... WHERE last_used_step < ?` rather than a read followed by a write. Presenting the same
six digits twice inside their own thirty seconds fails the second time, and so does an older code
somebody captured earlier.

Verification failures are rationed by the same limiter as password failures (`docs/http.md`,
*Sign-in throttling*), keyed on the account behind the pending state. That matters more than it
looks: a login that answers `mfa_required` is deliberately counted as **neither** a success nor a
failure, because counting it as a success would clear the account's failure budget — and somebody
who already holds the password could then sign in again between every guess and never run out.

Every way of failing verification is one answer: a wrong code, a spent one, an expired challenge and
no challenge at all produce byte-identical bodies. "That code was right but stale" is a much smaller
search space than "no".

### The secret at rest

The shared secret has to be readable — it is checked, not verified against a hash — so it is
encrypted rather than digested: AES-256-GCM under a key derived from `BLACKLIGHT_ENCRYPTION_KEY` by
HKDF-SHA256, with a fresh 12-byte nonce per record stored in front of the ciphertext. A copy of the
database is not a set of working authenticators.

`BLACKLIGHT_ENCRYPTION_KEY` is deliberately **not** `BLACKLIGHT_SESSION_SECRET`, and the server
refuses to start if they are the same value. Rotating the session secret is the documented way to
sign everybody out; if the two were shared, that lever would also make every enrolled authenticator
undecryptable, and the only symptom would be everyone's codes failing at once.
[`docs/deploy.md`](deploy.md) has the operator's version, including what to back up.

The secret appears in exactly one response — the enrolment that mints it — and in no log line.
`TestTheSecretAppearsOnceAndNeverAgain` checks both, and that what is on disk is not the base32
string.
