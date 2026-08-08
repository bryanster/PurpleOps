package httpapi

import (
	"context"
	"os"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// ImportCustomContent accepts a v1 custom content upload (M2-012).
//
// Small bodies and dry-runs run synchronously (200 + report). Larger uploads
// enqueue a v1_import job (202). content.manage is enforced by the spec.
func (h *handlers) ImportCustomContent(ctx context.Context, request gen.ImportCustomContentRequestObject) (gen.ImportCustomContentResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if h.custom == nil {
		return nil, errRunnerMissing
	}
	if h.runner == nil {
		return nil, errRunnerMissing
	}

	dryRun := request.Params.DryRun != nil && *request.Params.DryRun
	failFast := request.Params.FailFast != nil && *request.Params.FailFast

	path, sha, format, filename, size, err := h.runner.ReadImportMultipart(ctx, request.Body)
	if err != nil {
		return nil, err
	}
	// Always remove the spool for the synchronous path; the async path keeps
	// it until the job finishes (StartV1Import owns cleanup).
	cleanupSync := true
	defer func() {
		if cleanupSync && path != "" {
			_ = os.Remove(path)
		}
	}()

	// Async only when not dry-run and over the sync threshold.
	if !dryRun && size > content.SyncImportMaxBytes {
		cleanupSync = false
		job, err := h.runner.StartV1Import(ctx, subject, content.StartV1ImportRequest{
			BundlePath:   path,
			BundleSHA256: sha,
			Format:       format,
			FailFast:     failFast,
			Filename:     filename,
		})
		if err != nil {
			return nil, err
		}
		wire, err := contentSyncJob(job)
		if err != nil {
			return nil, err
		}
		return gen.ImportCustomContent202JSONResponse(wire), nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report, err := h.custom.Import(ctx, subject, content.ImportRequest{
		Format:   format,
		DryRun:   dryRun,
		FailFast: failFast,
		Filename: filename,
		Data:     raw,
	})
	if err != nil {
		return nil, err
	}
	return gen.ImportCustomContent200JSONResponse(contentImportReport(report)), nil
}

func contentImportReport(r content.ImportReport) gen.ContentImportReport {
	out := gen.ContentImportReport{
		DryRun:            r.DryRun,
		Format:            r.Format,
		ProceduresCreated: r.ProceduresCreated,
		ProceduresUpdated: r.ProceduresUpdated,
		NotesCreated:      r.NotesCreated,
		NotesUpdated:      r.NotesUpdated,
		DetectionsCreated: r.DetectionsCreated,
		DetectionsUpdated: r.DetectionsUpdated,
		Warnings:          make([]gen.ContentImportIssue, 0, len(r.Warnings)),
		Errors:            make([]gen.ContentImportIssue, 0, len(r.Errors)),
	}
	for _, w := range r.Warnings {
		out.Warnings = append(out.Warnings, gen.ContentImportIssue{Path: w.Path, Message: w.Message})
	}
	for _, e := range r.Errors {
		out.Errors = append(out.Errors, gen.ContentImportIssue{Path: e.Path, Message: e.Message})
	}
	return out
}
