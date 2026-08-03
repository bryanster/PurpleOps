// Package events carries things that happened: the append-only activity log
// (M1-015) and, later, the server-sent-events hub and presence tracking (M4).
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
package events
