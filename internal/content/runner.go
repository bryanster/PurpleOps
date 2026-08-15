package content

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Default runner knobs when config left a zero (tests that skip config.Load).
// defaultWriteBatch matches config.Content.WriteBatch (250); M2-016 loadtest
// justified that size against interactive p95 ≤ 200ms.
const (
	defaultMaxBytes    int64 = 512 << 20
	defaultJobTimeout        = 30 * time.Minute
	defaultWriteBatch        = 250
	runnerPollInterval       = 2 * time.Second
)

// Runner is the global single-slot content job worker (M2-003).
//
// At most one job is queued or running in the installation. StartSync enforces
// the gate; the worker goroutine drains the queue. Progress is persisted on the
// job row and fanned out to in-process subscribers for M2-004 SSE wiring.
//
// Construct with [NewRunner], call [Runner.Boot] once at process start, then
// [Runner.Start] to launch the worker. [Runner.Stop] cancels an in-flight job
// context and waits for the worker to exit.
type Runner struct {
	sources  *storecontent.Sources
	versions *storecontent.Versions
	jobs     *storecontent.Jobs
	activity *events.Log
	db       storecontent.DB
	paths    storecontent.Paths
	adapters map[storecontent.Kind]Adapter
	// custom applies v1_import jobs (M2-012). Optional until that path is used.
	custom *Custom
	http   HTTPDoer
	policy URLPolicy
	log    *slog.Logger

	maxBytes   int64
	jobTimeout time.Duration
	writeBatch int

	wake chan struct{}

	mu         sync.Mutex
	runCancel  context.CancelFunc
	runJobID   string
	subs       map[chan ProgressEvent]struct{}
	workerStop context.CancelFunc
	workerDone chan struct{}
	booted     bool
}

// RunnerDeps is everything a [Runner] is built from.
type RunnerDeps struct {
	DB       storecontent.DB
	Sources  *storecontent.Sources
	Versions *storecontent.Versions
	Jobs     *storecontent.Jobs
	Paths    storecontent.Paths
	Activity *events.Log // optional
	// Custom applies v1_import jobs (M2-012). Optional for runners that never
	// enqueue that kind.
	Custom *Custom
	// Adapters maps kind → implementation. Unknown kind at sync start is an error.
	Adapters map[storecontent.Kind]Adapter
	// MaxBytes, JobTimeout, WriteBatch come from config.Content. Zeros get defaults.
	MaxBytes   int64
	JobTimeout time.Duration
	WriteBatch int
	// HTTP is injected into FetchRequest. Nil uses the runner's policy client
	// (M7-014). Tests inject a short-timeout client.
	HTTP HTTPDoer
	// Policy fences the URLs a fetch may touch (M7-014). The zero value is the
	// production posture: https only, no private destinations.
	Policy URLPolicy
	Log    *slog.Logger
}

// NewRunner returns a Runner over deps, or an error naming what is missing.
func NewRunner(deps RunnerDeps) (*Runner, error) {
	switch {
	case deps.DB == nil:
		return nil, errors.New("content: runner: no database")
	case deps.Sources == nil:
		return nil, errors.New("content: runner: no source repository")
	case deps.Versions == nil:
		return nil, errors.New("content: runner: no version repository")
	case deps.Jobs == nil:
		return nil, errors.New("content: runner: no job repository")
	}
	maxBytes := deps.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	timeout := deps.JobTimeout
	if timeout <= 0 {
		timeout = defaultJobTimeout
	}
	batch := deps.WriteBatch
	if batch <= 0 {
		batch = defaultWriteBatch
	}
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	adapters := deps.Adapters
	if adapters == nil {
		adapters = map[storecontent.Kind]Adapter{}
	}
	return &Runner{
		sources:    deps.Sources,
		versions:   deps.Versions,
		jobs:       deps.Jobs,
		activity:   deps.Activity,
		db:         deps.DB,
		paths:      deps.Paths,
		adapters:   adapters,
		custom:     deps.Custom,
		http:       deps.HTTP,
		policy:     deps.Policy,
		log:        log,
		maxBytes:   maxBytes,
		jobTimeout: timeout,
		writeBatch: batch,
		wake:       make(chan struct{}, 1),
		subs:       make(map[chan ProgressEvent]struct{}),
	}, nil
}

