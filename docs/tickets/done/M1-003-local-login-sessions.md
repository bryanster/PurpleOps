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
  - Cookie: `HttpOnly`, `Secure` (relaxable only when `BLACKLIGHT_ENV=development`), `SameSite=Strict`,
    `Path=/`, no `Domain`, expiry matching the session.
  - Absolute expiry (default 12h) and idle timeout (default 2h), both configurable.
  - `Rotate` — issue a new token for the same session on: successful login, MFA completion, password
    change, platform-role change.
- Authentication middleware inserted at the marked point in `M0B-006`'s chain: resolves the cookie
  to a user, puts an `authn.Subject` in the context, and **does not** decide anything about access.
  Unauthenticated is allowed to proceed — `M1-013`'s authz middleware rejects.
- `blctl user create --email --name --admin` — the bootstrap path, prompting for a password on a
  TTY and accepting stdin otherwise. This replaces `M0B-014`'s stub.

**Out**

- Throttling (`M1-004`), CSRF (`M1-005`), MFA (`M1-006`–`M1-008`), SSO (`M1-009`, `M1-010`).
  The login handler must be written so those slot in without restructuring — in particular, login
  returns a *pending* state when MFA is required rather than a full session.

## Acceptance criteria

- [x] Correct credentials return 200, set the cookie, and record `last_login_at`.
- [x] Wrong password, unknown email, and disabled user all return the **same** 401 with the same
      body and no user-existence hint. Timing should not obviously distinguish unknown-email from
      wrong-password — verify the hash comparison runs in both cases (compare against a dummy hash
      for unknown users).
- [x] The session token never appears in a response body, a URL, a log line, or `GET /auth/me`.
- [x] Logging in twice produces two independent sessions; logging out of one leaves the other valid.
- [x] Logout revokes server-side (sets `revoked_at`) **and** clears the cookie. A replayed cookie
      after logout is 401 — don't rely on the browser dropping it.
- [x] An expired or idle-timed-out session is 401, and the row is not silently reusable.
- [x] Changing the password rotates the current session and revokes all *other* sessions for that
      user. Assert both.
- [x] A login with a `needsRehash` password transparently upgrades the stored hash (`M1-002`).
- [x] `GET /auth/me` unauthenticated is 401 with the standard problem shape.
- [x] `blctl user create` creates a working admin, refuses a duplicate email, and never echoes the
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

---

## Implementation notes

**`api/openapi.yaml` · `internal/authn/` · `internal/httpapi/` · `internal/cli/user.go` ·
`0003_user_updatable.sql`**

### The blocker: DuckDB will not update a referenced row

`UPDATE app."user" SET last_login_at = ?` fails for every account that has a session, an identity or
a membership:

```
Constraint Error: Violates foreign key constraint because key "user_id: 019f…" is still
referenced by a foreign key in a different table.
```

DuckDB (v1.5.5) implements an `UPDATE` on an indexed table as a delete followed by an insert, and
the delete half runs `0002_identity`'s `ON DELETE RESTRICT`. It is not about updating `id` — *any*
column is refused. So recording a login, upgrading a password hash and changing a password were all
impossible, and this ticket could not be finished as the schema stood.

`0003_user_updatable.sql` removes the three foreign keys pointing at `app."user"` — by recreating
each table, because DuckDB has no `ALTER TABLE ... DROP CONSTRAINT` either. **This reverses a
deliberate decision in `M1-001` and was agreed before it was written.** What it costs and what
replaces it:

| The key enforced | Now |
|---|---|
| A hard `DELETE` of a user who owns rows is refused | Nothing refuses it. There is no code that deletes a user — `identity.Users` has no `Delete`, by `M1-001`'s own design — and accounts are retired with `status = 'disabled'` |
| A session/identity/membership must belong to a real user | `requireUser`, inside each repository's write transaction. The serialized writer admits one transaction at a time, so it is as strong as the constraint was |

`TestAUserWhoOwnsRowsCanStillBeUpdated` is the regression case, and
`TestADependentRowMustBelongToARealUser` is the invariant that moved. `docs/migrations.md` gains
both limitations under "Adding a migration", because the next person to reach for a foreign key
needs to know before they write it, not after.

### The token: 32 random bytes, stored as a *keyed* hash

HMAC-SHA256 under `BLACKLIGHT_SESSION_SECRET`, not a bare SHA-256. The ticket asks only that the hash
be stored; keying it means a stolen database is not enough to look a token up, and it gives
`BLACKLIGHT_SESSION_SECRET` — which `.env.example` already promised "logs everybody out" when
rotated — something to actually do. It is deliberately a fast hash: the input is 256 bits of uniform
randomness, so there is no dictionary for a slow one to defend against, and this runs on every
authenticated request.

