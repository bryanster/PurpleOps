# M1-007 — MFA recovery codes

**Milestone:** M1 · **Size:** S · **Depends on:** M1-006

## Why

`PLAN.md` §4 lists recovery codes alongside TOTP. Without them, a lost phone in a single-tenant
self-hosted tool means an admin editing the database by hand — and if the *only* admin loses their
phone, it means reinstalling.

## Scope

**In**

- Migration: `user_recovery_code(id, user_id, code_hash, used_at, created_at)`.
- Generation: 10 codes at TOTP confirmation time, displayed **once**, never retrievable again.
- Storage: hashed (Argon2id from `M1-002`, or bcrypt/sha256 with a note on the tradeoff — these are
  high-entropy so a fast hash is defensible; justify the choice).
- `POST /auth/mfa/recovery/verify` — accepts a code in place of a TOTP code during login.
- `POST /auth/mfa/recovery/regenerate` — requires current password (and a valid second factor);
  invalidates all previous codes.
- UI surfaces remaining unused count and warns below 3.
- Every use writes to the activity log (`M1-015`) — recovery-code use is a security-relevant event.
- `blctl user reset-mfa --email` — the break-glass path for a locked-out admin, documented in
  `docs/security.md`.

**Out**

- Emailing codes. There is no mail transport in v1.

## Acceptance criteria

- [x] Codes are shown exactly once; a subsequent request cannot retrieve them, only regenerate.
- [x] A used code is rejected on reuse and marked `used_at`.
- [x] Using a recovery code produces a fully MFA-satisfied session (the user is genuinely logged in,
      not half-in).
- [x] Regenerating invalidates all outstanding codes, including unused ones.
- [x] Codes are high-entropy (≥ 80 bits) and formatted to be transcribable — grouped, unambiguous
      alphabet (no `0/O`, `1/l`).
- [x] Verification is throttled (`M1-004`) and constant-time against the stored hashes.
- [x] Disabling TOTP deletes the recovery codes.
- [x] `blctl user reset-mfa` clears TOTP and recovery codes, writes an activity entry, and prints
      a clear warning about what it just did.

## Tests

- Generate → use one → count decrements → reuse rejected.
- Regenerate invalidates old codes.
- Recovery login yields `mfa_satisfied` session.
- `blctl user reset-mfa` integration test.

---

## Implementation notes

### The hash is an HMAC, and it is keyed by the encryption key

The ticket left this open and asked for the tradeoff to be argued. It is
`HMAC-SHA256` over the canonical code, under a key derived from `BLACKLIGHT_ENCRYPTION_KEY` by
HKDF-SHA256 with its own info string — a different derivation from the one `internal/authn/secrets`
uses, so a bug in either can never read or forge the other's values.

Not Argon2id, for three reasons. A code carries 100 bits from `crypto/rand`, so there is no
dictionary for a work factor to slow down. Verification has to compare a presented code against
*every* unused code a person holds — they are unordered and the server does not know which one
arrived — which under Argon2id is ten sequential derivations, most of a second, on an endpoint
reachable before authentication: a denial-of-service lever aimed at the login path. And being keyed
is worth more here than being slow, because a stolen database alone yields nothing.

Keyed by the encryption key rather than the session secret for the reason `M1-006` gives about TOTP
secrets: rotating the session secret is the documented way to sign everybody out, and it must not
also destroy every recovery code in the deployment — silently, with the only symptom being that the
way back in stopped working at the moment somebody needed it.

### Crockford's alphabet, which keeps `0` and `1`

The criterion says "no `0/O`, `1/l`". What is implemented keeps the digits and drops the letters:
`0123456789ABCDEFGHJKMNPQRSTVWXYZ` — no `I`, `L`, `O` or `U`. That satisfies the criterion as stated
(no two characters in the alphabet look alike) and does better than it: the four that were left out
are *accepted* on the way in and folded onto what they resemble, `O`→`0` and `I`/`L`→`1`, along with
any case and any spacing. Somebody's handwriting must not be what locks them out.

Twenty characters, printed in five groups of four, is 100 bits.
`recovery.Parse` is the single definition of what a code is; the pattern in `api/openapi.yaml` is
deliberately *looser*, so anything roughly the right shape is answered as an incorrect code rather
than as a malformed request. Two definitions is how they drift apart.

