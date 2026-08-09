package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/google/uuid"
)

// Report template handlers (M6-003).

// ListReportTemplates returns every template in an engagement.
func (h *handlers) ListReportTemplates(ctx context.Context,
	request gen.ListReportTemplatesRequestObject) (gen.ListReportTemplatesResponseObject, error) {

	tmpls, err := h.templates.ListTemplates(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	var items []gen.ReportTemplate
	for _, t := range tmpls {
		w, err := templateToWire(t, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}

	return gen.ListReportTemplates200JSONResponse(items), nil
}

// CreateReportTemplate creates a new report template in an engagement.
func (h *handlers) CreateReportTemplate(ctx context.Context,
	request gen.CreateReportTemplateRequestObject) (gen.CreateReportTemplateResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	in := report.CreateTemplateInput{
		EngagementID: request.EngagementId.String(),
		ActorID:      subject.UserID,
	}
	if request.Body != nil {
		in.Name = request.Body.Name
	}

	tmpl, err := h.templates.CreateTemplate(ctx, in)
	if err != nil {
		return nil, err
	}

	w, err := templateToWire(tmpl, nil)
	if err != nil {
		return nil, err
	}
	return gen.CreateReportTemplate201JSONResponse(w), nil
}

// GetReportTemplate returns one template with its blocks.
func (h *handlers) GetReportTemplate(ctx context.Context,
	request gen.GetReportTemplateRequestObject) (gen.GetReportTemplateResponseObject, error) {

	tmpl, err := h.templates.GetTemplate(ctx, request.TemplateId.String())
	if err != nil {
		return nil, err
	}

	blocks, err := h.templates.TemplateBlocks(ctx, request.TemplateId.String())
	if err != nil {
		return nil, err
	}

	w, err := templateToWire(tmpl, blocks)
	if err != nil {
		return nil, err
	}
	return gen.GetReportTemplate200JSONResponse(w), nil
}

// PatchReportTemplate patches template fields.
func (h *handlers) PatchReportTemplate(ctx context.Context,
	request gen.PatchReportTemplateRequestObject) (gen.PatchReportTemplateResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch report template: missing body")
	}

	in := report.UpdateTemplateInput{
		ActorID: subject.UserID,
	}
	if request.Body.Name != nil {
		in.Name = request.Body.Name
	}

	tmpl, err := h.templates.UpdateTemplate(ctx, request.TemplateId.String(), in)
	if err != nil {
		return nil, err
	}

	w, err := templateToWire(tmpl, nil)
	if err != nil {
		return nil, err
	}
	return gen.PatchReportTemplate200JSONResponse(w), nil
}

// DeleteReportTemplate deletes a template and its blocks.
func (h *handlers) DeleteReportTemplate(ctx context.Context,
	request gen.DeleteReportTemplateRequestObject) (gen.DeleteReportTemplateResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.templates.DeleteTemplate(ctx, request.TemplateId.String(), subject.UserID); err != nil {
		return nil, err
	}

	return gen.DeleteReportTemplate204Response{}, nil
}

// ApplyReportTemplate applies a template to a report draft, replacing its blocks.
func (h *handlers) ApplyReportTemplate(ctx context.Context,
	request gen.ApplyReportTemplateRequestObject) (gen.ApplyReportTemplateResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: apply template: missing body")
	}

	r, err := h.templates.ApplyTemplate(ctx, report.ApplyTemplateInput{
		ReportID:   request.ReportId.String(),
		TemplateID: request.Body.TemplateId.String(),
		ActorID:    subject.UserID,
	})
	if err != nil {
		return nil, err
	}

	blocks, err := h.reports.Blocks(ctx, request.ReportId.String())
	if err != nil {
		return nil, err
	}

	w, err := reportToWire(r, blocks)
	if err != nil {
		return nil, err
	}
	return gen.ApplyReportTemplate200JSONResponse(w), nil
}

// CreateReportTemplateFromReport snapshots a report's blocks into a new template.
func (h *handlers) CreateReportTemplateFromReport(ctx context.Context,
	request gen.CreateReportTemplateFromReportRequestObject) (gen.CreateReportTemplateFromReportResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: create template from report: missing body")
	}

	tmpl, err := h.templates.CreateFromReport(ctx, report.CreateFromReportInput{
		EngagementID: request.EngagementId.String(),
		ReportID:     request.Body.ReportId.String(),
		Name:         request.Body.Name,
		ActorID:      subject.UserID,
	})
	if err != nil {
		return nil, err
	}

	blocks, err := h.templates.TemplateBlocks(ctx, tmpl.ID)
	if err != nil {
		return nil, err
	}

	w, err := templateToWire(tmpl, blocks)
	if err != nil {
		return nil, err
	}
	return gen.CreateReportTemplateFromReport201JSONResponse(w), nil
}

// templateToWire converts a store template to the wire format.
func templateToWire(t storereport.Template, blocks []storereport.TemplateBlock) (gen.ReportTemplate, error) {
	engID, err := uuid.Parse(t.EngagementID)
	if err != nil {
		return gen.ReportTemplate{}, fmt.Errorf("templateToWire: engagement_id: %w", err)
	}
	tmplID, err := uuid.Parse(t.ID)
	if err != nil {
		return gen.ReportTemplate{}, fmt.Errorf("templateToWire: id: %w", err)
	}

	w := gen.ReportTemplate{
		Id:           gen.TemplateId(tmplID),
		EngagementId: gen.EngagementId(engID),
		Name:         t.Name,
		CreatedBy:    t.CreatedBy,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}

	if blocks != nil {
		wireBlocks := make([]gen.ReportTemplateBlock, len(blocks))
		for i, b := range blocks {
			var params map[string]interface{}
			if len(b.Params) > 0 {
				if err := json.Unmarshal(b.Params, &params); err != nil {
					return gen.ReportTemplate{}, fmt.Errorf("templateToWire: block[%d] params: %w", i, err)
				}
			} else {
				params = make(map[string]interface{})
			}
			wireBlocks[i] = gen.ReportTemplateBlock{
				Ordinal: b.Ordinal,
				BlockId: b.BlockID,
				Params:  params,
			}
		}
		w.Blocks = &wireBlocks
	}

	return w, nil
}
