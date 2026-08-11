package blocks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/report"
)

func fixedTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s) //nolint:errcheck
	return t
}

// stubEnv returns a RenderEnv with enough fields for narrative blocks.
func stubEnv() report.RenderEnv {
	return report.RenderEnv{
		EngagementID:       "eng-1",
		EngagementName:     "Acme Corp Security Assessment",
		EngagementClient:   "Acme Corporation",
		EngagementStartsOn: fixedTime("2025-01-06T00:00:00Z"),
		EngagementEndsOn:   fixedTime("2025-01-10T00:00:00Z"),
		Branding: report.BrandingConfig{
			FirmName:       "Blacklight Security",
			PrimaryColor:   "#1a1a2e",
			SecondaryColor: "#16213e",
		},
	}
}

func instance(blockID report.ID, params map[string]any) report.Instance {
	raw, _ := json.Marshal(params) //nolint:errcheck
	return report.Instance{
		InstanceID: "inst-1",
		BlockID:    blockID,
		Ordinal:    0,
		Params:     raw,
	}
}

// ---------------------------------------------------------------------------
// Cover
// ---------------------------------------------------------------------------

func TestCoverRendersTitleAndClient(t *testing.T) {
	t.Parallel()

	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDCover, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "Acme Corp Security Assessment") {
		t.Error("cover should contain engagement name")
	}
	if !strings.Contains(s, "Acme Corporation") {
		t.Error("cover should contain client name")
	}
	if !strings.Contains(s, "Blacklight Security") {
		t.Error("cover should contain firm name")
	}
	if !strings.Contains(s, "2025-01-06") {
		t.Error("cover should contain start date")
	}
	if !strings.Contains(s, "2025-01-10") {
		t.Error("cover should contain end date")
	}
}

func TestCoverTitleOverride(t *testing.T) {
	t.Parallel()

	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDCover, map[string]any{
		"title": "Custom Title",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "Custom Title") {
		t.Error("cover should use title override")
	}
	if strings.Contains(s, "Acme Corp Security Assessment") {
		t.Error("cover should not contain engagement name when title overridden")
	}
}

func TestCoverSubtitle(t *testing.T) {
	t.Parallel()

	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDCover, map[string]any{
		"subtitle": "Final Report",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(frag), "Final Report") {
		t.Error("cover should contain subtitle")
	}
}

func TestCoverHideDate(t *testing.T) {
	t.Parallel()

	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDCover, map[string]any{
		"showDate": false,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if strings.Contains(s, "2025-01-06") {
		t.Error("cover should not show date when showDate=false")
	}
}

func TestCoverNoLogo(t *testing.T) {
	t.Parallel()

	env := stubEnv()
	env.Branding.LogoRef = "" // no logo
	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), env, instance(report.IDCover, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(string(frag), `<img`) {
		t.Error("cover should not emit <img> when logo ref is empty")
	}
}

func TestCoverWithLogo(t *testing.T) {
	t.Parallel()

	env := stubEnv()
	env.Branding.LogoRef = "abc123logo"
	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), env, instance(report.IDCover, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(frag), `<img`) {
		t.Error("cover should emit <img> when logo ref is present")
	}
}

func TestCoverHideLogo(t *testing.T) {
	t.Parallel()

	env := stubEnv()
	env.Branding.LogoRef = "abc123logo"
	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), env, instance(report.IDCover, map[string]any{
		"showLogo": false,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(string(frag), `<img`) {
		t.Error("cover should not emit <img> when showLogo=false")
	}
}

func TestCoverClientNameOverride(t *testing.T) {
	t.Parallel()

	env := stubEnv()
	env.Branding.ClientName = "Widgets Inc"
	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), env, instance(report.IDCover, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "Widgets Inc") {
		t.Error("cover should use branding ClientName override")
	}
	if strings.Contains(s, "Acme Corporation") {
		t.Error("cover should not show engagement client when branding ClientName set")
	}
}

