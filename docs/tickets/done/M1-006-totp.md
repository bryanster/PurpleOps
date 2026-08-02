# M1-006 — TOTP enrolment and verification

**Milestone:** M1 · **Size:** M · **Depends on:** M1-003, M1-004

## Why

Second factor for local accounts. This ticket delivers the mechanism; `M1-008` makes it
*enforceable*, which is the part v1 got wrong.

## Scope

**In**

- Migration `0003_mfa.sql`: `user_totp(user_id PK, secret_encrypted, confirmed_at, created_at)`.
- Endpoints:
  - `POST /auth/mfa/totp/enroll` — generates a secret, returns the `otpauth://` URI and a QR code
    (SVG or data URI). Secret is **unconfirmed** at this point.
  - `POST /auth/mfa/totp/confirm` — `{code}`; on success marks confirmed and rotates the session.
  - `POST /auth/mfa/totp/verify` — used during login when a factor is pending.
  - `DELETE /auth/mfa/totp` — requires current password; refused if MFA is enforced for the user
    (`M1-008`).
- Secret storage encrypted at rest with a key derived from `PURPLEOPS_SESSION_SECRET` (or a
  dedicated `PURPLEOPS_ENCRYPTION_KEY` — decide, and document; a dedicated key is cleaner).
- Login flow integration: valid password + confirmed TOTP → a short-lived **pending** state
  (not a full session), which only becomes a session after `verify`. `session.mfa_satisfied` is set
  then.
- Replay protection: a code, once used, cannot be reused within its window.
- Clock skew tolerance of ±1 step (30 s), no more.
- Throttling applied to verification (`M1-004`).

**Out**

- Recovery codes (`M1-007`), enforcement (`M1-008`), WebAuthn (not in v1).

## Acceptance criteria

- [x] Enrolment returns a URI whose issuer and account label are correct and which scans in a real
      authenticator app (state which one you tested with).
- [x] An unconfirmed secret does **not** gate login — a half-finished enrolment cannot lock a user
      out.
- [x] `confirm` with a wrong code fails and leaves the secret unconfirmed.
- [x] The pending state expires (default 5 minutes) and cannot be used afterwards.
- [x] The pending state is not a session: it grants access to nothing except the verify endpoint.
      Test that a pending token on `GET /auth/me` is 401.
- [x] A code from the previous or next 30 s window is accepted; two windows away is rejected.
- [x] Reusing an accepted code within its window is rejected (replay protection).
- [x] Verification failures are throttled and do not enumerate.
- [x] The secret never appears in any API response after enrolment, and never in a log.
- [x] Disabling TOTP requires the current password.

## Tests

- Unit tests with an injected clock covering: valid code, skew ±1, skew ±2 (reject), replay.
- Login-flow integration: password → pending → verify → session with `mfa_satisfied`.
- Pending-state expiry and scope tests.

## Notes for the implementer

- Use a maintained TOTP library (`pquerna/otp` is the usual choice) rather than implementing
  RFC 6238. Do implement the replay window yourself — libraries generally don't.
- Encrypt with AES-GCM and a per-record nonce. Do not reuse a nonce; the test suite should assert
  two encryptions of the same secret differ.

---

## Implementation notes

### The encryption key is its own variable, and required

The ticket left the choice open and said a dedicated key is cleaner. It is `PURPLEOPS_ENCRYPTION_KEY`,
required, and the server refuses to start when it equals `PURPLEOPS_SESSION_SECRET`.

The argument is not tidiness, it is blast radius. Rotating the session secret is the documented way
to sign everybody out (`.env.example`, `docs/deploy.md`) — a lever an operator is *meant* to pull. If
the TOTP secrets were keyed from it, pulling that lever would also make every enrolled authenticator
undecryptable, with no error and no symptom except everybody's codes failing at once. Two keys, two
consequences, and a startup check so they cannot quietly become one.

Required rather than optional-with-a-fallback for the same reason: a fallback is a deployment that
looks configured and is one `openssl rand` away from losing every enrolment. `deploy/entrypoint.sh`
generates and persists both keys beside the database when the environment carries neither, so
`docker compose up` on a clean clone still works — the same convenience the session secret already
had, refactored to serve two.

`internal/authn/secrets` is the package: HKDF-SHA256 to a 32-byte key, AES-256-GCM, a fresh nonce per
record in front of the ciphertext. It is general rather than TOTP-specific because M1-009's client
secret and M1-011's tokens will want it.

### Two extra columns, and a table the ticket did not name

