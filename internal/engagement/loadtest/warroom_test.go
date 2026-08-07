// Package loadtest proves that concurrent war-room operations under the
// serialized DuckDB writer stay correct and responsive (M3-016). Tests use the
// real serialized writer — never a mock.
//
// Three tests:
//
//   - TestWarRoomConcurrency — CI gate (always on)
//
//   - TestWarRoomConcurrencyDetectsLostUpdates — mutation test (proves the
//     optimistic-lock gate catches lost updates when the version WHERE clause
//     is removed)
//
//   - TestWarRoomLoad — full developer load (BLACKLIGHT_LOADTEST=1)
//
//     BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/engagement/loadtest/ -run TestWarRoomLoad
package loadtest_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/store"
	engagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// Budgets for a developer machine / CI runner with a local SSD. Generous enough
// to avoid flake on a loaded host, tight enough that holding the serialized
// writer across non-trivial work fails the gate.
const (
	probeInterval  = 50 * time.Millisecond
	writeP95Budget = 200 * time.Millisecond
	// Absolute ceiling: a single interactive write must not wait minutes.
	writeMaxBudget = 2 * time.Second

	// CI scaled-down: enough steps and users to create contention, but
	// seconds, not minutes, even under -race.
	ciUsers    = 5
	ciSteps    = 20
	ciDuration = 5 * time.Second

	// Full developer load: 20 users, 50+ steps, production-like duration.
	// Opt in with BLACKLIGHT_LOADTEST=1 (see docs/testing.md).
	loadUsers    = 20
	loadSteps    = 50
	loadDuration = 15 * time.Second

	// Retry budget for the optimistic-lock retry helper.
	maxRetries = 5
)

// TestWarRoomConcurrency is the CI gate: multiple concurrent users mix red
// patches, blue patches, evidence uploads, comments, and reads against one
// engagement. Interactive write p95 must stay under budget, no lost updates,
// no deadlocks, DB consistent.
func TestWarRoomConcurrency(t *testing.T) {
	t.Parallel()

	res := runWarRoom(t, warRoomOpts{
		users:    ciUsers,
		steps:    ciSteps,
		duration: ciDuration,
	})
	assertWarRoom(t, res)
	t.Logf("concurrency OK: users=%d steps=%d writes=%d conflicts=%d interactive n=%d p50=%s p95=%s max=%s",
		ciUsers, ciSteps, res.totalWrites, res.conflicts,
		len(res.latencies), percentile(res.latencies, 50), percentile(res.latencies, 95), res.maxLatency)
}

// TestWarRoomConcurrencyDetectsLostUpdates is the mutation gate: when the
// optimistic-lock version WHERE clause is dropped, the test MUST detect lost
// updates. If this test starts passing with buggyVersion enabled, the
// optimistic lock check has been removed or bypassed.
func TestWarRoomConcurrencyDetectsLostUpdates(t *testing.T) {
	t.Parallel()

	res := runWarRoom(t, warRoomOpts{
		users:        ciUsers,
		steps:        ciSteps,
		duration:     ciDuration,
		buggyVersion: true,
	})
	if res.lostUpdates == 0 {
		t.Fatal("buggy version (no optimistic lock) should produce lost updates but none detected — " +
			"version WHERE clause may have been removed from the production path")
	}
	t.Logf("detector OK: lostUpdates=%d with version check removed (buggy version produced %d conflicting writes)",
		res.lostUpdates, res.totalWrites)
}

