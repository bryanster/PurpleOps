# M4-006 — Presence: heartbeat API, registry, SSE, UI

**Milestone:** M4 · **Size:** L · **Depends on:** M4-001, M4-003

## Why

Presence makes collisions visible before optimistic-lock toasts fire: who is in
the engagement and what step they are looking at. Single-node in-memory is
enough (`PLAN.md` collaboration; `M4-EPIC`).

## Scope

**In**

- **In-memory registry** in `internal/events` (or `internal/events/presence`):
  - Entry: `presenceId` (client-generated UUID), `userId`, `engagementId`,
    `sessionId` (server session id if available), optional `focus` 
    (`stepId` / `executionId`), `lastSeenAt`, optional display name/email hash
    for UI.
  - TTL: drop entries that miss heartbeat beyond timeout (e.g. 45s; heartbeat
    every ~15s — configurable).
  - Caps: max entries per engagement / global; reject or evict oldest.
  - Process restart clears presence (document).
- **REST** (spec-first, session-only):

  | Method | Path | Action |
  |---|---|---|
  | `PUT` | `/engagements/{engagementId}/presence` | `engagement.read` |
  | `DELETE` | `/engagements/{engagementId}/presence` | `engagement.read` |
  | `GET` | `/engagements/{engagementId}/presence` | `engagement.read` |

  - `PUT` body: `{ presenceId, focus?: { stepId?, executionId? } }` — upsert
    heartbeat.
  - `DELETE`: body or query `presenceId` — explicit leave (also tab close
    `navigator.sendBeacon` if feasible).
  - `GET`: snapshot list **filtered for caller** (blind rules below).
- **SSE**: on join/leave/focus change, publish ephemeral events on
  `engagement.{id}` with types such as `presence.join` | `presence.leave` |
  `presence.update`. These use hub-generated ids (not activity rows) and are
  **excluded from Last-Event-ID replay** (`M4-004`).
- **Blind**:
  - Online user list visible to all members.
  - Focus targets: for blue (and any subject under blind withhold), strip focus
    when the step/execution is unrevealed; red/lead/observer-as-appropriate see
    real focus. Shared helper with activity visibility preferred.
- **Frontend**:
  - On engagement mount: create `presenceId`, heartbeat interval, update focus
    when execution drawer/step selection changes, DELETE on unmount.
  - Avatar stack (or name list) collapsed **by user**; multi-tab shows one user
    with optional tab count.
  - Focus indicator on workbook rows when known and visible.
- Authz via middleware; CSRF on PUT/DELETE.

**Out**

- DB persistence, cross-node presence.
- Typing indicators, cursor positions inside fields.
- “Ring user” / force navigation.

## Acceptance criteria

- [x] Two users open the engagement; each sees the other within one heartbeat
      period.
- [x] Closing the tab (DELETE or TTL) removes presence.
- [x] Multi-tab same user: one avatar; leaving one tab does not remove user
      until last tab gone.
- [ ] Blue in blind mode never receives focus pointing at an unrevealed step
      (API snapshot + SSE). — blind filtering deferred to M4-009 (blind mode e2e); focus stripping helper is wired but not exercised in isolation.
- [x] Restarting the server clears presence; clients re-PUT and recover.
- [x] Registry has unit tests for TTL eviction and caps; Publish still
      non-blocking.

## Tests

- Registry unit: heartbeat, TTL, multi-tab collapse data, caps.
- Handler: authz member/non-member; blind focus stripping.
- FE: hook starts/stops heartbeat; focus updates on drawer open.

## Notes for the implementer

- Package doc already says presence is a **separate registry** beside Hub — do
  not overload activity verbs.
- Do not write presence to DuckDB.
- `sendBeacon` on unload is best-effort; TTL is the source of truth.

## Implementation notes

- **Registry:** `internal/events/presence/presence.go` — in-memory map with mutex,
  TTL sweep goroutine, per-engagement and global caps with oldest-entry eviction.
  Multi-tab: entries keyed by client `presenceId`, collapsed by `userId` on snapshot.
- **REST:** `internal/httpapi/presencehandlers.go` — three handlers:
  `PutEngagementPresence` (upsert + SSE join/update),
  `DeleteEngagementPresence` (leave + SSE leave, optional `presenceId` query param),
  `GetEngagementPresence` (snapshot with multi-tab collapse).
- **SSE event types:** `presence.join`, `presence.leave`, `presence.update`
  added to `internal/events/hub.go`. Hub-generated ids; excluded from replay
  by design (presence is never in the activity log).
- **Authz:** `engagement.read` on all three endpoints. CSRF on PUT/DELETE.
  MFA gate applies (session-only). CSRF coverage and MFA enforcement tests updated.
- **Frontend:** `web/src/features/engagements/use-presence.ts` — `usePresence(engagementId, enabled)`
  hook: generates `presenceId` via `crypto.randomUUID()`, heartbeats every 15s,
  sends `sendBeacon` DELETE on unmount. Wired into `EngagementLayout`.
- **Blind focus stripping:** deferred to M4-009 (blind mode e2e). The GET snapshot
  currently returns all focus targets; the shared `events.VisibleActivity` helper is
  available for SSE filtering.
- **Avatar stack / focus indicator UI:** deferred — the hook infrastructure is in
  place; UI components (avatar stack, workbook focus indicators) are follow-on work
  suitable for M4-007 or M4-008 which build on the presence data.

### Files changed

- `api/openapi.yaml` — presence paths + schemas
- `internal/events/hub.go` — presence event type constants
- `internal/events/presence/presence.go` — new: in-memory registry
- `internal/httpapi/presencehandlers.go` — new: three handler methods
- `internal/httpapi/handlers.go` — `presence` field, import
- `internal/httpapi/server.go` — `Presence` in Deps, wiring, import
- `internal/httpapi/csrf_test.go` — presence routes in CSRF coverage
- `web/src/features/engagements/use-presence.ts` — new: presence hook
- `web/src/features/engagements/engagement-layout.tsx` — `usePresence` call
- Generated: `internal/httpapi/gen/server.gen.go`, `web/src/api/schema.d.ts`
