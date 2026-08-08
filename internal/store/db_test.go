package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// These tests run against a real DuckDB file (see storetest), so they exercise
// the concurrency behaviour they describe rather than a mock of it. Every wait
// on another goroutine is bounded, so a regression reports what blocked instead
// of hanging until the package timeout.

// settleTime is how long a test waits before asserting that an operation is
// still blocked. Those assertions cannot fail the other way — the operation
// under test cannot finish until the test releases it — so a slower machine
// makes the wait redundant, never flaky.
const settleTime = 50 * time.Millisecond

func TestOpenCreatesAUsableDatabase(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()

	if err := db.Health(ctx); err != nil {
		t.Fatalf("Health() = %v, want nil", err)
	}
	createEntries(t, db)
	if got := countEntries(t, db); got != 0 {
		t.Errorf("new database has %d rows, want 0", got)
	}
}

func TestOpenRejectsAMissingParentDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "blacklight.duckdb")
	_, err := store.Open(t.Context(), config.Database{Path: path})
	if err == nil {
		t.Fatal("Open() = nil error, want a failure")
	}
	// The operator has to be able to see which path, and what about it.
	assertErrorMentions(t, err, path, filepath.Dir(path), "does not exist")
}

func TestOpenRejectsAParentThatIsNotADirectory(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "blacklight.duckdb")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(file, "nested.duckdb")
	_, err := store.Open(t.Context(), config.Database{Path: path})
	if err == nil {
		t.Fatal("Open() = nil error, want a failure")
	}
	assertErrorMentions(t, err, path, "is not a directory")
}

func TestOpenRejectsAnUnwritableDirectory(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which can write to any directory")
	}

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "blacklight.duckdb")
	_, err := store.Open(t.Context(), config.Database{Path: path})
	if err == nil {
		t.Fatal("Open() = nil error, want a failure")
	}
	assertErrorMentions(t, err, path)
}

// A path with a "?" in it is a legal filename that the driver reads as the start
// of DSN options, so it has to be rejected here or it is reported as an
// unrecognised DuckDB setting named after the rest of the filename.
func TestOpenRejectsAPathContainingDSNOptions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "blacklight.duckdb?access_mode=read_only")
	_, err := store.Open(t.Context(), config.Database{Path: path})
	if err == nil {
		t.Fatal("Open() = nil error, want a failure")
	}
	assertErrorMentions(t, err, path, "DSN options")
}

func TestOpenRejectsAnEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := store.Open(t.Context(), config.Database{})
	if err == nil {
		t.Fatal("Open() = nil error, want a failure")
	}
	assertErrorMentions(t, err, "no database path")
}

// Reopening is what a restart does, so it is also the durability test: what a
// committed write survives is a process exit.
func TestReopenSeesCommittedData(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "blacklight.duckdb")
	ctx := t.Context()

	first, err := store.Open(ctx, config.Database{Path: path})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	createEntries(t, first)
	insertEntry(t, first, 1)
	if err := first.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	second, err := store.Open(ctx, config.Database{Path: path})
	if err != nil {
		t.Fatalf("reopen: Open() = %v", err)
	}
	defer closeOrFail(t, second)

	if got := countEntries(t, second); got != 1 {
		t.Errorf("after reopen: %d rows, want 1", got)
	}
}

func TestWriteCommits(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createEntries(t, db)

	insertEntry(t, db, 1)
	if got := countEntries(t, db); got != 1 {
		t.Errorf("after Write: %d rows, want 1", got)
	}
}

func TestWriteRollsBackOnError(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)

	sentinel := errors.New("business rule said no")
	err := db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (1)"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (2)"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Write() = %v, want the callback's error", err)
	}
	if got := countEntries(t, db); got != 0 {
		t.Errorf("after a rolled-back Write: %d rows, want 0 — partial state is visible", got)
	}

	// The connection is still usable: a rollback is not a fault.
	insertEntry(t, db, 3)
	if got := countEntries(t, db); got != 1 {
		t.Errorf("after the next Write: %d rows, want 1", got)
	}
}