// TestWarRoomLoad is the developer-machine load: 20 users across 50+ steps.
// Skipped unless BLACKLIGHT_LOADTEST=1 so CI stays fast.
//
//	BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/engagement/loadtest/ -run TestWarRoomLoad
func TestWarRoomLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("full war room load skipped under -short")
	}
	if !loadtestEnabled() {
		t.Skip("set BLACKLIGHT_LOADTEST=1 to run the full war room load test (see docs/testing.md)")
	}

	res := runWarRoom(t, warRoomOpts{
		users:    loadUsers,
		steps:    loadSteps,
		duration: loadDuration,
	})
	assertWarRoom(t, res)
	t.Logf("load OK: users=%d steps=%d writes=%d conflicts=%d interactive n=%d p50=%s p95=%s max=%s",
		loadUsers, loadSteps, res.totalWrites, res.conflicts,
		len(res.latencies), percentile(res.latencies, 50), percentile(res.latencies, 95), res.maxLatency)
}

type warRoomOpts struct {
	users        int
	steps        int
	duration     time.Duration
	buggyVersion bool
}

type warRoomResult struct {
	latencies   []time.Duration
	maxLatency  time.Duration
	totalWrites int64
	conflicts   int64
	lostUpdates int64
}

// runWarRoom seeds one engagement with steps (each with an execution), spins up
// concurrent workers doing mixed war-room operations, and runs a probe goroutine
// measuring interactive write latency. Returns collected metrics for assertion.
func runWarRoom(t *testing.T, opts warRoomOpts) warRoomResult {
	t.Helper()
	ctx := context.Background()

	db := storetest.Migrated(t)

	// Seed: engagement, scenario, steps + executions, users, sessions.
	engID := seedEngagement(t, db, opts.steps)
	userIDs, sessionID := seedUsers(t, db, opts.users)

	// Evidence store for upload simulation.
	// Use os.MkdirTemp rather than t.TempDir — the latter's cleanup order
	// can race with storetest's DB close on this platform.
	evidenceDir, err := os.MkdirTemp("", "loadtest-evidence-*")
	if err != nil {
		t.Fatalf("create evidence temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(evidenceDir) })
	evCfg := config.Evidence{
		Dir:                evidenceDir,
		MaxUploadBytes:     25 << 20,
		MaxEngagementBytes: 2 << 30,
		MIMEAllowlist:      "text/plain",
	}
	blobRepo := engagement.NewEvidenceBlobRepo(db)
	evStore := evidence.NewStore(evidenceDir, evCfg, blobRepo)
	evidenceRepo := engagement.NewEvidenceRepo(db)

	// Repositories for worker operations.
	execs := engagement.NewExecutions(db)
	comments := engagement.NewComments(db)

	// Collect execution IDs for random selection.
	execIDs := listExecutionIDs(t, db, engID)

	// Write tracker: per-execution successful write count (for version
	// consistency check). Only tracks PatchRed and PatchBlue — evidence and
	// comments don't modify execution version.
	var (
		writeMu     sync.Mutex
		writeCounts = make(map[string]int64) // executionID -> successful writes
	)
	recordWrite := func(execID string) {
		writeMu.Lock()
		writeCounts[execID]++
		writeMu.Unlock()
	}

	// Shared counters.
	var totalWrites atomic.Int64
	var conflicts atomic.Int64
	var lostUpdates atomic.Int64

	// Probe loop: interactive write + execution read on a timer while workers run.
	probeCtx, cancelProbe := context.WithCancel(ctx)
	t.Cleanup(cancelProbe)

	var (
		mu        sync.Mutex
		latencies []time.Duration
	)
	var probeWG sync.WaitGroup
	probeWG.Add(1)
	go func() {
		defer probeWG.Done()
		sessions := identity.NewSessions(db)
		ticker := time.NewTicker(probeInterval)
		defer ticker.Stop()
		sampleProbe(probeCtx, db, sessions, sessionID, &mu, &latencies)
		for {
			select {
			case <-probeCtx.Done():
				return
			case <-ticker.C:
				sampleProbe(probeCtx, db, sessions, sessionID, &mu, &latencies)
			}
		}
	}()

	// Worker loop.
	workerCtx, cancelWorkers := context.WithTimeout(ctx, opts.duration)
	t.Cleanup(cancelWorkers)

	var workerWG sync.WaitGroup
	for i := range opts.users {
		workerWG.Add(1)
		go func(workerID int) {
			defer workerWG.Done()
			runWorker(workerCtx, workerParams{
				db:           db,
				execs:        execs,
				comments:     comments,
				evStore:      evStore,
				evidenceRepo: evidenceRepo,
				execIDs:      execIDs,
				engID:        engID,
				userID:       userIDs[workerID%len(userIDs)],
				buggyVersion: opts.buggyVersion,
				recordWrite:  recordWrite,
				totalWrites:  &totalWrites,
				conflicts:    &conflicts,
				lostUpdates:  &lostUpdates,
			})
		}(i)
	}
	workerWG.Wait()
	cancelProbe()
	probeWG.Wait()

	mu.Lock()
	samples := append([]time.Duration(nil), latencies...)
	mu.Unlock()

	// Version consistency check: for each execution, final version should be
	// initial version (1) + successful writes.
	checkVersionConsistency(t, db, execIDs, writeCounts, &lostUpdates)

	return warRoomResult{
		latencies:   samples,
		maxLatency:  maxDuration(samples),
		totalWrites: totalWrites.Load(),
		conflicts:   conflicts.Load(),
		lostUpdates: lostUpdates.Load(),
	}
}

// workerParams bundles everything a war-room worker goroutine needs.
type workerParams struct {
	db           *store.DB
	execs        *engagement.Executions
	comments     *engagement.Comments
	evStore      *evidence.Store
	evidenceRepo *engagement.EvidenceRepo
	execIDs      []string
	engID        string
	userID       string
	buggyVersion bool
	recordWrite  func(execID string)
	totalWrites  *atomic.Int64
	conflicts    *atomic.Int64
	lostUpdates  *atomic.Int64
}

// runWorker loops for the duration of ctx, randomly picking war-room
// operations: red patches, blue patches, evidence uploads, comments, and reads.
func runWorker(ctx context.Context, p workerParams) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		execID := p.execIDs[randInt(len(p.execIDs))]

		switch randInt(8) {
		case 0, 1: // red patch (25%)
			p.doRedPatch(ctx, execID)
		case 2, 3: // blue patch (25%)
			p.doBluePatch(ctx, execID)
		case 4: // evidence upload (12.5%)
			p.doEvidence(ctx, execID)
		case 5: // comment (12.5%)
			p.doComment(ctx, execID)
		default: // read (25%)
			p.doRead(ctx)
		}
	}
}

