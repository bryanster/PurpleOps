package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/store/activity"
)

// Verb is one kind of thing that happened. The naming pattern is fixed:
// `object.past_tense_verb`. M3–M6 extend the vocabulary; they do not rename
// what is here.
type Verb string

// M1 security-relevant verbs. Each is what the activity row carries, and what
// the platform feed is filterable by.
const (
	VerbUserCreated         Verb = "user.created"
	VerbUserUpdated         Verb = "user.updated"
	VerbUserRoleChanged     Verb = "user.role_changed"
	VerbUserDisabled        Verb = "user.disabled"
	VerbUserEnabled         Verb = "user.enabled"
	VerbUserPasswordChanged Verb = "user.password_changed"

	// VerbUserSessionsRevoked is an administrator signing somebody out
	// everywhere (M1-016). It is deliberately not session.logout: that verb is
	// somebody ending their own session, and an incident review wants to be
	// able to tell the two apart by filtering rather than by reading deltas.
	VerbUserSessionsRevoked Verb = "user.sessions_revoked"

	VerbSessionLogin       Verb = "session.login"
	VerbSessionLoginFailed Verb = "session.login_failed"
	VerbSessionLogout      Verb = "session.logout"

	// VerbSessionOthersRevoked is somebody signing *themselves* out of every
	// browser but the one they are using (M1-017). It is a third thing again
	// from the two above it: not an administrator acting on an account
	// (VerbUserSessionsRevoked) and not one session ending
	// (VerbSessionLogout), but the account holder deciding that everywhere
	// else should stop. An incident review wants to tell those apart by
	// filtering, so they are three verbs rather than one with a delta.
	VerbSessionOthersRevoked Verb = "session.others_revoked"

	VerbLoginThrottled Verb = "login.throttled"

	VerbMFAEnrolled       Verb = "mfa.enrolled"
	VerbMFADisabled       Verb = "mfa.disabled"
	VerbMFARecoveryUsed   Verb = "mfa.recovery_used"
	VerbMFARecoveryIssued Verb = "mfa.recovery_issued"
	VerbMFARecoveryReset  Verb = "mfa.reset"

	VerbTokenCreated   Verb = "token.created"
	VerbTokenRevoked   Verb = "token.revoked"
	VerbTokenFirstUsed Verb = "token.first_used"

	// VerbTokenAdminRevoked is an administrator ending somebody *else's*
	// service token (M1-018). It is deliberately not token.revoked: that verb
	// is somebody rotating their own credential, which happens routinely, and
	// this one happens because there is an incident. An incident review wants
	// to tell the two apart by filtering rather than by comparing the actor
	// against a delta field, the same way VerbUserSessionsRevoked is kept apart
	// from VerbSessionLogout.
	//
	//nolint:gosec // G101: a verb in the activity vocabulary, not a credential.
	// The identifier holds the words "token" and "admin", which is what the
	// heuristic matches on; there is no spelling of "an administrator revoked a
	// token" that avoids them, and the name is right.
	VerbTokenAdminRevoked Verb = "token.admin_revoked"

	VerbSSOProvisioned Verb = "sso.provisioned"
	VerbSSOLinked      Verb = "sso.linked"

	// Content source lifecycle (M2-002). Platform-scoped: engagement_id is null.
	VerbContentSourceEnabled  Verb = "content.source.enabled"
	VerbContentSourceDisabled Verb = "content.source.disabled"
	VerbContentSourceUpdated  Verb = "content.source.updated"
	VerbContentSourceDeleted  Verb = "content.source.deleted"

	// Content sync job lifecycle (M2-003). Object is the job; source_id lives in
	// the delta so a feed filtered by source does not need a join.
	VerbContentSyncStarted   Verb = "content.sync.started"
	VerbContentSyncFinished  Verb = "content.sync.finished"
	VerbContentSyncFailed    Verb = "content.sync.failed"
	VerbContentSyncCancelled Verb = "content.sync.cancelled"
)

