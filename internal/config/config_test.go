package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testSecret is 32 random bytes, base64 — the shape `openssl rand -base64 32`
// produces, and the shape every deployment should use.
const testSecret = "kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU="

// validEnv is the smallest environment that loads cleanly: the two required
// variables and nothing else, so every other assertion is about a default.
func validEnv() map[string]string {
	return map[string]string{
		envBaseURL:       "https://purpleops.internal",
		envSessionSecret: testSecret,
	}
}

// envWith returns validEnv overridden with the given pairs. A value of "" is
// set as an empty variable, which the loader treats as unset.
func envWith(overrides map[string]string) map[string]string {
	env := validEnv()
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func TestParseAppliesDocumentedDefaults(t *testing.T) {
	cfg, errs := parse(validEnv())
	if len(errs) > 0 {
		t.Fatalf("parse(validEnv()) = %v, want no errors", errs)
	}

	if got, want := cfg.Env, EnvProduction; got != want {
		t.Errorf("Env = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Addr, ":8080"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Server.ShutdownTimeout, 15*time.Second; got != want {
		t.Errorf("Server.ShutdownTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.Database.Path, "./purpleops.duckdb"; got != want {
		t.Errorf("Database.Path = %q, want %q", got, want)
	}
	if got, want := cfg.Evidence.Dir, "./evidence"; got != want {
		t.Errorf("Evidence.Dir = %q, want %q", got, want)
	}
	if got, want := cfg.Log.Level, LevelInfo; got != want {
		t.Errorf("Log.Level = %q, want %q", got, want)
	}
	if got, want := cfg.Log.Format, FormatJSON; got != want {
		t.Errorf("Log.Format = %q, want %q", got, want)
	}
	if got := cfg.Report.ChromePath; got != "" {
		t.Errorf("Report.ChromePath = %q, want empty when unset", got)
	}
	if cfg.Session.Secret.IsZero() {
		t.Error("Session.Secret is zero, want the value from the environment")
	}
	if got, want := cfg.Throttle, (Throttle{
		AccountFailures: 5,
		AccountLockout:  15 * time.Minute,
		SourceFailures:  50,
		SourceLockout:   15 * time.Minute,
	}); got != want {
		t.Errorf("Throttle = %+v, want %+v", got, want)
	}
}

// TestFields is the per-variable table the ticket asks for: at least one value
// that must be accepted and one that must be rejected for every variable, with
// the rejection asserted on the operator-visible text.
func TestFields(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string // substring of the load error; empty means "must load"
		check   func(t *testing.T, cfg Config)
	}{{
		name: "env development",
		env:  map[string]string{envEnv: "development"},
		check: func(t *testing.T, cfg Config) {
			if !cfg.Env.IsDevelopment() {
				t.Errorf("Env = %q, want development", cfg.Env)
			}
		},
	}, {
		name: "env is case insensitive",
		env:  map[string]string{envEnv: "Production"},
		check: func(t *testing.T, cfg Config) {
			if cfg.Env != EnvProduction {
				t.Errorf("Env = %q, want %q", cfg.Env, EnvProduction)
			}
		},
	}, {
		name:    "env rejects anything else",
		env:     map[string]string{envEnv: "staging"},
		wantErr: `PURPLEOPS_ENV: must be one of "development", "production", got "staging"`,
	}, {
		name: "addr accepts host and port",
		env:  map[string]string{envAddr: "127.0.0.1:9000"},
		check: func(t *testing.T, cfg Config) {
			if cfg.Server.Addr != "127.0.0.1:9000" {
				t.Errorf("Server.Addr = %q", cfg.Server.Addr)
			}
		},
	}, {
		name: "addr accepts port zero for an ephemeral listener",
		env:  map[string]string{envAddr: ":0"},
	}, {
		name:    "addr rejects a bare port",
		env:     map[string]string{envAddr: "8080"},
		wantErr: `PURPLEOPS_ADDR: must be a host:port listen address, such as ":8080" or "127.0.0.1:8080", got "8080"`,
	}, {
		name:    "addr rejects a port out of range",
		env:     map[string]string{envAddr: ":70000"},
		wantErr: `PURPLEOPS_ADDR: must have a port between 0 and 65535, got ":70000"`,
	}, {
		name:    "addr rejects a non-numeric port",
		env:     map[string]string{envAddr: ":http"},
		wantErr: `PURPLEOPS_ADDR: must have a numeric port, got ":http"`,
	}, {
		name: "base url keeps a subpath",
		env:  map[string]string{envBaseURL: "https://example.internal/purpleops"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.BaseURL.String(), "https://example.internal/purpleops"; got != want {
				t.Errorf("BaseURL = %q, want %q", got, want)
			}
		},
	}, {
		name: "base url loses its trailing slash",
		env:  map[string]string{envBaseURL: "https://example.internal/purpleops/"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.BaseURL.String(), "https://example.internal/purpleops"; got != want {
				t.Errorf("BaseURL = %q, want %q", got, want)
			}
		},
	}, {
		name: "base url of a bare host loses its trailing slash",
		env:  map[string]string{envBaseURL: "https://example.internal/"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.BaseURL.String(), "https://example.internal"; got != want {
				t.Errorf("BaseURL = %q, want %q", got, want)
			}
		},
	}, {
		// The exact text the ticket's acceptance criteria names.
		name:    "base url rejects a value with no scheme",
		env:     map[string]string{envBaseURL: "localhost:8080"},
		wantErr: `PURPLEOPS_BASE_URL: must be an absolute URL, got "localhost:8080"`,
	}, {
		name:    "base url rejects a path-only value",
		env:     map[string]string{envBaseURL: "/purpleops"},
		wantErr: `PURPLEOPS_BASE_URL: must be an absolute URL, got "/purpleops"`,
	}, {
		name:    "base url rejects a non-http scheme",
		env:     map[string]string{envBaseURL: "ftp://example.internal"},
		wantErr: `PURPLEOPS_BASE_URL: must use scheme http or https, got "ftp://example.internal"`,
	}, {
		name:    "base url rejects embedded credentials",
		env:     map[string]string{envBaseURL: "https://admin:hunter2@example.internal"},
		wantErr: `PURPLEOPS_BASE_URL: must not contain credentials`,
	}, {
		name:    "base url rejects a query string",
		env:     map[string]string{envBaseURL: "https://example.internal?tenant=1"},
		wantErr: `PURPLEOPS_BASE_URL: must not contain a query string or fragment`,
	}, {
		name:    "base url rejects a port out of range",
		env:     map[string]string{envBaseURL: "https://example.internal:70000"},
		wantErr: `PURPLEOPS_BASE_URL: must have a port between 1 and 65535`,
	}, {
		name:    "base url is required",
		env:     map[string]string{envBaseURL: ""},
		wantErr: `PURPLEOPS_BASE_URL: must be set`,
	}, {
		name: "shutdown timeout accepts a duration",
		env:  map[string]string{envShutdownTimeout: "45s"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.ShutdownTimeout, 45*time.Second; got != want {
				t.Errorf("ShutdownTimeout = %v, want %v", got, want)
			}
		},
	}, {
		name:    "shutdown timeout rejects a bare number",
		env:     map[string]string{envShutdownTimeout: "15"},
		wantErr: `PURPLEOPS_SHUTDOWN_TIMEOUT: must be a duration with a unit, such as "15s" or "2m", got "15"`,
	}, {
		name:    "shutdown timeout rejects zero",
		env:     map[string]string{envShutdownTimeout: "0s"},
		wantErr: `PURPLEOPS_SHUTDOWN_TIMEOUT: must be a positive duration, got "0s"`,
	}, {
		name:    "shutdown timeout rejects a negative duration",
		env:     map[string]string{envShutdownTimeout: "-5s"},
		wantErr: `PURPLEOPS_SHUTDOWN_TIMEOUT: must be a positive duration, got "-5s"`,
	}, {
		name: "db path is taken verbatim",
		env:  map[string]string{envDBPath: "/var/lib/purpleops/db.duckdb"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Database.Path, "/var/lib/purpleops/db.duckdb"; got != want {
				t.Errorf("Database.Path = %q, want %q", got, want)
			}
		},
	}, {
		name: "evidence dir is taken verbatim",
		env:  map[string]string{envEvidenceDir: "/srv/evidence"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Evidence.Dir, "/srv/evidence"; got != want {
				t.Errorf("Evidence.Dir = %q, want %q", got, want)
			}
		},
	}, {
		name:    "session secret is required",
		env:     map[string]string{envSessionSecret: ""},
		wantErr: `PURPLEOPS_SESSION_SECRET: must be set`,
	}, {
		name:    "session secret rejects a short value",
		env:     map[string]string{envSessionSecret: "short"},
		wantErr: `PURPLEOPS_SESSION_SECRET: must carry at least 32 bytes of secret material, this carries 5`,
	}, {
		name: "session secret rejects base64 that is long but thin",
		// 40 characters, but only 30 bytes once decoded.
		env:     map[string]string{envSessionSecret: "cmVhbGx5IG5vdCB0aGlydHktdHdvIGJ5dGVzIQ=="},
		wantErr: `PURPLEOPS_SESSION_SECRET: must carry at least 32 bytes of secret material, this carries 28`,
	}, {
		name:    "session secret rejects a placeholder",
		env:     map[string]string{envSessionSecret: "please-change-me-before-going-live-ok"},
		wantErr: `PURPLEOPS_SESSION_SECRET: is a placeholder or a guessable value (contains "change-me")`,
	}, {
		name:    "session secret rejects padding pretending to be entropy",
		env:     map[string]string{envSessionSecret: strings.Repeat("ab!", 16)},
		wantErr: `PURPLEOPS_SESSION_SECRET: is a placeholder or a guessable value (only 3 distinct characters)`,
	}, {
		name: "session secret accepts a hex value",
		env:  map[string]string{envSessionSecret: "3f7a1c9e5b2d8046af13c5e7920bd4681fa3c05d9e7b264180ac539fe62d7b41"},
	}, {
		name: "session secret accepts a long passphrase",
		env:  map[string]string{envSessionSecret: "correct horse battery staple, and then some more of it"},
	}, {
		name: "throttle thresholds and lockouts",
		env: map[string]string{
			envAccountFailures: "3",
			envAccountLockout:  "5m",
			envSourceFailures:  "100",
			envSourceLockout:   "1h",
		},
		check: func(t *testing.T, cfg Config) {
			want := Throttle{
				AccountFailures: 3,
				AccountLockout:  5 * time.Minute,
				SourceFailures:  100,
				SourceLockout:   time.Hour,
			}
			if cfg.Throttle != want {
				t.Errorf("Throttle = %+v, want %+v", cfg.Throttle, want)
			}
		},
	}, {
		// Zero would lock out the first person to mistype anything, which is not
		// a stricter policy but a broken one.
		name:    "a throttle threshold rejects zero",
		env:     map[string]string{envAccountFailures: "0"},
		wantErr: `PURPLEOPS_LOGIN_ACCOUNT_FAILURES: must be a positive whole number, got "0"`,
	}, {
		name:    "a throttle threshold rejects a fraction",
		env:     map[string]string{envSourceFailures: "2.5"},
		wantErr: `PURPLEOPS_LOGIN_SOURCE_FAILURES: must be a whole number, got "2.5"`,
	}, {
		name:    "a throttle lockout rejects a bare number",
		env:     map[string]string{envAccountLockout: "900"},
		wantErr: `PURPLEOPS_LOGIN_ACCOUNT_LOCKOUT: must be a duration with a unit`,
	}, {
		name:    "a throttle lockout rejects zero",
		env:     map[string]string{envSourceLockout: "0s"},
		wantErr: `PURPLEOPS_LOGIN_SOURCE_LOCKOUT: must be a positive duration`,
	}, {
		name: "log level and format",
		env:  map[string]string{envLogLevel: "debug", envLogFormat: "text"},
		check: func(t *testing.T, cfg Config) {
			if cfg.Log.Level != LevelDebug || cfg.Log.Format != FormatText {
				t.Errorf("Log = %+v, want debug/text", cfg.Log)
			}
		},
	}, {
		name:    "log level rejects an unknown level",
		env:     map[string]string{envLogLevel: "verbose"},
		wantErr: `PURPLEOPS_LOG_LEVEL: must be one of "debug", "info", "warn", "error", got "verbose"`,
	}, {
		name:    "log format rejects an unknown format",
		env:     map[string]string{envLogFormat: "logfmt"},
		wantErr: `PURPLEOPS_LOG_FORMAT: must be one of "json", "text", got "logfmt"`,
	}, {
		name: "chrome path accepts an executable",
		env:  map[string]string{envChromePath: executableFile(t)},
		check: func(t *testing.T, cfg Config) {
			if cfg.Report.ChromePath == "" {
				t.Error("Report.ChromePath is empty, want the path from the environment")
			}
		},
	}, {
		name:    "chrome path rejects a missing file",
		env:     map[string]string{envChromePath: "/nonexistent/chromium"},
		wantErr: `PURPLEOPS_CHROME_PATH: must be an executable file, but nothing exists at that path, got "/nonexistent/chromium"`,
	}, {
		name:    "chrome path rejects a file that is not executable",
		env:     map[string]string{envChromePath: regularFile(t)},
		wantErr: `PURPLEOPS_CHROME_PATH: must be executable`,
	}, {
		name: "values are trimmed",
		env:  map[string]string{envLogLevel: "  warn  "},
		check: func(t *testing.T, cfg Config) {
			if cfg.Log.Level != LevelWarn {
				t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, LevelWarn)
			}
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, errs := parse(envWith(tt.env))

			if tt.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("parse() = %v, want no errors", errs)
				}
				if tt.check != nil {
					tt.check(t, cfg)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("parse() succeeded, want error containing %q", tt.wantErr)
			}
			got := (&LoadError{Errs: errs}).Error()
			if !strings.Contains(got, tt.wantErr) {
				t.Errorf("error was\n\t%s\nwant it to contain\n\t%s", got, tt.wantErr)
			}
		})
	}
}

