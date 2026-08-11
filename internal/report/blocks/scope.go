package blocks

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/sanitize"
)

// ScopeDef is the Definition for the scope/RoE block.
var ScopeDef = report.Definition{
	ID:          report.IDScopeRoE,
	Title:       "Scope & Rules of Engagement",
	Description: "Assessment scope, rules of engagement, and in-scope systems.",
	ParamsSchema: report.ParamSchema{
		"body": report.ParamProperty{
			Type:        "string",
			Description: "Scope narrative (rich text HTML).",
		},
		"systems": report.ParamProperty{
			Type:        "string",
			Description: "In-scope systems (plain text, one per line).",
		},
	},
	DefaultParams:   json.RawMessage(`{}`),
	AllowInTemplate: true,
	HTMLParamKeys:   []string{"body"},
}

// ScopeRenderer renders the scope/RoE block.
type ScopeRenderer struct{}

func (ScopeRenderer) Render(_ context.Context, _ report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		Body    string `json:"body,omitempty"`
		Systems string `json:"systems,omitempty"`
	}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("scope_roe: params: %w", err)
		}
	}

	body := sanitize.Sanitize(p.Body)

	hasBody := body != ""
	hasSystems := p.Systems != ""

	if !hasBody && !hasSystems {
		return report.HTMLFragment(""), nil
	}

	var b strings.Builder
	b.WriteString(`<section class="bl-report__section">`)
	b.WriteString(`<h2 class="bl-report__section-title">Scope &amp; Rules of Engagement</h2>`)

	if hasBody {
		b.WriteString(`<div class="bl-report__section-body">`)
		b.WriteString(body)
		b.WriteString(`</div>`)
	}

	if hasSystems {
		b.WriteString(`<h3>In-Scope Systems</h3>`)
		b.WriteString(`<ul>`)
		for _, line := range strings.Split(strings.TrimSpace(p.Systems), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				b.WriteString(`<li>`)
				b.WriteString(html.EscapeString(line))
				b.WriteString(`</li>`)
			}
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`</section>`)
	return report.HTMLFragment(b.String()), nil
}