// RegisterAdapter installs or replaces the adapter for its kind. Intended for
// tests and for server wiring that builds the map incrementally.
func (r *Runner) RegisterAdapter(a Adapter) {
	if a == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.adapters == nil {
		r.adapters = map[storecontent.Kind]Adapter{}
	}
	r.adapters[a.Kind()] = a
}

// Boot marks in-flight jobs interrupted and unsticks sources left in syncing.
// Call once before Start, at process boot. Does not resume work.
//
// An unmigrated database (no content schema yet) is a no-op rather than an
// error: production always migrates before NewServer, and unit tests that open
// an empty file via storetest.New never touch content jobs.
//
//nolint:contextcheck // Boot is process-scoped; callers pass Background at startup by design.
func (r *Runner) Boot(ctx context.Context) error {
	const msg = "process restarted while job was in flight"
	nJobs, err := r.jobs.InterruptInFlight(ctx, msg)
	if err != nil {
		if isMissingContentSchema(err) {
			return nil
		}
		return err
	}
	nSrc, err := r.sources.ResetSyncing(ctx, msg)
	if err != nil {
		if isMissingContentSchema(err) {
			return nil
		}
		return err
	}
	r.mu.Lock()
	r.booted = true
	r.mu.Unlock()
	if nJobs > 0 || nSrc > 0 {
		r.log.InfoContext(ctx, "content runner boot reconciliation",
			"interrupted_jobs", nJobs,
			"reset_sources", nSrc)
	}
	return nil
}

// Start launches the single worker goroutine. Idempotent. The worker exits when
// Stop is called or the parent ctx is cancelled.
//
//nolint:contextcheck // Start is process-scoped; the worker outlives any request.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workerDone != nil {
		return
	}
	if !r.booted {
		r.mu.Unlock()
		if err := r.Boot(ctx); err != nil {
			r.log.ErrorContext(ctx, "content runner boot failed", "error", err)
		}
		r.mu.Lock()
		if r.workerDone != nil {
			return
		}
	}
	wctx, cancel := context.WithCancel(ctx)
	r.workerStop = cancel
	r.workerDone = make(chan struct{})
	go func() {
		defer close(r.workerDone)
		r.loop(wctx)
	}()
}

// Stop cancels the worker and any in-flight job context, then waits for exit.
func (r *Runner) Stop() {
	r.mu.Lock()
	stop := r.workerStop
	done := r.workerDone
	if r.runCancel != nil {
		r.runCancel()
	}
	r.mu.Unlock()
	if stop != nil {
		stop()
	}
	if done != nil {
		<-done
	}
	r.mu.Lock()
	r.workerStop = nil
	r.workerDone = nil
	r.mu.Unlock()
}

// ProgressEvent is one progress tick for subscribers (M2-004 SSE).
type ProgressEvent struct {
	JobID   string
	Phase   string
	Current int64
	Total   int64
	Message string
	Status  storecontent.JobStatus
}

// Subscribe registers for progress events. The channel is closed when
// unsubscribe is called. buffer is the channel capacity (0 → 16).
func (r *Runner) Subscribe(buffer int) (<-chan ProgressEvent, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan ProgressEvent, buffer)
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	var once sync.Once
	unsub := func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.subs, ch)
			r.mu.Unlock()
			close(ch)
		})
	}
	return ch, unsub
}

