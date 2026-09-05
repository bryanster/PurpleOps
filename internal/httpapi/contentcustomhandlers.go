package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Custom content CRUD + export (M2-011). content.read for GET; content.manage
// for mutations. Domain logic lives in internal/content.Custom.

func (h *handlers) ListCustomProcedureTemplates(ctx context.Context, request gen.ListCustomProcedureTemplatesRequestObject) (gen.ListCustomProcedureTemplatesResponseObject, error) {
	f := storecontent.ProcedureListFilter{}
	if request.Params.Q != nil {
		f.Q = *request.Params.Q
	}
	if request.Params.Technique != nil {
		f.Technique = *request.Params.Technique
	}
	if request.Params.Platform != nil {
		f.Platform = *request.Params.Platform
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	items, err := h.custom.ListProcedures(ctx, f)
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
	return gen.ListCustomProcedureTemplates200JSONResponse{Items: out}, nil
}

func (h *handlers) CreateCustomProcedureTemplate(ctx context.Context, request gen.CreateCustomProcedureTemplateRequestObject) (gen.CreateCustomProcedureTemplateResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("create custom procedure: missing body")
	}
	in := content.ProcedureCreate{
		Name:                   body.Name,
		Description:            derefStr(body.Description),
		Platforms:              derefStrs(body.Platforms),
		Executor:               derefStr(body.Executor),
		ElevationRequired:      derefBool(body.ElevationRequired),
		Command:                derefStr(body.Command),
		Cleanup:                derefStr(body.Cleanup),
		TechniqueExternalIDs:   derefStrs(body.TechniqueExternalIds),
		DependencyExecutorName: derefStr(body.DependencyExecutorName),
		Dependencies:           derefStr(body.Dependencies),
	}
	if body.ExternalId != nil {
		in.ExternalID = *body.ExternalId
	}
	if body.InputArgs != nil {
		raw, err := json.Marshal(body.InputArgs)
		if err != nil {
			return nil, err
		}
		in.InputArgs = raw
	}
	p, err := h.custom.CreateProcedure(ctx, actor, in)
	if err != nil {
		return nil, err
	}
	wire, err := contentProcedureTemplate(p)
	if err != nil {
		return nil, err
	}
	return gen.CreateCustomProcedureTemplate201JSONResponse(wire), nil
}

func (h *handlers) GetCustomProcedureTemplate(ctx context.Context, request gen.GetCustomProcedureTemplateRequestObject) (gen.GetCustomProcedureTemplateResponseObject, error) {
	p, err := h.custom.GetProcedure(ctx, request.TemplateId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentProcedureTemplate(p)
	if err != nil {
		return nil, err
	}
	return gen.GetCustomProcedureTemplate200JSONResponse(wire), nil
}

func (h *handlers) UpdateCustomProcedureTemplate(ctx context.Context, request gen.UpdateCustomProcedureTemplateRequestObject) (gen.UpdateCustomProcedureTemplateResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("update custom procedure: missing body")
	}
	edit := content.ProcedureEdit{
		Name:                   body.Name,
		Description:            body.Description,
		Platforms:              body.Platforms,
		Executor:               body.Executor,
		ElevationRequired:      body.ElevationRequired,
		Command:                body.Command,
		Cleanup:                body.Cleanup,
		TechniqueExternalIDs:   body.TechniqueExternalIds,
		DependencyExecutorName: body.DependencyExecutorName,
		Dependencies:           body.Dependencies,
	}
	if body.InputArgs != nil {
		raw, err := json.Marshal(body.InputArgs)
		if err != nil {
			return nil, err
		}
		msg := json.RawMessage(raw)
		edit.InputArgs = &msg
	}
	p, err := h.custom.UpdateProcedure(ctx, actor, request.TemplateId.String(), edit)
	if err != nil {
		return nil, err
	}
	wire, err := contentProcedureTemplate(p)
	if err != nil {
		return nil, err
	}
	return gen.UpdateCustomProcedureTemplate200JSONResponse(wire), nil
}

