package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content/v1import"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// SyncImportMaxBytes is the largest upload that runs in-request. Larger
// uploads are spooled and run as a v1_import job (same global job slot).
const SyncImportMaxBytes int64 = 1 << 20 // 1 MiB

// ImportRequest is the caller half of a v1/custom import.
type ImportRequest struct {
	// Format is auto | testcases_json | testcases_yaml | knowledgebase_yaml.
	Format string
	// DryRun parses and counts without writing.
	DryRun bool
	// FailFast stops applying at the first per-item error (parse errors for a
	// whole file always record and, when FailFast, stop).
	FailFast bool
	// Filename is a diagnostic hint for the root upload name.
	Filename string
	// Data is the file or zip bytes. Mutually exclusive with Path.
	Data []byte
	// Path is a filesystem file or directory (blctl). Mutually exclusive with Data.
	Path string
	// SkipImportActivity suppresses content.import.* rows. The async job path
	// records its own activity against the job id.
	SkipImportActivity bool
}

// ImportReport is the synchronous result shape (and the activity delta source).
type ImportReport struct {
	DryRun            bool
	Format            string
	ProceduresCreated int
	ProceduresUpdated int
	NotesCreated      int
	NotesUpdated      int
	DetectionsCreated int
	DetectionsUpdated int
	Warnings          []ImportIssue
	Errors            []ImportIssue
}

// ImportIssue is one per-path warning or error.
type ImportIssue struct {
	Path    string
	Message string
}

// TotalWritten returns created+updated across all families.
func (r ImportReport) TotalWritten() int {
	return r.ProceduresCreated + r.ProceduresUpdated +
		r.NotesCreated + r.NotesUpdated +
		r.DetectionsCreated + r.DetectionsUpdated
}

// Summary is a one-line human message for job rows / CLI.
func (r ImportReport) Summary() string {
	return fmt.Sprintf(
		"format=%s dryRun=%v procedures=+%d/~%d notes=+%d/~%d detections=+%d/~%d warnings=%d errors=%d",
		r.Format, r.DryRun,
		r.ProceduresCreated, r.ProceduresUpdated,
		r.NotesCreated, r.NotesUpdated,
		r.DetectionsCreated, r.DetectionsUpdated,
		len(r.Warnings), len(r.Errors),
	)
}

