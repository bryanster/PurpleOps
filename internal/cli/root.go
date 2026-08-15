package cli

import (
	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/version"
)

// newRoot builds the whole command tree. It takes the app rather than reading
// anything itself, so a test constructs the same tree the binary runs.
func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   name,
		Short: "Administer a Blacklight deployment",
		Long: "blctl administers a Blacklight deployment from the command line: the database\n" +
			"it stores everything in, the users who may sign in, and the content and reports\n" +
			"built on top of them.\n\n" +
			"It reads the same BLACKLIGHT_* environment as the server (see .env.example) and\n" +
			"opens the same database file — which DuckDB will only hand to one process, so\n" +
			"most commands need the server stopped, or must run inside its container.",

		// Errors and usage are printed by Main, which knows which exit code
		// goes with which; cobra would otherwise print a usage block after
		// every runtime failure, where it is noise.
		SilenceUsage:  true,
		SilenceErrors: true,

		// --version, rather than only a subcommand, because it is the first
		// thing anybody types at an unfamiliar binary. Both print the same
		// line; `version --json` is the machine-readable form.
		Version: version.Get().String(),

		Args: subcommandArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return usagef(cmd, "a command is required")
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")

	// A bad flag is a bad command line, not a runtime failure — this is what
	// makes pflag's errors exit 2 like the rest of them. It is inherited by
	// every subcommand.
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{cmd: cmd, err: err}
	})

	flags := root.PersistentFlags()
	flags.StringVar(&a.dbPath, "db", "",
		"database file to work on (default: BLACKLIGHT_DB_PATH, then ./blacklight.duckdb)")
	flags.Var(&logLevelFlag{level: &a.logLevel}, "log-level",
		"log verbosity on stderr: debug, info, warn or error (default: BLACKLIGHT_LOG_LEVEL)")
	flags.BoolVar(&a.jsonOut, "json", false,
		"print the result as JSON on stdout, leaving stderr for logs")

	root.AddCommand(
		newVersionCommand(a),
		newMigrateCommand(a),
		newDBCommand(a),
		newUserCommand(a),
		newSetupCommand(a),
		newContentCommand(a),
		group("engagement", "Manage engagements",
			newArchiveCommand(a),
		),
		newBackupCommand(a),
		newReportCommand(),
	)
	return root
}

// group is a command that exists to hold others: `blctl migrate`, which does
// nothing on its own.
//
// Running one without a subcommand is a usage error rather than a help page and
// a zero exit, because the invocation that reaches here is a script with a typo
// far more often than it is a person looking around. Somebody looking around
// types --help, and gets the same text on stdout and a zero exit.
func group(use, short string, subs ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  subcommandArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return usagef(cmd, "%q needs a subcommand", cmd.CommandPath())
		},
	}
	cmd.AddCommand(subs...)
	return cmd
}

// subcommandArgs is Args for a command that only holds others. Cobra's own
// handling of an unknown subcommand produces an untyped error that [Main] would
// have to recognise by its text; this produces a [usageError] instead.
func subcommandArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil // handled by RunE, which knows what to say about it
	}

	err := usagef(cmd, "unknown subcommand %q for %q", args[0], cmd.CommandPath())
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		err = usagef(cmd, "unknown subcommand %q for %q — did you mean %q?",
			args[0], cmd.CommandPath(), suggestions[0])
	}
	return err
}

// logLevelFlag lets pflag parse a --log-level through config's own
// [config.LogLevel], which means the flag accepts exactly what
// BLACKLIGHT_LOG_LEVEL accepts and says the same thing when it does not — and
// that a bad value is rejected during flag parsing, so it exits 2 as a bad
// command line rather than 1 as a failure to run.
type logLevelFlag struct {
	level *config.LogLevel
}

func (f *logLevelFlag) String() string { return f.level.String() }
func (f *logLevelFlag) Type() string   { return "level" }

// Set returns config's own message unchanged: pflag already prefixes it with
// the flag and the value it was given.
func (f *logLevelFlag) Set(raw string) error { return f.level.UnmarshalText([]byte(raw)) }

// noArgs is Args for a command that takes none. Every command in this tree
// takes its input from flags, so a positional argument is always either a typo
// or a flag whose value lost its "--" — both worth stopping for.
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usagef(cmd, "%q takes no arguments, got %q", cmd.CommandPath(), args[0])
	}
	return nil
}