func (r *Runner) publish(ev ProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ch := range r.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// ListReleases asks a source's upstream what it offers, newest first.
//
// It is a read and only a read: no job row, no global slot, nothing written. A
// caller is about to choose a version — the first-run wizard's picker is the
// reason this exists — and the choosing is what starts a sync.
//
// A source whose adapter does not implement [ReleaseLister] has no release list
// to give: that is a rolling upstream, and the answer is an empty list rather
// than an error, because "there are no releases to choose between" is true and
// is what the caller needs to know.
//
// Network failure is reported as an error and not swallowed. The wizard has an
// offline path to fall back to, and it can only offer it if it is told.
func (r *Runner) ListReleases(ctx context.Context, sourceID string) ([]Release, error) {
	if sourceID == "" {
		return nil, apierr.Validation(apierr.Field("sourceId", "is required"))
	}
	src, err := r.sources.ByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	adapter := r.adapters[src.Kind]
	r.mu.Unlock()

	lister, ok := adapter.(ReleaseLister)
	if !ok {
		return []Release{}, nil
	}
	return lister.ListReleases(ctx, FetchRequest{
		Source: SourceInfo{
			ID:   src.ID,
			Kind: src.Kind,
			Name: src.Name,
			URL:  src.URL,
			Ref:  src.Ref,
		},
		MaxBytes: r.maxBytes,
		HTTP:     r.httpClient(),
		Policy:   r.policy,
	})
}

// StartSyncRequest is the caller half of enqueueing a sync job.
type StartSyncRequest struct {
	SourceID string
	// Version is optional. Empty means latest discoverable per adapter.
	// Rolling sources ignore this and always write version "current".
	Version string
	// Kind defaults to sync. M2-005/M2-012 pass reprocess / bundle_import / v1_import.
	Kind storecontent.JobKind
	// BundlePath, when set, skips Fetch and reads this absolute path as the
	// bundle (reprocess / offline upload). Must sit under the content data root
	// when provided by untrusted input — callers validate via
	// [Runner.StartBundleImport] / [Runner.StartReprocess].
	BundlePath string
	// BundleSHA256 optional precomputed digest of BundlePath.
	BundleSHA256 string
	// CleanupUpload, when true, removes BundlePath after the job reaches any
	// terminal status. Used for spooled offline uploads under uploads/; never
	// set for reprocess (that path is the durable raw snapshot).
	CleanupUpload bool
}

// StartSync enqueues a job or returns 409 if the global slot is taken.
func (r *Runner) StartSync(ctx context.Context, actor authn.Subject, req StartSyncRequest) (storecontent.Job, error) {
	if req.SourceID == "" {
		return storecontent.Job{}, apierr.Validation(apierr.Field("sourceId", "is required"))
	}
	src, err := r.sources.ByID(ctx, req.SourceID)
	if err != nil {
		return storecontent.Job{}, err
	}
	if src.Kind == storecontent.KindCustom && req.BundlePath == "" {
		return storecontent.Job{}, apierr.Conflict(
			"the custom source is not synced from upstream; author content through the custom API")
	}

	r.mu.Lock()
	adapter := r.adapters[src.Kind]
	r.mu.Unlock()
	if adapter == nil {
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("no adapter registered for content kind %q", src.Kind))
	}

	kind := req.Kind
	if kind == "" {
		kind = storecontent.JobKindSync
	}

	if active, ok, err := r.jobs.FindActive(ctx); err != nil {
		return storecontent.Job{}, err
	} else if ok {
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("a content job is already active (jobId: %s)", active.ID))
	}

	checkpoint := map[string]any{}
	if req.BundlePath != "" {
		if err := r.requirePathUnderRoot(req.BundlePath); err != nil {
			return storecontent.Job{}, err
		}
		checkpoint["bundle_path"] = req.BundlePath
		if req.BundleSHA256 != "" {
			checkpoint["bundle_sha256"] = req.BundleSHA256
		}
		checkpoint["skip_fetch"] = true
		if req.CleanupUpload {
			checkpoint["cleanup_upload"] = true
		}
	}
	rawCP, err := json.Marshal(checkpoint)
	if err != nil {
		return storecontent.Job{}, fmt.Errorf("content: encode job checkpoint: %w", err)
	}

	job, err := r.jobs.Create(ctx, storecontent.NewJob{
		SourceID:  src.ID,
		Version:   req.Version,
		Kind:      kind,
		CreatedBy: actor.UserID,
	})
	if err != nil {
		return storecontent.Job{}, err
	}
	if len(checkpoint) > 0 {
		job, err = r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
			Status:     job.Status,
			Phase:      job.Phase,
			Message:    job.Message,
			Checkpoint: rawCP,
		})
		if err != nil {
			return storecontent.Job{}, err
		}
	}

	if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusSyncing, src.ItemCount, "", time.Time{}); err != nil {
		return storecontent.Job{}, err
	}

	r.recordAlone(ctx, events.Entry{
		ActorID:    actor.UserID,
		Verb:       events.VerbContentSyncStarted,
		ObjectType: events.ObjectContentSyncJob,
		ObjectID:   job.ID,
		Delta: events.Delta(map[string]any{
			"source_id": src.ID,
			"kind":      string(kind),
			"version":   req.Version,
		}),
	})

	r.nudge()
	return job, nil
}

