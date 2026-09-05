package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	storengagement "github.com/bryanster/blacklight/internal/store/engagement"

	"github.com/bryanster/blacklight/internal/domain/scoring"
	"github.com/bryanster/blacklight/internal/report"
)

// WalkthroughDef is the Definition for the scenario walkthrough block.
var WalkthroughDef = report.Definition{
	ID:          report.IDScenarioWalkthrough,
	Title:       "Scenario Walkthrough",
	Description: "Per-scenario listing of steps, execution status, outcome, and detection detail.",
	ParamsSchema: report.ParamSchema{
		"scenarioIds": report.ParamProperty{
			Type:        "array",
			Description: "Optional list of scenario IDs to include; all when omitted.",
		},
		"verbosity": report.ParamProperty{
			Type:        "string",
			Description: "Detail level: summary or full.",
			Enum:        []string{"summary", "full"},
		},
	},
	DefaultParams: json.RawMessage(`{"verbosity":"summary"}`),
}

// WalkthroughRenderer renders the scenario walkthrough block.
type WalkthroughRenderer struct{}

func (WalkthroughRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		ScenarioIDs []string `json:"scenarioIds"`
		Verbosity   string   `json:"verbosity"`
	}
	p.Verbosity = "summary"
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("scenario_walkthrough: params: %w", err)
		}
	}
	if p.Verbosity != "summary" && p.Verbosity != "full" {
		p.Verbosity = "summary"
	}
	full := p.Verbosity == "full"

	scenarioSet := make(map[string]bool)
	if len(p.ScenarioIDs) > 0 {
		for _, id := range p.ScenarioIDs {
			scenarioSet[id] = true
		}
	}

	scenarios, err := env.Domain.ListScenarios(ctx, env.EngagementID)
	if err != nil {
		return "", fmt.Errorf("scenario_walkthrough: list scenarios: %w", err)
	}

	steps, err := env.Domain.ListSteps(ctx, env.EngagementID, env.BlindScope)
	if err != nil {
		return "", fmt.Errorf("scenario_walkthrough: list steps: %w", err)
	}

	executions, err := env.Domain.ListExecutions(ctx, env.EngagementID)
	if err != nil {
		return "", fmt.Errorf("scenario_walkthrough: list executions: %w", err)
	}

	// Index executions by step ID.
	execByStep := make(map[string]storengagement.Execution, len(executions))
	for _, e := range executions {
		execByStep[e.StepID] = e
	}

	// Index steps by scenario ID.
	stepsByScenario := make(map[string][]storengagement.Step)
	for _, s := range steps {
		stepsByScenario[s.ScenarioID] = append(stepsByScenario[s.ScenarioID], s)
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Scenario Walkthrough</h2>`)

	if len(scenarios) == 0 {
		b.WriteString(`<p class="bl-report__empty">No scenarios defined.</p>`)
		b.WriteString(`</section>`)
		return report.HTMLFragment(b.String()), nil
	}

	for _, sc := range scenarios {
		if len(scenarioSet) > 0 && !scenarioSet[sc.ID] {
			continue
		}
		scSteps := stepsByScenario[sc.ID]

		b.WriteString(`<div class="bl-report__walkthrough-scenario">`)
		b.WriteString(`<h3 class="bl-report__walkthrough-scenario-name">`)
		b.WriteString(html.EscapeString(sc.Name))
		b.WriteString(`</h3>`)
		if sc.Narrative != "" {
			b.WriteString(`<p class="bl-report__walkthrough-narrative">`)
			b.WriteString(html.EscapeString(sc.Narrative))
			b.WriteString(`</p>`)
		}

		if len(scSteps) == 0 {
			b.WriteString(`<p class="bl-report__empty">No steps in this scenario.</p>`)
			b.WriteString(`</div>`)
			continue
		}

		b.WriteString(`<table class="bl-report__table">`)
		b.WriteString(`<thead><tr>`)
		b.WriteString(`<th>#</th>`)
		b.WriteString(`<th>Step</th>`)
		b.WriteString(`<th>Technique</th>`)
		b.WriteString(`<th>Status</th>`)
		b.WriteString(`<th>Outcome</th>`)
		b.WriteString(`<th>Detection</th>`)
		b.WriteString(`<th>Protection</th>`)
		if full {
			b.WriteString(`<th>Notes</th>`)
		}
		b.WriteString(`</tr></thead><tbody>`)

		for _, step := range scSteps {
			exec, hasExec := execByStep[step.ID]

			technique := step.TechniqueID
			if step.SubtechniqueID != "" {
				technique = step.SubtechniqueID
			}

			status := "—"
			if hasExec {
				status = string(exec.Status)
			}

			outcomeLabel := "—"
			detCat := "—"
			prot := "—"
			if hasExec && exec.DetectionCategory != nil && exec.Protection != nil {
				cat := scoring.Category(*exec.DetectionCategory)
				pr := scoring.Protection(*exec.Protection)
				outcome, err := scoring.DeriveOutcome(cat, pr)
				if err == nil {
					outcomeLabel = string(outcome)
				}
				detCat = string(*exec.DetectionCategory)
				prot = string(*exec.Protection)
			} else if hasExec && exec.DetectionCategory != nil {
				detCat = string(*exec.DetectionCategory)
			} else if hasExec && exec.Protection != nil {
				prot = string(*exec.Protection)
			}

			b.WriteString(`<tr>`)
			fmt.Fprintf(&b, `<td class="bl-report__cell-num">%d</td>`, step.Ordinal+1)
			b.WriteString(`<td>`)
			b.WriteString(html.EscapeString(step.Name))
			b.WriteString(`</td>`)
			b.WriteString(`<td class="bl-report__cell-mono">`)
			b.WriteString(html.EscapeString(technique))
			b.WriteString(`</td>`)
			fmt.Fprintf(&b, `<td><span class="bl-report__status bl-report__status--%s">%s</span></td>`,
				status, html.EscapeString(status))
			fmt.Fprintf(&b, `<td><span class="bl-report__outcome bl-report__outcome--%s">%s</span></td>`,
				outcomeLabel, html.EscapeString(outcomeLabel))
			b.WriteString(`<td>`)
			b.WriteString(html.EscapeString(detCat))
			b.WriteString(`</td>`)
			b.WriteString(`<td>`)
			b.WriteString(html.EscapeString(prot))
			b.WriteString(`</td>`)
			if full {
				b.WriteString(`<td class="bl-report__cell-notes">`)
				if hasExec {
					if exec.RedNotes != "" {
						b.WriteString(`<p><strong>Red:</strong> `)
						b.WriteString(html.EscapeString(exec.RedNotes))
						b.WriteString(`</p>`)
					}
					if exec.BlueNotes != "" {
						b.WriteString(`<p><strong>Blue:</strong> `)
						b.WriteString(html.EscapeString(exec.BlueNotes))
						b.WriteString(`</p>`)
					}
				}
				b.WriteString(`</td>`)
			}
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</tbody></table>`)
		b.WriteString(`</div>`)

		// Scenario summary counts.
		attempted, detected, prevented, total := 0, 0, 0, len(scSteps)
		for _, step := range scSteps {
			exec, hasExec := execByStep[step.ID]
			if !hasExec {
				continue
			}
			if exec.Status == storengagement.ExecutionStatusComplete || exec.Status == storengagement.ExecutionStatusBlocked {
				attempted++
			}
			if exec.DetectionCategory != nil && *exec.DetectionCategory != storengagement.DetectionCategoryNone {
				detected++
			}
			if exec.Protection != nil && *exec.Protection == storengagement.ProtectionBlocked {
				prevented++
			}
		}
		b.WriteString(`<div class="bl-report__walkthrough-summary">`)
		fmt.Fprintf(&b, `<p>%s steps total, %s attempted, %s detected, %s prevented</p>`,
			f.Count(total), f.Count(attempted), f.Count(detected), f.Count(prevented))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</section>`)
	return report.HTMLFragment(b.String()), nil
}
