// Package sanitize provides server-side HTML sanitization for report
// content (M6-005). It uses a strict bluemonday allowlist designed for
// user-authored rich text that reaches a client-facing published page.
//
// The policy is called on write (when blocks are persisted) and again
// at render time (defense in depth). It is deliberately tight:
// structural markup + semantic inline only; no images, no embeds,
// no style attributes, no event handlers.
package sanitize

import (
	"github.com/microcosm-cc/bluemonday"
)

// Policy is the single server-side HTML sanitization policy for report
// rich text. It is safe for concurrent use (bluemonday policies are
// stateless once built).
var Policy = buildPolicy()

func buildPolicy() *bluemonday.Policy {
	// Start from scratch — allow nothing by default. This is safer
	// than starting from UGCPolicy and trying to subtract, because
	// new bluemonday releases could add elements to UGCPolicy that
	// we haven't reviewed.
	p := bluemonday.NewPolicy()

	// Block-level elements.
	p.AllowElements(
		"p", "h1", "h2", "h3",
		"ul", "ol", "li",
		"pre", "blockquote",
	)

	// Inline elements.
	p.AllowElements("strong", "em", "code", "br")

	// Links: http, https, mailto only. Force rel="noopener
	// noreferrer nofollow" on every link (defense in depth —
	// even if a link survived sanitization, it can't leak
	// referrer or give the target page JS access to our window).
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoReferrerOnLinks(true)
	p.RequireNoFollowOnLinks(true)

	// Everything else is stripped: script, style, iframe, object,
	// embed, form, img, video, audio, svg, canvas, input, button,
	// select, textarea, link, meta, event handlers, style attrs,
	// class, id, data-* attrs, javascript: URLs, data: URLs.

	return p
}

// Sanitize applies the report policy to html. Empty string in → empty
// string out. Never returns an error — dirty input is stripped, never
// rejected.
func Sanitize(html string) string {
	if html == "" {
		return ""
	}
	return Policy.Sanitize(html)
}