// Import runs a v1/custom import against the custom source.
//
// Authorization is not decided here. Callers hold content.manage (or blctl).
// Dry-run never writes and never records activity. A successful write path
// records content.import.finished; a hard failure (unreadable input) records
// content.import.failed when not dry-run.
func (c *Custom) Import(ctx context.Context, actor authn.Subject, req ImportRequest) (ImportReport, error) {
	format := v1import.Format(strings.TrimSpace(req.Format))
	if format == "" {
		format = v1import.FormatAuto
	}

	var (
		bundle v1import.Bundle
		err    error
	)
	switch {
	case req.Path != "":
		bundle, err = v1import.ParsePath(req.Path, format)
	case req.Data != nil:
		bundle, err = v1import.ParseBytes(req.Data, req.Filename, format)
	default:
		return ImportReport{}, apierr.Validation(apierr.Field("file", "is required"))
	}
	if err != nil {
		if !req.DryRun && !req.SkipImportActivity {
			c.recordImportFailed(ctx, actor, err.Error())
		}
		return ImportReport{}, apierr.Validation(apierr.Field("file", err.Error()))
	}

	report := ImportReport{
		DryRun: req.DryRun,
		Format: string(bundle.Format),
	}
	for _, w := range bundle.Warnings {
		report.Warnings = append(report.Warnings, ImportIssue{Path: w.Path, Message: w.Message})
	}
	for _, e := range bundle.Errors {
		report.Errors = append(report.Errors, ImportIssue{Path: e.Path, Message: e.Message})
		if req.FailFast {
			if !req.DryRun && !req.SkipImportActivity {
				c.recordImportFinished(ctx, actor, report)
			}
			return report, nil
		}
	}

	// Apply testcases → procedure templates.
	for _, tc := range bundle.Testcases {
		if req.DryRun {
			// Count as "would create" vs update by probing.
			if _, err := c.procedures.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, tc.ExternalID); err != nil {
				if errors.Is(err, apierr.ErrNotFound) {
					report.ProceduresCreated++
				} else {
					report.Errors = append(report.Errors, ImportIssue{Path: tc.SourcePath, Message: err.Error()})
					if req.FailFast {
						return report, nil
					}
				}
			} else {
				report.ProceduresUpdated++
			}
			continue
		}
		created, err := c.upsertProcedure(ctx, actor, tc)
		if err != nil {
			report.Errors = append(report.Errors, ImportIssue{Path: tc.SourcePath, Message: err.Error()})
			if req.FailFast {
				if !req.SkipImportActivity {
					c.recordImportFinished(ctx, actor, report)
				}
				return report, nil
			}
			continue
		}
		if created {
			report.ProceduresCreated++
		} else {
			report.ProceduresUpdated++
		}
	}

	// Apply notes.
	for _, n := range bundle.Notes {
		if req.DryRun {
			if _, err := c.notes.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, n.ExternalID); err != nil {
				if errors.Is(err, apierr.ErrNotFound) {
					report.NotesCreated++
				} else {
					report.Errors = append(report.Errors, ImportIssue{Path: n.SourcePath, Message: err.Error()})
					if req.FailFast {
						return report, nil
					}
				}
			} else {
				report.NotesUpdated++
			}
			continue
		}
		created, err := c.upsertNote(ctx, actor, n)
		if err != nil {
			report.Errors = append(report.Errors, ImportIssue{Path: n.SourcePath, Message: err.Error()})
			if req.FailFast {
				if !req.SkipImportActivity {
					c.recordImportFinished(ctx, actor, report)
				}
				return report, nil
			}
			continue
		}
		if created {
			report.NotesCreated++
		} else {
			report.NotesUpdated++
		}
	}

	// Apply detections (custom export only).
	for _, d := range bundle.Detections {
		if req.DryRun {
			if _, err := c.detections.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, d.ExternalID); err != nil {
				if errors.Is(err, apierr.ErrNotFound) {
					report.DetectionsCreated++
				} else {
					report.Errors = append(report.Errors, ImportIssue{Path: d.SourcePath, Message: err.Error()})
					if req.FailFast {
						return report, nil
					}
				}
			} else {
				report.DetectionsUpdated++
			}
			continue
		}
		created, err := c.upsertDetection(ctx, actor, d)
		if err != nil {
			report.Errors = append(report.Errors, ImportIssue{Path: d.SourcePath, Message: err.Error()})
			if req.FailFast {
				if !req.SkipImportActivity {
					c.recordImportFinished(ctx, actor, report)
				}
				return report, nil
			}
			continue
		}
		if created {
			report.DetectionsCreated++
		} else {
			report.DetectionsUpdated++
		}
	}

	if !req.DryRun && !req.SkipImportActivity {
		c.recordImportFinished(ctx, actor, report)
	}
	return report, nil
}

func (c *Custom) upsertProcedure(ctx context.Context, actor authn.Subject, tc v1import.Testcase) (created bool, err error) {
	existing, err := c.procedures.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, tc.ExternalID)
	if err != nil && !errors.Is(err, apierr.ErrNotFound) {
		return false, err
	}
	if errors.Is(err, apierr.ErrNotFound) {
		in := ProcedureCreate{
			ExternalID:             tc.ExternalID,
			Name:                   tc.Name,
			Description:            tc.Description,
			Platforms:              tc.Platforms,
			Executor:               tc.Executor,
			ElevationRequired:      tc.ElevationRequired,
			Command:                tc.Command,
			Cleanup:                tc.Cleanup,
			TechniqueExternalIDs:   tc.TechniqueExternalIDs,
			DependencyExecutorName: tc.DependencyExecutorName,
			Dependencies:           tc.Dependencies,
		}
		if len(tc.InputArgsJSON) > 0 {
			in.InputArgs = json.RawMessage(tc.InputArgsJSON)
		}
		_, err := c.CreateProcedure(ctx, actor, in)
		return true, err
	}

	name := tc.Name
	desc := tc.Description
	cmd := tc.Command
	cleanup := tc.Cleanup
	exec := tc.Executor
	elev := tc.ElevationRequired
	depExec := tc.DependencyExecutorName
	deps := tc.Dependencies
	techs := tc.TechniqueExternalIDs
	plats := tc.Platforms
	edit := ProcedureEdit{
		Name:                   &name,
		Description:            &desc,
		Command:                &cmd,
		Cleanup:                &cleanup,
		Executor:               &exec,
		ElevationRequired:      &elev,
		DependencyExecutorName: &depExec,
		Dependencies:           &deps,
		TechniqueExternalIDs:   &techs,
		Platforms:              &plats,
	}
	if len(tc.InputArgsJSON) > 0 {
		raw := json.RawMessage(tc.InputArgsJSON)
		edit.InputArgs = &raw
	}
	_, err = c.UpdateProcedure(ctx, actor, existing.ID, edit)
	return false, err
}

