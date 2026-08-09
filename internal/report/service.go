package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// Service is the report domain surface (M6-002). Construct with [New].
type Service struct {
	reports  *storereport.Reports
	registry *Registry
	activity *events.Log
}

// Deps is everything a Service needs.
type Deps struct {
	Reports  *storereport.Reports
	Registry *Registry
	Activity *events.Log // optional; nil skips durable activity rows
}

// New returns a Service over deps, or an error naming what is missing.
func New(deps Deps) (*Service, error) {
	var missing []string
	if deps.Reports == nil {
		missing = append(missing, "Reports")
	}
	if deps.Registry == nil {
		missing = append(missing, "Registry")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("report: missing dependencies: %s", strings.Join(missing, ", "))
	}
	return &Service{
		reports:  deps.Reports,
		registry: deps.Registry,
		activity: deps.Activity,
	}, nil
}

// CreateInput is the caller's half of creating a report draft.
type CreateInput struct {
	EngagementID string
	Title        string
	ActorID      string
}

// Create writes a new report draft and records activity.
func (s *Service) Create(ctx context.Context, in CreateInput) (storereport.Report, error) {
	in2 := storereport.NewReport{
		EngagementID: in.EngagementID,
		Title:        in.Title,
		CreatedBy:    in.ActorID,
	}

	rep, err := s.reports.Create(ctx, in2)
	if err != nil {
		return storereport.Report{}, fmt.Errorf("report: create: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: in.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportCreated,
			ObjectType:   events.ObjectReport,
			ObjectID:     rep.ID,
			Delta:        map[string]any{"title": in.Title},
		}); err != nil {
			return storereport.Report{}, fmt.Errorf("report: activity: %w", err)
		}
	}
	return rep, nil
}

// Get returns one report by id.
func (s *Service) Get(ctx context.Context, id string) (storereport.Report, error) {
	return s.reports.ByID(ctx, id)
}

// ListByEngagement returns every report in an engagement.
func (s *Service) ListByEngagement(ctx context.Context, engagementID string) ([]storereport.Report, error) {
	return s.reports.ListByEngagement(ctx, engagementID)
}

// UpdateInput is the caller's half of patching a report.
type UpdateInput struct {
	Title       *string
	ClientName  **string // nil=no change, *nil=clear, *s=set
	LogoBlobRef **string
	Colours     *json.RawMessage
	ActorID     string
}

// Update patches a report and records activity.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (storereport.Report, error) {
	rep, err := s.reports.ByID(ctx, id)
	if err != nil {
		return storereport.Report{}, err
	}
	// Validate per-report colour overrides (M6-004).
	if in.Colours != nil && len(*in.Colours) > 0 {
		if err := validateColoursJSON(*in.Colours); err != nil {
			return storereport.Report{}, err
		}
	}

	changes := storereport.ReportUpdate{
		Title:       in.Title,
		ClientName:  in.ClientName,
		LogoBlobRef: in.LogoBlobRef,
		Colours:     in.Colours,
		UpdatedBy:   in.ActorID,
	}

	rep, err = s.reports.Update(ctx, id, changes)
	if err != nil {
		return storereport.Report{}, fmt.Errorf("report: update: %w", err)
	}

	if s.activity != nil {
		delta := map[string]any{}
		if in.Title != nil {
			delta["title"] = *in.Title
		}
		if in.ClientName != nil {
			if *in.ClientName != nil {
				delta["clientName"] = **in.ClientName
			} else {
				delta["clientName"] = nil
			}
		}
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: rep.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportUpdated,
			ObjectType:   events.ObjectReport,
			ObjectID:     id,
			Delta:        delta,
		}); err != nil {
			return storereport.Report{}, fmt.Errorf("report: activity: %w", err)
		}
	}
	return rep, nil
}

// Delete removes a report and cascades to its blocks.
func (s *Service) Delete(ctx context.Context, id string, actorID string) error {
	rep, err := s.reports.ByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.reports.Delete(ctx, id); err != nil {
		return fmt.Errorf("report: delete: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: rep.EngagementID,
			ActorID:      actorID,
			Verb:         events.VerbReportDeleted,
			ObjectType:   events.ObjectReport,
			ObjectID:     id,
		}); err != nil {
			return fmt.Errorf("report: activity: %w", err)
		}
	}
	return nil
}

// ReplaceBlocksInput is the caller's half of replacing all blocks.
type ReplaceBlocksInput struct {
	ReportID string
	Blocks   []BlockInput
	ActorID  string
}

// BlockInput is one block in a replace request.
type BlockInput struct {
	BlockID string
	Params  json.RawMessage
}

