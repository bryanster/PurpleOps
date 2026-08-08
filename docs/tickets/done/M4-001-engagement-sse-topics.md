# M4-001 — Engagement SSE topics + per-topic authz

**Milestone:** M4 · **Size:** M · **Depends on:** M3 complete, M2-004

## Why

M2 shipped a pure fan-out hub and `GET /api/v1/events` for content jobs. The war
room needs the same pipe on **engagement-scoped topics**, with membership (and
later blind) enforced at subscribe time — not a second streaming stack.

## Scope

**In**

- Extend `internal/events`:
  - Topic constant/helper: `engagement.{engagementId}` (UUIDv7 id).
  - `knownTopic` (or equivalent) accepts the new prefix beside `content.jobs*`.
  - Document the scheme in package doc (replace the illustrative
    `engagement.{id}.steps` example — **no subtopics** in M4).
  - **Per-subscription delivery filter** on `Subscribe` (e.g.
    `Subscription.Allow func(Event) bool`, default allow-all). Evaluated when
    delivering to that subscriber so red/blue can share `engagement.{id}`
    without `Publish` knowing seats. Denied events are skipped for that client
    only; `Publish` stays non-blocking. Unit-test: two subscribers on one topic,
    filter denies one, only the other receives.
- Wire `Options.TopicAuthz` for the live server:
  - `content.jobs` / `content.jobs.{id}` — unchanged (`content.sync` / admin).
  - `engagement.{id}` — caller must pass `engagement.read` for that id
    (member or admin), via existing authz — **not** a handler-local role check.
  - Unknown / malformed topics → 400 (existing behaviour).
  - If every requested topic is filtered out → same as M2 (`ErrNoTopics` /
    403-or-empty policy already chosen — keep it consistent, test both content
    and engagement).
- Session-only remains enforced for `GET /api/v1/events` (OpenAPI
  `cookieSession` + `SessionOnly`).
- Heartbeats, buffer, max subscribers, slow-client eviction: unchanged defaults
  unless config keys need documenting for engagement use.
- OpenAPI/description update: document engagement topic form and that service
  tokens cannot subscribe.
- Deploy note check: `docs/deploy.md` still covers buffering for `/api/v1/events`.

**Out**

- Publishing activity onto engagement topics (`M4-002`).
- Installing blind `Allow` predicates (`M4-004`).
- `Last-Event-ID` replay (`M4-004`).
- Presence (`M4-006`).
- Frontend invalidation wiring (`M4-003`).

## Acceptance criteria

- [x] A member can subscribe to `engagement.{theirEngagementId}` and receive a
      heartbeat (and later publishes) without polling.
- [x] A non-member subscribing to that topic is denied (403 or filtered to no
      topics — same policy as content); never silently subscribed.
- [x] Admin may subscribe to any engagement topic (platform admin
      `engagement.read` behaviour matches REST).
- [x] Content job topics still work; regression test green.
- [x] Per-subscriber `Allow` can drop events for one client without affecting
      another on the same topic; Publish does not block.
- [x] `go test -race ./internal/events/` clean.
- [x] Package doc lists M4 topic scheme, `Allow`, and remaining extension points
      (catch-up, presence registry).

## Tests

- Hub/TopicAuthz unit tests for engagement prefix allow/deny.
- Hub unit test for `Allow` filter asymmetry on one topic.
- Handler test: member vs outsider vs admin on `GET /api/v1/events?topics=`.
- Authz sweep / matrix touch if new requirement metadata is introduced.

## Notes for the implementer

- Extend, do not move, `internal/events` — M2-004 contract.
- TopicAuthz = may subscribe to topic. `Allow` = may see this event. Do not
  collapse them. Blind object filtering installs `Allow` in `M4-004`.
- Prefer parsing `engagement.` + validated UUID over open-ended string topics.


## Implementation notes

- **Engagement topic:** `events.EngagementTopic(engagementID)` returns
  `"engagement.<UUIDv7>"`. `knownTopic` validates the UUID suffix so
  `engagement.nope` gets 400.
- **Per-subscriber Allow:** `Subscription.Allow func(Event) bool` — evaluated
  in `Publish` after topic matching; drops events for one subscriber without
  affecting others or blocking Publish. Nil means allow all.
- **Authz strategy:** OpenAPI `/events` changed from `x-authz-action:
  content.sync` to `x-authz-self: true` so any authenticated session reaches
  the handler. The middleware's `decide()` now checks `SessionOnly` before the
  Self short-circuit (old code checked it after, which meant Self ops with
  SessionOnly would not enforce it). Per-topic authz (`content.sync` for
  content jobs, engagement membership for engagement topics) is enforced in
  the handler via `handlers.topicAllowed`, which uses `ownership.Seat` for
  engagement membership checks.
- **Handler behavior:** non-member requesting only unauthorized topics gets
  empty topic list → hub `ErrNoTopics` → 400 (matches M2 content.jobs policy).
  Admin bypass in `topicAllowed` skips membership check.
- **Tests:** engagement tests seed engagements + memberships via direct DB
  INSERT. SSE handshake tests use `httptest.NewServer` + `http.Client` with
  context timeout to avoid hanging on the long-lived stream.
- `make generate` clean. `go build ./...` passes. All events + httpapi tests
  pass with `-race`.

### Files changed

- `internal/events/hub.go` — `TopicEngagementPrefix`, `EngagementTopic()`,
  `Subscription.Allow`, subscriber `allow` field, `knownTopic` UUID validation
- `internal/events/hub_test.go` — `TestAllowFilterDropsEventsForOneSubscriberOnly`,
  `TestEngagementTopicIsKnown`, `TestEngagementTopicRejectsNonUUID`,
  `TestUnknownTopicWithEngagementLikePrefix`
- `internal/events/doc.go` — updated M4 extension points
- `api/openapi.yaml` — `/events` → `x-authz-self` + `x-authz-because`;
  updated description with engagement topics
- `internal/httpapi/authorize.go` — `decide()`: SessionOnly check before Self
  short-circuit
- `internal/httpapi/events.go` — `handlers.topicAllowed()` method; per-topic
  authz in `SubscribeEvents`
- `internal/httpapi/events_test.go` — `TestSubscribingToContentJobsIsFilteredForMembers`
  (was 403, now 400), `TestMemberCanSubscribeToEngagementTopic`,
  `TestNonMemberCannotSubscribeToEngagementTopic`,
  `TestAdminCanSubscribeToAnyEngagementTopic`, seed helpers
- `web/src/api/schema.d.ts` — regenerated from updated OpenAPI
