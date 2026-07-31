// Command lockprobe exists only for TestASecondProcessCannotMigrateTheSameFile.
//
// The behaviour under test — DuckDB refusing a second read-write process on one
// database file — is invisible from inside a single process, because DuckDB's
// instance cache hands two handles in the same process the same instance. So it
// needs a real second process.
//
// It lives under testdata so that `go build ./...` ignores it.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/migrate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Println("FAILED:", err)
		os.Exit(1)
	}
	fmt.Println("MIGRATED")
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lockprobe <database path>")
	}

	ctx := context.Background()
	db, err := store.Open(ctx, config.Database{Path: args[0]})
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // The exit status already says what happened.

	_, err = migrate.Up(ctx, db)
	return err
}
