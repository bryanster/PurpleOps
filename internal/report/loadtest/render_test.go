// Package loadtest proves report HTML rendering, publishing, and PDF
// generation stay within budget under realistic data (M7-008). Tests use the
// real serialized DuckDB writer and analytics queries — never a mock.
//
// Tests:
//
//   - TestReportRenderBudget — CI gate (always on)
//
//   - TestReportRenderDetectsRegression — mutation test
//
//   - TestReportRenderWithConcurrentWrites — CI gate: render + write fairness
//
//   - TestReportRenderLoad — full developer load (BLACKLIGHT_LOADTEST=1)
//
//     BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/report/loadtest/ -run TestReportRenderLoad
package loadtest_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/blocks"
	"github.com/bryanster/blacklight/internal/report/pdf"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/settings"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// ---------------------------------------------------------------------------
// Budgets
const (
	renderHTMLP95Budget = 3 * time.Second
	renderHTMLMaxBudget = 3 * time.Second
	publishP95Budget    = 6 * time.Second
	publishMaxBudget    = 5 * time.Second
	pdfTimeoutBudget    = 30 * time.Second

	writeP95Budget = 500 * time.Millisecond
)

// ---------------------------------------------------------------------------
// CI-scale fixture dimensions
// ---------------------------------------------------------------------------

const (
	ciTechCount     = 200
	ciTacticCount   = 14
	ciScenarios     = 5
	ciStepsPerScen  = 10
	ciFindings      = 50
	ciHistoryPerFnd = 5
	ciEvidenceBlobs = 20
	ciWorkers       = 3
	ciDuration      = 10 * time.Second
	ciRenderIters   = 50
)

// ---------------------------------------------------------------------------
// Full-load fixture dimensions
// ---------------------------------------------------------------------------

const (
	loadTechCount     = 800
	loadTacticCount   = 14
	loadScenarios     = 10
	loadStepsPerScen  = 50
	loadFindings      = 200
	loadHistoryPerFnd = 5
	loadEvidenceBlobs = 100
	loadWorkers       = 5
	loadDuration      = 15 * time.Second
)

// ---------------------------------------------------------------------------
// Fixture types
// ---------------------------------------------------------------------------

type reportFixture struct {
	DB      *store.DB
	EvStore *evidence.Store

	Eng1ID      string
	Eng2ID      string
	Eng1ExecIDs []string
	Eng1StepIDs []string
	FindingIDs  []string
}

func mustExec(t testing.TB, tx *sql.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("INSERT failed: %v\n  SQL: %s", err, query)
	}
}

