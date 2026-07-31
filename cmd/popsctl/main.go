// Command popsctl is the PurpleOps administrative CLI: user management,
// content sync, backup and report rendering against a local database.
//
// The subcommand tree is built in M0B-014; for now it only reports its build
// identity, so that the binary and its Makefile target exist from the start.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bryanster/purpleops/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "popsctl:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("popsctl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version information and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, version.Get())
		return nil
	}

	fmt.Fprintln(stdout, "popsctl", version.Get())
	return nil
}
