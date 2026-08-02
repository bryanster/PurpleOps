# M1-004 — Login throttling and account lockout

**Milestone:** M1 · **Size:** M · **Depends on:** M1-003

## Why

`PLAN.md` §4: "login throttling (restoring the rate limiting deleted in the working tree)". v1 had
it, then lost it, and nothing noticed — which is the argument for tests that assert it rather than
a middleware someone can quietly remove.

## Scope

**In**

- `internal/authn/throttle` — two independent limiters, both required:
  - **Per-identifier** (normalized email): after N failures, apply increasing delay, then lock for a
    cooldown. Successful login resets the counter.
  - **Per-source-IP**: a broader cap, to blunt spraying across many accounts.
- In-memory storage with periodic eviction. Single-node deployment (`PLAN.md` §1) makes this correct;
  say so in a comment so a future reader doesn't assume it's a shortcut.
- Applied to: `POST /auth/login`, MFA verification (`M1-006`), password reset if one exists, and
  service-token authentication (`M1-011`).
- Configurable thresholds with safe defaults, e.g. 5 failures → lock 15 min per account; 50 failures
  per IP per 15 min.
- On throttle: 429 with `code: "rate_limited"` (`M0B-007`) and a `Retry-After` header.
- Each lockout writes an activity-log entry (`M1-015`) — an operator needs to see this.

**Out**

- Distributed rate limiting. Explicitly out; the deployment model is one node.
- CAPTCHA.

## Acceptance criteria

- [x] N consecutive failures for one account produce 429; a correct password during the lockout
      **also** produces 429, not a session. (A lockout that lets the right password through isn't a
      lockout.)
- [x] After the cooldown, a correct password succeeds and the counter resets.
- [x] A successful login before the threshold clears the counter.
- [x] Failures against account A do not lock out account B, but the per-IP limiter still trips when
      one source attacks many accounts.
- [x] The 429 response is identical for existing and non-existent accounts — no enumeration through
      the throttle.
- [x] `Retry-After` is present and accurate.
- [x] Memory is bounded: entries are evicted; a test hammering 100k distinct emails does not grow
      unboundedly. State the eviction policy.
- [x] Removing the throttle middleware makes a test fail. Verify by temporarily removing it — this
      is the regression guard the ticket exists for.
- [x] `go test -race` clean; the limiter is concurrency-safe.

## Tests

- Table-driven limiter unit tests with an injected clock (do not sleep in tests; make time an
  interface from the start).
- Handler-level tests for the 429 path and the correct-password-during-lockout case.
- A concurrency test firing parallel failures and asserting the count is exact.

---

## Implementation notes

**`internal/authn/throttle/` · `internal/httpapi/throttle.go` · `internal/httpapi/apierr/` ·
`internal/config/` · `api/openapi.yaml`**

### Two limiters, one middleware, and a state machine per key

`internal/authn/throttle` holds a table per limiter, keyed on the normalized email address and on
the client address respectively, and the HTTP layer holds none of it. Each key is a small state
machine:

| | |
|---|---|
| A failure below the threshold | counted |
| The threshold's failure | the key closes for the cooldown, and the failure count starts again |
| A failure while closed | nothing. Hammering must not extend a lockout, or knowing an address would be enough to keep its owner signed out for as long as a script kept running |
| Closing again | doubles the cooldown, three times: 15m, 30m, 1h, 2h |
| A success | the key is forgotten, count and doubling together — for the account limiter only |
| No activity for the longest lockout the key could be serving | the key is forgotten |

The last two rows are the ways back down. The second is also the eviction policy, and it is safe by
arithmetic rather than by care: a lockout ends at `lastAt + at most maxLockout` and an entry is kept
until `lastAt + maxLockout`, so **a stale entry is never a locked one** and flooding the table can
never be a way out of a lockout. `TestEvictionCannotUnlockAnAccount` is that case.

The ceiling — 50,000 keys per limiter, a constant rather than a variable, because an operator asked
to pick one has no way to know what a good answer is — refuses *new* keys rather than evicting live
ones, and warns once when it starts doing so. It fails open for the same reason: turning a full
table into a total outage would hand an attacker a cheaper attack than the one being prevented, and
the source limiter, whose key space is bounded by the number of hosts that can reach the server, is
still counting.

Sweeping is amortized on the calls that already hold the lock, at most once a minute of the injected
clock, so there is no background goroutine to leak, to stop, or to make a test wait for.

### The doubling, and what it costs

The ticket asks for "increasing delay, then lock for a cooldown". This doubles the *cooldown* rather
than inserting a sleep before answering: a handler that slept would tie up a goroutine per attempt,
which is a denial of service an attacker can trigger deliberately. The counter resets when a key
closes, so re-earning a lockout costs the same number of guesses as the first one did — at twice the
wait.

It is worth being explicit about the trade this makes. Any lockout lets somebody who knows an
address keep its owner out, and the doubling makes that *cheaper* for the attacker (one request
every two hours rather than one every fifteen minutes) while making brute force much more expensive.
The cap and the two ways back down are what bound the first; the per-IP limiter is what actually
stops spraying. `M1-016`'s admin API is where an "unlock this account now" action belongs.

