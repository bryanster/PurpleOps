package httpapi

import (
	"context"
	"errors"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// errRunnerMissing is a programming error: the server was built without a
// content runner. It must never reach a client on a correctly wired process.
var errRunnerMissing = apierr.Internal(errors.New("httpapi: content runner is not configured"))

// Content job endpoints (M2-003 / M2-005).
//
// Thin translators over content.Runner. Authorization is decided by
// api/openapi.yaml (content.sync for start/cancel/list/bundle/reprocess,
// content.read for get).

// StartContentSourceSync enqueues a sync job for a source.
func (h *handlers) StartContentSourceSync(ctx context.Context, request gen.StartContentSourceSyncRequestObject) (gen.StartContentSourceSyncResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	var version string
	if request.Body != nil && request.Body.Version != nil {
		version = *request.Body.Version
	}
	job, err := h.runner.StartSync(ctx, subject, content.StartSyncRequest{
		SourceID: request.SourceId.String(),
		Version:  version,
	})
	if err != nil {
		return nil, err
	}
	wire, err := contentSyncJob(job)
	if err != nil {
		return nil, err
	}
	return gen.StartContentSourceSync202JSONResponse(wire), nil
}

// UploadContentSourceBundle accepts an offline release archive and enqueues a
// bundle_import job (M2-005). The multipart body is streamed to disk under the
// content data root; oversized uploads fail before a job row exists.
func (h *handlers) UploadContentSourceBundle(ctx context.Context, request gen.UploadContentSourceBundleRequestObject) (gen.UploadContentSourceBundleResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	path, sha, version, _, err := h.runner.ReadBundleMultipart(ctx, request.Body)
	if err != nil {
		return nil, err
	}
	job, err := h.runner.StartBundleImport(ctx, subject, content.StartBundleImportRequest{
		SourceID:     request.SourceId.String(),
		Version:      version,
		BundlePath:   path,
		BundleSHA256: sha,
	})
	if err != nil {
		return nil, err
	}
	wire, err := contentSyncJob(job)
	if err != nil {
		return nil, err
	}
	return gen.UploadContentSourceBundle202JSONResponse(wire), nil
}

// ReprocessContentSource enqueues a reprocess job from the last raw snapshot
// (M2-005). No network I/O.
func (h *handlers) ReprocessContentSource(ctx context.Context, request gen.ReprocessContentSourceRequestObject) (gen.ReprocessContentSourceResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	var version string
	if request.Body != nil && request.Body.Version != nil {
		version = *request.Body.Version
	}
	job, err := h.runner.StartReprocess(ctx, subject, content.StartReprocessRequest{
		SourceID: request.SourceId.String(),
		Version:  version,
	})
	if err != nil {
		return nil, err
	}
	wire, err := contentSyncJob(job)
	if err != nil {
		return nil, err
	}
	return gen.ReprocessContentSource202JSONResponse(wire), nil
}

// ListContentJobs returns jobs newest first (admin).
func (h *handlers) ListContentJobs(ctx context.Context, request gen.ListContentJobsRequestObject) (gen.ListContentJobsResponseObject, error) {
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	f := storecontent.ListFilter{}
	if request.Params.Status != nil {
		f.Status = storecontent.JobStatus(*request.Params.Status)
	}
	if request.Params.SourceId != nil {
		f.SourceID = request.Params.SourceId.String()
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	jobs, err := h.runner.ListJobs(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]gen.ContentSyncJob, 0, len(jobs))
	for _, j := range jobs {
		wire, err := contentSyncJob(j)
		if err != nil {
			return nil, err
		}
		items = append(items, wire)
	}
	return gen.ListContentJobs200JSONResponse{Items: items}, nil
}

// GetContentJob returns one job.
func (h *handlers) GetContentJob(ctx context.Context, request gen.GetContentJobRequestObject) (gen.GetContentJobResponseObject, error) {
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	job, err := h.runner.GetJob(ctx, request.JobId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentSyncJob(job)
	if err != nil {
		return nil, err
	}
	return gen.GetContentJob200JSONResponse(wire), nil
}

// CancelContentJob requests cancellation.
func (h *handlers) CancelContentJob(ctx context.Context, request gen.CancelContentJobRequestObject) (gen.CancelContentJobResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if h.runner == nil {
		return nil, errRunnerMissing
	}
	job, err := h.runner.Cancel(ctx, subject, request.JobId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentSyncJob(job)
	if err != nil {
		return nil, err
	}
	return gen.CancelContentJob200JSONResponse(wire), nil
}

func contentSyncJob(j storecontent.Job) (gen.ContentSyncJob, error) {
	id, err := parseUUID(j.ID)
	if err != nil {
		return gen.ContentSyncJob{}, err
	}
	sourceID, err := parseUUID(j.SourceID)
	if err != nil {
		return gen.ContentSyncJob{}, err
	}
	out := gen.ContentSyncJob{
		Id:              id,
		SourceId:        sourceID,
		Kind:            gen.ContentSyncJobKind(j.Kind),
		Status:          gen.ContentSyncJobStatus(j.Status),
		Phase:           j.Phase,
		ProgressCurrent: j.ProgressCurrent,
		ProgressTotal:   j.ProgressTotal,
		Message:         j.Message,
		Error:           j.Error,
		CreatedBy:       j.CreatedBy,
		CreatedAt:       j.CreatedAt,
	}
	if j.Version != "" {
		v := j.Version
		out.Version = &v
	}
	if !j.StartedAt.IsZero() {
		t := j.StartedAt
		out.StartedAt = &t
	}
	if !j.FinishedAt.IsZero() {
		t := j.FinishedAt
		out.FinishedAt = &t
	}
	return out, nil
}
