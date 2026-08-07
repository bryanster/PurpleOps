# M3-016 — War-room concurrency load test (gate)

**Milestone:** M3 · **Size:** M · **Depends on:** M3-007, M3-009

## Why

DuckDB write contention under concurrent editing is a named risk (`PLAN.md` §8). The serialized
writer (`M0B-003`) plus optimistic locking must keep a simulated war room correct and responsive
**before** M4–M6 build on top. **Do not defer this to the end of later milestones.**

## Scope

**In**

- Repeatable load test (`internal/engagement/loadtest` or `hack/` + Go test) that:
  1. Seeds one engagement with ≥50 steps (each with execution), several members.
  2. Simulates **20 concurrent users** (goroutines with distinct sessions or direct service/store
     calls — prefer HTTP against a test server if feasible, else domain+store with real `store.Write`).
  3. Mix: red status/notes patches, blue detection patches, evidence uploads (small bytes),
     comment creates, reads interleaved.
  4. Asserts:
     - **Zero** lost updates that bypass optimistic locking (no silent overwrite — every conflicting
       write surfaces 409 or retries successfully with monotonic versions).
     - Interactive p95 write latency under a documented budget on developer SSD (start from M2-016
       ballpark: p95 < 200ms for small patches; tune with evidence, don't hollow out the test).
     - No deadlocks; all workers finish; DB consistent (execution versions = number of successful
       writes per row, spot-check).
  5. Optional retry helper demonstrating client 409 → re-GET → re-PATCH success under contention.
- CI: **scaled-down** variant (fewer users/steps) that still fails if the version `WHERE` clause is
  removed (mutation test once when writing).
- Document run command, budgets, and interpretation in `docs/testing.md`.
- If defaults fail: fix handler/store batching or lock scope — **not** "raise budget to 30s".

**Out**

- Multi-node / multi-process.
- Full browser Playwright load (API/store level is the gate).
- SSE fan-out stress (M4).

## Acceptance criteria

- [x] Documented command pass/fail on a developer machine (`BLACKLIGHT_LOADTEST=1` for full 20-user).
- [x] CI scaled-down test fails if optimistic version predicate is dropped (verified by mutation).
- [x] Completion notes record measured p95/pmax and final budgets.
- [x] README / epic marks this as **gate before M4–M6**.

## Tests

- The load test + CI mutation gate are the deliverable.

## Notes for the implementer

- Do not mock `store.Write`.
- Evidence uploads should use tiny payloads so the test measures writer fairness, not disk bandwidth.
- Reuse patterns from `internal/content/loadtest` (M2-016) where they fit.

## Implementation notes

**Measured metrics** (arm64 devcontainer, local SSD):

| Metric | CI gate (5 users × 20 steps) | Full load (20 users × 50 steps) |
|---|---|---|
| Interactive p95 | 16.7ms | 46.1ms |
| Interactive max | 21ms | 121ms |
| Successful writes | 1672 | 5900 |
| Version conflicts (retried) | 117 | 951 |
| Lost updates detected | 0 | 0 |
| Budget | p95 ≤ 200ms, max ≤ 2s | p95 ≤ 200ms, max ≤ 2s |

**Mutation test**: `TestWarRoomConcurrencyDetectsLostUpdates` removes the version WHERE
clause and uses literal version values (read-then-write pattern). The consistency check
detects -144 lost updates, proving the gate would catch a regression in the optimistic
lock path.

**Deviations from ticket**:

1. Direct store calls rather than HTTP — the ticket allowed this as a fallback. The
   serialized writer and optimistic lock are the same regardless of transport.
2. Evidence store uses `os.MkdirTemp` + manual cleanup rather than `t.TempDir()` — the
   latter's cleanup ordering races with `storetest`'s DB close on this DuckDB/driver version.
3. Engagement created with direct INSERT (active status) rather than Create+UPDATE —
   DuckDB has a bug where UPDATE on the `app.engagement.status` column fails with a
   catalog error referencing the intermediate `engagement_member_next` table from the
   migration rebuild.
4. `listExecutionIDs` uses a direct query rather than `ListByEngagement` — the latter
   has a pre-existing ambiguous `id` column bug in its JOIN.
5. Added `config.LoadTestEnabled()` helper — the `BLACKLIGHT_LOADTEST` env var was
   previously read via `os.Getenv` in both load test packages, which violated the
   `TestOnlyConfigReadsTheEnvironment` gate. Both load tests now use the config helper.

**Files changed**:

- `internal/engagement/loadtest/warroom_test.go` — new: three tests (CI gate, mutation
  detector, full dev load), retry helpers, no-version patch helpers for mutation test
- `internal/config/config.go` — new: `LoadTestEnabled()` helper
- `internal/content/loadtest/fairness_test.go` — updated: uses `config.LoadTestEnabled()`
  instead of direct `os.Getenv`
- `docs/testing.md` — new section: War-room concurrency (M3-016)