func (h *handlers) DeleteCustomProcedureTemplate(ctx context.Context, request gen.DeleteCustomProcedureTemplateRequestObject) (gen.DeleteCustomProcedureTemplateResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.custom.DeleteProcedure(ctx, actor, request.TemplateId.String()); err != nil {
		return nil, err
	}
	return gen.DeleteCustomProcedureTemplate204Response{}, nil
}

func (h *handlers) ListCustomDetectionRules(ctx context.Context, request gen.ListCustomDetectionRulesRequestObject) (gen.ListCustomDetectionRulesResponseObject, error) {
	f := storecontent.DetectionListFilter{}
	if request.Params.Q != nil {
		f.Q = *request.Params.Q
	}
	if request.Params.Technique != nil {
		f.Technique = *request.Params.Technique
	}
	if request.Params.Level != nil {
		f.Level = *request.Params.Level
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	items, err := h.custom.ListDetections(ctx, f)
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
	return gen.ListCustomDetectionRules200JSONResponse{Items: out}, nil
}

func (h *handlers) CreateCustomDetectionRule(ctx context.Context, request gen.CreateCustomDetectionRuleRequestObject) (gen.CreateCustomDetectionRuleResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("create custom detection: missing body")
	}
	in := content.DetectionCreate{
		Name:                 body.Name,
		Description:          derefStr(body.Description),
		TechniqueExternalIDs: derefStrs(body.TechniqueExternalIds),
		Level:                derefStr(body.Level),
		RuleStatus:           derefStr(body.Status),
		RuleYAML:             body.RuleYaml,
	}
	if body.ExternalId != nil {
		in.ExternalID = *body.ExternalId
	}
	if body.Logsource != nil {
		raw, err := json.Marshal(logsourceMap(*body.Logsource))
		if err != nil {
			return nil, err
		}
		in.Logsource = raw
	}
	d, err := h.custom.CreateDetection(ctx, actor, in)
	if err != nil {
		return nil, err
	}
	wire, err := contentDetectionRule(d)
	if err != nil {
		return nil, err
	}
	return gen.CreateCustomDetectionRule201JSONResponse(wire), nil
}

func (h *handlers) GetCustomDetectionRule(ctx context.Context, request gen.GetCustomDetectionRuleRequestObject) (gen.GetCustomDetectionRuleResponseObject, error) {
	d, err := h.custom.GetDetection(ctx, request.RuleId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentDetectionRule(d)
	if err != nil {
		return nil, err
	}
	return gen.GetCustomDetectionRule200JSONResponse(wire), nil
}

func (h *handlers) UpdateCustomDetectionRule(ctx context.Context, request gen.UpdateCustomDetectionRuleRequestObject) (gen.UpdateCustomDetectionRuleResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("update custom detection: missing body")
	}
	edit := content.DetectionEdit{
		Name:                 body.Name,
		Description:          body.Description,
		TechniqueExternalIDs: body.TechniqueExternalIds,
		Level:                body.Level,
		RuleStatus:           body.Status,
		RuleYAML:             body.RuleYaml,
	}
	if body.Logsource != nil {
		raw, err := json.Marshal(logsourceMap(*body.Logsource))
		if err != nil {
			return nil, err
		}
		msg := json.RawMessage(raw)
		edit.Logsource = &msg
	}

	d, err := h.custom.UpdateDetection(ctx, actor, request.RuleId.String(), edit)
	if err != nil {
		return nil, err
	}
	wire, err := contentDetectionRule(d)
	if err != nil {
		return nil, err
	}
	return gen.UpdateCustomDetectionRule200JSONResponse(wire), nil
}

func (h *handlers) DeleteCustomDetectionRule(ctx context.Context, request gen.DeleteCustomDetectionRuleRequestObject) (gen.DeleteCustomDetectionRuleResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.custom.DeleteDetection(ctx, actor, request.RuleId.String()); err != nil {
		return nil, err
	}
	return gen.DeleteCustomDetectionRule204Response{}, nil
}

