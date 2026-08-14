package report

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// TemplateService is the template domain surface (M6-003).
type TemplateService struct {
	templates *storereport.Templates
	reports   *storereport.Reports
	registry  *Registry
	activity  *events.Log
}

// TemplateDeps is everything a TemplateService needs.
type TemplateDeps struct {
	Templates *storereport.Templates
	Reports   *storereport.Reports
	Registry  *Registry
	Activity  *events.Log // optional; nil skips durable activity rows
}

// NewTemplateService returns a TemplateService over deps, or an error naming what is missing.
func NewTemplateService(deps TemplateDeps) (*TemplateService, error) {
	missing := missingTemplateDeps(deps)
	if len(missing) > 0 {
		return nil, fmt.Errorf("report templates: missing dependencies: %v", missing)
	}
	return &TemplateService{
		templates: deps.Templates,
		reports:   deps.Reports,
		registry:  deps.Registry,
		activity:  deps.Activity,
	}, nil
}

func missingTemplateDeps(deps TemplateDeps) []string {
	var m []string
	if deps.Templates == nil {
		m = append(m, "Templates")
	}
	if deps.Reports == nil {
		m = append(m, "Reports")
	}
	if deps.Registry == nil {
		m = append(m, "Registry")
	}
	return m
}

// Soft limits from M6-003.
const MaxTemplatesPerEngagement = 20

// CreateTemplateInput is the caller's half of creating a template.
type CreateTemplateInput struct {
	EngagementID string
	Name         string
	ActorID      string
}

// CreateTemplate writes a new template (empty blocks) and records activity.
func (s *TemplateService) CreateTemplate(ctx context.Context, in CreateTemplateInput) (storereport.Template, error) {
	count, err := s.countByEngagement(ctx, in.EngagementID)
	if err != nil {
		return storereport.Template{}, err
	}
	if count >= MaxTemplatesPerEngagement {
		return storereport.Template{}, apierr.Validation(
			apierr.FieldError{Field: "engagementId", Message: fmt.Sprintf("maximum %d templates per engagement", MaxTemplatesPerEngagement)},
		)
	}

	tmpl, err := s.templates.Create(ctx, storereport.NewTemplate{
		EngagementID: in.EngagementID,
		Name:         in.Name,
		CreatedBy:    in.ActorID,
	})
	if err != nil {
		return storereport.Template{}, fmt.Errorf("report templates: create: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: in.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportTemplateCreated,
			ObjectType:   events.ObjectReportTemplate,
			ObjectID:     tmpl.ID,
			Delta:        map[string]any{"name": in.Name},
		}); err != nil {
			return storereport.Template{}, fmt.Errorf("report templates: activity: %w", err)
		}
	}
	return tmpl, nil
}

// GetTemplate returns one template by id, 404 unless it belongs to the
// authorized engagement (M7-012).
func (s *TemplateService) GetTemplate(ctx context.Context, engagementID, id string) (storereport.Template, error) {
	tmpl, err := s.templates.ByID(ctx, id)
	if err != nil {
		return storereport.Template{}, err
	}
	if err := requireSameEngagement("template", id, tmpl.EngagementID, engagementID); err != nil {
		return storereport.Template{}, err
	}
	return tmpl, nil
}

// ListTemplates returns every template in an engagement.
func (s *TemplateService) ListTemplates(ctx context.Context, engagementID string) ([]storereport.Template, error) {
	return s.templates.ListByEngagement(ctx, engagementID)
}

// UpdateTemplateInput is the caller's half of patching a template.
type UpdateTemplateInput struct {
	Name    *string
	ActorID string
}

// UpdateTemplate patches a template and records activity.
func (s *TemplateService) UpdateTemplate(ctx context.Context, engagementID, id string, in UpdateTemplateInput) (storereport.Template, error) {
	current, err := s.templates.ByID(ctx, id)
	if err != nil {
		return storereport.Template{}, err
	}
	if err := requireSameEngagement("template", id, current.EngagementID, engagementID); err != nil {
		return storereport.Template{}, err
	}
	tmpl, err := s.templates.Update(ctx, id, storereport.TemplateUpdate{
		Name: in.Name,
	})
	if err != nil {
		return storereport.Template{}, fmt.Errorf("report templates: update: %w", err)
	}

	if s.activity != nil {
		delta := map[string]any{}
		if in.Name != nil {
			delta["name"] = *in.Name
		}
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: tmpl.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportTemplateUpdated,
			ObjectType:   events.ObjectReportTemplate,
			ObjectID:     id,
			Delta:        delta,
		}); err != nil {
			return storereport.Template{}, fmt.Errorf("report templates: activity: %w", err)
		}
	}
	return tmpl, nil
}

// DeleteTemplate removes a template, cascading to its blocks, and records activity.
func (s *TemplateService) DeleteTemplate(ctx context.Context, engagementID, id string, actorID string) error {
	tmpl, err := s.templates.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := requireSameEngagement("template", id, tmpl.EngagementID, engagementID); err != nil {
		return err
	}

	if err := s.templates.Delete(ctx, id); err != nil {
		return fmt.Errorf("report templates: delete: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: tmpl.EngagementID,
			ActorID:      actorID,
			Verb:         events.VerbReportTemplateDeleted,
			ObjectType:   events.ObjectReportTemplate,
			ObjectID:     id,
		}); err != nil {
			return fmt.Errorf("report templates: activity: %w", err)
		}
	}
	return nil
}