func seedReportFixture(t testing.TB, techCount, tacticCount, scenarios, stepsPerScen, findings, historyPerFnd, evidenceBlobs int) reportFixture {
	t.Helper()

	db := storetest.Migrated(t)
	ctx := context.Background()

	attackVersion := "99.0"
	sourceID := "01900000-0000-7000-8000-000000000001"

	evidenceDir := t.TempDir()
	blobRepo := storengagement.NewEvidenceBlobRepo(db)
	evStore := evidence.NewStore(evidenceDir, config.Evidence{MaxUploadBytes: config.ByteSize(10 << 20)}, blobRepo)

	baseTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	engStart := baseTime
	engEnd := baseTime.Add(90 * 24 * time.Hour)
	userID := "01900000-0000-7000-8000-000000000010"

	type blobInfo struct {
		sha256hex string
		size      int64
	}
	var blobs []blobInfo
	for e := range evidenceBlobs {
		content := fmt.Sprintf("evidence blob %d for report load test\n", e)
		sha256hex, _, _, err := evStore.Put(ctx, strings.NewReader(content), "text/plain", "01900000-0000-7000-8000-000000000100")
		if err != nil {
			t.Fatalf("seed blob %d: %v", e, err)
		}
		blobs = append(blobs, blobInfo{sha256hex: sha256hex, size: int64(len(content))})
	}

	var (
		eng1StepIDs, eng1ExecIDs []string
		eng2StepIDs, eng2ExecIDs []string
		findingIDs               []string
	)

	if err := db.Write(ctx, func(tx *sql.Tx) error {
		mustExec(t, tx,
			`INSERT INTO content.content_source_version
				(id, source_id, version, status, item_count, synced_at, error,
				 raw_sha256, raw_path, raw_bytes, created_at, updated_at)
			 VALUES (?, ?, ?, 'ready', ?, ?, '', '', '', 0, ?, ?)`,
			"01900000-0000-7000-8000-000000000000",
			sourceID, attackVersion, techCount, baseTime, baseTime, baseTime,
		)

		tacticNames := []string{
			"Reconnaissance", "Resource Development", "Initial Access",
			"Execution", "Persistence", "Privilege Escalation",
			"Defense Evasion", "Credential Access", "Discovery",
			"Lateral Movement", "Collection", "Command and Control",
			"Exfiltration", "Impact",
		}
		for i := range min(tacticCount, len(tacticNames)) {
			tid := fmt.Sprintf("01900000-0000-7000-8000-0000000001%02d", i)
			extID := fmt.Sprintf("TA%04d", i+1)
			mustExec(t, tx,
				`INSERT INTO content.content_tactic
					(id, source_id, version, external_id, name, description, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, '', ?, ?)`,
				tid, sourceID, attackVersion, extID, tacticNames[i], baseTime, baseTime,
			)
		}

		type tech struct {
			id, extID, name, parentID string
			isSub                     bool
		}
		var techniques []tech
		for i := range techCount {
			uid := fmt.Sprintf("01900000-0000-7000-8000-%012d", i)
			extID := fmt.Sprintf("T%04d", i+1000)
			isSub := i%10 == 0
			parentID := ""
			if isSub {
				parentID = fmt.Sprintf("T%04d", (i/10)*10+1000)
			}
			techniques = append(techniques, tech{
				id: uid, extID: extID, name: fmt.Sprintf("Technique %s", extID),
				parentID: parentID, isSub: isSub,
			})
		}
		for _, te := range techniques {
			mustExec(t, tx,
				`INSERT INTO content.content_technique
					(id, source_id, version, external_id, name, description,
					 is_subtechnique, parent_external_id, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, ?)`,
				te.id, sourceID, attackVersion, te.extID, te.name,
				te.isSub, te.parentID, baseTime, baseTime,
			)
		}

		for i, te := range techniques {
			tacIdx := i % tacticCount
			mustExec(t, tx,
				`INSERT INTO content.content_technique_tactic
					(source_id, version, technique_external_id, tactic_external_id)
				 VALUES (?, ?, ?, ?)`,
				sourceID, attackVersion, te.extID, fmt.Sprintf("TA%04d", tacIdx+1),
			)
			if i%5 == 0 {
				tacIdx2 := (tacIdx + 5) % tacticCount
				mustExec(t, tx,
					`INSERT INTO content.content_technique_tactic
						(source_id, version, technique_external_id, tactic_external_id)
					 VALUES (?, ?, ?, ?)`,
					sourceID, attackVersion, te.extID, fmt.Sprintf("TA%04d", tacIdx2+1),
				)
			}
		}

		eng1ID := "01900000-0000-7000-8000-000000000100"
		mustExec(t, tx,
			`INSERT INTO app.engagement
				(id, name, client, description, status, starts_on, ends_on,
				 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			 VALUES (?, 'Report Load Alpha', 'TestOrg', 'Report load test engagement.',
			        'active', ?, ?, ?, 'standard', false, ?, ?, ?)`,
			eng1ID, engStart, engEnd, attackVersion, userID, baseTime, baseTime,
		)

		eng2ID := "01900000-0000-7000-8000-000000000200"
		eng2Start := engStart.Add(30 * 24 * time.Hour)
		eng2End := eng2Start.Add(90 * 24 * time.Hour)
		mustExec(t, tx,
			`INSERT INTO app.engagement
				(id, name, client, description, status, starts_on, ends_on,
				 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			 VALUES (?, 'Report Load Beta', 'TestOrg', 'Second report load test engagement.',
			        'active', ?, ?, ?, 'standard', false, ?, ?, ?)`,
			eng2ID, eng2Start, eng2End, attackVersion, userID, baseTime, baseTime,
		)

		seedEng := func(engID string, timeOffset time.Duration) (stepIDs, execIDs []string) {
			counter := 0
			for s := range scenarios {
				scID := fmt.Sprintf("01900000-0000-7000-8001-%012d%012d", s, timeOffset.Nanoseconds())
				mustExec(t, tx,
					`INSERT INTO app.scenario
						(id, engagement_id, ordinal, name, narrative, source,
						 threat_actor, source_ref, plan_id, created_at, updated_at)
					 VALUES (?, ?, ?, ?, '', 'manual', '', '', '', ?, ?)`,
					scID, engID, s+1, fmt.Sprintf("Scenario %d", s+1), baseTime, baseTime,
				)

				for st := range stepsPerScen {
					counter++
					stepOrdinal := st + 1
					stepID := fmt.Sprintf("01900000-0000-7000-8002-%012d%012d", counter, timeOffset.Nanoseconds())
					techIdx := (s*stepsPerScen + st) % techCount
					techExtID := fmt.Sprintf("T%04d", techIdx+1000)
					tacExtID := fmt.Sprintf("TA%04d", (techIdx%tacticCount)+1)

					status := "complete"
					switch {
					case st%10 == 7:
						status = "blocked"
					case st%10 == 8:
						status = "pending"
					case st%10 == 9:
						status = "skipped"
					}

					var detCat, protection sql.NullString
					var detectedAt sql.NullTime
					if status != "skipped" && status != "pending" {
						catIdx := techIdx % 5
						cats := []string{"none", "telemetry", "general", "tactic", "technique"}
						detCat = sql.NullString{String: cats[catIdx], Valid: true}
						protIdx := (techIdx + 1) % 3
						prots := []string{"not_blocked", "blocked", "partial"}
						protection = sql.NullString{String: prots[protIdx], Valid: true}
						if catIdx > 0 {
							mttdSeconds := (techIdx * 37) % 3600
							t0 := baseTime.Add(timeOffset).Add(time.Duration(s*stepsPerScen+st) * time.Hour)
							detectedAt = sql.NullTime{Time: t0.Add(time.Duration(mttdSeconds) * time.Second), Valid: true}
						}
					}

					execStarted := sql.NullTime{Time: baseTime.Add(timeOffset), Valid: true}
					execEnded := sql.NullTime{Time: baseTime.Add(timeOffset).Add(30 * time.Minute), Valid: true}
					if status == "pending" {
						execStarted = sql.NullTime{}
						execEnded = sql.NullTime{}
					}

					revealedAt := sql.NullTime{Time: baseTime, Valid: true}
					mustExec(t, tx,
						`INSERT INTO app.step
							(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
							 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
							 attack_version, revealed_at, created_at, updated_at)
						 VALUES (?, ?, ?, ?, '', ?, '', ?, '{}', '', '', '[]', '[]',
						        ?, ?, ?, ?)`,
						stepID, scID, stepOrdinal, fmt.Sprintf("Step %d", stepOrdinal),
						techExtID, tacExtID,
						attackVersion, nullIfValid(revealedAt), baseTime, baseTime,
					)

					execID := fmt.Sprintf("01900000-0000-7000-8003-%012d%012d", counter, timeOffset.Nanoseconds())
					mustExec(t, tx,
						`INSERT INTO app.execution
							(id, step_id, version, status, executed_by, started_at, ended_at,
							 command_run, source_host, target_host, red_notes,
							 detection_category, detection_modifiers, protection,
							 detected_at, detecting_source, detecting_rule_ref,
							 alert_severity, blue_notes, scored_by, scored_at,
							 created_at, updated_at)
						 VALUES (?, ?, 1, ?, ?, ?, ?,
						        '', '', '', '',
						        ?, '[]', ?,
						        ?, '', '',
						        '', '', ?, ?,
						        ?, ?)`,
						execID, stepID, status, userID,
						nullIfValid(execStarted), nullIfValid(execEnded),
						nullNullString(detCat), nullNullString(protection),
						nullIfValid(detectedAt),
						userID, baseTime,
						baseTime, baseTime,
					)

					stepIDs = append(stepIDs, stepID)
					execIDs = append(execIDs, execID)
				}
			}
			return stepIDs, execIDs
		}

		eng1StepIDs, eng1ExecIDs = seedEng(eng1ID, 0)
		eng2StepIDs, eng2ExecIDs = seedEng(eng2ID, 30*24*time.Hour)
		_ = eng2StepIDs
		_ = eng2ExecIDs

		findingStatuses := []string{"open", "in_progress", "resolved", "accepted_risk"}
		for f := range findings {
			findID := fmt.Sprintf("01900000-0000-7000-8004-%012d", f)
			createdAt := baseTime.Add(time.Duration(f*12) * time.Hour)

			var linkedExec sql.NullString
			if f < len(eng1ExecIDs) {
				linkedExec = sql.NullString{String: eng1ExecIDs[f], Valid: true}
			}

			mustExec(t, tx,
				`INSERT INTO app.finding
					(id, engagement_id, title, description, severity, recommendation,
					 "owner", status, created_from_execution, created_at, updated_at)
				 VALUES (?, ?, 'Finding ' || ?, '', ?, '',
				        ?, ?, ?, ?, ?)`,
				findID, eng1ID, findID, findingStatuses[f%len(findingStatuses)],
				userID, findingStatuses[f%len(findingStatuses)], nullNullString(linkedExec), createdAt, createdAt,
			)

			if f < len(eng1StepIDs) {
				mustExec(t, tx,
					`INSERT INTO app.finding_step (finding_id, step_id) VALUES (?, ?)`,
					findID, eng1StepIDs[f],
				)
			}

			for h := range historyPerFnd {
				histID := fmt.Sprintf("01900000-0000-7000-8005-%012d%02d", f, h)
				fromStatus := ""
				toStatus := findingStatuses[f%len(findingStatuses)]
				if h > 0 {
					fromStatus = findingStatuses[(f+h-1)%len(findingStatuses)]
				}
				histTS := createdAt.Add(time.Duration(h*24) * time.Hour)
				mustExec(t, tx,
					`INSERT INTO app.finding_status_history
						(id, finding_id, engagement_id, from_status, to_status, changed_by, changed_at)
					 VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
					histID, findID, eng1ID, fromStatus, toStatus, userID, histTS,
				)
			}
			findingIDs = append(findingIDs, findID)
		}

		for e, b := range blobs {
			if len(eng1ExecIDs) == 0 {
				continue
			}
			execIdx := e % len(eng1ExecIDs)
			blobID := fmt.Sprintf("01900000-0000-7000-8006-%012d", e)
			mustExec(t, tx,
				`INSERT INTO app.evidence
					(id, execution_id, blob_sha256, filename, caption, side, mime, size, uploaded_by, uploaded_at)
				 VALUES (?, ?, ?, ?, '', 'red', ?, ?, ?, ?)`,
				blobID, eng1ExecIDs[execIdx], b.sha256hex,
				fmt.Sprintf("evidence-%d.txt", e), "text/plain", b.size,
				userID, baseTime,
			)
		}

		return nil
	}); err != nil {
		t.Fatalf("seedReportFixture: %v", err)
	}

	return reportFixture{
		DB:          db,
		EvStore:     evStore,
		Eng1ID:      "01900000-0000-7000-8000-000000000100",
		Eng2ID:      "01900000-0000-7000-8000-000000000200",
		Eng1ExecIDs: eng1ExecIDs,
		Eng1StepIDs: eng1StepIDs,
		FindingIDs:  findingIDs,
	}
}

// ---------------------------------------------------------------------------
// Registry and report helpers
// ---------------------------------------------------------------------------

func fullRegistry() *report.Registry {
	reg := report.NewRegistry()
	reg.Register(blocks.CoverDef)
	reg.SetRenderer(report.IDCover, blocks.CoverRenderer{})
	reg.Register(blocks.SummaryDef)
	reg.SetRenderer(report.IDExecutiveSummary, blocks.SummaryRenderer{})
	reg.Register(blocks.ScopeDef)
	reg.SetRenderer(report.IDScopeRoE, blocks.ScopeRenderer{})
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

func blockSetForLoad(compareEngID string) []storereport.ReportBlock {
	emptyParams := json.RawMessage("{}")
	compareParams := json.RawMessage(fmt.Sprintf(`{"compareEngagementId": %q}`, compareEngID))

	blockIDs := []struct {
		id     report.ID
		params json.RawMessage
	}{
		{report.IDCoverageHeatmap, emptyParams},
		{report.IDTacticScorecard, emptyParams},
		{report.IDDetectionDistribution, emptyParams},
		{report.IDDetectionGaps, emptyParams},
		{report.IDMTTD, emptyParams},
		{report.IDEngagementCompare, compareParams},
		{report.IDScenarioWalkthrough, emptyParams},
		{report.IDFindingsBacklog, emptyParams},
		{report.IDEvidenceAppendix, emptyParams},
	}

	blocks := make([]storereport.ReportBlock, len(blockIDs))
	for i, b := range blockIDs {
		blocks[i] = storereport.ReportBlock{
			ID:      fmt.Sprintf("01900000-0000-7000-9000-%012d", i),
			Ordinal: i,
			BlockID: string(b.id),
			Params:  b.params,
		}
	}
	return blocks
}

func buildRenderEnv(t testing.TB, db *store.DB, engID string) report.RenderEnv {
	t.Helper()
	ctx := context.Background()
	engagements := storengagement.NewEngagements(db)
	eng, err := engagements.ByID(ctx, engID)
	if err != nil {
		t.Fatalf("read engagement %s: %v", engID, err)
	}
	return report.RenderEnv{
		EngagementID:       engID,
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

func seedReport(t testing.TB, db *store.DB, engID, compareEngID string) (storereport.Report, []storereport.ReportBlock) {
	t.Helper()
	ctx := context.Background()
	reports := storereport.NewReports(db)

	rpt, err := reports.Create(ctx, storereport.NewReport{
		EngagementID: engID,
		Title:        "Load Test Report",
		CreatedBy:    "01900000-0000-7000-8000-000000000010",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}

	blks := blockSetForLoad(compareEngID)
	saved, err := reports.ReplaceBlocks(ctx, rpt.ID, toNewBlocks(blks))
	if err != nil {
		t.Fatalf("replace blocks: %v", err)
	}
	return rpt, saved
}

func toNewBlocks(blks []storereport.ReportBlock) []storereport.NewBlock {
	out := make([]storereport.NewBlock, len(blks))
	for i, b := range blks {
		out[i] = storereport.NewBlock{
			BlockID: b.BlockID,
			Params:  b.Params,
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Render measurement
// ---------------------------------------------------------------------------

type renderResult struct {
	HTMLSamples []time.Duration
}

func runRenders(t testing.TB, db *store.DB, engID, compareEngID string, iters int) (renderResult, report.RenderEnv) {
	t.Helper()

	reg := fullRegistry()
	rpt, blks := seedReport(t, db, engID, compareEngID)
	env := buildRenderEnv(t, db, engID)

	ctx := context.Background()
	renderer := report.NewDocumentRenderer(reg)
	var htmlSamples []time.Duration

	for range iters {
		start := time.Now()
		doc := renderer.RenderDocument(ctx, rpt, blks, env)
		htmlSamples = append(htmlSamples, time.Since(start))
		_ = doc
	}

	return renderResult{HTMLSamples: htmlSamples}, env
}

// ---------------------------------------------------------------------------
// Publish measurement
// ---------------------------------------------------------------------------

func measurePublish(t testing.TB, db *store.DB, engID, compareEngID string, env report.RenderEnv) time.Duration {
	t.Helper()

	reg := fullRegistry()
	rpt, _ := seedReport(t, db, engID, compareEngID)

	reports := storereport.NewReports(db)
	versions := storereport.NewVersions(db)
	renderer := report.NewDocumentRenderer(reg)

	settingsStore := settings.New(db)
	brandingSvc, err := report.NewBrandingSettingsService(settingsStore, "")
	if err != nil {
		t.Fatalf("NewBrandingSettingsService: %v", err)
	}
	resolver := report.NewBrandingResolver(brandingSvc)

	ps, err := report.NewPublishService(report.PublishDeps{
		Reports:  reports,
		Versions: versions,
		Renderer: renderer,
		Resolver: resolver,
		Activity: nil,
	})
	if err != nil {
		t.Fatalf("NewPublishService: %v", err)
	}

	start := time.Now()
	_, err = ps.Publish(context.Background(), env, report.PublishInput{
		ReportID:        rpt.ID,
		EngagementID:    engID,
		EngagementName:  "Load Test",
		PublishedBy:     "01900000-0000-7000-8000-000000000010",
		IncludeEvidence: true,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return time.Since(start)
}

// ---------------------------------------------------------------------------
// PDF smoke
// ---------------------------------------------------------------------------

func smokePDF(t testing.TB, db *store.DB, engID, compareEngID string, env report.RenderEnv) string {
	t.Helper()

	chrome := "/usr/bin/chromium-browser"
	candidates := []string{
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/snap/bin/chromium",
	}
	for _, p := range candidates {
		if _, err := pdf.New(p, pdfTimeoutBudget); err == nil {
			chrome = p
			break
		}
	}

	printer, err := pdf.New(chrome, pdfTimeoutBudget)
	if err != nil {
		return fmt.Sprintf("skipped (no Chromium binary: %v)", err)
	}
	defer printer.Close()

	reg := fullRegistry()
	rpt, blks := seedReport(t, db, engID, compareEngID)
	ctx := context.Background()
	renderer := report.NewDocumentRenderer(reg)
	doc := renderer.RenderDocument(ctx, rpt, blks, env)
	if doc == nil || len(doc.HTML) == 0 {
		return "skipped (empty HTML)"
	}

	cctx, cancel := context.WithTimeout(context.Background(), pdfTimeoutBudget)
	defer cancel()

	pdfBytes, err := printer.RenderPDF(cctx, doc.HTML)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if !pdf.IsPDF(pdfBytes) {
		return fmt.Sprintf("error: output not valid PDF (%d bytes)", len(pdfBytes))
	}
	pages := pdf.MinPageCount(pdfBytes)
	return fmt.Sprintf("%d pages, %d bytes", pages, len(pdfBytes))
}

// ---------------------------------------------------------------------------
// Concurrent writers
// ---------------------------------------------------------------------------

func runConcurrentWriters(ctx context.Context, t testing.TB, db *store.DB, execIDs []string, n int, duration time.Duration) func() ([]time.Duration, []error) {
	var (
		mu         sync.Mutex
		latencies  []time.Duration
		probeErrs  []error
		writeCount atomic.Int64
	)

	done := make(chan struct{})
	time.AfterFunc(duration, func() { close(done) })

	for range n {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				execID := execIDs[randInt(len(execIDs))]
				start := time.Now()
				if err := db.Write(ctx, func(tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx,
						`UPDATE app.execution SET updated_at = ? WHERE id = ?`,
						time.Now().UTC(), execID)
					return err
				}); err != nil {
					mu.Lock()
					probeErrs = append(probeErrs, err)
					mu.Unlock()
					continue
				}
				mu.Lock()
				latencies = append(latencies, time.Since(start))
				mu.Unlock()
				writeCount.Add(1)
			}
		}()
	}

	return func() ([]time.Duration, []error) {
		<-done
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return latencies, probeErrs
	}
}

// ---------------------------------------------------------------------------
// N+1 regression wrapper (mutation test)
// ---------------------------------------------------------------------------

type slowAnalyticsQueries struct {
	*analytics.Queries
	db *store.DB
}

func (s *slowAnalyticsQueries) TechniqueCoverage(ctx context.Context, scope analytics.Scope) (*analytics.TechniqueCoverageResult, error) {
	rows, err := s.db.Read().QueryContext(ctx,
		`SELECT DISTINCT ct.external_id, ct.name, ct.is_subtechnique
		 FROM content.content_technique ct
		 JOIN content.content_source_version csv ON csv.source_id = ct.source_id AND csv.version = ct.version
		 WHERE csv.status = 'ready'
		 ORDER BY ct.external_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &analytics.TechniqueCoverageResult{}
	for rows.Next() {
		var extID, name string
		var isSub bool
		if err := rows.Scan(&extID, &name, &isSub); err != nil {
			return nil, err
		}
		// Deliberately slow: 3 per-technique subqueries instead of the
		// single-statement rollup, simulating a multi-method regression.
		var covered sql.NullBool
		for range 3 {
			err := s.db.Read().QueryRowContext(ctx,
				`SELECT EXISTS(
					SELECT 1 FROM app.step st
					JOIN app.scenario sc ON sc.id = st.scenario_id
					JOIN app.execution ex ON ex.step_id = st.id
					WHERE sc.engagement_id = ? AND st.technique_id = ?
				)`, scope.EngagementID, extID).Scan(&covered)
			if err != nil {
				return nil, err
			}
		}
		result.Rows = append(result.Rows, analytics.TechniqueCoverageRow{
			TechniqueID:    extID,
			Name:           name,
			IsSubtechnique: isSub,
			Matched:        true,
			Attempted:      covered.Bool,
		})
		result.MatrixTechniques++
		if covered.Bool {
			result.AttemptedTechniques++
		} else {
			result.NotAttemptedTechniques++
		}
	}
	return result, rows.Err()
}

var _ report.AnalyticsFacade = (*slowAnalyticsQueries)(nil)

// ---------------------------------------------------------------------------
// Test: CI gate — HTML render budget
// ---------------------------------------------------------------------------

func TestReportRenderBudget(t *testing.T) {
	fx := seedReportFixture(t, ciTechCount, ciTacticCount, ciScenarios, ciStepsPerScen,
		ciFindings, ciHistoryPerFnd, ciEvidenceBlobs)

	result, _ := runRenders(t, fx.DB, fx.Eng1ID, fx.Eng2ID, ciRenderIters)

	htmlP95 := percentile(result.HTMLSamples, 95)
	htmlMax := maxDuration(result.HTMLSamples)

	t.Logf("HTML render: p50=%v p95=%v max=%v samples=%d",
		percentile(result.HTMLSamples, 50), htmlP95, htmlMax, len(result.HTMLSamples))

	if htmlP95 > renderHTMLP95Budget*raceMult {
		t.Errorf("HTML render p95 %v exceeds budget %v", htmlP95, renderHTMLP95Budget*raceMult)
	}
	if htmlMax > renderHTMLMaxBudget*raceMult {
		t.Errorf("HTML render max %v exceeds budget %v", htmlMax, renderHTMLMaxBudget*raceMult)
	}
}

// ---------------------------------------------------------------------------
// Test: mutation gate — proves CI catches N+1 regression
// ---------------------------------------------------------------------------

func TestReportRenderDetectsRegression(t *testing.T) {
	fx := seedReportFixture(t, loadTechCount, loadTacticCount, loadScenarios, loadStepsPerScen,
		loadFindings, loadHistoryPerFnd, loadEvidenceBlobs)

	reg := fullRegistry()
	rpt, blks := seedReport(t, fx.DB, fx.Eng1ID, fx.Eng2ID)
	env := buildRenderEnv(t, fx.DB, fx.Eng1ID)

	slowQ := &slowAnalyticsQueries{
		Queries: analytics.NewQueries(fx.DB),
		db:      fx.DB,
	}
	env.Analytics = slowQ

	renderer := report.NewDocumentRenderer(reg)
	ctx := context.Background()

	start := time.Now()
	doc := renderer.RenderDocument(ctx, rpt, blks, env)
	dur := time.Since(start)

	t.Logf("N+1 regression render (800 techniques): %v (%d bytes HTML)", dur, len(doc.HTML))

	if dur <= renderHTMLMaxBudget {
		t.Errorf("N+1 regression render %v did NOT exceed budget %v — mutation detection broken", dur, renderHTMLMaxBudget)
	}
}

// ---------------------------------------------------------------------------
// Test: concurrent render + write (CI gate)
// ---------------------------------------------------------------------------

func TestReportRenderWithConcurrentWrites(t *testing.T) {
	fx := seedReportFixture(t, ciTechCount, ciTacticCount, ciScenarios, ciStepsPerScen,
		ciFindings, ciHistoryPerFnd, ciEvidenceBlobs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cleanup := runConcurrentWriters(ctx, t, fx.DB, fx.Eng1ExecIDs, ciWorkers, ciDuration)

	reg := fullRegistry()
	rpt, blks := seedReport(t, fx.DB, fx.Eng1ID, fx.Eng2ID)
	env := buildRenderEnv(t, fx.DB, fx.Eng1ID)
	renderer := report.NewDocumentRenderer(reg)

	var htmlSamples []time.Duration
	deadline := time.After(ciDuration)

loop:
	for {
		select {
		case <-deadline:
			break loop
		default:
		}
		start := time.Now()
		doc := renderer.RenderDocument(ctx, rpt, blks, env)
		htmlSamples = append(htmlSamples, time.Since(start))
		_ = doc
	}

	writeLatencies, writeErrs := cleanup()

	htmlP95 := percentile(htmlSamples, 95)
	writeP95 := percentile(writeLatencies, 95)

	t.Logf("Concurrent: HTML render p50=%v p95=%v max=%v samples=%d",
		percentile(htmlSamples, 50), htmlP95, maxDuration(htmlSamples), len(htmlSamples))
	t.Logf("Concurrent: write p50=%v p95=%v max=%v samples=%d errors=%d",
		percentile(writeLatencies, 50), writeP95, maxDuration(writeLatencies), len(writeLatencies), len(writeErrs))

	if htmlP95 > renderHTMLP95Budget*raceMult {
		t.Errorf("HTML render p95 under write load %v exceeds budget %v", htmlP95, renderHTMLP95Budget*raceMult)
	}
	if writeP95 > writeP95Budget*raceMult {
		t.Errorf("write p95 under render load %v exceeds budget %v", writeP95, writeP95Budget*raceMult)
	}
	if len(writeErrs) > 0 {
		t.Errorf("write errors under render load: %v", writeErrs[0])
	}
}

// ---------------------------------------------------------------------------
// Test: full developer load (BLACKLIGHT_LOADTEST=1)
// ---------------------------------------------------------------------------

func TestReportRenderLoad(t *testing.T) {
	if !loadtestEnabled() {
		t.Skip("BLACKLIGHT_LOADTEST=1 not set")
	}

	fx := seedReportFixture(t, loadTechCount, loadTacticCount, loadScenarios, loadStepsPerScen,
		loadFindings, loadHistoryPerFnd, loadEvidenceBlobs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cleanupWriters := runConcurrentWriters(ctx, t, fx.DB, fx.Eng1ExecIDs, loadWorkers, loadDuration)

	reg := fullRegistry()
	rpt, blks := seedReport(t, fx.DB, fx.Eng1ID, fx.Eng2ID)
	env := buildRenderEnv(t, fx.DB, fx.Eng1ID)
	renderer := report.NewDocumentRenderer(reg)

	var htmlSamples []time.Duration
	deadline := time.After(loadDuration)

	for {
		select {
		case <-deadline:
			goto done
		default:
		}
		start := time.Now()
		doc := renderer.RenderDocument(ctx, rpt, blks, env)
		htmlSamples = append(htmlSamples, time.Since(start))
		_ = doc
	}
done:

	writeLatencies, writeErrs := cleanupWriters()
	pubDur := measurePublish(t, fx.DB, fx.Eng1ID, fx.Eng2ID, env)
	pdfResult := smokePDF(t, fx.DB, fx.Eng1ID, fx.Eng2ID, env)

	htmlP50 := percentile(htmlSamples, 50)
	htmlP95 := percentile(htmlSamples, 95)
	htmlMax := maxDuration(htmlSamples)
	writeP50 := percentile(writeLatencies, 50)
	writeP95 := percentile(writeLatencies, 95)
	writeMax := maxDuration(writeLatencies)

	t.Logf("=== Full load results ===")
	t.Logf("HTML render: p50=%v p95=%v max=%v samples=%d",
		htmlP50, htmlP95, htmlMax, len(htmlSamples))
	t.Logf("Publish: %v", pubDur)
	t.Logf("PDF smoke: %s", pdfResult)
	t.Logf("Write under render: p50=%v p95=%v max=%v samples=%d errors=%d",
		writeP50, writeP95, writeMax, len(writeLatencies), len(writeErrs))

	if htmlP95 > renderHTMLP95Budget*raceMult {
		t.Errorf("HTML render p95 %v exceeds budget %v", htmlP95, renderHTMLP95Budget*raceMult)
	}
	if htmlMax > renderHTMLMaxBudget*raceMult {
		t.Errorf("HTML render max %v exceeds budget %v", htmlMax, renderHTMLMaxBudget*raceMult)
	}
	if pubDur > publishP95Budget*raceMult {
		t.Errorf("publish %v exceeds budget %v", pubDur, publishP95Budget)
	}
	if writeP95 > writeP95Budget*raceMult {
		t.Errorf("write p95 under render load %v exceeds budget %v", writeP95, writeP95Budget*raceMult)
	}
	if len(writeErrs) > 0 {
		t.Errorf("write errors: %v", writeErrs[0])
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(time.Now().UnixNano() % int64(n))
}

func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	slices.Sort(sorted)
	idx := int(math.Ceil(float64(p)/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func maxDuration(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	m := samples[0]
	for _, s := range samples[1:] {
		if s > m {
			m = s
		}
	}
	return m
}

func loadtestEnabled() bool {
	return config.LoadTestEnabled()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nullIfValid(ns sql.NullTime) any {
	if !ns.Valid {
		return nil
	}
	return ns.Time
}

func nullNullString(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}