// Object types naming what a verb acted on. Kept as constants so a typo is a
// compile error rather than a row nobody can filter for.
const (
	ObjectUser           = "user"
	ObjectSession        = "session"
	ObjectToken          = "service_token"
	ObjectTOTP           = "totp"
	ObjectRecoveryCode   = "recovery_code"
	ObjectIdentity       = "identity"
	ObjectLogin          = "login"
	ObjectContentSource  = "content_source"
	ObjectContentSyncJob = "content_sync_job"
)

// Entry is one activity row as a caller wants it recorded. Secrets do not
// belong in Delta — see [Delta] and the redaction helpers below.
type Entry struct {
	EngagementID string
	ActorID      string
	Verb         Verb
	ObjectType   string
	ObjectID     string
	Delta        map[string]any
	At           time.Time
}

// Log is the append-only activity recorder (M1-015). Construct it with [New].
//
// Record writes inside the caller's transaction so the log and the change it
// describes commit or roll back together. That is the central design
// constraint of this package.
type Log struct {
	entries *activity.Entries
}

// New returns a Log over the activity store.
func New(entries *activity.Entries) *Log {
	if entries == nil {
		panic("events: New called with nil activity store")
	}
	return &Log{entries: entries}
}

// Entries exposes the underlying repository for list endpoints. Handlers read;
// nothing outside this package should insert through it.
func (l *Log) Entries() *activity.Entries { return l.entries }

// Record appends entry on the caller's write transaction.
//
// tx must be the transaction the change is happening on. Passing nil is a
// programming error — use [Log.RecordAlone] for events that are not
// accompanying a mutation.
func (l *Log) Record(ctx context.Context, tx *sql.Tx, e Entry) error {
	row, err := toStore(e)
	if err != nil {
		return err
	}
	_, err = l.entries.Insert(ctx, tx, row)
	return err
}

// RecordAlone opens its own write transaction and appends entry. It is for
// events whose whole substance is the log row — a failed login, a lockout —
// where there is no sibling mutation to share a commit with.
func (l *Log) RecordAlone(ctx context.Context, e Entry) error {
	row, err := toStore(e)
	if err != nil {
		return err
	}
	_, err = l.entries.Append(ctx, row)
	return err
}

func toStore(e Entry) (activity.Entry, error) {
	out := activity.Entry{
		EngagementID: e.EngagementID,
		ActorID:      e.ActorID,
		Verb:         string(e.Verb),
		ObjectType:   e.ObjectType,
		ObjectID:     e.ObjectID,
		At:           e.At,
	}
	if len(e.Delta) > 0 {
		raw, err := json.Marshal(e.Delta)
		if err != nil {
			return activity.Entry{}, fmt.Errorf("events: marshal delta for %s: %w", e.Verb, err)
		}
		out.Delta = raw
	}
	return out, nil
}

// Delta builds a redacted before/after map. Keys whose values look like
// secrets are dropped rather than stored — defence in depth on top of callers
// not putting them there in the first place.
func Delta(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		if secretKey(k) {
			continue
		}
		if s, ok := v.(string); ok && secretValue(s) {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// secretKey names fields that must never appear in a delta, regardless of
// value. Matching is exact on the wire name callers use.
func secretKey(k string) bool {
	switch k {
	case "password", "password_hash", "current_password", "new_password",
		"token", "token_hash", "secret", "totp_secret", "shared_secret",
		"recovery_code", "recovery_codes", "session_token", "code":
		return true
	}
	return false
}

// secretValue is a last-ditch filter for values that look like credentials
// even under an innocuous key. It is deliberately narrow: a high false-positive
// rate would strip useful audit data.
func secretValue(s string) bool {
	// Argon2id encoded hashes and raw TOTP base32 secrets are the two shapes
	// most likely to leak through a mis-keyed delta.
	if len(s) >= 20 && (hasPrefix(s, "$argon2") || hasPrefix(s, "bl_")) {
		return true
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