// Cancel requests cancellation of a job. Queued jobs flip to cancelled
// immediately; running jobs enter cancelling and the adapter ctx is cancelled.
func (r *Runner) Cancel(ctx context.Context, actor authn.Subject, jobID string) (storecontent.Job, error) {
	job, err := r.jobs.ByID(ctx, jobID)
	if err != nil {
		return storecontent.Job{}, err
	}
	switch job.Status {
	case storecontent.JobStatusSucceeded, storecontent.JobStatusFailed,
		storecontent.JobStatusCancelled, storecontent.JobStatusInterrupted:
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("job %s is already %s", job.ID, job.Status))
	case storecontent.JobStatusCancelling:
		return job, nil
	case storecontent.JobStatusQueued:
		now := time.Now().UTC()
		job, err = r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
			Status:     storecontent.JobStatusCancelled,
			Phase:      job.Phase,
			Message:    "cancelled before start",
			FinishedAt: now,
		})
		if err != nil {
			return storecontent.Job{}, err
		}
		src, srcErr := r.sources.ByID(ctx, job.SourceID)
		if srcErr == nil {
			if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusIdle, src.ItemCount, "", time.Time{}); err != nil {
				r.log.ErrorContext(ctx, "content: clear source after cancel", "error", err, "source_id", src.ID)
			}
		}
		r.recordAlone(ctx, events.Entry{
			ActorID:    actor.UserID,
			Verb:       events.VerbContentSyncCancelled,
			ObjectType: events.ObjectContentSyncJob,
			ObjectID:   job.ID,
			Delta:      events.Delta(map[string]any{"source_id": job.SourceID}),
		})
		r.publish(ProgressEvent{
			JobID:   job.ID,
			Phase:   job.Phase,
			Message: "cancelled before start",
			Status:  storecontent.JobStatusCancelled,
		})
		r.cleanupJobUpload(job)
		return job, nil
	case storecontent.JobStatusRunning:
		job, err = r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
			Status:          storecontent.JobStatusCancelling,
			Phase:           job.Phase,
			ProgressCurrent: job.ProgressCurrent,
			ProgressTotal:   job.ProgressTotal,
			Message:         "cancellation requested",
		})
		if err != nil {
			return storecontent.Job{}, err
		}
		r.mu.Lock()
		if r.runJobID == job.ID && r.runCancel != nil {
			r.runCancel()
		}
		r.mu.Unlock()
		return job, nil
	default:
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("job %s cannot be cancelled from status %s", job.ID, job.Status))
	}
}

// GetJob returns one job by id.
func (r *Runner) GetJob(ctx context.Context, id string) (storecontent.Job, error) {
	return r.jobs.ByID(ctx, id)
}

// ListJobs returns jobs newest first.
func (r *Runner) ListJobs(ctx context.Context, f storecontent.ListFilter) ([]storecontent.Job, error) {
	return r.jobs.List(ctx, f)
}

// Wait blocks until jobID reaches a terminal status or ctx is done.
func (r *Runner) Wait(ctx context.Context, jobID string) (storecontent.Job, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := r.jobs.ByID(ctx, jobID)
		if err != nil {
			return storecontent.Job{}, err
		}
		switch job.Status {
		case storecontent.JobStatusSucceeded, storecontent.JobStatusFailed,
			storecontent.JobStatusCancelled, storecontent.JobStatusInterrupted:
			return job, nil
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *Runner) nudge() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runner) loop(ctx context.Context) {
	ticker := time.NewTicker(runnerPollInterval)
	defer ticker.Stop()
	for {
		if err := r.tick(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, store.ErrClosed) || isDBGone(err) {
				return
			}
			if isMissingContentSchema(err) {
				return
			}
			r.log.ErrorContext(ctx, "content runner tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-ticker.C:
		}
	}
}

