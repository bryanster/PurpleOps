// Command versionprobe exists only for TestLDFlagsPopulateInfo.
//
// It lives under testdata so that `go build ./...` ignores it and so that the
// test never has to link the real server — which grows a CGO DuckDB dependency
// in M0B-003 and would make this test take minutes instead of a second.
package main

import (
	"fmt"

	"github.com/bryanster/blacklight/internal/version"
)

func main() {
	i := version.Get()
	fmt.Printf("%s\t%s\t%s\t%t\n", i.Version, i.Commit, i.BuildDate, version.Stamped())
}