- **`user_totp.last_used_step`** — the replay window. The ticket asks for replay protection and notes
  that libraries do not implement it; it has to live somewhere, and somewhere that survives a
  restart, or a restart re-opens every spent code. `UPDATE ... WHERE last_used_step < ?` is the whole
  check, so two verifications racing on one code cannot both win.
- **`app.mfa_challenge`** — the pending state. The alternative was a signed stateless token, which
  cannot be *spent*: the ticket requires that a pending state cannot be used after it expires, and
  "one correct code buys one session" needs a row to mark. Opening a challenge deletes the previous
  one for that person, which is both the rule and what bounds the table.
- Migration is `0004_mfa.sql`, not `0003` as the ticket says: `0003_user_updatable` was taken by
  `M1-003`, and migrations are append-only.

### The pending state is a cookie, not a body field

`pops_mfa`, `HttpOnly`, `Secure`, `SameSite=Strict`, and `Path=/api/v1/auth/mfa` — scoped to the MFA
endpoints and nothing else. A body field would have put a live credential where script can read it,
which is exactly what the session token is kept out of.

It lives in `internal/authn/challenge`, a separate package from `internal/authn/session` with its own
cookie name, table and HMAC domain. That separation is what makes "a pending token is not a session"
true in the plumbing rather than in a comment: the authentication middleware never resolves one, so
a request carrying only `pops_mfa` is anonymous everywhere, and `GET /auth/me` answers it 401 whether
the token arrives in its own cookie or is pasted into `pops_session`. Both are tested.

### A throttling hole the ticket does not mention, and the fix

Throttling verification (`M1-004`) needed two changes rather than one table entry.

`credentialRoutes` became `credentialAccounts`, mapping a route to a *function*: the verification
body is six digits and names nobody, so the account comes from the pending challenge behind the
cookie. The comment in `throttle.go` already anticipated an extractor for `M1-011`; this is it.

The second is the one worth reading. The throttle reads the outcome from the response status, and a
login answering `mfa_required` is a `200` — which would call `Succeeded` and clear the account's
failure count. Somebody who already holds the password could then sign in again between every code
guess and never run out of budget, which defeats the throttle entirely on the one endpoint where six
digits is the whole search space. The login handler now calls `markCredentialIncomplete`, and such an
attempt counts as neither a success nor a failure. `TestAPendingLoginDoesNotClearTheLockoutBudget` is
that hole, as a test.

### Scope, slightly widened in one place

`MFAState` gained `enrolled`, alongside `enforced` and `satisfied`. Without it no client can tell
whether to offer "set up an authenticator" or "remove it", and `M1-017` would have had to add it
anyway. It reports a *confirmed* enrolment only — an unconfirmed one gates nothing, so reporting it
would be a lie the interface acted on. The three facts come from three different rows, which is the
point of `PLAN.md` §4's complaint about v1.

`DELETE /auth/mfa/totp` refuses with 403 while `user.mfa_enforced` is set. The ticket defers
enforcement to `M1-008` but asks for this refusal here, and the column already existed.

### What a test cannot check

The acceptance criterion asks which authenticator app the QR code was scanned with. **It was not
scanned** — there was no device to scan it with. What was checked instead:

- Every field an app reads out of the `otpauth://` URI — scheme, `issuer:account` label, `issuer`,
  `algorithm`, `digits`, `period`, and a base32 secret of the size RFC 4226 asks for
  (`TestGenerateProducesAURIAnAppCanRead`, `TestEnrolmentReturnsAURIAnAppCanScan`).
- That a code generated from the secret the endpoint hands out is one the endpoint accepts, which is
  the end-to-end statement that the secret shown is the secret stored.
- That the PNG is a structurally valid QR code: decoded by hand off a running server, it is 256×256
  with a 22-pixel quiet zone, a 4-pixel module and correct 7×7 finder patterns in all three corners.

The QR is produced by `boombuler/barcode`'s encoder from that exact URI, so a decode failure would be
a bug in a widely used library rather than in this code — but **please scan it once** with whatever
app your users will use, and note here which one.

### Verified

`make lint test build` green; `make generate` idempotent. Driven by hand against `./bin/purpleops`
with a real DuckDB file: `popsctl user create`, login, enrol, confirm (204, session rotated,
`mfa.enrolled` and `mfa.satisfied` both true), logout, login again → `mfa_required` with only
`pops_mfa` set, `GET /auth/me` on that cookie → 401, verify with the next window's code →
`authenticated` with the session and CSRF cookies set and `pops_mfa` cleared, a second enrolment →
409, disable with the wrong password → 400 and with the right one → 204.
