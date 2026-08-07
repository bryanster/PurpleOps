# M4-002 — Activity → engagement event fan-out

**Milestone:** M4 · **Size:** M · **Depends on:** M4-001, M1-015

## Why

`PLAN.md` gives the activity table two jobs: SSE feed and report timeline. M3
already writes engagement verbs in the same transaction as the change. Live
collaboration must **derive** SSE events from those rows after commit — never a
second ad-hoc publish path that can drift from audit.

## Scope

**In**

- Post-commit fan-out hook from activity recording when `engagement_id` is
  non-null:
  - After the write transaction commits successfully, `Hub.Publish` on
    `engagement.{engagementId}`.
  - Event shape (invalidate-oriented):
    - `id` = activity row id (UUIDv7) — stable cursor for `M4-004`
    - `topic` = `engagement.{id}`
    - `type` = activity verb string (e.g. `execution.red_updated`,
      `step.revealed`, `comment.created`)
    - `at` = activity timestamp
    - `data` JSON: `{ "engagementId", "actorId", "verb", "objectType",
      "objectId", …optional parent refs (scenarioId, stepId, executionId) }`
      — **no** full resource bodies, **no** secrets, **no** large deltas
  - Platform activity (`engagement_id` null) does **not** publish to engagement
    topics.
- All engagement-scoped verbs already emitted by M1/M3 are in scope (member.*,
  engagement.*, scenario.*, step.*, execution.*, evidence.*, comment.*,
  finding.*). No need to re-emit historical rows until catch-up (`M4-004`).
- Failure policy: if Publish panics/fails, log and continue — **durability is
  the activity row**; SSE is best-effort after commit. Never roll back a user
  write because a subscriber is slow (Publish already non-blocking).
- **Shared topic:** Publish once to `engagement.{id}`. Do **not** fan out per
  seat at Publish time. Blind non-leakage is per-subscriber `Allow` + catch-up
  filtering in `M4-004` (hook from `M4-001`). Until `M4-004` lands, engagement
  SSE is membership-gated only — **do not demo blind live updates** before then.
- Payload: **no** withheld procedure/name/title fields in `data`. Id refs only
  (still an existence leak for blue until `M4-004` drops those events).
- Document the bridge in `internal/events` doc.go.

**Out**

- Catch-up / `Last-Event-ID` / blind `Allow` install (`M4-004`).
- Frontend invalidation map (`M4-003`).
- Presence events (not activity verbs) (`M4-006`).
- Changing when activity is recorded (already M3).

## Acceptance criteria

- [ ] Red PATCHes execution → activity row exists **and** a subscribed member
      receives one SSE event with `id` equal to that activity id and
      `type=execution.red_updated` (or the locked verb constant).
- [ ] Rolled-back writes produce **no** activity row and **no** SSE event
      (extend the M1-015 transactional test with a hub subscriber).
- [ ] Publish path never blocks on a stalled subscriber (reuse hub eviction).
- [ ] No full execution/step bodies in `data` — contract test on payload keys.
- [ ] Content job SSE path unaffected.
- [ ] Single publish site for activity→SSE (no scattered handler `Publish` calls).

## Tests

- Integration: start test server, subscribe as member, perform one mutation per
  major verb family (execution, comment, reveal, finding), assert event type +
  id correlation with activity list API.
- Transaction rollback: no publish.
- Race: `go test -race` on fan-out wiring.

## Notes for the implementer

- Hook shape is implementation choice: `events.Log` callback, store `After`
  commit hook, or explicit `Fanout.Activity(entry)` called only from Log after
  successful commit. **One** place — grep should not find random `hub.Publish`
  in handlers.
- Parent refs: include what invalidation needs (e.g. `executionId` on comment
  events) so `M4-003` does not have to GET activity details first.
- Keep `internal/events.Hub` free of DuckDB; the bridge may live in
  `internal/events` or `internal/httpapi` wiring, but the hub stays pure.
