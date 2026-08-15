package report_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/report"
)

// TestRenderAllBlocks renders a report containing every registered block id in
// the stable catalogue order and asserts that each one renders without error
// and emits its distinctive section marker.
//
// This is the "all components work" regression test. The golden full-report
// test exercises only six of the fourteen blocks (cover, executive_summary,
// scope_roe, coverage_heatmap, mttd, findings_backlog); the other eight
// (rich_text, page_break, tactic_scorecard, detection_distribution,
// detection_gaps, engagement_compare, scenario_walkthrough, evidence_appendix)
// were covered only by isolated unit tests, so a wiring regression in one of
// them — for example a block that returns an empty fragment, or a renderer
// missing from the registry — went uncaught by the end-to-end render.
func TestRenderAllBlocks(t *testing.T) {
	fx := analyticstest.Seed(t)
	reg := fullRegistry()
	renderer := report.NewDocumentRenderer(reg)
	env := fullEnv(t, fx)
	rep := makeReport(fx.BaselineID)

	// Every block id from report.AllBlockIDs(), in catalogue order. The compare
	// block requires a baseline engagement; use the retest engagement so the
	// comparison is non-degenerate (baseline vs retest).
	blocks := buildReportBlocks(rep.ID,
		blockSpec{BlockID: "cover", Params: map[string]any{
			"title": "All Components Report", "subtitle": "Regression coverage",
		}},
		blockSpec{BlockID: "executive_summary", Params: map[string]any{
			"body": "<p>Executive summary body.</p>",
		}},
		blockSpec{BlockID: "scope_roe", Params: map[string]any{
			"body": "<p>Scope narrative.</p>", "systems": "dc01.corp.example.com\ndc02.corp.example.com",
		}},
		blockSpec{BlockID: "rich_text", Params: map[string]any{
			"html": "<h3>Custom Analysis</h3><p>Extra context.</p>",
		}},
		blockSpec{BlockID: "page_break", Params: nil},
		blockSpec{BlockID: "coverage_heatmap", Params: map[string]any{"verbosity": "full"}},
		blockSpec{BlockID: "tactic_scorecard", Params: nil},
		blockSpec{BlockID: "detection_distribution", Params: nil},
		blockSpec{BlockID: "detection_gaps", Params: nil},
		blockSpec{BlockID: "mttd", Params: nil},
		blockSpec{BlockID: "engagement_compare", Params: map[string]any{
			"baselineEngagementId": fx.RetestID,
		}},
		blockSpec{BlockID: "scenario_walkthrough", Params: nil},
		blockSpec{BlockID: "findings_backlog", Params: map[string]any{"includeResolved": true}},
		blockSpec{BlockID: "evidence_appendix", Params: nil},
	)

	doc := renderer.RenderDocument(context.Background(), rep, blocks, env)
	html := string(doc.HTML)

	if len(doc.Warnings) != 0 {
		t.Fatalf("render produced %d block warning(s): %v", len(doc.Warnings), doc.Warnings)
	}
	if strings.Contains(html, `class="bl-report__error-callout"`) {
		t.Fatalf("render produced error callouts; all blocks must render cleanly")
	}

	// Each block must emit its distinctive marker. These are the exact section
	// titles / CSS classes emitted by each renderer (see internal/report/blocks).
	wantMarkers := []struct {
		block  string
		marker string
	}{
		{"cover", "bl-report__cover"},
		{"executive_summary", "Executive Summary"},
		{"scope_roe", "Scope &amp; Rules of Engagement"},
		{"rich_text", "Custom Analysis"},
		{"page_break", "bl-report__page-break"},
		{"coverage_heatmap", "Coverage Heatmap"},
		{"tactic_scorecard", "Tactic Scorecard"},
		{"detection_distribution", "Detection Distribution"},
		{"detection_gaps", "Detection Gaps"},
		{"mttd", "Mean Time to Detect (MTTD)"},
		{"engagement_compare", "Engagement Comparison"},
		{"scenario_walkthrough", "Scenario Walkthrough"},
		{"findings_backlog", "Findings Backlog"},
		{"evidence_appendix", "Evidence Appendix"},
	}
	for _, w := range wantMarkers {
		if !strings.Contains(html, w.marker) {
			t.Errorf("block %q: rendered report is missing marker %q", w.block, w.marker)
		}
	}

	// The document must be a complete, self-contained HTML page.
	if !strings.HasPrefix(html, "<!DOCTYPE html>") || !strings.Contains(html, "</html>") {
		t.Errorf("rendered document is not a complete HTML page")
	}
}
