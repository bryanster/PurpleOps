package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/report"
)

// MTTDDef is the Definition for the MTTD block.
var MTTDDef = report.Definition{
	ID:             report.IDMTTD,
	Title:          "Mean Time to Detect",
	Description:    "MTTD percentiles (p50/p90/max) with mandatory denominator counts.",
	DefaultParams:  json.RawMessage(`{}`),
	AllowInTemplate: true,
}

// MTTDRenderer renders the MTTD block.
type MTTDRenderer struct{}

func (MTTDRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	scope := analytics.Scope{
		EngagementID: env.EngagementID,
		Blind:        env.BlindScope,
	}

	m, err := env.Analytics.MTTD(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("mttd: %w", err)
	}

	// No attempted executions at all.
	if m.AttemptedCount == 0 {
		return report.HTMLFragment(`<div class="bl-report__empty">No scored executions yet.</div>`), nil
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Mean Time to Detect (MTTD)</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	// Percentile cards.
	b.WriteString(`<div class="bl-report__mttd-percentiles">`)

	writeMTTDPercentile(&b, "p50", m.P50, f)
	writeMTTDPercentile(&b, "p90", m.P90, f)
	writeMTTDPercentile(&b, "max", m.Max, f)

	b.WriteString(`</div>`)

	// Denominator table.
	b.WriteString(`<table class="bl-report__table bl-report__mttd-table">`)
	b.WriteString(`<thead><tr><th>Component</th><th>Count</th><th>Description</th></tr></thead><tbody>`)
	b.WriteString(fmt.Sprintf(`<tr><td>Detected</td><td class="bl-report__num">%s</td><td>Executions with a computable MTTD — the percentile denominator</td></tr>`, f.Count(m.DetectedCount)))
	b.WriteString(fmt.Sprintf(`<tr><td>Undetected</td><td class="bl-report__num">%s</td><td>Attempted executions with category &ldquo;none&rdquo; or no detected_at</td></tr>`, f.Count(m.UndetectedCount)))
	b.WriteString(fmt.Sprintf(`<tr><td>Unscored</td><td class="bl-report__num">%s</td><td>Attempted executions blue has not scored</td></tr>`, f.Count(m.UnscoredCount)))
	b.WriteString(fmt.Sprintf(`<tr><td>Unmeasurable</td><td class="bl-report__num">%s</td><td>Detected but no started_at timestamp</td></tr>`, f.Count(m.UnmeasurableCount)))
	b.WriteString(`<tr class="bl-report__mttd-total"><td>Attempted</td><td class="bl-report__num">` + f.Count(m.AttemptedCount) + `</td><td>All attempted executions</td></tr>`)
	b.WriteString(`</tbody></table>`)

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// writeMTTDPercentile emits a single percentile card.
func writeMTTDPercentile(b *strings.Builder, label string, seconds *int, f report.FormatHelpers) {
	b.WriteString(`<div class="bl-report__mttd-card">`)
	b.WriteString(fmt.Sprintf(`<span class="bl-report__mttd-card-label">%s</span>`, label))
	if seconds != nil {
		b.WriteString(fmt.Sprintf(`<span class="bl-report__mttd-card-value">%s</span>`, f.Duration(*seconds)))
	} else {
		b.WriteString(`<span class="bl-report__mttd-card-value bl-report__mttd-na">—</span>`)
	}
	b.WriteString(`</div>`)
}