func (c *Custom) upsertNote(ctx context.Context, actor authn.Subject, n v1import.Note) (created bool, err error) {
	existing, err := c.notes.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, n.ExternalID)
	if err != nil && !errors.Is(err, apierr.ErrNotFound) {
		return false, err
	}
	if errors.Is(err, apierr.ErrNotFound) {
		_, err := c.CreateNote(ctx, actor, NoteCreate{
			ExternalID:          n.ExternalID,
			Title:               n.Title,
			BodyMarkdown:        n.BodyMarkdown,
			Tags:                n.Tags,
			TechniqueExternalID: n.TechniqueExternalID,
		})
		return true, err
	}
	title := n.Title
	body := n.BodyMarkdown
	tags := n.Tags
	tech := n.TechniqueExternalID
	_, err = c.UpdateNote(ctx, actor, existing.ID, NoteEdit{
		Title:               &title,
		BodyMarkdown:        &body,
		Tags:                &tags,
		TechniqueExternalID: &tech,
	})
	return false, err
}

func (c *Custom) upsertDetection(ctx context.Context, actor authn.Subject, d v1import.Detection) (created bool, err error) {
	if strings.TrimSpace(d.RuleYAML) == "" {
		return false, fmt.Errorf("detection ruleYaml must not be empty")
	}
	existing, err := c.detections.ByExternalID(ctx, storecontent.SourceIDCustom, storecontent.VersionCurrent, d.ExternalID)
	if err != nil && !errors.Is(err, apierr.ErrNotFound) {
		return false, err
	}
	var ls json.RawMessage
	if len(d.LogsourceJSON) > 0 {
		ls = json.RawMessage(d.LogsourceJSON)
	}
	if errors.Is(err, apierr.ErrNotFound) {
		_, err := c.CreateDetection(ctx, actor, DetectionCreate{
			ExternalID:           d.ExternalID,
			Name:                 d.Name,
			Description:          d.Description,
			TechniqueExternalIDs: d.TechniqueExternalIDs,
			Level:                d.Level,
			RuleStatus:           d.RuleStatus,
			Logsource:            ls,
			RuleYAML:             d.RuleYAML,
		})
		return true, err
	}
	name := d.Name
	desc := d.Description
	techs := d.TechniqueExternalIDs
	level := d.Level
	status := d.RuleStatus
	rule := d.RuleYAML
	edit := DetectionEdit{
		Name:                 &name,
		Description:          &desc,
		TechniqueExternalIDs: &techs,
		Level:                &level,
		RuleStatus:           &status,
		RuleYAML:             &rule,
	}
	if ls != nil {
		edit.Logsource = &ls
	}
	_, err = c.UpdateDetection(ctx, actor, existing.ID, edit)
	return false, err
}

func (c *Custom) recordImportFinished(ctx context.Context, actor authn.Subject, report ImportReport) {
	if c.activity == nil {
		return
	}
	// Best-effort: import rows already committed; a log failure must not roll them back.
	if err := c.activity.RecordAlone(ctx, events.Entry{
		ActorID:    actor.UserID,
		Verb:       events.VerbContentImportFinished,
		ObjectType: events.ObjectContentSource,
		ObjectID:   storecontent.SourceIDCustom,
		Delta: events.Delta(map[string]any{
			"format":             report.Format,
			"dry_run":            report.DryRun,
			"procedures_created": report.ProceduresCreated,
			"procedures_updated": report.ProceduresUpdated,
			"notes_created":      report.NotesCreated,
			"notes_updated":      report.NotesUpdated,
			"detections_created": report.DetectionsCreated,
			"detections_updated": report.DetectionsUpdated,
			"warnings":           len(report.Warnings),
			"errors":             len(report.Errors),
		}),
	}); err != nil {
		slog.Default().ErrorContext(ctx, "content import activity write failed", "error", err, "verb", events.VerbContentImportFinished)
	}
}

func (c *Custom) recordImportFailed(ctx context.Context, actor authn.Subject, msg string) {
	if c.activity == nil {
		return
	}
	if err := c.activity.RecordAlone(ctx, events.Entry{
		ActorID:    actor.UserID,
		Verb:       events.VerbContentImportFailed,
		ObjectType: events.ObjectContentSource,
		ObjectID:   storecontent.SourceIDCustom,
		Delta:      events.Delta(map[string]any{"error": msg}),
	}); err != nil {
		slog.Default().ErrorContext(ctx, "content import activity write failed", "error", err, "verb", events.VerbContentImportFailed)
	}
}