### `POST /auth/mfa/totp/confirm` answers 200 now, not 204

The ticket says codes are generated at TOTP confirmation time and displayed once. The confirmation
response carries them, which makes "shown exactly once" structural rather than a rule somebody has
to keep: there is no second endpoint that returns them, so there is nothing to call twice.
`TestConfirmingAnEnrolmentIssuesTenCodesOnce` walks the API and the log looking for one.

The alternative — confirm stays 204, a separate `POST /auth/mfa/recovery/generate` follows — was
rejected because a client that skips the second call leaves somebody enrolled with no way back in,
and nothing would notice.

### Regenerating takes a satisfied session, not a fresh TOTP code

The ticket asks for "current password (and a valid second factor)". The second factor is the
requirement that **this session has already satisfied MFA**, checked against `session.mfa_satisfied`
— not a live code in the body.

The argument is decisive rather than a preference: signing in with a recovery code produces a
satisfied session, so this is reachable by the person whose phone is gone. Requiring a code from the
authenticator would lock exactly that person out of replacing the codes they are spending, which is
the case recovery codes exist for. `TestRegeneratingNeedsASatisfiedSession` states both halves.

It is refused with `409` when nothing is enrolled — a reachable state, since a session that
satisfied MFA and then removed the factor keeps its flag, and without the check it could mint
credentials outliving the thing they replace.

### The activity log does not exist yet

`M1-015` is not built. Every event the ticket wants recorded — codes issued, a code used with the
count remaining, a set regenerated, a factor reset from the command line — is written through
`slog` at `warn` where it is security-relevant, carrying the fields an activity entry would. This is
the precedent `M1-004` set for its lockout line, and the same comment is on these: `M1-015` gives
them a durable home. The codes themselves are never in them — `recovery.Code` redacts itself for
`fmt`, `slog` and the JSON encoder alike, so the two handlers that legitimately send one spell out
`Printed()`, and leaking one is a deliberate act rather than an available accident.

### The UI is `M1-017`'s

The ticket asks the interface to surface the remaining count and warn below three. `web/` has no
auth UI at all — the login route is still a placeholder — so there is nothing to hang it off, and
building a login page here would widen this ticket into that one.

What landed instead is the fact the UI needs: `mfa.recoveryCodesRemaining` on `CurrentUser`,
alongside `enforced`, `enrolled` and `satisfied`, documented in `api/openapi.yaml` with the
below-three warning as the note on the field. **`M1-017` owns the rendering and the warning**, and
should not need to change the API to do it.

### `blctl user reset-mfa`, and why it has no endpoint

It clears the enrolment, every code and any pending challenge, reports what it removed, and prints a
warning that says the account now signs in with a password and nothing else. It does not touch the
password, the role, `mfa_enforced` or any session — an account an administrator requires MFA of is
still required to have it, and `M1-008` will walk them through enrolling again.

There is deliberately no API for it. Needing the database file means needing the host, and that is
the access control: an endpoint that strips somebody's second factor is an endpoint worth attacking.
Documented in `docs/cli.md` and `docs/security.md`, both of which point at the recovery codes first.

Pending challenges are cleared too, which the ticket does not name. A challenge outliving the factor
it was opened against is unanswerable anyway; it is the sort of leftover this command exists to
clear, and it is one call.

### Verified

`make test build` green and `make generate` idempotent. `make lint` is green for Go and `web/`;
**`e2e` fails `prettier --check` on `harness/paths.ts` and `README.md`, and did so before this
branch was touched** — neither file is in this diff.

Driven by hand against `./bin/blacklight` with a real DuckDB file: enrol and confirm (200, ten codes,
session rotated), fresh login → `mfa_required` with only `bl_mfa` set, `GET /auth/me` on that cookie
→ 401, `recovery/verify` with a code typed in lower case with the hyphens stripped → 200
`authenticated` with `satisfied: true` and `recoveryCodesRemaining: 9`, the same code again → 401,
regenerate → 200 with ten different codes, an *unused* code from the old set → 401, regenerate with
a wrong password → 400 on `currentPassword`. None of the twenty codes appears anywhere in the server
log. Then `blctl user reset-mfa --email ALICE@Example.com` (matched without regard to case) →
authenticator and ten codes removed with the warning, login afterwards → `authenticated` with
`enrolled: false`, and running it a second time → "had no second factor. Nothing was removed."
