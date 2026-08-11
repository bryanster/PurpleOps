package blocks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/sanitize"
)

// SummaryDef is the Definition for the executive summary block.
var SummaryDef = report.Definition{
	ID:          report.IDExecutiveSummary,
	Title:       "Executive Summary",
	Description: "Narrative executive summary of the assessment.",
	ParamsSchema: report.ParamSchema{
		"body": report.ParamProperty{
			Type:        "string",
			Description: "Executive summary content (rich text HTML).",
		},
	},
	DefaultParams:   json.RawMessage(`{}`),
	AllowInTemplate: true,
	HTMLParamKeys:   []string{"body"},
}

// SummaryRenderer renders the executive summary block.
type SummaryRenderer struct{}

func (SummaryRenderer) Render(_ context.Context, _ report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	body := ""
	if len(inst.Params) > 0 {
		var p struct {
			Body string `json:"body,omitempty"`
		}
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("executive_summary: params: %w", err)
		}
		body = p.Body
	}

	// Defense in depth: sanitize at render even if write already sanitized.
	body = sanitize.Sanitize(body)

	// Empty body: omit the section entirely (clean output).
	// Draft preview labels are the UI's responsibility (M6-013/014).
	if body == "" {
		return report.HTMLFragment(""), nil
	}

	return report.HTMLFragment(
		`<section class="bl-report__section">` +
			`<h2 class="bl-report__section-title">Executive Summary</h2>` +
			`<div class="bl-report__section-body">` + body + `</div>` +
			`</section>`), nil
}
