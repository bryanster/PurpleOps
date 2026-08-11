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

// ScorecardDef is the Definition for the tactic scorecard block.
var ScorecardDef = report.Definition{
	ID:              report.IDTacticScorecard,
	Title:           "Tactic Scorecard",
	Description:     "Per-tactic coverage scorecard with dual denominators and category distribution.",
	DefaultParams:   json.RawMessage(`{}`),
	AllowInTemplate: true,
}

// ScorecardRenderer renders the tactic scorecard block.
type ScorecardRenderer struct{}

func (ScorecardRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	scope := analytics.Scope{
		EngagementID: env.EngagementID,
		Blind:        env.BlindScope,
	}

	tacticCov, err := env.Analytics.TacticCoverage(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("tactic_scorecard: tactic coverage: %w", err)
	}

	if len(tacticCov.Rows) == 0 {
		return report.HTMLFragment(`<div class="bl-report__empty">No scored executions yet.</div>`), nil
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Tactic Scorecard</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	// Note about dual denominators.
	b.WriteString(`<p class="bl-report__note">Techniques attempted against the pinned ATT&amp;CK matrix. A technique in multiple tactics counts in each.</p>`)

	// Per-tactic cards.
	for _, tactic := range tacticCov.Rows {
		b.WriteString(`<div class="bl-report__scorecard-tactic">`)
		b.WriteString(fmt.Sprintf(`<h3 class="bl-report__scorecard-tactic-name">%s</h3>`, html.EscapeString(tactic.TacticName)))

		b.WriteString(`<div class="bl-report__scorecard-metrics">`)
		b.WriteString(fmt.Sprintf(`<div class="bl-report__scorecard-metric"><span class="bl-report__scorecard-metric-label">Attempted</span><span class="bl-report__scorecard-metric-value">%s</span></div>`,
			f.Count(tactic.TechniquesAttempted)))
		b.WriteString(fmt.Sprintf(`<div class="bl-report__scorecard-metric"><span class="bl-report__scorecard-metric-label">In Matrix</span><span class="bl-report__scorecard-metric-value">%s</span></div>`,
			f.Count(tactic.TechniquesInMatrix)))
		b.WriteString(fmt.Sprintf(`<div class="bl-report__scorecard-metric"><span class="bl-report__scorecard-metric-label">Coverage</span><span class="bl-report__scorecard-metric-value">%s</span></div>`,
			f.Percent(tactic.TechniquesAttempted, tactic.TechniquesInMatrix)))
		b.WriteString(`</div>`)

		// Category distribution within this tactic.
		if len(tactic.CategoryDistribution) > 0 {
			b.WriteString(`<div class="bl-report__scorecard-distribution">`)
			for _, cat := range scorecardCategoryOrder {
				if count, ok := tactic.CategoryDistribution[cat]; ok && count > 0 {
					b.WriteString(fmt.Sprintf(`<span class="bl-report__scorecard-dist-item">%s: %s</span>`,
						html.EscapeString(cat), f.Count(count)))
				}
			}
			b.WriteString(`</div>`)
		}

		b.WriteString(`</div>`)
	}

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// scorecardCategoryOrder is the display order for detection categories.
var scorecardCategoryOrder = []string{"none", "telemetry", "general", "tactic", "technique"}
