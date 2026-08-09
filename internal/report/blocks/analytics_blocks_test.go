package blocks

import (
	"context"
	"encoding/json"
	"testing"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/store/blind"
)

// analyticsEnv builds a RenderEnv backed by the fixture's analytics queries.
func analyticsEnv(fx analyticstest.Fixture, blindScope blind.Scope) report.RenderEnv {
	queries := analytics.NewQueries(fx.DB)
	return report.RenderEnv{
		EngagementID:     fx.BaselineID,
		EngagementName:   "Baseline Assessment",
		EngagementClient: "Acme Corporation",
		Branding: report.BrandingConfig{
			FirmName:       "Blacklight Security",
			PrimaryColor:   "#1a1a2e",
			SecondaryColor: "#16213e",
		},
		Analytics:  queries,
		BlindScope: blindScope,
	}
}

// leadScope returns a non-blind scope for the lead seat.
func leadScope() blind.Scope {
	return blind.Scope{Blind: true, Seat: authz.EngagementRoleLead}
}

// blueScope returns a blind scope for the blue seat.
func blueScope() blind.Scope {
	return blind.Scope{Blind: true, Seat: authz.EngagementRoleBlue}
}

// renderBlock is a test helper that renders a block and returns the HTML string.
func renderBlock(t *testing.T, r report.Renderer, env report.RenderEnv, params map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(params)
	inst := report.Instance{
		InstanceID: "inst-test",
		BlockID:    report.IDCoverageHeatmap,
		Ordinal:    0,
		Params:     raw,
	}
	frag, err := r.Render(context.Background(), env, inst)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(frag)
}

// mustContain fails if html does not contain want.
func mustContain(t *testing.T, html, want string) {
	t.Helper()
	if !strings.Contains(html, want) {
		t.Errorf("HTML missing %q:\n%s", want, truncate(html, 800))
	}
}

