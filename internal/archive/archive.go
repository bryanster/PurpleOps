package archive

import (
	"encoding/json"
	"time"

	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// FormatVersion is the integer that appears first in manifest.json. A reader
// that does not recognise it MUST bail before parsing anything else — this
// number is the only compatibility signal the format offers.
const FormatVersion = 1

// ---------------------------------------------------------------------------
// Archive member types — the JSON members inside the ZIP
// ---------------------------------------------------------------------------

// Manifest is the first thing written to the archive and the first thing a
// reader parses. Put FormatVersion first in the struct so it is first in the
// JSON output even to a reader that gives up early.
type Manifest struct {
	FormatVersion   int    `json:"formatVersion"`
	ExportedAt      string `json:"exportedAt"`
	ToolVersion     string `json:"toolVersion"`
	EngagementID    string `json:"engagementId"`
	EngagementName  string `json:"engagementName"`
	Client          string `json:"client"`
	Mode            string `json:"mode"`
	AttackVersion   string `json:"attackVersion"`
	BlindFiltered   bool   `json:"blindFiltered"`
}

// EngagementArchive is the top-level engagement.json member.
type EngagementArchive struct {
	Engagement EngagementJSON        `json:"engagement"`
	Scenarios  []ScenarioWithSteps   `json:"scenarios"`
	Findings   []FindingWithSteps    `json:"findings"`
	Comments   []CommentJSON         `json:"comments"`
}

// EngagementJSON is the archive-safe representation of an engagement header.
type EngagementJSON struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Client            string    `json:"client"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	StartsOn          time.Time `json:"startsOn"`
	EndsOn            time.Time `json:"endsOn"`
	AttackVersion     string    `json:"attackVersion"`
	Mode              string    `json:"mode"`
	AutoRevealOnStart bool      `json:"autoRevealOnStart"`
	CreatedBy         UserRef   `json:"createdBy"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ScenarioWithSteps nests a scenario and its ordered steps.
type ScenarioWithSteps struct {
	Scenario ScenarioJSON  `json:"scenario"`
	Steps    []StepWithExec `json:"steps"`
}

// ScenarioJSON is the archive-safe representation of a scenario.
type ScenarioJSON struct {
	ID           string    `json:"id"`
	EngagementID string    `json:"engagementId"`
	Ordinal      int       `json:"ordinal"`
	Name         string    `json:"name"`
	Narrative    string    `json:"narrative"`
	Source       string    `json:"source"`
	ThreatActor  string    `json:"threatActor"`
	SourceRef    string    `json:"sourceRef"`
	PlanID       string    `json:"planId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// StepWithExec nests a step and its execution.
type StepWithExec struct {
	Step      StepJSON      `json:"step"`
	Execution ExecutionJSON `json:"execution"`
}

// StepJSON is the archive-safe representation of a step.
type StepJSON struct {
	ID              string          `json:"id"`
	ScenarioID      string          `json:"scenarioId"`
	Ordinal         int             `json:"ordinal"`
	Name            string          `json:"name"`
	Objective       string          `json:"objective"`
	TechniqueID     string          `json:"techniqueId"`
	SubtechniqueID  string          `json:"subtechniqueId"`
	TacticID        string          `json:"tacticId"`
	Procedure       json.RawMessage `json:"procedure,omitempty"`
	TemplateID      string          `json:"templateId"`
	TargetAsset     string          `json:"targetAsset"`
	Tools           json.RawMessage `json:"tools,omitempty"`
	ControlsInScope json.RawMessage `json:"controlsInScope,omitempty"`
	AttackVersion   string          `json:"attackVersion"`
	RevealedAt      *time.Time      `json:"revealedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// ExecutionJSON is the archive-safe representation of an execution.
type ExecutionJSON struct {
	ID      string              `json:"id"`
	StepID  string              `json:"stepId"`
	Version int                 `json:"version"`
	Status  ExecutionStatusJSON `json:"status"`

	// Red side
	ExecutedBy UserRef    `json:"executedBy,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	CommandRun string     `json:"commandRun,omitempty"`
	SourceHost string     `json:"sourceHost,omitempty"`
	TargetHost string     `json:"targetHost,omitempty"`
	RedNotes   string     `json:"redNotes,omitempty"`

	// Blue side
	DetectionCategory  *string        `json:"detectionCategory,omitempty"`
	DetectionModifiers json.RawMessage `json:"detectionModifiers,omitempty"`
	Protection         *string        `json:"protection,omitempty"`
	DetectedAt         *time.Time     `json:"detectedAt,omitempty"`
	DetectingSource    string         `json:"detectingSource,omitempty"`
	DetectingRuleRef   string         `json:"detectingRuleRef,omitempty"`
	AlertSeverity      string         `json:"alertSeverity,omitempty"`
	BlueNotes          string         `json:"blueNotes,omitempty"`
	ScoredBy           UserRef        `json:"scoredBy,omitempty"`
	ScoredAt           *time.Time     `json:"scoredAt,omitempty"`
}

// ExecutionStatusJSON is the archive-safe execution status.
type ExecutionStatusJSON struct {
	Status    string     `json:"status"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// FindingWithSteps nests a finding and its linked step IDs.
type FindingWithSteps struct {
	Finding       FindingJSON `json:"finding"`
	LinkedStepIDs []string    `json:"linkedStepIds"`
}

// FindingJSON is the archive-safe representation of a finding.
type FindingJSON struct {
	ID                   string    `json:"id"`
	EngagementID         string    `json:"engagementId"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	Severity             string    `json:"severity"`
	Recommendation       string    `json:"recommendation"`
	Owner                UserRef   `json:"owner,omitempty"`
	Status               string    `json:"status"`
	CreatedFromExecution string    `json:"createdFromExecution,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// CommentJSON is the archive-safe representation of a comment.
type CommentJSON struct {
	ID          string     `json:"id"`
	ExecutionID string     `json:"executionId"`
	Author      UserRef    `json:"author"`
	Body        string     `json:"body"`
	CreatedAt   time.Time  `json:"createdAt"`
	EditedAt    *time.Time `json:"editedAt,omitempty"`
}

// UserRef is a user identifier + display name pair. No email, no password
// hash, no session token, no MFA secret — the archive leaves the building.
type UserRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// EvidenceManifest lists every evidence metadata row in the archive.
type EvidenceManifest struct {
	Entries []EvidenceEntry `json:"entries"`
}

// EvidenceEntry is one evidence metadata row in the archive.
type EvidenceEntry struct {
	ID          string    `json:"id"`
	BlobSHA256  string    `json:"blobSha256"`
	Filename    string    `json:"filename"`
	Caption     string    `json:"caption,omitempty"`
	Side        string    `json:"side"`
	ExecutionID string    `json:"executionId,omitempty"`
	CommentID   string    `json:"commentId,omitempty"`
	UploadedBy  UserRef   `json:"uploadedBy"`
	UploadedAt  time.Time `json:"uploadedAt"`
	Size        int64     `json:"size"`
	MIME        string    `json:"mime"`
}

// ---------------------------------------------------------------------------
// Conversion helpers — exported for use by httpapi and cli packages
// ---------------------------------------------------------------------------

// EngagementToJSON converts a store engagement to its archive-safe representation.
func EngagementToJSON(e storengagement.Engagement, createdBy UserRef) EngagementJSON {
	return EngagementJSON{
		ID:                e.ID,
		Name:              e.Name,
		Client:            e.Client,
		Description:       e.Description,
		Status:            string(e.Status),
		StartsOn:          e.StartsOn,
		EndsOn:            e.EndsOn,
		AttackVersion:     e.AttackVersion,
		Mode:              string(e.Mode),
		AutoRevealOnStart: e.AutoRevealOnStart,
		CreatedBy:         createdBy,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}
}

// ScenarioToJSON converts a store scenario to its archive-safe representation.
func ScenarioToJSON(s storengagement.Scenario) ScenarioJSON {
	return ScenarioJSON{
		ID:           s.ID,
		EngagementID: s.EngagementID,
		Ordinal:      s.Ordinal,
		Name:         s.Name,
		Narrative:    s.Narrative,
		Source:       string(s.Source),
		ThreatActor:  s.ThreatActor,
		SourceRef:    s.SourceRef,
		PlanID:       s.PlanID,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// StepToJSON converts a store step to its archive-safe representation.
func StepToJSON(s storengagement.Step) StepJSON {
	return StepJSON{
		ID:              s.ID,
		ScenarioID:      s.ScenarioID,
		Ordinal:         s.Ordinal,
		Name:            s.Name,
		Objective:       s.Objective,
		TechniqueID:     s.TechniqueID,
		SubtechniqueID:  s.SubtechniqueID,
		TacticID:        s.TacticID,
		Procedure:       s.Procedure,
		TemplateID:      s.TemplateID,
		TargetAsset:     s.TargetAsset,
		Tools:           s.Tools,
		ControlsInScope: s.ControlsInScope,
		AttackVersion:   s.AttackVersion,
		RevealedAt:      s.RevealedAt,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// ExecutionToJSON converts a store execution to its archive-safe representation.
func ExecutionToJSON(e storengagement.Execution, executedBy, scoredBy UserRef) ExecutionJSON {
	j := ExecutionJSON{
		ID:      e.ID,
		StepID:  e.StepID,
		Version: e.Version,
		Status: ExecutionStatusJSON{
			Status:    string(e.Status),
			StartedAt: e.StartedAt,
			EndedAt:   e.EndedAt,
		},
		ExecutedBy:  executedBy,
		CommandRun:  e.CommandRun,
		SourceHost:  e.SourceHost,
		TargetHost:  e.TargetHost,
		RedNotes:    e.RedNotes,
		ScoredBy:    scoredBy,
		ScoredAt:    e.ScoredAt,
	}
	if e.DetectionCategory != nil {
		v := string(*e.DetectionCategory)
		j.DetectionCategory = &v
	}
	if e.Protection != nil {
		v := string(*e.Protection)
		j.Protection = &v
	}
	j.DetectionModifiers = e.DetectionModifiers
	j.DetectedAt = e.DetectedAt
	j.DetectingSource = e.DetectingSource
	j.DetectingRuleRef = e.DetectingRuleRef
	j.AlertSeverity = e.AlertSeverity
	j.BlueNotes = e.BlueNotes
	return j
}

// FindingToJSON converts a store finding to its archive-safe representation.
func FindingToJSON(f storengagement.Finding, owner UserRef, stepIDs []string) FindingWithSteps {
	return FindingWithSteps{
		Finding: FindingJSON{
			ID:                   f.ID,
			EngagementID:         f.EngagementID,
			Title:                f.Title,
			Description:          f.Description,
			Severity:             f.Severity,
			Recommendation:       f.Recommendation,
			Owner:                owner,
			Status:               string(f.Status),
			CreatedFromExecution: f.CreatedFromExecution,
			CreatedAt:            f.CreatedAt,
			UpdatedAt:            f.UpdatedAt,
		},
		LinkedStepIDs: stepIDs,
	}
}

// CommentToJSON converts a store comment to its archive-safe representation.
func CommentToJSON(c storengagement.Comment, author UserRef) CommentJSON {
	return CommentJSON{
		ID:          c.ID,
		ExecutionID: c.ExecutionID,
		Author:      author,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
		EditedAt:    c.EditedAt,
	}
}

// EvidenceToEntry converts a store evidence row to its archive-safe representation.
func EvidenceToEntry(e storengagement.Evidence, uploadedBy UserRef) EvidenceEntry {
	return EvidenceEntry{
		ID:          e.ID,
		BlobSHA256:  e.BlobSHA256,
		Filename:    e.Filename,
		Caption:     e.Caption,
		Side:        string(e.Side),
		ExecutionID: e.ExecutionID,
		CommentID:   e.CommentID,
		UploadedBy:  uploadedBy,
		UploadedAt:  e.UploadedAt,
		Size:        e.Size,
		MIME:        e.MIME,
	}
}
