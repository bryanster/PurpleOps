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

- [ ] N consecutive failures for one account produce 429; a correct password during the lockout
      **also** produces 429, not a session. (A lockout that lets the right password through isn't a
      lockout.)
- [ ] After the cooldown, a correct password succeeds and the counter resets.
- [ ] A successful login before the threshold clears the counter.
- [ ] Failures against account A do not lock out account B, but the per-IP limiter still trips when
      one source attacks many accounts.
- [ ] The 429 response is identical for existing and non-existent accounts — no enumeration through
      the throttle.
- [ ] `Retry-After` is present and accurate.
- [ ] Memory is bounded: entries are evicted; a test hammering 100k distinct emails does not grow
      unboundedly. State the eviction policy.
- [ ] Removing the throttle middleware makes a test fail. Verify by temporarily removing it — this
      is the regression guard the ticket exists for.
- [ ] `go test -race` clean; the limiter is concurrency-safe.

## Tests

- Table-driven limiter unit tests with an injected clock (do not sleep in tests; make time an
  interface from the start).
- Handler-level tests for the 429 path and the correct-password-during-lockout case.
- A concurrency test firing parallel failures and asserting the count is exact.
