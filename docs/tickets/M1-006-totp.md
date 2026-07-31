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

- [ ] Enrolment returns a URI whose issuer and account label are correct and which scans in a real
      authenticator app (state which one you tested with).
- [ ] An unconfirmed secret does **not** gate login — a half-finished enrolment cannot lock a user
      out.
- [ ] `confirm` with a wrong code fails and leaves the secret unconfirmed.
- [ ] The pending state expires (default 5 minutes) and cannot be used afterwards.
- [ ] The pending state is not a session: it grants access to nothing except the verify endpoint.
      Test that a pending token on `GET /auth/me` is 401.
- [ ] A code from the previous or next 30 s window is accepted; two windows away is rejected.
- [ ] Reusing an accepted code within its window is rejected (replay protection).
- [ ] Verification failures are throttled and do not enumerate.
- [ ] The secret never appears in any API response after enrolment, and never in a log.
- [ ] Disabling TOTP requires the current password.

## Tests

- Unit tests with an injected clock covering: valid code, skew ±1, skew ±2 (reject), replay.
- Login-flow integration: password → pending → verify → session with `mfa_satisfied`.
- Pending-state expiry and scope tests.

## Notes for the implementer

- Use a maintained TOTP library (`pquerna/otp` is the usual choice) rather than implementing
  RFC 6238. Do implement the replay window yourself — libraries generally don't.
- Encrypt with AES-GCM and a per-record nonce. Do not reuse a nonce; the test suite should assert
  two encryptions of the same secret differ.
