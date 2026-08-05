package httpapi

import (
	"context"
	"encoding/json"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// ATT&CK / Atomic / Sigma library browse endpoints (M2-006, M2-008, M2-009).
// content.read; enabled sources only.

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

func (h *handlers) ListContentProcedureTemplates(ctx context.Context, request gen.ListContentProcedureTemplatesRequestObject) (gen.ListContentProcedureTemplatesResponseObject, error) {
	f := storecontent.ProcedureListFilter{EnabledOnly: true}
	if request.Params.Q != nil {
		f.Q = *request.Params.Q
	}
	if request.Params.Technique != nil {
		f.Technique = *request.Params.Technique
	}
	if request.Params.Platform != nil {
		f.Platform = *request.Params.Platform
	}
	if request.Params.SourceId != nil {
		f.SourceID = request.Params.SourceId.String()
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	items, err := h.procedures.List(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentProcedureTemplate, 0, len(items))
	for _, p := range items {
		wire, err := contentProcedureTemplate(p)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentProcedureTemplates200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentProcedureTemplate(ctx context.Context, request gen.GetContentProcedureTemplateRequestObject) (gen.GetContentProcedureTemplateResponseObject, error) {
	p, err := h.procedures.ByIDEnabled(ctx, request.TemplateId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentProcedureTemplate(p)
	if err != nil {
		return nil, err
	}
	return gen.GetContentProcedureTemplate200JSONResponse(wire), nil
}

func (h *handlers) ListContentDetectionRules(ctx context.Context, request gen.ListContentDetectionRulesRequestObject) (gen.ListContentDetectionRulesResponseObject, error) {
	f := storecontent.DetectionListFilter{EnabledOnly: true}
	if request.Params.Q != nil {
		f.Q = *request.Params.Q
	}
	if request.Params.Technique != nil {
		f.Technique = *request.Params.Technique
	}
	if request.Params.Level != nil {
		f.Level = *request.Params.Level
	}
	if request.Params.SourceId != nil {
		f.SourceID = request.Params.SourceId.String()
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	items, err := h.detections.List(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentDetectionRule, 0, len(items))
	for _, d := range items {
		wire, err := contentDetectionRule(d)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListContentDetectionRules200JSONResponse{Items: out}, nil
}

func (h *handlers) GetContentDetectionRule(ctx context.Context, request gen.GetContentDetectionRuleRequestObject) (gen.GetContentDetectionRuleResponseObject, error) {
	d, err := h.detections.ByIDEnabled(ctx, request.RuleId.String(), true)
	if err != nil {
		return nil, err
	}
	wire, err := contentDetectionRule(d)
	if err != nil {
		return nil, err
	}
	return gen.GetContentDetectionRule200JSONResponse(wire), nil
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

func contentProcedureTemplate(p storecontent.ProcedureTemplate) (gen.ContentProcedureTemplate, error) {
	id, err := parseUUID(p.ID)
	if err != nil {
		return gen.ContentProcedureTemplate{}, err
	}
	sourceID, err := parseUUID(p.SourceID)
	if err != nil {
		return gen.ContentProcedureTemplate{}, err
	}

	platforms, err := decodeStringArray(p.Platforms)
	if err != nil {
		return gen.ContentProcedureTemplate{}, err
	}
	techs, err := decodeStringArray(p.TechniqueExternalIDs)
	if err != nil {
		return gen.ContentProcedureTemplate{}, err
	}
	args, err := decodeInputArgs(p.InputArgs)
	if err != nil {
		return gen.ContentProcedureTemplate{}, err
	}

	return gen.ContentProcedureTemplate{
		Id:                     id,
		SourceId:               sourceID,
		Version:                p.Version,
		ExternalId:             p.ExternalID,
		Name:                   p.Name,
		Description:            p.Description,
		Platforms:              platforms,
		Executor:               p.Executor,
		ElevationRequired:      p.ElevationRequired,
		Command:                p.Command,
		Cleanup:                p.Cleanup,
		InputArgs:              args,
		TechniqueExternalIds:   techs,
		DependencyExecutorName: p.DependencyExecutorName,
		Dependencies:           p.Dependencies,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
	}, nil
}

func contentDetectionRule(d storecontent.DetectionRuleRef) (gen.ContentDetectionRule, error) {
	id, err := parseUUID(d.ID)
	if err != nil {
		return gen.ContentDetectionRule{}, err
	}
	sourceID, err := parseUUID(d.SourceID)
	if err != nil {
		return gen.ContentDetectionRule{}, err
	}
	techs, err := decodeStringArray(d.TechniqueExternalIDs)
	if err != nil {
		return gen.ContentDetectionRule{}, err
	}
	logsource, err := decodeLogsource(d.Logsource)
	if err != nil {
		return gen.ContentDetectionRule{}, err
	}
	return gen.ContentDetectionRule{
		Id:                   id,
		SourceId:             sourceID,
		Version:              d.Version,
		ExternalId:           d.ExternalID,
		Name:                 d.Name,
		Description:          d.Description,
		TechniqueExternalIds: techs,
		Level:                d.Level,
		Status:               d.RuleStatus,
		Logsource:            logsource,
		RuleYaml:             d.RuleYAML,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
	}, nil
}

func decodeLogsource(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

func decodeStringArray(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

func decodeInputArgs(raw json.RawMessage) ([]gen.ContentProcedureInputArg, error) {
	if len(raw) == 0 {
		return []gen.ContentProcedureInputArg{}, nil
	}
	// Adapter writes a JSON array of {name,description,type,default}.
	// Older/custom rows may store the upstream map shape — accept both.
	var arr []gen.ContentProcedureInputArg
	if err := json.Unmarshal(raw, &arr); err == nil {
		if arr == nil {
			arr = []gen.ContentProcedureInputArg{}
		}
		return arr, nil
	}
	var obj map[string]struct {
		Description string `json:"description"`
		Type        string `json:"type"`
		Default     any    `json:"default"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	out := make([]gen.ContentProcedureInputArg, 0, len(obj))
	for name, a := range obj {
		out = append(out, gen.ContentProcedureInputArg{
			Name:        name,
			Description: a.Description,
			Type:        a.Type,
			Default:     coerceString(a.Default),
		})
	}
	return out, nil
}

func coerceString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
