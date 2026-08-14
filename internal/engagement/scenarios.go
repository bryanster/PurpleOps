package engagement

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// CreateScenarioInput is the caller's half of creating a scenario.
type CreateScenarioInput struct {
	EngagementID string
	Name         string
	Narrative    string
	ThreatActor  string
	Source       storengagement.ScenarioSource
	SourceRef    string
}

func (in *CreateScenarioInput) defaults() {
	if !in.Source.Valid() || in.Source == "" {
		in.Source = storengagement.ScenarioSourceManual
	}
}

// CreateScenario writes a new scenario into an engagement. The ordinal is
// assigned as the next dense position. Closed/archived engagements are refused.
func (s *Service) CreateScenario(ctx context.Context, actor authn.Subject, in CreateScenarioInput) (storengagement.Scenario, error) {
	in.defaults()

	eng, err := s.engagements.ByID(ctx, in.EngagementID)
	if err != nil {
		return storengagement.Scenario{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Scenario{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	ord, err := s.scenarios.NextOrdinal(ctx, in.EngagementID)
	if err != nil {
		return storengagement.Scenario{}, fmt.Errorf("scenario: next ordinal: %w", err)
	}

	scenario, err := s.scenarios.Create(ctx, storengagement.NewScenario{
		EngagementID: in.EngagementID,
		Ordinal:      ord,
		Name:         in.Name,
		Narrative:    in.Narrative,
		Source:       in.Source,
		ThreatActor:  in.ThreatActor,
		SourceRef:    in.SourceRef,
	})
	if err != nil {
		return storengagement.Scenario{}, fmt.Errorf("scenario: create: %w", err)
	}
	s.recordActivity(ctx, actor.UserID, in.EngagementID,
		events.VerbScenarioCreated, events.ObjectScenario, scenario.ID,
		map[string]any{"name": in.Name},
	)
	return scenario, nil
}

// GetScenario returns one scenario by id.
func (s *Service) GetScenario(ctx context.Context, id string) (storengagement.Scenario, error) {
	return s.scenarios.ByID(ctx, id)
}

// GetScenarioInEngagement returns one scenario, 404 unless it belongs to the
// authorized engagement. The raw GetScenario remains for callers walking a
// parent chain rather than naming a path engagement (M7-012).
func (s *Service) GetScenarioInEngagement(ctx context.Context, engagementID, scenarioID string) (storengagement.Scenario, error) {
	scenario, err := s.scenarios.ByID(ctx, scenarioID)
	if err != nil {
		return storengagement.Scenario{}, err
	}
	if err := requireSameEngagement("scenario", scenarioID, scenario.EngagementID, engagementID); err != nil {
		return storengagement.Scenario{}, err
	}
	return scenario, nil
}

// ListScenarios returns every scenario in an engagement, ordered by ordinal.
func (s *Service) ListScenarios(ctx context.Context, engagementID string) ([]storengagement.Scenario, error) {
	return s.scenarios.ListByEngagement(ctx, engagementID)
}

// PatchScenarioInput is the caller's half of patching a scenario.
type PatchScenarioInput struct {
	Name        *string
	Narrative   *string
	ThreatActor *string
}

// PatchScenario updates one scenario's fields. Closed/archived engagements
// are refused. Only non-nil fields are changed.
func (s *Service) PatchScenario(ctx context.Context, actor authn.Subject, engagementID, id string, in PatchScenarioInput) (storengagement.Scenario, error) {
	current, err := s.scenarios.ByID(ctx, id)
	if err != nil {
		return storengagement.Scenario{}, err
	}
	if err := requireSameEngagement("scenario", id, current.EngagementID, engagementID); err != nil {
		return storengagement.Scenario{}, err
	}

	eng, err := s.engagements.ByID(ctx, current.EngagementID)
	if err != nil {
		return storengagement.Scenario{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Scenario{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	changes := storengagement.ScenarioUpdateChanges{
		Name:        current.Name,
		Narrative:   current.Narrative,
		ThreatActor: current.ThreatActor,
	}
	delta := map[string]any{}

	if in.Name != nil {
		changes.Name = *in.Name
		delta["name"] = *in.Name
	}
	if in.Narrative != nil {
		changes.Narrative = *in.Narrative
		delta["narrative"] = "[changed]"
	}
	if in.ThreatActor != nil {
		changes.ThreatActor = *in.ThreatActor
		delta["threat_actor"] = *in.ThreatActor
	}

	after := storengagement.After(func(ctx context.Context, tx *sql.Tx) error {
		return s.recordActivityTx(ctx, tx, actor.UserID, current.EngagementID,
			events.VerbScenarioUpdated, events.ObjectScenario, id, delta,
		)
	})

	scenario, err := s.scenarios.Update(ctx, id, changes, after)
	if err != nil {
		return storengagement.Scenario{}, fmt.Errorf("scenario: patch: %w", err)
	}
	return scenario, nil
}

// DeleteScenario removes a scenario and its child graph (steps, executions,
// comments, evidence, finding_step links). Closed/archived engagements are
// refused.
func (s *Service) DeleteScenario(ctx context.Context, actor authn.Subject, engagementID, id string) error {
	current, err := s.scenarios.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := requireSameEngagement("scenario", id, current.EngagementID, engagementID); err != nil {
		return err
	}

	eng, err := s.engagements.ByID(ctx, current.EngagementID)
	if err != nil {
		return err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	if err := s.scenarios.Delete(ctx, id); err != nil {
		return fmt.Errorf("scenario: delete: %w", err)
	}

	delta := map[string]any{"name": current.Name, "ordinal": current.Ordinal}
	s.recordActivity(ctx, actor.UserID, current.EngagementID,
		events.VerbScenarioDeleted, events.ObjectScenario, id, delta,
	)
	return nil
}

// ReorderScenarios reassigns ordinals 1..N to match the requested order.
// Every scenario in the engagement must appear exactly once.
// Closed/archived engagements are refused.
func (s *Service) ReorderScenarios(ctx context.Context, actor authn.Subject, engagementID string, ids []string) ([]storengagement.Scenario, error) {
	eng, err := s.engagements.ByID(ctx, engagementID)
	if err != nil {
		return nil, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return nil, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	existing, err := s.scenarios.ListByEngagement(ctx, engagementID)
	if err != nil {
		return nil, fmt.Errorf("scenario: reorder: list: %w", err)
	}

	if len(ids) != len(existing) {
		return nil, apierr.Conflict(fmt.Sprintf("must include every scenario in the engagement (%d provided, %d exist)", len(ids), len(existing)))
	}

	present := make(map[string]bool, len(existing))
	for _, sc := range existing {
		present[sc.ID] = true
	}
	for _, id := range ids {
		if !present[id] {
			return nil, apierr.Conflict(fmt.Sprintf("scenario %q does not belong to engagement %s", id, engagementID))
		}
	}

	if err := s.scenarios.Reorder(ctx, ids); err != nil {
		return nil, fmt.Errorf("scenario: reorder: %w", err)
	}

	delta := map[string]any{"order": ids}
	s.recordActivity(ctx, actor.UserID, engagementID,
		events.VerbScenarioReordered, events.ObjectScenario, engagementID, delta,
	)

	return s.scenarios.ListByEngagement(ctx, engagementID)
}

// recordActivity writes an activity entry for a scenario change, or is a
// no-op when activity recording is disabled.
func (s *Service) recordActivity(ctx context.Context, actorID, engagementID string, verb events.Verb, objectType, objectID string, delta map[string]any) {
	if s.activity == nil {
		return
	}
	entry := events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   objectType,
		ObjectID:     objectID,
	}
	if delta != nil {
		entry.Delta = events.Delta(delta)
	}
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	s.activity.RecordAlone(ctx, entry)
}

// recordActivityTx is recordActivity for a [storengagement.After] hook, which
// runs inside the mutation's own write transaction. See recordActivityStepTx
// for why an After hook must not record through RecordAlone.
func (s *Service) recordActivityTx(ctx context.Context, tx *sql.Tx, actorID, engagementID string,
	verb events.Verb, objectType, objectID string, delta map[string]any) error {
	if s.activity == nil {
		return nil
	}
	entry := events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   objectType,
		ObjectID:     objectID,
	}
	if delta != nil {
		entry.Delta = events.Delta(delta)
	}
	return s.activity.Record(ctx, tx, entry)
}
