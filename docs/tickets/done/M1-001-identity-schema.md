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

- [x] Email uniqueness is **case-insensitive**: inserting `Alice@x.com` after `alice@x.com` fails.
      DuckDB has no `citext`; store a normalized lowercase column and enforce uniqueness on that,
      keeping the original for display. Document the approach in the migration.
- [x] Role columns are constrained to their allowed values (CHECK constraint), so an invalid role
      cannot be written even by a bug. `PLAN.md` §4's "field safety comes from the schema" applies
      to roles too.
- [x] Deleting a user does not orphan rows: decide and implement cascade vs. restrict per FK, and
      write the reasoning in the migration comment. (Recommendation: soft-delete users via `status`,
      restrict hard deletes.)
- [x] Every repository method takes `context.Context` as its first argument.
- [x] Writes go through `store.Write` (`M0B-003`); reads use the read pool. A test asserts no
      repository holds its own `*sql.DB`.
- [x] Timestamps are UTC.
- [x] `popsctl db info` shows the new tables.

## Tests

- Store integration tests per repository: create, get by ID, get by email (case variations), update,
  list, not-found returns `apierr.NotFound`.
- Constraint tests: duplicate email, invalid role, invalid status — each must error.
- Migration-from-empty test (already the pattern from `M0B-004`).

---

## Implementation notes

**`0002_identity.sql` · `internal/store/identity/` · `internal/store/constraint.go`**

### DuckDB has no `ON DELETE CASCADE`

`FOREIGN KEY … ON DELETE CASCADE` does not parse — *"FOREIGN KEY constraints cannot use CASCADE, SET
NULL or SET DEFAULT"*. `RESTRICT` is the only referential action available, so the ticket's
"decide cascade vs. restrict per FK" was decided for us. It is the recommendation anyway, and the
migration says so rather than leaving a reader to wonder: accounts are retired with
`status = 'disabled'`, which keeps a person's name on what they wrote, and a hard delete is refused
until the dependent rows are cleared by hand. `RESTRICT` is written out explicitly even though it is
the default, and it is enforced — `TestDeletingAUserWhoOwnsAnythingIsRefused` removes one kind of
dependent row at a time and checks the delete stays refused until the last is gone.

### `"user"` is quoted everywhere

DuckDB accepts `user` as a bare table name; PostgreSQL and the standard do not. It is quoted in the
migration and in every query, for the escape hatch in `PLAN.md` §1. `popsctl db info` already quoted
catalog-derived identifiers, so it lists `app.user` unchanged — asserted in `internal/cli/db_test.go`.

(Related: `at` *is* reserved in DuckDB and cannot be a column name. Nothing here is called that, but
it is worth knowing before naming a timestamp column in M3.)

### Case-insensitive email: two columns and a CHECK, not a generated column

`email` keeps what was typed; `email_normalized` is `UNIQUE` and is what every lookup uses. The two
are tied together by `CHECK (email_normalized = lower(trim(email)))`, which is the part that matters
— without it, uniqueness could be evaded by writing an unnormalized key
(`TestTheSchemaRefusesAMismatchedNormalizedEmail`).

A generated column would express this with less to get wrong, and DuckDB supports one with a unique
index over it — verified. It was rejected on portability: DuckDB offers only `VIRTUAL` generated
columns, PostgreSQL only `STORED`.

**Normalization is done in SQL, never in Go.** Every statement that touches an email wraps it in
`lower(trim(?))`, so there is one definition of "the same address" and Go's `strings.ToLower` cannot
disagree with DuckDB's `lower()` on some Unicode edge and trip the CHECK. The repository therefore
exposes no `NormalizeEmail` helper — comparing addresses means asking the database.

### Package placement

Repositories are in `internal/store/identity/` rather than in package `store` itself. `internal/store`
is the DuckDB-specific layer and would become very large by M3 if every table's queries landed in it;
the subpackage keeps the connection machinery and the schema-specific SQL apart, and matches the
existing `internal/store/migrate`. The `DB` interface each repository depends on is declared in the
consuming package, as the ticket asks.

`store.IsUniqueViolation` is new and lives in `internal/store`, because recognising a duplicate-key
error is a fact about DuckDB and that package is where that knowledge is allowed. It is what turns a
taken email into an `apierr.Conflict` instead of a 500.

### Surface beyond "create, read, update, list"

The ticket's test list implies the user repository's shape. Three methods were added because the
columns they write would otherwise have no writer at all:

- `Users.SetLastLoginAt` — `Update` deliberately does *not* touch `last_login_at`, so a
  read-modify-write of a stale `User` cannot roll back a login that happened while the account was
  being edited.
- `Sessions.RevokeAllForUser` — `PLAN.md` §4's "rotation on privilege change" needs it in M1-003.
- `Sessions.DeleteExpired` — takes a retention cutoff rather than "now", because the rows are the
  record of who was signed in.

There is no `Users.Delete` (retire via `status`) and no `Identities.Update` (a login method is
attached or detached; silently repointing one at another user is how an account gets taken over).

### `engagement_member.engagement_id` has no foreign key

As the ticket permits, and preferred over a placeholder `engagement` table for M3 to inherit or
migrate away from. `TestAnUnknownEngagementIsAccepted` documents the gap so a future reader does not
read it as an oversight; M3's migration adds the table and the reference.

### `storetest.Migrated`

New helper: `storetest.New` plus the embedded migrations, with the migrator's logging discarded. It
is what makes every repository test run against the same schema a server starts against, so SQL that
disagrees with a migration fails in the package that owns it.

### Verified

`make lint test build` green; `make generate` produced no drift. Also `go test -race ./internal/store/...`,
and `popsctl migrate up` / `popsctl db info` against a fresh file, which lists all four new tables.
