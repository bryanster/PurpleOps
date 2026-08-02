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

- [ ] **Regression case (must exist by name):** a request with an invalid/absent token to a
      protected route is 401 and never reaches the handler. Enumerate routes from the generated
      router so this can't rot.
- [ ] A token missing the required scope is 403, not 401 — the distinction is real and clients rely
      on it.
- [ ] **A token cannot exceed its owner's live permissions** (`PLAN.md` §9). Test: create a token
      while the owner is an admin, demote the owner, assert an admin-only call now fails.
- [ ] Revoking the owner's account immediately disables their tokens.
- [ ] An engagement-scoped token gets 403/404 for other engagements — and the response for a
      *different* engagement's resource must not confirm its existence.
- [ ] The secret appears in exactly one response ever (creation) and never in list, logs, or the
      activity feed. A test greps a full log capture for the secret.
- [ ] Expired token → 401; revoked token → 401.
- [ ] Token comparison is constant-time and the lookup is by prefix (not a full-table scan of
      hashes).
- [ ] CSRF is not applied to token-authenticated requests, and this exemption cannot be triggered by
      an invalid `Authorization` header (`M1-005`).
- [ ] Creation, use-first-time, and revocation are written to the activity log.

## Tests

- Auth middleware table: valid / invalid / expired / revoked / malformed / owner-disabled.
- Scope enforcement per scope.
- The two-fence tests (demotion, revocation) — the important ones.
- A route-coverage test asserting every non-public route rejects an unauthenticated request. Fold
  this into `M1-014`'s matrix if it fits more naturally there.