// A panic must roll back and, above all, must not leave the write lock held: a
// deadlocked writer is worse than a crash, and an HTTP recovery middleware turns
// the crash into a single failed request.
func TestWriteRollsBackOnPanicAndReleasesTheLock(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)

	func() {
		defer func() {
			if recovered := recover(); recovered != "handler bug" {
				t.Errorf("recovered %v, want the callback's own panic value", recovered)
			}
		}()
		err := db.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (1)"); err != nil {
				return err
			}
			panic("handler bug")
		})
		// Unreachable unless Write swallowed the panic, which is itself the bug
		// this test is about.
		t.Errorf("Write() returned %v, want the panic to keep unwinding", err)
	}()

	if got := countEntries(t, db); got != 0 {
		t.Errorf("after a panicking Write: %d rows, want 0", got)
	}

	// The next writer must not be waiting for a lock the panic never gave back.
	done := make(chan error, 1)
	go func() { done <- db.Write(ctx, insert(ctx, 2)) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write() after a panic = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Write() after a panic blocked: the write lock was left held")
	}
	if got := countEntries(t, db); got != 1 {
		t.Errorf("after the write following a panic: %d rows, want 1", got)
	}
}

// The reason this package exists: unserialized, this loses about nine writes in
// ten to "TransactionContext Error: Conflict on update!".
func TestConcurrentWritersAllSucceed(t *testing.T) {
	t.Parallel()

	const writers = 100

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)

	var (
		inFlight    atomic.Int32
		maxInFlight atomic.Int32
		wg          sync.WaitGroup
		mu          sync.Mutex
		failures    []error
	)
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Write(ctx, func(tx *sql.Tx) error {
				// Serialization is the point, so assert it directly rather
				// than inferring it from the row count.
				n := inFlight.Add(1)
				defer inFlight.Add(-1)
				for {
					peak := maxInFlight.Load()
					if n <= peak || maxInFlight.CompareAndSwap(peak, n) {
						break
					}
				}
				_, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (?)", i)
				return err
			})
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, err := range failures {
		t.Errorf("Write() = %v, want nil for every writer", err)
	}
	if got := maxInFlight.Load(); got != 1 {
		t.Errorf("%d write callbacks ran at once, want 1 — writes are not serialized", got)
	}
	if got := countEntries(t, db); got != writers {
		t.Errorf("after %d concurrent writers: %d rows, want %d", writers, got, writers)
	}
}

