package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Custom is the user-authored content surface (M2-011): CRUD for procedure
// templates, detection rule refs, and KB notes under the singleton custom
// source, plus YAML/JSON export.
//
// Authorization is not decided here. Callers already hold content.read or
// content.manage (or are blctl).
type Custom struct {
	sources    *storecontent.Sources
	procedures *storecontent.Procedures
	detections *storecontent.Detections
	notes      *storecontent.Notes
	activity   *events.Log
	// noteMaxBytes caps markdown body size on notes. Zero means the package
	// default (256 KiB).
	noteMaxBytes int
}

// CustomDeps is everything a [Custom] is built from.
type CustomDeps struct {
	Sources      *storecontent.Sources
	Procedures   *storecontent.Procedures
	Detections   *storecontent.Detections
	Notes        *storecontent.Notes
	Activity     *events.Log
	NoteMaxBytes int
}

// DefaultNoteMaxBytes is the markdown body cap when config does not set one.
const DefaultNoteMaxBytes = 256 << 10 // 256 KiB

// NewCustom returns a Custom service, or an error naming what is missing.
func NewCustom(deps CustomDeps) (*Custom, error) {
	switch {
	case deps.Sources == nil:
		return nil, fmt.Errorf("content: custom: Sources is required")
	case deps.Procedures == nil:
		return nil, fmt.Errorf("content: custom: Procedures is required")
	case deps.Detections == nil:
		return nil, fmt.Errorf("content: custom: Detections is required")
	case deps.Notes == nil:
		return nil, fmt.Errorf("content: custom: Notes is required")
	}
	max := deps.NoteMaxBytes
	if max <= 0 {
		max = DefaultNoteMaxBytes
	}
	return &Custom{
		sources:      deps.Sources,
		procedures:   deps.Procedures,
		detections:   deps.Detections,
		notes:        deps.Notes,
		activity:     deps.Activity,
		noteMaxBytes: max,
	}, nil
}

// techniqueIDRE matches MITRE ATT&CK technique external ids: T#### or T####.###.
var techniqueIDRE = regexp.MustCompile(`^T\d{4}(\.\d{3})?$`)

// ExportType selects which object families an export includes.
type ExportType string

const (
	ExportAll                ExportType = ""
	ExportProcedureTemplates ExportType = "procedure_templates"
	ExportDetectionRules     ExportType = "detection_rules"
	ExportNotes              ExportType = "notes"
)

// ExportFormat is the serialization the caller asked for.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportYAML ExportFormat = "yaml"
)

// ExportDoc is the document shape returned by [Custom.Export]. Wire mappers
// turn the store rows into OpenAPI types; this is the domain half.
type ExportDoc struct {
	Meta               ExportMeta
	ProcedureTemplates []storecontent.ProcedureTemplate
	DetectionRules     []storecontent.DetectionRuleRef
	Notes              []storecontent.Note
}

// ExportMeta is license/attribution for the installation's custom library.
type ExportMeta struct {
	SourceName  string
	LicenseSPDX string
	LicenseName string
	LicenseURL  string
	Attribution string
	ExportedAt  time.Time
}

// ProcedureCreate is the validated input for creating a custom template.
type ProcedureCreate struct {
	ExternalID             string
	Name                   string
	Description            string
	Platforms              []string
	Executor               string
	ElevationRequired      bool
	Command                string
	Cleanup                string
	InputArgs              json.RawMessage // JSON array of {name,description,type,default}
	TechniqueExternalIDs   []string
	DependencyExecutorName string
	Dependencies           string
}

// ProcedureEdit is a partial patch. Nil means "leave alone".
type ProcedureEdit struct {
	Name                   *string
	Description            *string
	Platforms              *[]string
	Executor               *string
	ElevationRequired      *bool
	Command                *string
	Cleanup                *string
	InputArgs              *json.RawMessage
	TechniqueExternalIDs   *[]string
	DependencyExecutorName *string
	Dependencies           *string
}

// DetectionCreate is the validated input for creating a custom detection ref.
type DetectionCreate struct {
	ExternalID           string
	Name                 string
	Description          string
	TechniqueExternalIDs []string
	Level                string
	RuleStatus           string
	Logsource            json.RawMessage // JSON object
	RuleYAML             string
}

// DetectionEdit is a partial patch.
type DetectionEdit struct {
	Name                 *string
	Description          *string
	TechniqueExternalIDs *[]string
	Level                *string
	RuleStatus           *string
	Logsource            *json.RawMessage
	RuleYAML             *string
}

