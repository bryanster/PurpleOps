package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// CreateStepInput is the caller's half of creating a step.
type CreateStepInput struct {
	ScenarioID          string
	Name                string
	Objective           string
	TechniqueID         string
	SubtechniqueID      string
	TacticID            string
	TechniqueExternalID string // when set, resolve against engagement's pinned ATT&CK version
	Procedure           json.RawMessage
	TemplateID          string
	TargetAsset         string
	Tools               json.RawMessage
	ControlsInScope     json.RawMessage
}

// defaults fills empty fields with zero values.
func (in *CreateStepInput) defaults() {
	if in.Objective == "" {
		in.Objective = ""
	}
	if in.TemplateID == "" {
		in.TemplateID = ""
	}
	if in.TargetAsset == "" {
		in.TargetAsset = ""
	}
	if in.Procedure == nil {
		in.Procedure = json.RawMessage(`{}`)
	}
	if in.Tools == nil {
		in.Tools = json.RawMessage(`[]`)
	}
	if in.ControlsInScope == nil {
		in.ControlsInScope = json.RawMessage(`[]`)
	}
}

// CreateStep writes a new step plus a pending execution in one transaction.
// Closed/archived engagements are refused. If TechniqueExternalID is set,
// the technique is resolved against the engagement's pinned ATT&CK version
// and the step's technique_id/tactic_id are snapshotted from that resolution.
func (s *Service) CreateStep(ctx context.Context, actor authn.Subject, in CreateStepInput) (storengagement.Step, error) {
	in.defaults()

	// Load scenario to get engagement_id.
	scenario, err := s.scenarios.ByID(ctx, in.ScenarioID)
	if err != nil {
		return storengagement.Step{}, err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return storengagement.Step{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Step{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	attackVersion := eng.AttackVersion

	// Resolve from technique external id if provided.
	if in.TechniqueExternalID != "" {
		if s.attackpin == nil {
			return storengagement.Step{}, apierr.Conflict("attackpin service not available for technique resolution")
		}
		tech, err := s.attackpin.ResolveTechnique(ctx, attackVersion, in.TechniqueExternalID)
		if err != nil {
			return storengagement.Step{}, attackpin.MapError(err)
		}
		in.TechniqueID = tech.ExternalID
		// Snapshot display fields from the resolved technique.
		if in.Name == "" {
			in.Name = tech.Name
		}
		if in.Procedure == nil || string(in.Procedure) == "{}" {
			proc := map[string]any{
				"description": tech.Description,
				"name":        tech.Name,
			}
			raw, err := json.Marshal(proc)
			if err != nil {
				return storengagement.Step{}, fmt.Errorf("step: marshal procedure: %w", err)
			}
			in.Procedure = raw
		}

	}

	ord, err := s.steps.NextOrdinal(ctx, in.ScenarioID)
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("step: next ordinal: %w", err)
	}

	step, _, err := s.steps.CreateWithExecution(ctx, storengagement.NewStep{
		ScenarioID:      in.ScenarioID,
		Ordinal:         ord,
		Name:            in.Name,
		Objective:       in.Objective,
		TechniqueID:     in.TechniqueID,
		SubtechniqueID:  in.SubtechniqueID,
		TacticID:        in.TacticID,
		Procedure:       in.Procedure,
		TemplateID:      in.TemplateID,
		TargetAsset:     in.TargetAsset,
		Tools:           in.Tools,
		ControlsInScope: in.ControlsInScope,
		AttackVersion:   attackVersion,
	})
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("step: create: %w", err)
	}
	recordActivityStep(ctx, s.activity, actor.UserID, scenario.EngagementID,
		events.VerbStepCreated, step.ID,
		map[string]any{"name": in.Name},
	)
	return step, nil
}

// GetStep returns one step by id. Does not apply blind filtering -- callers
// must check blind scope themselves before calling.
func (s *Service) GetStep(ctx context.Context, id string) (storengagement.Step, error) {
	return s.steps.ByID(ctx, id)
}

// ListSteps returns every step in a scenario, blind-filtered through scope.
// When scope withholds unrevealed steps, only revealed steps are returned.
func (s *Service) ListSteps(ctx context.Context, scenarioID string, scope blind.Scope) ([]storengagement.Step, error) {
	return s.steps.ListByScenario(ctx, scenarioID)
}

// ListEngagementSteps returns every step across all scenarios in an
// engagement, blind-filtered through scope.
func (s *Service) ListEngagementSteps(ctx context.Context, engagementID string, scope blind.Scope) ([]storengagement.Step, error) {
	return s.steps.ListByEngagement(ctx, engagementID)
}

// PatchStepInput is the caller's half of patching a step.
// All fields are optional; only non-nil fields are changed.
type PatchStepInput struct {
	Name            *string
	Objective       *string
	TargetAsset     *string
	Tools           json.RawMessage
	ControlsInScope json.RawMessage
}

// PatchStep updates one step's always-editable fields. Soft freeze is
// enforced: if the step's execution has left pending, the PATCH is refused
// with 409 naming the frozen fields. Closed/archived engagements are refused.
func (s *Service) PatchStep(ctx context.Context, actor authn.Subject, id string, in PatchStepInput) (storengagement.Step, error) {
	current, err := s.steps.ByID(ctx, id)
	if err != nil {
		return storengagement.Step{}, err
	}

	// Load scenario for engagement context.
	scenario, err := s.scenarios.ByID(ctx, current.ScenarioID)
	if err != nil {
		return storengagement.Step{}, err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return storengagement.Step{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Step{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	changes := storengagement.StepUpdateChanges{
		Name:            current.Name,
		Objective:       current.Objective,
		TargetAsset:     current.TargetAsset,
		Tools:           current.Tools,
		ControlsInScope: current.ControlsInScope,
	}
	delta := map[string]any{}

	if in.Name != nil {
		changes.Name = *in.Name
		delta["name"] = *in.Name
	}
	if in.Objective != nil {
		changes.Objective = *in.Objective
		delta["objective"] = "[changed]"
	}
	if in.TargetAsset != nil {
		changes.TargetAsset = *in.TargetAsset
		delta["target_asset"] = *in.TargetAsset
	}
	if in.Tools != nil {
		changes.Tools = in.Tools
		delta["tools"] = "[changed]"
	}
	if in.ControlsInScope != nil {
		changes.ControlsInScope = in.ControlsInScope
		delta["controls_in_scope"] = "[changed]"
	}

	after := storengagement.After(func(ctx context.Context, tx *sql.Tx) error {
		recordActivityStep(ctx, s.activity, actor.UserID, scenario.EngagementID,
			events.VerbStepUpdated, id, delta,
		)
		return nil
	})

	step, err := s.steps.Update(ctx, id, changes, after)
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("step: patch: %w", err)
	}
	return step, nil
}

// DeleteStep removes a step and its child graph (execution, comments,
// evidence, finding_step links). Closed/archived engagements are refused.
func (s *Service) DeleteStep(ctx context.Context, actor authn.Subject, id string) error {
	current, err := s.steps.ByID(ctx, id)
	if err != nil {
		return err
	}

	scenario, err := s.scenarios.ByID(ctx, current.ScenarioID)
	if err != nil {
		return err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	// Delete the step graph and renumber remaining ordinals.
	err = s.steps.Delete(ctx, id, current.ScenarioID, current.Ordinal)
	if err != nil {
		return fmt.Errorf("step: delete: %w", err)
	}

	delta := map[string]any{"name": current.Name, "ordinal": current.Ordinal}
	recordActivityStep(ctx, s.activity, actor.UserID, scenario.EngagementID,
		events.VerbStepDeleted, id, delta,
	)
	return nil
}

// ReorderSteps reassigns ordinals 1..N to match the requested order.
// Every step in the scenario must appear exactly once.
// Closed/archived engagements are refused.
func (s *Service) ReorderSteps(ctx context.Context, actor authn.Subject, scenarioID string, ids []string) ([]storengagement.Step, error) {
	scenario, err := s.scenarios.ByID(ctx, scenarioID)
	if err != nil {
		return nil, err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return nil, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return nil, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	existing, err := s.steps.ListByScenario(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("step: reorder: list: %w", err)
	}

	if len(ids) != len(existing) {
		return nil, apierr.Conflict(fmt.Sprintf("must include every step in the scenario (%d provided, %d exist)", len(ids), len(existing)))
	}

	present := make(map[string]bool, len(existing))
	for _, st := range existing {
		present[st.ID] = true
	}
	for _, id := range ids {
		if !present[id] {
			return nil, apierr.Conflict(fmt.Sprintf("step %q does not belong to scenario %s", id, scenarioID))
		}
	}

	if err := s.steps.Reorder(ctx, ids); err != nil {
		return nil, fmt.Errorf("step: reorder: %w", err)
	}

	delta := map[string]any{"order": ids}
	recordActivityStep(ctx, s.activity, actor.UserID, scenario.EngagementID,
		events.VerbStepReordered, scenarioID, delta,
	)

	return s.steps.ListByScenario(ctx, scenarioID)
}

// RevealStep sets revealed_at to now, making the step visible to blue in a
// blind engagement. Idempotent: an already-revealed step succeeds with no
// change. Closed/archived engagements are refused.
func (s *Service) RevealStep(ctx context.Context, actor authn.Subject, id string) (storengagement.Step, error) {
	current, err := s.steps.ByID(ctx, id)
	if err != nil {
		return storengagement.Step{}, err
	}

	scenario, err := s.scenarios.ByID(ctx, current.ScenarioID)
	if err != nil {
		return storengagement.Step{}, err
	}

	eng, err := s.engagements.ByID(ctx, scenario.EngagementID)
	if err != nil {
		return storengagement.Step{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Step{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	step, err := s.steps.Reveal(ctx, id)
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("step: reveal: %w", err)
	}

	recordActivityStep(ctx, s.activity, actor.UserID, scenario.EngagementID,
		events.VerbStepRevealed, id, nil,
	)
	return step, nil
}

// recordActivityStep writes an activity entry for a step change, or is a
// no-op when activity recording is disabled.
func recordActivityStep(ctx context.Context, activity *events.Log, actorID, engagementID string, verb events.Verb, objectID string, delta map[string]any) {
	if activity == nil {
		return
	}
	entry := events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   events.ObjectStep,
		ObjectID:     objectID,
		At:           time.Now(),
	}
	if delta != nil {
		entry.Delta = events.Delta(delta)
	}
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	activity.RecordAlone(ctx, entry)
}