func TestEveryMissingRequiredVariableIsReported(t *testing.T) {
	_, errs := parse(map[string]string{})
	if len(errs) == 0 {
		t.Fatal("parse(nothing) succeeded, want errors")
	}
	got := (&LoadError{Errs: errs}).Error()

	for _, want := range []string{envBaseURL + ": must be set", envSessionSecret + ": must be set"} {
		if !strings.Contains(got, want) {
			t.Errorf("error was\n\t%s\nwant it to contain\n\t%s", got, want)
		}
	}
	// A missing required variable must not be reported as anything else.
	if strings.Contains(got, "must be an absolute URL") {
		t.Errorf("unset variable reported as a parse failure:\n\t%s", got)
	}
}

func TestLoadErrorNamesTheVariable(t *testing.T) {
	_, errs := parse(envWith(map[string]string{envBaseURL: "localhost:8080"}))

	err := error(&LoadError{Errs: errs})
	var fieldErr *FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("errors.As(%v, *FieldError) = false; the individual problems must stay inspectable", err)
	}
	if got, want := fieldErr.Error(), `PURPLEOPS_BASE_URL: must be an absolute URL, got "localhost:8080"`; got != want {
		t.Errorf("FieldError.Error() = %q, want %q", got, want)
	}
	if got := err.Error(); !strings.HasPrefix(got, "invalid configuration") {
		t.Errorf("LoadError.Error() = %q, want it to start with %q", got, "invalid configuration")
	}
}

