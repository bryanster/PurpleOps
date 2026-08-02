// Command popsctl is the PurpleOps administrative CLI: migrations, database
// inspection, and — as the milestones that build them land — user management,
// content sync, backup and report rendering.
//
// It is a main function over [cli], which holds the command tree, so that every
// command is reachable from a test without a process to spawn. Run
// `popsctl --help` for the commands, and see the package documentation of
// internal/cli for the contract they all keep: results on stdout, everything
// else on stderr, and an exit code that distinguishes a bad command line from a
// command that ran and failed.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bryanster/purpleops/internal/cli"
)

func main() {
	// A command holds the database open and may be part-way through a write.
	// Cancelling the context rolls that transaction back and closes the file
	// cleanly, which matters more here than in a server: the next process to
	// want this database is usually the server starting back up. Stopping the
	// handler restores the default disposition, so a second Ctrl-C from an
	// impatient operator still kills the process outright.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Main(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