// NoteCreate is the validated input for creating a custom note.
type NoteCreate struct {
	ExternalID          string
	Title               string
	BodyMarkdown        string
	Tags                []string
	TechniqueExternalID string
}

// NoteEdit is a partial patch.
type NoteEdit struct {
	Title               *string
	BodyMarkdown        *string
	Tags                *[]string
	TechniqueExternalID *string
}

// ListProcedures returns custom procedure templates matching f.
func (c *Custom) ListProcedures(ctx context.Context, f storecontent.ProcedureListFilter) ([]storecontent.ProcedureTemplate, error) {
	f.SourceID = storecontent.SourceIDCustom
	f.Version = storecontent.VersionCurrent
	f.EnabledOnly = false
	return c.procedures.List(ctx, f)
}

// GetProcedure returns one custom procedure template, or [apierr.NotFound].
func (c *Custom) GetProcedure(ctx context.Context, id string) (storecontent.ProcedureTemplate, error) {
	p, err := c.procedures.ByID(ctx, id)
	if err != nil {
		return storecontent.ProcedureTemplate{}, err
	}
	if p.SourceID != storecontent.SourceIDCustom {
		return storecontent.ProcedureTemplate{}, apierr.NotFound("content_procedure_template", id)
	}
	return p, nil
}

// CreateProcedure inserts a custom procedure template.
func (c *Custom) CreateProcedure(ctx context.Context, actor authn.Subject, in ProcedureCreate) (storecontent.ProcedureTemplate, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return storecontent.ProcedureTemplate{}, apierr.Validation(apierr.Field("name", "name must not be empty"))
	}
	if err := validateTechniqueIDs(in.TechniqueExternalIDs, "techniqueExternalIds"); err != nil {
		return storecontent.ProcedureTemplate{}, err
	}

	id, err := storecontent.NewID()
	if err != nil {
		return storecontent.ProcedureTemplate{}, err
	}
	ext := strings.TrimSpace(in.ExternalID)
	if ext == "" {
		ext = id
	}

	row := storecontent.ProcedureTemplate{
		ID:                     id,
		SourceID:               storecontent.SourceIDCustom,
		Version:                storecontent.VersionCurrent,
		ExternalID:             ext,
		Name:                   name,
		Description:            in.Description,
		Platforms:              mustJSONArray(in.Platforms),
		Executor:               in.Executor,
		ElevationRequired:      in.ElevationRequired,
		Command:                in.Command,
		Cleanup:                in.Cleanup,
		InputArgs:              defaultJSONArray(in.InputArgs),
		TechniqueExternalIDs:   mustJSONArray(in.TechniqueExternalIDs),
		DependencyExecutorName: in.DependencyExecutorName,
		Dependencies:           in.Dependencies,
	}
	delta := map[string]any{
		"objectType": "procedure_template",
		"name":       row.Name,
		"externalId": row.ExternalID,
	}
	return c.procedures.Create(ctx, row, c.recordInTx(actor.UserID, events.VerbContentCustomCreated, events.ObjectContentProcedureTemplate, delta))
}