func TestProductionRequiresHTTPS(t *testing.T) {
	tests := []struct {
		name    string
		env     Environment
		baseURL string
		wantErr bool
	}{
		{name: "production over https", env: EnvProduction, baseURL: "https://purpleops.internal", wantErr: false},
		{name: "production over http", env: EnvProduction, baseURL: "http://purpleops.internal", wantErr: true},
		{name: "production over http on localhost", env: EnvProduction, baseURL: "http://localhost:8080"},
		{name: "production over http on a loopback IP", env: EnvProduction, baseURL: "http://127.0.0.1:8080"},
		{name: "production over http on ipv6 loopback", env: EnvProduction, baseURL: "http://[::1]:8080"},
		{name: "development over http", env: EnvDevelopment, baseURL: "http://dev.internal:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := parse(envWith(map[string]string{
				envEnv:     string(tt.env),
				envBaseURL: tt.baseURL,
			}))

			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("parse() accepted %s on %s, want a startup error", tt.env, tt.baseURL)
				}
				if got := (&LoadError{Errs: errs}).Error(); !strings.Contains(got, "must use https") {
					t.Errorf("error was\n\t%s\nwant it to mention https", got)
				}
				return
			}
			if len(errs) > 0 {
				t.Fatalf("parse() = %v, want no errors", errs)
			}
		})
	}
}

