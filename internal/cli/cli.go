package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
)

// Exit codes. See the package comment for what a caller may conclude from each.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// name is the binary, used to prefix errors on stderr the way a Unix tool does.
const name = "popsctl"

// Main runs one command and returns the process exit code. It writes
// everything it has to say to the streams it is given and never calls
// os.Exit, so a test can run any command the binary can.
//
// args excludes the program name, as os.Args[1:] does.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	a := &app{out: stdout, errOut: stderr}
	//nolint:contextcheck // ctx reaches the commands through cobra: it is handed
	// to ExecuteContext below and read back by every RunE as cmd.Context().
	root := newRoot(a)
	root.SetArgs(args)
	// Help and usage follow the same rule as everything else: asked-for output
	// (`--help`) is the result and goes to stdout; usage printed because a
	// command line was wrong is a diagnostic and is written to stderr below.
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.ExecuteContext(ctx)
	switch {
	case err == nil:
		return ExitOK

	case isUsage(err):
		var usage *usageError
		errors.As(err, &usage)
		// The usage block, not the whole help: the reader is looking for the
		// one flag or subcommand they got wrong, not for a tour of the tool.
		fmt.Fprintf(stderr, "%s: %v\n\n%s", name, err, usage.cmd.UsageString())
		return ExitUsage

	default:
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return ExitFailure
	}
}

// app is the state shared by every command: where output goes, what the global
// flags were set to, and the configuration and logger built from them.
//
// The configuration is loaded on first use rather than up front, so that
// `popsctl version`, `popsctl --help` and the stub commands still work on a
// machine whose environment a server would refuse to start on.
type app struct {
	out    io.Writer
	errOut io.Writer

	// Global flags. See newRoot for what each one means. An empty logLevel is
	// "not given"; the flag rejects anything that is not a level, so a value
	// that got this far is one config would accept.
	jsonOut  bool
	dbPath   string
	logLevel config.LogLevel

	cfg    config.Tool
	loaded bool
	log    *slog.Logger
}

// settings returns the configuration this invocation runs with: the
// environment, with the global flags applied over it.
func (a *app) settings() (config.Tool, error) {
	if a.loaded {
		return a.cfg, nil
	}

	cfg, err := config.LoadTool()
	if err != nil {
		return config.Tool{}, err
	}
	if a.dbPath != "" {
		cfg.Database.Path = a.dbPath
	}
	if a.logLevel != "" {
		cfg.Log.Level = a.logLevel
	}

	a.cfg, a.loaded = cfg, true
	return a.cfg, nil
}

// logger returns the process logger, which writes to stderr in the configured
// format. Nothing a command logs is its result — that goes to stdout.
func (a *app) logger(cfg config.Tool) *slog.Logger {
	if a.log != nil {
		return a.log
	}

	options := &slog.HandlerOptions{Level: cfg.Log.Level.Slog()}
	var handler slog.Handler
	if cfg.Log.Format == config.FormatText {
		handler = slog.NewTextHandler(a.errOut, options)
	} else {
		handler = slog.NewJSONHandler(a.errOut, options)
	}
	a.log = slog.New(handler)
	return a.log
}

// withStore opens the database, runs fn against it, and closes it again.
//
// Closing is not optional bookkeeping: DuckDB checkpoints the write-ahead log
// on close, and a command that exited without doing so would leave the next
// process — usually the server starting back up — to recover it.
func (a *app) withStore(ctx context.Context, fn func(context.Context, *store.DB) error) (err error) {
	cfg, err := a.settings()
	if err != nil {
		return err
	}

	db, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return openFailure(cfg.Database.Path, err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return fn(ctx, db)
}

// openFailure explains the one failure to open a database that is not a fault:
// somebody else has it. The driver reports it as an IO error about locks, which
// is accurate and tells an operator nothing about what to do next.
func openFailure(path string, err error) error {
	if !errors.Is(err, store.ErrLocked) {
		return err
	}
	// Not "run it inside the container": being in the same container is still
	// being a second process, and an operator who reaches this message from
	// `docker compose exec` would try it and fail again. Stopping the holder is
	// the only thing that works.
	return fmt.Errorf("%w\n\n"+
		"DuckDB gives a database file to one process at a time, and something else is holding\n"+
		"%s — normally the server. Stop it and run this again;\n"+
		"with the container image that is:\n\n"+
		"    docker compose stop\n"+
		"    docker compose run --rm purpleops popsctl <command>",
		err, path)
}

// usageError is a command line this tool could not make sense of. It is a
// distinct type so that [Main] can exit 2 for it without pattern-matching on
// the text of cobra's errors.
type usageError struct {
	cmd *cobra.Command
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func isUsage(err error) bool {
	var usage *usageError
	return errors.As(err, &usage)
}

// usagef reports a bad command line against cmd, whose usage block the caller
// will see.
func usagef(cmd *cobra.Command, format string, args ...any) error {
	return &usageError{cmd: cmd, err: fmt.Errorf(format, args...)}
}
