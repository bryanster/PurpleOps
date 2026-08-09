package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/store"
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

	// Content version lifecycle (M2-007). Object is the version row; the
	// release label lives in the delta so a feed filtered by pin string does
	// not need a join.
	VerbContentVersionDeleted Verb = "content.version.deleted"

	// Custom content lifecycle (M2-011). Object is the custom row; object_type
	// in the entry distinguishes procedure_template / detection_rule_ref /
	// content_note, and the delta carries a short type label too.
	VerbContentCustomCreated Verb = "content.custom.created"
	VerbContentCustomUpdated Verb = "content.custom.updated"
	VerbContentCustomDeleted Verb = "content.custom.deleted"

	// Content v1/custom import lifecycle (M2-012). Object is the custom source
	// (sync path) or the job id (async path); counts live in the delta.
	VerbContentImportFinished Verb = "content.import.finished"
	VerbContentImportFailed   Verb = "content.import.failed"

	// Engagement lifecycle (M3-002). Object is the engagement itself.
	VerbEngagementCreated       Verb = "engagement.created"
	VerbEngagementUpdated       Verb = "engagement.updated"
	VerbEngagementStatusChanged Verb = "engagement.status_changed"
	VerbEngagementDeleted       Verb = "engagement.deleted"

	// Engagement membership lifecycle (M3-003). Object is the member row;
	// engagement_id names which engagement they were added to / removed from.
	VerbMemberAdded       Verb = "member.added"
	VerbMemberRoleChanged Verb = "member.role_changed"
	VerbMemberRemoved     Verb = "member.removed"

	// Workbook scenario lifecycle (M3-004). Object is the scenario row;
	// engagement_id names which engagement they belong to.
	VerbScenarioCreated   Verb = "scenario.created"
	VerbScenarioUpdated   Verb = "scenario.updated"
	VerbScenarioDeleted   Verb = "scenario.deleted"
	VerbScenarioReordered Verb = "scenario.reordered"
	VerbScenarioImported  Verb = "scenario.imported"

	// Workbook step lifecycle (M3-005). Object is the step row;
	// engagement_id names which engagement and scenario the step belongs to.
	VerbStepCreated   Verb = "step.created"
	VerbStepUpdated   Verb = "step.updated"
	VerbStepDeleted   Verb = "step.deleted"
	VerbStepReordered Verb = "step.reordered"
	VerbStepRevealed  Verb = "step.revealed"

	// Workbook execution lifecycle (M3-006). Object is the execution row;
	// engagement_id names which engagement the execution belongs to.
	VerbExecutionRedUpdated  Verb = "execution.red_updated"
	VerbExecutionBlueUpdated Verb = "execution.blue_updated"

	// Evidence blob lifecycle (M3-009). Object is the evidence row;
	// engagement_id names which engagement the evidence belongs to.
	VerbEvidenceUploaded Verb = "evidence.uploaded"
	VerbEvidenceDeleted  Verb = "evidence.deleted"

	// Comment lifecycle (M3-010). Object is the comment row;
	// engagement_id names which engagement the execution belongs to.
	VerbCommentCreated Verb = "comment.created"
	VerbCommentEdited  Verb = "comment.edited"

	// Finding lifecycle (M3-011). Object is the finding row;
	// engagement_id names which engagement the finding belongs to.
	VerbFindingCreated      Verb = "finding.created"
	VerbFindingUpdated      Verb = "finding.updated"
	VerbFindingDeleted      Verb = "finding.deleted"
	VerbFindingStepsChanged Verb = "finding.steps_changed"

	// Report lifecycle (M6-002). Object is the report row;
	// engagement_id names which engagement the report belongs to.
	VerbReportCreated Verb = "report.created"
	VerbReportUpdated Verb = "report.updated"
	VerbReportDeleted Verb = "report.deleted"

	// VerbReportPublished records that a report was published as an immutable
	// version (M6-011). Object is the report_version row; engagement_id naming
	VerbReportPublished Verb = "report.published"

	// Report template lifecycle (M6-003). Object is the template row;
	// engagement_id names which engagement the template belongs to.
	VerbReportTemplateCreated Verb = "report_template.created"
	VerbReportTemplateUpdated Verb = "report_template.updated"
	VerbReportTemplateDeleted Verb = "report_template.deleted"

	// VerbReportTemplateApplied records that a template was applied to a report draft.
	VerbReportTemplateApplied Verb = "report.template_applied"

	// Report share lifecycle (M6-012). Object is the report_share or
	// report_share_grant row.
	VerbReportShareCreated Verb = "report.share_created"
	VerbReportShareRevoked Verb = "report.share_revoked"
	VerbReportShareClaimed Verb = "report.share_claimed"
)


