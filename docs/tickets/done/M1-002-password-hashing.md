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

- [x] Hashing the same password twice produces different encoded strings, and both verify.
- [x] `Verify` returns false — not an error — for a wrong password, and an error only for a
      malformed/unparseable stored hash.
- [x] Comparison is constant-time (`crypto/subtle`). A reviewer should be able to see this at a
      glance.
- [x] Verifying a hash produced with weaker parameters succeeds and reports `needsRehash: true`; the
      login path (`M1-003`) is expected to transparently upgrade it.
- [x] No `PASSWORD_SALT`-style global exists anywhere in config or code.
- [x] Policy rejects: under 12 chars, empty, whitespace-only, and a common password; accepts a long
      passphrase containing spaces.
- [x] A password that is 200 characters is rejected cleanly rather than being silently truncated or
      causing a memory spike.
- [x] The plaintext password never appears in a log line, an error message, or a struct with a
      default `String()`. Use a dedicated type or discipline plus a test.

## Tests

- Round-trip hash/verify, wrong password, malformed hash.
- `needsRehash` on downgraded parameters.
- Policy table: one case per rule, asserting the specific field error.
- A benchmark, with the measured time on the developer's machine recorded in the PR — target
  roughly 100–500 ms per hash. Faster than that is too weak; much slower is a login-latency and
  denial-of-service problem.

---

## Implementation notes

**`internal/authn/password/`** — `hash.go`, `policy.go`, `plaintext.go`, `common_passwords.txt`.
New dependency: `golang.org/x/crypto` (for `argon2`).

### Cost parameters: m=64 MiB, t=3, p=1

Measured with `go test ./internal/authn/password -run '^$' -bench . -benchtime 25x`, on an
Apple M1 Pro:

```
BenchmarkHash-8       25    131942950 ns/op
BenchmarkVerify-8     25    131509172 ns/op
```

132 ms, inside the 100–500 ms band the ticket asks for. Verify costs the same as Hash, which is the
point — it is the same derivation, and it is what a login pays.

OWASP's current Argon2id minimum (m=19456, t=2, p=1) finishes in well under 50 ms on this hardware,
so it is a floor rather than a setting. The comment on `Default` says both the OWASP values and why
these are above them. `p=1` deliberately: a login should cost one core, not several, or a handful of
concurrent logins becomes a way to stall the server.

### `Params` is a type, and every hash carries its own

`needsRehash` is `Default().strongerThan(stored)`, where `stored` is decoded out of the PHC string.
That is the whole mechanism: raising `Default` is a one-line change with no migration, because
existing hashes keep verifying under the parameters they were made with and are replaced on the next
successful login. `Decode` is exported for M1-003 (and the tests), so the login path can look at how
old a hash is without re-deriving it.

Parallelism is compared the other way round from everything else — a hash made with *fewer* lanes at
the same memory is not weaker, so it is left alone. Comment in `strongerThan`.

### `Plaintext`, not `string`

The ticket allows "a dedicated type or discipline plus a test". A type, because discipline does not
survive M1-003 through M1-017 adding login, MFA, password reset and admin user management, each with
a request struct somebody will eventually log. `Plaintext` implements `fmt.Formatter` (not just
`Stringer`, so `%q` and `%x` are covered too), `slog.LogValuer`, `json.Marshaler` and
`encoding.TextMarshaler`, all of which render `[redacted]`; `Reveal()` is the only way to the
characters, and it greps. `plaintext_test.go` puts one inside an ordinary struct and prints it every
way this codebase prints things, including through a JSON `slog` handler.

Unmarshalling is *not* redacted — a password has to arrive from a client — so a request body can
decode straight into a field of this type.

### Two guards the ticket did not ask for

- `MaxPlaintextBytes` (1024) in `Hash`/`Verify`. `Validate` already caps at 128 characters, but
  `Verify` runs on an unauthenticated path and is reachable without asking policy first. Over the
  limit is `ErrTooLong` from `Hash` and a plain `false` from `Verify` — no policy-legal password is
  that long, so it cannot be a real login.
- `Params.validate`. `argon2.IDKey` *panics* on zero rounds or zero lanes, and `Params` is exported
  and constructible. `ErrInvalidParams` turns a footgun into a sentence.

`Decode` likewise refuses a stored `m=` above 1 GiB: verifying allocates whatever the string asks
for, and a damaged row should not be able to exhaust the host.

### The common-password list is short on purpose

Every entry is at least `MinLength` characters, because anything shorter is rejected by the length
rule before the list is consulted — `TestTheCommonPasswordListEarnsItsPlace` enforces that, so the
file cannot silently fill with dead weight. It is therefore not the familiar "top 100": it is the
long tail people write when told to make a password longer (`password123456`, `qwertyuiopasdfghjkl`,
`summer2024!!`, `changeme1234`).

Matching is exact and case-insensitive — no substring matching, no stripping of trailing digits.
Guessing at the shape of a password is how a policy starts refusing passphrases that happen to
contain a common word. **Follow-up, as the ticket notes:** a breach-corpus check (HIBP k-anonymity
range query, or an offline Pwned Passwords file) is the real version of this, and would replace the
file rather than grow it.

### Operator-facing

`docs/deploy.md` gains a short section under Configuration saying that `PASSWORD_SALT` is gone and
that there is nothing to set in its place, plus the policy in two sentences. An operator upgrading
from v1 will go looking for that variable; a grep-able answer beats silence. (`PLAN.md` §7 still
mentions `PASSWORD_SALT` — that is the inventory of v1's `.env`, not a v2 setting.)

### Verified

`make lint test build` green; `make generate && git diff --exit-code` clean. Also
`go test -race ./internal/authn/...`.