func TestCoverXSSInTitle(t *testing.T) {
	t.Parallel()

	r := CoverRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDCover, map[string]any{
		"title": `<script>alert(1)</script>`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if strings.Contains(s, `<script>`) {
		t.Error("cover should escape HTML in title")
	}
	if strings.Contains(s, `<script`) && !strings.Contains(s, "&lt;script") {
		t.Error("cover should not contain raw script tags in title")
	}
}

// ---------------------------------------------------------------------------
// Executive summary
// ---------------------------------------------------------------------------

func TestSummaryRendersBody(t *testing.T) {
	t.Parallel()

	r := SummaryRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDExecutiveSummary, map[string]any{
		"body": "<p>This is a summary.</p>",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "Executive Summary") {
		t.Error("summary should have section title")
	}
	if !strings.Contains(s, "<p>This is a summary.</p>") {
		t.Error("summary should contain body content")
	}
}

func TestSummaryEmptyBody(t *testing.T) {
	t.Parallel()

	r := SummaryRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDExecutiveSummary, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(frag) != "" {
		t.Error("summary should be empty when body is empty")
	}
}

func TestSummarySanitizesOnRender(t *testing.T) {
	t.Parallel()

	r := SummaryRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDExecutiveSummary, map[string]any{
		"body": `<p>Safe</p><script>alert(1)</script>`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if strings.Contains(s, `<script>`) {
		t.Error("summary should strip script tags on render")
	}
	if !strings.Contains(s, "<p>Safe</p>") {
		t.Error("summary should preserve safe HTML")
	}
}

// ---------------------------------------------------------------------------
// Scope / RoE
// ---------------------------------------------------------------------------

func TestScopeRendersBody(t *testing.T) {
	t.Parallel()

	r := ScopeRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDScopeRoE, map[string]any{
		"body": "<p>Scoped systems.</p>",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "Scope &amp; Rules of Engagement") {
		t.Error("scope should have section title")
	}
	if !strings.Contains(s, "<p>Scoped systems.</p>") {
		t.Error("scope should contain body")
	}
}

func TestScopeSystemsList(t *testing.T) {
	t.Parallel()

	r := ScopeRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDScopeRoE, map[string]any{
		"systems": "app.acme.com\napi.acme.com",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "In-Scope Systems") {
		t.Error("scope should have systems heading")
	}
	if !strings.Contains(s, "<li>app.acme.com</li>") {
		t.Error("scope should list system entries")
	}
	if !strings.Contains(s, "<li>api.acme.com</li>") {
		t.Error("scope should list all system entries")
	}
}

func TestScopeEmptyAll(t *testing.T) {
	t.Parallel()

	r := ScopeRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDScopeRoE, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(frag) != "" {
		t.Error("scope should be empty when body and systems are empty")
	}
}

func TestScopeSanitizesHTML(t *testing.T) {
	t.Parallel()

	r := ScopeRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDScopeRoE, map[string]any{
		"body": `<p>Safe</p><img src=x onerror=alert(1)>`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if strings.Contains(s, "onerror") {
		t.Error("scope should strip event handlers")
	}
	if strings.Contains(s, `<img`) {
		t.Error("scope should strip img tags (not in allowlist)")
	}
}

// ---------------------------------------------------------------------------
// Rich text
// ---------------------------------------------------------------------------

func TestRichTextRendersHTML(t *testing.T) {
	t.Parallel()

	r := RichTextRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDRichText, map[string]any{
		"html": "<h2>Findings</h2><p>Details here.</p>",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "<h2>Findings</h2>") {
		t.Error("rich_text should render h2")
	}
	if !strings.Contains(s, "<p>Details here.</p>") {
		t.Error("rich_text should render p")
	}
}

func TestRichTextEmpty(t *testing.T) {
	t.Parallel()

	r := RichTextRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDRichText, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(frag) != "" {
		t.Error("rich_text should be empty when html is empty")
	}
}

func TestRichTextXSSStripped(t *testing.T) {
	t.Parallel()

	// Multiple XSS vectors — none should survive.
	xssPayloads := []struct {
		name, payload string
	}{
		{"script tag", `<p>text</p><script>alert(document.cookie)</script>`},
		{"iframe", `<p>text</p><iframe src="evil"></iframe>`},
		{"img onerror", `<img src=x onerror="alert(1)">`},
		{"javascript href", `<a href="javascript:alert(1)">click</a>`},
		{"style tag", `<style>body{color:red}</style><p>text</p>`},
		{"style attribute", `<p style="color:red">text</p>`},
		{"onclick", `<p onclick="alert(1)">text</p>`},
		{"data URL", `<a href="data:text/html,<script>alert(1)</script>">link</a>`},
		{"object", `<object data="evil.swf"></object><p>text</p>`},
		{"embed", `<embed src="evil.swf"><p>text</p>`},
	}

	r := RichTextRenderer{}
	for _, tc := range xssPayloads {
		t.Run(tc.name, func(t *testing.T) {
			frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDRichText, map[string]any{
				"html": tc.payload,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(frag)
			if strings.Contains(s, "<script") {
				t.Errorf("rich_text should strip <script> (payload: %s)", tc.name)
			}
			if strings.Contains(s, "<iframe") {
				t.Errorf("rich_text should strip <iframe> (payload: %s)", tc.name)
			}
			if strings.Contains(s, "onerror") || strings.Contains(s, "onclick") {
				t.Errorf("rich_text should strip event handlers (payload: %s)", tc.name)
			}
			if strings.Contains(s, "<style") {
				t.Errorf("rich_text should strip <style> (payload: %s)", tc.name)
			}
			if strings.Contains(s, "javascript:") {
				t.Errorf("rich_text should strip javascript: URLs (payload: %s)", tc.name)
			}
			if strings.Contains(s, "<object") || strings.Contains(s, "<embed") {
				t.Errorf("rich_text should strip object/embed (payload: %s)", tc.name)
			}
		})
	}
}

func TestRichTextSafeHTMLIdempotent(t *testing.T) {
	t.Parallel()

	safePayloads := []string{
		"<p>Plain paragraph</p>",
		"<h1>Heading 1</h1><p>Content</p>",
		"<ul><li>Item 1</li><li>Item 2</li></ul>",
		"<strong>Bold</strong> and <em>italic</em>",
		"<blockquote>A quote</blockquote>",
		"<pre>code block</pre>",
		"<p>Link: <a href=\"https://example.com\">example</a></p>",
	}

	r := RichTextRenderer{}
	for _, payload := range safePayloads {
		frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDRichText, map[string]any{
			"html": payload,
		}))
		if err != nil {
			t.Fatalf("unexpected error for safe HTML: %v (payload: %s)", err, payload)
		}
		s := string(frag)
		// Safe content should appear (at minimum, the payload stripped of tags should have non-empty text).
		if s == "" && payload != "" {
			t.Errorf("rich_text should not return empty for safe HTML: %q", payload)
		}
	}
}

// ---------------------------------------------------------------------------
// Page break
// ---------------------------------------------------------------------------

func TestPageBreakEmitsMarker(t *testing.T) {
	t.Parallel()

	r := PageBreakRenderer{}
	frag, err := r.Render(context.Background(), stubEnv(), instance(report.IDPageBreak, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(frag)
	if !strings.Contains(s, "bl-report__page-break") {
		t.Error("page_break should emit the page-break CSS class")
	}
}

func TestPageBreakIgnoresParams(t *testing.T) {
	t.Parallel()

	r := PageBreakRenderer{}
	frag1, _ := r.Render(context.Background(), stubEnv(), instance(report.IDPageBreak, nil))           //nolint:errcheck
	frag2, _ := r.Render(context.Background(), stubEnv(), instance(report.IDPageBreak, map[string]any{ //nolint:errcheck
		"extra": "should be ignored",
	}))

	if frag1 != frag2 {
		t.Error("page_break output should be identical regardless of params")
	}
}

// ---------------------------------------------------------------------------
// Registry: all narrative blocks have non-nil renderers
// ---------------------------------------------------------------------------

func TestNarrativeBlockDefinitions(t *testing.T) {
	t.Parallel()

	defs := map[report.ID]report.Definition{
		report.IDCover:            CoverDef,
		report.IDExecutiveSummary: SummaryDef,
		report.IDScopeRoE:         ScopeDef,
		report.IDRichText:         RichTextDef,
		report.IDPageBreak:        PageBreakDef,
	}

	for id, def := range defs {
		if def.ID != id {
			t.Errorf("Definition.ID mismatch for %q: got %q", id, def.ID)
		}
		if def.Title == "" {
			t.Errorf("block %q has empty Title", id)
		}
		// Cover, summary, scope, rich_text must have HTMLParamKeys.
		htmlBlocks := map[report.ID]bool{
			report.IDExecutiveSummary: true,
			report.IDScopeRoE:         true,
			report.IDRichText:         true,
		}
		if htmlBlocks[id] && len(def.HTMLParamKeys) == 0 {
			t.Errorf("block %q must declare HTMLParamKeys", id)
		}
		// Page break must have nil params schema.
		if id == report.IDPageBreak && def.ParamsSchema != nil {
			t.Errorf("page_break should have nil ParamsSchema, got %v", def.ParamsSchema)
		}
	}
}

func TestNarrativeBlockRenderersNonNil(t *testing.T) {
	t.Parallel()

	reg := report.NewRegistry()
	reg.Register(CoverDef)
	reg.SetRenderer(report.IDCover, CoverRenderer{})
	reg.Register(SummaryDef)
	reg.SetRenderer(report.IDExecutiveSummary, SummaryRenderer{})
	reg.Register(ScopeDef)
	reg.SetRenderer(report.IDScopeRoE, ScopeRenderer{})
	reg.Register(RichTextDef)
	reg.SetRenderer(report.IDRichText, RichTextRenderer{})
	reg.Register(PageBreakDef)
	reg.SetRenderer(report.IDPageBreak, PageBreakRenderer{})

	narrativeIDs := []report.ID{
		report.IDCover,
		report.IDExecutiveSummary,
		report.IDScopeRoE,
		report.IDRichText,
		report.IDPageBreak,
	}
	for _, id := range narrativeIDs {
		rend, ok := reg.Renderer(id)
		if !ok {
			t.Fatalf("block %q has no renderer", id)
		}
		if rend == nil {
			t.Fatalf("block %q renderer is nil", id)
		}
	}
}

func TestSetRendererDuplicatePanics(t *testing.T) {
	t.Parallel()

	reg := report.NewRegistry()
	reg.Register(PageBreakDef)
	reg.SetRenderer(report.IDPageBreak, PageBreakRenderer{})

	defer func() {
		if r := recover(); r == nil {
			t.Error("SetRenderer on duplicate should panic")
		}
	}()
	reg.SetRenderer(report.IDPageBreak, PageBreakRenderer{})
}

func TestSetRendererUnregisteredPanics(t *testing.T) {
	t.Parallel()

	reg := report.NewRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetRenderer on unregistered block should panic")
		}
	}()
	reg.SetRenderer(report.IDPageBreak, PageBreakRenderer{})
}

func TestRendererMissing(t *testing.T) {
	t.Parallel()

	reg := report.NewRegistry()
	reg.Register(PageBreakDef)

	_, ok := reg.Renderer(report.IDPageBreak)
	if ok {
		t.Error("Renderer on unset block should return false")
	}
}

// ---------------------------------------------------------------------------
// formatEngagementWindow
// ---------------------------------------------------------------------------

func TestFormatEngagementWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		starts   string
		ends     string
		expected string
	}{
		{"both zero", "", "", ""},
		{"only start", "2025-01-06T00:00:00Z", "", "2025-01-06"},
		{"only end", "", "2025-01-10T00:00:00Z", "2025-01-10"},
		{"same day", "2025-01-06T00:00:00Z", "2025-01-06T12:00:00Z", "2025-01-06"},
		{"range", "2025-01-06T00:00:00Z", "2025-01-10T00:00:00Z", "2025-01-06 – 2025-01-10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			starts := time.Time{}
			ends := time.Time{}
			if tc.starts != "" {
				starts = fixedTime(tc.starts)
			}
			if tc.ends != "" {
				ends = fixedTime(tc.ends)
			}
			got := formatEngagementWindow(starts, ends)
			if got != tc.expected {
				t.Errorf("formatEngagementWindow(%q, %q) = %q, want %q",
					tc.starts, tc.ends, got, tc.expected)
			}
		})
	}
}