// Object types naming what a verb acted on. Kept as constants so a typo is a
// compile error rather than a row nobody can filter for.
const (
	ObjectUser                     = "user"
	ObjectSession                  = "session"
	ObjectToken                    = "service_token"
	ObjectTOTP                     = "totp"
	ObjectRecoveryCode             = "recovery_code"
	ObjectIdentity                 = "identity"
	ObjectLogin                    = "login"
	ObjectEngagement               = "engagement"
	ObjectContentSource            = "content_source"
	ObjectContentSyncJob           = "content_sync_job"
	ObjectContentSourceVersion     = "content_source_version"
	ObjectContentProcedureTemplate = "content_procedure_template"
	ObjectContentDetectionRuleRef  = "content_detection_rule_ref"
	ObjectContentNote              = "content_note"

	ObjectMember = "member"

	ObjectExecution = "execution"

	ObjectReport = "report"

	ObjectFinding  = "finding"
	ObjectEvidence = "evidence"

	ObjectReportVersion = "report_version"
	ObjectScenario = "scenario"

	ObjectStep = "step"

	ObjectComment = "comment"

	ObjectReportTemplate = "report_template"

	ObjectReportShare      = "report_share"
	ObjectReportShareGrant = "report_share_grant"
)


// Entry is one activity row as a caller wants it recorded. Secrets do not
// belong in Delta — see [Delta] and the redaction helpers below.
type Entry struct {
	EngagementID string
	ActorID      string
	Verb         Verb
	ObjectType   string
	ObjectID     string
	// ParentIDs carries optional parent object references for SSE fan-out
	// invalidation (M4-002). Keys are the parent field names sent on the wire
	// (e.g. "executionId", "scenarioId", "stepId"). Nil means no parents.
	ParentIDs map[string]string
	Delta     map[string]any
	At        time.Time
}

// Log is the append-only activity recorder (M1-015). Construct it with [New].
//
// Record writes inside the caller's transaction so the log and the change it
// describes commit or roll back together. That is the central design
// constraint of this package.
//
// When [SetHub] has been called, engagement-scoped activity entries are
// automatically fanned out to SSE subscribers after the transaction commits
// (M4-002). The returned context carries the post-commit hook; callers MUST
// capture and use it inside [store.DB.Write] callbacks for the hook to run.
type Log struct {
	entries      *activity.Entries
	hub          *Hub
	revealLookup RevealLookup
}

// New returns a Log over the activity store. Call [Log.SetHub] to enable
// SSE fan-out.
func New(entries *activity.Entries) *Log {
	if entries == nil {
		panic("events: New called with nil activity store")
	}
	return &Log{entries: entries}
}

// SetHub enables post-commit SSE fan-out for engagement-scoped activity
// entries (M4-002). Safe to call after construction; nil resets.
func (l *Log) SetHub(h *Hub) {
	l.hub = h
}

// SetRevealLookup enables the revealed field in engagement-scoped event
// payloads (M4-004). Safe to call after construction; nil resets.
func (l *Log) SetRevealLookup(r RevealLookup) {
	l.revealLookup = r
}

// Entries exposes the underlying repository for list endpoints. Handlers read;
// nothing outside this package should insert through it.
func (l *Log) Entries() *activity.Entries { return l.entries }

