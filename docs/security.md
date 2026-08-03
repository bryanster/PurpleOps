# Security model

What this file covers is the parts of the design a reader has to understand *before* changing them,
because the failure mode is silent. Sessions, sign-in throttling and the middleware chain are in
[`docs/http.md`](http.md); **who may do what** is [`docs/authz.md`](authz.md), which is generated
from the rule table and is the whole of the permission model. This is the cross-site request forgery
model and the multi-factor one, and it will grow as the rest of M1 lands.

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
| `enforced` | `app."user".mfa_enforced` | An administrator requires a second factor of *this person specifically* |
| `required` | computed | A second factor is required of them at all — `enforced`, or the platform policy |
| `enrolled` | `app.user_totp.confirmed_at` | This person has an authenticator that works |
| `satisfied` | `app.session.mfa_satisfied` | *This session* presented one |

`GET /auth/me` reports all four, alongside `recoveryCodesRemaining` — a count of the ways back in
that are left, so an interface can warn somebody before the last one is spent. Conflating any two of
them is the hole. An interface deciding whether to block reads `required`, never `enforced`:
somebody can be required by the platform policy with the flag off.

### Enforcement

The platform policy is two booleans in `app.platform_setting`, read and written by administrators at
`GET`/`PUT /settings/mfa`:

| Setting | Effect |
|---|---|
| `mfa.required_for_all` | Every account that signs in with a local password must hold a second factor |
| `mfa.required_for_admins` | Every account with the `admin` platform role must |

The effective requirement for one person is the **or** of those and their own `mfa_enforced` flag.
Neither is a default: a database nobody has configured has no rows here and requires nothing, so a
fresh installation does not confine its first administrator to enrolling before they have seen the
product.

**Policy is evaluated before enrolment is looked at.** That order is the whole fix. v1 asked "have
they enrolled?" and enforced only if the answer was yes, which made enrolment optional by
construction — the people who skipped it were exactly the people enforcement stopped applying to.
What a sign-in does now, once the password is right:

| Required | Enrolled | Outcome |
|---|---|---|
| no | no | An ordinary session |
| no | yes | A challenge — a factor somebody set up is asked for whether or not anybody requires it |
| yes | yes | A challenge |
| yes | **no** | A session confined to enrolment |

A **confined session** is a real session with a real cookie that may reach exactly four routes:
`POST /auth/mfa/totp/enroll`, `POST /auth/mfa/totp/confirm`, `GET /auth/me` and `POST /auth/logout`.
Everything else is `403` with `code: "mfa_enrolment_required"`, from one middleware in front of all
of them (`internal/httpapi/mfagate.go`) rather than from a check each endpoint remembers to make.
Confirming an enrolment marks the session satisfied and rotates it onto a new token in the same
exchange, so there is no second sign-in.

The requirement is evaluated **per request**, not recorded on the session at sign-in. Three
consequences, and they are the point:

- Turning a requirement on reaches everybody already signed in, at their next request. There is no
  set of sessions grandfathered under the old policy.
- A session that never presented a factor, belonging to somebody who *has* one, is answered `401`
  and has to sign in again — the flow that asks for the factor they hold. Confining it to enrolment
  would be a dead end, because enrolment refuses while a confirmed authenticator exists.
- Turning a requirement off deletes nothing. Enrolments, recovery codes and per-user flags all
  survive, so switching it back on does not make everybody enrol again.

`DELETE /auth/mfa/totp` is refused with `403` while a factor is required of the caller, by policy or
by flag. Removing it would leave an account subject to a requirement it can no longer satisfy — and
the account most likely to try is the administrator who has just turned the policy on.

**SSO accounts are exempt from the local policy.** The rule is `password_hash IS NULL`: an account
that signs in through an identity provider presents no password here, so there is no local sign-in
for a local second factor to stand behind, and the provider is where its factors are asserted and
verified. The exemption covers the per-user flag too, because a requirement somebody has no way to
satisfy is a lockout rather than a policy. `M1-009` and `M1-010` refine this when there is an
assertion to read: an IdP that says what it verified (an `amr` claim, an authentication context
class) is what should satisfy the requirement, and until one exists the honest answer is that this
deployment is not the thing enforcing it. An account that holds both a local password and an SSO
identity is **not** exempt — it can sign in locally, so the local requirement applies to it.