`session.Token` is a redacting type in the mould of `password.Plaintext` (`fmt.Formatter`,
`slog.LogValuer`, `json.Marshaler`), so the compiler keeps it out of logs and bodies rather than a
reviewer's memory. `TestTheTokenNeverLeavesTheCookie` checks the response, `/auth/me` and the log.

### One 401 code, two constructors

There was no way to return a 401: the `ProblemCode` enum had no code for it. Added
`unauthenticated`, with a `components/responses/Unauthenticated`, the row in
`internal/httpapi/apierr/codes.go` and the `Record<ProblemCode, true>` entry in `web/src/api`.

Two constructors share it. `apierr.Unauthenticated(reason)` is "no usable session";
`apierr.BadCredentials(reason)` is every failed sign-in. They are separate so that the handler
*cannot* vary the login response — the constructor takes no detail — which is how "wrong password,
unknown address and disabled account are byte-identical" becomes structural rather than a rule to
remember. `TestEveryFailedLoginIsTheSameAnswer` compares the bodies with `instance` removed.

The timing half is a decoy hash, built once on first use with today's Argon2id parameters, verified
against whenever there is no stored hash to check. `TestAnUnknownAddressCostsAHashToo` measures both
paths with a deliberately wide margin: the difference it is looking for is a derivation running or
not running at all.

### Rotation replaces the token in place

`Sessions.Rotate` updates `token_hash` on the existing row rather than revoking and re-creating, so
the identifier, the creation time and the **absolute expiry** all survive. Rotating is therefore not
a way to stay signed in forever, and the old token resolves to nothing because its hash is gone.
`Sessions.RevokeOthersForUser` is the other new repository method, for the password-change path:
the browser making the change keeps its session, everywhere else is signed out.

### MFA fails closed

The ticket asks for login to return a *pending* state rather than a session when MFA is required.
`user.mfa_enforced` produces `status: "mfa_required"`, no cookie and no session row — a user an
administrator has flagged cannot sign in until `M1-006`–`M1-008` give them a challenge to answer.
Nothing sets that column today, so nobody is locked out by it, and the alternative (ignore the flag,
issue a full session) is the v1 hole `PLAN.md` §4 names.

### The validator's `AuthenticationFunc` allows everything

It has to exist — `kin-openapi` refuses to serve an operation with a security requirement without
one — and it has to say yes. It runs *before* the cookie has been resolved, so the only question it
could answer is "is there a cookie header", and a 401 from it would not be this API's 401. The
comment in `internal/httpapi/validate.go` says so at length, because "permissive stub" is exactly
what it looks like. Authentication is step 8 of the chain; authorization is `M1-013`.

Until `M1-013` lands, the two endpoints that need a caller call `subjectFrom(ctx)` — one helper, so
four handlers cannot phrase the 401 four ways. That is the line `M1-013` deletes.

### Two things the ticket did not ask for

- **`BLACKLIGHT_SESSION_LIFETIME` and `BLACKLIGHT_SESSION_IDLE_TIMEOUT`** (12h, 2h). The ticket says
  both are configurable; these are the variables, validated against each other — an idle timeout
  longer than the lifetime is a startup error, because it is not a generous idle policy, it is none.
- **`last_seen_at` is written at most once a minute** per session (`touchInterval`). Writing it on
  every request would put the serialized write lock in front of every authenticated read, for a
  column whose only consumer is a timeout measured in hours.

### A password change also refuses the current password

`newPassword` is checked against the stored hash and rejected with a field error if it matches. A
small addition, but "change your password" that accepts the same password is not a change.

### Formats dropped from the spec, deliberately

`format: uuid` and `format: email` make oapi-codegen generate `openapi_types.UUID` and
`openapi_types.Email`, which means a new module dependency (`github.com/oapi-codegen/runtime`) and a
`uuid.Parse` at every boundary for identifiers this codebase stores and passes as `TEXT`. They are
plain strings with the constraint in the description instead. `docs/api.md`'s prose still describes
identifiers as UUIDv7 — that remains true; it is the `format` keyword that is not carrying its
weight.

### `blctl user create`

Prompts twice on a terminal with echo off (`golang.org/x/term`, a new direct dependency), reads
stdin once when there is not one, and strips a single trailing newline so `echo … |` works. **No
`--password` flag and no environment variable**: both end up in shell history, in `ps` and in the
logs. `cli.Main` takes a `stdin io.Reader` now, which is the only signature change; the terminal is
detected by type-asserting it to `*os.File`, so a test gets the piped path.

It creates the local `identity` row alongside the user, in a second transaction. Local login does not
need it — that resolves through `Users.ByEmail` — but `M1-009`'s account linking reads that table,
and a deployment should not be left with a gap in it. A failure between the two says exactly that.

### Verified

`make lint test build` green, `make test-spa` green, `make generate` idempotent. Also driven by hand
against a real binary: `blctl user create`, then login, `/auth/me`, a password change with the
session rotating, logout, and the replay of the old cookie coming back 401.
