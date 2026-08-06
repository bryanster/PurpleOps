// Package loadtest proves content sync does not starve interactive store
// writers (M2-016). Tests use the real serialized writer — never a mock.
package loadtest_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// Budgets for a developer machine / CI runner with a local SSD. Generous enough
// to avoid flake on a loaded host, tight enough that holding the write lock
// across a ≥100ms sleep in Apply fails the gate.
const (
	interactiveInterval  = 50 * time.Millisecond
	interactiveP95Budget = 200 * time.Millisecond
	// Absolute ceiling: a single interactive write must not wait for a whole
	// sync. 2s is far above a healthy batch and far below a job timeout.
	interactiveMaxBudget = 2 * time.Second
	// Hold duration used by the fault-injection test. Above the p95 budget so
	// a lock held across the sleep is unambiguous.
	faultHoldWrite = 250 * time.Millisecond

	// CI scaled-down: multi-batch but seconds, not minutes, even under -race.
	ciNoteCount  = 200
	ciWriteBatch = 25

	// Full developer load: tens of thousands of rows at the production default
	// batch size. Opt in with BLACKLIGHT_LOADTEST=1 (see docs/testing.md).
	loadNoteCount  = 20_000
	loadWriteBatch = 250
)

// TestSyncWriteFairness is the CI gate: a multi-batch fixture sync runs while
// session-touch writes and content reads fire every 50ms. Interactive p95 must
// stay under interactiveP95Budget and the sync must succeed.
func TestSyncWriteFairness(t *testing.T) {
	t.Parallel()

	res := runFairness(t, fairnessOpts{
		notes:      ciNoteCount,
		writeBatch: ciWriteBatch,
	})
	assertFair(t, res)
	t.Logf("fairness OK: notes=%d batches≈%d sync=%s interactive n=%d p50=%s p95=%s max=%s",
		ciNoteCount, res.batchesApplied, res.syncDuration,
		len(res.latencies), percentile(res.latencies, 50), percentile(res.latencies, 95), res.maxLatency)
}

// TestSyncWriteFairnessDetectsLockHold keeps the mutation check alive: when
// Apply sleeps ≥100ms inside store.Write, interactive p95 exceeds the budget.
// If this test starts passing with HoldWrite set, the fairness gate is broken.
func TestSyncWriteFairnessDetectsLockHold(t *testing.T) {
	t.Parallel()

	res := runFairness(t, fairnessOpts{
		notes:      ciNoteCount,
		writeBatch: ciWriteBatch,
		holdWrite:  faultHoldWrite,
	})
	if res.syncErr != nil {
		t.Fatalf("sync under fault injection: %v", res.syncErr)
	}
	if res.jobStatus != storecontent.JobStatusSucceeded {
		t.Fatalf("sync status = %s error=%q, want succeeded", res.jobStatus, res.jobError)
	}
	p95 := percentile(res.latencies, 95)
	if p95 < interactiveP95Budget {
		t.Fatalf("HoldWrite=%s should push interactive p95 over the %s budget; got p95=%s max=%s n=%d — "+
			"fairness detector is too loose or HoldWrite is not inside the write lock",
			faultHoldWrite, interactiveP95Budget, p95, res.maxLatency, len(res.latencies))
	}
	if res.maxLatency < faultHoldWrite {
		t.Fatalf("max interactive latency %s is below HoldWrite %s; probes never observed the held lock",
			res.maxLatency, faultHoldWrite)
	}
	t.Logf("detector OK: HoldWrite=%s p95=%s max=%s (budget %s would fail)",
		faultHoldWrite, p95, res.maxLatency, interactiveP95Budget)
}

// TestSyncWriteLoad is the developer-machine load: 20k notes at the production
// WriteBatch default. Skipped unless BLACKLIGHT_LOADTEST=1 so CI stays fast.
//
//	BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/content/loadtest/ -run TestSyncWriteLoad
func TestSyncWriteLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("full sync write load skipped under -short")
	}
	if !loadtestEnabled() {
		t.Skip("set BLACKLIGHT_LOADTEST=1 to run the full sync write load test (see docs/testing.md)")
	}

	res := runFairness(t, fairnessOpts{
		notes:      loadNoteCount,
		writeBatch: loadWriteBatch,
	})
	assertFair(t, res)
	t.Logf("load OK: notes=%d batch=%d sync=%s interactive n=%d p50=%s p95=%s max=%s",
		loadNoteCount, loadWriteBatch, res.syncDuration,
		len(res.latencies), percentile(res.latencies, 50), percentile(res.latencies, 95), res.maxLatency)
}

type fairnessOpts struct {
	notes      int
	writeBatch int
	holdWrite  time.Duration
}

type fairnessResult struct {
	latencies      []time.Duration
	maxLatency     time.Duration
	syncDuration   time.Duration
	syncErr        error
	jobStatus      storecontent.JobStatus
	jobError       string
	batchesApplied int64
	noteCount      int
}