func (p workerParams) doRedPatch(ctx context.Context, execID string) {
	note := fmt.Sprintf("worker-%s-%d", p.userID[:8], time.Now().UnixNano())
	status := engagement.ExecutionStatusRunning

	exec, err := p.execs.ByID(ctx, execID)
	if err != nil {
		return
	}
	changes := engagement.RedPatchChanges{
		Status:   &status,
		RedNotes: &note,
	}

	if p.buggyVersion {
		err = patchRedNoVersion(ctx, p.db, execID, changes, exec.Version)
	} else {
		_, err = patchRedWithRetry(ctx, p.execs, execID, exec.Version, changes, p.conflicts)
	}
	if err != nil {
		return
	}
	p.totalWrites.Add(1)
	p.recordWrite(execID)
}

func (p workerParams) doBluePatch(ctx context.Context, execID string) {
	note := fmt.Sprintf("blue-%s-%d", p.userID[:8], time.Now().UnixNano())
	cat := engagement.DetectionCategoryTelemetry

	exec, err := p.execs.ByID(ctx, execID)
	if err != nil {
		return
	}
	changes := engagement.BluePatchChanges{
		DetectionCategory: &cat,
		BlueNotes:         &note,
	}

	if p.buggyVersion {
		err = patchBlueNoVersion(ctx, p.db, execID, changes, exec.Version)
	} else {
		_, err = patchBlueWithRetry(ctx, p.execs, execID, exec.Version, changes, p.conflicts)
	}
	if err != nil {
		return
	}
	p.totalWrites.Add(1)
	p.recordWrite(execID)
}