// TemplateBlocks returns the ordered blocks in a template.
func (s *TemplateService) TemplateBlocks(ctx context.Context, templateID string) ([]storereport.TemplateBlock, error) {
	return s.templates.BlocksByTemplate(ctx, templateID)
}

// ---------------------------------------------------------------------------
// Apply template to report
// ---------------------------------------------------------------------------

// ApplyTemplateInput is the caller's half of applying a template to a report.
type ApplyTemplateInput struct {
	ReportID     string
	TemplateID   string
	EngagementID string
	ActorID      string
}

// ApplyTemplate atomically replaces a report's draft blocks with a deep copy
// of the template's blocks. Title and branding are left alone.
func (s *TemplateService) ApplyTemplate(ctx context.Context, in ApplyTemplateInput) (storereport.Report, error) {
	report, err := s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return storereport.Report{}, err
	}
	if err := requireSameEngagement("report", in.ReportID, report.EngagementID, in.EngagementID); err != nil {
		return storereport.Report{}, err
	}
	tmpl, err := s.templates.ByID(ctx, in.TemplateID)
	if err != nil {
		return storereport.Report{}, err
	}
	if err := requireSameEngagement("template", in.TemplateID, tmpl.EngagementID, in.EngagementID); err != nil {
		return storereport.Report{}, err
	}

	tmplBlocks, err := s.templates.BlocksByTemplate(ctx, in.TemplateID)
	if err != nil {
		return storereport.Report{}, fmt.Errorf("report templates: apply: read template blocks: %w", err)
	}

	newBlocks := make([]storereport.NewBlock, len(tmplBlocks))
	for i, tb := range tmplBlocks {
		params := make(json.RawMessage, len(tb.Params))
		copy(params, tb.Params)
		newBlocks[i] = storereport.NewBlock{
			BlockID: tb.BlockID,
			Params:  params,
		}
	}

	if _, err := s.reports.ReplaceBlocks(ctx, in.ReportID, newBlocks); err != nil {
		return storereport.Report{}, fmt.Errorf("report templates: apply: replace blocks: %w", err)
	}

	rep, err := s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return storereport.Report{}, err
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: rep.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportTemplateApplied,
			ObjectType:   events.ObjectReport,
			ObjectID:     in.ReportID,
			Delta:        map[string]any{"templateId": in.TemplateID, "blockCount": len(newBlocks)},
		}); err != nil {
			return storereport.Report{}, fmt.Errorf("report templates: activity: %w", err)
		}
	}

	return rep, nil
}

// ---------------------------------------------------------------------------
// Create template from report
// ---------------------------------------------------------------------------

// CreateFromReportInput is the caller's half of creating a template from a report.
type CreateFromReportInput struct {
	EngagementID string
	ReportID     string
	Name         string
	ActorID      string
}

// CreateFromReport snapshots a report's current draft blocks into a new template.
func (s *TemplateService) CreateFromReport(ctx context.Context, in CreateFromReportInput) (storereport.Template, error) {
	src, err := s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return storereport.Template{}, err
	}
	if err := requireSameEngagement("report", in.ReportID, src.EngagementID, in.EngagementID); err != nil {
		return storereport.Template{}, err
	}

	count, err := s.countByEngagement(ctx, in.EngagementID)
	if err != nil {
		return storereport.Template{}, err
	}
	if count >= MaxTemplatesPerEngagement {
		return storereport.Template{}, apierr.Validation(
			apierr.FieldError{Field: "engagementId", Message: fmt.Sprintf("maximum %d templates per engagement", MaxTemplatesPerEngagement)},
		)
	}

	reportBlocks, err := s.reports.BlocksByReport(ctx, in.ReportID)
	if err != nil {
		return storereport.Template{}, fmt.Errorf("report templates: from-report: read blocks: %w", err)
	}

	tmpl, err := s.templates.Create(ctx, storereport.NewTemplate{
		EngagementID: in.EngagementID,
		Name:         in.Name,
		CreatedBy:    in.ActorID,
	})
	if err != nil {
		return storereport.Template{}, fmt.Errorf("report templates: from-report: create: %w", err)
	}

	tmplBlocks := make([]storereport.NewTemplateBlock, len(reportBlocks))
	for i, rb := range reportBlocks {
		params := make(json.RawMessage, len(rb.Params))
		copy(params, rb.Params)
		tmplBlocks[i] = storereport.NewTemplateBlock{
			BlockID: rb.BlockID,
			Params:  params,
		}
	}

	if _, err := s.templates.ReplaceBlocks(ctx, tmpl.ID, tmplBlocks); err != nil {
		_ = s.templates.Delete(ctx, tmpl.ID) //nolint:errcheck
		return storereport.Template{}, fmt.Errorf("report templates: from-report: replace blocks: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: in.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportTemplateCreated,
			ObjectType:   events.ObjectReportTemplate,
			ObjectID:     tmpl.ID,
			Delta:        map[string]any{"name": in.Name, "fromReportId": in.ReportID, "blockCount": len(tmplBlocks)},
		}); err != nil {
			return storereport.Template{}, fmt.Errorf("report templates: activity: %w", err)
		}
	}

	return tmpl, nil
}

// countByEngagement returns the number of templates in an engagement.
func (s *TemplateService) countByEngagement(ctx context.Context, engagementID string) (int, error) {
	tmpls, err := s.templates.ListByEngagement(ctx, engagementID)
	if err != nil {
		return 0, err
	}
	return len(tmpls), nil
}
