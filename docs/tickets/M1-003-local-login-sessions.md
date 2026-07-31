# M1-003 — Local login, logout, session cookies and rotation

**Milestone:** M1 · **Size:** L · **Depends on:** M1-001, M1-002, M0B-006

## Why

The first real authenticated path. `PLAN.md` §4 requires "secure session cookies, rotation on
privilege change" — session fixation is the attack this prevents, and it matters more than usual
here because the platform's whole point is that red and blue see different things.

## Scope

**In**

- Endpoints (spec first, in `api/openapi.yaml`):
  - `POST /auth/login` — `{email, password}` → session cookie + current-user body, or 401.
  - `POST /auth/logout` — revokes the current session.
  - `GET /auth/me` — the current user: id, email, display name, platform role, MFA state,
    engagement memberships.
  - `POST /auth/password` — change own password (requires current password).
- `internal/authn/session`:
  - Token: 32 bytes from `crypto/rand`, base64url. **Only the hash is stored** (`M1-001`), so a
    database leak doesn't hand over live sessions.
  - Cookie: `HttpOnly`, `Secure` (relaxable only when `PURPLEOPS_ENV=development`), `SameSite=Strict`,
    `Path=/`, no `Domain`, expiry matching the session.
  - Absolute expiry (default 12h) and idle timeout (default 2h), both configurable.
  - `Rotate` — issue a new token for the same session on: successful login, MFA completion, password
    change, platform-role change.
- Authentication middleware inserted at the marked point in `M0B-006`'s chain: resolves the cookie
  to a user, puts an `authn.Subject` in the context, and **does not** decide anything about access.
  Unauthenticated is allowed to proceed — `M1-013`'s authz middleware rejects.
- `popsctl user create --email --name --admin` — the bootstrap path, prompting for a password on a
  TTY and accepting stdin otherwise. This replaces `M0B-014`'s stub.

**Out**

- Throttling (`M1-004`), CSRF (`M1-005`), MFA (`M1-006`–`M1-008`), SSO (`M1-009`, `M1-010`).
  The login handler must be written so those slot in without restructuring — in particular, login
  returns a *pending* state when MFA is required rather than a full session.

## Acceptance criteria

- [ ] Correct credentials return 200, set the cookie, and record `last_login_at`.
- [ ] Wrong password, unknown email, and disabled user all return the **same** 401 with the same
      body and no user-existence hint. Timing should not obviously distinguish unknown-email from
      wrong-password — verify the hash comparison runs in both cases (compare against a dummy hash
      for unknown users).
- [ ] The session token never appears in a response body, a URL, a log line, or `GET /auth/me`.
- [ ] Logging in twice produces two independent sessions; logging out of one leaves the other valid.
- [ ] Logout revokes server-side (sets `revoked_at`) **and** clears the cookie. A replayed cookie
      after logout is 401 — don't rely on the browser dropping it.
- [ ] An expired or idle-timed-out session is 401, and the row is not silently reusable.
- [ ] Changing the password rotates the current session and revokes all *other* sessions for that
      user. Assert both.
- [ ] A login with a `needsRehash` password transparently upgrades the stored hash (`M1-002`).
- [ ] `GET /auth/me` unauthenticated is 401 with the standard problem shape.
- [ ] `popsctl user create` creates a working admin, refuses a duplicate email, and never echoes the
      password.

## Tests

- Handler tests for every acceptance bullet, using a real temp DuckDB (`storetest`).
- A session-lifecycle test: create → use → rotate → old token rejected, new token accepted.
- Cookie attribute assertions, including that `Secure` is set in production mode and the exact
  `SameSite` value.

## Notes for the implementer

- Do not build your own session middleware chain. There is one chain (`M0B-006`); add to it.
- Store `mfa_satisfied` on the session now even though nothing sets it until `M1-008` — retrofitting
  a column onto live sessions is a migration you can avoid for free today.