**There is no way to lock the platform out of itself.** An administrator who turns on a requirement
they do not meet is confined to enrolling and can enrol; if their phone is gone, the recovery codes
are the way through; if those are gone too, `blctl user reset-mfa` clears the factor from the host
and the account is confined to enrolling again rather than locked out. None of those paths needs
somebody else to be signed in.

### The pending state

A correct password against an account with a **confirmed** authenticator does not produce a session.
It produces a *challenge*: a row in `app.mfa_challenge` and a `bl_mfa` cookie, and nothing else.

The distinction is enforced in the plumbing rather than in the logic. `internal/authn/challenge` is
a separate package from `internal/authn/session`, with a separate cookie name, a separate table and
a separate HMAC domain; the authentication middleware does not resolve one, so a request carrying
only `bl_mfa` is anonymous everywhere. `POST /auth/mfa/totp/verify` and its recovery-code twin,
`POST /auth/mfa/recovery/verify`, are the only two endpoints that look at it, and the only thing
either can produce is a session.

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

### Recovery codes

A second factor that cannot be recovered from is a way to lose an account, not a way to protect one.
In a self-hosted, single-tenant tool there is no help desk and no mail transport: if the only
administrator drops their phone in a river, the alternatives are a recovery code or a reinstall.

Ten codes are minted when an enrolment is **confirmed**, returned by that one response, and never
again. There is no endpoint that reads them back, because the server keeps only their hashes —
`TestConfirmingAnEnrolmentIssuesTenCodesOnce` walks the API and the log looking for one.

| | |
|---|---|
| Alphabet | Crockford base32 — digits and uppercase letters, less `I`, `L`, `O`, `U` |
| Length | 20 characters, printed in five groups of four |
| Entropy | 100 bits each, against the 80 `M1-007` asks for |
| Stored as | `HMAC-SHA256(key, "blacklight/recovery-code\0" ‖ code)`, base64url |
| Key | HKDF-SHA256 of `BLACKLIGHT_ENCRYPTION_KEY`, under its own info string |

**Why the alphabet.** No two characters in it look alike, and the four that were left out are
*accepted* on the way in and folded onto what they resemble — `O`→`0`, `I`/`L`→`1` — along with any
case and any spacing. Somebody's handwriting must not be what locks them out. `recovery.Parse` is
the one definition of what a code is; the pattern in `api/openapi.yaml` is deliberately looser, so
that anything shaped roughly right is answered as an incorrect code rather than as a malformed
request.

**Why an HMAC and not Argon2id.** A code carries 100 bits from `crypto/rand`, so there is no
dictionary for a work factor to slow down. Verification has to compare against every unused code a
person holds, which under Argon2id would be ten sequential derivations — most of a second — on an
endpoint reachable before authentication, which is a denial-of-service lever aimed at the login
path. Being keyed is worth more here than being slow: a stolen database is not enough to forge a
code, because the key is in the environment and not in the file.

**Why the encryption key and not the session secret.** Rotating the session secret is the documented
way to sign everybody out (`docs/deploy.md`). If codes were keyed from it, pulling that lever would
also destroy every recovery code in the deployment — silently, with the only symptom being that the
way back in stopped working at the moment somebody needed it. Same argument as the TOTP secret
above, same conclusion.

**Using one is a full sign-in.** `POST /auth/mfa/recovery/verify` takes the same `bl_mfa` pending
cookie as the TOTP endpoint and issues a session with `mfa_satisfied` set. Anything less would be
asking the person for the authenticator they have just told us they no longer have. It is spent by
the statement (`UPDATE ... WHERE used_at IS NULL`), so one code buys exactly one session; the row is
kept and marked rather than deleted, so "three of ten left" is answerable. Failures are rationed by
the same limiter and against the same account budget as password and TOTP failures — two counters
would mean an attacker who exhausted one could carry on against the other.

**Regenerating** needs the current password *and* a session that has already satisfied MFA. Not a
fresh code from the authenticator: signing in with a recovery code produces a satisfied session, and
requiring a code would lock out exactly the person these exist for. It invalidates every outstanding
code including the unused ones, because somebody regenerating after a printout went missing is
telling us the missing printout must stop working.