### Where it runs: step 8, before authentication

`credentialRoutes` in `internal/httpapi/throttle.go` maps a guarded path to the body field naming
the account being attempted — today just `POST /auth/login` and `email`. It is a middleware and not
a check inside the handler because the defect this ticket exists for is that v1 *had* login rate
limiting and quietly lost it; deleting the line from the chain now fails five tests, which was
verified by deleting it.

It sits **before** authentication rather than after, so a locked-out caller costs no session lookup
and no Argon2id derivation — that second half matters as much as the refusal does, because a slow
hash is otherwise a way to spend the server's CPU. It sits **after** validation, so a request that
does not match the specification is not counted as a guess.

The outcome is read from the status the handler produced: 401 is a failed attempt, 2xx a successful
one, anything else neither. That is what keeps the rule in one place rather than asking every
handler to remember to report itself, and it is what makes `M1-006`'s MFA endpoint and `M1-011`'s
token endpoint a line in the table rather than a new middleware. `M1-011`'s credential is a header
rather than a body field, so it will need an extractor there; the comment says so.

The middleware reads the body to find the account and puts it back, which the request validator
above it also does. `TestTheThrottleLeavesTheBodyForTheHandler` is the regression case for the
obvious way to break every login at once.

### `Retry-After` travels on the error

`apierr.RateLimited` now takes the wait, and `apierr.Responder` — the one writer of every error this
API sends — turns it into the header, rounded **up** to whole seconds. A client that came back at
the rounded-down second would still be locked out and would spend its retry finding that out. The
alternative, having the limiter set the header itself, would mean each future caller of this
constructor could forget; this way a 429 without a `Retry-After` is not expressible.

The ticket's "the 429 is identical for existing and non-existent accounts" is structural for the
same reason `M1-003`'s 401 is: the detail is a package constant and the throttle passes nothing
else. `TestTheThrottleTellsARealAccountAndAnInventedOneApartNoBetterThanTheLoginDoes` compares the
bodies. It deliberately does **not** compare `Retry-After`, which counts down from when each lockout
began — a fact about when the requests were sent, not about whether the account exists.

### The activity-log entry is a log line for now

The ticket asks for an activity-log entry per lockout. `M1-015` owns that table and has not landed —
it depends on `M1-013` — and inventing the schema here would mean `M1-015` migrating it. Until then
a lockout is a `WARN` line carrying the scope, the key and the client address, which is what an
operator needs when somebody reports they cannot sign in. `M1-015`'s "M1 wiring" bullet already
lists lockouts; the line to change is in `throttleCredentials`.

### Configuration

Four variables, defaulting to the ticket's numbers: `PURPLEOPS_LOGIN_ACCOUNT_FAILURES` (5),
`PURPLEOPS_LOGIN_ACCOUNT_LOCKOUT` (15m), `PURPLEOPS_LOGIN_SOURCE_FAILURES` (50),
`PURPLEOPS_LOGIN_SOURCE_LOCKOUT` (15m). The cap on the doubling is derived rather than configured:
three doublings of whichever lockout was set.

`internal/config` learned to parse an `int` for them, and rejects zero and negatives for every count
it reads — a threshold of zero is not a stricter policy, it locks out the first person through the
door. A zero-valued `config.Throttle` is a startup error rather than "no throttling", which is why
`testConfig` in `internal/httpapi` grew the defaults.

One consequence worth knowing: `Config` now holds a field with no `String` method, so `fmt.Sprintf`
with `%s` of a whole `Config` is a verb the vet and staticcheck both object to.
`TestSecretIsRedactedInEveryRendering` keeps that rendering with a `//nolint` — a redaction that only
holds for the verbs a linter approves of is not a redaction.

### Tests

Everything about time is unit-tested against an injected clock in
`internal/authn/throttle/throttle_test.go` — the doubling, the cap, the eviction, the 100,000
distinct keys, and a `-race` test firing 64 parallel failures at a threshold of 64 and asserting
that **exactly one** of them reports a lockout on each limiter (one fewer would be a lost update;
one more would be two goroutines both believing they were last).

The handler tests in `internal/httpapi/throttle_test.go` are what a browser meets. One of them
sleeps: `TestTheLockoutEndsWhenItSaysItWill` configures a 500ms lockout and waits it out, because
the assertion in the middle is refused before the handler runs and therefore costs no password hash,
so the gap between locking and asserting is microseconds and the sleep can only ever be too long.

### Verified

`make lint test build` green, `make generate` idempotent, `go test -race ./...` clean. Deleting
`throttleCredentials` from the chain fails five tests. Also driven by hand against a real binary
with a 3-failure, 30-second policy: the fourth attempt 429s with `Retry-After: 30`, the correct
password during the lockout 429s identically, an address nobody holds is byte-identical to a real
one, the right password succeeds after the cooldown, and a second lockout without a success in
between reports `Retry-After: 60`.
