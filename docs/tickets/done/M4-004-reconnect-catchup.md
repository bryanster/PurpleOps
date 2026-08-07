# M4-004 — Reconnection + `Last-Event-ID` + blind delivery filter

**Milestone:** M4 · **Size:** L · **Depends on:** M4-002, M4-003

## Why

Corporate Wi‑Fi and laptop sleep drop SSE. Without catch-up, blue misses red’s
score and the war-room demo lies. Activity ids are UUIDv7 and already power the
feed — use them as the cursor (`M4-EPIC` decision; closes the M2 “best-effort
only” gap).

The same ticket installs **per-subscriber visibility**: red and blue share
`engagement.{id}`, so filtering must happen at delivery/replay, not at Publish.
Id-only payloads still leak step existence to blue.

## Scope

**In**

- Server:
  - On `GET /api/v1/events`, honour `Last-Event-ID` (header; optional query twin
    if needed for EventSource limitations — document the chosen approach).
  - For each authorized `engagement.{id}` topic in the subscription, load
    activity rows with `id > cursor` (UUIDv7 order) **or** `(at, id)` order
    consistent with activity list — pick one, test it, document it.
  - Replay as SSE events **before** live tail, same envelope as live fan-out
    (`id` = activity id, etc.). SSE frames set `id: {activityId}` so browsers
    resend `Last-Event-ID` automatically.
  - **Blind / visibility (mandatory this ticket):**
    - One shared helper (e.g. `VisibleActivity(subject, entry)`) used by:
      1. `Subscription.Allow` on live engagement streams,
      2. catch-up replay,
      3. activity list API if it still returns concealed objects for blue —
         fix the list here if needed.
    - Blue on blind engagements: **drop** events for unrevealed steps and their
      executions/comments/evidence. `step.revealed` is delivered at reveal time;
      subsequent events for that step pass. Lead/red (and seats not under blind
      withhold) see the full stream.
    - Synthetic `stream.gap` / `sync.required` is always allowed.
  - Bounds: max events per reconnect (config, sane default e.g. 500–2000) and/or
    max age. If truncated, send `stream.gap` / `sync.required` so the client
    full-refetches engagement queries, then continue live.
  - Presence is **not** replayed (focus stripping is `M4-006`, reusing the same
    step-visibility primitive where possible).
  - Content topics: `Last-Event-ID` remains ignored or no-ops (jobs are
    ephemeral) — document.
- Frontend:
  - Persist last seen event id per engagement (memory + `sessionStorage`).
  - Pass cursor on reconnect; handle `sync.required` with broad engagement
    invalidation.
  - On first subscribe without cursor: live tail only (REST already loaded the
    world).
- Docs: short note in `docs/http.md` or `docs/deploy.md` on cursors, caps, and
  blind delivery.

**Out**

- Multi-node sticky sessions / external event bus.
- Guarantees for presence.
- Compacting old activity (ops/`blctl` later).
- Playwright blind script (`M4-009`) — API tests here are required.

## Acceptance criteria

- [ ] Subscriber disconnects, N engagement mutations occur, reconnect with
      `Last-Event-ID` → receives those N (or `sync.required` if over cap) then
      live events.
- [ ] Blue live subscribe on blind engagement does **not** receive events for
      unrevealed steps (object ids absent).
- [ ] Blue reconnect after create+reveal receives reveal and post-reveal events
      only for that step’s pre-reveal history is withheld.
- [ ] Cursor from another engagement is ignored/harmless (no cross-tenant
      replay).
- [ ] Over-cap replay signals client refetch; client recovers without stuck UI.
- [ ] Stalled client during replay still subject to buffer eviction rules (no
      deadlock).
- [ ] Activity list for blue matches the same visibility rules as SSE (no REST
      leak that SSE fixed).

## Tests

- Handler/integration: replay order, cap, blind filter live + replay,
  unauthorized topic.
- Two-subscriber test: red receives step.created; blue does not until reveal.
- FE unit: cursor storage + `sync.required` invalidation.
- Race on subscribe+replay+live interleaving.

## Notes for the implementer

- EventSource resends `Last-Event-ID` when `id:` was set on frames — verify
  before inventing query params.
- Do not replay into `content.jobs` from activity.
- Keep the visibility helper free of HTTP types where practical so presence
  (`M4-006`) can reuse step/execution revealed checks.

## Implementation notes

- **Cursor ordering:** Activity rows are replayed in `id > cursor ORDER BY id ASC` (UUIDv7 order). This is consistent with the fan-out id field and gives chronological replay.
- **Query param twin:** Added `lastEventId` query parameter to OpenAPI spec alongside the `Last-Event-ID` header. The browser's EventSource auto-sends the header on reconnect, but first connection after page load uses the query param populated from `sessionStorage`.
- **Revealed field:** The fan-out path now includes a `revealed` boolean in engagement-scoped event data for step/execution/evidence/comment objects. This is computed post-commit via `RevealLookup` so it sees the data committed in the same transaction. The `VisibleActivity` helper reads this field for blind filtering; nil means "not step-scoped, always visible."
- **Stream gap:** When replay exceeds `BLACKLIGHT_EVENTS_MAX_REPLAY` (default 500), a `stream.gap` event is sent before live tail. The frontend handles this by invalidating all engagement queries.
- **Config:** `BLACKLIGHT_EVENTS_MAX_REPLAY` (default 500, 0 disables replay).
- **Files changed:**
  - `internal/config/config.go` + `validate.go` — new `MaxReplayEvents` field
  - `internal/events/activity.go` — `RevealLookup`, `revealed` in `buildEventData`, `lookupRevealed`
  - `internal/events/blind.go` — `VisibleActivity`, `RevealLookup`, `EventData`, `StepIDForEvent`
  - `internal/events/replay.go` — `ReplayAfter`, `NewGapEvent`
  - `internal/events/hub.go` — `TypeStreamGap`, `TypeSyncRequired` constants
  - `internal/store/activity/activity.go` — `ReplayAfterCursor`
  - `internal/httpapi/events.go` — rewritten `SubscribeEvents` with replay + Allow filter, `streamWithReplay`, `IsStepRevealed`
  - `internal/httpapi/server.go` — wired `eventsMaxReplay` + `SetRevealLookup`
  - `api/openapi.yaml` — `lastEventId` query param twin
  - `web/src/lib/use-event-source.ts` — `initialLastEventId` option, `eventsUrl` accepts cursor
  - `web/src/features/engagements/use-engagement-events.ts` — cursor persistence in `sessionStorage`, `stream.gap` handling
  - `.env.example` — `BLACKLIGHT_EVENTS_MAX_REPLAY`
- After this ticket, blind war-room demos are allowed; before it, they are not.
