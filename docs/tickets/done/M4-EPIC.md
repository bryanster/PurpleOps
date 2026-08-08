# M4 — Collaboration (epic)

**State:** refined · **Depends on:** M3 complete (including **M3-016** gate)

## Goal

Make the workbook feel like one shared room: SSE live updates driven by the
append-only activity log, presence, comments and activity in context, and blind
mode working end to end over the wire (`PLAN.md` §2 decisions table, §4, §9
step 4).

## Decisions (locked)

| Topic | Decision |
|---|---|
| **Catch-up** | **`Last-Event-ID` replay from the activity log.** On subscribe/reconnect, replay durable engagement activity after the cursor (activity row id = UUIDv7). Ephemeral presence is best-effort and is not replayed. |
| **Event payload** | **Invalidate-by-default.** SSE carries `id`, `type` (activity verb or presence type), `at`, and compact object refs (`engagementId`, `objectType`, `objectId`, optional parent ids). Clients invalidate precise TanStack Query keys and refetch. Compact resource deltas are **out of M4** (optional later if chatty). |
| **Fan-out source** | Activity log is the **only** durable source. No parallel “live event” write path. Fan-out runs after successful commit (same pattern as today: record in-txn → publish post-commit). |
| **Fan-out verbs** | **All engagement-scoped activity verbs** (structure, execution, evidence, comment, finding, reveal, membership, engagement status, …). Platform-only verbs never hit engagement topics. |
| **Topics** | Keep `content.jobs` / `content.jobs.{id}`. Add **`engagement.{engagementId}`** only — no per-kind subtopics. Event `type` discriminates. |
| **Blind delivery** | Red and blue share one topic. **Per-subscriber filter at delivery** (and on catch-up replay), not at `Publish`. Drop events whose object is unrevealed to that subject — id-only payloads still leak existence. One visibility helper used by live SSE, replay, activity list, and presence focus stripping. |
| **SSE auth** | **Session cookie only** (M2 rule stands). Service tokens remain REST. |
| **Presence storage** | **In-memory**, single-node. Lost on process restart. Not durable, not in DuckDB. |
| **Presence model** | Per **tab/session** heartbeat with client `presenceId`; UI **collapses by user**. Optional **focus target** (`stepId` / `executionId`). |
| **Presence transport** | **REST** heartbeat + focus (`PUT`/`DELETE`); **SSE** delivers presence snapshots/diffs on the engagement topic. |
| **Presence × blind** | Members see who is online. **Focus targets are seat-aware:** blue must not receive unrevealed step/execution focus; server strips or generalizes before fan-out/snapshot. |
| **Comments unread** | **Client-only** “new since last view” (e.g. `localStorage` per execution/thread). No server read-receipts in M4. |
| **Activity UI** | **Engagement-wide live activity rail**, filterable by verb family; click focuses the object when visible under blind rules. |
| **Conflict UX** | Optimistic lock stays server-side (`execution.version`). UI: **toast + reload row** on 409; user re-applies. No auto-merge, no diff UI. |
| **Blind reveal policy** | Unchanged from **M3-EPIC**: lead/red manual reveal; optional `auto_reveal_on_start`. M4 owns live UX, SSE non-leakage, audit visibility of reveals. |
| **Reconnect ticket** | Own ticket after hub + fan-out; FE wires `Last-Event-ID` there. Visibility filter for live + replay lands in that ticket (`M4-004`). |
| **Exit gate** | **M4-010** SSE/presence war-room stress + slow-client eviction — required before M5/M6 lean on live updates. |

## Tickets

Build roughly in this order — the dependency chain is real.

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M4-001](M4-001-engagement-sse-topics.md) | Engagement SSE topics + per-topic authz | M | M3, M2-004 |
| [M4-002](M4-002-activity-event-fanout.md) | Activity → engagement event fan-out | M | M4-001, M1-015 |
| [M4-003](M4-003-frontend-event-consumption.md) | Frontend event consumption + precise cache invalidation | M | M4-001, M0B-009 |
| [M4-004](M4-004-reconnect-catchup.md) | Reconnection + `Last-Event-ID` + blind delivery filter | L | M4-002, M4-003 |
| [M4-005](M4-005-live-workbook.md) | Live workbook updates + 409 conflict toast | M | M4-003, M4-004, M3-014 |
| [M4-006](M4-006-presence.md) | Presence: heartbeat API, registry, SSE, UI | L | M4-001, M4-003 |
| [M4-007](M4-007-comment-threads-ui.md) | Live comment threads + lightweight unread | M | M4-005, M3-010 |
| [M4-008](M4-008-activity-rail-ui.md) | Engagement activity rail UI | M | M4-002, M4-003, M4-004 |
| [M4-009](M4-009-blind-mode-e2e.md) | Blind mode end-to-end (SSE + Playwright) | L | M4-005, M4-006, M4-008 |
| [M4-010](M4-010-sse-load-gate.md) | SSE war-room load test (**gate before M5–M6**) | M | M4-004, M4-006 |

## Risks

- **SSE through proxies.** Buffering breaks live UX. Deploy docs already note `proxy_buffering off` for `/api/v1/events` (`M2-004`); M4 must verify through the compose/deploy path, not only Vite dev.
- **Blind leakage via SSE/catch-up/presence.** Shared topic + object ids means **per-subscriber drop** is mandatory. Tests that subscribe as blue and assert **absence** of unrevealed object ids are mandatory.
- **Publish must never block on slow clients.** Hub eviction stays sacred; fan-out is post-commit and best-effort after durability.
- **Catch-up cost.** Replay is bounded (cursor → now for one engagement). Cap page size / max replay window in config; over-cap → client full refetch.
- **Presence memory.** Max presence entries and TTL eviction; multi-tab must not unbounded-grow.
- **M4-010** is the collaboration gate. If subscribers stall the process or catch-up blows memory, fix here — do not defer to M5.

## Out of milestone (do not pull in)

- Compact resource deltas on the wire (invalidate-only).
- Server-side read receipts / mentions / reactions.
- WebSockets or multi-node presence.
- Analytics rollups, Navigator, exports (M5).
- Report builder / share links (M6).
- Retest rounds (still deferred per M3-EPIC).
- Service-token SSE.
