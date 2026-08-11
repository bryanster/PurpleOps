package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/bryanster/blacklight/internal/report"
)

// EvidenceDef is the Definition for the evidence appendix block.
var EvidenceDef = report.Definition{
	ID:          report.IDEvidenceAppendix,
	Title:       "Evidence Appendix",
	Description: "Index of evidence items with captions and step linkage. Images inlined only when evidence is included.",
	ParamsSchema: report.ParamSchema{
		"limit": report.ParamProperty{
			Type:        "integer",
			Description: "Maximum number of evidence items to list. Default 50.",
		},
		"imagesOnly": report.ParamProperty{
			Type:        "boolean",
			Description: "Show only allowed image MIME evidence items.",
		},
	},
	DefaultParams:      json.RawMessage(`{"limit":50}`),
	NeedsEvidenceOptIn: true,
}

// allowedImageMIMEs is the set of MIME types that may be inlined as <img>.
var allowedImageMIMEs = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

// EvidenceRenderer renders the evidence appendix block.
type EvidenceRenderer struct{}

func (EvidenceRenderer) Render(ctx context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		Limit      int  `json:"limit"`
		ImagesOnly bool `json:"imagesOnly"`
	}
	p.Limit = 50
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("evidence_appendix: params: %w", err)
		}
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}

	// When evidence is not included, render a placeholder.
	if !env.IncludeEvidence {
		return report.HTMLFragment(
			`<section class="bl-report__section"><h2 class="bl-report__section-title">Evidence Appendix</h2>` +
				`<p class="bl-report__empty">Evidence omitted at publish.</p></section>`), nil
	}

	// Collect all executions, then all evidence for each.
	executions, err := env.Domain.ListExecutions(ctx, env.EngagementID)
	if err != nil {
		return "", fmt.Errorf("evidence_appendix: list executions: %w", err)
	}

	type evRow struct {
		Evidence string // filename
		Caption  string
		Side     string
		StepName string
		MIME     string
		Size     int64
		IsImage  bool
	}
	var rows []evRow
	stepNames := make(map[string]string) // stepID → step name, lazy

	for _, exec := range executions {
		evList, err := env.Domain.ListEvidence(ctx, exec.ID)
		if err != nil {
			return "", fmt.Errorf("evidence_appendix: list evidence for %s: %w", exec.ID, err)
		}
		for _, ev := range evList {
			if p.ImagesOnly && !allowedImageMIMEs[ev.MIME] {
				continue
			}
			// Lazy-load step name.
			if _, ok := stepNames[ev.ExecutionID]; !ok {
				// We don't have a direct StepName on evidence; derive from step.
				// For now, use execution ID as fallback — the assembler (M6-009)
				// can enrich with step names.
				stepNames[ev.ExecutionID] = ""
			}
			rows = append(rows, evRow{
				Evidence: ev.Filename,
				Caption:  ev.Caption,
				Side:     string(ev.Side),
				MIME:     ev.MIME,
				Size:     ev.Size,
				IsImage:  allowedImageMIMEs[ev.MIME],
			})
		}
	}

	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Evidence Appendix</h2>`)

	if len(rows) == 0 {
		b.WriteString(`<p class="bl-report__empty">No evidence items.</p>`)
		b.WriteString(`</section>`)
		return report.HTMLFragment(b.String()), nil
	}

	// Apply limit.
	if p.Limit > 0 && len(rows) > p.Limit {
		rows = rows[:p.Limit]
	}

	b.WriteString(`<table class="bl-report__table">`)
	b.WriteString(`<thead><tr>`)
	b.WriteString(`<th>#</th>`)
	b.WriteString(`<th>Filename</th>`)
	b.WriteString(`<th>Caption</th>`)
	b.WriteString(`<th>Side</th>`)
	b.WriteString(`<th>Type</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for i, r := range rows {
		b.WriteString(`<tr>`)
		b.WriteString(fmt.Sprintf(`<td class="bl-report__cell-num">%d</td>`, i+1))
		b.WriteString(`<td class="bl-report__cell-mono">`)
		b.WriteString(html.EscapeString(r.Evidence))
		b.WriteString(`</td>`)
		b.WriteString(`<td>`)
		b.WriteString(html.EscapeString(r.Caption))
		b.WriteString(`</td>`)
		b.WriteString(`<td>`)
		b.WriteString(html.EscapeString(r.Side))
		b.WriteString(`</td>`)
		b.WriteString(`<td>`)
		b.WriteString(html.EscapeString(r.MIME))
		b.WriteString(`</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)

	if p.Limit > 0 && len(rows) >= p.Limit {
		b.WriteString(fmt.Sprintf(
			`<p class="bl-report__footnote">Showing first %d items. Increase the limit parameter to include more.</p>`,
			p.Limit,
		))
	}

	b.WriteString(`</section>`)
	return report.HTMLFragment(b.String()), nil
}