// A caller who gave up while queued must not have its write applied — otherwise
// serialization turns every cancelled request into a delayed one.
func TestWriteCancelledWhileQueuedAppliesNothing(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)

	holding, release := make(chan struct{}), make(chan struct{})
	releaseWriter := releaseOnCleanup(t, release)
	held := make(chan error, 1)
	go func() {
		held <- db.Write(ctx, func(tx *sql.Tx) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	queuedCtx, cancel := context.WithCancel(ctx)
	queued := make(chan error, 1)
	go func() { queued <- db.Write(queuedCtx, insert(queuedCtx, 1)) }()

	// The second writer cannot return while the first holds the lock, so
	// finding it still blocked here is what proves it is queued rather than
	// rejected before it ever got that far.
	time.Sleep(settleTime)
	select {
	case err := <-queued:
		t.Fatalf("Write() = %v before its context was cancelled, want it queued", err)
	default:
	}

	cancel()
	select {
	case err := <-queued:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Write() = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("a cancelled Write() stayed queued")
	}

	releaseWriter()
	if err := <-held; err != nil {
		t.Fatalf("the holding Write() = %v, want nil", err)
	}
	if got := countEntries(t, db); got != 0 {
		t.Errorf("after a cancelled Write: %d rows, want 0 — the mutation was applied anyway", got)
	}
}

func TestWriteRejectsAnAlreadyCancelledContext(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	// Nothing holds the lock, so an implementation that only selected on
	// ctx.Done() alongside a free lock would take the lock about half the time.
	for range 20 {
		if err := db.Write(cancelled, insert(cancelled, 1)); !errors.Is(err, context.Canceled) {
			t.Fatalf("Write() = %v, want context.Canceled", err)
		}
	}
	if got := countEntries(t, db); got != 0 {
		t.Errorf("after cancelled writes: %d rows, want 0", got)
	}
}

func TestReadsAreNotBlockedByAnInFlightWrite(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()
	createEntries(t, db)
	insertEntry(t, db, 1)

	holding, release := make(chan struct{}), make(chan struct{})
	releaseWriter := releaseOnCleanup(t, release)
	held := make(chan error, 1)
	go func() {
		held <- db.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (2)"); err != nil {
				return err
			}
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	// Bounded: if readers were blocked by the writer this would otherwise wait
	// for a release that only happens after the read returns.
	readCtx, cancelRead := context.WithTimeout(ctx, 5*time.Second)
	var got int
	err := db.Read().QueryRowContext(readCtx, "SELECT count(*) FROM entries").Scan(&got)
	cancelRead()

	switch {
	case err != nil:
		t.Errorf("read during a write = %v, want it to complete", err)
	case got != 1:
		// Snapshot isolation: the reader sees the state it started with.
		t.Errorf("read during a write saw %d rows, want 1 (the uncommitted row leaked)", got)
	}

	releaseWriter()
	if err := <-held; err != nil {
		t.Fatalf("Write() = %v, want nil", err)
	}
	if got := countEntries(t, db); got != 2 {
		t.Errorf("after the write committed: %d rows, want 2", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	db, err := store.Open(t.Context(), config.Database{
		Path: filepath.Join(t.TempDir(), "blacklight.duckdb"),
	})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("first Close() = %v, want nil", err)
	}
	if err := db.Close(); err != nil {
		t.Errorf("second Close() = %v, want the same nil", err)
	}

	// Concurrent closers see the same result rather than a double-close error.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := db.Close(); err != nil {
				t.Errorf("concurrent Close() = %v, want nil", err)
			}
		}()
	}
	wg.Wait()
}

func TestCloseWaitsForAnInFlightWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "blacklight.duckdb")
	ctx := t.Context()

	db, err := store.Open(ctx, config.Database{Path: path})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	createEntries(t, db)

	holding, release := make(chan struct{}), make(chan struct{})
	releaseWriter := releaseOnCleanup(t, release)
	var committed atomic.Bool
	held := make(chan error, 1)
	go func() {
		held <- db.Write(ctx, func(tx *sql.Tx) error {
			close(holding)
			<-release
			if _, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (1)"); err != nil {
				return err
			}
			committed.Store(true)
			return nil
		})
	}()
	<-holding

	closed := make(chan error, 1)
	go func() { closed <- db.Close() }()

	// Close must still be waiting: it cannot have torn down the connection the
	// write is using.
	time.Sleep(settleTime)
	select {
	case err := <-closed:
		t.Fatalf("Close() = %v while a write was in flight, want it to wait", err)
	default:
	}

	releaseWriter()
	if err := <-held; err != nil {
		t.Fatalf("the in-flight Write() = %v, want nil", err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return after the in-flight write finished")
	}
	if !committed.Load() {
		t.Fatal("Close() returned before the write callback finished")
	}

	// The write it waited for is on disk, not merely finished.
	reopened, err := store.Open(ctx, config.Database{Path: path})
	if err != nil {
		t.Fatalf("reopen: Open() = %v", err)
	}
	defer closeOrFail(t, reopened)
	if got := countEntries(t, reopened); got != 1 {
		t.Errorf("after reopening: %d rows, want the write Close waited for", got)
	}
}

func TestWriteAndHealthAfterCloseReportErrClosed(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	db, err := store.Open(ctx, config.Database{
		Path: filepath.Join(t.TempDir(), "blacklight.duckdb"),
	})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if err := db.Write(ctx, func(*sql.Tx) error {
		t.Error("the write callback ran after Close()")
		return nil
	}); !errors.Is(err, store.ErrClosed) {
		t.Errorf("Write() after Close = %v, want store.ErrClosed", err)
	}
	if err := db.Health(ctx); !errors.Is(err, store.ErrClosed) {
		t.Errorf("Health() after Close = %v, want store.ErrClosed", err)
	}
}

