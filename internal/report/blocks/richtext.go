package blocks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/sanitize"
)

// RichTextDef is the Definition for the rich text block.
var RichTextDef = report.Definition{
	ID:          report.IDRichText,
	Title:       "Rich Text",
	Description: "Free-form rich text section with headings, lists, and formatting.",
	ParamsSchema: report.ParamSchema{
		"html": report.ParamProperty{
			Type:        "string",
			Description: "Rich text content (HTML).",
		},
	},
	DefaultParams:  json.RawMessage(`{}`),
	AllowInTemplate: true,
	HTMLParamKeys:  []string{"html"},
}

// RichTextRenderer renders the rich text block.
type RichTextRenderer struct{}

func (RichTextRenderer) Render(_ context.Context, _ report.RenderEnv, inst report.Instance) (report.HTMLFragment, error) {
	var p struct {
		HTML string `json:"html,omitempty"`
	}
	if len(inst.Params) > 0 {
		if err := json.Unmarshal(inst.Params, &p); err != nil {
			return "", fmt.Errorf("rich_text: params: %w", err)
		}
	}

	html := sanitize.Sanitize(p.HTML)
	if html == "" {
		return report.HTMLFragment(""), nil
	}

	return report.HTMLFragment(
		`<section class="bl-report__section">` +
			`<div class="bl-report__section-body">` + html + `</div>` +
			`</section>`), nil
}
