package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	identity "github.com/bryanster/blacklight/internal/store/identity"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/blind"
	"github.com/google/uuid"
)
// Report CRUD + blocks handlers (M6-002).

// ListReports returns every report in an engagement.
func (h *handlers) ListReports(ctx context.Context,
	request gen.ListReportsRequestObject) (gen.ListReportsResponseObject, error) {

	reports, err := h.reports.ListByEngagement(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	var items []gen.Report
	for _, r := range reports {
		w, err := reportToWire(r, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}

	return gen.ListReports200JSONResponse(items), nil
}

// CreateReport creates a new report draft in an engagement.
func (h *handlers) CreateReport(ctx context.Context,
	request gen.CreateReportRequestObject) (gen.CreateReportResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	in := report.CreateInput{
		EngagementID: request.EngagementId.String(),
		ActorID:      subject.UserID,
	}
	if request.Body != nil && request.Body.Title != nil {
		in.Title = *request.Body.Title
	}

	r, err := h.reports.Create(ctx, in)
	if err != nil {
		return nil, err
	}

	w, err := reportToWire(r, nil)
	if err != nil {
		return nil, err
	}
	return gen.CreateReport201JSONResponse(w), nil
}

// GetReport returns one report with its blocks.
func (h *handlers) GetReport(ctx context.Context,
	request gen.GetReportRequestObject) (gen.GetReportResponseObject, error) {

	r, err := h.reports.Get(ctx, request.ReportId.String())
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
	return gen.GetReport200JSONResponse(w), nil
}

// PatchReport patches report fields.
func (h *handlers) PatchReport(ctx context.Context,
	request gen.PatchReportRequestObject) (gen.PatchReportResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch report: missing body")
	}

	in := report.UpdateInput{
		ActorID: subject.UserID,
	}

	if request.Body.Title != nil {
		in.Title = request.Body.Title
	}
	if request.Body.ClientName.IsSpecified() {
		v := nullableToStringPtr(request.Body.ClientName)
		in.ClientName = &v
	}
	if request.Body.LogoBlobRef.IsSpecified() {
		v := nullableToStringPtr(request.Body.LogoBlobRef)
		in.LogoBlobRef = &v
	}
	if request.Body.Colours.IsSpecified() {
		switch {
		case request.Body.Colours.IsNull():
			empty := json.RawMessage(nil)
			in.Colours = &empty
		default:
			raw, err := json.Marshal(request.Body.Colours.MustGet())
			if err != nil {
				return nil, fmt.Errorf("httpapi: patch report colours: %w", err)
			}
			msg := json.RawMessage(raw)
			in.Colours = &msg
		}
	}

	r, err := h.reports.Update(ctx, request.ReportId.String(), in)
	if err != nil {
		return nil, err
	}

	w, err := reportToWire(r, nil)
	if err != nil {
		return nil, err
	}
	return gen.PatchReport200JSONResponse(w), nil
}

// DeleteReport deletes a report and its draft blocks.
func (h *handlers) DeleteReport(ctx context.Context,
	request gen.DeleteReportRequestObject) (gen.DeleteReportResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.reports.Delete(ctx, request.ReportId.String(), subject.UserID); err != nil {
		return nil, err
	}

	return gen.DeleteReport204Response{}, nil
}

// PutReportBlocks replaces the complete ordered list of draft blocks.
func (h *handlers) PutReportBlocks(ctx context.Context,
	request gen.PutReportBlocksRequestObject) (gen.PutReportBlocksResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: put report blocks: missing body")
	}

	blocks := make([]report.BlockInput, len(request.Body.Blocks))
	for i, bi := range request.Body.Blocks {
		var params json.RawMessage
		if bi.Params != nil {
			raw, err := json.Marshal(*bi.Params)
			if err != nil {
				return nil, fmt.Errorf("httpapi: put report blocks[%d].params: %w", i, err)
			}
			params = json.RawMessage(raw)
		} else {
			params = json.RawMessage(`{}`)
		}
		blocks[i] = report.BlockInput{
			BlockID: bi.BlockId,
			Params:  params,
		}
	}

	r, err := h.reports.ReplaceBlocks(ctx, report.ReplaceBlocksInput{
		ReportID: request.ReportId.String(),
		Blocks:   blocks,
		ActorID:  subject.UserID,
	})
	if err != nil {
		return nil, err
	}

	reportBlocks, err := h.reports.Blocks(ctx, request.ReportId.String())
	if err != nil {
		return nil, err
	}

	w, err := reportToWire(r, reportBlocks)
	if err != nil {
		return nil, err
	}
	return gen.PutReportBlocks200JSONResponse(w), nil
}