func (r *Runner) tick(ctx context.Context) error {
	job, ok, err := r.jobs.NextQueued(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return r.execute(ctx, job)
}

type pipelineResult struct {
	version string
	count   int64
	message string // optional success message override from the adapter catalog
}

func (r *Runner) execute(parent context.Context, job storecontent.Job) error {
	// Drop spooled uploads once the job can no longer need them — success,
	// failure, or cancel. Reprocess paths (cleanup_upload=false) are left alone.
	defer r.cleanupJobUpload(job)

	if job.Kind == storecontent.JobKindV1Import {
		return r.executeV1Import(parent, job)
	}

	src, err := r.sources.ByID(parent, job.SourceID)
	if err != nil {
		return r.failJob(parent, job, "", fmt.Errorf("load source: %w", err))
	}
	r.mu.Lock()
	adapter := r.adapters[src.Kind]
	r.mu.Unlock()
	if adapter == nil {
		return r.failJob(parent, job, "", fmt.Errorf("no adapter for kind %q", src.Kind))
	}

	started := time.Now().UTC()
	job, err = r.jobs.Update(parent, job.ID, storecontent.JobUpdate{
		Status:    storecontent.JobStatusRunning,
		Phase:     PhaseFetch,
		Message:   "starting",
		StartedAt: started,
	})
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(parent, r.jobTimeout)
	defer cancel()
	r.mu.Lock()
	r.runCancel = cancel
	r.runJobID = job.ID
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.runCancel = nil
		r.runJobID = ""
		r.mu.Unlock()
	}()

	prog := &jobProgress{runner: r, jobID: job.ID, status: storecontent.JobStatusRunning}
	result, runErr := r.runPipeline(runCtx, adapter, src, job, prog)

	current, err := r.jobs.ByID(parent, job.ID)
	if err != nil {
		return err
	}
	if result.version != "" {
		current.Version = result.version
	}

	finished := time.Now().UTC()
	switch {
	case runErr == nil:
		return r.succeedJob(parent, current, src, result, finished)
	case errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return r.failJob(parent, current, result.version, fmt.Errorf("job exceeded timeout of %s", r.jobTimeout))
	case errors.Is(runErr, context.Canceled) || current.Status == storecontent.JobStatusCancelling:
		return r.cancelJob(parent, current, src, finished)
	default:
		return r.failJob(parent, current, result.version, runErr)
	}
}

func (r *Runner) runPipeline(ctx context.Context, adapter Adapter, src storecontent.Source, job storecontent.Job, prog *jobProgress) (pipelineResult, error) {
	var (
		bundle Bundle
		err    error
		out    pipelineResult
	)

	skipFetch, bundlePath, bundleSHA, _ := parseCheckpoint(job.Checkpoint)
	if skipFetch && bundlePath != "" {
		prog.Report(ctx, PhaseFetch, 0, 1, "using pre-seated bundle")
		raw, err := ReadAll(ctx, FileSource{Path: bundlePath})
		if err != nil {
			return out, err
		}
		bundle = Bundle{
			Bytes:   raw,
			Path:    bundlePath,
			SHA256:  bundleSHA,
			Size:    int64(len(raw)),
			Version: job.Version,
		}
		if bundle.Version == "" {
			bundle.Version = storecontent.VersionCurrent
		}
		prog.Report(ctx, PhaseFetch, 1, 1, "bundle loaded")
	} else {
		prog.Report(ctx, PhaseFetch, 0, 1, "fetching")
		bundle, err = adapter.Fetch(ctx, FetchRequest{
			Source: SourceInfo{
				ID:   src.ID,
				Kind: src.Kind,
				Name: src.Name,
				URL:  src.URL,
				Ref:  src.Ref,
			},
			Version:  job.Version,
			MaxBytes: r.maxBytes,
			HTTP:     r.httpClient(),
			Policy:   r.policy,
		})
		if err != nil {
			return out, err
		}
		prog.Report(ctx, PhaseFetch, 1, 1, "fetch complete")
	}

	if src.Kind != storecontent.KindAttack {
		bundle.Version = storecontent.VersionCurrent
	}
	if bundle.Version == "" {
		return out, errors.New("adapter returned bundle with empty version")
	}
	out.version = bundle.Version

	if len(bundle.Bytes) == 0 && bundle.Path != "" {
		raw, err := ReadAll(ctx, FileSource{Path: bundle.Path})
		if err != nil {
			return out, err
		}
		bundle.Bytes = raw
		bundle.Size = int64(len(raw))
	}
	if len(bundle.Bytes) == 0 {
		return out, errors.New("bundle has no bytes")
	}
	if bundle.SHA256 == "" {
		sum := sha256.Sum256(bundle.Bytes)
		bundle.SHA256 = hex.EncodeToString(sum[:])
	}
	bundle.Size = int64(len(bundle.Bytes))

	prog.Report(ctx, PhaseParse, 0, 1, "parsing")
	ast, err := adapter.Parse(ctx, bundle)
	if err != nil {
		return out, err
	}
	prog.Report(ctx, PhaseParse, 1, 1, "parse complete")

	prog.Report(ctx, PhaseNormalize, 0, 1, "normalizing")
	objects, err := adapter.Normalize(ctx, ast)
	if err != nil {
		return out, err
	}
	prog.Report(ctx, PhaseNormalize, 1, 1, fmt.Sprintf("%d objects", len(objects)))
	out.count = countObjects(objects)
	if msg := successMessage(objects); msg != "" {
		out.message = msg
	}

	if err := r.ensureVersion(ctx, src.ID, bundle.Version); err != nil {
		return out, err
	}

	w := &jobWriter{
		db:        r.db,
		sourceID:  src.ID,
		version:   bundle.Version,
		batchSize: r.writeBatch,
	}
	prog.Report(ctx, PhaseApply, 0, out.count, "applying")
	if err := adapter.Apply(ctx, w, objects, prog); err != nil {
		return out, err
	}

	prog.Report(ctx, PhaseFinalize, 0, 1, "persisting raw snapshot")
	if err := r.finalizeSuccess(ctx, src, bundle, out.count); err != nil {
		return out, err
	}
	prog.Report(ctx, PhaseFinalize, 1, 1, "done")
	return out, nil
}

