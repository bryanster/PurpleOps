// Package version exposes the build identity of the running binary.
//
// The three unexported variables below are populated at link time by
// `-ldflags -X`; see the LDFLAGS variable in the Makefile. They are deliberately
// unexported so that the only way to read them is through [Get], which
// substitutes readable placeholders for an unstamped (`go run`, `go test`,
// plain `go build`) binary rather than returning empty strings.
package version

// Injected via -ldflags at build time. Renaming any of these, or moving this
// package, breaks the -X paths in the Makefile — TestLDFlagsPopulateInfo exists
// to turn that into a failing test instead of a binary that reports "dev".
var (
	version   string
	commit    string
	buildDate string
)

// Placeholders reported by [Get] for a binary built without version stamping.
const (
	unknownVersion   = "dev"
	unknownCommit    = "unknown"
	unknownBuildDate = "unknown"
)

// Info is the build identity of this binary. It is the payload of the
// `GET /version` endpoint (M0B-005) and of `purpleops --version`.
type Info struct {
	// Version is the release identifier, e.g. "v2.1.0" or "v2.1.0-3-gabc1234".
	Version string `json:"version"`
	// Commit is the git SHA the binary was built from.
	Commit string `json:"commit"`
	// BuildDate is the UTC build timestamp, RFC 3339.
	BuildDate string `json:"buildDate"`
}

// Get returns the build identity. Every field is non-empty: an unstamped build
// reports placeholders, so callers never have to special-case the empty string.
func Get() Info {
	return Info{
		Version:   orDefault(version, unknownVersion),
		Commit:    orDefault(commit, unknownCommit),
		BuildDate: orDefault(buildDate, unknownBuildDate),
	}
}

// Stamped reports whether this binary was built with version ldflags. A release
// build for which this is false was mis-built and should not be shipped.
func Stamped() bool {
	return version != "" && commit != "" && buildDate != ""
}

// String renders the identity on one line, for `--version` and startup logs.
func (i Info) String() string {
	return i.Version + " (commit " + i.Commit + ", built " + i.BuildDate + ")"
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
