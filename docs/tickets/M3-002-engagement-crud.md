# M3-002 — Engagement CRUD, status lifecycle, attack pin, mode

**Milestone:** M3 · **Size:** M · **Depends on:** M3-001, M2-007, M1-013

## Why

Engagements are the root of every workbook. Pinning `attack_version` is what stops a report
re-mapping itself when ATT&CK updates mid-engagement (`PLAN.md` §2, `docs/content-copy-on-use.md`).
Mode and `auto_reveal_on_start` encode blind-policy knobs locked in `M3-EPIC`.

## Scope

**In**

- Spec-first endpoints (`api/openapi.yaml`):

  | Method | Path | Action | Notes |
  |---|---|---|---|
  | `POST` | `/engagements` | `engagement.create` | Creator becomes **lead** member in same txn |
  | `GET` | `/engagements` | (authn) | List engagements the caller can read (member or admin) |
  | `GET` | `/engagements/{engagementId}` | `engagement.read` | Detail |
  | `PATCH` | `/engagements/{engagementId}` | `engagement.manage` | name, client, description, dates, mode, auto_reveal_on_start |
  | `POST` | `/engagements/{engagementId}/status` | `engagement.manage` | Explicit status transition body |
  | `DELETE` | `/engagements/{engagementId}` | `engagement.delete` | Deletes workbook graph (app-enforced order) |

- **Create body:** `name` (required), `client`, `description`, `starts_on`, `ends_on`,
  `attack_version` (required — must pass `attackpin.AssertPinned`), `mode` (default `standard`),
  `auto_reveal_on_start` (default false).
- **Status lifecycle:** `draft` → `active` → `closed` → `archived`, plus `draft` → `closed` if never
  run. Disallow illegal skips (e.g. `archived` → `active`) with 409 + problem detail. Document the
  state machine in `docs/` or OpenAPI description.
- **Pin immutability:** Once any **step** exists on the engagement, `attack_version` cannot change
  (409). Before that, PATCH may change pin if `AssertPinned` succeeds.
- **Mode:** `standard` \| `blind`. Switching `blind` → `standard` is allowed (everything becomes
  visible); `standard` → `blind` allowed only if no step has `revealed_at` set **or** lead confirms
  via explicit flag — pick one, default **allow with warning activity** is fine if simpler: allow
  and leave `revealed_at` as-is (unrevealed stay hidden).
- Delete: one write transaction removes comments/revisions, evidence links (blob refcount via
  `M3-009` helper or deferred if evidence ships later — coordinate), findings, executions, steps,
  scenarios, members, activity optional retain vs delete (prefer delete engagement-scoped activity),
  engagement. **409** if you choose soft-delete only — hard delete is fine for v1 single-tenant.
- Activity (engagement-scoped): `engagement.created`, `.updated`, `.status_changed`, `.deleted`.
- List supports filter `status`, cursor/limit pagination consistent with existing APIs.
- Non-members: conceal existence (404) per `M1-013` / authz conceal on engagement resources.

**Out**

- Membership add/list beyond creator-as-lead (`M3-003`).
- Scenarios/steps (`M3-004`/`M3-005`).
- UI.

## Acceptance criteria

- [ ] Create with valid pin succeeds; creator has `lead` membership; activity row written.
- [ ] Create with unknown/disabled ATT&CK version → 400/409 from AssertPinned mapping.
- [ ] PATCH pin after a step exists → 409.
- [ ] Status transition illegal edge → 409; legal edge updates and logs activity.
- [ ] Member of engagement A cannot GET engagement B (404 conceal).
- [ ] Delete removes member rows and child workbook rows; subsequent GET is 404.
- [ ] `make generate` clean; authz actions already exist (no new action required unless you split
      status — prefer not to).

## Tests

- Handler + store tests for create/list/get/patch/status/delete.
- Pin freeze after step (insert step via store).
- AssertPinned failure mapping.
- Authz: non-member conceal; member read; lead manage; observer cannot manage.

## Notes for the implementer

- Reuse identity membership repository for creator lead insert.
- Do not invent a second pin normalizer — call `attackpin` only.
- Engagement list for admins: all engagements; for members: only memberships. Match `docs/authz.md`.