// Writers queued behind the write Close is waiting for must be turned away
// rather than left waiting for a lock that is never released again.
func TestWriteQueuedAtCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	db, err := store.Open(ctx, config.Database{
		Path: filepath.Join(t.TempDir(), "blacklight.duckdb"),
	})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	createEntries(t, db)

	holding, release := make(chan struct{}), make(chan struct{})
	releaseWriter := releaseOnCleanup(t, release)
	held := make(chan error, 1)
	go func() {
		held <- db.Write(ctx, func(tx *sql.Tx) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	queued := make(chan error, 1)
	go func() { queued <- db.Write(ctx, insert(ctx, 1)) }()
	time.Sleep(settleTime)

	closed := make(chan error, 1)
	go func() { closed <- db.Close() }()

	select {
	case err := <-queued:
		if !errors.Is(err, store.ErrClosed) {
			t.Errorf("a queued Write() = %v during Close, want store.ErrClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("a queued Write() was not released by Close()")
	}

	releaseWriter()
	if err := <-held; err != nil {
		t.Errorf("the in-flight Write() = %v, want nil", err)
	}
	if err := <-closed; err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// Every connection is configured, not just the first one out of the pool: a
// reader whose session is in the server's local time zone returns timestamps
// that disagree with the writer's.
func TestEveryConnectionIsConfigured(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	ctx := t.Context()

	settings := map[string]string{
		"TimeZone":                     "UTC",
		"autoinstall_known_extensions": "false",
		"autoload_known_extensions":    "false",
	}

	for name, want := range settings {
		var fromReader string
		if err := db.Read().QueryRowContext(ctx,
			"SELECT current_setting(?)", name).Scan(&fromReader); err != nil {
			t.Fatalf("reading %s from the read pool: %v", name, err)
		}
		if fromReader != want {
			t.Errorf("read connection has %s=%q, want %q", name, fromReader, want)
		}

		var fromWriter string
		if err := db.Write(ctx, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, "SELECT current_setting(?)", name).Scan(&fromWriter)
		}); err != nil {
			t.Fatalf("reading %s from the write connection: %v", name, err)
		}
		if fromWriter != want {
			t.Errorf("write connection has %s=%q, want %q", name, fromWriter, want)
		}
	}
}

// Helpers.

func createEntries(t *testing.T, db *store.DB) {
	t.Helper()
	ctx := t.Context()
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "CREATE TABLE entries (id INTEGER)")
		return err
	}); err != nil {
		t.Fatalf("creating the test table: %v", err)
	}
}

// insert returns a write callback, for the tests that hand one to a goroutine.
func insert(ctx context.Context, id int) func(*sql.Tx) error {
	return func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO entries VALUES (?)", id)
		return err
	}
}

func insertEntry(t *testing.T, db *store.DB, id int) {
	t.Helper()
	if err := db.Write(t.Context(), insert(t.Context(), id)); err != nil {
		t.Fatalf("inserting %d: %v", id, err)
	}
}

func countEntries(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Read().QueryRowContext(t.Context(),
		"SELECT count(*) FROM entries").Scan(&n); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	return n
}

// releaseOnCleanup returns the function that lets a deliberately slow write
// finish, and also registers it with t.Cleanup: a t.Fatal while the write is
// still held would otherwise leave storetest's Close — which waits for
// in-flight writes, by design — waiting for a write nothing will ever release.
// Registered after storetest.New, so it runs before that Close.
func releaseOnCleanup(t *testing.T, release chan struct{}) func() {
	t.Helper()
	releaseWriter := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseWriter)
	return releaseWriter
}

func closeOrFail(t *testing.T, db *store.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// assertErrorMentions keeps the acceptance criterion honest: an error an
// operator cannot act on is not a pass.
func assertErrorMentions(t *testing.T, err error, substrings ...string) {
	t.Helper()
	for _, want := range substrings {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
