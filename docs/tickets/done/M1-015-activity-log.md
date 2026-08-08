# M1-015 — Append-only activity log

**Milestone:** M1 · **Size:** M · **Depends on:** M1-001, M1-013

## Why

`PLAN.md` §2 gives this table two jobs at once: it "drives the SSE feed **and** the report
timeline". So it is not an afterthought audit table bolted on later — M4's live collaboration and
M6's engagement narrative both read from it. Building it now, with the right shape, is what makes
those cheap.

## Scope

**In**

- Migration matching `PLAN.md` §2:
  ```
  activity  id, engagement_id (nullable — platform events have none), actor_id, verb,
            object_type, object_id, delta JSON, at
  ```
- `internal/events.Activity` recorder: `Record(ctx, entry) error`, written inside the **same
  transaction** as the change it describes, so the log can never disagree with the data. This is the
  central design constraint of the ticket.
- Verb vocabulary as constants (`user.created`, `user.role_changed`, `session.login_failed`,
  `mfa.enrolled`, `mfa.recovery_used`, `token.created`, `token.revoked`, `login.throttled`, …).
  M3–M6 extend it; the naming pattern `object.past_tense_verb` is fixed here.
- `delta` holds before/after for changed fields, with **secrets and PII redacted** — never a
  password hash, token secret, or TOTP secret.
- Append-only: no update or delete API, and a comment in the migration saying so. Retention/pruning
  is a `blctl` command, not an endpoint.
- Read API: `GET /activity` (platform, admin only) and `GET /engagements/{id}/activity` (members),
  paginated with the standard cursor convention (`M0B-005`), filterable by actor, verb, object.
- Indexes for `(engagement_id, at DESC)` and `(actor_id, at DESC)`.
- M1 wiring: log every security-relevant event — login success/failure, lockout, logout, password
  change, MFA enrol/disable/recovery-use, role change, user create/disable, token create/revoke,
  SSO first-login and provisioning.

**Out**

- SSE delivery (`M4`) — but the schema must not need changing for it. Sanity-check the shape against
  that use before merging.
- A UI beyond a basic admin list (`M1-017`).

## Acceptance criteria

- [x] An activity row is written in the same transaction as its change: if the change rolls back, no
      row exists. Test with a deliberately failing write.
- [x] There is no code path that updates or deletes an activity row.
- [x] `delta` never contains a password hash, token secret, TOTP secret, or session token — a test
      records a full set of M1 events and greps the serialized rows for known secret values.
- [x] Failed logins record the attempted email and source IP but **no password material**, and are
      readable by an admin.
- [x] A non-admin cannot read the platform activity feed; a non-member cannot read an engagement's
      (via `M1-013`, not a handler check).
- [x] Listing 10,000 entries is paginated and the query uses the index (check the plan; DuckDB makes
      this easy).
- [x] Timestamps are UTC and ordering is stable for entries in the same millisecond (UUIDv7 id as
      tiebreaker — the reason for that ID choice).

## Tests

- Transactional-consistency test (the important one).
- Redaction test across all M1 verbs.
- Authorization tests through the matrix in `M1-014`.
- Pagination and ordering tests, including same-timestamp entries.

## Implementation notes

- **Recorder API.** Ticket sketched `Record(ctx, entry)`. The implementation is
  `events.Log.Record(ctx, tx, entry)` so the caller's write transaction is explicit, plus
  `RecordAlone(ctx, entry)` for events with no sibling mutation (failed login, lockout,
  first token use). Same-tx is the default for Create/Revoke paths via `identity.After` hooks.
- **Column `"at"`.** SQL reserved word; quoted in migration and queries. Indexes use `"at" DESC`.
- **Store layout.** SQL in `internal/store/activity`; verbs/redaction/facade in `internal/events`.
  No Update/Delete methods exist on the repository.
- **Ownership default.** `GET /engagements/{id}/activity` is engagement-scoped, so the server
  needs an `Ownership` loader. Until M3, `NewMembershipOwnership` answers Seats from
  `engagement_member` and treats any path engagement id as existing (not blind). `newServer`
  installs it when `Deps.Ownership` is nil. M3 replaces Facts with a real engagement row check.
- **Authz.** Platform feed: `activity.read` / `ResourcePlatform` / admin. Engagement feed:
  `engagement.read` / `ResourceEngagement` / members + admins; non-members concealed to 404.
- **M1 wiring.** session.login/logout (same-tx After on Sessions.Create/Revoke);
  session.login_failed + login.throttled (RecordAlone); token.created/revoked (same-tx);
  token.first_used (RecordAlone); mfa.enrolled/disabled/recovery_*; user.password_changed;
  user.created + sso.provisioned/linked + user.role_changed on federated paths.
  Admin user create/disable/role-change API is M1-016 (verbs ready).
- **Cursor.** Opaque base64url of `RFC3339Nano|id`; list orders by `"at" DESC, id DESC`.
- **SSE readiness.** Rows are append-only, UUIDv7-ordered, engagement-scoped index matches
  M4 Last-Event-ID plans; no schema change expected for the hub.
