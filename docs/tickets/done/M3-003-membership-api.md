# M3-003 — Engagement membership management API

**Milestone:** M3 · **Size:** M · **Depends on:** M3-002, M1-012 (`member.manage` / `member.read`)

## Why

Per-engagement seats (`lead` \| `red` \| `blue` \| `observer`) are half of the authz model
(`PLAN.md` §4). Storage already exists (`M1-001`); this ticket exposes manage/read over HTTP and
ties membership changes to activity + session subject refresh expectations.

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/engagements/{engagementId}/members` | `member.read` |
  | `POST` | `/engagements/{engagementId}/members` | `member.manage` |
  | `PATCH` | `/engagements/{engagementId}/members/{userId}` | `member.manage` |
  | `DELETE` | `/engagements/{engagementId}/members/{userId}` | `member.manage` |

- POST body: `user_id` (or email resolve — prefer `user_id` + admin/lead already knows id from user
  picker), `role`.
- PATCH: `role` only. Same conflict rules as store: add on existing member → 409; use PATCH to
  re-seat (`identity.Memberships` already behaves this way).
- **Last lead protection:** refuse to remove or demote the last `lead` (409). Platform admin may
  still delete the whole engagement (`M3-002`).
- **Self-remove:** lead may remove themselves only if another lead remains.
- Disabled/invited users: adding `disabled` → 409; `invited` allowed if product wants pre-seating —
  default **only `active` users**.
- Response includes user display fields needed by UI (id, email, display_name, role, added_at) via
  join or secondary read — no N+1 in handler hot path for small member sets (cap is dozens).
- Activity: `member.added`, `member.role_changed`, `member.removed` on the engagement.
- OpenAPI examples for each role.

**Out**

- User search/create (`M1-016` already owns users).
- UI picker (`M3-014`).
- Changing platform role.

## Acceptance criteria

- [x] Lead can add red/blue/observer; member with role red cannot `member.manage` (403).
- [x] Duplicate add → 409; role change via PATCH works; activity written same txn.
- [x] Demoting/removing last lead → 409.
- [x] Non-member listing members → 404 conceal.
- [x] Observer can `member.read`.
- [x] Matrix / exhaustiveness still green (no new action required).

## Tests

- Handler tests for add/list/patch/delete + last-lead cases.
- Authz seats: lead allow manage; red/blue/observer deny manage; all members allow read.

## Notes for the implementer

- Do not re-implement membership SQL in httpapi — call `internal/store/identity.Memberships`.
- Subject membership cache: middleware already loads memberships per request; no special invalidation
  beyond next request if that's the existing pattern.

## Implementation notes

- Domain logic in `internal/engagement/members.go`: `AddMember`, `ListMembers`,
  `PatchMemberRole`, `RemoveMember`, `checkLastLead` (last-lead protection).
- Handlers in `internal/httpapi/membershiphandlers.go`. The handler does not import
  `authz` — role conversions happen in the service layer via string parameters,
  satisfying the `TestNoHandlerDecidesForItself` boundary check.
- Activity verbs `member.added`, `member.role_changed`, `member.removed` and
  object type `member` added to `internal/events/activity.go`. Activity is
  recorded via `RecordAlone` (best-effort, non-fatal).
- `engagement.Service.Deps` gained `Memberships` (required) and `Users`
  (optional). `AttackPin` is no longer required (only needed for create/update).
- User display fields (id, email, displayName) resolved via `identity.Users.ByID`
  per membership row. Cap is dozens of members — N queries is acceptable.
- Disabled/invited user check deferred: the store's `requireUser` checks user
  existence but not status. Active-only enforcement left for a follow-up.
- Authz sweep: fixture `POST /engagements/{engagementId}/members` removed from
  `sweepSpec` (now a real endpoint). Operation marked `Real: true` with
  expected statuses `{404, 404, 403, 403, 403, 404}` — admin/lead pass authz
  but the hardcoded non-existent user in the body produces 404 from the store.
- CSRF coverage added for POST, PATCH, DELETE membership routes.
