package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	out := flag.String("out", "", "path to write the rendered document to (required)")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "authzdoc: -out is required")
		os.Exit(2)
	}

	// 0o644: this is documentation, committed to the repository and read by
	// everyone who reads the repository.
	if err := os.WriteFile(*out, render(), 0o644); err != nil { //nolint:gosec // documentation, not a secret
		fmt.Fprintf(os.Stderr, "authzdoc: writing %s: %v\n", *out, err)
		os.Exit(1)
	}
}
