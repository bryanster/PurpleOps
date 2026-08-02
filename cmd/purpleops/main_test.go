package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/version"
)

// TestRunStartsAndStopsCleanly is the wiring test for the whole process:
// configuration, store, migrations, server and listener, then a signal and a
// graceful exit. Each piece is tested in its own package; what this asserts is
// that they are connected, in that order, and that stopping is a clean exit
// rather than something an orchestrator reports as a crash.
func TestRunStartsAndStopsCleanly(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, map[string]string{
		"PURPLEOPS_ENV":            "development",
		"PURPLEOPS_ADDR":           "127.0.0.1:0",
		"PURPLEOPS_BASE_URL":       "http://localhost:8080",
		"PURPLEOPS_DB_PATH":        filepath.Join(dir, "purpleops.duckdb"),
		"PURPLEOPS_EVIDENCE_DIR":   filepath.Join(dir, "evidence"),
		"PURPLEOPS_SESSION_SECRET": "Qk3nP7wZs9Lx2Vd4Rt6Yu8Ia0Oe5Cg1Hj3Mb7Nv9=",
		"PURPLEOPS_ENCRYPTION_KEY": "7Xb2Fq8Jm4Ts6Wp0Zc3Vn5Ky9Ld1Ru7Ae5Gh2Bi4=",
		"PURPLEOPS_LOG_FORMAT":     "json",
	})

	logs := &syncBuffer{}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, io.Discard, logs) }()

	// The listener is up once it has said so — waiting on the log rather than
	// on a sleep, so a slow machine makes the test slower and not flaky.
	waitFor(t, logs, "listening")

	stop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil: a signal is a clean stop", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("run() did not return after the context was cancelled")
	}

	// The store was opened and migrated on the way: the file exists, and the
	// migrator said what it did.
	if _, err := os.Stat(filepath.Join(dir, "purpleops.duckdb")); err != nil {
		t.Errorf("the database file was not created: %v", err)
	}
	if logged := logs.String(); !strings.Contains(logged, "applied migration") {
		t.Errorf("no migration was applied before serving:\n%s", logged)
	}
}

// TestRunReportsABadConfiguration keeps a misconfigured deployment a sentence
// on stderr rather than a server that starts and then cannot do anything.
func TestRunReportsABadConfiguration(t *testing.T) {
	setEnv(t, map[string]string{
		"PURPLEOPS_BASE_URL":       "not-a-url",
		"PURPLEOPS_SESSION_SECRET": "short",
	})

	err := run(context.Background(), nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("run() = nil, want the configuration errors")
	}
	for _, want := range []string{"PURPLEOPS_BASE_URL", "PURPLEOPS_SESSION_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s: every problem is reported, not just the first", err, want)
		}
	}
	// The rejected secret must not be quoted back into an error that will be
	// logged.
	if strings.Contains(err.Error(), "short") {
		t.Errorf("the error repeats the secret value: %v", err)
	}
}

func TestVersionFlagPrintsTheBuildAndExits(t *testing.T) {
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--version"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run(--version) = %v, want nil", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), version.Get().String(); got != want {
		t.Errorf("--version printed %q, want %q", got, want)
	}
}

// serverEnv is every variable the server reads (.env.example). The tests set
// all of them, so a developer's own PURPLEOPS_* exports cannot change what is
// being tested — and reading the process environment to find them is what
// TestOnlyConfigReadsTheEnvironment forbids outside internal/config.
var serverEnv = []string{
	"PURPLEOPS_ENV",
	"PURPLEOPS_ADDR",
	"PURPLEOPS_BASE_URL",
	"PURPLEOPS_REQUEST_TIMEOUT",
	"PURPLEOPS_SHUTDOWN_TIMEOUT",
	"PURPLEOPS_TRUSTED_PROXIES",
	"PURPLEOPS_DB_PATH",
	"PURPLEOPS_EVIDENCE_DIR",
	"PURPLEOPS_SESSION_SECRET",
	"PURPLEOPS_ENCRYPTION_KEY",
	"PURPLEOPS_MFA_PENDING_TTL",
	"PURPLEOPS_LOG_LEVEL",
	"PURPLEOPS_LOG_FORMAT",
	"PURPLEOPS_CHROME_PATH",
}

// setEnv applies env for the duration of the test and empties every other
// variable the server reads — config treats an empty value as unset.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()

	for name := range env {
		if !slices.Contains(serverEnv, name) {
			t.Fatalf("%s is not a variable the server reads; check the name", name)
		}
	}
	for _, name := range serverEnv {
		t.Setenv(name, env[name])
	}
}

// waitFor blocks until the log contains message, which is how a test
// synchronises with a server that is starting in another goroutine.
func waitFor(t *testing.T, logs *syncBuffer, message string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(logs.String(), message) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the server never logged %q:\n%s", message, logs.String())
}

// syncBuffer is a bytes.Buffer safe to read while the server writes to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
