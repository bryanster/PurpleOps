# M1-002 — Argon2id password hashing and password policy

**Milestone:** M1 · **Size:** S · **Depends on:** M1-001

## Why

`PLAN.md` §4 specifies Argon2id. v1 used a global `PASSWORD_SALT` from the environment — a shared
salt means identical passwords produce identical hashes across all users, which defeats the purpose
of salting. Per-hash random salts, encoded into the hash string, remove that environment variable
entirely.

## Scope

**In**

- `internal/authn/password`:
  - `Hash(plaintext) (string, error)` — Argon2id, cryptographically random per-hash salt, encoded in
    the standard PHC string format `$argon2id$v=19$m=...,t=...,p=...$salt$hash` so parameters travel
    with the hash.
  - `Verify(plaintext, encoded) (ok bool, needsRehash bool, err error)` — constant-time comparison;
    `needsRehash` true when the stored parameters are weaker than current settings.
  - Tunable parameters with documented defaults (start from OWASP's current Argon2id guidance;
    state the values and the source in a comment).
- Password policy: minimum length 12, no maximum below 128, no composition rules (no "must contain a
  symbol"), reject passwords appearing in a small embedded list of common passwords. Returns
  field-level validation errors via `apierr.Validation` (`M0B-007`).
- A benchmark so the cost parameters can be re-tuned on the target hardware.

**Out**

- Login flow, throttling, sessions — later tickets.
- Breach-corpus (HIBP) lookups. Out of scope for v1; note it as a possible follow-up.

## Acceptance criteria

- [ ] Hashing the same password twice produces different encoded strings, and both verify.
- [ ] `Verify` returns false — not an error — for a wrong password, and an error only for a
      malformed/unparseable stored hash.
- [ ] Comparison is constant-time (`crypto/subtle`). A reviewer should be able to see this at a
      glance.
- [ ] Verifying a hash produced with weaker parameters succeeds and reports `needsRehash: true`; the
      login path (`M1-003`) is expected to transparently upgrade it.
- [ ] No `PASSWORD_SALT`-style global exists anywhere in config or code.
- [ ] Policy rejects: under 12 chars, empty, whitespace-only, and a common password; accepts a long
      passphrase containing spaces.
- [ ] A password that is 200 characters is rejected cleanly rather than being silently truncated or
      causing a memory spike.
- [ ] The plaintext password never appears in a log line, an error message, or a struct with a
      default `String()`. Use a dedicated type or discipline plus a test.

## Tests

- Round-trip hash/verify, wrong password, malformed hash.
- `needsRehash` on downgraded parameters.
- Policy table: one case per rule, asserting the specific field error.
- A benchmark, with the measured time on the developer's machine recorded in the PR — target
  roughly 100–500 ms per hash. Faster than that is too weak; much slower is a login-latency and
  denial-of-service problem.