// Soft limits from M6-002.
const (
	MaxBlocks      = 50
	MaxParamsBytes = 32 * 1024
)

// ReplaceBlocks validates and replaces every block in a report draft.
func (s *Service) ReplaceBlocks(ctx context.Context, in ReplaceBlocksInput) (storereport.Report, error) {
	if len(in.Blocks) > MaxBlocks {
		return storereport.Report{}, apierr.Validation(
			apierr.FieldError{Field: "blocks", Message: fmt.Sprintf("maximum %d blocks", MaxBlocks)},
		)
	}

	for i, bi := range in.Blocks {
		def, ok := s.registry.Get(ID(bi.BlockID))
		if !ok {
			return storereport.Report{}, apierr.Validation(
				apierr.FieldError{
					Field:   fmt.Sprintf("blocks[%d].blockId", i),
					Message: fmt.Sprintf("unknown block id: %q", bi.BlockID),
				},
			)
		}

		if _, err := ValidateParams(def.ParamsSchema, bi.Params); err != nil {
			return storereport.Report{}, apierr.Validation(
				apierr.FieldError{
					Field:   fmt.Sprintf("blocks[%d].params", i),
					Message: err.Error(),
				},
			)
		}

		if len(bi.Params) > MaxParamsBytes {
			return storereport.Report{}, apierr.Validation(
				apierr.FieldError{
					Field:   fmt.Sprintf("blocks[%d].params", i),
					Message: fmt.Sprintf("params exceeds %d bytes", MaxParamsBytes),
				},
			)
		}
		_ = def
	}

	newBlocks := make([]storereport.NewBlock, len(in.Blocks))
	for i, bi := range in.Blocks {
		def, _ := s.registry.Get(ID(bi.BlockID))
		params := applyDefaults(def.ParamsSchema, def.DefaultParams, bi.Params)
		newBlocks[i] = storereport.NewBlock{
			BlockID: bi.BlockID,
			Params:  params,
		}
	}

	blocks, err := s.reports.ReplaceBlocks(ctx, in.ReportID, newBlocks)
	if err != nil {
		return storereport.Report{}, fmt.Errorf("report: replace blocks: %w", err)
	}

	rep, err := s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return storereport.Report{}, err
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: rep.EngagementID,
			ActorID:      in.ActorID,
			Verb:         events.VerbReportUpdated,
			ObjectType:   events.ObjectReport,
			ObjectID:     in.ReportID,
			Delta:        map[string]any{"blockCount": len(blocks)},
		}); err != nil {
			return storereport.Report{}, fmt.Errorf("report: activity: %w", err)
		}
	}

	rep, err = s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return storereport.Report{}, err
	}
	return rep, nil
}

// Blocks returns the ordered blocks in a report.
func (s *Service) Blocks(ctx context.Context, reportID string) ([]storereport.ReportBlock, error) {
	return s.reports.BlocksByReport(ctx, reportID)
}

// applyDefaults merges block params with registry defaults.
func applyDefaults(schema ParamSchema, defaults json.RawMessage, provided json.RawMessage) json.RawMessage {
	merged := make(map[string]any)

	if len(defaults) > 0 {
		if err := json.Unmarshal(defaults, &merged); err != nil {
			return provided
		}
	}

	if len(provided) > 0 {
		var providedMap map[string]any
		if err := json.Unmarshal(provided, &providedMap); err != nil {
			return provided
		}
		for k, v := range providedMap {
			merged[k] = v
		}
	}

	result, err := json.Marshal(merged)
	if err != nil {
		return provided
	}
	return json.RawMessage(result)
}


// validateColoursJSON checks that a colours JSON object contains valid
// hex colour values. Both "primary" and "secondary" are optional; when
// present they must match #RRGGBB.
func validateColoursJSON(raw json.RawMessage) error {
	var c struct {
		Primary   string `json:"primary"`
		Secondary string `json:"secondary"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return apierr.Validation(apierr.FieldError{
			Field:   "colours",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
	}
	if c.Primary != "" && !isHexColor(c.Primary) {
		return apierr.Validation(apierr.FieldError{
			Field:   "colours.primary",
			Message: fmt.Sprintf("%q is not a valid hex colour (expected #RRGGBB)", c.Primary),
		})
	}
	if c.Secondary != "" && !isHexColor(c.Secondary) {
		return apierr.Validation(apierr.FieldError{
			Field:   "colours.secondary",
			Message: fmt.Sprintf("%q is not a valid hex colour (expected #RRGGBB)", c.Secondary),
		})
	}
	return nil
}