// UpdateProcedure applies a patch to a custom procedure template.
func (c *Custom) UpdateProcedure(ctx context.Context, actor authn.Subject, id string, edit ProcedureEdit) (storecontent.ProcedureTemplate, error) {
	before, err := c.GetProcedure(ctx, id)
	if err != nil {
		return storecontent.ProcedureTemplate{}, err
	}
	after := before
	delta := map[string]any{"objectType": "procedure_template"}

	if edit.Name != nil {
		name := strings.TrimSpace(*edit.Name)
		if name == "" {
			return storecontent.ProcedureTemplate{}, apierr.Validation(apierr.Field("name", "name must not be empty"))
		}
		if name != before.Name {
			delta["name"] = change(before.Name, name)
			after.Name = name
		}
	}
	if edit.Description != nil && *edit.Description != before.Description {
		delta["description"] = change(before.Description, *edit.Description)
		after.Description = *edit.Description
	}
	if edit.Platforms != nil {
		raw := mustJSONArray(*edit.Platforms)
		if string(raw) != string(before.Platforms) {
			delta["platforms"] = change(string(before.Platforms), string(raw))
			after.Platforms = raw
		}
	}
	if edit.Executor != nil && *edit.Executor != before.Executor {
		delta["executor"] = change(before.Executor, *edit.Executor)
		after.Executor = *edit.Executor
	}
	if edit.ElevationRequired != nil && *edit.ElevationRequired != before.ElevationRequired {
		delta["elevationRequired"] = change(before.ElevationRequired, *edit.ElevationRequired)
		after.ElevationRequired = *edit.ElevationRequired
	}
	if edit.Command != nil && *edit.Command != before.Command {
		delta["command"] = change(before.Command, *edit.Command)
		after.Command = *edit.Command
	}
	if edit.Cleanup != nil && *edit.Cleanup != before.Cleanup {
		delta["cleanup"] = change(before.Cleanup, *edit.Cleanup)
		after.Cleanup = *edit.Cleanup
	}
	if edit.InputArgs != nil {
		raw := defaultJSONArray(*edit.InputArgs)
		if string(raw) != string(before.InputArgs) {
			delta["inputArgs"] = map[string]any{"changed": true}
			after.InputArgs = raw
		}
	}
	if edit.TechniqueExternalIDs != nil {
		if err := validateTechniqueIDs(*edit.TechniqueExternalIDs, "techniqueExternalIds"); err != nil {
			return storecontent.ProcedureTemplate{}, err
		}
		raw := mustJSONArray(*edit.TechniqueExternalIDs)
		if string(raw) != string(before.TechniqueExternalIDs) {
			delta["techniqueExternalIds"] = change(string(before.TechniqueExternalIDs), string(raw))
			after.TechniqueExternalIDs = raw
		}
	}
	if edit.DependencyExecutorName != nil && *edit.DependencyExecutorName != before.DependencyExecutorName {
		delta["dependencyExecutorName"] = change(before.DependencyExecutorName, *edit.DependencyExecutorName)
		after.DependencyExecutorName = *edit.DependencyExecutorName
	}
	if edit.Dependencies != nil && *edit.Dependencies != before.Dependencies {
		delta["dependencies"] = change(before.Dependencies, *edit.Dependencies)
		after.Dependencies = *edit.Dependencies
	}
	if len(delta) == 1 { // only objectType
		return before, nil
	}
	return c.procedures.Update(ctx, after, c.recordInTx(actor.UserID, events.VerbContentCustomUpdated, events.ObjectContentProcedureTemplate, delta))
}

// DeleteProcedure hard-deletes a custom procedure template when unreferenced.
func (c *Custom) DeleteProcedure(ctx context.Context, actor authn.Subject, id string) error {
	before, err := c.GetProcedure(ctx, id)
	if err != nil {
		return err
	}
	if n := customRefCount(); n > 0 {
		return apierr.Conflict(fmt.Sprintf("procedure template is referenced by %d engagement object(s)", n))
	}
	delta := map[string]any{
		"objectType": "procedure_template",
		"name":       before.Name,
		"externalId": before.ExternalID,
	}
	return c.procedures.Delete(ctx, id, c.recordInTx(actor.UserID, events.VerbContentCustomDeleted, events.ObjectContentProcedureTemplate, delta))
}

// ListDetections returns custom detection rule refs matching f.
func (c *Custom) ListDetections(ctx context.Context, f storecontent.DetectionListFilter) ([]storecontent.DetectionRuleRef, error) {
	f.SourceID = storecontent.SourceIDCustom
	f.Version = storecontent.VersionCurrent
	f.EnabledOnly = false
	return c.detections.List(ctx, f)
}

// GetDetection returns one custom detection rule ref, or [apierr.NotFound].
func (c *Custom) GetDetection(ctx context.Context, id string) (storecontent.DetectionRuleRef, error) {
	d, err := c.detections.ByID(ctx, id)
	if err != nil {
		return storecontent.DetectionRuleRef{}, err
	}
	if d.SourceID != storecontent.SourceIDCustom {
		return storecontent.DetectionRuleRef{}, apierr.NotFound("content_detection_rule_ref", id)
	}
	return d, nil
}

