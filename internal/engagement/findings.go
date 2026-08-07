package engagement

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// CreateFindingInput is the caller's half of raising a finding.
type CreateFindingInput struct {
	EngagementID         string
	Title                string
	Description          string
	Severity             string
	Recommendation       string
	Owner                string
	Status               string
	CreatedFromExecution string
}

// UpdateFindingInput is the caller's half of patching a finding.
type UpdateFindingInput struct {
	Title          string
	Description    string
	Severity       string
	Recommendation string
	Owner          string
	Status         string
}

// CreateFinding raises a new finding in an engagement. Authorisation
// (finding.write) is handled by the authz middleware. Activity: finding.created.
func (s *Service) CreateFinding(ctx context.Context, actor authn.Subject, in CreateFindingInput) (storengagement.Finding, error) {
	if in.Title == "" {
		return storengagement.Finding{}, fmt.Errorf("engagement: finding title is required")
	}
	if in.EngagementID == "" {
		return storengagement.Finding{}, fmt.Errorf("engagement: finding engagement id is required")
	}

	// Check engagement is not closed.
	eng, err := s.engagements.ByID(ctx, in.EngagementID)
	if err != nil {
		return storengagement.Finding{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Finding{}, fmt.Errorf("engagement: cannot create findings on closed/archived engagements")
	}

	if in.Owner == "" {
		in.Owner = actor.UserID
	}
	if in.Severity == "" {
		in.Severity = "medium"
	}
	if in.Status == "" {
		in.Status = string(storengagement.FindingStatusOpen)
	}

	finding, err := s.findings.Create(ctx, storengagement.NewFinding{
		EngagementID:         in.EngagementID,
		Title:                in.Title,
		Description:          in.Description,
		Severity:             in.Severity,
		Recommendation:       in.Recommendation,
		Owner:                in.Owner,
		CreatedFromExecution: in.CreatedFromExecution,
	})
	if err != nil {
		return storengagement.Finding{}, err
	}

	recordFindingActivity(ctx, s.activity, actor.UserID, finding.ID, events.VerbFindingCreated, nil)

	return finding, nil
}

// UpdateFinding patches a finding. Only non-empty fields are applied.
// Closed engagements may only update status to resolved.
func (s *Service) UpdateFinding(ctx context.Context, actor authn.Subject, id string, in UpdateFindingInput) (storengagement.Finding, error) {
	existing, err := s.findings.ByID(ctx, id)
	if err != nil {
		return storengagement.Finding{}, err
	}

	// Check engagement closedness: on closed, only status transitions allowed.
	eng, err := s.engagements.ByID(ctx, existing.EngagementID)
	if err != nil {
		return storengagement.Finding{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		// On closed/archived, only allow status changes.
		if in.Title != "" || in.Description != "" || in.Severity != "" || in.Recommendation != "" || in.Owner != "" {
			return storengagement.Finding{}, fmt.Errorf("engagement: only status may be changed on closed/archived engagements")
		}
	}

	finding, err := s.findings.Update(ctx, id, storengagement.PatchFinding{
		Title:          in.Title,
		Description:    in.Description,
		Severity:       in.Severity,
		Recommendation: in.Recommendation,
		Owner:          in.Owner,
		Status:         in.Status,
	})
	if err != nil {
		return storengagement.Finding{}, err
	}

	delta := events.Delta(map[string]any{
		"title":          in.Title,
		"description":    in.Description,
		"severity":       in.Severity,
		"recommendation": in.Recommendation,
		"owner":          in.Owner,
		"status":         in.Status,
	})
	recordFindingActivity(ctx, s.activity, actor.UserID, finding.ID, events.VerbFindingUpdated, delta)

	return finding, nil
}

// DeleteFinding removes a finding and its step join rows.
func (s *Service) DeleteFinding(ctx context.Context, actor authn.Subject, id string) error {
	if err := s.findings.Delete(ctx, id); err != nil {
		return err
	}

	recordFindingActivity(ctx, s.activity, actor.UserID, id, events.VerbFindingDeleted, nil)
	return nil
}

// GetFinding returns a finding by id.
func (s *Service) GetFinding(ctx context.Context, id string) (storengagement.Finding, error) {
	return s.findings.ByID(ctx, id)
}

// ListFindings returns every finding in an engagement, newest first.
func (s *Service) ListFindings(ctx context.Context, engagementID string) ([]storengagement.Finding, error) {
	return s.findings.ListByEngagement(ctx, engagementID)
}

// SetFindingSteps replaces the step set for a finding. All step ids must
// belong to the same engagement as the finding.
func (s *Service) SetFindingSteps(ctx context.Context, actor authn.Subject, findingID string, stepIDs []string) error {
	finding, err := s.findings.ByID(ctx, findingID)
	if err != nil {
		return err
	}

	// Validate all steps belong to the same engagement.
	for _, stepID := range stepIDs {
		step, err := s.steps.ByID(ctx, stepID)
		if err != nil {
			return fmt.Errorf("engagement: step %q: %w", stepID, err)
		}
		scenario, err := s.scenarios.ByID(ctx, step.ScenarioID)
		if err != nil {
			return fmt.Errorf("engagement: scenario for step %q: %w", stepID, err)
		}
		if scenario.EngagementID != finding.EngagementID {
			return fmt.Errorf("engagement: step %q belongs to engagement %q, not %q",
				stepID, scenario.EngagementID, finding.EngagementID)
		}
	}

	if err := s.findings.SetSteps(ctx, findingID, stepIDs); err != nil {
		return err
	}

	recordFindingActivity(ctx, s.activity, actor.UserID, findingID, events.VerbFindingStepsChanged, nil)
	return nil
}

// FindingSteps returns the steps linked to a finding.
func (s *Service) FindingSteps(ctx context.Context, findingID string) ([]storengagement.Step, error) {
	return s.findings.Steps(ctx, findingID)
}

// recordFindingActivity writes an activity entry for a finding change,
// or is a no-op when activity recording is disabled.
func recordFindingActivity(ctx context.Context, log *events.Log, actorID, objectID string, verb events.Verb, delta map[string]any) {
	if log == nil {
		return
	}
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	log.RecordAlone(ctx, events.Entry{
		ActorID:    actorID,
		Verb:       verb,
		ObjectType: events.ObjectFinding,
		ObjectID:   objectID,
		Delta:      delta,
	})
}
