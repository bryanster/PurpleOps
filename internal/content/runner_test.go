package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestRunnerSuccessWritesNotesAndRawSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)

	bundle := FixtureBundle(storecontent.VersionCurrent, []FixtureNote{
		{ExternalID: "n1", Title: "One", Body: "body-1"},
		{ExternalID: "n2", Title: "Two", Body: "body-2"},
	})
	rt.fixture.FetchBytes = bundle

	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("status = %s error=%q, want succeeded", job.Status, job.Error)
	}

	var n int
	if err := rt.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ? AND version = ?`,
		storecontent.SourceIDAtomic, storecontent.VersionCurrent,
	).Scan(&n); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if n != 2 {
		t.Fatalf("notes = %d, want 2", n)
	}

	ver, err := rt.versions.BySourceVersion(ctx, storecontent.SourceIDAtomic, storecontent.VersionCurrent)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	sum := sha256.Sum256(bundle)
	wantSHA := hex.EncodeToString(sum[:])
	if ver.RawSHA256 != wantSHA {
		t.Fatalf("raw sha = %s, want %s", ver.RawSHA256, wantSHA)
	}
	abs, err := rt.paths.Abs(ver.RawPath)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	gotSum := sha256.Sum256(got)
	if hex.EncodeToString(gotSum[:]) != wantSHA {
		t.Fatalf("on-disk sha mismatch")
	}

	src, err := rt.sources.ByID(ctx, storecontent.SourceIDAtomic)
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if src.Status != storecontent.SourceStatusIdle {
		t.Fatalf("source status = %s, want idle", src.Status)
	}
	if src.ItemCount != 2 {
		t.Fatalf("item_count = %d, want 2", src.ItemCount)
	}
	if src.LastSyncedAt.IsZero() {
		t.Fatal("last_synced_at is zero")
	}
}

func TestRunnerSecondStartConflicts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)

	rt.fixture.FetchBytes = FixtureBundle(storecontent.VersionCurrent, manyNotes(5))
	rt.fixture.DelayBatch = 200 * time.Millisecond

	job1, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("first StartSync: %v", err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		conflict int
	)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
				SourceID: storecontent.SourceIDSigma,
			})
			if err == nil {
				return
			}
			if errors.Is(err, apierr.ErrConflict) {
				mu.Lock()
				conflict++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if conflict < 1 {
		t.Fatalf("expected at least one 409, got conflict=%d", conflict)
	}

	_, err = rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDSigma,
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), job1.ID) {
		t.Fatalf("conflict detail %q does not name jobId %s", err.Error(), job1.ID)
	}

	if _, err := rt.runner.Wait(ctx, job1.ID); err != nil {
		t.Fatalf("Wait first job: %v", err)
	}
}

func TestRunnerCancelDuringApply(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 1)
	rt.fixture.FetchBytes = FixtureBundle(storecontent.VersionCurrent, manyNotes(20))
	rt.fixture.DelayBatch = 150 * time.Millisecond

	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, err := rt.runner.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		if j.Status == storecontent.JobStatusRunning && j.Phase == PhaseApply {
			break
		}
		if j.Status == storecontent.JobStatusSucceeded {
			t.Fatal("job finished before cancel")
		}
		time.Sleep(20 * time.Millisecond)
	}

	before := rt.fixture.BatchesApplied()
	if _, err := rt.runner.Cancel(ctx, testActor(), job.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusCancelled {
		t.Fatalf("status = %s error=%q, want cancelled", job.Status, job.Error)
	}
	after := rt.fixture.BatchesApplied()
	if after > before+2 {
		t.Fatalf("batches continued after cancel: before=%d after=%d", before, after)
	}
}

func TestRunnerFetchFailureKeepsCatalog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)

	rt.fixture.FetchBytes = FixtureBundle(storecontent.VersionCurrent, []FixtureNote{
		{ExternalID: "keep", Title: "Keep", Body: "me"},
	})
	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("seed StartSync: %v", err)
	}
	if job, err = rt.runner.Wait(ctx, job.ID); err != nil || job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("seed job: status=%s err=%v jobErr=%q", job.Status, err, job.Error)
	}

	rt.fixture.FetchBytes = nil
	rt.fixture.FetchErr = errors.New("upstream 503")
	job, err = rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("failing StartSync: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("status = %s, want failed", job.Status)
	}
	if !strings.Contains(job.Error, "upstream 503") {
		t.Fatalf("error = %q, want upstream 503", job.Error)
	}

	var n int
	if err := rt.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ? AND external_id = 'keep'`,
		storecontent.SourceIDAtomic,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("catalog note missing after failed fetch: count=%d", n)
	}
}

func TestRunnerBootInterruptsRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	jobs := storecontent.NewJobs(db)
	sources := storecontent.NewSources(db)

	job, err := jobs.Create(ctx, storecontent.NewJob{
		SourceID:  storecontent.SourceIDAtomic,
		Kind:      storecontent.JobKindSync,
		CreatedBy: "boot-test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := jobs.Update(ctx, job.ID, storecontent.JobUpdate{
		Status:    storecontent.JobStatusRunning,
		Phase:     PhaseApply,
		Message:   "mid-flight",
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := sources.SetSyncState(ctx, storecontent.SourceIDAtomic, storecontent.SourceStatusSyncing, 0, "", time.Time{}); err != nil {
		t.Fatalf("SetSyncState: %v", err)
	}

	runner, err := NewRunner(RunnerDeps{
		DB:       db,
		Sources:  sources,
		Versions: storecontent.NewVersions(db, paths),
		Jobs:     jobs,
		Paths:    paths,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if err := runner.Boot(ctx); err != nil {
		t.Fatalf("Boot: %v", err)
	}

	job, err = jobs.ByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if job.Status != storecontent.JobStatusInterrupted {
		t.Fatalf("status = %s, want interrupted", job.Status)
	}
	src, err := sources.ByID(ctx, storecontent.SourceIDAtomic)
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if src.Status != storecontent.SourceStatusIdle {
		t.Fatalf("source status = %s, want idle", src.Status)
	}
}

func TestRunnerOversizedFetchFailsNamingLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)
	rt.fixture.FetchErr = &ErrTooLarge{Limit: 1024, Got: 2048}

	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("status = %s, want failed", job.Status)
	}
	if !strings.Contains(job.Error, "1024") {
		t.Fatalf("error %q does not name the limit", job.Error)
	}
}

func TestRunnerApplyBatchesWhenOverBatchSize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 3)
	rt.fixture.FetchBytes = FixtureBundle(storecontent.VersionCurrent, manyNotes(10))

	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("status = %s err=%q", job.Status, job.Error)
	}
	// 1 delete batch + ceil(10/3)=4 insert batches = 5
	if got := rt.fixture.BatchesApplied(); got < 5 {
		t.Fatalf("batches = %d, want at least 5 (delete + 4 inserts)", got)
	}
}

func TestErrTooLargeNamesLimit(t *testing.T) {
	t.Parallel()
	err := &ErrTooLarge{Limit: 512 << 20}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d", int64(512<<20))) {
		t.Fatalf("error %q does not name limit", err.Error())
	}
}

type testRuntime struct {
	db       *store.DB
	paths    storecontent.Paths
	sources  *storecontent.Sources
	versions *storecontent.Versions
	fixture  *FixtureAdapter
	runner   *Runner
}

func newTestRunner(t *testing.T, kind storecontent.Kind, writeBatch int) *testRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	fixture := NewFixtureAdapter(kind)
	adapters := map[storecontent.Kind]Adapter{
		storecontent.KindAtomic: NewFixtureAdapter(storecontent.KindAtomic),
		storecontent.KindSigma:  NewFixtureAdapter(storecontent.KindSigma),
		storecontent.KindAttack: NewFixtureAdapter(storecontent.KindAttack),
		storecontent.KindCTID:   NewFixtureAdapter(storecontent.KindCTID),
	}
	adapters[kind] = fixture

	r, err := NewRunner(RunnerDeps{
		DB:         db,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   events.New(activity.New(db)),
		Adapters:   adapters,
		MaxBytes:   512 << 20,
		JobTimeout: time.Minute,
		WriteBatch: writeBatch,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if err := r.Boot(context.Background()); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	r.Start(context.Background())
	t.Cleanup(r.Stop)

	return &testRuntime{
		db:       db,
		paths:    paths,
		sources:  sources,
		versions: versions,
		fixture:  fixture,
		runner:   r,
	}
}

func testActor() authn.Subject {
	return authn.Subject{UserID: "test-admin", Email: "admin@example.com"}
}

func manyNotes(n int) []FixtureNote {
	out := make([]FixtureNote, n)
	for i := range n {
		out[i] = FixtureNote{
			ExternalID: fmt.Sprintf("n-%d", i),
			Title:      fmt.Sprintf("Note %d", i),
			Body:       "x",
		}
	}
	return out
}