// CreateDetection inserts a custom detection rule ref.
func (c *Custom) CreateDetection(ctx context.Context, actor authn.Subject, in DetectionCreate) (storecontent.DetectionRuleRef, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return storecontent.DetectionRuleRef{}, apierr.Validation(apierr.Field("name", "name must not be empty"))
	}
	if strings.TrimSpace(in.RuleYAML) == "" {
		return storecontent.DetectionRuleRef{}, apierr.Validation(apierr.Field("ruleYaml", "ruleYaml must not be empty"))
	}
	if err := validateTechniqueIDs(in.TechniqueExternalIDs, "techniqueExternalIds"); err != nil {
		return storecontent.DetectionRuleRef{}, err
	}

	id, err := storecontent.NewID()
	if err != nil {
		return storecontent.DetectionRuleRef{}, err
	}
	ext := strings.TrimSpace(in.ExternalID)
	if ext == "" {
		ext = id
	}

	row := storecontent.DetectionRuleRef{
		ID:                   id,
		SourceID:             storecontent.SourceIDCustom,
		Version:              storecontent.VersionCurrent,
		ExternalID:           ext,
		Name:                 name,
		Description:          in.Description,
		TechniqueExternalIDs: mustJSONArray(in.TechniqueExternalIDs),
		Level:                in.Level,
		RuleStatus:           in.RuleStatus,
		Logsource:            defaultJSONObject(in.Logsource),
		RuleYAML:             in.RuleYAML,
	}
	delta := map[string]any{
		"objectType": "detection_rule_ref",
		"name":       row.Name,
		"externalId": row.ExternalID,
	}
	return c.detections.Create(ctx, row, c.recordInTx(actor.UserID, events.VerbContentCustomCreated, events.ObjectContentDetectionRuleRef, delta))
}

// UpdateDetection applies a patch to a custom detection rule ref.
func (c *Custom) UpdateDetection(ctx context.Context, actor authn.Subject, id string, edit DetectionEdit) (storecontent.DetectionRuleRef, error) {
	before, err := c.GetDetection(ctx, id)
	if err != nil {
		return storecontent.DetectionRuleRef{}, err
	}
	after := before
	delta := map[string]any{"objectType": "detection_rule_ref"}

	if edit.Name != nil {
		name := strings.TrimSpace(*edit.Name)
		if name == "" {
			return storecontent.DetectionRuleRef{}, apierr.Validation(apierr.Field("name", "name must not be empty"))
		}
		if name != before.Name {
			delta["name"] = change(before.Name, name)
			after.Name = name
		}
	}
	if edit.Description != nil && *edit.Description != before.Description {
		delta["description"] = change(before.Description, *edit.Description)
		after.Description = *edit.Description
	}
	if edit.TechniqueExternalIDs != nil {
		if err := validateTechniqueIDs(*edit.TechniqueExternalIDs, "techniqueExternalIds"); err != nil {
			return storecontent.DetectionRuleRef{}, err
		}
		raw := mustJSONArray(*edit.TechniqueExternalIDs)
		if string(raw) != string(before.TechniqueExternalIDs) {
			delta["techniqueExternalIds"] = change(string(before.TechniqueExternalIDs), string(raw))
			after.TechniqueExternalIDs = raw
		}
	}
	if edit.Level != nil && *edit.Level != before.Level {
		delta["level"] = change(before.Level, *edit.Level)
		after.Level = *edit.Level
	}
	if edit.RuleStatus != nil && *edit.RuleStatus != before.RuleStatus {
		delta["status"] = change(before.RuleStatus, *edit.RuleStatus)
		after.RuleStatus = *edit.RuleStatus
	}
	if edit.Logsource != nil {
		raw := defaultJSONObject(*edit.Logsource)
		if string(raw) != string(before.Logsource) {
			delta["logsource"] = map[string]any{"changed": true}
			after.Logsource = raw
		}
	}
	if edit.RuleYAML != nil {
		if strings.TrimSpace(*edit.RuleYAML) == "" {
			return storecontent.DetectionRuleRef{}, apierr.Validation(apierr.Field("ruleYaml", "ruleYaml must not be empty"))
		}
		if *edit.RuleYAML != before.RuleYAML {
			delta["ruleYaml"] = map[string]any{"changed": true}
			after.RuleYAML = *edit.RuleYAML
		}
	}
	if len(delta) == 1 {
		return before, nil
	}
	return c.detections.Update(ctx, after, c.recordInTx(actor.UserID, events.VerbContentCustomUpdated, events.ObjectContentDetectionRuleRef, delta))
}

// DeleteDetection hard-deletes a custom detection rule ref when unreferenced.
func (c *Custom) DeleteDetection(ctx context.Context, actor authn.Subject, id string) error {
	before, err := c.GetDetection(ctx, id)
	if err != nil {
		return err
	}
	if n := customRefCount(); n > 0 {
		return apierr.Conflict(fmt.Sprintf("detection rule is referenced by %d engagement object(s)", n))
	}
	delta := map[string]any{
		"objectType": "detection_rule_ref",
		"name":       before.Name,
		"externalId": before.ExternalID,
	}
	return c.detections.Delete(ctx, id, c.recordInTx(actor.UserID, events.VerbContentCustomDeleted, events.ObjectContentDetectionRuleRef, delta))
}