func runFairness(t *testing.T, opts fairnessOpts) fairnessResult {
	t.Helper()
	ctx := context.Background()

	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	fixture.FetchBytes = content.FixtureBundle(storecontent.VersionCurrent, manyNotes(opts.notes))
	fixture.HoldWrite = opts.holdWrite

	runner, err := content.NewRunner(content.RunnerDeps{
		DB:       db,
		Sources:  sources,
		Versions: versions,
		Jobs:     jobs,
		Paths:    paths,
		Activity: events.New(activity.New(db)),
		Adapters: map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
		},
		MaxBytes:   512 << 20,
		JobTimeout: 10 * time.Minute,
		WriteBatch: opts.writeBatch,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if err := runner.Boot(ctx); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	runner.Start(ctx)
	t.Cleanup(runner.Stop)

	sessionID := seedSession(t, db)

	// Probe loop: interactive write + content read on a timer while sync runs.
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
		ticker := time.NewTicker(interactiveInterval)
		defer ticker.Stop()
		// Fire once immediately so a fast sync still yields samples.
		sampleInteractive(probeCtx, db, sessions, sessionID, &mu, &latencies)
		for {
			select {
			case <-probeCtx.Done():
				return
			case <-ticker.C:
				sampleInteractive(probeCtx, db, sessions, sessionID, &mu, &latencies)
			}
		}
	}()

	syncStart := time.Now()
	job, err := runner.StartSync(ctx, authn.Subject{UserID: "loadtest", Email: "loadtest@example.test"}, content.StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		cancelProbe()
		probeWG.Wait()
		return fairnessResult{syncErr: err}
	}
	job, waitErr := runner.Wait(ctx, job.ID)
	syncDur := time.Since(syncStart)
	cancelProbe()
	probeWG.Wait()

	mu.Lock()
	samples := append([]time.Duration(nil), latencies...)
	mu.Unlock()

	var n int
	if err := db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ? AND version = ?`,
		storecontent.SourceIDAtomic, storecontent.VersionCurrent,
	).Scan(&n); err != nil {
		t.Fatalf("count notes: %v", err)
	}

	return fairnessResult{
		latencies:      samples,
		maxLatency:     maxDuration(samples),
		syncDuration:   syncDur,
		syncErr:        waitErr,
		jobStatus:      job.Status,
		jobError:       job.Error,
		batchesApplied: fixture.BatchesApplied(),
		noteCount:      n,
	}
}

func sampleInteractive(
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
	// Real store.Write path — the same lock content Apply uses.
	err := sessions.SetLastSeenAt(ctx, sessionID, time.Now().UTC())
	elapsed := time.Since(start)
	if err != nil && ctx.Err() == nil {
		// Surface via a huge latency so assertFair fails loudly rather than
		// silently dropping a failed probe.
		elapsed = interactiveMaxBudget + time.Second
	}
	// Content read on the read pool — must stay unblocked by writers.
	var n int
	if err := db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_source WHERE id = ?`,
		storecontent.SourceIDAtomic,
	).Scan(&n); err != nil {
		// Scan can fail on context deadline; that's expected under load.
		_ = err
	}

	mu.Lock()
	*latencies = append(*latencies, elapsed)
	mu.Unlock()
}

func assertFair(t *testing.T, res fairnessResult) {
	t.Helper()
	if res.syncErr != nil {
		t.Fatalf("sync Wait: %v", res.syncErr)
	}
	if res.jobStatus != storecontent.JobStatusSucceeded {
		t.Fatalf("sync status = %s error=%q, want succeeded", res.jobStatus, res.jobError)
	}
	if res.noteCount == 0 {
		t.Fatal("sync wrote zero notes")
	}
	if res.batchesApplied < 2 {
		t.Fatalf("batches applied = %d, want multi-batch (≥2 Write transactions)", res.batchesApplied)
	}
	if len(res.latencies) < 3 {
		t.Fatalf("interactive samples = %d, want ≥3 (sync too fast or probe broken)", len(res.latencies))
	}
	p95 := percentile(res.latencies, 95)
	if p95 > interactiveP95Budget {
		t.Fatalf("interactive write p95 = %s, want ≤ %s (n=%d max=%s sync=%s). "+
			"Apply is holding the serialized writer too long — shrink BLACKLIGHT_CONTENT_WRITE_BATCH "+
			"or stop holding store.Write across non-trivial work",
			p95, interactiveP95Budget, len(res.latencies), res.maxLatency, res.syncDuration)
	}
	if res.maxLatency > interactiveMaxBudget {
		t.Fatalf("interactive write max = %s, want ≤ %s", res.maxLatency, interactiveMaxBudget)
	}
}

func seedSession(t *testing.T, db *store.DB) string {
	t.Helper()
	ctx := context.Background()
	user, err := identity.NewUsers(db).Create(ctx, identity.NewUser{
		Email:        "loadtest@example.test",
		DisplayName:  "Load Test",
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	sess, err := identity.NewSessions(db).Create(ctx, identity.NewSession{
		UserID:       user.ID,
		TokenHash:    "loadtest-session-hash",
		ExpiresAt:    time.Now().Add(12 * time.Hour),
		MFASatisfied: true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return sess.ID
}

func manyNotes(n int) []content.FixtureNote {
	out := make([]content.FixtureNote, n)
	for i := range n {
		out[i] = content.FixtureNote{
			ExternalID: fmt.Sprintf("n-%d", i),
			Title:      fmt.Sprintf("Note %d", i),
			Body:       "loadtest body",
		}
	}
	return out
}

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
	// Nearest-rank: index = ceil(p/100 * N) - 1
	rank := (p*len(sorted) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func maxDuration(samples []time.Duration) time.Duration {
	var max time.Duration
	for _, d := range samples {
		if d > max {
			max = d
		}
	}
	return max
}

func loadtestEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BLACKLIGHT_LOADTEST"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
