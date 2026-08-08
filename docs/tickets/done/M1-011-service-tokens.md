# M1-011 — Scoped service tokens, actually enforced

**Milestone:** M1 · **Size:** L · **Depends on:** M1-012, M1-013

## Why

`PLAN.md` §4, on v1: service tokens must be "**actually enforced on every API route** — today's API
keys authenticate nothing." A token system that authenticates nothing is worse than none, because
the UI implies protection that doesn't exist.

`PLAN.md` also makes the public REST API "the only integration surface in v1", so these tokens are
how anyone automates against Blacklight. They carry real weight.

## Scope

**In**

- Migration: `service_token(id, name, token_hash, prefix, owner_user_id, scopes[], engagement_id
  NULL, created_at, expires_at, last_used_at, revoked_at, created_by)`.
- Token format: `bl_<prefix>_<secret>` where `prefix` is a short public identifier stored in clear
  for lookup, and only the secret's hash is stored. Displayed **once** at creation.
- Endpoints (admin, or owner for their own): create, list (never returning the secret), revoke.
- Authentication: `Authorization: Bearer <token>` resolved in the same authn middleware as sessions
  (`M1-003`), producing the same `authn.Subject` type so authorization code doesn't branch on
  auth method.
- **Scopes**: coarse and few — e.g. `engagements:read`, `engagements:write`, `content:read`,
  `content:sync`, `reports:read`. List them in the spec. Resist a per-endpoint scope explosion.
