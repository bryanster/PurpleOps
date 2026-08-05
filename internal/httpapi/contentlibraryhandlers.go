package httpapi

import (
	"context"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// ATT&CK library browse endpoints (M2-006). content.read; enabled sources only.

func (h *handlers) ListContentTechniques(ctx context.Context, request gen.ListContentTechniquesRequestObject) (gen.ListContentTechniquesResponseObject, error) {
	f := libraryFilter(request.Params.Version, request.Params.Q, request.Params.Limit)
	if request.Params.Tactic != nil {
		f.Tactic = *request.Params.Tactic
	}
	if request.Params.IsSubtechnique != nil {
		f.IsSubtechnique = request.Params.IsSubtechnique
	}
	items, err := h.objects.ListTechniques(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentTechnique, 0, len(items))
	for _, t := range items {
		wire, err := contentTechnique(t)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentTechniques200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentTechnique(ctx context.Context, request gen.GetContentTechniqueRequestObject) (gen.GetContentTechniqueResponseObject, error) {
	d, err := h.objects.TechniqueDetailByID(ctx, request.TechniqueId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentTechniqueDetail(d)
	if err != nil {
		return nil, err
	}
	return gen.GetContentTechnique200JSONResponse(wire), nil
}

func (h *handlers) ListContentTactics(ctx context.Context, request gen.ListContentTacticsRequestObject) (gen.ListContentTacticsResponseObject, error) {
	items, err := h.objects.ListTactics(ctx, libraryFilter(request.Params.Version, request.Params.Q, request.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentTactic, 0, len(items))
	for _, t := range items {
		wire, err := contentTactic(t)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentTactics200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentTactic(ctx context.Context, request gen.GetContentTacticRequestObject) (gen.GetContentTacticResponseObject, error) {
	t, err := h.objects.TacticByIDEnabled(ctx, request.TacticId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentTactic(t)
	if err != nil {
		return nil, err
	}
	return gen.GetContentTactic200JSONResponse(wire), nil
}

func (h *handlers) ListContentMitigations(ctx context.Context, request gen.ListContentMitigationsRequestObject) (gen.ListContentMitigationsResponseObject, error) {
	items, err := h.objects.ListMitigations(ctx, libraryFilter(request.Params.Version, request.Params.Q, request.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentMitigation, 0, len(items))
	for _, m := range items {
		wire, err := contentMitigation(m)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentMitigations200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentMitigation(ctx context.Context, request gen.GetContentMitigationRequestObject) (gen.GetContentMitigationResponseObject, error) {
	m, err := h.objects.MitigationByIDEnabled(ctx, request.MitigationId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentMitigation(m)
	if err != nil {
		return nil, err
	}
	return gen.GetContentMitigation200JSONResponse(wire), nil
}

func (h *handlers) ListContentGroups(ctx context.Context, request gen.ListContentGroupsRequestObject) (gen.ListContentGroupsResponseObject, error) {
	items, err := h.objects.ListGroups(ctx, libraryFilter(request.Params.Version, request.Params.Q, request.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentGroup, 0, len(items))
	for _, g := range items {
		wire, err := contentGroup(g)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentGroups200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentGroup(ctx context.Context, request gen.GetContentGroupRequestObject) (gen.GetContentGroupResponseObject, error) {
	g, err := h.objects.GroupByIDEnabled(ctx, request.GroupId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentGroup(g)
	if err != nil {
		return nil, err
	}
	return gen.GetContentGroup200JSONResponse(wire), nil
}

func (h *handlers) ListContentSoftware(ctx context.Context, request gen.ListContentSoftwareRequestObject) (gen.ListContentSoftwareResponseObject, error) {
	items, err := h.objects.ListSoftware(ctx, libraryFilter(request.Params.Version, request.Params.Q, request.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentSoftware, 0, len(items))
	for _, s := range items {
		wire, err := contentSoftware(s)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentSoftware200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentSoftware(ctx context.Context, request gen.GetContentSoftwareRequestObject) (gen.GetContentSoftwareResponseObject, error) {
	s, err := h.objects.SoftwareByIDEnabled(ctx, request.SoftwareId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentSoftware(s)
	if err != nil {
		return nil, err
	}
	return gen.GetContentSoftware200JSONResponse(wire), nil
}

func libraryFilter(version, q *string, limit *int) storecontent.ObjectListFilter {
	f := storecontent.ObjectListFilter{EnabledOnly: true}
	if version != nil {
		f.Version = *version
	}
	if q != nil {
		f.Q = *q
	}
	if limit != nil {
		f.Limit = *limit
	}
	return f
}

func contentTechnique(t storecontent.Technique) (gen.ContentTechnique, error) {
	id, err := parseUUID(t.ID)
	if err != nil {
		return gen.ContentTechnique{}, err
	}
	sourceID, err := parseUUID(t.SourceID)
	if err != nil {
		return gen.ContentTechnique{}, err
	}
	return gen.ContentTechnique{
		Id:               id,
		SourceId:         sourceID,
		Version:          t.Version,
		ExternalId:       t.ExternalID,
		Name:             t.Name,
		Description:      t.Description,
		IsSubtechnique:   t.IsSubtechnique,
		ParentExternalId: t.ParentExternalID,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}, nil
}

func contentTechniqueDetail(d storecontent.TechniqueDetail) (gen.ContentTechniqueDetail, error) {
	base, err := contentTechnique(d.Technique)
	if err != nil {
		return gen.ContentTechniqueDetail{}, err
	}
	tactics := d.Tactics
	if tactics == nil {
		tactics = []string{}
	}
	mits := d.Mitigations
	if mits == nil {
		mits = []string{}
	}
	return gen.ContentTechniqueDetail{
		Id:               base.Id,
		SourceId:         base.SourceId,
		Version:          base.Version,
		ExternalId:       base.ExternalId,
		Name:             base.Name,
		Description:      base.Description,
		IsSubtechnique:   base.IsSubtechnique,
		ParentExternalId: base.ParentExternalId,
		CreatedAt:        base.CreatedAt,
		UpdatedAt:        base.UpdatedAt,
		Tactics:          tactics,
		Mitigations:      mits,
	}, nil
}

func contentTactic(t storecontent.Tactic) (gen.ContentTactic, error) {
	id, err := parseUUID(t.ID)
	if err != nil {
		return gen.ContentTactic{}, err
	}
	sourceID, err := parseUUID(t.SourceID)
	if err != nil {
		return gen.ContentTactic{}, err
	}
	return gen.ContentTactic{
		Id:          id,
		SourceId:    sourceID,
		Version:     t.Version,
		ExternalId:  t.ExternalID,
		Name:        t.Name,
		Description: t.Description,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}, nil
}

func contentMitigation(m storecontent.Mitigation) (gen.ContentMitigation, error) {
	id, err := parseUUID(m.ID)
	if err != nil {
		return gen.ContentMitigation{}, err
	}
	sourceID, err := parseUUID(m.SourceID)
	if err != nil {
		return gen.ContentMitigation{}, err
	}
	return gen.ContentMitigation{
		Id:          id,
		SourceId:    sourceID,
		Version:     m.Version,
		ExternalId:  m.ExternalID,
		Name:        m.Name,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}

func contentGroup(g storecontent.Group) (gen.ContentGroup, error) {
	id, err := parseUUID(g.ID)
	if err != nil {
		return gen.ContentGroup{}, err
	}
	sourceID, err := parseUUID(g.SourceID)
	if err != nil {
		return gen.ContentGroup{}, err
	}
	return gen.ContentGroup{
		Id:          id,
		SourceId:    sourceID,
		Version:     g.Version,
		ExternalId:  g.ExternalID,
		Name:        g.Name,
		Description: g.Description,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}, nil
}

func contentSoftware(s storecontent.Software) (gen.ContentSoftware, error) {
	id, err := parseUUID(s.ID)
	if err != nil {
		return gen.ContentSoftware{}, err
	}
	sourceID, err := parseUUID(s.SourceID)
	if err != nil {
		return gen.ContentSoftware{}, err
	}
	return gen.ContentSoftware{
		Id:           id,
		SourceId:     sourceID,
		Version:      s.Version,
		ExternalId:   s.ExternalID,
		Name:         s.Name,
		Description:  s.Description,
		SoftwareType: gen.ContentSoftwareType(s.SoftwareType),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}, nil
}
