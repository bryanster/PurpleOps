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

- [ ] **Regression:** a platform `member` gets 403 on every endpoint in this ticket, including
      `PATCH /users/{id}` targeting themselves with a role change. Covered by `M1-014`; assert here
      at HTTP level too.
- [ ] A user cannot change their own platform role through **any** endpoint, including `PATCH
      /users/me` — because the field does not exist in that schema. Demonstrate the request being
      rejected by spec validation (400), not by a handler check.
- [ ] The last active admin cannot be demoted, disabled, or deleted — 409 with a clear message.
      Test with exactly one admin, and with two (the second case must succeed).
- [ ] Demoting a user takes effect on their **existing** session immediately, without re-login.
- [ ] Disabling a user invalidates their sessions and their service tokens (`M1-011`) at once.
- [ ] Creating a duplicate email (any case) is 409, not 500.
- [ ] The user list never includes password hashes, TOTP secrets, or session tokens. Assert on the
      serialized JSON, not on the struct.
- [ ] Search and pagination behave with 1,000 users; `limit` is capped per `M0B-005`.
- [ ] Every mutation appears in the activity log with the acting admin and the before/after values.

## Tests

- HTTP tests for every endpoint × (admin, member, non-member, unauthenticated).
- Last-admin protection, both branches.
- Immediate-effect tests for demotion and disabling (session and token).
- A serialization test asserting no secret field is present in any response.
