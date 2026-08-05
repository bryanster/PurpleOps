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

// Content job endpoints (M2-003).
//
// Thin translators over content.Runner. Authorization is decided by
// api/openapi.yaml (content.sync for start/cancel/list, content.read for get).

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