func (h *handlers) ListCustomNotes(ctx context.Context, request gen.ListCustomNotesRequestObject) (gen.ListCustomNotesResponseObject, error) {
	f := storecontent.NoteListFilter{}
	if request.Params.Q != nil {
		f.Q = *request.Params.Q
	}
	if request.Params.Technique != nil {
		f.Technique = *request.Params.Technique
	}
	if request.Params.Limit != nil {
		f.Limit = *request.Params.Limit
	}
	items, err := h.custom.ListNotes(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]gen.ContentNote, 0, len(items))
	for _, n := range items {
		wire, err := contentNote(n)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return gen.ListCustomNotes200JSONResponse{Items: out}, nil
}

func (h *handlers) CreateCustomNote(ctx context.Context, request gen.CreateCustomNoteRequestObject) (gen.CreateCustomNoteResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("create custom note: missing body")
	}
	in := content.NoteCreate{
		Title:               body.Title,
		BodyMarkdown:        body.BodyMarkdown,
		Tags:                derefStrs(body.Tags),
		TechniqueExternalID: derefStr(body.TechniqueExternalId),
	}
	if body.ExternalId != nil {
		in.ExternalID = *body.ExternalId
	}
	n, err := h.custom.CreateNote(ctx, actor, in)
	if err != nil {
		return nil, err
	}
	wire, err := contentNote(n)
	if err != nil {
		return nil, err
	}
	return gen.CreateCustomNote201JSONResponse(wire), nil
}

func (h *handlers) GetCustomNote(ctx context.Context, request gen.GetCustomNoteRequestObject) (gen.GetCustomNoteResponseObject, error) {
	n, err := h.custom.GetNote(ctx, request.NoteId.String())
	if err != nil {
		return nil, err
	}
	wire, err := contentNote(n)
	if err != nil {
		return nil, err
	}
	return gen.GetCustomNote200JSONResponse(wire), nil
}

func (h *handlers) UpdateCustomNote(ctx context.Context, request gen.UpdateCustomNoteRequestObject) (gen.UpdateCustomNoteResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body == nil {
		return nil, fmt.Errorf("update custom note: missing body")
	}
	edit := content.NoteEdit{
		Title:               body.Title,
		BodyMarkdown:        body.BodyMarkdown,
		Tags:                body.Tags,
		TechniqueExternalID: body.TechniqueExternalId,
	}
	n, err := h.custom.UpdateNote(ctx, actor, request.NoteId.String(), edit)
	if err != nil {
		return nil, err
	}
	wire, err := contentNote(n)
	if err != nil {
		return nil, err
	}
	return gen.UpdateCustomNote200JSONResponse(wire), nil
}

func (h *handlers) DeleteCustomNote(ctx context.Context, request gen.DeleteCustomNoteRequestObject) (gen.DeleteCustomNoteResponseObject, error) {
	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.custom.DeleteNote(ctx, actor, request.NoteId.String()); err != nil {
		return nil, err
	}
	return gen.DeleteCustomNote204Response{}, nil
}

func (h *handlers) ExportCustomContent(ctx context.Context, request gen.ExportCustomContentRequestObject) (gen.ExportCustomContentResponseObject, error) {
	typ := content.ExportAll
	if request.Params.Type != nil {
		typ = content.ExportType(*request.Params.Type)
	}
	format := content.ExportYAML
	if request.Params.Format != nil {
		format = content.ExportFormat(*request.Params.Format)
	}

	doc, err := h.custom.Export(ctx, typ)
	if err != nil {
		return nil, err
	}
	wire, err := contentCustomExport(doc)
	if err != nil {
		return nil, err
	}

	if format == content.ExportJSON {
		return gen.ExportCustomContent200JSONResponse(wire), nil
	}

	// YAML: header comments with license/attribution, then the document body.
	var buf bytes.Buffer
	buf.WriteString("# Blacklight custom content export\n")
	fmt.Fprintf(&buf, "# Source: %s\n", doc.Meta.SourceName)
	if doc.Meta.LicenseSPDX != "" {
		fmt.Fprintf(&buf, "# License: %s", doc.Meta.LicenseSPDX)
		if doc.Meta.LicenseName != "" {
			fmt.Fprintf(&buf, " (%s)", doc.Meta.LicenseName)
		}
		buf.WriteByte('\n')
	}
	if doc.Meta.LicenseURL != "" {
		fmt.Fprintf(&buf, "# License-URL: %s\n", doc.Meta.LicenseURL)
	}
	if doc.Meta.Attribution != "" {
		for _, line := range strings.Split(doc.Meta.Attribution, "\n") {
			buf.WriteString("# Attribution: " + line + "\n")
		}
	}
	fmt.Fprintf(&buf, "# Exported-At: %s\n", doc.Meta.ExportedAt.Format(time.RFC3339))
	buf.WriteByte('\n')

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(wire); err != nil {
		return nil, fmt.Errorf("export custom yaml: %w", err)
	}
	_ = enc.Close()

	body := bytes.NewReader(buf.Bytes())
	return gen.ExportCustomContent200ApplicationyamlResponse{
		Body:          body,
		ContentLength: int64(buf.Len()),
	}, nil
}