// reportToWire converts a store report to the wire format.
func reportToWire(r storereport.Report, blocks []storereport.ReportBlock) (gen.Report, error) {
	engID, err := uuid.Parse(r.EngagementID)
	if err != nil {
		return gen.Report{}, fmt.Errorf("reportToWire: engagement_id: %w", err)
	}
	repID, err := uuid.Parse(r.ID)
	if err != nil {
		return gen.Report{}, fmt.Errorf("reportToWire: id: %w", err)
	}

	w := gen.Report{
		Id:           gen.ReportId(repID),
		EngagementId: gen.EngagementId(engID),
		Title:        r.Title,
		CreatedBy:    r.CreatedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}

	if r.ClientName != nil {
		w.ClientName.Set(*r.ClientName)
	}
	if r.LogoBlobRef != nil {
		w.LogoBlobRef.Set(*r.LogoBlobRef)
	}
	if len(r.Colours) > 0 {
		var c map[string]interface{}
		if err := json.Unmarshal(r.Colours, &c); err != nil {
			return gen.Report{}, fmt.Errorf("reportToWire: colours: %w", err)
		}
		w.Colours.Set(c)
	}
	if r.UpdatedBy != nil {
		w.UpdatedBy.Set(*r.UpdatedBy)
	}

	if blocks != nil {
		wireBlocks := make([]gen.ReportBlock, len(blocks))
		for i, b := range blocks {
			bid, err := uuid.Parse(b.ID)
			if err != nil {
				return gen.Report{}, fmt.Errorf("reportToWire: block id: %w", err)
			}
			var params map[string]interface{}
			if len(b.Params) > 0 {
				if err := json.Unmarshal(b.Params, &params); err != nil {
					return gen.Report{}, fmt.Errorf("reportToWire: block params: %w", err)
				}
			} else {
				params = make(map[string]interface{})
			}
			wireBlocks[i] = gen.ReportBlock{
				Id:      bid,
				Ordinal: b.Ordinal,
				BlockId: b.BlockID,
				Params:  params,
			}
		}
		w.Blocks = &wireBlocks
	}

	return w, nil
}

// nullableToStringPtr converts a nullable string to *string for the service layer.
func nullableToStringPtr(ns nullable.Nullable[string]) *string {
	if ns.IsNull() {
		s := new(string)
		*s = ""
		return s
	}
	v, err := ns.Get()
	if err != nil {
		return nil
	}
	return &v
}
// PreviewReport renders the draft report as HTML (M6-009).
func (h *handlers) PreviewReport(ctx context.Context, request gen.PreviewReportRequestObject) (gen.PreviewReportResponseObject, error) {
	env, errResp := h.previewReportEnv(ctx, request.EngagementId.String(), request.ReportId.String(), request.Params.IncludeEvidence)
	if errResp != nil {
		return errResp, nil
	}

	rep, blocks, errResp := h.previewReportData(ctx, request.ReportId.String())
	if errResp != nil {
		return errResp, nil
	}

	doc := h.docRenderer.RenderDocument(ctx, *rep, blocks, *env)

	return gen.PreviewReport200TexthtmlResponse{
		Body: bytes.NewReader(doc.HTML),
	}, nil
}