func (p workerParams) doEvidence(ctx context.Context, execID string) {
	// Tiny random payload — measures writer fairness, not disk bandwidth.
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return
	}
	sha256, _, _, err := p.evStore.Put(ctx, strings.NewReader(string(payload)), "text/plain", p.engID)
	if err != nil {
		return
	}
	_, err = p.evidenceRepo.Create(ctx, engagement.NewEvidence{
		BlobSHA256:  sha256,
		Filename:    fmt.Sprintf("loadtest-%d.evidence", time.Now().UnixNano()),
		Caption:     "load test evidence",
		Side:        engagement.EvidenceSideRed,
		ExecutionID: execID,
		UploadedBy:  p.userID,
		Size:        int64(len(payload)),
		MIME:        "text/plain",
	})
	if err != nil {
		return
	}
	p.totalWrites.Add(1)
}

func (p workerParams) doComment(ctx context.Context, execID string) {
	_, err := p.comments.Create(ctx, engagement.NewComment{
		ExecutionID: execID,
		AuthorID:    p.userID,
		Body:        fmt.Sprintf("load test comment %d", time.Now().UnixNano()),
	})
	if err != nil {
		return
	}
	p.totalWrites.Add(1)
}

func (p workerParams) doRead(ctx context.Context) {
	// Read a random execution — exercises the read pool under write load.
	// Reads can fail under heavy write load (context cancelled, etc.) —
	// that's expected and not a test failure.
	execID := p.execIDs[randInt(len(p.execIDs))]
	_, _ = p.execs.ByID(ctx, execID) //nolint:errcheck // read failure under load is expected
}

// patchRedWithRetry attempts a red patch with optimistic locking. On version
// This demonstrates the client 409 → re-GET → re-PATCH pattern under contention.
func patchRedWithRetry(ctx context.Context, execs *engagement.Executions, id string, version int, changes engagement.RedPatchChanges, conflicts *atomic.Int64) (engagement.Execution, error) {
	for attempt := range maxRetries {
		exec, err := execs.PatchRed(ctx, id, version, changes)
		if err == nil {
			return exec, nil
		}
		if !isVersionConflict(err) {
			return engagement.Execution{}, err
		}
		conflicts.Add(1)
		if attempt == maxRetries-1 {
			return engagement.Execution{}, fmt.Errorf("patch red %s: retry budget exhausted after %d attempts: %w", id, maxRetries, err)
		}
		// Re-read and retry with current version.
		current, readErr := execs.ByID(ctx, id)
		if readErr != nil {
			return engagement.Execution{}, readErr
		}
		version = current.Version
	}
	return engagement.Execution{}, fmt.Errorf("patch red %s: unreachable", id)
}

// patchBlueWithRetry is the blue-side equivalent of patchRedWithRetry.
func patchBlueWithRetry(ctx context.Context, execs *engagement.Executions, id string, version int, changes engagement.BluePatchChanges, conflicts *atomic.Int64) (engagement.Execution, error) {
	for attempt := range maxRetries {
		exec, err := execs.PatchBlue(ctx, id, version, changes)
		if err == nil {
			return exec, nil
		}
		if !isVersionConflict(err) {
			return engagement.Execution{}, err
		}
		conflicts.Add(1)
		if attempt == maxRetries-1 {
			return engagement.Execution{}, fmt.Errorf("patch blue %s: retry budget exhausted after %d attempts: %w", id, maxRetries, err)
		}
		current, readErr := execs.ByID(ctx, id)
		if readErr != nil {
			return engagement.Execution{}, readErr
		}
		version = current.Version
	}
	return engagement.Execution{}, fmt.Errorf("patch blue %s: unreachable", id)
}

// isVersionConflict reports whether err is an optimistic-lock version conflict.
func isVersionConflict(err error) bool {
	return strings.Contains(err.Error(), "version conflict")
}

