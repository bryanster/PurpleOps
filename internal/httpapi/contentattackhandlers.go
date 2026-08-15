package httpapi

import (
	"context"

	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// ATT&CK version catalog & pin surface (M2-007). content.read for GETs;
// content.manage for DELETE. Domain logic lives in internal/content/attackpin.

func (h *handlers) ListContentAttackVersions(ctx context.Context, _ gen.ListContentAttackVersionsRequestObject) (gen.ListContentAttackVersionsResponseObject, error) {
	items, err := h.attackpin.ListVersions(ctx)
	if err != nil {
		return nil, attackpin.MapError(err)
	}
	out := make([]gen.ContentAttackVersion, 0, len(items))
	for _, v := range items {
		out = append(out, contentAttackVersion(v))
	}
	return gen.ListContentAttackVersions200JSONResponse{Items: out}, nil
}

// ListContentAttackReleases answers what upstream offers next to what is
// installed. An unreachable upstream is part of the answer rather than an
// error — see the operation's description, and attackpin.ListReleases.
func (h *handlers) ListContentAttackReleases(ctx context.Context, _ gen.ListContentAttackReleasesRequestObject) (gen.ListContentAttackReleasesResponseObject, error) {
	catalog, err := h.attackpin.ListReleases(ctx)
	if err != nil {
		return nil, attackpin.MapError(err)
	}
	items := make([]gen.ContentAttackRelease, 0, len(catalog.Items))
	for _, r := range catalog.Items {
		items = append(items, contentAttackRelease(r))
	}
	out := gen.ContentAttackReleaseList{
		Items:         items,
		Reachable:     catalog.Reachable,
		SourceEnabled: catalog.SourceEnabled,
	}
	if catalog.Unreachable != "" {
		out.Unreachable.Set(catalog.Unreachable)
	}
	return gen.ListContentAttackReleases200JSONResponse(out), nil
}

func (h *handlers) GetContentAttackVersion(ctx context.Context, request gen.GetContentAttackVersionRequestObject) (gen.GetContentAttackVersionResponseObject, error) {
	d, err := h.attackpin.ResolveDetail(ctx, request.Version)
	if err != nil {
		return nil, attackpin.MapError(err)
	}
	return gen.GetContentAttackVersion200JSONResponse(contentAttackVersionDetail(d)), nil
}

func (h *handlers) DeleteContentAttackVersion(ctx context.Context, request gen.DeleteContentAttackVersionRequestObject) (gen.DeleteContentAttackVersionResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.attackpin.DeleteVersion(ctx, actor, request.Version); err != nil {
		return nil, attackpin.MapError(err)
	}
	return gen.DeleteContentAttackVersion204Response{}, nil
}

//nolint:staticcheck // ST1003: operationId/ExternalId spelling is fixed by oapi-codegen from OpenAPI.
func (h *handlers) GetContentAttackTechniqueByExternalId(ctx context.Context, request gen.GetContentAttackTechniqueByExternalIdRequestObject) (gen.GetContentAttackTechniqueByExternalIdResponseObject, error) {
	t, err := h.attackpin.ResolveTechnique(ctx, request.Version, request.ExternalId)
	if err != nil {
		return nil, attackpin.MapError(err)
	}
	wire, err := contentTechnique(t)
	if err != nil {
		return nil, err
	}
	return gen.GetContentAttackTechniqueByExternalId200JSONResponse(wire), nil
}

func contentAttackVersion(v attackpin.VersionInfo) gen.ContentAttackVersion {
	out := gen.ContentAttackVersion{
		Version:       v.Version,
		Status:        gen.ContentSourceVersionStatus(v.Status),
		ItemCount:     v.ItemCount,
		SourceEnabled: v.SourceEnabled,
	}
	if !v.SyncedAt.IsZero() {
		t := v.SyncedAt
		out.SyncedAt = &t
	}
	return out
}

func contentAttackRelease(r attackpin.ReleaseInfo) gen.ContentAttackRelease {
	out := gen.ContentAttackRelease{
		Version:   r.Version,
		Installed: r.Installed,
		Latest:    r.Latest,
	}
	if !r.Released.IsZero() {
		at := r.Released
		out.Released = &at
	}
	if r.Status != "" {
		status := string(r.Status)
		out.Status = &status
	}
	return out
}

func contentAttackVersionDetail(d attackpin.VersionDetail) gen.ContentAttackVersionDetail {
	base := contentAttackVersion(d.VersionInfo)
	return gen.ContentAttackVersionDetail{
		Version:       base.Version,
		Status:        base.Status,
		ItemCount:     base.ItemCount,
		SyncedAt:      base.SyncedAt,
		SourceEnabled: base.SourceEnabled,
		Counts: gen.ContentAttackVersionCounts{
			Tactics:     d.Counts.Tactics,
			Techniques:  d.Counts.Techniques,
			Mitigations: d.Counts.Mitigations,
			Groups:      d.Counts.Groups,
			Software:    d.Counts.Software,
			DataSources: d.Counts.DataSources,
		},
	}
}
