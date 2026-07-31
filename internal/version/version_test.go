package version_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/bryanster/purpleops/internal/version"
)

func TestGetNeverReturnsEmptyFields(t *testing.T) {
	// The test binary is built without ldflags, so this exercises the
	// placeholder path: callers must never see an empty string.
	got := version.Get()

	if got.Version == "" || got.Commit == "" || got.BuildDate == "" {
		t.Fatalf("Get() returned an empty field: %#v", got)
	}
	if version.Stamped() {
		t.Errorf("Stamped() = true for a `go test` binary; ldflags cannot have been applied")
	}
	for _, want := range []string{got.Version, got.Commit, got.BuildDate} {
		if !strings.Contains(got.String(), want) {
			t.Errorf("String() = %q, missing %q", got.String(), want)
		}
	}
}

// ldflagsPattern matches one `-X <import/path>.<var>=<value>` linker flag.
var ldflagsPattern = regexp.MustCompile(`-X\s+(\S+?)=(\S*)`)

// TestLDFlagsPopulateInfo is the guard the ticket asks for: it proves the
// -X paths in the Makefile actually reach these variables. A typo in the
// package path, or a rename of `version`/`commit`/`buildDate`, silently
// produces a binary that reports "dev" — this fails instead.
//
// It reads the flags from `make print-ldflags` rather than hard-coding them, so
// the Makefile stays the single source of truth for how a build is stamped.
func TestLDFlagsPopulateInfo(t *testing.T) {
	root := repoRoot(t)

	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed; this test verifies the Makefile's ldflags")
	}
	out, err := runIn(t, root, "make", "-s", "print-ldflags")
	if err != nil {
		t.Fatalf("make print-ldflags: %v", err)
	}

	const pkg = "github.com/bryanster/purpleops/internal/version"
	sentinels := map[string]string{
		pkg + ".version":   "v0.0.0-ldflags-test",
		pkg + ".commit":    "0123456789ab",
		pkg + ".buildDate": "2026-01-02T03:04:05Z",
	}

	// Re-stamp with known values: the real ones vary per checkout.
	stamped := map[string]bool{}
	flags := ldflagsPattern.ReplaceAllStringFunc(strings.TrimSpace(out), func(flag string) string {
		target := ldflagsPattern.FindStringSubmatch(flag)[1]
		sentinel, ok := sentinels[target]
		if !ok {
			t.Errorf("Makefile stamps unexpected symbol %q; update this test or fix the Makefile", target)
			return flag
		}
		stamped[target] = true
		return "-X " + target + "=" + sentinel
	})
	for target := range sentinels {
		if !stamped[target] {
			t.Errorf("Makefile LDFLAGS does not stamp %q (got %q)", target, out)
		}
	}
	if t.Failed() {
		t.FailNow() // building with bogus flags below would only add noise
	}

	probe := filepath.Join(t.TempDir(), "versionprobe")
	if _, err := runIn(t, root, "go", "build", "-ldflags", flags, "-o", probe, "./internal/version/testdata/versionprobe"); err != nil {
		t.Fatalf("building the probe with the Makefile's ldflags failed: %v", err)
	}
	got, err := runIn(t, root, probe)
	if err != nil {
		t.Fatalf("running the probe: %v", err)
	}

	want := strings.Join([]string{
		sentinels[pkg+".version"],
		sentinels[pkg+".commit"],
		sentinels[pkg+".buildDate"],
		"true", // version.Stamped()
	}, "\t")
	if strings.TrimSpace(got) != want {
		t.Errorf("stamped binary reported\n\t%q\nwant\n\t%q", strings.TrimSpace(got), want)
	}
}

func runIn(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w\n%s", name, err, out)
	}
	return string(out), nil
}

// repoRoot returns the module root, derived from this file's own path so the
// test does not depend on the working directory it is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repository root")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
