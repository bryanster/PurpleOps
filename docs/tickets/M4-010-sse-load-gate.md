# M4-010 — SSE war-room load test (gate)

**Milestone:** M4 · **Size:** M · **Depends on:** M4-004, M4-006

## Why

M3-016 proved the serialized writer under concurrent PATCHes. Collaboration adds
**many long-lived subscribers**, catch-up bursts, and presence heartbeats. A
slow client or unbounded replay must not stall writes or exhaust memory before
M5/M6 pile on. **Gate before M5–M6.**

## Scope

**In**

- Repeatable test (`internal/events/loadtest` or `internal/engagement/loadtest`
  SSE companion) that:
  1. Seeds one engagement with enough activity history for catch-up (e.g. ≥200
     rows) and ≥20 steps.
  2. Opens **N concurrent SSE subscribers** (target full run: 20 users × 2 tabs
     ≈ 40; CI scaled-down: fewer).
  3. Mix: presence heartbeats, activity-producing PATCHes (reuse war-room write
     mix at reduced rate), occasional reconnect with `Last-Event-ID`.
  4. Asserts:
     - `Hub.Publish` / write path p95 stays within documented budget (do not
       regress M3-016 interactive write budget by more than a small stated
       factor under SSE load).
     - Stalled subscriber (never reads) is **evicted**; publishers continue.
     - Subscriber count returns to baseline after cancel.
     - Catch-up respects max-replay cap (no multi-100MB buffers).
     - No goroutine explosion / obvious leak in a short run (optional
       `runtime.NumGoroutine` bound with slack).
  5. `BLACKLIGHT_LOADTEST=1` for full; CI runs scaled-down always.
- Document command, budgets, and proxy note cross-link in `docs/testing.md`.
- If fail: fix hub/catch-up/presence — **not** “disable SSE in prod”.

**Out**

- Multi-node, real browser fan-out at scale.
- Replacing M3-016 (keep both gates).
- Tuning production reverse proxies beyond docs.

## Acceptance criteria

- [ ] Documented pass/fail command on developer machine.
- [ ] CI scaled-down fails if slow-client eviction is broken (mutation or
      deliberate full-buffer test already in hub — wire into loadtest story).
- [ ] Completion notes record subscriber counts, publish lag, write p95.
- [ ] README / M4-EPIC mark this **gate before M5–M6**.

## Tests

- The load test + CI variant are the deliverable.
- Keep `go test -race` on events packages green.

## Notes for the implementer

- Prefer HTTP SSE clients against a test server so authz + catch-up are real.
- Tiny write payloads; this is not an evidence bandwidth test.
- Reuse `config.LoadTestEnabled()` from M3-016.
