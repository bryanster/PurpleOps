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

// CompareDef is the Definition for the engagement compare block.
var CompareDef = report.Definition{
	ID:          report.IDEngagementCompare,
	Title:       "Engagement Comparison",
	Description: "Cross-engagement technique-by-technique comparison — improved, regressed, added, removed.",
	ParamsSchema: report.ParamSchema{
		"baselineEngagementId": report.ParamProperty{
			Type:        "string",
			Description: "Baseline engagement ID to compare against (required).",
		},
	},
	DefaultParams:  json.RawMessage(`{}`),
	AllowInTemplate: true,
}

// CompareRenderer renders the engagement compare block.
type CompareRenderer struct{}

func (CompareRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		BaselineEngagementID string `json:"baselineEngagementId"`
	}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("engagement_compare: params: %w", err)
		}
	}

	if p.BaselineEngagementID == "" {
		return "", fmt.Errorf("engagement_compare: baselineEngagementId is required")
	}

	// Construct compare scope using the same blind scope for both sides.
	// The assembler (M6-009) is responsible for setting the correct scopes.
	compScope := analytics.CompareScope{
		Baseline: analytics.Scope{
			EngagementID: p.BaselineEngagementID,
			Blind:        env.BlindScope,
		},
		Current: analytics.Scope{
			EngagementID: env.EngagementID,
			Blind:        env.BlindScope,
		},
	}

	result, err := env.Analytics.Compare(ctx, compScope)
	if err != nil {
		// Draft preview: return a clear fragment error inline.
		return report.HTMLFragment(fmt.Sprintf(
			`<div class="bl-report__error">Comparison unavailable: baseline engagement %s is not accessible or does not exist.</div>`,
			html.EscapeString(p.BaselineEngagementID),
		)), nil
	}

	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Engagement Comparison</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	// Pin mismatch advisory.
	if result.PinMismatch != nil {
		b.WriteString(fmt.Sprintf(`<p class="bl-report__compare-mismatch">Note: engagements pin different ATT&amp;CK versions (baseline: %s, current: %s). Comparison may be incomplete.</p>`,
			html.EscapeString(result.PinMismatch.Baseline),
			html.EscapeString(result.PinMismatch.Current),
		))
	}

	// Summary counts.
	b.WriteString(`<div class="bl-report__compare-summary">`)
	writeCompareSummaryItem(&b, "Improved", result.Improved, "#16a34a")
	writeCompareSummaryItem(&b, "Regressed", result.Regressed, "#d13c3c")
	writeCompareSummaryItem(&b, "Unchanged", result.Unchanged, "#6b7280")
	writeCompareSummaryItem(&b, "Added", result.NewlyAttempted, "#2563eb")
	writeCompareSummaryItem(&b, "Removed", result.NoLongerAttempted, "#9333ea")
	b.WriteString(`</div>`)

	// Detail table.
	if len(result.Rows) > 0 {
		b.WriteString(`<table class="bl-report__table bl-report__compare-table">`)
		b.WriteString(`<thead><tr><th>ID</th><th>Technique</th><th>Classification</th><th>Baseline</th><th>Current</th></tr></thead><tbody>`)

		for _, row := range result.Rows {
			cls := html.EscapeString(row.Classification)
			baselineCat := "—"
			if row.BaselineCategory != "" {
				baselineCat = row.BaselineCategory
			}
			currentCat := "—"
			if row.CurrentCategory != "" {
				currentCat = row.CurrentCategory
			}

			b.WriteString(fmt.Sprintf(`<tr class="bl-report__compare-row-%s"><td class="bl-report__mono">%s</td><td>%s</td><td class="bl-report__compare-classification">%s</td><td>%s</td><td>%s</td></tr>`,
				cls,
				html.EscapeString(row.TechniqueID),
				html.EscapeString(row.Name),
				cls,
				html.EscapeString(baselineCat),
				html.EscapeString(currentCat),
			))
		}

		b.WriteString(`</tbody></table>`)
	}

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// writeCompareSummaryItem emits one summary metric.
func writeCompareSummaryItem(b *strings.Builder, label string, count int, color string) {
	b.WriteString(fmt.Sprintf(`<div class="bl-report__compare-metric"><span class="bl-report__compare-metric-dot" style="background:%s"></span>%s: <strong>%d</strong></div>`,
		color, label, count))
}