func TestSecretValueNeverReachesAnError(t *testing.T) {
	const leak = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // weak, so it is rejected

	_, errs := parse(envWith(map[string]string{envSessionSecret: leak}))
	if len(errs) == 0 {
		t.Fatal("parse() accepted a weak secret")
	}
	if got := (&LoadError{Errs: errs}).Error(); strings.Contains(got, leak) {
		t.Errorf("the rejected secret appears in the error text:\n\t%s", got)
	}
}

// TestEverySecretFieldIsMarkedSensitive stops the next secret from being added
// with the flag that keeps it out of error messages left off.
func TestEverySecretFieldIsMarkedSensitive(t *testing.T) {
	var cfg Config
	for _, b := range cfg.bindings() {
		if _, isSecret := b.target.(*Secret); isSecret && !b.sensitive {
			t.Errorf("binding %s holds a Secret but is not marked sensitive", b.name)
		}
	}
}

func TestEveryVariableUsesThePrefix(t *testing.T) {
	var cfg Config
	for _, b := range cfg.bindings() {
		if !strings.HasPrefix(b.name, prefix) {
			t.Errorf("binding %s does not start with %q", b.name, prefix)
		}
	}
}

func TestLoadReadsTheProcessEnvironment(t *testing.T) {
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")

	t.Setenv(envBaseURL, "https://purpleops.internal")
	t.Setenv(envSessionSecret, testSecret)
	t.Setenv(envDBPath, filepath.Join(dir, "purpleops.duckdb"))
	t.Setenv(envEvidenceDir, evidence)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want a valid config", err)
	}
	if got, want := cfg.Server.BaseURL.String(), "https://purpleops.internal"; got != want {
		t.Errorf("BaseURL = %q, want %q", got, want)
	}

	// The evidence directory is promised to exist after a successful load.
	info, err := os.Stat(evidence)
	if err != nil {
		t.Fatalf("evidence directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", evidence)
	}
}

