package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	storengagement "github.com/bryanster/blacklight/internal/store/engagement"

	"github.com/bryanster/blacklight/internal/report"
)

// FindingsDef is the Definition for the findings backlog block.
var FindingsDef = report.Definition{
	ID:          report.IDFindingsBacklog,
	Title:       "Findings Backlog",
	Description: "Table of open and in-progress findings with severity, status, and linked techniques.",
	ParamsSchema: report.ParamSchema{
		"includeResolved": report.ParamProperty{
			Type:        "boolean",
			Description: "Include resolved and accepted-risk findings.",
		},
	},
	DefaultParams: json.RawMessage(`{}`),
}

// FindingsRenderer renders the findings backlog block.
type FindingsRenderer struct{}

func (FindingsRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		IncludeResolved bool `json:"includeResolved"`
	}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("findings_backlog: params: %w", err)
		}
	}

	findings, err := env.Domain.ListFindings(ctx, env.EngagementID)
	if err != nil {
		return "", fmt.Errorf("findings_backlog: list findings: %w", err)
	}

	// Filter by status. Cache linked step technique IDs per finding.
	type findingRow struct {
		Finding    storengagement.Finding
		Techniques []string
	}
	var rows []findingRow
	for _, f := range findings {
		if !p.IncludeResolved {
			if f.Status == storengagement.FindingStatusResolved || f.Status == storengagement.FindingStatusAcceptedRisk {
				continue
			}
		}
		steps, err := env.Domain.FindingSteps(ctx, f.ID)
		if err != nil {
			return "", fmt.Errorf("findings_backlog: steps for %s: %w", f.ID, err)
		}
		var techs []string
		for _, s := range steps {
			t := s.TechniqueID
			if s.SubtechniqueID != "" {
				t = s.SubtechniqueID
			}
			techs = append(techs, t)
		}
		rows = append(rows, findingRow{Finding: f, Techniques: techs})
	}

	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Findings Backlog</h2>`)

	if len(rows) == 0 {
		b.WriteString(`<p class="bl-report__empty">No findings match the selected criteria.</p>`)
		b.WriteString(`</section>`)
		return report.HTMLFragment(b.String()), nil
	}

	b.WriteString(`<table class="bl-report__table">`)
	b.WriteString(`<thead><tr>`)
	b.WriteString(`<th>Severity</th>`)
	b.WriteString(`<th>Title</th>`)
	b.WriteString(`<th>Status</th>`)
	b.WriteString(`<th>Techniques</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for _, row := range rows {
		f := row.Finding
		statusLabel := string(f.Status)
		// Use same labels as product UI: snake_case → Title Case.
		statusLabel = findingStatusDisplay(statusLabel)

		techsStr := strings.Join(row.Techniques, ", ")
		if techsStr == "" {
			techsStr = "—"
		}

		b.WriteString(`<tr>`)
		b.WriteString(fmt.Sprintf(`<td class="bl-report__cell-sev bl-report__cell-sev--%s">%s</td>`,
			f.Severity, html.EscapeString(f.Severity)))
		b.WriteString(`<td>`)
		b.WriteString(html.EscapeString(f.Title))
		b.WriteString(`</td>`)
		b.WriteString(fmt.Sprintf(`<td><span class="bl-report__status bl-report__status--%s">%s</span></td>`,
			string(f.Status), html.EscapeString(statusLabel)))
		b.WriteString(`<td class="bl-report__cell-mono">`)
		b.WriteString(html.EscapeString(techsStr))
		b.WriteString(`</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)

	b.WriteString(`</section>`)
	return report.HTMLFragment(b.String()), nil
}

// findingStatusDisplay converts a finding status constant to a display label.
// Labels match the product UI and analytics.md vocabulary.
func findingStatusDisplay(s string) string {
	switch s {
	case "open":
		return "Open"
	case "in_progress":
		return "In Progress"
	case "resolved":
		return "Resolved"
	case "accepted_risk":
		return "Accepted Risk"
	default:
		return s
	}
}
