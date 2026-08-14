package engagement

import (
	"context"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// requireSameEngagement returns apierr.NotFound when a child resource's owning
// engagement (got) is not the authorized path engagement (want). It is the
// single place a nested child id is bound to its parent, so a handler cannot
// copy a per-route "if got != want" wrong. A child id from another engagement
// must be indistinguishable from a missing id, exactly like a non-member's 404
// (M7-012).
func requireSameEngagement(resource, id, got, want string) error {
	if got != want {
		return apierr.NotFound(resource, id)
	}
	return nil
}

// ScenarioEngagementID resolves a scenario id to its owning engagement id.
func (s *Service) ScenarioEngagementID(ctx context.Context, scenarioID string) (string, error) {
	sc, err := s.scenarios.ByID(ctx, scenarioID)
	if err != nil {
		return "", err
	}
	return sc.EngagementID, nil
}

// StepEngagementID resolves a step id to its owning engagement id, walking
// step -> scenario -> engagement.
func (s *Service) StepEngagementID(ctx context.Context, stepID string) (string, error) {
	step, err := s.steps.ByID(ctx, stepID)
	if err != nil {
		return "", err
	}
	return s.ScenarioEngagementID(ctx, step.ScenarioID)
}

// ExecutionEngagementID resolves an execution id to its owning engagement id,
// walking execution -> step -> scenario -> engagement.
func (s *Service) ExecutionEngagementID(ctx context.Context, executionID string) (string, error) {
	exec, err := s.executions.ByID(ctx, executionID)
	if err != nil {
		return "", err
	}
	return s.StepEngagementID(ctx, exec.StepID)
}

// CommentEngagementID resolves a comment id to its owning engagement id,
// walking comment -> execution -> step -> scenario -> engagement.
func (s *Service) CommentEngagementID(ctx context.Context, commentID string) (string, error) {
	c, err := s.comments.ByID(ctx, commentID)
	if err != nil {
		return "", err
	}
	return s.ExecutionEngagementID(ctx, c.ExecutionID)
}