// mustNotContain fails if html contains forbidden.
func mustNotContain(t *testing.T, html, forbidden string) {
	t.Helper()
	if strings.Contains(html, forbidden) {
		t.Errorf("HTML contains forbidden %q", forbidden)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---------------------------------------------------------------------------
// Coverage Heatmap
// ---------------------------------------------------------------------------

func TestHeatmapRenders(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := HeatmapRenderer{}

	html := renderBlock(t, r, env, map[string]any{"verbosity": "full"})

	mustContain(t, html, "Coverage Heatmap")

	// Legend colours match NavigatorColourRamp.
	for _, hex := range analytics.NavigatorColourRamp {
		mustContain(t, html, hex)
	}

	// Summary section present.
	mustContain(t, html, "Techniques attempted")

	// At least one technique from the fixture appears.
	if len(fx.BaselineAttempted) > 0 {
		mustContain(t, html, fx.BaselineAttempted[0])
	}
}

func TestHeatmapSummaryVerbosity(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := HeatmapRenderer{}

	html := renderBlock(t, r, env, map[string]any{"verbosity": "summary"})

	mustContain(t, html, "<table")
	mustContain(t, html, "Tactic")
	mustContain(t, html, "Coverage")
}

func TestHeatmapEmptyEngagement(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.FutureID
	r := HeatmapRenderer{}

	html := renderBlock(t, r, env, map[string]any{"verbosity": "full"})
	mustContain(t, html, "No scored executions yet")
}

// ---------------------------------------------------------------------------
// Tactic Scorecard
// ---------------------------------------------------------------------------

func TestScorecardRendersTactics(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := ScorecardRenderer{}

	html := renderBlock(t, r, env, nil)

	mustContain(t, html, "Tactic Scorecard")
	mustContain(t, html, "Attempted")
	mustContain(t, html, "In Matrix")
	mustContain(t, html, "Coverage")
	mustContain(t, html, "technique in multiple tactics counts in each")
}

func TestScorecardEmptyEngagement(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.FutureID
	r := ScorecardRenderer{}

	html := renderBlock(t, r, env, nil)

	// Future engagement: 0 attempted techniques.
	// The scorecard either shows "No scored executions yet" or renders tactics with 0 counts.
	if !strings.Contains(html, "No scored executions yet") {
		mustContain(t, html, "Tactic Scorecard")
	}
}

// ---------------------------------------------------------------------------
// Detection Distribution
// ---------------------------------------------------------------------------

func TestDistributionBuckets(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := DistributionRenderer{}

	html := renderBlock(t, r, env, nil)

	mustContain(t, html, "Detection Distribution")
	mustContain(t, html, "Detection Category")
	mustContain(t, html, "Protection Rate")
	mustContain(t, html, "Outcome")

	// Unscored bucket is always present.
	mustContain(t, html, "unscored")
}

func TestDistributionEmptyEngagement(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.FutureID
	r := DistributionRenderer{}

	html := renderBlock(t, r, env, nil)
	mustContain(t, html, "No scored executions yet")
}

// ---------------------------------------------------------------------------
// Detection Gaps
// ---------------------------------------------------------------------------

func TestGapsRendersSections(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := GapsRenderer{}

	html := renderBlock(t, r, env, nil)

	mustContain(t, html, "Detection Gaps")

	// At least one gap section heading.
	hasGaps := strings.Contains(html, "Attempted") ||
		strings.Contains(html, "Not Attempted")
	if !hasGaps {
		t.Errorf("expected gap headings, got:\n%s", truncate(html, 800))
	}

	// No "round" vocabulary anywhere.
	mustNotContain(t, html, "round")
}

func TestGapsFutureEngagement(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.FutureID
	r := GapsRenderer{}
	html := renderBlock(t, r, env, nil)
	_ = html // Future engagement: block renders without error.
}

// ---------------------------------------------------------------------------
// MTTD
// ---------------------------------------------------------------------------

func TestMTTDRendersPercentiles(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := MTTDRenderer{}

	html := renderBlock(t, r, env, nil)

	mustContain(t, html, "Mean Time to Detect")
	mustContain(t, html, "p50")
	mustContain(t, html, "p90")
	mustContain(t, html, "max")

	// Denominator labels from analytics.md vocabulary.
	for _, label := range []string{"Detected", "Undetected", "Unscored", "Unmeasurable", "Attempted"} {
		mustContain(t, html, label)
	}
}

func TestMTTDEmptyEngagement(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.FutureID
	r := MTTDRenderer{}

	html := renderBlock(t, r, env, nil)
	mustContain(t, html, "No scored executions yet")
}
// Engagement Compare
// ---------------------------------------------------------------------------

func TestCompareRendersSummary(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	env.EngagementID = fx.RetestID
	r := CompareRenderer{}

	html := renderBlock(t, r, env, map[string]any{"baselineEngagementId": fx.BaselineID})

	mustContain(t, html, "Engagement Comparison")

	// Summary terms — must use improved/regressed/added/removed, NEVER "round".
	for _, term := range []string{"Improved", "Regressed", "Unchanged", "Added", "Removed"} {
		mustContain(t, html, term)
	}
	mustNotContain(t, html, "retest round")

	// Summary section renders.
	mustContain(t, html, "bl-report__compare-summary")
}

func TestCompareRequiresBaselineID(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := CompareRenderer{}

	_, err := r.Render(context.Background(), env, report.Instance{
		InstanceID: "inst-test",
		BlockID:    report.IDEngagementCompare,
		Ordinal:    0,
		Params:     json.RawMessage(`{}`),
	})
	if err == nil {
		t.Error("expected error for missing baselineEngagementId")
	}
	if !strings.Contains(err.Error(), "baselineEngagementId") {
		t.Errorf("error should mention baselineEngagementId: %v", err)
	}
}

func TestCompareUnknownBaseline(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	env := analyticsEnv(fx, leadScope())
	r := CompareRenderer{}

	html := renderBlock(t, r, env, map[string]any{"baselineEngagementId": "00000000-0000-0000-0000-000000000000"})

	// Compare with unknown baseline: DuckDB returns empty results, not an error.
	// The block renders normally with 0 rows.
	mustContain(t, html, "Engagement Comparison")
}

// ---------------------------------------------------------------------------
// Blind scope: blue seat on blind engagement
// ---------------------------------------------------------------------------

func TestHeatmapBlindRendersWithoutError(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	leadEnv := analyticsEnv(fx, leadScope())
	blueEnv := analyticsEnv(fx, blueScope())

	r := HeatmapRenderer{}
	leadHTML := renderBlock(t, r, leadEnv, map[string]any{"verbosity": "full"})
	blueHTML := renderBlock(t, r, blueEnv, map[string]any{"verbosity": "full"})

	mustContain(t, leadHTML, "Coverage Heatmap")
	mustContain(t, blueHTML, "Coverage Heatmap")

	// Totals may differ because blue is withheld.
	if len(fx.BaselineAttempted) != len(fx.BaselineBlueAttempted) {
		if leadHTML == blueHTML {
			t.Error("lead and blue heatmap should differ for blind engagement with unrevealed steps")
		}
	}
}

func TestMTTDBlindOmitsUnrevealed(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	leadEnv := analyticsEnv(fx, leadScope())
	blueEnv := analyticsEnv(fx, blueScope())

	r := MTTDRenderer{}
	leadHTML := renderBlock(t, r, leadEnv, nil)
	blueHTML := renderBlock(t, r, blueEnv, nil)

	mustContain(t, leadHTML, "Mean Time to Detect")
	mustContain(t, blueHTML, "Mean Time to Detect")

	// Lead and blue should differ when the engagement has unrevealed steps.
	if len(fx.BaselineMTTDSeconds) != len(fx.BaselineBlueMTTDSeconds) {
		if leadHTML == blueHTML {
			t.Error("lead and blue MTTD should differ for blind engagement with unrevealed steps")
		}
	}
}

// ---------------------------------------------------------------------------
// Registry: all analytics blocks have non-nil renderers
// ---------------------------------------------------------------------------

func TestAnalyticsBlockDefinitions(t *testing.T) {
	t.Parallel()

	defs := []struct {
		def report.Definition
		id  report.ID
	}{
		{HeatmapDef, report.IDCoverageHeatmap},
		{ScorecardDef, report.IDTacticScorecard},
		{DistributionDef, report.IDDetectionDistribution},
		{GapsDef, report.IDDetectionGaps},
		{MTTDDef, report.IDMTTD},
		{CompareDef, report.IDEngagementCompare},
	}

	for _, d := range defs {
		if d.def.ID != d.id {
			t.Errorf("definition ID %q != expected %q", d.def.ID, d.id)
		}
		if d.def.Title == "" {
			t.Errorf("definition %q has empty Title", d.id)
		}
	}
}

func TestAnalyticsBlockRenderersNonNil(t *testing.T) {
	t.Parallel()

	reg := report.NewRegistry()
	reg.Register(HeatmapDef)
	reg.SetRenderer(report.IDCoverageHeatmap, HeatmapRenderer{})
	reg.Register(ScorecardDef)
	reg.SetRenderer(report.IDTacticScorecard, ScorecardRenderer{})
	reg.Register(DistributionDef)
	reg.SetRenderer(report.IDDetectionDistribution, DistributionRenderer{})
	reg.Register(GapsDef)
	reg.SetRenderer(report.IDDetectionGaps, GapsRenderer{})
	reg.Register(MTTDDef)
	reg.SetRenderer(report.IDMTTD, MTTDRenderer{})
	reg.Register(CompareDef)
	reg.SetRenderer(report.IDEngagementCompare, CompareRenderer{})

	ids := []report.ID{
		report.IDCoverageHeatmap,
		report.IDTacticScorecard,
		report.IDDetectionDistribution,
		report.IDDetectionGaps,
		report.IDMTTD,
		report.IDEngagementCompare,
	}

	for _, id := range ids {
		r, ok := reg.Renderer(id)
		if !ok {
			t.Errorf("no renderer for %s", id)
		}
		if r == nil {
			t.Errorf("renderer for %s is nil", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Format helpers
// ---------------------------------------------------------------------------

func TestFormatCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{5, "5"},
		{999, "999"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
	}

	f := report.FormatHelpers{}
	for _, tt := range tests {
		got := f.Count(tt.n)
		if got != tt.want {
			t.Errorf("Count(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sec  int
		want string
	}{
		{0, "0s"},
		{15, "15s"},
		{60, "1m"},
		{61, "1m 1s"},
		{3600, "60m"},
	}

	f := report.FormatHelpers{}
	for _, tt := range tests {
		got := f.Duration(tt.sec)
		if got != tt.want {
			t.Errorf("Duration(%d) = %q, want %q", tt.sec, got, tt.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	t.Parallel()

	f := report.FormatHelpers{}
	if got := f.Date(time.Time{}); got != "" {
		t.Errorf("Date(zero) = %q, want empty", got)
	}
}

func TestFormatPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		num, denom int
		want       string
	}{
		{0, 10, "0%"},
		{5, 10, "50%"},
		{1, 3, "33%"},
		{10, 0, "—"},
	}

	f := report.FormatHelpers{}
	for _, tt := range tests {
		got := f.Percent(tt.num, tt.denom)
		if got != tt.want {
			t.Errorf("Percent(%d, %d) = %q, want %q", tt.num, tt.denom, got, tt.want)
		}
	}
}