- **The two-fence rule** (this is the ticket's core): an action is permitted only if the token's
  scopes allow it **and** the owning user's live permissions allow it. Revoking or demoting the
  owner immediately constrains the token, with no token change.
- Optional engagement-scoping: a token bound to one engagement can touch nothing else.
- Expiry required at creation, with a maximum (e.g. 1 year); expired tokens are 401.
- `last_used_at` updated asynchronously (do not put a write on every read request's critical path —
  batch or debounce it, and say which).
- Throttling on token auth failures (`M1-004`).
- `docs/api-tokens.md`: create, use with `curl`, rotate, revoke.

**Out**

- OAuth2 client credentials. Bearer tokens only in v1.
- Per-token IP allowlists.

## Acceptance criteria

- [x] **Regression case (must exist by name):** a request with an invalid/absent token to a
      protected route is 401 and never reaches the handler. Enumerate routes from the generated
      router so this can't rot.
- [x] A token missing the required scope is 403, not 401 — the distinction is real and clients rely
      on it.
- [x] **A token cannot exceed its owner's live permissions** (`PLAN.md` §9). Test: create a token
      while the owner is an admin, demote the owner, assert an admin-only call now fails.
- [x] Revoking the owner's account immediately disables their tokens.
- [x] An engagement-scoped token gets 403/404 for other engagements — and the response for a
      *different* engagement's resource must not confirm its existence.
- [x] The secret appears in exactly one response ever (creation) and never in list, logs, or the
      activity feed. A test greps a full log capture for the secret.
- [x] Expired token → 401; revoked token → 401.
- [x] Token comparison is constant-time and the lookup is by prefix (not a full-table scan of
      hashes).
- [x] CSRF is not applied to token-authenticated requests, and this exemption cannot be triggered by
      an invalid `Authorization` header (`M1-005`).
- [~] Creation, use-first-time, and revocation are written to the activity log.

## Tests

- Auth middleware table: valid / invalid / expired / revoked / malformed / owner-disabled.
- Scope enforcement per scope.
- The two-fence tests (demotion, revocation) — the important ones.
- A route-coverage test asserting every non-public route rejects an unauthenticated request. Fold
  this into `M1-014`'s matrix if it fits more naturally there.

---

## Implementation notes

Read these before starting a ticket that depends on this one.

### Scope decisions taken during implementation

**The endpoints are owner-only.** The ticket says "admin, or owner for their own". `authz.Can` is
purely role-based — there is no "owner of this row" concept, and inventing one would have put a
second kind of decision in the policy — so `POST/GET /auth/tokens` and
`DELETE /auth/tokens/{tokenId}` are scoped to the caller in the repository (`WHERE owner_user_id =
?`), the way `GET /auth/me` is. An administrator sees their own tokens like everybody else. What an
administrator has instead is disabling the account, which stops every token that account holds at
its next request — which is the acceptance criterion that actually matters. Administrative
management of *other people's* tokens is `M1-018`, and belongs next to `M1-016`'s admin surface.

**A service token cannot create or revoke a service token.** Not in the ticket, and added
deliberately: the two fences do not catch a leaked token minting a longer-lived sibling, because the
sibling exceeds neither the owner's role nor the scope list — it merely survives the revocation of
the token that made it. This needed a new mechanism in M1-012's table, `authz.GuardSessionOnly`, and
therefore a change to `Guard.blocks`: it now takes the `Subject` and returns a whole `Decision`
rather than a reason, because the two guards refuse differently (blind mode conceals, this one does
not). `docs/authz.md` renders it.

**Activity log entries are structured log lines for now.** `M1-015` owns `internal/events.Activity`
and the `token.created` / `token.revoked` verbs and is not built yet, so creation, first use and
revocation each emit a log line at the exact call site with the fields that entry will carry —
the same thing `throttle.go` already does for lockouts. `TestTheLifecycleIsRecorded` asserts all
three. **M1-015 should replace these three call sites**, in `internal/authn/servicetoken/manager.go`,
and add `token.first_used` to its verb vocabulary.

### Where the implementation deviates from the obvious reading

**The token is base32, not base64url.** Every other opaque value in this tree is base64url, which
cannot be used here: its alphabet contains `_`, which is the separator in `bl_<prefix>_<secret>`. A
prefix that happened to encode one would split into four parts — reliably, for a few in a thousand
tokens, and only in production. The stored *hash* is still base64url; it is never parsed.

**Scopes are one space-separated column, not an array and not a child table.** `TEXT[]` is
DuckDB-only syntax and the migration is outside `internal/store/duckdb/`. A child table would need a
foreign key back to `app.service_token` to be worth having, and `0003_user_updatable` established
that a foreign key makes the parent row un-`UPDATE`-able under DuckDB — which would break both
`last_used_at` and `revoked_at`. Space-separated is OAuth 2.0's own spelling (RFC 6749 §3.3), and no
scope contains a space. For the same reason, `owner_user_id` and `created_by` have no foreign keys
and are checked with `requireUser` inside the write transaction.

**`last_used_at` is debounced *and* moved off the request.** The ticket asks for one or the other;
both are here because the debounce alone would still put a serialized write on a caller's critical
path once a minute. `servicetoken.Options.Background` is the seam — a goroutine per job in the
binary, run inline in a test so an assertion is not racing it. There is no second implementation and
no lifecycle to manage.

**An invalid bearer token falls back to the session cookie.** The first implementation refused
outright, which is arguably tidier; M1-005's merged `TestOnlyRealServiceTokenAuthenticationIsExempt`
asserts the fallback, and it is the more conservative behaviour — the request is then judged as the
cookie request it actually is, CSRF check and all, so the exemption genuinely cannot be claimed by
sending a header.

**Token throttling reads an explicit verdict, not the response status.** `M1-004`'s middleware
judges a sign-in by the status the handler produced, which is wrong for tokens in both directions: a
token refused for want of a scope answers 403 and has authenticated perfectly well, and a wrong token
that fell back to a good cookie answers 2xx and has not. `credentialOutcome.verdict` is set by the
authentication step, which is the only place that knows. A token is counted against its *prefix*, on
every route rather than on one endpoint — a credential checked everywhere cannot be rationed in one
place.

### What is not testable yet

**The engagement binding** has no HTTP-level test against a real engagement endpoint, because there
are none until M3. It is covered at the policy layer
(`TestAnEngagementBoundTokenReachesNothingElse`) and through the authorization middleware over
`authorize_test.go`'s fixture spec
(`TestAnEngagementBoundTokenIsAnsweredWithTheSame404AsANonMember`), which is the layer that has to
carry the binding from the resolved token into `authz.Can`. **M3 should add the real-endpoint case
when it adds the endpoints.**

**`M1-014`** should fold `TestM1011NoProtectedRouteIsReachableWithoutACredential` into its matrix if
it fits more naturally there, as this ticket's Tests section suggests. It currently lives in
`internal/httpapi/servicetoken_test.go` and walks the real router, so it covers endpoints added
later without being edited.