// ReadImportMultipart walks a multipart body for `file` + optional `format`,
// spooling the file under the content data root via the runner. Returns the
// spooled path, sha, size, format string, and original filename hint.
func (r *Runner) ReadImportMultipart(ctx context.Context, mr *multipart.Reader) (path, sha, format, filename string, size int64, err error) {
	if mr == nil {
		return "", "", "", "", 0, apierr.Validation(apierr.Field("file", "is required"))
	}
	format = string(v1import.FormatAuto)
	var (
		gotFile             bool
		spoolPath, spoolSHA string
		spoolSize           int64
		hint                string
	)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", "", "", "", 0, apierr.Validation(apierr.Field("file", "invalid multipart body"))
		}
		name := part.FormName()
		switch name {
		case "file":
			hint = part.FileName()
			spoolPath, spoolSHA, spoolSize, err = r.SpoolUpload(ctx, part, hint)
			_ = part.Close()
			if err != nil {
				return "", "", "", "", 0, mapUploadErr(err)
			}
			gotFile = true
		case "format":
			raw, err := io.ReadAll(io.LimitReader(part, 64))
			_ = part.Close()
			if err != nil {
				return "", "", "", "", 0, apierr.Validation(apierr.Field("format", "could not be read"))
			}
			f := strings.TrimSpace(string(raw))
			if f != "" {
				format = f
			}
		default:
			if _, copyErr := io.Copy(io.Discard, io.LimitReader(part, 1<<20)); copyErr != nil {
				_ = part.Close()
				return "", "", "", "", 0, apierr.Validation(apierr.Field("file", "invalid multipart body"))
			}
			_ = part.Close()
		}
	}
	if !gotFile {
		return "", "", "", "", 0, apierr.Validation(apierr.Field("file", "is required"))
	}
	return spoolPath, spoolSHA, format, filepath.Base(hint), spoolSize, nil
}

// StartV1ImportRequest enqueues an async v1_import job over a pre-spooled file.
type StartV1ImportRequest struct {
	BundlePath   string
	BundleSHA256 string
	Format       string
	FailFast     bool
	Filename     string
}

// StartV1Import enqueues a v1_import job. The spooled file is removed when the
// job reaches any terminal status. Requires a Custom service on the runner.
func (r *Runner) StartV1Import(ctx context.Context, actor authn.Subject, req StartV1ImportRequest) (storecontent.Job, error) {
	if r.custom == nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, errors.New("content: runner: custom service is required for v1_import")
	}
	if req.BundlePath == "" {
		return storecontent.Job{}, apierr.Validation(apierr.Field("file", "is required"))
	}
	if err := r.requirePathUnderRoot(req.BundlePath); err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}

	// Reuse StartSync's slot + job row machinery against the custom source,
	// but skip the adapter requirement via a dedicated path.
	if active, ok, err := r.jobs.FindActive(ctx); err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	} else if ok {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("a content job is already active (jobId: %s)", active.ID))
	}

	checkpoint := map[string]any{
		"bundle_path":    req.BundlePath,
		"skip_fetch":     true,
		"cleanup_upload": true,
		"v1_import":      true,
		"format":         req.Format,
		"fail_fast":      req.FailFast,
		"filename":       req.Filename,
	}
	if req.BundleSHA256 != "" {
		checkpoint["bundle_sha256"] = req.BundleSHA256
	}
	rawCP, err := json.Marshal(checkpoint)
	if err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, fmt.Errorf("content: encode job checkpoint: %w", err)
	}

	job, err := r.jobs.Create(ctx, storecontent.NewJob{
		SourceID:  storecontent.SourceIDCustom,
		Version:   storecontent.VersionCurrent,
		Kind:      storecontent.JobKindV1Import,
		CreatedBy: actor.UserID,
	})
	if err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}
	job, err = r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
		Status:     job.Status,
		Phase:      job.Phase,
		Message:    job.Message,
		Checkpoint: rawCP,
	})
	if err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}

	src, err := r.sources.ByID(ctx, storecontent.SourceIDCustom)
	if err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}
	if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusSyncing, src.ItemCount, "", time.Time{}); err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}

	r.recordAlone(ctx, events.Entry{
		ActorID:    actor.UserID,
		Verb:       events.VerbContentSyncStarted,
		ObjectType: events.ObjectContentSyncJob,
		ObjectID:   job.ID,
		Delta: events.Delta(map[string]any{
			"source_id": src.ID,
			"kind":      string(storecontent.JobKindV1Import),
			"format":    req.Format,
		}),
	})
	r.nudge()
	return job, nil
}
