// Command purpleops is the PurpleOps server: a single binary serving the API,
// the embedded SPA and an embedded DuckDB database.
//
// It currently only reports its build identity. The HTTP server arrives in
// M0B-006.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bryanster/purpleops/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "purpleops:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("purpleops", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version information and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, version.Get())
		return nil
	}

	// Until M0B-006 wires up the HTTP server there is nothing to serve, so the
	// bare invocation reports what it is and exits successfully.
	fmt.Fprintln(stdout, "purpleops", version.Get())
	return nil
}
