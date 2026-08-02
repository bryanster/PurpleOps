package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/duckdb/duckdb-go/v2"

	"github.com/bryanster/blacklight/internal/config"
)

// readerConns bounds the read pool. DuckDB parallelises a single query across
// its own thread pool, so a connection buys concurrent *statements*, not
// throughput: past a handful they mostly add memory (each connection carries its
// own buffers) and contend for the same threads. Eight is sized for this
// deployment — one node, dozens of users, a bursty but low request rate — and,
// being a limit rather than a target, it also stops a traffic spike from opening
// one DuckDB connection per in-flight HTTP request.
const readerConns = 8

// connSettings are applied to every connection as it is opened, including the
// writer's. They are session settings, so there is no init-once alternative.
var connSettings = []string{
	// Every timestamp in this application is stored and served in UTC
	// (docs/tickets/README.md, "Time"). Without this, now() and a cast from a
	// string would follow the server's local zone and the database would
	// disagree with itself after a deployment moved.
	"SET TimeZone='UTC'",

	// A single-binary deployment must never reach the network mid-query. Both
	// default to true, so a query mentioning an httpfs- or spatial-backed
	// function would otherwise try to download an extension while serving a
	// request — slow when it works, and a confusing failure when the host is
	// air-gapped. Everything this application uses (icu, json, parquet) is
	// linked into the driver and already loaded.
	"SET autoinstall_known_extensions=false",
	"SET autoload_known_extensions=false",
}

// DB is a DuckDB database: a pool for readers, and one connection every write is
// funnelled through. Construct it with [Open]; there is deliberately no
// package-level instance (PLAN.md §6).
type DB struct {
	// path is kept for error messages: "the database" is not actionable when an
	// operator has three deployments on one host.
	path string

	// read is the pooled handle. It also owns the connector, so closing it
	// closes the underlying DuckDB instance.
	read *sql.DB

	// writeConn is checked out of read's pool for the lifetime of the process
	// and is only ever used by the goroutine holding writeLock.
	writeConn *sql.Conn

	// writeLock is a one-slot semaphore: send to acquire, receive to release.
	// It is a channel rather than a sync.Mutex so that a caller can give up
	// while queued — see the package comment.
	writeLock chan struct{}

	// closing is closed by Close to turn away writers before they queue behind
	// a lock that will never be released again.
	closing chan struct{}

	closeOnce sync.Once
	closeErr  error
}

// Open opens (creating it if necessary) the database file named by cfg, applies
// the connection settings and verifies the database answers. The caller owns the
// returned handle and must Close it.
func Open(ctx context.Context, cfg config.Database) (*DB, error) {
	if err := checkPath(cfg.Path); err != nil {
		return nil, openError(cfg.Path, err)
	}

	connector, err := duckdb.NewConnector(cfg.Path, initConn)
	if err != nil {
		return nil, openError(cfg.Path, err)
	}

	// From here on the connector is owned by the *sql.DB, which closes it —
	// so every failure path below closes sqlDB rather than the connector, and
	// reports a failure to do so rather than leaking the DuckDB instance.
	sqlDB := sql.OpenDB(connector)
	sqlDB.SetMaxOpenConns(readerConns + 1) // +1: the writer's reserved connection.
	sqlDB.SetMaxIdleConns(readerConns + 1) // Keep them; connSettings runs per new connection.
	sqlDB.SetConnMaxLifetime(0)            // Nothing in-process expires a connection.

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, errors.Join(openError(cfg.Path, err), sqlDB.Close())
	}

	// Reserving the write connection at startup rather than per write means a
	// writer never waits for the pool, and the single-writer rule cannot be
	// broken by a pool that decided to hand out a second connection.
	writeConn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, errors.Join(openError(cfg.Path, err), sqlDB.Close())
	}

	return &DB{
		path:      cfg.Path,
		read:      sqlDB,
		writeConn: writeConn,
		writeLock: make(chan struct{}, 1),
		closing:   make(chan struct{}),
	}, nil
}