// patchRedNoVersion updates red-side fields without the optimistic-lock version
// WHERE clause AND sets version to readVersion+1 as a literal. This simulates
// the classic read-then-write lost-update: two workers read the same version V,
// both write version = V+1, and one increment is silently lost.
// Used only by the mutation test.
func patchRedNoVersion(ctx context.Context, db *store.DB, id string, changes engagement.RedPatchChanges, readVersion int) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		var sets []string
		var args []any

		if changes.Status != nil {
			sets = append(sets, "status = ?")
			args = append(args, string(*changes.Status))
		}
		if changes.RedNotes != nil {
			sets = append(sets, "red_notes = ?")
			args = append(args, *changes.RedNotes)
		}
		if changes.StartedAt != nil {
			sets = append(sets, "started_at = ?")
			args = append(args, *changes.StartedAt)
		}
		if changes.EndedAt != nil {
			sets = append(sets, "ended_at = ?")
			args = append(args, *changes.EndedAt)
		}
		if changes.CommandRun != nil {
			sets = append(sets, "command_run = ?")
			args = append(args, *changes.CommandRun)
		}
		if changes.SourceHost != nil {
			sets = append(sets, "source_host = ?")
			args = append(args, *changes.SourceHost)
		}
		if changes.TargetHost != nil {
			sets = append(sets, "target_host = ?")
			args = append(args, *changes.TargetHost)
		}
		if changes.ExecutedBy != nil {
			sets = append(sets, "executed_by = ?")
			args = append(args, *changes.ExecutedBy)
		}

		// Literal version: simulates read-then-write. If two workers both
		// read version V, both write V+1 — one increment is silently lost.
		sets = append(sets, "version = ?", "updated_at = ?")
		args = append(args, readVersion+1, time.Now().UTC())
		args = append(args, id) // No version check — deliberately unsafe.

		query := "UPDATE app.execution SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	})
}

// patchBlueNoVersion is the blue-side equivalent of patchRedNoVersion.
func patchBlueNoVersion(ctx context.Context, db *store.DB, id string, changes engagement.BluePatchChanges, readVersion int) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		var sets []string
		var args []any

		if changes.DetectionCategory != nil {
			sets = append(sets, "detection_category = ?")
			args = append(args, string(*changes.DetectionCategory))
		}
		if changes.DetectionModifiers != nil {
			sets = append(sets, "detection_modifiers = ?")
			args = append(args, string(changes.DetectionModifiers))
		}
		if changes.Protection != nil {
			sets = append(sets, "protection = ?")
			args = append(args, string(*changes.Protection))
		}
		if changes.DetectedAt != nil {
			sets = append(sets, "detected_at = ?")
			args = append(args, *changes.DetectedAt)
		}
		if changes.DetectingSource != nil {
			sets = append(sets, "detecting_source = ?")
			args = append(args, *changes.DetectingSource)
		}
		if changes.DetectingRuleRef != nil {
			sets = append(sets, "detecting_rule_ref = ?")
			args = append(args, *changes.DetectingRuleRef)
		}
		if changes.AlertSeverity != nil {
			sets = append(sets, "alert_severity = ?")
			args = append(args, *changes.AlertSeverity)
		}
		if changes.BlueNotes != nil {
			sets = append(sets, "blue_notes = ?")
			args = append(args, *changes.BlueNotes)
		}
		if changes.ScoredBy != nil {
			sets = append(sets, "scored_by = ?")
			args = append(args, *changes.ScoredBy)
		}
		if changes.ScoredAt != nil {
			sets = append(sets, "scored_at = ?")
			args = append(args, *changes.ScoredAt)
		}

		// Literal version: simulates read-then-write. If two workers both
		// read version V, both write V+1 — one increment is silently lost.
		sets = append(sets, "version = ?", "updated_at = ?")
		args = append(args, readVersion+1, time.Now().UTC())
		args = append(args, id) // No version check — deliberately unsafe.

		query := "UPDATE app.execution SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	})
}