func TestLoadRejectsUnwritablePaths(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{{
		name:    "database parent directory does not exist",
		env:     map[string]string{envDBPath: filepath.Join(dir, "missing", "purpleops.duckdb")},
		wantErr: envDBPath + ": needs a writable parent directory",
	}, {
		name:    "evidence directory is a file",
		env:     map[string]string{envEvidenceDir: regularFile(t)},
		wantErr: envEvidenceDir + ": could not be created",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, errs := parse(envWith(tt.env))
			if len(errs) > 0 {
				t.Fatalf("parse() = %v, want the failure to come from ensurePaths", errs)
			}

			errs = cfg.ensurePaths()
			if len(errs) == 0 {
				t.Fatalf("ensurePaths() accepted %v", tt.env)
			}
			if got := (&LoadError{Errs: errs}).Error(); !strings.Contains(got, tt.wantErr) {
				t.Errorf("error was\n\t%s\nwant it to contain\n\t%s", got, tt.wantErr)
			}
		})
	}
}

// TestLoadCreatesNothingWhenTheConfigIsInvalid guards the ordering inside Load:
// a rejected configuration must not have left a directory behind.
func TestLoadCreatesNothingWhenTheConfigIsInvalid(t *testing.T) {
	evidence := filepath.Join(t.TempDir(), "evidence")

	t.Setenv(envBaseURL, "not-a-url")
	t.Setenv(envSessionSecret, testSecret)
	t.Setenv(envEvidenceDir, evidence)

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid base URL")
	}
	if _, err := os.Stat(evidence); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%s) = %v, want the directory not to have been created", evidence, err)
	}
}

// executableFile returns a path that passes the Chrome-binary check.
func executableFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "chromium")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing a fake executable: %v", err)
	}
	return path
}

// regularFile returns a path to a file that exists and is not executable.
func regularFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(path, []byte("data\n"), 0o600); err != nil {
		t.Fatalf("writing a regular file: %v", err)
	}
	return path
}
