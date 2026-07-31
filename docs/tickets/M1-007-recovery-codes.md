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
- `popsctl user reset-mfa --email` — the break-glass path for a locked-out admin, documented in
  `docs/security.md`.

**Out**

- Emailing codes. There is no mail transport in v1.

## Acceptance criteria

- [ ] Codes are shown exactly once; a subsequent request cannot retrieve them, only regenerate.
- [ ] A used code is rejected on reuse and marked `used_at`.
- [ ] Using a recovery code produces a fully MFA-satisfied session (the user is genuinely logged in,
      not half-in).
- [ ] Regenerating invalidates all outstanding codes, including unused ones.
- [ ] Codes are high-entropy (≥ 80 bits) and formatted to be transcribable — grouped, unambiguous
      alphabet (no `0/O`, `1/l`).
- [ ] Verification is throttled (`M1-004`) and constant-time against the stored hashes.
- [ ] Disabling TOTP deletes the recovery codes.
- [ ] `popsctl user reset-mfa` clears TOTP and recovery codes, writes an activity entry, and prints
      a clear warning about what it just did.

## Tests

- Generate → use one → count decrements → reuse rejected.
- Regenerate invalidates old codes.
- Recovery login yields `mfa_satisfied` session.
- `popsctl user reset-mfa` integration test.
