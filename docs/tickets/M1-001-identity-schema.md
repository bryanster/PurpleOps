# M1-001 — Users, sessions and membership schema

**Milestone:** M1 · **Size:** M · **Depends on:** M0B-004

## Why

Everything in M1 needs somewhere to live. Critically, `PLAN.md` §4 defines **two levels** of role —
platform (`admin` | `member`) and per-engagement (`lead` | `red` | `blue` | `observer`) — and v1's
failures came partly from having only one, fuzzily-defined level. The schema should make the
distinction impossible to blur.

## Scope

**In**

- Migration `0002_identity.sql` in the `app` schema:

| Table | Columns (indicative) |
|---|---|
| `user` | `id`, `email` (citext-equivalent, unique), `display_name`, `password_hash` (nullable — SSO users have none), `platform_role` (`admin`\|`member`), `status` (`active`\|`disabled`\|`invited`), `mfa_enforced`, `created_at`, `updated_at`, `last_login_at` |
| `identity` | `id`, `user_id`, `provider` (`local`\|`oidc`\|`saml`), `subject`, `created_at` — unique `(provider, subject)`. Lets one user hold several login methods |
| `session` | `id`, `user_id`, `token_hash`, `created_at`, `last_seen_at`, `expires_at`, `revoked_at`, `ip`, `user_agent`, `mfa_satisfied` |
| `engagement_member` | `engagement_id`, `user_id`, `role` (`lead`\|`red`\|`blue`\|`observer`), `added_by`, `added_at` — PK `(engagement_id, user_id)` |

- A minimal `engagement` table (id, name, created_at) **only** if the FK requires it — mark it
  clearly as a placeholder that `M3` replaces. Prefer no FK to a table that doesn't exist yet over
  a half-designed engagement table; note the decision in the migration.
- Indexes for every access path M1 uses: `user(email)`, `session(token_hash)`,
  `session(user_id, expires_at)`, `engagement_member(user_id)`. v1 had none — that's called out in
  `PLAN.md` as a stack problem, so do not repeat it.
- Repositories in `internal/store` for user, identity, session, membership — constructor-injected,
  interface-defined in the consuming package.

**Out**

- Service tokens (`M1-011` owns its own table).
- Any password/session *logic* — this is storage only.

## Acceptance criteria

- [ ] Email uniqueness is **case-insensitive**: inserting `Alice@x.com` after `alice@x.com` fails.
      DuckDB has no `citext`; store a normalized lowercase column and enforce uniqueness on that,
      keeping the original for display. Document the approach in the migration.
- [ ] Role columns are constrained to their allowed values (CHECK constraint), so an invalid role
      cannot be written even by a bug. `PLAN.md` §4's "field safety comes from the schema" applies
      to roles too.
- [ ] Deleting a user does not orphan rows: decide and implement cascade vs. restrict per FK, and
      write the reasoning in the migration comment. (Recommendation: soft-delete users via `status`,
      restrict hard deletes.)
- [ ] Every repository method takes `context.Context` as its first argument.
- [ ] Writes go through `store.Write` (`M0B-003`); reads use the read pool. A test asserts no
      repository holds its own `*sql.DB`.
- [ ] Timestamps are UTC.
- [ ] `popsctl db info` shows the new tables.

## Tests

- Store integration tests per repository: create, get by ID, get by email (case variations), update,
  list, not-found returns `apierr.NotFound`.
- Constraint tests: duplicate email, invalid role, invalid status — each must error.
- Migration-from-empty test (already the pattern from `M0B-004`).
