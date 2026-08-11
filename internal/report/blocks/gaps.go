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

// GapsDef is the Definition for the detection gaps block.
var GapsDef = report.Definition{
	ID:          report.IDDetectionGaps,
	Title:       "Detection Gaps",
	Description: "Techniques with detection gaps: attempted but undetected, or in-scope but not attempted.",
	ParamsSchema: report.ParamSchema{
		"maxRows": report.ParamProperty{
			Type:        "number",
			Description: "Maximum rows to show per section (default 50).",
		},
	},
	DefaultParams:   json.RawMessage(`{"maxRows":50}`),
	AllowInTemplate: true,
}

// GapsRenderer renders the detection gaps block.
type GapsRenderer struct{}

func (GapsRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	params := struct {
		MaxRows int `json:"maxRows"`
	}{MaxRows: 50}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &params); err != nil {
			return "", fmt.Errorf("detection_gaps: params: %w", err)
		}
	}

	scope := analytics.Scope{
		EngagementID: env.EngagementID,
		Blind:        env.BlindScope,
	}

	tc, err := env.Analytics.TechniqueCoverage(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("detection_gaps: technique coverage: %w", err)
	}

	// A "gap" is:
	//   1. Attempted technique with category "none" — tested, no detection.
	//   2. Technique in the pinned ATT&CK matrix that was not attempted — untested.
	//
	// This definition matches docs/analytics.md § "Detection gaps".
	var undetected, uncovered []analytics.TechniqueCoverageRow
	for _, row := range tc.Rows {
		if row.Attempted && row.BestCategory == "none" {
			undetected = append(undetected, row)
		} else if !row.Attempted {
			uncovered = append(uncovered, row)
		}
	}

	if len(undetected) == 0 && len(uncovered) == 0 {
		return report.HTMLFragment(`<div class="bl-report__empty">No detection gaps — every technique in scope is both attempted and detected.</div>`), nil
	}

	f := env.Format
	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Detection Gaps</h2>`)
	b.WriteString(`<div class="bl-report__section-body">`)

	b.WriteString(fmt.Sprintf(`<p class="bl-report__note">Attempted: %s techniques. No detection (category &ldquo;none&rdquo;): %s. Not attempted: %s.</p>`,
		f.Count(tc.AttemptedTechniques),
		f.Count(len(undetected)),
		f.Count(len(uncovered)),
	))

	// Undetected: attempted but category "none".
	if len(undetected) > 0 {
		writeGapsTable(&b, "Attempted — No Detection", undetected, params.MaxRows, f)
	}

	// Uncovered: in matrix but not attempted.
	if len(uncovered) > 0 {
		writeGapsTable(&b, "Not Attempted", uncovered, params.MaxRows, f)
	}

	b.WriteString(`</div></section>`)
	return report.HTMLFragment(b.String()), nil
}

// writeGapsTable emits a table of gap techniques.
func writeGapsTable(b *strings.Builder, title string, rows []analytics.TechniqueCoverageRow, maxRows int, f report.FormatHelpers) {
	count := len(rows)
	shown := count
	if shown > maxRows {
		shown = maxRows
	}

	b.WriteString(fmt.Sprintf(`<h3 class="bl-report__gaps-title">%s (%s)</h3>`, html.EscapeString(title), f.Count(count)))
	b.WriteString(`<table class="bl-report__table bl-report__gaps-table"><thead><tr><th>ID</th><th>Technique</th></tr></thead><tbody>`)

	for i := 0; i < shown; i++ {
		name := rows[i].Name
		if rows[i].IsSubtechnique {
			name = rows[i].TechniqueID
		}
		b.WriteString(fmt.Sprintf(`<tr><td class="bl-report__mono">%s</td><td>%s</td></tr>`,
			html.EscapeString(rows[i].TechniqueID),
			html.EscapeString(name),
		))
	}

	if count > maxRows {
		b.WriteString(fmt.Sprintf(`<tr><td colspan="2" class="bl-report__gaps-truncated">… and %s more</td></tr>`, f.Count(count-maxRows)))
	}

	b.WriteString(`</tbody></table>`)
}
