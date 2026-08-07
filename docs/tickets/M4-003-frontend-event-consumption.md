# M4-003 — Frontend event consumption + precise cache invalidation

**Milestone:** M4 · **Size:** M · **Depends on:** M4-001, M0B-009, M3-014

## Why

Live updates are useless if every event refetches the world, and fragile if each
page hand-rolls `EventSource`. One engagement-scoped consumer should map verbs
→ TanStack Query keys and share reconnect plumbing with content sync
(`useEventSource`).

## Scope

**In**

- Extend `web/src/lib/use-event-source.ts` as needed:
  - Stable parse of hub envelope (`id`, `type`, nested `data`).
  - Expose last event id to the caller (for `M4-004` `Last-Event-ID` — may stub
    until that ticket wires the header/query).
  - Reconnect/backoff remains correct when the tab backgrounds.
- Engagement subscription helper (e.g. `useEngagementEvents(engagementId)`):
  - Subscribes to `engagement.{id}` while the engagement layout is mounted.
  - **Precise invalidation map** from verb → query keys (engagement detail,
    scenarios, steps, executions list/detail, comments, evidence, findings,
    members, activity list). Prefer partial keys already used in
    `web/src/features/engagements/`.
  - Unknown verbs: invalidate engagement activity list only (or no-op with
    dev log) — do **not** blanket `invalidateQueries()` on the whole cache.
- Mount the hook from `EngagementLayout` (or equivalent) so workbook, findings,
  and settings all share one stream.
- Unit tests for the verb → key mapper (table-driven).
- No presence UI here (`M4-006`); ignore presence event types if any leak early.

**Out**

- Catch-up replay semantics (`M4-004`) — hook may accept `lastEventId` option
  but behaviour can be “not sent yet”.
- Live conflict toasts (`M4-005`).
- Activity rail chrome (`M4-008`).
- Comment unread badges (`M4-007`).

## Acceptance criteria

- [ ] With two browsers (or two sessions), a mutation in A causes B’s open
      engagement queries to refetch the affected keys without full page reload.
- [ ] An `execution.red_updated` event does **not** refetch unrelated admin
      content library queries.
- [ ] Leaving the engagement route closes the EventSource (no leaked
      subscribers growing in devtools).
- [ ] Mapper tests cover at least: execution red/blue, step reveal, comment
      create, finding update, member added, scenario reorder.

## Tests

- Vitest: mapper table.
- Component/hook test with mock EventSource or injected `onEvent`.
- Optional lightweight Playwright smoke deferred to `M4-005` / `M4-009`.

## Notes for the implementer

- Match existing query key conventions in engagements features — do not invent a
  second key scheme.
- Session cookie + `withCredentials` already in `useEventSource`.
- Keep the invalidation map in one module; M4-005/007/008 import it rather than
  forking switch statements.