func (r *Runner) ensureVersion(ctx context.Context, sourceID, version string) error {
	_, err := r.versions.BySourceVersion(ctx, sourceID, version)
	if err == nil {
		return nil
	}
	if !errors.Is(err, apierr.ErrNotFound) {
		return err
	}
	_, err = r.versions.Create(ctx, storecontent.NewSourceVersion{
		SourceID: sourceID,
		Version:  version,
		Status:   storecontent.VersionStatusPending,
	})
	if err != nil && !errors.Is(err, apierr.ErrConflict) {
		return err
	}
	return nil
}

func (r *Runner) finalizeSuccess(ctx context.Context, src storecontent.Source, bundle Bundle, itemCount int64) error {
	rel, err := r.paths.RawRel(src.ID, bundle.Version, bundle.SHA256)
	if err != nil {
		return err
	}
	abs, err := r.paths.Abs(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return fmt.Errorf("content: mkdir raw snapshot: %w", err)
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, bundle.Bytes, 0o600); err != nil {
		return fmt.Errorf("content: write raw snapshot: %w", err)
	}
	if err := os.Rename(tmp, abs); err != nil {
		if rmErr := os.Remove(tmp); rmErr != nil && !os.IsNotExist(rmErr) {
			err = errors.Join(err, rmErr)
		}
		return fmt.Errorf("content: rename raw snapshot: %w", err)
	}

	ver, err := r.versions.BySourceVersion(ctx, src.ID, bundle.Version)
	if err != nil {
		return err
	}
	if ver.RawPath != "" && ver.RawPath != rel {
		if oldAbs, err := r.paths.Abs(ver.RawPath); err == nil {
			if rmErr := os.Remove(oldAbs); rmErr != nil && !os.IsNotExist(rmErr) {
				r.log.WarnContext(ctx, "content: remove previous raw snapshot", "error", rmErr, "path", oldAbs)
			}
		}
	}
	if _, err := r.versions.SetRaw(ctx, ver.ID, rel, bundle.SHA256, bundle.Size); err != nil {
		return err
	}
	now := time.Now().UTC()
	return r.versions.SetState(ctx, ver.ID, storecontent.VersionStatusReady, itemCount, "", now)
}