// Record appends entry on the caller's write transaction.
//
// tx must be the transaction the change is happening on. Passing nil is a
// programming error — use [Log.RecordAlone] for events that are not
// accompanying a mutation.
//
// When the hub is set and the entry has an EngagementID, a post-commit
// callback is queued via [store.PostCommitFanout] and executed after the
func (l *Log) Record(ctx context.Context, tx *sql.Tx, e Entry) error {
	storeEntry, err := toStore(e)
	if err != nil {
		return err
	}
	row, err := l.entries.Insert(ctx, tx, storeEntry)
	if err != nil {
		return err
	}
	// Queue post-commit SSE fan-out for engagement-scoped events (M4-002).
	// The revealed lookup happens inside the callback (post-commit) so it
	// sees the step committed in the same transaction.
	if e.EngagementID != "" && l.hub != nil {
		if q := store.PostCommitFanout.Load(); q != nil {
			q.Push(l.fanOut(e.EngagementID, row, e)) //nolint:contextcheck // post-commit: handler ctx is cancelled
		}
	}
	return nil
}

// RecordAlone opens its own write transaction and appends entry. It is for
// events whose whole substance is the log row — a failed login, a lockout —
// where there is no sibling mutation to share a commit with.
func (l *Log) RecordAlone(ctx context.Context, e Entry) error {
	storeEntry, err := toStore(e)
	if err != nil {
		return err
	}
	row, err := l.entries.Append(ctx, storeEntry)
	if err != nil {
		return err
	}
	if e.EngagementID != "" && l.hub != nil {
		revealed := l.lookupRevealed(ctx, e)
		l.hub.Publish(EngagementTopic(e.EngagementID), Event{
			ID:    row.ID,
			Type:  string(e.Verb),
			At:    row.At,
			Topic: EngagementTopic(e.EngagementID),
			Data:  buildEventData(row, e, revealed),
		})
	}
	return nil
}

// fanOut returns a post-commit callback that publishes one SSE event.
// The revealed lookup runs inside the callback so it sees data committed
// in the same write transaction (step.created, step.revealed).
func (l *Log) fanOut(engagementID string, row activity.Row, e Entry) store.PostCommitFunc { //nolint:contextcheck // post-commit callback: handler ctx is cancelled
	return func() { //nolint:contextcheck // post-commit: handler ctx is cancelled
		revealed := l.lookupRevealed(context.Background(), e)
		l.hub.Publish(EngagementTopic(engagementID), Event{
			ID:    row.ID,
			Type:  string(e.Verb),
			At:    row.At,
			Topic: EngagementTopic(engagementID),
			Data:  buildEventData(row, e, revealed),
		})
	}
}

// buildEventData constructs the SSE event payload from the stored activity
// row and the caller's entry. The payload is id-refs only — no full resource
// bodies, no secrets, no deltas.
//
// revealed is the step reveal status for step-scoped objects, or nil for
// non-step-scoped events. When nil, the "revealed" key is omitted from the
func buildEventData(row activity.Row, e Entry, revealed *bool) json.RawMessage {
	m := map[string]any{
		"engagementId": row.EngagementID,
		"actorId":      row.ActorID,
		"verb":         row.Verb,
		"objectType":   row.ObjectType,
		"objectId":     row.ObjectID,
	}
	for k, v := range e.ParentIDs {
		m[k] = v
	}
	if revealed != nil {
		m["revealed"] = *revealed
	}
	raw, err := json.Marshal(m)
	if err != nil {
		// Infallible for map[string]any, but handle defensively.
		raw = []byte(`{}`)
	}
	return raw
}

// lookupRevealed returns the revealed status for an entry's step, or nil
// when the entry is not step-scoped or no RevealLookup is configured.
func (l *Log) lookupRevealed(ctx context.Context, e Entry) *bool {
	if l.revealLookup == nil {
		return nil
	}
	stepID := stepIDForEntry(e)
	if stepID == "" {
		return nil
	}
	revealed, err := l.revealLookup.IsStepRevealed(ctx, stepID)
	if err != nil {
		return nil // conservative: don't drop events on lookup failure
	}
	return &revealed
}

// stepIDForEntry returns the step id an entry is about, or "" if not
// step-scoped.
func stepIDForEntry(e Entry) string {
	switch e.ObjectType {
	case ObjectStep:
		return e.ObjectID
	case ObjectExecution, ObjectEvidence, ObjectComment:
		if e.ParentIDs != nil {
			return e.ParentIDs["stepId"]
		}
		return ""
	default:
		return ""
	}
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
