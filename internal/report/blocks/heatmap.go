package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/report"
)

// HeatmapDef is the Definition for the coverage heatmap block.
var HeatmapDef = report.Definition{
	ID:          report.IDCoverageHeatmap,
	Title:       "Coverage Heatmap",
	Description: "ATT&CK technique coverage heatmap with Navigator colour ramp.",
	ParamsSchema: report.ParamSchema{
		"verbosity": report.ParamProperty{
			Type:        "string",
			Description: "Detail level: summary (tactic level) or full (individual techniques).",
			Enum:        []string{"summary", "full"},
		},
	},
	DefaultParams:   json.RawMessage(`{"verbosity":"full"}`),
	AllowInTemplate: true,
}

// HeatmapRenderer renders the coverage heatmap block.
type HeatmapRenderer struct{}

func (HeatmapRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	params := struct {
		Verbosity string `json:"verbosity"`
	}{Verbosity: "full"}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &params); err != nil {
			return "", fmt.Errorf("coverage_heatmap: params: %w", err)
		}
	}

	scope := analytics.Scope{
		EngagementID: env.EngagementID,
		Blind:        env.BlindScope,
	}

	tc, err := env.Analytics.TechniqueCoverage(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("coverage_heatmap: technique coverage: %w", err)
	}

	tacticCov, err := env.Analytics.TacticCoverage(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("coverage_heatmap: tactic coverage: %w", err)
	}

	if tc.AttemptedTechniques == 0 && tc.NotAttemptedTechniques == 0 {
		return report.HTMLFragment(`<div class="bl-report__empty">No scored executions yet.</div>`), nil
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Coverage Heatmap</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	// Legend.
	b.WriteString(`<div class="bl-report__heatmap-legend"><span class="bl-report__heatmap-legend-label">Detection:</span>`)
	for i, label := range analytics.NavigatorLegendLabels {
		b.WriteString(fmt.Sprintf(`<span class="bl-report__heatmap-legend-item"><span class="bl-report__heatmap-swatch" style="background:%s"></span> %s</span>`,
			analytics.NavigatorColourRamp[i], html.EscapeString(label)))
	}
	b.WriteString(`</div>`)

	// Summary counts.
	b.WriteString(`<div class="bl-report__heatmap-summary">`)
	b.WriteString(fmt.Sprintf(`<p>Techniques attempted: <strong>%s</strong> / %s (%.0f%%)</p>`,
		f.Count(tc.AttemptedTechniques), f.Count(tc.MatrixTechniques),
		float64(tc.AttemptedTechniques)/float64(tc.MatrixTechniques)*100))
	b.WriteString(`</div>`)

	if params.Verbosity == "full" {
		// Per-tactic grid with technique cells.
		for _, tactic := range tacticCov.Rows {
			b.WriteString(fmt.Sprintf(`<h3 class="bl-report__heatmap-tactic">%s <span class="bl-report__heatmap-tactic-count">(%s / %s)</span></h3>`,
				html.EscapeString(tactic.TacticName),
				f.Count(tactic.TechniquesAttempted),
				f.Count(tactic.TechniquesInMatrix),
			))

			b.WriteString(`<div class="bl-report__heatmap-grid">`)
			for _, tech := range tc.Rows {
				if tech.BestCategoryOrdinal == nil {
					// Not attempted: grey.
					b.WriteString(fmt.Sprintf(`<div class="bl-report__heatmap-cell" style="background:%s" title="%s — not attempted">%s</div>`,
						"#e8eaed", html.EscapeString(tech.Name), html.EscapeString(tech.TechniqueID)))
				} else {
					c := *tech.BestCategoryOrdinal
					if c < 0 {
						c = 0
					}
					if c > 4 {
						c = 4
					}
					b.WriteString(fmt.Sprintf(`<div class="bl-report__heatmap-cell" style="background:%s;color:%s" title="%s — %s">%s</div>`,
						analytics.NavigatorColourRamp[c],
						heatmapTextColor(c),
						html.EscapeString(tech.Name),
						html.EscapeString(tech.BestCategory),
						html.EscapeString(tech.TechniqueID)))
				}
			}
			b.WriteString(`</div>`)
		}
	} else {
		// Summary: per-tactic bar showing attempted/matrix.
		b.WriteString(`<table class="bl-report__table bl-report__heatmap-table">`)
		b.WriteString(`<thead><tr><th>Tactic</th><th>Attempted</th><th>In Matrix</th><th>Coverage</th></tr></thead><tbody>`)
		for _, tactic := range tacticCov.Rows {
			pct := ""
			if tactic.TechniquesInMatrix > 0 {
				pct = fmt.Sprintf("%.0f%%", float64(tactic.TechniquesAttempted)/float64(tactic.TechniquesInMatrix)*100)
			}
			b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				html.EscapeString(tactic.TacticName),
				f.Count(tactic.TechniquesAttempted),
				f.Count(tactic.TechniquesInMatrix),
				pct,
			))
		}
		b.WriteString(`</tbody></table>`)
	}

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// heatmapTextColor returns an appropriate text color (white or dark) for a
// heatmap cell, based on the luminance of the ramp colour at the given ordinal.
func heatmapTextColor(ordinal int) string {
	if ordinal >= 2 {
		return "#fff"
	}
	return "#333"
}
