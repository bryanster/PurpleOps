// Package events carries things that happened: the append-only activity log
// (M1-015) and the server-sent-events hub that fans ephemeral UI progress out
// to live subscribers (M2-004, extended by M4).
//
// # Activity log
//
// [Log.Record] writes one row inside the caller's database transaction, so the
// log can never disagree with the data — if the change rolls back, the row is
// gone too. That is the central design constraint of M1-015. Events with no
// sibling mutation (a failed login, a lockout) use [Log.RecordAlone], which
// opens its own write.
//
// Verbs follow `object.past_tense_verb`. The M1 set lives as constants here;
// M3–M6 extend it. Deltas hold before/after for changed fields and never
// secrets: password hashes, token secrets, TOTP shared secrets, session
// tokens, recovery-code plaintext.
//
// # SSE hub
//
// [Hub] is pure in-process fan-out. It does not touch DuckDB, adapters, or the
// activity log. Job progress is ephemeral UI; durable audit remains activity
// rows written by the content runner (M2-003). Do not invent a second durable
// stream.
//
//	ch, unsub, err := hub.Subscribe(ctx, events.Subscription{Topics: []string{events.TopicContentJobs}})
//	hub.Publish(events.TopicContentJobs, events.Event{Type: events.TypeContentJobProgress, Data: payload})
//
// Backpressure: each subscriber has a fixed buffer. On overflow the subscriber
// is dropped and its channel closed — Publish never blocks on a slow client.
//
// # M4 extension points
//
// Keep changes additive; M4 should extend, not move, this package:
//
//   - Topic prefix: add engagement-scoped names (for example
//     `engagement.{id}.steps`) beside the M2 content.jobs* constants. Teach
//     [knownTopic] the new prefix.
//   - Authz callback: set [Options.TopicAuthz] so Subscribe intersects the
//     requested topics with what the subject may see (membership, blind mode).
//     The HTTP layer already refuses the connection when the caller cannot
//     hold any requested topic; TopicAuthz is the per-topic half.
//   - Catch-up: accept Last-Event-ID and replay from the activity log. M2
//     accepts the header and ignores it (best-effort live tail only).
//   - Presence: a separate in-memory registry can live alongside [Hub]; do not
//     overload Event.Type for "who is here".
package events