Removing an authenticator deletes the codes with it. A factor that was removed must not still be
presentable, and the next enrolment mints its own set.

### When both are gone

`blctl user reset-mfa --email …` clears the enrolment, the codes and any pending challenge, and says
loudly what it has just done. There is deliberately **no API for it**: needing the database file
means needing the host, and that is the access control. [`docs/cli.md`](cli.md) has the operator's
version.

It does not make a password sufficient again where a policy says otherwise, and it is not meant to.
It touches the factor and nothing else — not `mfa_enforced`, not the platform policy, not any
session — so an account of which a second factor is required still has one required of it, and the
next sign-in is confined to enrolling a new authenticator. That is the break-glass path in full: it
turns "locked out" into "enrol again", and it never turns enforcement off behind an administrator's
back.

## Single sign-on

An account reached through an identity provider is an account like any other here: the same session,
the same cookie, the same rotation, and the same authorization decisions. What changes is only how
the person proved who they are, and two rules follow from that.

**A confirmed authenticator is still asked for.** Somebody who enrolled one meets the code entry
screen whichever door they came in through, because enrolling was their decision and single sign-on
is not a way around it.

**An account with no local password is exempt from being *required* to enrol.** `MFAPolicy.Requires`
returns false for one, and the comment there says why: there is no local sign-in for a local second
factor to stand behind, so the factor is the provider's to enforce. Turning the platform policy on
therefore does not confine every federated account to an enrolment screen it has no reason to be at.

Both protocols land on the same code for all of that. `authn.SignInWithFederatedIdentity` decides
which account a verified assertion becomes, and it neither knows nor cares which of them produced
one — which is why SAML (M1-010) was a second caller rather than a second copy of these rules.

**An assertion is single-use.** OpenID Connect binds an ID token to one exchange with a nonce; SAML
has no equivalent, so a signed assertion stays a working credential for its whole validity window
unless somebody remembers it. `app.saml_assertion` is that memory — a table rather than a map in the
process, because a restart inside that window would otherwise be a scheduled hole in it.

The rest — the protocols, the state and nonce, the key rotation handling, and what a group in the
identity provider does to a role here — is [`docs/sso-oidc.md`](sso-oidc.md) and
[`docs/sso-saml.md`](sso-saml.md).

## Service tokens

The REST API is the only supported integration surface, so these are the credentials for all of it.
`PLAN.md` §4 on v1: "API keys authenticate nothing." That is the defect M1-011 closes, and closing
it is mostly about *where* the checking happens.

**One authentication step, two credentials.** `Authorization: Bearer` is resolved in the same
middleware as the session cookie, into the same `authn.Subject`. Nothing above that line branches on
how a request proved who it is — which is what stops a token from being a credential the session
path never visits, the way v1's were.

**Two fences, and the narrower wins.** An action is permitted only if the token's scopes allow it
*and* the owner's live permissions allow it. Nothing about a role is stored on the token row, so a
demotion applies at that person's next request and there is no cached copy to invalidate. A token
bound to one engagement is held to a third fence that only ever subtracts.

**A token cannot create a token.** `authz.GuardSessionOnly` refuses `token.read` and `token.manage`
to a token-authenticated request whatever it carries. Without it, a leaked token could mint a
longer-lived sibling and outlive its own revocation — which neither of the two fences catches,
because the sibling exceeds neither.

**The secret is a type, not a discipline.** `servicetoken.Token` renders as `[redacted]` under every
printf verb, `log/slog` attribute and JSON encoder; reading it takes `Reveal()`, which one handler
calls. `TestTheSecretAppearsInExactlyOneResponseEverAndInNoLog` greps a full debug-level capture for
a live token's value.

Hashes are keyed from `BLACKLIGHT_ENCRYPTION_KEY` and deliberately not from
`BLACKLIGHT_SESSION_SECRET`: rotating the session secret is the documented way to sign every browser
out, and it must not also break every integration in the deployment.

The operator's half — creating, using, rotating, revoking, and what to do when one leaks — is
[`docs/api-tokens.md`](api-tokens.md).
