package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// executeV1Import runs a spooled upload through [Custom.Import].
func (r *Runner) executeV1Import(parent context.Context, job storecontent.Job) error {
	if r.custom == nil {
		return r.failJob(parent, job, storecontent.VersionCurrent, errors.New("v1_import: custom service is not configured"))
	}

	src, err := r.sources.ByID(parent, job.SourceID)
	if err != nil {
		// Fall back to the well-known custom source id for status updates.
		src = storecontent.Source{ID: storecontent.SourceIDCustom, ItemCount: 0}
	}

	started := time.Now().UTC()
	job, err = r.jobs.Update(parent, job.ID, storecontent.JobUpdate{
		Status:    storecontent.JobStatusRunning,
		Phase:     PhaseParse,
		Message:   "importing v1 custom content",
		StartedAt: started,
	})
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(parent, r.jobTimeout)
	defer cancel()
	r.mu.Lock()
	r.runCancel = cancel
	r.runJobID = job.ID
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.runCancel = nil
		r.runJobID = ""
		r.mu.Unlock()
	}()

	cp := parseV1Checkpoint(job.Checkpoint)
	if cp.BundlePath == "" {
		return r.failJob(parent, job, storecontent.VersionCurrent, errors.New("v1_import: missing bundle_path in checkpoint"))
	}
	raw, err := os.ReadFile(cp.BundlePath)
	if err != nil {
		return r.failJob(parent, job, storecontent.VersionCurrent, fmt.Errorf("v1_import: read upload: %w", err))
	}

	prog := &jobProgress{runner: r, jobID: job.ID, status: storecontent.JobStatusRunning}
	prog.Report(runCtx, PhaseParse, 0, 1, "parsing")

	actor := authn.Subject{UserID: job.CreatedBy}
	report, runErr := r.custom.Import(runCtx, actor, ImportRequest{
		Format:             cp.Format,
		FailFast:           cp.FailFast,
		Filename:           cp.Filename,
		Data:               raw,
		SkipImportActivity: true,
	})

	current, err := r.jobs.ByID(parent, job.ID)
	if err != nil {
		return err
	}
	finished := time.Now().UTC()

	switch {
	case runErr == nil:
		prog.Report(runCtx, PhaseApply, 1, 1, report.Summary())
		// succeedJob would re-read version item counts from ATT&CK-style
		// snapshots; for custom import we set the job message ourselves and
		// refresh the custom source item count from the report.
		count := int64(report.TotalWritten())
		// Prefer absolute library size when we can list — but TotalWritten is
		// the delta for this run, which is what operators care about in the
		// job row. Source item_count is left to a cheap recount below.
		if _, err := r.jobs.Update(parent, current.ID, storecontent.JobUpdate{
			Status:          storecontent.JobStatusSucceeded,
			Phase:           PhaseFinalize,
			ProgressCurrent: 1,
			ProgressTotal:   1,
			Message:         report.Summary(),
			FinishedAt:      finished,
		}); err != nil {
			return err
		}
		itemCount := src.ItemCount
		if n, err := r.countCustomItems(parent); err == nil {
			itemCount = n
		} else if count > 0 {
			itemCount = src.ItemCount + count
		}
		if err := r.sources.SetSyncState(parent, storecontent.SourceIDCustom, storecontent.SourceStatusIdle, itemCount, "", finished); err != nil {
			return err
		}
		r.recordAlone(parent, events.Entry{
			ActorID:    job.CreatedBy,
			Verb:       events.VerbContentImportFinished,
			ObjectType: events.ObjectContentSyncJob,
			ObjectID:   job.ID,
			Delta: events.Delta(map[string]any{
				"source_id":          storecontent.SourceIDCustom,
				"format":             report.Format,
				"procedures_created": report.ProceduresCreated,
				"procedures_updated": report.ProceduresUpdated,
				"notes_created":      report.NotesCreated,
				"notes_updated":      report.NotesUpdated,
				"detections_created": report.DetectionsCreated,
				"detections_updated": report.DetectionsUpdated,
				"warnings":           len(report.Warnings),
				"errors":             len(report.Errors),
			}),
		})
		r.publish(ProgressEvent{
			JobID: job.ID, Phase: PhaseFinalize, Current: 1, Total: 1,
			Message: report.Summary(), Status: storecontent.JobStatusSucceeded,
		})
		return nil
	case errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return r.failJob(parent, current, storecontent.VersionCurrent, fmt.Errorf("job exceeded timeout of %s", r.jobTimeout))
	case errors.Is(runErr, context.Canceled) || current.Status == storecontent.JobStatusCancelling:
		return r.cancelJob(parent, current, src, finished)
	default:
		return r.failJob(parent, current, storecontent.VersionCurrent, runErr)
	}
}

type v1Checkpoint struct {
	BundlePath string
	Format     string
	FailFast   bool
	Filename   string
}

func parseV1Checkpoint(raw json.RawMessage) v1Checkpoint {
	var cp struct {
		BundlePath string `json:"bundle_path"`
		Format     string `json:"format"`
		FailFast   bool   `json:"fail_fast"`
		Filename   string `json:"filename"`
	}
	if err := json.Unmarshal(raw, &cp); err != nil {
		return v1Checkpoint{}
	}
	return v1Checkpoint{
		BundlePath: cp.BundlePath,
		Format:     cp.Format,
		FailFast:   cp.FailFast,
		Filename:   cp.Filename,
	}
}

func (r *Runner) countCustomItems(ctx context.Context) (int64, error) {
	if r.custom == nil {
		return 0, errors.New("no custom service")
	}
	procs, err := r.custom.ListProcedures(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		return 0, err
	}
	notes, err := r.custom.ListNotes(ctx, storecontent.NoteListFilter{})
	if err != nil {
		return 0, err
	}
	dets, err := r.custom.ListDetections(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		return 0, err
	}
	return int64(len(procs) + len(notes) + len(dets)), nil
}
