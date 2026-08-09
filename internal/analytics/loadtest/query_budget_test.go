// Package loadtest proves analytics queries stay within budget under
// concurrent write load (M5-015). Tests use the real serialized DuckDB
// writer — never a mock.
//
//	BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/analytics/loadtest/ -run TestAnalyticsQueryLoad
package loadtest_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"math/big"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/archive"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

const (
	rollupP95Budget = 250 * time.Millisecond
	rollupMaxBudget = 1 * time.Second
	dashboardBudget = 1 * time.Second

	probeInterval  = 50 * time.Millisecond
	writeP95Budget = 200 * time.Millisecond
	writeMaxBudget = 2 * time.Second
)

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
)

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

type analyticsFixture struct {
	DB          *store.DB
	EvidenceDir string
	EvStore     *evidence.Store

	Eng1ID      string
	Eng2ID      string
	Eng1StepIDs []string
	Eng1ExecIDs []string
	Eng2StepIDs []string
	Eng2ExecIDs []string
	FindingIDs  []string
	BlobIDs     []string
}

func mustExec(t testing.TB, tx *sql.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("INSERT failed: %v\n  SQL: %s", err, query)
	}
}

func seedAnalyticsFixture(t testing.TB, techCount, tacticCount, scenarios, stepsPerScen, findings, historyPerFnd, evidenceBlobs int) analyticsFixture {
	t.Helper()

	db := storetest.Migrated(t)
	ctx := context.Background()

	attackVersion := "99.0"
	sourceID := "01900000-0000-7000-8000-000000000001"

	evidenceDir := filepath.Join(t.TempDir(), "evidence")
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
	var blobSHA256s []string
	for e := range evidenceBlobs {
		content := fmt.Sprintf("evidence blob %d for analytics load test\n", e)
		sha256hex, _, _, err := evStore.Put(ctx, strings.NewReader(content), "text/plain", "01900000-0000-7000-8000-000000000100")
		if err != nil {
			t.Fatalf("seed blob %d: %v", e, err)
		}
		blobs = append(blobs, blobInfo{sha256hex: sha256hex, size: int64(len(content))})
		blobSHA256s = append(blobSHA256s, sha256hex)
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
			extID := fmt.Sprintf("T%04d", i+1000)
			uid := fmt.Sprintf("01900000-0000-7000-8000-%012d", i)
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
			 VALUES (?, 'Load Test Alpha', 'TestOrg', 'Analytics load test engagement.',
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
			 VALUES (?, 'Load Test Beta', 'TestOrg', 'Second analytics load test engagement.',
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

		findingStatuses := []string{"open", "in_progress", "resolved", "accepted_risk"}
		for f := range findings {
			findID := fmt.Sprintf("01900000-0000-7000-8004-%012d", f)
			status := findingStatuses[f%len(findingStatuses)]
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
				userID, status, nullNullString(linkedExec), createdAt, createdAt,
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
				toStatus := status
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
		t.Fatalf("seedAnalyticsFixture: %v", err)
	}

	return analyticsFixture{
		DB:          db,
		EvidenceDir: evidenceDir,
		EvStore:     evStore,
		Eng1ID:      "01900000-0000-7000-8000-000000000100",
		Eng2ID:      "01900000-0000-7000-8000-000000000200",
		Eng1StepIDs: eng1StepIDs,
		Eng1ExecIDs: eng1ExecIDs,
		Eng2StepIDs: eng2StepIDs,
		Eng2ExecIDs: eng2ExecIDs,
		FindingIDs:  findingIDs,
		BlobIDs:     blobSHA256s,
	}
}

func nullIfValid(ns sql.NullTime) any {
	if !ns.Valid { return nil }
	return ns.Time
}

func nullNullString(ns sql.NullString) any {
	if !ns.Valid { return nil }
	return ns.String
}

func scope(engID string) analytics.Scope {
	return analytics.Scope{
		EngagementID: engID,
		Blind:        blind.Scope{Blind: false, Seat: authz.EngagementRoleLead},
	}
}

type writerConfig struct {
	db      *store.DB
	engID   string
	execIDs []string
	userID  string
}

func runWriters(ctx context.Context, cfg writerConfig, n int) func() {
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				doWrite(ctx, cfg)
			}
		}()
	}
	return wg.Wait
}

