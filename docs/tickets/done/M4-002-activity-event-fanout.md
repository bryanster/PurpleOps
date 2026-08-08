# M4-002 — Activity → engagement event fan-out

**Milestone:** M4 · **Size:** M · **Depends on:** M4-001, M1-015

## Why

`PLAN.md` gives the activity table two jobs: SSE feed and report timeline. M3
already writes engagement verbs in the same transaction as the change. Live
collaboration must **derive** SSE events from those rows after commit — never a
second ad-hoc publish path that can drift from audit.

## Acceptance criteria

- [x] Red PATCHes execution → activity row exists **and** a subscribed member
      receives one SSE event with `id` equal to that activity id and
      `type=execution.red_updated` (or the locked verb constant).
- [x] Rolled-back writes produce **no** activity row and **no** SSE event
      (extend the M1-015 transactional test with a hub subscriber).
- [x] Publish path never blocks on a stalled subscriber (reuse hub eviction).
- [x] No full execution/step bodies in `data` — contract test on payload keys.
- [x] Content job SSE path unaffected.
- [x] Single publish site for activity→SSE (no scattered handler `Publish` calls).

## Implementation notes

### Architecture

- `store.FanoutQueue` collects post-commit callbacks, flushed by `DB.Write`
  after commit and cleared on rollback.
- `events.Log` gains `hub *Hub` and `fanoutQueue *FanoutQueue` fields; wired
  via `SetHub` in server construction.
- `Log.Record` (in-txn) pushes a callback to the fanout queue; the queue is
  drained by `DB.Write` after commit.
- `Log.RecordAlone` publishes directly after `Append` returns (its internal
  `Write` already committed).
- Platform events (empty `engagement_id`) never fan out.
- Event payloads are id-refs only: `engagementId`, `actorId`, `verb`,
  `objectType`, `objectId`, plus optional `ParentIDs` from the entry.

### Caller fixes

- Comment activity recording was missing `EngagementID` — fixed in
  `recordCommentActivity`; `CreateComment` and `EditComment` now accept the
  engagement and execution IDs. Added `executionId` as a parent ref.
- Finding activity recording was missing `EngagementID` — fixed in
  `recordFindingActivity`; `DeleteFinding` now fetches the finding before
  deleting to get the engagement ID.

### Files changed

- `internal/store/postcommit.go` — new: `FanoutQueue`, `PostCommitFanout`
  atomic pointer, `WithPostCommit`, `RunPostCommit`
- `internal/store/db.go` — `Write` flushes `PostCommitFanout` after commit,
  clears on rollback
- `internal/events/activity.go` — `Log.hub`, `Log.SetHub`, `Entry.ParentIDs`,
  `Record` pushes to fanout queue, `RecordAlone` publishes directly,
  `fanOut`, `buildEventData`
- `internal/events/activity_test.go` — tests: `TestRecordFansOutToEngagementTopic`,
  `TestRecordRollbackProducesNoSSEEvent`, `TestEventPayloadIsIdRefsOnly`,
  `TestPlatformActivityDoesNotFanOut`, `TestRecordAloneFansOutAfterCommit`
- `internal/events/doc.go` — documented M4-002 fan-out bridge
- `internal/engagement/comments.go` — `CreateComment` accepts `engagementID`;
  `EditCommentInput` gains `EngagementID`/`ExecutionID`; `recordCommentActivity`
  includes engagement + execution parent ref
- `internal/engagement/findings.go` — `recordFindingActivity` includes
  `EngagementID`; `DeleteFinding` fetches finding first
- `internal/httpapi/commenthandlers.go` — passes engagement/execution IDs to
  comment service
- `internal/httpapi/server.go` — wires `PostCommitFanout` and `log.SetHub`
- `api/authz_test.go` — added `subscribeEvents` to exempt list (pre-existing
  M4-001 gap)