// Read returns the pooled read handle, safe for any number of goroutines. It
// cannot write: see [Reader] and the package comment.
func (db *DB) Read() Reader { return db.read }

// Write runs fn inside a transaction on the single write connection, one caller
// at a time. Returning an error from fn rolls back and returns that error;
// returning nil commits.
//
// While queued, Write honours ctx: a caller whose request was cancelled returns
// ctx.Err() and its mutation is never applied. After [DB.Close] it returns
// [ErrClosed].
//
// A panic inside fn rolls the transaction back, releases the lock and continues
// panicking, so that a recovered handler leaves a usable database behind.
//
// The tx belongs to Write and is finished by the time it returns; do not retain
// it. Do not call Write from inside fn — it is not re-entrant.
func (db *DB) Write(ctx context.Context, fn func(tx *sql.Tx) error) error {
	release, err := db.acquireWrite(ctx)
	if err != nil {
		return err
	}
	defer release()

	tx, err := db.writeConn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin write transaction: %w", err)
	}

	// done marks the exit paths that have already finished with tx. The one
	// that has not is fn panicking — or calling runtime.Goexit, which is what
	// t.Fatal does. Without this, the shared write connection would be handed
	// to the next writer with a transaction still open on it, and every later
	// write would fail.
	//
	// The panic itself is not recovered: recovering to re-panic would replace
	// the handler's panic value with this package's, and would turn a Goexit
	// into a panic on nil.
	done := false
	defer func() {
		if done {
			return
		}
		if err := rollback(tx); err != nil {
			// There is nowhere to return this to, and it is not worth a second
			// panic on top of the first. It is also not a silent failure:
			// database/sql discards a connection whose rollback failed, so the
			// next write reports the broken connection for itself.
			slog.Error("rollback after a panicking write failed",
				"error", err, "database", db.path)
		}
	}()

	if err := fn(tx); err != nil {
		done = true
		if rbErr := rollback(tx); rbErr != nil {
			return errors.Join(err, fmt.Errorf("store: rollback: %w", rbErr))
		}
		return err
	}

	done = true
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit write transaction: %w", err)
	}
	return nil
}

// Health reports whether the database is answering, for /healthz.
//
// It exercises the read pool only. A check that took the write lock would queue
// behind a long write and report a busy server as a dead one — and an
// orchestrator restarting the process in the middle of a write is the outage
// this endpoint is supposed to prevent, not cause. Readers and the writer share
// one DuckDB instance, so an instance that has stopped answering fails here too.
func (db *DB) Health(ctx context.Context) error {
	select {
	case <-db.closing:
		return ErrClosed
	default:
	}

	// A query rather than a ping: it proves a connection can be obtained and a
	// statement can run on it, which is what a caller is about to need.
	var ok int
	if err := db.read.QueryRowContext(ctx, "SELECT 1").Scan(&ok); err != nil {
		return fmt.Errorf("store: database %q is not answering: %w", db.path, err)
	}
	return nil
}

// Close stops accepting writes, waits for the in-flight write (there is at most
// one) to commit or roll back, and closes the connections. It is safe to call
// from any number of goroutines and any number of times; every call returns the
// same error.
func (db *DB) Close() error {
	db.closeOnce.Do(func() {
		// Turn away new writers first, then wait for the one that may already
		// hold the lock. Writers queued behind it leave through the closing
		// case in acquireWrite instead of waiting for a lock this never gives
		// back. A writer that wins the race to acquire it here is in flight by
		// definition, and is waited for like any other.
		close(db.closing)
		db.writeLock <- struct{}{}

		// The write connection is checked out of the pool, and sql.DB.Close
		// does not close a connection that is still checked out, so it goes
		// first. Closing read closes the connector, and with it the DuckDB
		// instance.
		db.closeErr = errors.Join(db.writeConn.Close(), db.read.Close())
	})
	return db.closeErr
}

