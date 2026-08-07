package engagement

import (
	"context"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// PatchRedExecutionInput is the caller's half of patching a red execution.
type PatchRedExecutionInput struct {
	Version    int
	Status     *storengagement.ExecutionStatus
	StartedAt  *time.Time
	EndedAt    *time.Time
	CommandRun *string
	SourceHost *string
	TargetHost *string
	RedNotes   *string
}

// validStatusTransitionsExecution is the execution status state machine.
var validStatusTransitionsExecution = map[storengagement.ExecutionStatus][]storengagement.ExecutionStatus{
	storengagement.ExecutionStatusPending: {
		storengagement.ExecutionStatusRunning,
		storengagement.ExecutionStatusSkipped,
		storengagement.ExecutionStatusBlocked,
	},
	storengagement.ExecutionStatusRunning: {
		storengagement.ExecutionStatusComplete,
		storengagement.ExecutionStatusBlocked,
		storengagement.ExecutionStatusSkipped,
	},
	// Terminal states (complete, blocked, skipped): no transitions allowed,
	// but note/host/command edits are permitted without a status change.
}

// PatchRedExecution applies red-side changes to an execution with optimistic
// locking, status transition validation, auto-reveal and activity logging.
func (s *Service) PatchRedExecution(ctx context.Context, actor authn.Subject, id string, in PatchRedExecutionInput) (storengagement.Execution, error) {
	// Load the execution and its parent chain.
	exec, err := s.executions.ByID(ctx, id)
	if err != nil {
		return storengagement.Execution{}, err
	}

	step, err := s.steps.ByID(ctx, exec.StepID)
	if err != nil {
		return storengagement.Execution{}, err
	}

	scenario, err := s.scenarios.ByID(ctx, step.ScenarioID)
	if err != nil {
		return storengagement.Execution{}, err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return storengagement.Execution{}, err
	}

	// Closed/archived engagements refuse all writes.
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Execution{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	// Build store changes.
	changes := storengagement.RedPatchChanges{
		Status:     in.Status,
		StartedAt:  in.StartedAt,
		EndedAt:    in.EndedAt,
		CommandRun: in.CommandRun,
		SourceHost: in.SourceHost,
		TargetHost: in.TargetHost,
		RedNotes:   in.RedNotes,
	}

	// Validate status transition.
	if in.Status != nil {
		newStatus := *in.Status
		// Same status is always valid (terminal-state note/host/command edits).
		if newStatus != exec.Status {
			allowed, ok := validStatusTransitionsExecution[exec.Status]
			if !ok {
				return storengagement.Execution{}, apierr.Conflict(
					fmt.Sprintf("illegal status transition: %s is a terminal state", exec.Status))
			}
			found := false
			for _, s := range allowed {
				if s == newStatus {
					found = true
					break
				}
			}
			if !found {
				return storengagement.Execution{}, apierr.Conflict(
					fmt.Sprintf("illegal status transition from %s to %s", exec.Status, newStatus))
			}

			// Set executed_by on first non-pending transition.
			if exec.ExecutedBy == "" && exec.Status == storengagement.ExecutionStatusPending {
				executedBy := actor.UserID
				changes.ExecutedBy = &executedBy
			}

			// Auto-reveal: when status becomes running or complete, engagement
			// is blind with auto_reveal_on_start, and step is unrevealed.
			if (newStatus == storengagement.ExecutionStatusRunning || newStatus == storengagement.ExecutionStatusComplete) &&
				eng.Mode == storengagement.EngagementModeBlind &&
				eng.AutoRevealOnStart &&
				step.RevealedAt == nil {
				if _, err := s.steps.Reveal(ctx, step.ID); err != nil {
					return storengagement.Execution{}, fmt.Errorf("execution: auto-reveal step %q: %w", step.ID, err)
				}
			}
		}
	}

	// Server-set started_at if transitioning to running and no time given.
	if in.Status != nil && *in.Status == storengagement.ExecutionStatusRunning && in.StartedAt == nil && exec.StartedAt == nil {
		now := time.Now()
		changes.StartedAt = &now
	}

	// Apply the patch with optimistic locking.
	patched, err := s.executions.PatchRed(ctx, id, in.Version, changes)
	if err != nil {
		return storengagement.Execution{}, err
	}

	// Record activity with a delta of changed fields.
	recordActivityExecution(ctx, s.activity, actor.UserID, scenario.EngagementID,
		events.VerbExecutionRedUpdated, id, redDelta(in, exec, patched),
	)

	return patched, nil
}

// redDelta builds a delta map of what changed in this red PATCH.
func redDelta(in PatchRedExecutionInput, before, after storengagement.Execution) map[string]any {
	d := map[string]any{}
	if in.Status != nil {
		d["status"] = map[string]any{"from": string(before.Status), "to": string(after.Status)}
	}
	if in.CommandRun != nil {
		d["commandRun"] = true
	}
	if in.SourceHost != nil {
		d["sourceHost"] = true
	}
	if in.TargetHost != nil {
		d["targetHost"] = true
	}
	if in.StartedAt != nil {
		d["startedAt"] = true
	}
	if in.EndedAt != nil {
		d["endedAt"] = true
	}
	if in.RedNotes != nil {
		d["redNotes"] = true
	}
	return d
}

// GetExecution returns one execution by id. Callers must apply blind filtering
// themselves before calling (the execution belongs to a step that may be
// unrevealed).
func (s *Service) GetExecution(ctx context.Context, id string) (storengagement.Execution, error) {
	return s.executions.ByID(ctx, id)
}

// ListEngagementExecutions returns executions in an engagement, optionally
// filtered by scenario and/or status. Callers apply blind filtering.
func (s *Service) ListEngagementExecutions(ctx context.Context, engagementID string, scenarioID *string, status *storengagement.ExecutionStatus) ([]storengagement.Execution, error) {
	return s.executions.ListByEngagement(ctx, engagementID, scenarioID, status)
}

// recordActivityExecution writes an activity entry for an execution change,
// or is a no-op when activity recording is disabled.
func recordActivityExecution(ctx context.Context, activity *events.Log, actorID, engagementID string, verb events.Verb, objectID string, delta map[string]any) {
	if activity == nil {
		return
	}
	entry := events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   events.ObjectExecution,
		ObjectID:     objectID,
		At:           time.Now(),
	}
	if delta != nil {
		entry.Delta = events.Delta(delta)
	}
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	activity.RecordAlone(ctx, entry)
}