func (r *Runner) succeedJob(ctx context.Context, job storecontent.Job, src storecontent.Source, result pipelineResult, finished time.Time) error {
	r.cleanupJobUpload(job)

	itemCount := result.count
	version := result.version
	if version == "" {
		version = job.Version
	}
	if version == "" {
		version = storecontent.VersionCurrent
	}
	if ver, err := r.versions.BySourceVersion(ctx, src.ID, version); err == nil && ver.ItemCount > 0 {
		itemCount = ver.ItemCount
	}

	msg := "succeeded"
	if result.message != "" {
		msg = result.message
	}
	if _, err := r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
		Status:          storecontent.JobStatusSucceeded,
		Phase:           PhaseFinalize,
		ProgressCurrent: 1,
		ProgressTotal:   1,
		Message:         msg,
		FinishedAt:      finished,
	}); err != nil {
		return err
	}
	if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusIdle, itemCount, "", finished); err != nil {
		return err
	}
	r.recordAlone(ctx, events.Entry{
		ActorID:    job.CreatedBy,
		Verb:       events.VerbContentSyncFinished,
		ObjectType: events.ObjectContentSyncJob,
		ObjectID:   job.ID,
		Delta: events.Delta(map[string]any{
			"source_id":  src.ID,
			"version":    version,
			"item_count": itemCount,
		}),
	})
	r.publish(ProgressEvent{
		JobID: job.ID, Phase: PhaseFinalize, Current: 1, Total: 1,
		Message: msg, Status: storecontent.JobStatusSucceeded,
	})
	return nil
}

func (r *Runner) failJob(ctx context.Context, job storecontent.Job, version string, cause error) error {
	r.cleanupJobUpload(job)

	msg := cause.Error()
	finished := time.Now().UTC()
	if _, err := r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
		Status:     storecontent.JobStatusFailed,
		Phase:      job.Phase,
		Message:    "failed",
		Error:      msg,
		FinishedAt: finished,
	}); err != nil {
		return errors.Join(cause, err)
	}
	src, srcErr := r.sources.ByID(ctx, job.SourceID)
	if srcErr == nil {
		if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusError, src.ItemCount, msg, time.Time{}); err != nil {
			r.log.ErrorContext(ctx, "content: mark source error", "error", err, "source_id", src.ID)
		}
	}
	verToken := version
	if verToken == "" {
		verToken = job.Version
	}
	if verToken != "" {
		if ver, err := r.versions.BySourceVersion(ctx, job.SourceID, verToken); err == nil {
			if err := r.versions.SetState(ctx, ver.ID, storecontent.VersionStatusError, ver.ItemCount, msg, time.Time{}); err != nil {
				r.log.ErrorContext(ctx, "content: mark version error", "error", err, "version_id", ver.ID)
			}
		}
	}
	r.recordAlone(ctx, events.Entry{
		ActorID:    job.CreatedBy,
		Verb:       events.VerbContentSyncFailed,
		ObjectType: events.ObjectContentSyncJob,
		ObjectID:   job.ID,
		Delta: events.Delta(map[string]any{
			"source_id": job.SourceID,
			"error":     msg,
		}),
	})
	r.publish(ProgressEvent{
		JobID: job.ID, Phase: job.Phase, Message: msg, Status: storecontent.JobStatusFailed,
	})
	return nil
}

func (r *Runner) cancelJob(ctx context.Context, job storecontent.Job, src storecontent.Source, finished time.Time) error {
	r.cleanupJobUpload(job)

	if _, err := r.jobs.Update(ctx, job.ID, storecontent.JobUpdate{
		Status:          storecontent.JobStatusCancelled,
		Phase:           job.Phase,
		ProgressCurrent: job.ProgressCurrent,
		ProgressTotal:   job.ProgressTotal,
		Message:         "cancelled",
		FinishedAt:      finished,
	}); err != nil {
		return err
	}
	if err := r.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusIdle, src.ItemCount, "", time.Time{}); err != nil {
		r.log.ErrorContext(ctx, "content: clear source after cancel", "error", err, "source_id", src.ID)
	}
	r.recordAlone(ctx, events.Entry{
		ActorID:    job.CreatedBy,
		Verb:       events.VerbContentSyncCancelled,
		ObjectType: events.ObjectContentSyncJob,
		ObjectID:   job.ID,
		Delta:      events.Delta(map[string]any{"source_id": src.ID}),
	})
	r.publish(ProgressEvent{
		JobID: job.ID, Phase: job.Phase, Message: "cancelled", Status: storecontent.JobStatusCancelled,
	})
	return nil
}

func (r *Runner) httpClient() HTTPDoer {
	if r.http != nil {
		return r.http
	}
	return r.policy.NewClient()
}