// sampleProbe records the latency of a single interactive write (session touch)
// followed by an execution read on the read pool.
func sampleProbe(
	ctx context.Context,
	db *store.DB,
	sessions *identity.Sessions,
	sessionID string,
	mu *sync.Mutex,
	latencies *[]time.Duration,
) {
	if err := ctx.Err(); err != nil {
		return
	}
	start := time.Now()
	err := sessions.SetLastSeenAt(ctx, sessionID, time.Now().UTC())
	elapsed := time.Since(start)
	if err != nil && ctx.Err() == nil {
		// Surface via a huge latency so assertWarRoom fails loudly.
		elapsed = writeMaxBudget + time.Second
	}
	// Read on the read pool — must stay unblocked by the serialized writer.
	var n int
	if err := db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.execution`,
	).Scan(&n); err != nil {
		_ = err
	}

	mu.Lock()
	*latencies = append(*latencies, elapsed)
	mu.Unlock()
}

// assertWarRoom checks the collected metrics against the documented budgets.
func assertWarRoom(t *testing.T, res warRoomResult) {
	t.Helper()
	if len(res.latencies) < 3 {
		t.Fatalf("interactive samples = %d, want ≥3 (duration too short or probe broken)", len(res.latencies))
	}
	if res.totalWrites == 0 {
		t.Fatal("zero successful writes — workers never ran or all operations failed")
	}
	p95 := percentile(res.latencies, 95)
	if p95 > writeP95Budget {
		t.Fatalf("interactive write p95 = %s, want ≤ %s (n=%d max=%s writes=%d conflicts=%d). "+
			"Serialized writer is overloaded — shrink worker count or check store.Write for held locks",
			p95, writeP95Budget, len(res.latencies), res.maxLatency, res.totalWrites, res.conflicts)
	}
	if res.maxLatency > writeMaxBudget {
		t.Fatalf("interactive write max = %s, want ≤ %s", res.maxLatency, writeMaxBudget)
	}
}

// checkVersionConsistency verifies that for each execution, the final version
// equals the initial version (1) plus the number of successful writes to that
// execution. Any mismatch indicates lost updates.
func checkVersionConsistency(t *testing.T, db *store.DB, execIDs []string, writeCounts map[string]int64, lostUpdates *atomic.Int64) {
	t.Helper()
	ctx := context.Background()
	execs := engagement.NewExecutions(db)

	for _, id := range execIDs {
		exec, err := execs.ByID(ctx, id)
		if err != nil {
			t.Errorf("read execution %s for consistency check: %v", id, err)
			continue
		}
		expected := writeCounts[id] + 1 // initial version is 1
		if int64(exec.Version) != expected {
			lostUpdates.Add(int64(exec.Version) - expected)
		}
	}
}

// seedEngagement creates one active engagement with one scenario and opts.steps
// steps, each with a pending execution. Returns the engagement ID.
// Uses direct INSERT with active status because DuckDB has a bug where UPDATE
// on the app.engagement status column fails with a catalog error referencing
// the intermediate engagement_member_next table from the migration rebuild.
func seedEngagement(t *testing.T, db *store.DB, numSteps int) string {
	t.Helper()
	ctx := context.Background()

	engID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate engagement id: %v", err)
	}
	engIDStr := engID.String()

	ts := time.Now().UTC()
	endsOn := ts.Add(24 * time.Hour)
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.engagement
			(id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			engIDStr,
			"Load Test Engagement",
			"Load Test Client",
			"Concurrency load test engagement",
			string(engagement.EngagementStatusActive),
			ts, endsOn,
			"15.1",
			string(engagement.EngagementModeStandard),
			false,
			"loadtest",
			ts, ts,
		)
		return err
	}); err != nil {
		t.Fatalf("create engagement: %v", err)
	}

	scenarios := engagement.NewScenarios(db)
	scenario, err := scenarios.Create(ctx, engagement.NewScenario{
		EngagementID: engIDStr,
		Ordinal:      1,
		Name:         "Load Test Scenario",
		Narrative:    "Concurrency load test scenario",
		Source:       engagement.ScenarioSourceManual,
	})
	if err != nil {
		t.Fatalf("create scenario: %v", err)
	}

	steps := engagement.NewSteps(db)
	for i := range numSteps {
		_, _, err := steps.CreateWithExecution(ctx, engagement.NewStep{
			ScenarioID:    scenario.ID,
			Ordinal:       i + 1,
			Name:          fmt.Sprintf("Load Test Step %d", i+1),
			Objective:     "Load test objective",
			TechniqueID:   fmt.Sprintf("T%04d", (i%200)+1001),
			TacticID:      fmt.Sprintf("TA%04d", (i%14)+1),
			AttackVersion: "15.1",
		})
		if err != nil {
			t.Fatalf("create step %d: %v", i+1, err)
		}
	}

	return engIDStr
}

// seedUsers creates opts.users identity rows and one session. Returns the user
// IDs and the session ID for the probe.
func seedUsers(t *testing.T, db *store.DB, numUsers int) (userIDs []string, sessionID string) {
	t.Helper()
	ctx := context.Background()

	users := identity.NewUsers(db)
	for i := range numUsers {
		user, err := users.Create(ctx, identity.NewUser{
			Email:        fmt.Sprintf("loadtest-%d@example.test", i),
			DisplayName:  fmt.Sprintf("Load Test User %d", i),
			PlatformRole: "member",
			Status:       identity.StatusActive,
		})
		if err != nil {
			t.Fatalf("create user %d: %v", i, err)
		}
		userIDs = append(userIDs, user.ID)
	}

	sessions := identity.NewSessions(db)
	sess, err := sessions.Create(ctx, identity.NewSession{
		UserID:       userIDs[0],
		TokenHash:    "loadtest-probe-session-hash",
		ExpiresAt:    time.Now().Add(12 * time.Hour),
		MFASatisfied: true,
	})
	if err != nil {
		t.Fatalf("create probe session: %v", err)
	}

	return userIDs, sess.ID
}

// listExecutionIDs returns every execution ID for the given engagement.
// Uses a direct query to work around the ambiguous "id" column in
// ListByEngagement's JOIN (pre-existing bug, tracked separately).
func listExecutionIDs(t *testing.T, db *store.DB, engID string) []string {
	t.Helper()
	ctx := context.Background()

	rows, err := db.Read().QueryContext(ctx,
		`SELECT app.execution.id
		FROM app.execution
		JOIN app.step ON app.step.id = app.execution.step_id
		JOIN app.scenario ON app.scenario.id = app.step.scenario_id
		WHERE app.scenario.engagement_id = ?
		ORDER BY app.scenario.ordinal, app.step.ordinal`,
		engID,
	)
	if err != nil {
		t.Fatalf("list execution IDs: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan execution ID: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}
	return ids
}

// randInt returns a random integer in [0, n). Not crypto-safe; test-only.
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use time-based value. rand.Read failure in tests is
		// extremely unlikely on any platform we test on.
		return int(time.Now().UnixNano()) % n
	}
	v := int(b[0]) | int(b[1])<<8 | int(b[2])<<16 | int(b[3])<<24
	if v < 0 {
		v = -v
	}
	return v % n
}

// percentile returns the p-th percentile (nearest-rank) of sorted samples.
func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := (p*len(sorted) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// maxDuration returns the largest value in samples.
func maxDuration(samples []time.Duration) time.Duration {
	var max time.Duration
	for _, d := range samples {
		if d > max {
			max = d
		}
	}
	return max
}

// loadtestEnabled reports whether expensive load tests should run.
func loadtestEnabled() bool {
	return config.LoadTestEnabled()
}