func doWrite(ctx context.Context, cfg writerConfig) {
	idx := randInt(len(cfg.execIDs))
	execID := cfg.execIDs[idx]
	_ = cfg.db.Write(ctx, func(tx *sql.Tx) error {
		cat := []string{"none", "telemetry", "general", "tactic", "technique"}[randInt(5)]
		_, err := tx.ExecContext(ctx,
			`UPDATE app.execution
			 SET detection_category = ?, blue_notes = 'loadtest probe', updated_at = ?
			 WHERE id = ?`,
			cat, time.Now().UTC(), execID,
		)
		return err
	})
}

func writeProbeSample(ctx context.Context, db *store.DB, execIDs []string, _ string) time.Duration {
	idx := randInt(len(execIDs))
	execID := execIDs[idx]
	start := time.Now()
	_ = db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE app.execution SET blue_notes = 'probe', updated_at = ? WHERE id = ?`,
			time.Now().UTC(), execID,
		)
		return err
	})
	return time.Since(start)
}

func runAllAnalytics(ctx context.Context, q *analytics.Queries, fx analyticsFixture) map[string][]time.Duration {
	samples := map[string][]time.Duration{
		"TechniqueCoverage":    {},
		"TacticCoverage":       {},
		"CategoryDistribution": {},
		"ProtectionRate":       {},
		"OutcomeMix":           {},
		"ModifierDistribution": {},
		"MTTD":                 {},
		"Burndown":             {},
		"FindingsBySeverity":   {},
		"Compare":              {},
		"Navigator":            {},
		"ExecutionsExport":     {},
		"FindingsExport":       {},
		"DashboardSet":         {},
	}

	for {
		select {
		case <-ctx.Done():
			return samples
		default:
		}
		engScope := scope(fx.Eng1ID)

		t0 := time.Now()
		if _, err := q.TechniqueCoverage(ctx, engScope); err == nil {
			samples["TechniqueCoverage"] = append(samples["TechniqueCoverage"], time.Since(t0))
		}
		t0 = time.Now()
		if _, err := q.TacticCoverage(ctx, engScope); err == nil {
			samples["TacticCoverage"] = append(samples["TacticCoverage"], time.Since(t0))
		}

		dashStart := time.Now()
		var wg sync.WaitGroup
		var catDur, protDur, outcDur, modDur time.Duration
		wg.Add(4)
		go func() { defer wg.Done(); s := time.Now(); q.CategoryDistribution(ctx, engScope); catDur = time.Since(s) }()
		go func() { defer wg.Done(); s := time.Now(); q.ProtectionRate(ctx, engScope); protDur = time.Since(s) }()
		go func() { defer wg.Done(); s := time.Now(); q.OutcomeMix(ctx, engScope); outcDur = time.Since(s) }()
		go func() { defer wg.Done(); s := time.Now(); q.ModifierDistribution(ctx, engScope); modDur = time.Since(s) }()
		wg.Wait()
		samples["DashboardSet"] = append(samples["DashboardSet"], time.Since(dashStart))
		samples["CategoryDistribution"] = append(samples["CategoryDistribution"], catDur)
		samples["ProtectionRate"] = append(samples["ProtectionRate"], protDur)
		samples["OutcomeMix"] = append(samples["OutcomeMix"], outcDur)
		samples["ModifierDistribution"] = append(samples["ModifierDistribution"], modDur)

		t0 = time.Now()
		if _, err := q.MTTD(ctx, engScope); err == nil {
			samples["MTTD"] = append(samples["MTTD"], time.Since(t0))
		}
		t0 = time.Now()
		if _, err := q.FindingsBurndown(ctx, engScope, ""); err == nil {
			samples["Burndown"] = append(samples["Burndown"], time.Since(t0))
		}
		t0 = time.Now()
		if _, err := q.FindingsBySeverity(ctx, engScope); err == nil {
			samples["FindingsBySeverity"] = append(samples["FindingsBySeverity"], time.Since(t0))
		}
		t0 = time.Now()
		if _, err := q.Compare(ctx, analytics.CompareScope{Baseline: scope(fx.Eng1ID), Current: scope(fx.Eng2ID)}); err == nil {
			samples["Compare"] = append(samples["Compare"], time.Since(t0))
		}
		t0 = time.Now()
		if _, err := q.NavigatorLayer(ctx, engScope); err == nil {
			samples["Navigator"] = append(samples["Navigator"], time.Since(t0))
		}
		t0 = time.Now()
		if rows, err := q.ExecutionsExport(ctx, engScope); err == nil {
			for rows.Next() {}
			rows.Close()
			samples["ExecutionsExport"] = append(samples["ExecutionsExport"], time.Since(t0))
		}
		t0 = time.Now()
		if rows, err := q.FindingsExport(ctx, engScope); err == nil {
			for rows.Next() {}
			rows.Close()
			samples["FindingsExport"] = append(samples["FindingsExport"], time.Since(t0))
		}
	}
}

func measureArchiveMemory(t testing.TB, db *store.DB, evStore *evidence.Store, engID string) {
	t.Helper()
	ctx := context.Background()

	scenarios := storengagement.NewScenarios(db)
	scRows, err := scenarios.ListByEngagement(ctx, engID)
	if err != nil { t.Fatalf("list scenarios: %v", err) }

	steps := storengagement.NewSteps(db)
	allSteps, err := steps.ListByEngagement(ctx, engID, blind.Scope{Blind: false})
	if err != nil { t.Fatalf("list steps: %v", err) }
	stepsByScen := make(map[string][]storengagement.Step)
	for _, s := range allSteps {
		stepsByScen[s.ScenarioID] = append(stepsByScen[s.ScenarioID], s)
	}

	execs := storengagement.NewExecutions(db)
	executions, err := execs.ListByEngagement(ctx, engID, nil, nil)
	if err != nil { t.Fatalf("list executions: %v", err) }
	execByStep := make(map[string]storengagement.Execution)
	for _, e := range executions { execByStep[e.StepID] = e }

	evRepo := storengagement.NewEvidenceRepo(db)
	var evEntries []archive.EvidenceEntry
	for _, e := range executions {
		evRows, err := evRepo.ListByExecution(ctx, e.ID)
		if err != nil { t.Fatalf("list evidence: %v", err) }
		for _, ev := range evRows {
			evEntries = append(evEntries, archive.EvidenceEntry{
				ID: ev.ID, BlobSHA256: ev.BlobSHA256, Filename: ev.Filename,
				Side: string(ev.Side), ExecutionID: ev.ExecutionID,
				UploadedBy: archive.UserRef{ID: ev.UploadedBy}, UploadedAt: ev.UploadedAt,
				Size: ev.Size, MIME: ev.MIME,
			})
		}
	}

	engData := archive.EngagementArchive{}
	for _, sc := range scRows {
		scJSON := archive.ScenarioToJSON(sc)
		var stepsWith []archive.StepWithExec
		for _, s := range stepsByScen[sc.ID] {
			stepJSON := archive.StepToJSON(s)
			var execJSON archive.ExecutionJSON
			if e, ok := execByStep[s.ID]; ok {
				execJSON = archive.ExecutionToJSON(e, archive.UserRef{}, archive.UserRef{})
			}
			stepsWith = append(stepsWith, archive.StepWithExec{Step: stepJSON, Execution: execJSON})
		}
		engData.Scenarios = append(engData.Scenarios, archive.ScenarioWithSteps{Scenario: scJSON, Steps: stepsWith})
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- archive.WriteArchive(pw, archive.WriteOptions{
			EngagementID: engID, EngagementName: "Load Test", Client: "TestOrg",
			Mode: "standard", AttackVersion: "99.0",
			Engagement: engData, Analytics: nil,
			Evidence: evEntries, EvidenceOpener: evStore.Open,
		})
		pw.CloseWithError(nil)
	}()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, pr); err != nil { t.Fatalf("read archive: %v", err) }
	if _, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		t.Fatalf("not a valid ZIP: %v", err)
	}
	if err := <-done; err != nil { t.Fatalf("WriteArchive: %v", err) }

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	heapDelta := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	if heapDelta < 0 { heapDelta = 0 }
	const maxDelta = 50 << 20
	t.Logf("archive memory: before=%d KiB after=%d KiB delta=%d KiB",
		m1.HeapAlloc>>10, m2.HeapAlloc>>10, heapDelta>>10)
	if heapDelta > maxDelta {
		t.Errorf("archive heap growth %d KiB exceeds budget %d KiB", heapDelta>>10, maxDelta>>10)
	}
}

func TestAnalyticsQueryBudget(t *testing.T) {
	fx := seedAnalyticsFixture(t, ciTechCount, ciTacticCount, ciScenarios, ciStepsPerScen, ciFindings, ciHistoryPerFnd, ciEvidenceBlobs)
	q := analytics.NewQueries(fx.DB)

	ctx, cancel := context.WithTimeout(context.Background(), ciDuration+5*time.Second)
	defer cancel()

	writerCtx, writerCancel := context.WithTimeout(ctx, ciDuration)
	defer writerCancel()
	cfg := writerConfig{db: fx.DB, engID: fx.Eng1ID, execIDs: fx.Eng1ExecIDs, userID: "01900000-0000-7000-8000-000000000010"}
	waitWriters := runWriters(writerCtx, cfg, ciWorkers)

	var writeMu sync.Mutex
	var writeLatencies []time.Duration
	probeCtx, probeCancel := context.WithTimeout(ctx, ciDuration)
	defer probeCancel()
	go func() {
		ticker := time.NewTicker(probeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-probeCtx.Done():
				return
			case <-ticker.C:
				d := writeProbeSample(probeCtx, fx.DB, fx.Eng1ExecIDs, "01900000-0000-7000-8000-000000000010")
				writeMu.Lock()
				writeLatencies = append(writeLatencies, d)
				writeMu.Unlock()
			}
		}
	}()

	measureCtx, measureCancel := context.WithTimeout(ctx, ciDuration)
	defer measureCancel()
	samples := runAllAnalytics(measureCtx, q, fx)
	waitWriters()

	rollups := []string{
		"TechniqueCoverage", "TacticCoverage", "CategoryDistribution",
		"ProtectionRate", "OutcomeMix", "ModifierDistribution",
		"MTTD", "Burndown", "FindingsBySeverity", "Compare",
		"Navigator", "ExecutionsExport", "FindingsExport",
	}
	for _, name := range rollups {
		s := samples[name]
		if len(s) == 0 { t.Errorf("%s: no samples", name); continue }
		m := measureRollup(name, s)
		t.Logf("%s: p50=%v p95=%v max=%v n=%d", name, m.P50, m.P95, m.Max, m.Count)
		if m.P95 > rollupP95Budget { t.Errorf("%s: p95 %v > budget %v", name, m.P95, rollupP95Budget) }
		if m.Max > rollupMaxBudget { t.Errorf("%s: max %v > budget %v", name, m.Max, rollupMaxBudget) }
	}

	if dash := samples["DashboardSet"]; len(dash) > 0 {
		m := measureRollup("DashboardSet", dash)
		t.Logf("DashboardSet: p95=%v", m.P95)
		if m.P95 > dashboardBudget { t.Errorf("DashboardSet: p95 %v > budget %v", m.P95, dashboardBudget) }
	}

	writeMu.Lock()
	sort.Slice(writeLatencies, func(i, j int) bool { return writeLatencies[i] < writeLatencies[j] })
	wP95 := percentile(writeLatencies, 95)
	wMax := maxDuration(writeLatencies)
	writeMu.Unlock()
	t.Logf("write probe: p95=%v max=%v n=%d", wP95, wMax, len(writeLatencies))
	if wP95 > writeP95Budget { t.Errorf("write p95 %v > budget %v", wP95, writeP95Budget) }
	if wMax > writeMaxBudget { t.Errorf("write max %v > budget %v", wMax, writeMaxBudget) }

	measureArchiveMemory(t, fx.DB, fx.EvStore, fx.Eng1ID)
}

func TestAnalyticsQueryBudgetDetectsRegression(t *testing.T) {
	fx := seedAnalyticsFixture(t, ciTechCount, ciTacticCount, ciScenarios, ciStepsPerScen, ciFindings, ciHistoryPerFnd, 0)
	q := analytics.NewQueries(fx.DB)
	ctx := t.Context()
	engScope := scope(fx.Eng1ID)

	start := time.Now()
	_, err := q.TechniqueCoverage(ctx, engScope)
	baselineDur := time.Since(start)
	if err != nil { t.Fatalf("baseline: %v", err) }
	t.Logf("baseline TechniqueCoverage: %v", baselineDur)

	rows, err := fx.DB.Read().QueryContext(ctx,
		`SELECT DISTINCT s.technique_id FROM app.step s
		 JOIN app.scenario sc ON s.scenario_id = sc.id
		 WHERE sc.engagement_id = $1 AND s.technique_id IS NOT NULL AND s.technique_id != ''`,
		fx.Eng1ID)
	if err != nil { t.Fatalf("list tech IDs: %v", err) }
	var techIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil { rows.Close(); t.Fatalf("scan: %v", err) }
		techIDs = append(techIDs, tid)
	}
	rows.Close()

	// Run the N+1 pattern 20 times to ensure it exceeds budget even
	// on CI-scaled data. A single pass is fast with 50 steps, but
	// a broken rollup accumulates across repeated calls in production.
	brokenStart := time.Now()
	for range 20 {
		for _, tid := range techIDs {
			r2, err := fx.DB.Read().QueryContext(ctx,
				`SELECT e.detection_category, e.protection FROM app.execution e
				 JOIN app.step s ON s.id = e.step_id
				 JOIN app.scenario sc ON s.scenario_id = sc.id
				 WHERE sc.engagement_id = $1 AND s.technique_id = $2 AND e.status IN ('complete', 'blocked')`,
				fx.Eng1ID, tid)
			if err != nil { continue }
			for r2.Next() {}
			r2.Close()
		}
	}
	brokenDur := time.Since(brokenStart)
	t.Logf("broken N+1: %v (baseline %v)", brokenDur, baselineDur)

	if brokenDur <= rollupP95Budget {
		t.Errorf("broken N+1 (%v) within budget %v — gate would not catch regression", brokenDur, rollupP95Budget)
	} else {
		t.Logf("gate correctly catches broken query: %v > budget %v", brokenDur, rollupP95Budget)
	}
}

func TestAnalyticsQueryLoad(t *testing.T) {
	if !config.LoadTestEnabled() {
		t.Skip("BLACKLIGHT_LOADTEST not set")
	}
	fx := seedAnalyticsFixture(t, loadTechCount, loadTacticCount, loadScenarios, loadStepsPerScen, loadFindings, loadHistoryPerFnd, loadEvidenceBlobs)
	q := analytics.NewQueries(fx.DB)

	ctx, cancel := context.WithTimeout(context.Background(), loadDuration+10*time.Second)
	defer cancel()

	writerCtx, writerCancel := context.WithTimeout(ctx, loadDuration)
	defer writerCancel()
	cfg := writerConfig{db: fx.DB, engID: fx.Eng1ID, execIDs: fx.Eng1ExecIDs, userID: "01900000-0000-7000-8000-000000000010"}
	waitWriters := runWriters(writerCtx, cfg, loadWorkers)

	var writeMu sync.Mutex
	var writeLatencies []time.Duration
	probeCtx, probeCancel := context.WithTimeout(ctx, loadDuration)
	defer probeCancel()
	go func() {
		ticker := time.NewTicker(probeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-probeCtx.Done(): return
			case <-ticker.C:
				d := writeProbeSample(probeCtx, fx.DB, fx.Eng1ExecIDs, "01900000-0000-7000-8000-000000000010")
				writeMu.Lock()
				writeLatencies = append(writeLatencies, d)
				writeMu.Unlock()
			}
		}
	}()

	measureCtx, measureCancel := context.WithTimeout(ctx, loadDuration)
	defer measureCancel()
	samples := runAllAnalytics(measureCtx, q, fx)
	waitWriters()

	t.Log("--- Full Load Results ---")
	rollups := []string{"TechniqueCoverage", "TacticCoverage", "CategoryDistribution", "ProtectionRate", "OutcomeMix", "ModifierDistribution", "MTTD", "Burndown", "FindingsBySeverity", "Compare", "Navigator", "ExecutionsExport", "FindingsExport"}
	failures := 0
	for _, name := range rollups {
		s := samples[name]
		if len(s) == 0 { t.Errorf("%s: no samples", name); failures++; continue }
		m := measureRollup(name, s)
		t.Logf("%s: p50=%v p95=%v max=%v n=%d", name, m.P50, m.P95, m.Max, m.Count)
		if m.P95 > rollupP95Budget { t.Errorf("%s: p95 %v > budget %v", name, m.P95, rollupP95Budget); failures++ }
		if m.Max > rollupMaxBudget { t.Errorf("%s: max %v > budget %v", name, m.Max, rollupMaxBudget); failures++ }
	}
	if dash := samples["DashboardSet"]; len(dash) > 0 {
		m := measureRollup("DashboardSet", dash)
		t.Logf("DashboardSet: p95=%v", m.P95)
		if m.P95 > dashboardBudget { t.Errorf("DashboardSet: p95 %v > budget %v", m.P95, dashboardBudget); failures++ }
	}
	writeMu.Lock()
	sort.Slice(writeLatencies, func(i, j int) bool { return writeLatencies[i] < writeLatencies[j] })
	wP95 := percentile(writeLatencies, 95)
	wMax := maxDuration(writeLatencies)
	writeMu.Unlock()
	t.Logf("write probe: p95=%v max=%v n=%d", wP95, wMax, len(writeLatencies))
	if wP95 > writeP95Budget { t.Errorf("write p95 %v > budget %v", wP95, writeP95Budget); failures++ }
	if wMax > writeMaxBudget { t.Errorf("write max %v > budget %v", wMax, writeMaxBudget); failures++ }
	if failures > 0 { t.Errorf("%d budget failures", failures) }
	measureArchiveMemory(t, fx.DB, fx.EvStore, fx.Eng1ID)
}

func TestArchiveExportMemory(t *testing.T) {
	fx := seedAnalyticsFixture(t, ciTechCount, ciTacticCount, ciScenarios, ciStepsPerScen, ciFindings, ciHistoryPerFnd, ciEvidenceBlobs)
	measureArchiveMemory(t, fx.DB, fx.EvStore, fx.Eng1ID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func randInt(n int) int {
	if n <= 0 { return 0 }
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil { panic(fmt.Sprintf("randInt: %v", err)) }
	return int(v.Int64())
}

func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 { return 0 }
	if p <= 0 { return samples[0] }
	if p >= 100 { return samples[len(samples)-1] }
	k := (p * len(samples)) / 100
	if k >= len(samples) { k = len(samples) - 1 }
	return samples[k]
}

func maxDuration(samples []time.Duration) time.Duration {
	if len(samples) == 0 { return 0 }
	m := samples[0]
	for _, s := range samples[1:] {
		if s > m { m = s }
	}
	return m
}

type rollupMeasurement struct {
	Name  string
	P50   time.Duration
	P95   time.Duration
	Max   time.Duration
	Count int
}

func measureRollup(name string, samples []time.Duration) rollupMeasurement {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return rollupMeasurement{
		Name: name, P50: percentile(samples, 50), P95: percentile(samples, 95),
		Max: maxDuration(samples), Count: len(samples),
	}
}