// acquireWrite takes the write lock and returns the function that gives it back.
// On error the lock is not held.
func (db *DB) acquireWrite(ctx context.Context) (func(), error) {
	// Both conditions are checked before the blocking select as well as inside
	// it, because a select whose cases are all ready picks one at random: an
	// already-cancelled caller must not be able to win the lock and write
	// anyway.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-db.closing:
		return nil, ErrClosed
	default:
	}

	select {
	case db.writeLock <- struct{}{}:
	case <-db.closing:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Cancelled while queued. BeginTx would fail on this context anyway, but
	// the guarantee that a cancelled write applies nothing should not depend on
	// database/sql's internals.
	if err := ctx.Err(); err != nil {
		<-db.writeLock
		return nil, err
	}

	return func() { <-db.writeLock }, nil
}

// rollback undoes tx, tolerating a transaction database/sql has already rolled
// back itself — which it does, from its own goroutine, when the context passed
// to BeginTx is cancelled.
func rollback(tx *sql.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// initConn applies connSettings to a newly opened connection. A connection that
// cannot be configured is failed rather than used: a reader whose session is in
// the wrong time zone silently returns wrong data.
//
// The context is Background because the driver's hook does not pass one; the
// cost is three SET statements against an already-open connection.
func initConn(execer driver.ExecerContext) error {
	for _, stmt := range connSettings {
		if _, err := execer.ExecContext(context.Background(), stmt, nil); err != nil {
			return fmt.Errorf("store: apply connection setting %q: %w", stmt, err)
		}
	}
	return nil
}

// checkPath rejects the configurations that would otherwise surface as a DuckDB
// error an operator cannot act on.
func checkPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("no database path is configured")
	}

	// The driver reads a DSN, not a filename: everything after the first "?"
	// becomes DuckDB settings, and the failure is reported as an unrecognised
	// option named after the tail of the path.
	if strings.Contains(path, "?") {
		return errors.New(`the path must not contain "?", which the driver reads as the start of DSN options`)
	}

	// DuckDB creates the database file and its WAL, but not the directory they
	// live in.
	dir := filepath.Dir(path)
	switch info, err := os.Stat(dir); {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("its parent directory %q does not exist", dir)
	case err != nil:
		return fmt.Errorf("its parent directory %q could not be read: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("its parent %q is not a directory", dir)
	}
	return nil
}

// openError names the file in every failure to open it. An operator reading a
// log line needs to know which database, and the driver only sometimes says.
//
// A lock conflict is additionally tagged with [ErrLocked], so a caller can
// recognise the one open failure that is not a fault — a second process on a
// perfectly healthy database — and say what to do about it. The driver's own
// message is kept: it carries the command and the PID holding the lock.
func openError(path string, err error) error {
	if isLockConflict(err) {
		return fmt.Errorf("store: open database %q: %w: %w", path, ErrLocked, err)
	}
	return fmt.Errorf("store: open database %q: %w", path, err)
}

// lockConflictMarker appears in the DuckDB message for exactly one condition:
// the database file is held by another process. The full text is
//
//	IO Error: Could not set lock on file "x.duckdb": Conflicting lock is held
//	in /usr/local/bin/blacklight (PID 1) by user blacklight.
const lockConflictMarker = "Conflicting lock is held"

// isLockConflict reports whether err is DuckDB refusing to open a file another
// process already has open.
//
// The driver exposes a category ([duckdb.Error.Type]) but no code, and the
// category is IO — shared with a missing directory and a full disk. So the
// category narrows it and the message decides. Matching on message text is
// fragile by nature, which is why getting it wrong is survivable here: the
// error is still returned, still names the file, and still carries DuckDB's
// explanation. Only the extra sentence advising which process to stop is lost.
func isLockConflict(err error) bool {
	var dbErr *duckdb.Error
	if !errors.As(err, &dbErr) || dbErr.Type != duckdb.ErrorTypeIO {
		return false
	}
	return strings.Contains(dbErr.Msg, lockConflictMarker)
}
