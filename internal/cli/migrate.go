package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/migrate"
)

// The server migrates on startup (cmd/purpleops), so these commands exist for
// the cases where that is not enough: seeing what a release is about to do
// before it does it, and applying a schema change to a database whose server is
// deliberately still down.

func newMigrateCommand(a *app) *cobra.Command {
	return group("migrate", "Inspect and apply database migrations",
		newMigrateStatusCommand(a),
		newMigrateUpCommand(a),
	)
}

// migrationReport is the JSON shape of both migrate commands: the same field
// means the same thing whichever one produced it.
type migrationReport struct {
	// Database is echoed because --db and PURPLEOPS_DB_PATH make "which
	// database did this just report on" a real question.
	Database string `json:"database"`
	// SchemaVersion is the highest migration this database has applied, and 0
	// for one that has never been migrated.
	SchemaVersion int `json:"schemaVersion"`
	// ExpectedSchemaVersion is the highest this binary carries. Higher than
	// SchemaVersion means there is something left to apply.
	ExpectedSchemaVersion int `json:"expectedSchemaVersion"`
	Pending               int `json:"pending"`
	// AppliedNow are the versions this invocation applied. Absent from
	// `migrate status`, which applies nothing, and from a `migrate up` that
	// found nothing to do.
	AppliedNow []int              `json:"appliedNow,omitempty"`
	Migrations []migrationDetails `json:"migrations"`
}

type migrationDetails struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	File    string `json:"file"`
	Applied bool   `json:"applied"`
	// AppliedAt is absent for a migration that has not run.
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
}

func newMigrateStatusCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "List every migration and whether this database has applied it",
		Long: "Reports the migrations built into this binary against the ones the database\n" +
			"has recorded, in version order.\n\n" +
			"It fails rather than reporting if the two disagree: a migration applied under a\n" +
			"name or contents this binary does not have means the database in front of you is\n" +
			"not the one this build's queries were written against (docs/migrations.md).\n\n" +
			"The database is opened read-write, and created if it does not exist — so this is\n" +
			"not a command to point at a path you are unsure of.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				cfg, err := a.settings()
				if err != nil {
					return err
				}
				report, err := a.readMigrations(ctx, db, cfg)
				if err != nil {
					return err
				}
				return a.print(report, func(w *tabwriter.Writer) {
					writeMigrationTable(w, report)
				})
			})
		},
	}
}

func newMigrateUpCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply every migration this database has not run yet",
		Long: "Applies the pending migrations in version order, each in its own transaction,\n" +
			"and stops at the first failure with everything before it still applied.\n\n" +
			"The server does this for itself at startup, so running it by hand is for the\n" +
			"deployment where you want the schema change to happen — and to be seen to\n" +
			"happen — before the new binary starts serving. There are no down migrations;\n" +
			"docs/migrations.md says what to do instead.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				cfg, err := a.settings()
				if err != nil {
					return err
				}

				// The migrator logs a line per file as it goes, on stderr. The
				// report below is the summary, on stdout, and is read after the
				// fact by whatever is watching the deployment.
				applied, err := migrate.Up(ctx, db, migrate.WithLogger(a.logger(cfg)))
				if err != nil {
					return err
				}

				report, err := a.readMigrations(ctx, db, cfg)
				if err != nil {
					return err
				}
				for _, migration := range applied {
					report.AppliedNow = append(report.AppliedNow, migration.Version)
				}

				return a.print(report, func(w *tabwriter.Writer) {
					if len(applied) == 0 {
						fmt.Fprintf(w, "Already up to date at version %04d.\n\n",
							report.SchemaVersion)
					} else {
						files := make([]string, len(applied))
						for i, migration := range applied {
							files[i] = migration.Filename()
						}
						fmt.Fprintf(w, "Applied %s: %s\n\n",
							plural(len(applied), "migration"), strings.Join(files, ", "))
					}
					writeMigrationTable(w, report)
				})
			})
		},
	}
}

// readMigrations asks the migrator what the database has run and shapes the
// answer. Both commands report the same thing, so both build it here.
func (a *app) readMigrations(ctx context.Context, db *store.DB, cfg config.Tool) (migrationReport, error) {
	states, err := migrate.Status(ctx, db, migrate.WithLogger(a.logger(cfg)))
	if err != nil {
		return migrationReport{}, err
	}

	report := migrationReport{
		Database:   cfg.Database.Path,
		Migrations: make([]migrationDetails, 0, len(states)),
	}
	for _, state := range states {
		details := migrationDetails{
			Version: state.Version,
			Name:    state.Name,
			File:    state.Filename(),
			Applied: state.Applied,
		}
		if state.Applied {
			// A copy, because the loop variable's address would be the same
			// pointer in every element.
			appliedAt := state.AppliedAt
			details.AppliedAt = &appliedAt
			report.SchemaVersion = max(report.SchemaVersion, state.Version)
		} else {
			report.Pending++
		}
		report.ExpectedSchemaVersion = max(report.ExpectedSchemaVersion, state.Version)
		report.Migrations = append(report.Migrations, details)
	}
	return report, nil
}

func writeMigrationTable(w *tabwriter.Writer, report migrationReport) {
	fmt.Fprintln(w, "VERSION\tNAME\tSTATUS\tAPPLIED AT")
	for _, migration := range report.Migrations {
		status, appliedAt := "pending", "-"
		if migration.Applied {
			// RFC 3339 in UTC, as everywhere else in this application. A time
			// formatted for the reader's locale is a time nobody can grep for.
			status, appliedAt = "applied", migration.AppliedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%04d\t%s\t%s\t%s\n", migration.Version, migration.Name, status, appliedAt)
	}
	fmt.Fprintf(w, "\n%d of %d applied, %d pending.\n",
		len(report.Migrations)-report.Pending, len(report.Migrations), report.Pending)
}
