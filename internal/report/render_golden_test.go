package report_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/blocks"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

var update = flag.Bool("update", false, "update golden files")

func fullRegistry() *report.Registry {
	reg := report.NewRegistry()
	reg.Register(blocks.CoverDef)
	reg.SetRenderer(report.IDCover, blocks.CoverRenderer{})
	reg.Register(blocks.SummaryDef)
	reg.SetRenderer(report.IDExecutiveSummary, blocks.SummaryRenderer{})
	reg.Register(blocks.ScopeDef)
	reg.SetRenderer(report.IDScopeRoE, blocks.ScopeRenderer{})
	reg.Register(blocks.RichTextDef)
	reg.SetRenderer(report.IDRichText, blocks.RichTextRenderer{})
	reg.Register(blocks.PageBreakDef)
	reg.SetRenderer(report.IDPageBreak, blocks.PageBreakRenderer{})
	reg.Register(blocks.HeatmapDef)
	reg.SetRenderer(report.IDCoverageHeatmap, blocks.HeatmapRenderer{})
	reg.Register(blocks.ScorecardDef)
	reg.SetRenderer(report.IDTacticScorecard, blocks.ScorecardRenderer{})
	reg.Register(blocks.DistributionDef)
	reg.SetRenderer(report.IDDetectionDistribution, blocks.DistributionRenderer{})
	reg.Register(blocks.GapsDef)
	reg.SetRenderer(report.IDDetectionGaps, blocks.GapsRenderer{})
	reg.Register(blocks.MTTDDef)
	reg.SetRenderer(report.IDMTTD, blocks.MTTDRenderer{})
	reg.Register(blocks.CompareDef)
	reg.SetRenderer(report.IDEngagementCompare, blocks.CompareRenderer{})
	reg.Register(blocks.WalkthroughDef)
	reg.SetRenderer(report.IDScenarioWalkthrough, blocks.WalkthroughRenderer{})
	reg.Register(blocks.FindingsDef)
	reg.SetRenderer(report.IDFindingsBacklog, blocks.FindingsRenderer{})
	reg.Register(blocks.EvidenceDef)
	reg.SetRenderer(report.IDEvidenceAppendix, blocks.EvidenceRenderer{})
	return reg
}

func fullEnv(t *testing.T, fx analyticstest.Fixture) report.RenderEnv {
	t.Helper()
	db := fx.DB
	engagements := storengagement.NewEngagements(db)
	eng, err := engagements.ByID(context.Background(), fx.BaselineID)
	if err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	return report.RenderEnv{
		EngagementID:       fx.BaselineID,
		EngagementName:     eng.Name,
		EngagementClient:   eng.Client,
		EngagementStartsOn: eng.StartsOn,
		EngagementEndsOn:   eng.EndsOn,
		Branding: report.BrandingConfig{
			FirmName:       "Blacklight Security",
			PrimaryColor:   "#1a1a2e",
			SecondaryColor: "#16213e",
		},
		Analytics: analytics.NewQueries(db),
		Domain: &report.DomainAdapter{
			Scenarios:  storengagement.NewScenarios(db),
			Steps:      storengagement.NewSteps(db),
			Executions: storengagement.NewExecutions(db),
			Findings:   storengagement.NewFindings(db),
			Evidence:   storengagement.NewEvidenceRepo(db),
		},
		BlindScope:      blind.Scope{},
		IncludeEvidence: true,
		Format:          report.FormatHelpers{},
	}
}

func blueEnv(t *testing.T, fx analyticstest.Fixture) report.RenderEnv {
	env := fullEnv(t, fx)
	env.BlindScope = blind.Scope{
		Blind: true,
		Seat:  authz.EngagementRoleBlue,
	}
	return env
}

type blockSpec struct {
	BlockID string
	Params  map[string]any
}

func buildReportBlocks(reportID string, blocks ...blockSpec) []storereport.ReportBlock {
	out := make([]storereport.ReportBlock, len(blocks))
	for i, b := range blocks {
		params := json.RawMessage(`{}`)
		if b.Params != nil {
			params, _ = json.Marshal(b.Params) //nolint:errcheck
		}
		out[i] = storereport.ReportBlock{
			ID:       "",
			ReportID: reportID,
			Ordinal:  i,
			BlockID:  b.BlockID,
			Params:   params,
		}
	}
	return out
}

func makeReport(engagementID string) storereport.Report {
	return storereport.Report{
		ID:           "golden-test-report",
		EngagementID: engagementID,
		Title:        "Golden Test Report",
	}
}

func TestRenderGoldenFullReport(t *testing.T) {
	fx := analyticstest.Seed(t)
	reg := fullRegistry()
	renderer := report.NewDocumentRenderer(reg)
	env := fullEnv(t, fx)
	rep := makeReport(fx.BaselineID)

	blocks := buildReportBlocks(rep.ID,
		blockSpec{BlockID: "cover", Params: map[string]any{
			"title": "Baseline Security Assessment", "subtitle": "Purple Team Exercise",
		}},
		blockSpec{BlockID: "executive_summary", Params: map[string]any{
			"body": "<p>This assessment evaluated detection coverage across the ATT&amp;CK matrix.</p>",
		}},
		blockSpec{BlockID: "scope_roe", Params: map[string]any{
			"body":    "<p>Testing conducted against production systems.</p>",
			"systems": "dc01.corp.example.com\ndc02.corp.example.com",
		}},
		blockSpec{BlockID: "coverage_heatmap", Params: map[string]any{"verbosity": "full"}},
		blockSpec{BlockID: "mttd", Params: nil},
		blockSpec{BlockID: "findings_backlog", Params: map[string]any{"includeResolved": false}},
	)

	doc := renderer.RenderDocument(context.Background(), rep, blocks, env)

	goldenPath := filepath.Join("testdata", "full_report.html")
	if *update {
		if err := os.WriteFile(goldenPath, doc.HTML, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v — run with -update to create", goldenPath, err)
	}

	if !report.NormalizeAndCompare(string(doc.HTML), string(want)) {
		actualPath := filepath.Join("testdata", "full_report.actual.html")
		_ = os.WriteFile(actualPath, doc.HTML, 0o644) //nolint:errcheck
		t.Errorf("HTML differs from golden %s. Actual written to %s for diff.", goldenPath, actualPath)
	}
}

func TestRenderGoldenBlindPreview(t *testing.T) {
	fx := analyticstest.Seed(t)
	reg := fullRegistry()
	renderer := report.NewDocumentRenderer(reg)
	env := blueEnv(t, fx)
	rep := makeReport(fx.BaselineID)

	blocks := buildReportBlocks(rep.ID,
		blockSpec{BlockID: "cover", Params: map[string]any{"title": "Blind Preview Test"}},
		blockSpec{BlockID: "coverage_heatmap", Params: map[string]any{"verbosity": "full"}},
	)

	doc := renderer.RenderDocument(context.Background(), rep, blocks, env)

	goldenPath := filepath.Join("testdata", "blind_preview.html")
	if *update {
		if err := os.WriteFile(goldenPath, doc.HTML, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v — run with -update to create", goldenPath, err)
	}

	if !report.NormalizeAndCompare(string(doc.HTML), string(want)) {
		actualPath := filepath.Join("testdata", "blind_preview.actual.html")
		_ = os.WriteFile(actualPath, doc.HTML, 0o644) //nolint:errcheck
		t.Errorf("HTML differs from golden %s. Actual written to %s for diff.", goldenPath, actualPath)
	}
}
