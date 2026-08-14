package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// CreateStepFromTemplateInput is the caller's half of creating a step from a
// content procedure template. The template is already resolved from the content
// store by the handler (copy-on-use).
type CreateStepFromTemplateInput struct {
	EngagementID string
	ScenarioID   string
	Template     storecontent.ProcedureTemplate
	Name         string // override; empty = use template name
	Objective    string // override; empty = use template description
	TargetAsset  string // override
	ArgValues    map[string]string
}

// CreateStepFromTemplate snapshots a procedure template into a new step plus a
// pending execution. Closed/archived engagements are refused. A disabled
// source is already refused by the handler (ByIDEnabled check) before this
// method is called. Template technique ids are resolved against the
// engagement's pinned ATT&CK version.
func (s *Service) CreateStepFromTemplate(ctx context.Context, actor authn.Subject, in CreateStepFromTemplateInput) (storengagement.Step, error) {
	eng, err := s.engagements.ByID(ctx, in.EngagementID)
	if err != nil {
		return storengagement.Step{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return storengagement.Step{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	scenario, err := s.scenarios.ByID(ctx, in.ScenarioID)
	if err != nil {
		return storengagement.Step{}, err
	}
	if err := requireSameEngagement("scenario", in.ScenarioID, scenario.EngagementID, in.EngagementID); err != nil {
		return storengagement.Step{}, err
	}

	attackVersion := eng.AttackVersion
	tmpl := in.Template

	// Resolve technique ids from the template.
	var techIDs []string
	if len(tmpl.TechniqueExternalIDs) > 0 {
		if err := json.Unmarshal(tmpl.TechniqueExternalIDs, &techIDs); err != nil {
			return storengagement.Step{}, fmt.Errorf("from_template: unmarshal technique ids: %w", err)
		}
	}

	var techniqueID, subtechniqueID, tacticID string
	if len(techIDs) > 0 {
		if s.attackpin == nil {
			return storengagement.Step{}, apierr.Conflict("attackpin service not available for technique resolution")
		}
		if err := s.attackpin.AssertPinned(ctx, attackVersion); err != nil {
			return storengagement.Step{}, attackpin.MapError(err)
		}

		// Resolve the first technique for the step's identity fields.
		tech, err := s.attackpin.ResolveTechnique(ctx, attackVersion, techIDs[0])
		if err != nil {
			return storengagement.Step{}, attackpin.MapError(err)
		}
		if tech.IsSubtechnique {
			techniqueID = tech.ParentExternalID
			subtechniqueID = tech.ExternalID
		} else {
			techniqueID = tech.ExternalID
		}

		// If name override not provided, use the resolved technique name.
		if in.Name == "" && tech.Name != "" {
			in.Name = tech.Name
		}
	}

	// Apply field overrides.
	name := in.Name
	if name == "" {
		name = tmpl.Name
	}
	objective := in.Objective
	if objective == "" {
		objective = tmpl.Description
	}

	// Build the procedure snapshot from template fields — preserve structure.
	procedure := buildProcedureSnapshot(tmpl, in.ArgValues, techIDs)

	// Build tools list from platforms.
	tools := json.RawMessage(`[]`)
	if len(tmpl.Platforms) > 0 {
		tools = tmpl.Platforms
	}

	ord, err := s.steps.NextOrdinal(ctx, in.ScenarioID)
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("from_template: next ordinal: %w", err)
	}

	after := storengagement.After(func(ctx context.Context, tx *sql.Tx) error {
		id := storengagement.AfterEntityID(ctx)
		return recordActivityStepTx(ctx, s.activity, tx, actor.UserID, in.EngagementID,
			events.VerbStepCreated, id,
			map[string]any{
				"name":            name,
				"template_id":     tmpl.ID,
				"template_name":   tmpl.Name,
				"template_source": tmpl.SourceID,
				"technique_ids":   techIDs,
				"attack_version":  attackVersion,
			},
		)
	})

	step, _, err := s.steps.CreateWithExecution(ctx, storengagement.NewStep{
		ScenarioID:      in.ScenarioID,
		Ordinal:         ord,
		Name:            name,
		Objective:       objective,
		TechniqueID:     techniqueID,
		SubtechniqueID:  subtechniqueID,
		TacticID:        tacticID,
		Procedure:       procedure,
		TemplateID:      tmpl.ID,
		TargetAsset:     in.TargetAsset,
		Tools:           tools,
		ControlsInScope: json.RawMessage(`[]`),
		AttackVersion:   attackVersion,
	}, after)
	if err != nil {
		return storengagement.Step{}, fmt.Errorf("from_template: create step: %w", err)
	}

	return step, nil
}

// buildProcedureSnapshot produces the step.procedure JSON from a template,
// applying arg value substitution.
func buildProcedureSnapshot(tmpl storecontent.ProcedureTemplate, argValues map[string]string, resolvedTechIDs []string) json.RawMessage {
	proc := map[string]any{
		"platforms":              tmpl.Platforms,
		"executor":               tmpl.Executor,
		"elevationRequired":      tmpl.ElevationRequired,
		"dependencyExecutorName": tmpl.DependencyExecutorName,
	}

	// Substitute #{key} placeholders in command and cleanup.
	command := substituteArgs(tmpl.Command, argValues)
	cleanup := substituteArgs(tmpl.Cleanup, argValues)
	proc["command"] = command
	proc["cleanup"] = cleanup

	// Input args as-is.
	if len(tmpl.InputArgs) > 0 && string(tmpl.InputArgs) != "null" {
		proc["inputArgs"] = tmpl.InputArgs
	}
	if len(argValues) > 0 {
		proc["argValues"] = argValues
	}

	// Resolved technique external ids.
	if len(resolvedTechIDs) > 0 {
		proc["techniqueExternalIds"] = resolvedTechIDs
	}

	// Dependency metadata.
	if tmpl.Dependencies != "" {
		proc["dependencies"] = json.RawMessage(tmpl.Dependencies)
	}

	raw, err := json.Marshal(proc)
	if err != nil {
		// Should never happen with map[string]any, but guard.
		return json.RawMessage("{}")
	}
	return raw
}

var argPlaceholder = regexp.MustCompile(`#\{([^}]+)\}`)

// substituteArgs replaces #{key} placeholders in s with values from m.
// Keys without a provided value are left as-is.
func substituteArgs(s string, m map[string]string) string {
	if s == "" || len(m) == 0 {
		return s
	}
	return argPlaceholder.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1] // strip #{ and }
		if val, ok := m[key]; ok {
			return val
		}
		return match // leave unresolved
	})
}

// Ensure strings import is used.
var _ = strings.TrimSpace
