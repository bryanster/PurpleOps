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

// DistributionDef is the Definition for the detection distribution block.
var DistributionDef = report.Definition{
	ID:             report.IDDetectionDistribution,
	Title:          "Detection Distribution",
	Description:    "Detection category distribution, protection rate, and outcome mix for the engagement.",
	DefaultParams:  json.RawMessage(`{}`),
	AllowInTemplate: true,
}

// DistributionRenderer renders the detection distribution block.
type DistributionRenderer struct{}

func (DistributionRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	scope := analytics.Scope{
		EngagementID: env.EngagementID,
		Blind:        env.BlindScope,
	}

	catDist, err := env.Analytics.CategoryDistribution(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("detection_distribution: category distribution: %w", err)
	}

	protRate, err := env.Analytics.ProtectionRate(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("detection_distribution: protection rate: %w", err)
	}

	outcome, err := env.Analytics.OutcomeMix(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("detection_distribution: outcome mix: %w", err)
	}

	if catDist.Attempted == 0 {
		return report.HTMLFragment(`<div class="bl-report__empty">No scored executions yet.</div>`), nil
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Detection Distribution</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	b.WriteString(fmt.Sprintf(`<p class="bl-report__note">Based on %s attempted executions.</p>`, f.Count(catDist.Attempted)))

	// Category distribution.
	writeDistTable(&b, "Detection Category", catDist, f)

	// Protection rate.
	writeDistTable(&b, "Protection Rate", protRate, f)

	// Outcome mix.
	writeDistTable(&b, "Outcome", outcome, f)

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// writeDistTable emits an HTML table for a DistributionResult.
func writeDistTable(b *strings.Builder, title string, d *analytics.DistributionResult, f report.FormatHelpers) {
	b.WriteString(fmt.Sprintf(`<h3 class="bl-report__dist-title">%s</h3>`, html.EscapeString(title)))
	b.WriteString(`<table class="bl-report__table bl-report__dist-table"><thead><tr><th>Bucket</th><th>Count</th><th>% of Attempted</th></tr></thead><tbody>`)

	for _, bucket := range d.Buckets {
		b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td class="bl-report__num">%s</td><td class="bl-report__num">%s</td></tr>`,
			html.EscapeString(bucket.Label),
			f.Count(bucket.Count),
			f.Percent(bucket.Count, d.Attempted),
		))
	}

	b.WriteString(`</tbody></table>`)
}
