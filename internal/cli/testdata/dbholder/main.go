// Command dbholder exists only for TestRefusesADatabaseAnotherProcessHolds.
//
// DuckDB gives a database file to one process at a time, and that is invisible
// from inside a single process: two opens of the same path there share one
// instance and both succeed. So the test that asserts blctl refuses a
// database the server is holding needs a real second process to do the holding
// — this one, which opens the file and then does nothing until its stdin
// closes.
//
// It lives under testdata so that `go build ./...` ignores it; the test
// compiles it. internal/store/migrate/testdata/lockprobe is the same idea from
// the other side, where the second process is the one that fails.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
)

// held is what the test waits for before it does anything: a holder that has
// not opened the file yet would let the test pass for the wrong reason.
const held = "HELD"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Println("FAILED:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: dbholder <database path>")
	}

	ctx := context.Background()
	db, err := store.Open(ctx, config.Database{Path: args[0]})
	if err != nil {
		return err
	}

	fmt.Println(held)

	// Waiting on stdin rather than on a signal or a timeout: the test releases
	// the database by closing the pipe, so the holder cannot outlive it and be
	// left holding a file in somebody's temporary directory.
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		return err
	}
	return db.Close()
}
