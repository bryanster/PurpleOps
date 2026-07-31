# M4 — Collaboration (epic)

**State:** needs refinement · **Depends on:** M3

## Goal

Make the workbook feel like one shared room: SSE live updates, presence, comments and activity in
context, and blind mode working end to end (`PLAN.md` §2 decisions table, §4).

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| M4-001 | SSE hub | `GET /api/v1/events`; per-engagement topics, authorized per subscriber, backpressure and slow-client eviction |
| M4-002 | Activity → event fan-out | The append-only log from `M1-015` is the source; events are derived, not a parallel path |
| M4-003 | Frontend event consumption | `EventSource` with reconnect/backoff, and precise TanStack Query cache invalidation — not a blanket refetch |
| M4-004 | Live execution and score updates | Blue's browser reflects red's progress without reload (`PLAN.md` §9 step 4) |
| M4-005 | Presence | Who is viewing/editing what, with heartbeat and timeout |
| M4-006 | Comment threads in the UI | Real-time, with unread indicators |
| M4-007 | Blind mode end to end | Reveal controls, blue's filtered view, reveal events, and the audit trail of reveals |
| M4-008 | Reconnection correctness | Missed-event catch-up after a disconnect — an event ID / `Last-Event-ID` cursor over the activity log |

## Open questions to resolve before writing tickets

1. **Catch-up guarantees.** Is the SSE stream best-effort with a refetch on reconnect, or does it
   guarantee no missed events via `Last-Event-ID`? The latter is more work but makes M4-004's demo
   trustworthy. Recommendation: activity IDs are already sortable (UUIDv7), so use them as the
   cursor.
2. **Connection limits.** 20 users × several tabs each, one process. Confirm the SSE hub's memory and
   file-descriptor behaviour, and whether a reverse proxy in the deployment guide needs buffering
   disabled (nginx does).
3. **Presence storage.** In-memory (correct for single-node, lost on restart) vs. persisted. Memory
   is almost certainly right — state it explicitly.
4. **Blind-mode reveal policy** — carried over from M3's open questions; must be settled before
   M4-007.
5. **Concurrent editing UX.** If M3 chose optimistic locking, what does the UI show on conflict?
   Presence makes collisions visible, but not impossible.

## Risks

- SSE through corporate proxies is a classic source of "works for me". Document the requirement and
  test through the deployment path in `M0B-011`, not just against the dev server.
- A slow or dead client must never block the writer or the hub. Explicit buffering and eviction
  policy required, with a test that proves a stalled subscriber gets dropped.