func (r *Runner) recordAlone(ctx context.Context, e events.Entry) {
	if r.activity == nil {
		return
	}
	if err := r.activity.RecordAlone(ctx, e); err != nil {
		r.log.ErrorContext(ctx, "content activity write failed", "error", err, "verb", e.Verb)
	}
}

// jobWriter implements [Writer].
type jobWriter struct {
	db        storecontent.DB
	sourceID  string
	version   string
	batchSize int
}

func (w *jobWriter) Write(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return w.db.Write(ctx, func(tx *sql.Tx) error {
		return fn(ctx, tx)
	})
}
func (w *jobWriter) SourceID() string { return w.sourceID }
func (w *jobWriter) Version() string  { return w.version }
func (w *jobWriter) BatchSize() int   { return w.batchSize }

// jobProgress implements [Progress] and persists to the job row.
type jobProgress struct {
	runner *Runner
	jobID  string
	status storecontent.JobStatus
}

func (p *jobProgress) Report(ctx context.Context, phase string, current, total int64, message string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Never clobber a cancellation the operator just requested: Cancel flips
	// the row to cancelling, and a progress tick that rewrote status=running
	// would leave the job stuck until the adapter happened to notice ctx.
	status := p.status
	if cur, err := p.runner.jobs.ByID(ctx, p.jobID); err == nil {
		switch cur.Status {
		case storecontent.JobStatusCancelling, storecontent.JobStatusCancelled,
			storecontent.JobStatusSucceeded, storecontent.JobStatusFailed,
			storecontent.JobStatusInterrupted:
			status = cur.Status
		}
	}

	if _, err := p.runner.jobs.Update(ctx, p.jobID, storecontent.JobUpdate{
		Status:          status,
		Phase:           phase,
		ProgressCurrent: current,
		ProgressTotal:   total,
		Message:         message,
	}); err != nil {
		p.runner.log.ErrorContext(ctx, "content: persist job progress", "error", err, "job_id", p.jobID)
	}
	p.runner.publish(ProgressEvent{
		JobID:   p.jobID,
		Phase:   phase,
		Current: current,
		Total:   total,
		Message: message,
		Status:  status,
	})
}

func parseCheckpoint(raw json.RawMessage) (skip bool, path, sha string, cleanup bool) {
	if len(raw) == 0 {
		return false, "", "", false
	}
	var cp struct {
		SkipFetch     bool   `json:"skip_fetch"`
		BundlePath    string `json:"bundle_path"`
		BundleSHA256  string `json:"bundle_sha256"`
		CleanupUpload bool   `json:"cleanup_upload"`
	}
	if err := json.Unmarshal(raw, &cp); err != nil {
		return false, "", "", false
	}
	return cp.SkipFetch, cp.BundlePath, cp.BundleSHA256, cp.CleanupUpload
}

func (r *Runner) cleanupJobUpload(job storecontent.Job) {
	_, path, _, cleanup := parseCheckpoint(job.Checkpoint)
	if !cleanup || path == "" {
		return
	}
	r.removeUpload(path)
}

func isMissingContentSchema(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") &&
		(strings.Contains(msg, "content") || strings.Contains(msg, "schema"))
}

func isDBGone(err error) bool {
	if err == nil {
		return false
	}
	// database/sql after Close; DuckDB may also surface driver-closed forms.
	msg := err.Error()
	return strings.Contains(msg, "database is closed") ||
		strings.Contains(msg, "sql: database is closed") ||
		strings.Contains(msg, "conn closed")
}

// objectCounter is implemented by adapter objects that wrap many library rows
// in one envelope (ATT&CK catalog). When present, the runner uses it for
// version/source item_count instead of len(objects).
type objectCounter interface {
	ItemCount() int64
}

func countObjects(objects []Object) int64 {
	if len(objects) == 1 {
		if c, ok := objects[0].(objectCounter); ok {
			return c.ItemCount()
		}
	}
	return int64(len(objects))
}

// successMessenger is implemented by adapter catalogs that want a richer
// terminal job message (for example Sigma skip counts).
type successMessenger interface {
	SuccessMessage() string
}

func successMessage(objects []Object) string {
	if len(objects) != 1 {
		return ""
	}
	if m, ok := objects[0].(successMessenger); ok {
		return m.SuccessMessage()
	}
	return ""
}