// ListNotes returns custom notes matching f.
func (c *Custom) ListNotes(ctx context.Context, f storecontent.NoteListFilter) ([]storecontent.Note, error) {
	f.SourceID = storecontent.SourceIDCustom
	f.Version = storecontent.VersionCurrent
	f.EnabledOnly = false
	return c.notes.List(ctx, f)
}

// GetNote returns one custom note, or [apierr.NotFound].
func (c *Custom) GetNote(ctx context.Context, id string) (storecontent.Note, error) {
	n, err := c.notes.ByID(ctx, id)
	if err != nil {
		return storecontent.Note{}, err
	}
	if n.SourceID != storecontent.SourceIDCustom {
		return storecontent.Note{}, apierr.NotFound("content_note", id)
	}
	return n, nil
}

// CreateNote inserts a custom knowledge-base note.
func (c *Custom) CreateNote(ctx context.Context, actor authn.Subject, in NoteCreate) (storecontent.Note, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return storecontent.Note{}, apierr.Validation(apierr.Field("title", "title must not be empty"))
	}
	if err := c.validateNoteBody(in.BodyMarkdown); err != nil {
		return storecontent.Note{}, err
	}
	tech := strings.TrimSpace(in.TechniqueExternalID)
	if err := validateOptionalTechniqueID(tech, "techniqueExternalId"); err != nil {
		return storecontent.Note{}, err
	}

	id, err := storecontent.NewID()
	if err != nil {
		return storecontent.Note{}, err
	}
	ext := strings.TrimSpace(in.ExternalID)
	if ext == "" {
		ext = id
	}

	row := storecontent.Note{
		ID:                  id,
		SourceID:            storecontent.SourceIDCustom,
		Version:             storecontent.VersionCurrent,
		ExternalID:          ext,
		Title:               title,
		BodyMarkdown:        in.BodyMarkdown,
		Tags:                mustJSONArray(in.Tags),
		TechniqueExternalID: tech,
	}
	delta := map[string]any{
		"objectType": "content_note",
		"title":      row.Title,
		"externalId": row.ExternalID,
	}
	return c.notes.Create(ctx, row, c.recordInTx(actor.UserID, events.VerbContentCustomCreated, events.ObjectContentNote, delta))
}

// UpdateNote applies a patch to a custom note.
func (c *Custom) UpdateNote(ctx context.Context, actor authn.Subject, id string, edit NoteEdit) (storecontent.Note, error) {
	before, err := c.GetNote(ctx, id)
	if err != nil {
		return storecontent.Note{}, err
	}
	after := before
	delta := map[string]any{"objectType": "content_note"}

	if edit.Title != nil {
		title := strings.TrimSpace(*edit.Title)
		if title == "" {
			return storecontent.Note{}, apierr.Validation(apierr.Field("title", "title must not be empty"))
		}
		if title != before.Title {
			delta["title"] = change(before.Title, title)
			after.Title = title
		}
	}
	if edit.BodyMarkdown != nil {
		if err := c.validateNoteBody(*edit.BodyMarkdown); err != nil {
			return storecontent.Note{}, err
		}
		if *edit.BodyMarkdown != before.BodyMarkdown {
			delta["bodyMarkdown"] = map[string]any{"changed": true}
			after.BodyMarkdown = *edit.BodyMarkdown
		}
	}
	if edit.Tags != nil {
		raw := mustJSONArray(*edit.Tags)
		if string(raw) != string(before.Tags) {
			delta["tags"] = change(string(before.Tags), string(raw))
			after.Tags = raw
		}
	}
	if edit.TechniqueExternalID != nil {
		tech := strings.TrimSpace(*edit.TechniqueExternalID)
		if err := validateOptionalTechniqueID(tech, "techniqueExternalId"); err != nil {
			return storecontent.Note{}, err
		}
		if tech != before.TechniqueExternalID {
			delta["techniqueExternalId"] = change(before.TechniqueExternalID, tech)
			after.TechniqueExternalID = tech
		}
	}
	if len(delta) == 1 {
		return before, nil
	}
	return c.notes.Update(ctx, after, c.recordInTx(actor.UserID, events.VerbContentCustomUpdated, events.ObjectContentNote, delta))
}

