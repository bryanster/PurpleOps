package blocks

import (
	"context"

	"github.com/bryanster/blacklight/internal/report"
)

// PageBreakDef is the Definition for the page break block.
// It has no parameters — it emits a CSS page-break marker.
var PageBreakDef = report.Definition{
	ID:            report.IDPageBreak,
	Title:         "Page Break",
	Description:   "Forces a page break in print and PDF output.",
	ParamsSchema:  nil,
	DefaultParams: nil,
	AllowInTemplate: true,
}

// PageBreakRenderer renders a zero-height page-break marker.
type PageBreakRenderer struct{}

func (PageBreakRenderer) Render(_ context.Context, _ report.RenderEnv, _ report.Instance) (report.HTMLFragment, error) {
	return report.HTMLFragment(`<div class="bl-report__page-break"></div>`), nil
}
