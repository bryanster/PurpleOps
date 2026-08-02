// Package cli is the command tree behind blctl, the administrative CLI
// (PLAN.md §6). cmd/blctl is a main function over it and nothing else, so
// every command is reachable from a test without spawning a process.
//
// It is the second entrypoint into one codebase: the same internal packages,
// the same environment, the same database. What differs is that a command runs
// once and exits, and that it is usually run by a person at a terminal or by a
// script — which is what the three rules below are for.
//
// # Streams
//
// The command's result goes to stdout; everything else — progress, warnings,
// log lines, errors — goes to stderr. With --json the result is exactly one
// JSON document, so `blctl db info --json | jq` works while the log is still
// visible in the terminal.
//
// # Exit codes
//
//	0  the command did what it said
//	1  it ran and failed (locked database, bad migration, feature not built yet)
//	2  the command line was wrong (unknown command or flag, missing subcommand)
//
// The split matters to a script: a 2 will not be fixed by retrying, and a
// deployment pipeline should treat it as a typo in itself rather than as a
// broken deployment. Every leaf command names its arguments strictly for the
// same reason.
//
// # One process at a time
//
// DuckDB gives a database file to a single process, so a command run against a
// deployment whose server is up cannot open its database (see
// [store.ErrLocked]). That is a property of the storage engine rather than
// something this package could work around, so it is reported as advice —
// which process to stop, or which container to run inside — instead of a
// driver error about locks.
//
// # Why cobra
//
// The subcommand tree, per-command --help, and shell completion are all things
// this would otherwise grow by hand as M1 through M7 add commands. Between the
// two candidates in the ticket, cobra is the one most contributors have already
// used, and the one whose conventions an operator's fingers already know. The
// cost is two dependencies (cobra and pflag) that nothing else in the tree
// touches — the server does not import this package.
package cli
