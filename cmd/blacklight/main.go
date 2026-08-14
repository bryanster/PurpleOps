// Command blacklight is the Blacklight server: a single binary serving the API,
// the embedded SPA and an embedded DuckDB database.
//
// Everything it needs is in the environment (internal/config) — there are no
// flags beyond --version, so a container and a systemd unit are configured the
// same way. It runs until it is asked to stop, and a SIGINT or SIGTERM starts a
// graceful shutdown rather than dropping whatever is in flight.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bryanster/blacklight/internal/bootstrap"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/events/presence"
	"github.com/bryanster/blacklight/internal/httpapi"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/migrate"
	"github.com/bryanster/blacklight/internal/version"
	"github.com/bryanster/blacklight/web"
)

func main() {
	// The signal handler is installed before anything else so that a Ctrl-C
	// during a slow startup — opening the database, applying migrations — is
	// still honoured. Stopping it restores the default disposition, which makes
	// a second signal kill the process outright: an operator who sends SIGTERM
	// twice means it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "blacklight:", err)
		os.Exit(1)
	}
}

// run is main with its environment passed in, so that everything below it is
// reachable from a test.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) (err error) {
	flags := flag.NewFlagSet("blacklight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print version information and exit")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(stdout, version.Get())
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg.Log, stderr)
	// Everything that logs without being handed a logger — a rollback failing
	// inside the store, a library — writes in the configured format and level
	// rather than to stderr in whatever shape it fancied.
	slog.SetDefault(log)

	log.InfoContext(ctx, "starting",
		slog.String("version", version.Get().Version),
		slog.String("commit", version.Get().Commit),
		slog.String("env", cfg.Env.String()),
		slog.String("base_url", cfg.Server.BaseURL.String()))

	db, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	// Closing the store is the last thing that happens, after the server has
	// stopped serving: a request still finishing its transaction must not find
	// the database gone. Its error joins whatever run is already returning, so
	// a failure to flush is not lost behind a successful shutdown.
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = joinErrors(err, closeErr)
		}
	}()

	if _, err := migrate.Up(ctx, db, migrate.WithLogger(log)); err != nil {
		return err
	}

	// Force a DuckDB CHECKPOINT so the WAL is flushed to disk. Without this,
	// a container restart replays the WAL that recorded migration 0016's
	// DROP TABLE, and DuckDB v2.10505.0 crashes in
	// WriteAheadLogDeserializer::ReplayDropTable (SIGSEGV in BindCheckConstraint).
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "CHECKPOINT")
		return err
	}); err != nil {
		return fmt.Errorf("checkpoint after migration: %w", err)
	}

	// After the schema exists and before anything is listening: on a database
	// with no accounts, and only then, this creates the administrator a
	// deployment tool configured. It is a no-op on every other start.
	if err := bootstrap.Apply(ctx, db, cfg.Bootstrap, log); err != nil {
		return err
	}

	presenceReg := presence.New(presence.Options{})

	ui, embedded := web.Dist()
	if !embedded {
		// Loud, because the symptom is otherwise a mystery: the API answers,
		// the health check is green, and the browser shows a page nobody wrote.
		log.WarnContext(ctx, "this binary carries a placeholder instead of the user interface; "+
			"it was built without the `spa` build tag — use `make build`")
	}

	handler, err := httpapi.NewServer(httpapi.Deps{
		Config:   cfg,
		Store:    db,
		Logger:   log,
		UI:       ui,
		Presence: presenceReg,
	})
	if err != nil {
		return err
	}

	return httpapi.ListenAndServe(ctx, cfg.Server, handler, log)
}

// newLogger builds the process logger from the configuration.
//
// It writes to stderr, leaving stdout for output a caller might parse
// (--version today, blctl's reports later), and it is created after the
// configuration is loaded — so a configuration error is a plain sentence on
// stderr rather than a JSON log line about the log format.
func newLogger(cfg config.Log, w io.Writer) *slog.Logger {
	options := &slog.HandlerOptions{Level: cfg.Level.Slog()}

	var handler slog.Handler
	if cfg.Format == config.FormatText {
		handler = slog.NewTextHandler(w, options)
	} else {
		handler = slog.NewJSONHandler(w, options)
	}
	return slog.New(handler)
}

// joinErrors keeps a deferred cleanup failure without discarding the failure
// that is already on its way out.
func joinErrors(first, second error) error {
	if first == nil {
		return second
	}
	return fmt.Errorf("%w; also: %w", first, second)
}
