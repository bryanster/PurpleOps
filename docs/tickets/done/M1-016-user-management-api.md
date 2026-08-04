# M1-016 — Admin user management API

**Milestone:** M1 · **Size:** M · **Depends on:** M1-013, M1-014, M1-015

## Why

This is the endpoint family v1 shipped **unprotected** — `/manage/access` was reachable by any
logged-in user, who could grant themselves Admin (`PLAN.md` §4). Rebuilding it after `M1-013` and
`M1-014` exist means it is protected by construction, and the regression test already covers it
before the code is written.

## Scope

**In**

- Endpoints, all requiring `user.manage` (platform admin):
  - `GET /users` — paginated, filterable by status/role, searchable by email or name.
  - `POST /users` — create with a role and status; either sets a password or marks the account
    `invited` for SSO.
  - `GET /users/{id}`, `PATCH /users/{id}` — display name, platform role, status, `mfa_enforced`.
  - `POST /users/{id}/disable` / `enable`.
  - `DELETE /users/{id}` — soft delete (status → `disabled`), per `M1-001`. Hard delete is not
    exposed.
  - `POST /users/{id}/sessions/revoke` — kill all sessions for a user, immediately.
- Self-service subset any user may call on themselves: `PATCH /users/me` (display name only).
  Platform role and status are **not** in that request schema — separate schema, not a field filter
  (`PLAN.md` §4, "field safety comes from the schema").
- Every mutation writes to the activity log with before/after (`M1-015`).
- Changing a user's platform role rotates their sessions (`M1-003`) so the new role takes effect
  immediately in both directions.

**Out**

- Engagement membership management — that belongs to `M3` with the engagement resource, and uses
  the `member.manage` action already defined in `M1-012`.
- Email invitations. No mail transport in v1; `POST /users` returns an invite link the admin passes
  on manually.

## Acceptance criteria

- [x] **Regression:** a platform `member` gets 403 on every endpoint in this ticket, including
      `PATCH /users/{id}` targeting themselves with a role change. Covered by `M1-014`; assert here
      at HTTP level too.
- [x] A user cannot change their own platform role through **any** endpoint, including `PATCH
      /users/me` — because the field does not exist in that schema. Demonstrate the request being
      rejected by spec validation (400), not by a handler check.
- [x] The last active admin cannot be demoted, disabled, or deleted — 409 with a clear message.
      Test with exactly one admin, and with two (the second case must succeed).
- [x] Demoting a user takes effect on their **existing** session immediately, without re-login.
- [x] Disabling a user invalidates their sessions and their service tokens (`M1-011`) at once.
- [x] Creating a duplicate email (any case) is 409, not 500.
- [x] The user list never includes password hashes, TOTP secrets, or session tokens. Assert on the
      serialized JSON, not on the struct.
- [x] Search and pagination behave with 1,000 users; `limit` is capped per `M0B-005`.
- [x] Every mutation appears in the activity log with the acting admin and the before/after values.

## Tests

- HTTP tests for every endpoint × (admin, member, non-member, unauthenticated).
- Last-admin protection, both branches.
- Immediate-effect tests for demotion and disabling (session and token).
- A serialization test asserting no secret field is present in any response.

## Implementation notes

### Sessions are not rotated on a role change

The scope says a role change "rotates their sessions"; the acceptance criteria say a demotion takes
effect on the existing session "without re-login". Those cannot both hold: an administrator cannot
hand a rotated session token to somebody else's browser, so rotating or revoking the target's
sessions *is* forcing a re-login.

Implemented as the criterion, not as the scope line. `authn.Authenticate` re-reads the account on
every request, so a demotion applies at the target's next request and a promotion applies just as
fast — in both directions, which is what the scope line was after. Nothing anywhere caches a
platform role, which is what `PLAN.md` §4's "rotation on privilege change" is protecting against.
`TestDemotionTakesEffectOnAnExistingSession` drives both directions on one never-refreshed cookie.

Disabling *does* revoke sessions, and for a different reason: not to make the change take effect (the
same live read already refuses a session whose owner is not active) but so the rows record when
access stopped, and so enabling the account later does not silently restore a browser tab.

### `invited` had to become claimable

`POST /users` without a password creates an `invited` account, per the scope. Before this ticket
nothing could ever sign one in: `Service.Login` refuses a non-active account and so did
`SignInWithFederatedIdentity`, and there is no claim endpoint in scope. `invited` was therefore a
dead end.

`authn.claimInvitation` closes it: a verified federated sign-in flips `invited` → `active`, which is
what `identity.StatusInvited`'s own comment ("exists but has never been claimed") already described.
A `disabled` account is untouched — somebody turned that off deliberately. Local password sign-in
still refuses an invited account, because an invited account has no password; creating one with both
`status: invited` and a password is a `400` rather than a password nobody could ever use.

### The invite link is the sign-in page

The scope asks `POST /users` to return "an invite link the admin passes on manually". There is no
invite-token mechanism in the schema and no claim endpoint in scope, so `inviteUrl` is this
deployment's `/login`, built from the configured base URL and never from a request header. It carries
no credential and grants nothing. A single-use invite token with its own claim endpoint would be a
migration and a new public endpoint — a follow-up, not this ticket.

### Where the code went

The ticket has no **Files** section, so: the store gained `Users.Page` (filter, search, cursor) and
`identity.CountActiveAdmins`, plus `after ...After` hooks on `Users.Create` and `Users.Update` — the
mechanism `Sessions` and `ServiceTokens` already had. `Users.List` is gone; `Page` is the one way to
read a set of accounts, so the 1,000-account criterion is not something a second listing path can
quietly bypass.

The rules live in `internal/authn/users.go`, beside the rest of the identity service, which already
holds the users repository, the session manager and the activity log. The last-administrator guard is
an `After` hook: it counts *inside* the write transaction and returns `apierr.Conflict`, which rolls
the change back. Counting first and writing second would be a check two administrators demoting each
other simultaneously could both pass. The guard is ordered before the activity hooks, so a refused
change leaves no row claiming it happened (`TestARefusedChangeLeavesNoActivityRow`).

`internal/httpapi/userhandlers.go` translates and nothing else. Roles and statuses travel as the
strings they arrived as, the way token scopes do: a handler that could name a role is a handler that
could decide with one, and `TestNoHandlerDecidesForItself` fails the build over the import.

### Activity: one row per kind of change

`changeRecords` writes `user.role_changed` for a role, `user.disabled`/`user.enabled` for a status,
and one `user.updated` carrying everything else — all on the same transaction as the write. A patch
that changes a role *and* disables an account writes two rows, because two things happened and
somebody filtering for either should find it. Three verbs are new in `internal/events`:
`user.updated`, `user.enabled`, `user.sessions_revoked`.

### The M1-014 sweep now drives the real endpoint

`authzsweep_test.go` asked, in its own comment, to be repointed when the real endpoint arrived. Its
"manage the users" row is now `PATCH /users/{userId}` against the shipped handler rather than a stub
at v1's `/manage/access` path, and the `/manage/access` fixture is deleted. A row marked `Real`
registers no stub handler — chi would refuse the duplicate route — so for that row "the request
reached the handler" is read off the status rather than off the header the stub sets.

### Not in scope, and still missing

There is no way to clear a login lockout (`M1-004`) from the API; `docs/deploy.md` said "no override
yet (`M1-016`)" and now says plainly that this ticket does not add one. Worth its own ticket if an
operator ever asks.
