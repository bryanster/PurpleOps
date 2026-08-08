package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// DB is the part of the store these repositories need: pooled reads, and writes
// serialized into one transaction at a time. [store.Store] satisfies it.
//
// It is declared here, in the package that consumes it, rather than exported by
// the store — so a test can substitute one, and so this package's dependency is
// the two methods it calls rather than everything a database happens to offer.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// After runs inside a write transaction after the primary mutation succeeds.
// Activity recording (M1-015) is the first consumer: the log row and the change
// share one commit, so a failure here rolls both back.
//
// The entity the mutation just touched is available to the hook as
// [AfterEntityID] — mutators put it on the context before calling.
type After func(ctx context.Context, tx *sql.Tx) error

type afterEntityKey struct{}

// AfterEntityID is the primary key of the row the mutator just touched, when
// called from inside an [After] hook. Outside a hook it is "".
func AfterEntityID(ctx context.Context) string {
	if id, ok := ctx.Value(afterEntityKey{}).(string); ok {
		return id
	}
	return ""
}

// WithAfterEntity puts id on ctx for [AfterEntityID].
func WithAfterEntity(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, afterEntityKey{}, id)
}

// runAfter invokes every non-nil side effect in order.
func runAfter(ctx context.Context, tx *sql.Tx, after []After) error {
	for _, fn := range after {
		if fn == nil {
			continue
		}
		if err := fn(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// EngagementStatus is the lifecycle state of an assessment.
type EngagementStatus string

const (
	EngagementStatusDraft    EngagementStatus = "draft"
	EngagementStatusActive   EngagementStatus = "active"
	EngagementStatusClosed   EngagementStatus = "closed"
	EngagementStatusArchived EngagementStatus = "archived"
)

// Valid reports whether s is a known status.
func (s EngagementStatus) Valid() bool {
	switch s {
	case EngagementStatusDraft, EngagementStatusActive, EngagementStatusClosed, EngagementStatusArchived:
		return true
	}
	return false
}

// EngagementMode distinguishes standard from blind assessments.
type EngagementMode string

const (
	EngagementModeStandard EngagementMode = "standard"
	EngagementModeBlind    EngagementMode = "blind"
)

// Valid reports whether m is a known mode.
func (m EngagementMode) Valid() bool {
	return m == EngagementModeStandard || m == EngagementModeBlind
}

// ScenarioSource is the provenance of a scenario.
type ScenarioSource string

const (
	ScenarioSourceManual   ScenarioSource = "manual"
	ScenarioSourceCTID     ScenarioSource = "ctid"
	ScenarioSourceImported ScenarioSource = "imported"
)

// Valid reports whether s is a known source.
func (s ScenarioSource) Valid() bool {
	switch s {
	case ScenarioSourceManual, ScenarioSourceCTID, ScenarioSourceImported:
		return true
	}
	return false
}

// ExecutionStatus is the red-side state of one execution.
type ExecutionStatus string

const (
	ExecutionStatusPending  ExecutionStatus = "pending"
	ExecutionStatusRunning  ExecutionStatus = "running"
	ExecutionStatusComplete ExecutionStatus = "complete"
	ExecutionStatusBlocked  ExecutionStatus = "blocked"
	ExecutionStatusSkipped  ExecutionStatus = "skipped"
)

// Valid reports whether s is a known status.
func (s ExecutionStatus) Valid() bool {
	switch s {
	case ExecutionStatusPending, ExecutionStatusRunning, ExecutionStatusComplete, ExecutionStatusBlocked, ExecutionStatusSkipped:
		return true
	}
	return false
}

// DetectionCategory is the blue-side detection rating.
type DetectionCategory string

const (
	DetectionCategoryNone      DetectionCategory = "none"
	DetectionCategoryTelemetry DetectionCategory = "telemetry"
	DetectionCategoryGeneral   DetectionCategory = "general"
	DetectionCategoryTactic    DetectionCategory = "tactic"
	DetectionCategoryTechnique DetectionCategory = "technique"
)

// Valid reports whether c is a known category.
func (c DetectionCategory) Valid() bool {
	switch c {
	case DetectionCategoryNone, DetectionCategoryTelemetry, DetectionCategoryGeneral, DetectionCategoryTactic, DetectionCategoryTechnique:
		return true
	}
	return false
}

// DetectionModifiers are the flags that qualify a detection category without
// changing the ordinal. Stored as a JSON array; validated in M3-008.
var DetectionModifierValues = []string{
	"alert", "correlated", "delayed", "config_change", "residual_artifact",
}

// Protection is the blue-side prevention rating.
type Protection string

const (
	ProtectionBlocked    Protection = "blocked"
	ProtectionPartial    Protection = "partial"
	ProtectionNotBlocked Protection = "not_blocked"
	ProtectionNA         Protection = "n/a"
)

// Valid reports whether p is a known protection level.
func (p Protection) Valid() bool {
	switch p {
	case ProtectionBlocked, ProtectionPartial, ProtectionNotBlocked, ProtectionNA:
		return true
	}
	return false
}

// FindingStatus is the lifecycle of a remediation item.
type FindingStatus string

const (
	FindingStatusOpen         FindingStatus = "open"
	FindingStatusInProgress   FindingStatus = "in_progress"
	FindingStatusResolved     FindingStatus = "resolved"
	FindingStatusAcceptedRisk FindingStatus = "accepted_risk"
)

// Valid reports whether s is a known status.
func (s FindingStatus) Valid() bool {
	switch s {
	case FindingStatusOpen, FindingStatusInProgress, FindingStatusResolved, FindingStatusAcceptedRisk:
		return true
	}
	return false
}

// EvidenceSide is which team uploaded an evidence file.
type EvidenceSide string

const (
	EvidenceSideRed  EvidenceSide = "red"
	EvidenceSideBlue EvidenceSide = "blue"
)

// Valid reports whether s is a known side.
func (s EvidenceSide) Valid() bool {
	return s == EvidenceSideRed || s == EvidenceSideBlue
}

// ---------------------------------------------------------------------------
// Domain types — committed rows
// ---------------------------------------------------------------------------

// Engagement is one assessment.
type Engagement struct {
	ID                string
	Name              string
	Client            string
	Description       string
	Status            EngagementStatus
	StartsOn          time.Time
	EndsOn            time.Time
	AttackVersion     string
	Mode              EngagementMode
	AutoRevealOnStart bool
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewEngagement is the caller's half of creating an engagement.
type NewEngagement struct {
	Name              string
	Client            string
	Description       string
	StartsOn          time.Time
	EndsOn            time.Time
	AttackVersion     string
	Mode              EngagementMode
	AutoRevealOnStart bool
	CreatedBy         string
}

// Scenario is one ordered attack-chain section inside an engagement.
type Scenario struct {
	ID           string
	EngagementID string
	Ordinal      int
	Name         string
	Narrative    string
	Source       ScenarioSource
	ThreatActor  string
	SourceRef    string
	PlanID       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewScenario is the caller's half of creating a scenario.
type NewScenario struct {
	EngagementID string
	Ordinal      int
	Name         string
	Narrative    string
	Source       ScenarioSource
	ThreatActor  string
	SourceRef    string
	PlanID       string
}

// Step is one technique/procedure row under a scenario. Identity fields
// (technique_id, subtechnique_id, tactic_id, procedure, template_id) are
// frozen once the step's execution leaves pending (soft freeze, M3-005).
type Step struct {
	ID              string
	ScenarioID      string
	Ordinal         int
	Name            string
	Objective       string
	TechniqueID     string
	SubtechniqueID  string
	TacticID        string
	Procedure       json.RawMessage
	TemplateID      string
	TargetAsset     string
	Tools           json.RawMessage
	ControlsInScope json.RawMessage
	AttackVersion   string
	RevealedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewStep is the caller's half of creating a step.
type NewStep struct {
	ScenarioID      string
	Ordinal         int
	Name            string
	Objective       string
	TechniqueID     string
	SubtechniqueID  string
	TacticID        string
	Procedure       json.RawMessage
	TemplateID      string
	TargetAsset     string
	Tools           json.RawMessage
	ControlsInScope json.RawMessage
	AttackVersion   string
}

// Execution is the red + blue fill-in for one step. One execution per step
// (UNIQUE(step_id)). Version is the optimistic-lock column.
type Execution struct {
	ID      string
	StepID  string
	Version int

	// Red side
	Status     ExecutionStatus
	ExecutedBy string
	StartedAt  *time.Time
	EndedAt    *time.Time
	CommandRun string
	SourceHost string
	TargetHost string
	RedNotes   string

	// Blue side
	DetectionCategory  *DetectionCategory
	DetectionModifiers json.RawMessage
	Protection         *Protection
	DetectedAt         *time.Time
	DetectingSource    string
	DetectingRuleRef   string
	AlertSeverity      string
	BlueNotes          string
	ScoredBy           string
	ScoredAt           *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Finding is a remediation item on an engagement.
type Finding struct {
	ID                   string
	EngagementID         string
	Title                string
	Description          string
	Severity             string
	Recommendation       string
	Owner                string
	Status               FindingStatus
	CreatedFromExecution string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// NewFinding is the caller's half of creating a finding.
type NewFinding struct {
	EngagementID         string
	Title                string
	Description          string
	Severity             string
	Recommendation       string
	Owner                string
	CreatedFromExecution string
	// CreatedBy is the user id recorded in the finding_status_history
	// creation row (NULL → open).
	CreatedBy string
}

// PatchFinding is the caller's half of updating a finding. Only non-empty
// fields are applied; "" means "leave unchanged".
type PatchFinding struct {
	Title          string
	Description    string
	Severity       string
	Recommendation string
	Owner          string
	Status         string
	// ChangedBy is the user id recorded in finding_status_history when
	// this patch changes the status. Ignored when Status is "" or
	// matches the current value.
	ChangedBy string
}

// FindingStatusHistory is one status transition for a finding.
// Rows are append-only; there is no update or delete path except
// the cascade that follows a finding deletion.
type FindingStatusHistory struct {
	ID           string
	FindingID    string
	EngagementID string
	FromStatus   *FindingStatus // NULL means creation
	ToStatus     FindingStatus
	ChangedBy    string
	ChangedAt    time.Time
}

// EvidenceBlob is one content-addressed file stored on disk.
type EvidenceBlob struct {
	SHA256      string
	Size        int64
	MIME        string
	StoragePath string
	RefCount    int
	CreatedAt   time.Time
}

// Evidence is the metadata row linking a blob to an execution or comment.
type Evidence struct {
	ID          string
	BlobSHA256  string
	Filename    string
	Caption     string
	Side        EvidenceSide
	ExecutionID string
	CommentID   string
	UploadedBy  string
	UploadedAt  time.Time
	Size        int64
	MIME        string
}

// NewEvidence is the caller's half of linking a blob.
type NewEvidence struct {
	BlobSHA256  string
	Filename    string
	Caption     string
	Side        EvidenceSide
	ExecutionID string // XOR with CommentID
	CommentID   string // XOR with ExecutionID
	UploadedBy  string
	Size        int64
	MIME        string
}

// Comment is one threaded note on an execution.
type Comment struct {
	ID          string
	ExecutionID string
	AuthorID    string
	Body        string
	CreatedAt   time.Time
	EditedAt    *time.Time
}

// NewComment is the caller's half of writing a comment.
type NewComment struct {
	ExecutionID string
	AuthorID    string
	Body        string
}

// CommentRevision is one historical body of an edited comment.
type CommentRevision struct {
	ID        string
	CommentID string
	Body      string
	EditedBy  string
	EditedAt  time.Time
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newID mints a UUIDv7: sortable by creation time, so ORDER BY id is a stable
// tiebreaker and rows arrive in the order they were made.
func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// now is the timestamp written to a row: UTC, and truncated to the microsecond
// DuckDB's TIMESTAMP stores. Without the truncation a value read back would
// differ from the one written by up to a microsecond.
func now() time.Time { return toStorage(time.Now()) }

// toStorage prepares a caller's timestamp the same way, so that a time supplied
// to a repository and a time generated by it behave alike.
func toStorage(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// nullString maps Go's "" to SQL NULL for nullable text columns.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fromNullString maps SQL NULL to "".
func fromNullString(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// nullTime maps a nil *time.Time to SQL NULL.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return toStorage(*t)
}

// fromNullTime maps SQL NULL to nil, otherwise to a pointer.
func fromNullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// requireOneRow turns "the statement matched nothing" into [apierr.NotFound].
func requireOneRow(result sql.Result, resource, id string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	switch n {
	case 1:
		return nil
	case 0:
		return apierr.NotFound(resource, id)
	default:
		return err
	}
}

// jsonBytes normalises whatever the driver handed back for a JSON column into
// json.RawMessage. The DuckDB driver may return nil, []byte, string, or a
// decoded Go value (map[string]any, []any) depending on the column type.
func jsonBytes(v any) (json.RawMessage, error) {
	switch d := v.(type) {
	case nil:
		return json.RawMessage(`{}`), nil
	case []byte:
		if len(d) == 0 {
			return json.RawMessage(`{}`), nil
		}
		buf := make([]byte, len(d))
		copy(buf, d)
		return json.RawMessage(buf), nil
	case string:
		if d == "" {
			return json.RawMessage(`{}`), nil
		}
		return json.RawMessage(d), nil
	default:
		raw, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	}
}

// bindJSON returns a driver-friendly value for a JSON array column.
func bindJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	buf := make([]byte, len(raw))
	copy(buf, raw)
	return buf
}

// bindJSONObject is bindJSON for object-shaped defaults.
func bindJSONObject(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	buf := make([]byte, len(raw))
	copy(buf, raw)
	return buf
}
