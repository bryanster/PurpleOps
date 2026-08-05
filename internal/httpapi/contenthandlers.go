package httpapi

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Content source registry endpoints (M2-002).
//
// Like every other handler in this package they translate and nothing else:
// enable/disable/delete semantics and activity rows live in internal/content.
// Who may call them is decided by api/openapi.yaml → content.read /
// content.manage and the authorization middleware (M1-013).

// ListContentSources returns every source matching the optional filters.
func (h *handlers) ListContentSources(ctx context.Context, request gen.ListContentSourcesRequestObject) (gen.ListContentSourcesResponseObject, error) {
	filter := content.SourceFilter{}
	if request.Params.Kind != nil {
		filter.Kind = string(*request.Params.Kind)
	}
	if request.Params.Enabled != nil {
		filter.Enabled = request.Params.Enabled
	}

	items, err := h.content.ListSources(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentSource, 0, len(items))
	for _, s := range items {
		wire, err := contentSource(s)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentSources200JSONResponse{Items: out}, nil
}

// GetContentSource returns one source plus its most recent job summary.
func (h *handlers) GetContentSource(ctx context.Context, request gen.GetContentSourceRequestObject) (gen.GetContentSourceResponseObject, error) {
	detail, err := h.content.GetSource(ctx, request.SourceId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentSourceDetail(detail)
	if err != nil {
		return nil, err
	}
	return gen.GetContentSource200JSONResponse(wire), nil
}

// UpdateContentSource patches name/url/ref.
func (h *handlers) UpdateContentSource(ctx context.Context, request gen.UpdateContentSourceRequestObject) (gen.UpdateContentSourceResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: update content source: missing body")
	}

	src, err := h.content.UpdateSource(ctx, subject, request.SourceId.String(), content.SourceEdit{
		Name: request.Body.Name,
		URL:  request.Body.Url,
		Ref:  request.Body.Ref,
	})
	if err != nil {
		return nil, err
	}
	wire, err := contentSource(src)
	if err != nil {
		return nil, err
	}
	return gen.UpdateContentSource200JSONResponse(wire), nil
}

// EnableContentSource sets enabled=true.
func (h *handlers) EnableContentSource(ctx context.Context, request gen.EnableContentSourceRequestObject) (gen.EnableContentSourceResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	src, err := h.content.EnableSource(ctx, subject, request.SourceId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentSource(src)
	if err != nil {
		return nil, err
	}
	return gen.EnableContentSource200JSONResponse(wire), nil
}

// DisableContentSource sets enabled=false.
func (h *handlers) DisableContentSource(ctx context.Context, request gen.DisableContentSourceRequestObject) (gen.DisableContentSourceResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	src, err := h.content.DisableSource(ctx, subject, request.SourceId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentSource(src)
	if err != nil {
		return nil, err
	}
	return gen.DisableContentSource200JSONResponse(wire), nil
}

// DeleteContentSource hard-deletes a source and its content subtree.
func (h *handlers) DeleteContentSource(ctx context.Context, request gen.DeleteContentSourceRequestObject) (gen.DeleteContentSourceResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.content.DeleteSource(ctx, subject, request.SourceId.String()); err != nil {
		return nil, err
	}
	return gen.DeleteContentSource204Response{}, nil
}

// ListContentSourceVersions lists version snapshots under a source.
func (h *handlers) ListContentSourceVersions(ctx context.Context, request gen.ListContentSourceVersionsRequestObject) (gen.ListContentSourceVersionsResponseObject, error) {
	items, err := h.content.ListVersions(ctx, request.SourceId.String())
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentSourceVersion, 0, len(items))
	for _, v := range items {
		wire, err := contentSourceVersion(v)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentSourceVersions200JSONResponse{Items: out}, nil
}

func contentSource(s storecontent.Source) (gen.ContentSource, error) {
	id, err := parseUUID(s.ID)
	if err != nil {
		return gen.ContentSource{}, err
	}
	out := gen.ContentSource{
		Id:          id,
		Kind:        gen.ContentSourceKind(s.Kind),
		Name:        s.Name,
		Url:         s.URL,
		Ref:         s.Ref,
		Enabled:     s.Enabled,
		Status:      gen.ContentSourceStatus(s.Status),
		ItemCount:   s.ItemCount,
		Error:       s.Error,
		LicenseSpdx: s.LicenseSPDX,
		LicenseName: s.LicenseName,
		LicenseUrl:  s.LicenseURL,
		Attribution: s.Attribution,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
	if !s.LastSyncedAt.IsZero() {
		t := s.LastSyncedAt
		out.LastSyncedAt = &t
	}
	return out, nil
}

func contentSourceDetail(d content.SourceDetail) (gen.ContentSourceDetail, error) {
	base, err := contentSource(d.Source)
	if err != nil {
		return gen.ContentSourceDetail{}, err
	}
	out := gen.ContentSourceDetail{
		Id:           base.Id,
		Kind:         base.Kind,
		Name:         base.Name,
		Url:          base.Url,
		Ref:          base.Ref,
		Enabled:      base.Enabled,
		Status:       base.Status,
		LastSyncedAt: base.LastSyncedAt,
		ItemCount:    base.ItemCount,
		Error:        base.Error,
		LicenseSpdx:  base.LicenseSpdx,
		LicenseName:  base.LicenseName,
		LicenseUrl:   base.LicenseUrl,
		Attribution:  base.Attribution,
		CreatedAt:    base.CreatedAt,
		UpdatedAt:    base.UpdatedAt,
	}
	if d.LastJob != nil {
		summary, err := contentJobSummary(*d.LastJob)
		if err != nil {
			return gen.ContentSourceDetail{}, err
		}
		out.LastJob = &summary
	}
	return out, nil
}

func contentJobSummary(j storecontent.Job) (gen.ContentSyncJobSummary, error) {
	id, err := parseUUID(j.ID)
	if err != nil {
		return gen.ContentSyncJobSummary{}, err
	}
	out := gen.ContentSyncJobSummary{
		Id:        id,
		Kind:      gen.ContentSyncJobKind(j.Kind),
		Status:    gen.ContentSyncJobStatus(j.Status),
		CreatedAt: j.CreatedAt,
	}
	if j.Version != "" {
		v := j.Version
		out.Version = &v
	}
	if j.Phase != "" {
		p := j.Phase
		out.Phase = &p
	}
	if j.Message != "" {
		m := j.Message
		out.Message = &m
	}
	if j.Error != "" {
		e := j.Error
		out.Error = &e
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

func contentSourceVersion(v storecontent.SourceVersion) (gen.ContentSourceVersion, error) {
	id, err := parseUUID(v.ID)
	if err != nil {
		return gen.ContentSourceVersion{}, err
	}
	sourceID, err := parseUUID(v.SourceID)
	if err != nil {
		return gen.ContentSourceVersion{}, err
	}
	out := gen.ContentSourceVersion{
		Id:        id,
		SourceId:  sourceID,
		Version:   v.Version,
		Status:    gen.ContentSourceVersionStatus(v.Status),
		ItemCount: v.ItemCount,
		Error:     v.Error,
		RawSha256: v.RawSHA256,
		RawPath:   v.RawPath,
		RawBytes:  v.RawBytes,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
	if !v.SyncedAt.IsZero() {
		t := v.SyncedAt
		out.SyncedAt = &t
	}
	return out, nil
}
