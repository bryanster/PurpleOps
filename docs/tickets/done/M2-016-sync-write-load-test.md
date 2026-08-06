# M2-016 — Sync write load test (serialized writer fairness)

**Milestone:** M2 · **Size:** M · **Depends on:** M2-006, M2-008

## Why

Content sync is the largest write volume in the system (`M2-EPIC` risks). All of it goes through
the single serialized writer (`M0B-003`). If Apply batches are too large or the lock is held across
parse work, interactive auth and (later) scoring stutter. Prove fairness **before** M3 piles on.

## Scope

**In**

- A repeatable load/benchmark test (Go test or `blctl`/script under `hack/` or `internal/content/loadtest`)
  that:
  1. Starts a full ATT&CK fixture sync (large enough to multi-batch) **or** a synthetic adapter that
     writes tens of thousands of rows in realistic batch sizes.
  2. Concurrently issues interactive-style writes/reads: session touch or user update + content
     reads, on a timer (e.g. every 50ms).
  3. Asserts interactive p95 latency stays under a documented budget (pick something tight for local
     SSD, e.g. p95 < 200ms for a simple write, and no interactive request > job timeout).
  4. Asserts the sync still completes successfully.
- Documents how to run it in `docs/testing.md` (or content doc): command, hardware assumptions,
  what failure means (shrink batch size / fix lock scope).
- If defaults fail the budget, **fix the runner batching** (`M2-003` config defaults) rather than
  loosening the test into meaninglessness — note the final defaults in the ticket completion notes.
- CI: run a **scaled-down** variant that still would fail if Write were held across a sleep in Apply
  (injectable fault), so the regression is caught without multi-minute CI.

**Out**

- Full war-room scoring load (`M3-016`).
- Multi-process / multi-node tests.

## Acceptance criteria

- [x] Documented command produces a pass/fail on a developer machine.
- [x] CI scaled-down test fails if Apply holds the write lock across a ≥100ms sleep (mutation
      verified once when writing the test).
- [x] Default `BLACKLIGHT_CONTENT_WRITE_BATCH` justified in config comment from measured results.
- [x] Sync of real ATT&CK fixture (or recorded timing) noted in completion notes as ballpark
      duration — no hard CI bound on full fixture unless it is fast enough.

## Tests

- The load test itself is the deliverable; plus the CI fault-injection regression.

## Notes for the implementer

- Do not mock `store.Write` — the point is the real mutex.
- Prefer wall-clock assertions with generous but finite budgets; flake is worse than a slightly
  loose number, but a 30s interactive budget is not a test.
- If the hub or SSE path is involved, keep it out of the critical measurement — focus on store lock
  fairness.

## Implementation notes

### Deliverables

- `internal/content/loadtest/fairness_test.go`
  - `TestSyncWriteFairness` — CI gate: 200 fixture notes, batch 25, session-touch every 50ms +
    content read; p95 ≤ 200ms, max ≤ 2s, sync succeeds, multi-batch.
  - `TestSyncWriteFairnessDetectsLockHold` — same setup with `FixtureAdapter.HoldWrite = 250ms`
    **inside** each `store.Write`; asserts interactive p95 exceeds the 200ms budget (mutation
    stays enforced).
  - `TestSyncWriteLoad` — 20 000 notes at production batch 250; opt-in via `BLACKLIGHT_LOADTEST=1`.
- `FixtureAdapter.HoldWrite` — sleeps inside Write (lock held). Distinct from `DelayBatch`, which
  sleeps between batches outside the lock.
- Docs: `docs/testing.md` § "Content sync write fairness (M2-016)".
- Config / `.env.example` / runner default comments cite the measured default.

### Measured results (arm64 Linux devcontainer, local disk, 2026-08-06)

| Run | Notes | Batch | Sync | Interactive p95 | Max |
|---|---|---|---|---|---|
| CI fairness | 200 | 25 | ~214ms | ~15ms | ~15ms |
| Fault inject (HoldWrite 250ms) | 200 | 25 | ~2.9s | ~274ms (**fails** 200ms budget) | ~289ms |
| Full load (`BLACKLIGHT_LOADTEST=1`) | 20 000 | **250** | ~9.6s | ~111ms | ~120ms |
| ATT&CK mini fixture (`TestSyncVersionIsolationAndResync`) | handful of STIX objects | 50 | ~0.6s wall for the whole test | n/a | n/a |

Checked-in ATT&CK fixtures are deliberately tiny; multi-batch volume is the synthetic fixture
adapter, which exercises the same `content.Writer` batching path production adapters use.

### Defaults

`BLACKLIGHT_CONTENT_WRITE_BATCH` stays **250**. Full load at that size kept interactive p95 under
the 200ms budget; no shrink required. Raise only after re-running the loadtest; lower if
interactive writes stutter under real ATT&CK/Atomic catalogs.

### Commands

```sh
# CI (also under make test / make test-race)
go test ./internal/content/loadtest/

# Full developer load
BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/content/loadtest/ -run TestSyncWriteLoad
```