// DeleteNote hard-deletes a custom note when unreferenced.
func (c *Custom) DeleteNote(ctx context.Context, actor authn.Subject, id string) error {
	before, err := c.GetNote(ctx, id)
	if err != nil {
		return err
	}
	if n := customRefCount(); n > 0 {
		return apierr.Conflict(fmt.Sprintf("note is referenced by %d engagement object(s)", n))
	}
	delta := map[string]any{
		"objectType": "content_note",
		"title":      before.Title,
		"externalId": before.ExternalID,
	}
	return c.notes.Delete(ctx, id, c.recordInTx(actor.UserID, events.VerbContentCustomDeleted, events.ObjectContentNote, delta))
}

// Export builds a document of custom content for re-import.
func (c *Custom) Export(ctx context.Context, typ ExportType) (ExportDoc, error) {
	src, err := c.sources.ByID(ctx, storecontent.SourceIDCustom)
	if err != nil {
		return ExportDoc{}, err
	}
	doc := ExportDoc{
		Meta: ExportMeta{
			SourceName:  src.Name,
			LicenseSPDX: src.LicenseSPDX,
			LicenseName: src.LicenseName,
			LicenseURL:  src.LicenseURL,
			Attribution: src.Attribution,
			ExportedAt:  time.Now().UTC().Truncate(time.Second),
		},
		ProcedureTemplates: []storecontent.ProcedureTemplate{},
		DetectionRules:     []storecontent.DetectionRuleRef{},
		Notes:              []storecontent.Note{},
	}

	wantAll := typ == ExportAll
	if wantAll || typ == ExportProcedureTemplates {
		items, err := c.ListProcedures(ctx, storecontent.ProcedureListFilter{})
		if err != nil {
			return ExportDoc{}, err
		}
		doc.ProcedureTemplates = items
	}
	if wantAll || typ == ExportDetectionRules {
		items, err := c.ListDetections(ctx, storecontent.DetectionListFilter{})
		if err != nil {
			return ExportDoc{}, err
		}
		doc.DetectionRules = items
	}
	if wantAll || typ == ExportNotes {
		items, err := c.ListNotes(ctx, storecontent.NoteListFilter{})
		if err != nil {
			return ExportDoc{}, err
		}
		doc.Notes = items
	}
	return doc, nil
}

func (c *Custom) validateNoteBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return apierr.Validation(apierr.Field("bodyMarkdown", "bodyMarkdown must not be empty"))
	}
	// Cap is bytes of the UTF-8 encoding — same unit the config key names.
	if len(body) > c.noteMaxBytes {
		return apierr.Validation(apierr.Field(
			"bodyMarkdown",
			fmt.Sprintf("bodyMarkdown exceeds the configured limit of %d bytes", c.noteMaxBytes),
		))
	}
	return nil
}

func (c *Custom) recordInTx(actorID string, verb events.Verb, objectType string, delta map[string]any) storecontent.After {
	if c.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return c.activity.Record(ctx, tx, events.Entry{
			ActorID:    actorID,
			Verb:       verb,
			ObjectType: objectType,
			ObjectID:   storecontent.AfterEntityID(ctx),
			Delta:      events.Delta(delta),
		})
	}
}

// customRefCount is the M2 stub for engagement reference counting. Always 0
// until M3 wires real refs; delete then answers 409 with counts.
func customRefCount() int { return 0 }

func validateTechniqueIDs(ids []string, field string) error {
	for i, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return apierr.Validation(apierr.Field(
				fmt.Sprintf("%s[%d]", field, i),
				"technique id must not be empty",
			))
		}
		if !techniqueIDRE.MatchString(id) {
			return apierr.Validation(apierr.Field(
				fmt.Sprintf("%s[%d]", field, i),
				fmt.Sprintf("%q is not a MITRE technique id (want T#### or T####.###)", id),
			))
		}
	}
	return nil
}

func validateOptionalTechniqueID(id, field string) error {
	if id == "" {
		return nil
	}
	if !techniqueIDRE.MatchString(id) {
		return apierr.Validation(apierr.Field(
			field,
			fmt.Sprintf("%q is not a MITRE technique id (want T#### or T####.###)", id),
		))
	}
	return nil
}

func mustJSONArray(ss []string) json.RawMessage {
	if ss == nil {
		ss = []string{}
	}
	b, err := json.Marshal(ss)
	if err != nil {
		// []string always marshals.
		return json.RawMessage(`[]`)
	}
	return b
}

func defaultJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
