# M2-004 — Minimal shared SSE hub + sync progress

**Milestone:** M2 · **Size:** L · **Depends on:** M2-003, M1-013, M0B-006

## Why

`PLAN.md` streams content sync progress over SSE and later puts the whole war room on the same
mechanism (`M4`). Building a throwaway `/content/.../events` pipe guarantees a rewrite. Build the
**smallest hub M4 can extend**: topics, authz per subscribe, backpressure, slow-client eviction.

## Scope

**In**

- `internal/events` hub (alongside the existing activity recorder from `M1-015`):
  - `Subscribe(ctx, sub Subscription) (<-chan Event, func(), error)`
  - `Publish(topic string, evt Event)`
  - Topics as string constants; M2 defines at least:
    - `content.jobs` — all job progress (admin)
    - `content.jobs.{jobId}` — single job (admin)
  - Event payload: `id` (UUIDv7), `topic`, `type`, `at`, `data` JSON. Stable `type` values:
    `content.job.progress`, `content.job.terminal`.
- HTTP: `GET /api/v1/events` (or `/events` under the API prefix already used) with:
  - Session cookie auth (service tokens: decide allow/deny — default **session only** for SSE in
    M2; document).
  - Query `topics` (repeatable) — server intersects with what the subject may see.
  - `text/event-stream`, `Cache-Control: no-cache`, explicit flush, heartbeat comments every N
    seconds.
  - Optional `Last-Event-ID` accepted but **best-effort only in M2** (no activity-log catch-up yet;
    M4 owns guaranteed catch-up). Document the gap.
- Authz: subscribe to content job topics requires `content.sync` (admin). Unknown topics → ignore
  or 400; never silently widen.
- Backpressure:
  - Per-subscriber buffer (small, fixed).
  - On overflow: drop subscriber and close the stream (never block `Publish` on a slow client).
  - Test proves a stalled consumer is evicted and publishers continue.
- Wire `M2-003` progress callbacks → `Publish`.
- Frontend: a tiny `useEventSource` hook or module used by sources admin UI (`M2-014`) — can land
  the hook here with a storybook/component test, or in M2-014; prefer here so the contract is real.
- Deploy note in `docs/deploy.md`: reverse proxies must disable response buffering for the events
  path (nginx `proxy_buffering off` example).

**Out**

- Engagement topics, presence, activity fan-out (`M4-001`…`M4-008`).
- Guaranteed replay / `Last-Event-ID` against the activity log.
- Multiplexed binary protocols.

## Acceptance criteria

- [x] An admin starting a sync receives progress events without polling; a member subscribing to
      `content.jobs` is 403 at subscribe or filtered to zero topics (pick one, test it).
- [x] Heartbeats keep the connection alive through an idle proxy timeout of ≥60s in local compose.
- [x] A subscriber that never reads is dropped; a subsequent Publish still returns promptly.
- [x] Job terminal event fires once per job with final status matching `GET /content/jobs/{id}`.
- [x] Hub has no dependency on DuckDB or adapters — pure fan-out. Activity recorder stays separate.
- [x] Package doc lists extension points M4 will use (engagement topic prefix, authz callback).

## Tests

- Hub unit tests: multi-subscriber, eviction, cancel ctx unsubscribes, topic filter.
- Handler test: headers, authz, at least one progress event end-to-end with the fake adapter from
  `M2-003`.
- Race test: `go test -race` on the hub package.

## Notes for the implementer

- Prefer standard library `net/http` streaming + chi route over a framework.
- Do not invent a second activity stream. Job progress is ephemeral UI; durable audit remains
  `activity` rows from `M2-003`.
- Keep memory bounded: max subscribers config (sane default), max buffer length.
- Naming: put hub code where M4 expects it (`internal/events`) so M4-001 is "extend", not "move".

## Implementation notes

- **Hub:** `internal/events/hub.go` — pure fan-out beside the activity `Log`. Topics
  `content.jobs` / `content.jobs.{id}`; types `content.job.progress` |
  `content.job.terminal`. `Options.TopicAuthz` is the M4 per-topic filter hook;
  package doc lists engagement prefix + catch-up extension points.
- **Backpressure:** per-subscriber buffer (default 16); overflow closes the
  channel and detaches under a send mutex so Publish never blocks and never
  races close. `BLACKLIGHT_EVENTS_{MAX_SUBSCRIBERS,BUFFER,HEARTBEAT}` (256 / 16
  / 15s).
- **HTTP:** `GET /api/v1/events` in OpenAPI → strict handler returns
  `text/event-stream` via `io.Pipe` (generated Visit already flushes). Timeout
  middleware opts out by path. Heartbeat comment frames (`: ping`).
- **Authz:** operation uses `content.sync` (admin → member 403 at subscribe).
  Session-only via OpenAPI `security: [cookieSession]` +
  `api.Requirement.SessionOnly` enforced in authorize middleware *before*
  `authz.Can` — so `content.sync` stays token-reachable for REST start/cancel.
  Unknown topics → 400 validation. `topics` is optional in the schema so the
  request validator does not 400 before auth on the M1-011/M1-008 route sweeps;
  empty list is still 400 from the handler after authz.
- **Bridge:** `runner.Subscribe` → `bridgeContentProgress` publishes both
  broadcast and per-job topics. Queued-job cancel now also publishes a terminal
  tick (gap closed from M2-003).
- **Frontend:** `web/src/lib/use-event-source.ts` + URL unit test. Deploy note
  under reverse proxy in `docs/deploy.md`; timeout table updated in
  `docs/http.md`.
- **Verified:** `make lint test build`; `go test -race ./internal/events/`.
