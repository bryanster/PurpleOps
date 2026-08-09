package report

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"

	storereport "github.com/bryanster/blacklight/internal/store/report"
)

//go:embed assets/report.css
var reportCSS []byte

// RenderedDocument is the output of [DocumentRenderer.RenderDocument]: a
// complete self-contained HTML document ready for draft preview, published
// version, share view, or PDF conversion.
type RenderedDocument struct {
	HTML     []byte
	Warnings []string
}

// DocumentRenderer assembles a complete HTML report from a list of block
// instances. It is the single rendering path: draft preview, published
// versions, share view, and PDF input all use it.
type DocumentRenderer struct {
	Registry *Registry
}

// NewDocumentRenderer returns a renderer using reg to look up block
// definitions and renderers.
func NewDocumentRenderer(reg *Registry) *DocumentRenderer {
	return &DocumentRenderer{Registry: reg}
}

// RenderDocument builds a complete self-contained HTML document from the
// ordered list of block instances.
//
// For draft preview, block errors become in-document callouts rather than
// failing the whole render. Publish (M6-011) will require zero errors.
func (r *DocumentRenderer) RenderDocument(ctx context.Context, report storereport.Report, blocks []storereport.ReportBlock, env RenderEnv) *RenderedDocument {
	var fragments []string
	var warnings []string

	for _, rb := range blocks {
		def, ok := r.Registry.Get(ID(rb.BlockID))
		if !ok {
			msg := fmt.Sprintf("Unknown block type %q at ordinal %d", rb.BlockID, rb.Ordinal)
			warnings = append(warnings, msg)
			fragments = append(fragments, renderErrorCallout(rb.BlockID, rb.Ordinal, fmt.Errorf("%s", msg)))
			continue
		}

		rend, ok := r.Registry.Renderer(def.ID)
		if !ok || rend == nil {
			fragments = append(fragments, "")
			continue
		}

		inst := Instance{
			BlockID: def.ID,
			Ordinal: rb.Ordinal,
			Params:  rb.Params,
		}

		frag, err := rend.Render(ctx, env, inst)
		if err != nil {
			msg := fmt.Sprintf("Block %q at ordinal %d: %v", rb.BlockID, rb.Ordinal, err)
			warnings = append(warnings, msg)
			fragments = append(fragments, renderErrorCallout(rb.BlockID, rb.Ordinal, err))
			continue
		}

		if frag != "" {
			fragments = append(fragments, string(frag))
		}
	}

	doc := buildDocument(env, fragments)
	doc.Warnings = warnings
	return doc
}

// buildDocument wraps block fragments in a full HTML document.
func buildDocument(env RenderEnv, fragments []string) *RenderedDocument {
	title := env.EngagementName + " — Assessment Report"

	var b strings.Builder
	b.Grow(4096 + len(reportCSS) + len(fragments)*1024)

	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>`)
	writeEscaped(&b, title)
	b.WriteString(`</title><style>`)
	b.WriteString(injectBrandingVars(env.Branding))
	b.Write(reportCSS)
	b.WriteString(`</style></head><body><div class="bl-report">`)

	if env.BlindScope.Withholds() {
		b.WriteString(`<div class="bl-report__blind-banner">`)
		b.WriteString(`Blue-seat preview — data is scoped to revealed steps only.`)
		b.WriteString(`</div>`)
	}

	for _, f := range fragments {
		b.WriteString(f)
	}

	b.WriteString(`</div></body></html>`)

	return &RenderedDocument{HTML: []byte(b.String())}
}

func injectBrandingVars(b BrandingConfig) string {
	primary := b.PrimaryColor
	if primary == "" {
		primary = "#1e3a5f"
	}
	secondary := b.SecondaryColor
	if secondary == "" {
		secondary = "#2563eb"
	}
	return fmt.Sprintf(
		`:root{--bl-primary:%s;--bl-secondary:%s;--bl-primary-fg:#ffffff;}`,
		primary, secondary,
	)
}

func renderErrorCallout(blockID string, ordinal int, err error) string {
	return fmt.Sprintf(
		`<div class="bl-report__error-callout"><strong>Block error:</strong> %s (ordinal %d) — %s</div>`,
		blockID, ordinal, escapeForHTML(err.Error()),
	)
}

func escapeForHTML(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&#34;")
		case '\'':
			buf.WriteString("&#39;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func writeEscaped(b *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
}

// IsBlindPreview reports whether the environment represents a blue-seat
// draft preview that needs a banner.
func IsBlindPreview(env RenderEnv) bool {
	return env.BlindScope.Withholds()
}
