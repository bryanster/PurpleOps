package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/report"
)

// CoverDef is the Definition for the cover block.
var CoverDef = report.Definition{
	ID:          report.IDCover,
	Title:       "Cover Page",
	Description: "Title page with engagement name, client, dates, and logo.",
	ParamsSchema: report.ParamSchema{
		"title": report.ParamProperty{
			Type:        "string",
			Description: "Override the report title. Defaults to the engagement name.",
		},
		"subtitle": report.ParamProperty{
			Type:        "string",
			Description: "Subtitle displayed below the title.",
		},
		"showDate": report.ParamProperty{
			Type:        "boolean",
			Description: "Show the engagement date window.",
			Default:     json.RawMessage("true"),
		},
		"showLogo": report.ParamProperty{
			Type:        "boolean",
			Description: "Show the branding logo.",
			Default:     json.RawMessage("true"),
		},
	},
	DefaultParams:   json.RawMessage(`{"showDate": true, "showLogo": true}`),
	AllowInTemplate: true,
}

// CoverRenderer renders the cover page block.
type CoverRenderer struct{}

func (CoverRenderer) Render(_ context.Context, env report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	params := coverParams{
		ShowDate: true,
		ShowLogo: true,
	}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &params); err != nil {
			return "", fmt.Errorf("cover: params: %w", err)
		}
	}

	title := env.EngagementName
	if params.Title != "" {
		title = params.Title
	}
	clientName := env.EngagementClient
	if env.Branding.ClientName != "" {
		clientName = env.Branding.ClientName
	}
	firmName := env.Branding.FirmName

	var b strings.Builder
	b.WriteString(`<div class="bl-report__cover">`)

	// Logo.
	if params.ShowLogo && env.Branding.LogoRef != "" {
		b.WriteString(`<img class="bl-report__cover-logo" src="`)
		b.WriteString(html.EscapeString(logoDataURL(env.Branding.LogoRef)))
		b.WriteString(`" alt="`)
		b.WriteString(html.EscapeString(firmName))
		b.WriteString(` logo">`)
	}

	// Title.
	b.WriteString(`<h1 class="bl-report__cover-title">`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</h1>`)

	// Subtitle.
	if params.Subtitle != "" {
		b.WriteString(`<p class="bl-report__cover-subtitle">`)
		b.WriteString(html.EscapeString(params.Subtitle))
		b.WriteString(`</p>`)
	}

	// Meta: client and firm.
	b.WriteString(`<div class="bl-report__cover-meta">`)
	if clientName != "" {
		b.WriteString(`<p class="bl-report__cover-meta-item"><span class="bl-report__cover-meta-label">Prepared for:</span> `)
		b.WriteString(html.EscapeString(clientName))
		b.WriteString(`</p>`)
	}
	b.WriteString(`<p class="bl-report__cover-meta-item"><span class="bl-report__cover-meta-label">Prepared by:</span> `)
	b.WriteString(html.EscapeString(firmName))
	b.WriteString(`</p>`)

	// Date window.
	if params.ShowDate {
		dateStr := formatEngagementWindow(env.EngagementStartsOn, env.EngagementEndsOn)
		if dateStr != "" {
			b.WriteString(`<p class="bl-report__cover-meta-item"><span class="bl-report__cover-meta-label">Assessment window:</span> `)
			b.WriteString(html.EscapeString(dateStr))
			b.WriteString(`</p>`)
		}
	}
	b.WriteString(`</div>`)

	b.WriteString(`</div>`)
	return report.HTMLFragment(b.String()), nil
}

type coverParams struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	ShowDate bool   `json:"showDate"`
	ShowLogo bool   `json:"showLogo"`
}

// formatEngagementWindow returns a human-readable date range string.
// Returns empty string when both dates are zero.
func formatEngagementWindow(startsOn, endsOn time.Time) string {
	if startsOn.IsZero() && endsOn.IsZero() {
		return ""
	}
	if startsOn.IsZero() {
		return endsOn.Format("2006-01-02")
	}
	if endsOn.IsZero() {
		return startsOn.Format("2006-01-02")
	}
	if startsOn.Format("2006-01-02") == endsOn.Format("2006-01-02") {
		return startsOn.Format("2006-01-02")
	}
	return startsOn.Format("2006-01-02") + " – " + endsOn.Format("2006-01-02")
}

// logoDataURL returns a placeholder data URL for a logo blob reference.
// In M6-009 the assembler will replace this with an actual asset URL.
func logoDataURL(ref string) string {
	// Placeholder — the assembler (M6-009) resolves this to a real URL.
	// Use a CSS-friendly fallback so no broken <img> in the fragment.
	return "data:image/svg+xml," + `%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'%3E%3C/svg%3E`
}