// PreviewReportPdf renders draft report as PDF via Chromium (M6-010).
func (h *handlers) PreviewReportPdf(ctx context.Context, request gen.PreviewReportPdfRequestObject) (gen.PreviewReportPdfResponseObject, error) {
	if h.pdfPrinter == nil {
		return gen.PreviewReportPdf503Response{}, nil
	}

	env, errResp := h.previewReportEnv(ctx, request.EngagementId.String(), request.ReportId.String(), request.Params.IncludeEvidence)
	if errResp != nil {
		return gen.PreviewReportPdf500ApplicationProblemPlusJSONResponse{}, nil
	}

	rep, blocks, errResp := h.previewReportData(ctx, request.ReportId.String())
	if errResp != nil {
		return gen.PreviewReportPdf500ApplicationProblemPlusJSONResponse{}, nil
	}

	doc := h.docRenderer.RenderDocument(ctx, *rep, blocks, *env)

	pdf, err := h.pdfPrinter.RenderPDF(ctx, doc.HTML)
	if err != nil {
		h.log.Error("preview-pdf: render", "report_id", request.ReportId, "err", err)
		return gen.PreviewReportPdf500ApplicationProblemPlusJSONResponse{}, nil
	}

	return gen.PreviewReportPdf200ApplicationpdfResponse{
		Body: bytes.NewReader(pdf),
	}, nil
}

// previewReportEnv loads the engagement, resolves branding, determines blind
// scope, and constructs the RenderEnv for preview handlers.
func (h *handlers) previewReportEnv(ctx context.Context, engagementID, reportID string, includeEvidence *bool) (*report.RenderEnv, gen.PreviewReportResponseObject) {
	engagements := storengagement.NewEngagements(h.store)
	eng, err := engagements.ByID(ctx, engagementID)
	if err != nil {
		h.log.Error("preview: get engagement", "engagement_id", engagementID, "err", err)
		return nil, gen.PreviewReport404ApplicationProblemPlusJSONResponse{}
	}

	rep, err := h.reports.Get(ctx, reportID)
	if err != nil {
		h.log.Error("preview: get report", "report_id", reportID, "err", err)
		return nil, gen.PreviewReport404ApplicationProblemPlusJSONResponse{}
	}

	resolver := report.NewBrandingResolver(h.brandingSettings)
	branding, err := resolver.Resolve(ctx, rep)
	if err != nil {
		h.log.Error("preview: branding", "report_id", reportID, "err", err)
		return nil, gen.PreviewReport500ApplicationProblemPlusJSONResponse{}
	}

	scope := blind.Scope{}
	if eng.Mode == storengagement.EngagementModeBlind {
		scope.Blind = true
		if subj, ok := authn.SubjectFrom(ctx); ok {
			memberships := identity.NewMemberships(h.store)
			list, err := memberships.ListByUser(ctx, subj.UserID)
			if err == nil {
				for _, m := range list {
					if m.EngagementID == engagementID {
						scope.Seat = authz.EngagementRole(m.Role)
						break
					}
				}
			}
		}
	}

	ev := true
	if includeEvidence != nil {
		ev = *includeEvidence
	}

	return &report.RenderEnv{
		EngagementID:       eng.ID,
		EngagementName:     eng.Name,
		EngagementClient:   eng.Client,
		EngagementStartsOn: eng.StartsOn,
		EngagementEndsOn:   eng.EndsOn,
		Branding:           branding,
		Analytics:          h.analytics,
		Domain: &report.DomainAdapter{
			Scenarios:  h.scenarios,
			Steps:      h.steps,
			Executions: h.executions,
			Findings:   h.findings,
			Evidence:   h.evidenceRepo,
		},
		Evidence:        &report.EvidenceStorage{Store: h.evidenceStore},
		IncludeEvidence: ev,
		BlindScope:      scope,
		Format:          report.FormatHelpers{},
	}, nil
}

// previewReportData fetches the report and its blocks.
func (h *handlers) previewReportData(ctx context.Context, reportID string) (*storereport.Report, []storereport.ReportBlock, gen.PreviewReportResponseObject) {
	rep, err := h.reports.Get(ctx, reportID)
	if err != nil {
		h.log.Error("preview: get report", "report_id", reportID, "err", err)
		return nil, nil, gen.PreviewReport404ApplicationProblemPlusJSONResponse{}
	}

	blocks, err := h.reports.Blocks(ctx, reportID)
	if err != nil {
		h.log.Error("preview: get blocks", "report_id", reportID, "err", err)
		return nil, nil, gen.PreviewReport500ApplicationProblemPlusJSONResponse{}
	}

	return &rep, blocks, nil
}
