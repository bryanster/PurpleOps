package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
)

// walSuffix is what DuckDB calls the write-ahead log beside a database file. It
// is reported separately because a large one is a signal in itself: the
// database was not closed cleanly, or a very large write is in progress.
const walSuffix = ".wal"

// listTables is ANSI information_schema rather than DuckDB's duckdb_tables(),
// so this command still works if the store is ever moved to another engine
// (PLAN.md §1).
//
// Every schema, not the current one: the tables that matter are in "app" and
// "content" (migration 0001), and only the migrator's bookkeeping lives in
// "main". The catalog is pinned so that a future ATTACHed database — a restore
// being compared against, say — cannot silently double the listing.
const listTables = `SELECT table_schema, table_name
	FROM information_schema.tables
	WHERE table_catalog = current_database()
	  AND table_type = 'BASE TABLE'
	ORDER BY table_schema, table_name`

func newDBCommand(a *app) *cobra.Command {
	return group("db", "Inspect the database file", newDBInfoCommand(a))
}

// dbInfo is what `db info --json` prints. It is the answer to "what am I
// actually connected to, and does it have anything in it".
type dbInfo struct {
	Path string `json:"path"`
	// SizeBytes and WALBytes are the files on disk, not the logical size of
	// the data: DuckDB reuses freed blocks, so a file does not shrink when
	// rows are deleted.
	SizeBytes int64 `json:"sizeBytes"`
	WALBytes  int64 `json:"walBytes"`

	SchemaVersion         int `json:"schemaVersion"`
	ExpectedSchemaVersion int `json:"expectedSchemaVersion"`
	PendingMigrations     int `json:"pendingMigrations"`

	Tables []tableInfo `json:"tables"`
}

type tableInfo struct {
	// Schema and Name are separate fields rather than one qualified string,
	// so a consumer can group by schema without parsing anything.
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Rows   int64  `json:"rows"`
}

// qualified is how a table is written in SQL and printed for a person.
func (t tableInfo) qualified() string { return t.Schema + "." + t.Name }

func newDBInfoCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Report the database's location, size, schema version and row counts",
		Long: "Prints where the database is, how large it and its write-ahead log are, which\n" +
			"schema version it is at, and how many rows are in each table.\n\n" +
			"This is the command to run before and after anything destructive, and the one\n" +
			"to paste into a bug report. Counting rows reads every table, so it is not free\n" +
			"on a large deployment — though DuckDB is columnar, so it is far cheaper than\n" +
			"the same question would be elsewhere.\n\n" +
			"The database is opened read-write, and created if it does not exist.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				cfg, err := a.settings()
				if err != nil {
					return err
				}
				info, err := a.readDBInfo(ctx, db, cfg)
				if err != nil {
					return err
				}
				return a.print(info, func(w *tabwriter.Writer) { writeDBInfo(w, info) })
			})
		},
	}
}

func (a *app) readDBInfo(ctx context.Context, db *store.DB, cfg config.Tool) (dbInfo, error) {
	migrations, err := a.readMigrations(ctx, db, cfg)
	if err != nil {
		return dbInfo{}, err
	}

	size, err := fileSize(cfg.Database.Path)
	if err != nil {
		return dbInfo{}, err
	}
	wal, err := fileSize(cfg.Database.Path + walSuffix)
	if err != nil {
		return dbInfo{}, err
	}

	tables, err := tableCounts(ctx, db)
	if err != nil {
		return dbInfo{}, err
	}

	return dbInfo{
		Path:                  cfg.Database.Path,
		SizeBytes:             size,
		WALBytes:              wal,
		SchemaVersion:         migrations.SchemaVersion,
		ExpectedSchemaVersion: migrations.ExpectedSchemaVersion,
		PendingMigrations:     migrations.Pending,
		Tables:                tables,
	}, nil
}

// tableCounts lists the tables and counts the rows in each.
//
// The names come from the catalog, so they are whatever the migrations created;
// they are quoted anyway, because building SQL from a string read out of the
// database is the shape of the mistake even when this particular string is
// safe.
func tableCounts(ctx context.Context, db *store.DB) ([]tableInfo, error) {
	tables, err := tableNames(ctx, db)
	if err != nil {
		return nil, err
	}

	for i, table := range tables {
		query := `SELECT count(*) FROM ` +
			quoteIdentifier(table.Schema) + "." + quoteIdentifier(table.Name)
		if err := db.Read().QueryRowContext(ctx, query).Scan(&tables[i].Rows); err != nil {
			return nil, fmt.Errorf("counting the rows in %s: %w", table.qualified(), err)
		}
	}
	return tables, nil
}

func tableNames(ctx context.Context, db *store.DB) ([]tableInfo, error) {
	rows, err := db.Read().QueryContext(ctx, listTables)
	if err != nil {
		return nil, fmt.Errorf("listing the tables: %w", err)
	}
	defer rows.Close()

	var tables []tableInfo
	for rows.Next() {
		var table tableInfo
		if err := rows.Scan(&table.Schema, &table.Name); err != nil {
			return nil, fmt.Errorf("listing the tables: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing the tables: %w", err)
	}
	return tables, nil
}

// quoteIdentifier renders name as a SQL identifier: double quotes, with any of
// its own doubled. ANSI, and what DuckDB and PostgreSQL both accept.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fileSize reports a file's size, treating "not there" as zero rather than as
// a failure: a database that has never been written has no write-ahead log,
// which is a fact about it and not an error.
func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("reading the size of %s: %w", path, err)
	}
	return info.Size(), nil
}

func writeDBInfo(w *tabwriter.Writer, info dbInfo) {
	fmt.Fprintf(w, "path\t%s\n", info.Path)
	fmt.Fprintf(w, "size\t%s\t(%d bytes)\n", humanBytes(info.SizeBytes), info.SizeBytes)
	fmt.Fprintf(w, "write-ahead log\t%s\t(%d bytes)\n", humanBytes(info.WALBytes), info.WALBytes)

	schema := fmt.Sprintf("%04d of %04d", info.SchemaVersion, info.ExpectedSchemaVersion)
	if info.PendingMigrations > 0 {
		schema += fmt.Sprintf("\t(%s pending — run `blctl migrate up`)",
			plural(info.PendingMigrations, "migration"))
	}
	fmt.Fprintf(w, "schema version\t%s\n", schema)

	fmt.Fprintf(w, "\nTABLE\tROWS\n")
	if len(info.Tables) == 0 {
		fmt.Fprintln(w, "(none)\t-")
		return
	}
	for _, table := range info.Tables {
		fmt.Fprintf(w, "%s\t%d\n", table.qualified(), table.Rows)
	}
}