func contentNote(n storecontent.Note) (gen.ContentNote, error) {
	id, err := parseUUID(n.ID)
	if err != nil {
		return gen.ContentNote{}, err
	}
	sourceID, err := parseUUID(n.SourceID)
	if err != nil {
		return gen.ContentNote{}, err
	}
	tags, err := decodeStringArray(n.Tags)
	if err != nil {
		return gen.ContentNote{}, err
	}
	return gen.ContentNote{
		Id:                  id,
		SourceId:            sourceID,
		Version:             n.Version,
		ExternalId:          n.ExternalID,
		Title:               n.Title,
		BodyMarkdown:        n.BodyMarkdown,
		Tags:                tags,
		TechniqueExternalId: n.TechniqueExternalID,
		CreatedAt:           n.CreatedAt,
		UpdatedAt:           n.UpdatedAt,
	}, nil
}

func contentCustomExport(doc content.ExportDoc) (gen.ContentCustomExport, error) {
	procs := make([]gen.ContentProcedureTemplate, 0, len(doc.ProcedureTemplates))
	for _, p := range doc.ProcedureTemplates {
		wire, err := contentProcedureTemplate(p)
		if err != nil {
			return gen.ContentCustomExport{}, err
		}
		procs = append(procs, wire)
	}
	dets := make([]gen.ContentDetectionRule, 0, len(doc.DetectionRules))
	for _, d := range doc.DetectionRules {
		wire, err := contentDetectionRule(d)
		if err != nil {
			return gen.ContentCustomExport{}, err
		}
		dets = append(dets, wire)
	}
	notes := make([]gen.ContentNote, 0, len(doc.Notes))
	for _, n := range doc.Notes {
		wire, err := contentNote(n)
		if err != nil {
			return gen.ContentCustomExport{}, err
		}
		notes = append(notes, wire)
	}
	meta := gen.ContentCustomExportMeta{
		SourceName:  doc.Meta.SourceName,
		Attribution: doc.Meta.Attribution,
		ExportedAt:  doc.Meta.ExportedAt,
	}
	if doc.Meta.LicenseSPDX != "" {
		s := doc.Meta.LicenseSPDX
		meta.LicenseSpdx = &s
	}
	if doc.Meta.LicenseName != "" {
		s := doc.Meta.LicenseName
		meta.LicenseName = &s
	}
	if doc.Meta.LicenseURL != "" {
		s := doc.Meta.LicenseURL
		meta.LicenseUrl = &s
	}
	return gen.ContentCustomExport{
		Meta:               meta,
		ProcedureTemplates: procs,
		DetectionRules:     dets,
		Notes:              notes,
	}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefStrs(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func logsourceMap(ls gen.ContentDetectionLogsource) map[string]string {
	out := map[string]string{}
	if ls.Category != nil && *ls.Category != "" {
		out["category"] = *ls.Category
	}
	if ls.Product != nil && *ls.Product != "" {
		out["product"] = *ls.Product
	}
	if ls.Service != nil && *ls.Service != "" {
		out["service"] = *ls.Service
	}
	if ls.Definition != nil && *ls.Definition != "" {
		out["definition"] = *ls.Definition
	}
	return out
}
