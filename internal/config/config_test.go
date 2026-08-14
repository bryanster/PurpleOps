package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testSecret is 32 random bytes, base64 — the shape `openssl rand -base64 32`
// produces, and the shape every deployment should use.
const testSecret = "kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU="

// testKey is a second one, and is deliberately not testSecret: the loader
// refuses a deployment that uses one value for both.
const testKey = "9Qd3JmE7uZpA0xTnCiL5wHrYbVsK2fGoP4jXeM8tUcR="

// validEnv is the smallest environment that loads cleanly: the three required
// variables and nothing else, so every other assertion is about a default.
func validEnv() map[string]string {
	return map[string]string{
		envBaseURL:       "https://blacklight.internal",
		envSessionSecret: testSecret,
		envEncryptionKey: testKey,
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
	if got, want := cfg.Database.Path, "./blacklight.duckdb"; got != want {
		t.Errorf("Database.Path = %q, want %q", got, want)
	}
	if got, want := cfg.Evidence.Dir, "./evidence"; got != want {
		t.Errorf("Evidence.Dir = %q, want %q", got, want)
	}
	if got, want := cfg.Content.Dir, "./content"; got != want {
		t.Errorf("Content.Dir = %q, want %q", got, want)
	}
	if got, want := cfg.Content.MaxBytes.Int64(), int64(512<<20); got != want {
		t.Errorf("Content.MaxBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Content.JobTimeout, 30*time.Minute; got != want {
		t.Errorf("Content.JobTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.Content.WriteBatch, 250; got != want {
		t.Errorf("Content.WriteBatch = %d, want %d", got, want)
	}
	if got, want := cfg.Content.NoteMaxBytes.Int64(), int64(256<<10); got != want {
		t.Errorf("Content.NoteMaxBytes = %d, want %d", got, want)
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
	if cfg.Encryption.Key.IsZero() {
		t.Error("Encryption.Key is zero, want the value from the environment")
	}
	if got, want := cfg.MFA.PendingTTL, 5*time.Minute; got != want {
		t.Errorf("MFA.PendingTTL = %v, want %v", got, want)
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
		wantErr: `BLACKLIGHT_ENV: must be one of "development", "production", got "staging"`,
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
		wantErr: `BLACKLIGHT_ADDR: must be a host:port listen address, such as ":8080" or "127.0.0.1:8080", got "8080"`,
	}, {
		name:    "addr rejects a port out of range",
		env:     map[string]string{envAddr: ":70000"},
		wantErr: `BLACKLIGHT_ADDR: must have a port between 0 and 65535, got ":70000"`,
	}, {
		name:    "addr rejects a non-numeric port",
		env:     map[string]string{envAddr: ":http"},
		wantErr: `BLACKLIGHT_ADDR: must have a numeric port, got ":http"`,
	}, {
		name: "base url keeps a subpath",
		env:  map[string]string{envBaseURL: "https://example.internal/blacklight"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.BaseURL.String(), "https://example.internal/blacklight"; got != want {
				t.Errorf("BaseURL = %q, want %q", got, want)
			}
		},
	}, {
		name: "base url loses its trailing slash",
		env:  map[string]string{envBaseURL: "https://example.internal/blacklight/"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Server.BaseURL.String(), "https://example.internal/blacklight"; got != want {
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
		wantErr: `BLACKLIGHT_BASE_URL: must be an absolute URL, got "localhost:8080"`,
	}, {
		name:    "base url rejects a path-only value",
		env:     map[string]string{envBaseURL: "/blacklight"},
		wantErr: `BLACKLIGHT_BASE_URL: must be an absolute URL, got "/blacklight"`,
	}, {
		name:    "base url rejects a non-http scheme",
		env:     map[string]string{envBaseURL: "ftp://example.internal"},
		wantErr: `BLACKLIGHT_BASE_URL: must use scheme http or https, got "ftp://example.internal"`,
	}, {
		name:    "base url rejects embedded credentials",
		env:     map[string]string{envBaseURL: "https://admin:hunter2@example.internal"},
		wantErr: `BLACKLIGHT_BASE_URL: must not contain credentials`,
	}, {
		name:    "base url rejects a query string",
		env:     map[string]string{envBaseURL: "https://example.internal?tenant=1"},
		wantErr: `BLACKLIGHT_BASE_URL: must not contain a query string or fragment`,
	}, {
		name:    "base url rejects a port out of range",
		env:     map[string]string{envBaseURL: "https://example.internal:70000"},
		wantErr: `BLACKLIGHT_BASE_URL: must have a port between 1 and 65535`,
	}, {
		name:    "base url is required",
		env:     map[string]string{envBaseURL: ""},
		wantErr: `BLACKLIGHT_BASE_URL: must be set`,
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
		wantErr: `BLACKLIGHT_SHUTDOWN_TIMEOUT: must be a duration with a unit, such as "15s" or "2m", got "15"`,
	}, {
		name:    "shutdown timeout rejects zero",
		env:     map[string]string{envShutdownTimeout: "0s"},
		wantErr: `BLACKLIGHT_SHUTDOWN_TIMEOUT: must be a positive duration, got "0s"`,
	}, {
		name:    "shutdown timeout rejects a negative duration",
		env:     map[string]string{envShutdownTimeout: "-5s"},
		wantErr: `BLACKLIGHT_SHUTDOWN_TIMEOUT: must be a positive duration, got "-5s"`,
	}, {
		name: "db path is taken verbatim",
		env:  map[string]string{envDBPath: "/var/lib/blacklight/db.duckdb"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Database.Path, "/var/lib/blacklight/db.duckdb"; got != want {
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
		name: "content dir is taken verbatim",
		env:  map[string]string{envContentDir: "/srv/content"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Content.Dir, "/srv/content"; got != want {
				t.Errorf("Content.Dir = %q, want %q", got, want)
			}
		},
	}, {
		name:    "session secret is required",
		env:     map[string]string{envSessionSecret: ""},
		wantErr: `BLACKLIGHT_SESSION_SECRET: must be set`,
	}, {
		name:    "session secret rejects a short value",
		env:     map[string]string{envSessionSecret: "short"},
		wantErr: `BLACKLIGHT_SESSION_SECRET: must carry at least 32 bytes of secret material, this carries 5`,
	}, {
		name: "session secret rejects base64 that is long but thin",
		// 40 characters, but only 30 bytes once decoded.
		env:     map[string]string{envSessionSecret: "cmVhbGx5IG5vdCB0aGlydHktdHdvIGJ5dGVzIQ=="},
		wantErr: `BLACKLIGHT_SESSION_SECRET: must carry at least 32 bytes of secret material, this carries 28`,
	}, {
		name:    "session secret rejects a placeholder",
		env:     map[string]string{envSessionSecret: "please-change-me-before-going-live-ok"},
		wantErr: `BLACKLIGHT_SESSION_SECRET: is a placeholder or a guessable value (contains "change-me")`,
	}, {
		name:    "session secret rejects padding pretending to be entropy",
		env:     map[string]string{envSessionSecret: strings.Repeat("ab!", 16)},
		wantErr: `BLACKLIGHT_SESSION_SECRET: is a placeholder or a guessable value (only 3 distinct characters)`,
	}, {
		name: "session secret accepts a hex value",
		env:  map[string]string{envSessionSecret: "3f7a1c9e5b2d8046af13c5e7920bd4681fa3c05d9e7b264180ac539fe62d7b41"},
	}, {
		name: "session secret accepts a long passphrase",
		env:  map[string]string{envSessionSecret: "correct horse battery staple, and then some more of it"},
	}, {
		name: "the encryption key is held to the same strength as the session secret",
		env:  map[string]string{envEncryptionKey: "short"},
		wantErr: `BLACKLIGHT_ENCRYPTION_KEY: must carry at least 32 bytes of secret material, ` +
			`this carries 5`,
	}, {
		// The one check that only exists because there are two keys: sharing a
		// value re-attaches the consequence of rotating one to the other.
		name:    "the encryption key may not be the session secret",
		env:     map[string]string{envEncryptionKey: testSecret},
		wantErr: `BLACKLIGHT_ENCRYPTION_KEY: must not be the same value as BLACKLIGHT_SESSION_SECRET`,
	}, {
		name: "the pending MFA window accepts a duration",
		env:  map[string]string{envMFAPending: "90s"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.MFA.PendingTTL, 90*time.Second; got != want {
				t.Errorf("MFA.PendingTTL = %v, want %v", got, want)
			}
		},
	}, {
		name:    "the pending MFA window rejects a bare number",
		env:     map[string]string{envMFAPending: "300"},
		wantErr: `BLACKLIGHT_MFA_PENDING_TTL: must be a duration with a unit`,
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
		wantErr: `BLACKLIGHT_LOGIN_ACCOUNT_FAILURES: must be a positive whole number, got "0"`,
	}, {
		name:    "a throttle threshold rejects a fraction",
		env:     map[string]string{envSourceFailures: "2.5"},
		wantErr: `BLACKLIGHT_LOGIN_SOURCE_FAILURES: must be a whole number, got "2.5"`,
	}, {
		name:    "a throttle lockout rejects a bare number",
		env:     map[string]string{envAccountLockout: "900"},
		wantErr: `BLACKLIGHT_LOGIN_ACCOUNT_LOCKOUT: must be a duration with a unit`,
	}, {
		name:    "a throttle lockout rejects zero",
		env:     map[string]string{envSourceLockout: "0s"},
		wantErr: `BLACKLIGHT_LOGIN_SOURCE_LOCKOUT: must be a positive duration`,
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
		wantErr: `BLACKLIGHT_LOG_LEVEL: must be one of "debug", "info", "warn", "error", got "verbose"`,
	}, {
		name:    "log format rejects an unknown format",
		env:     map[string]string{envLogFormat: "logfmt"},
		wantErr: `BLACKLIGHT_LOG_FORMAT: must be one of "json", "text", got "logfmt"`,
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
		wantErr: `BLACKLIGHT_CHROME_PATH: must be an executable file, but nothing exists at that path, got "/nonexistent/chromium"`,
	}, {
		name:    "chrome path rejects a file that is not executable",
		env:     map[string]string{envChromePath: regularFile(t)},
		wantErr: `BLACKLIGHT_CHROME_PATH: must be executable`,
	}, {
		name: "oidc is off by default",
		env:  map[string]string{},
		check: func(t *testing.T, cfg Config) {
			if cfg.OIDC.Enabled() {
				t.Error("OIDC.Enabled() is true with no issuer configured")
			}
		},
	}, {
		// The one normalization the base URL performs and this must not: an
		// issuer identifier is compared byte for byte with the `iss` claim.
		name: "oidc issuer keeps its trailing slash",
		env: map[string]string{
			envOIDCIssuer:   "https://tenant.example.com/",
			envOIDCClientID: "blacklight",
		},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.OIDC.Issuer.String(), "https://tenant.example.com/"; got != want {
				t.Errorf("OIDC.Issuer = %q, want %q", got, want)
			}
			if !cfg.OIDC.Enabled() {
				t.Error("OIDC.Enabled() is false with an issuer configured")
			}
		},
	}, {
		name:    "oidc issuer rejects a value with no scheme",
		env:     map[string]string{envOIDCIssuer: "idp.example.com", envOIDCClientID: "blacklight"},
		wantErr: `BLACKLIGHT_OIDC_ISSUER: must be an absolute URL, got "idp.example.com"`,
	}, {
		name:    "oidc issuer rejects a query string",
		env:     map[string]string{envOIDCIssuer: "https://idp.example.com?realm=x", envOIDCClientID: "blacklight"},
		wantErr: `BLACKLIGHT_OIDC_ISSUER: must not contain a query string or fragment`,
	}, {
		name:    "oidc issuer must be https in production",
		env:     map[string]string{envOIDCIssuer: "http://idp.example.com", envOIDCClientID: "blacklight"},
		wantErr: `BLACKLIGHT_OIDC_ISSUER: must use https when BLACKLIGHT_ENV=production`,
	}, {
		name: "oidc issuer may be plain http on loopback",
		env: map[string]string{
			envOIDCIssuer:   "http://localhost:8081/realms/blacklight",
			envOIDCClientID: "blacklight",
		},
	}, {
		name:    "oidc needs a client id",
		env:     map[string]string{envOIDCIssuer: "https://idp.example.com"},
		wantErr: `BLACKLIGHT_OIDC_CLIENT_ID: must be set when BLACKLIGHT_OIDC_ISSUER is`,
	}, {
		// Half-configured is the case worth a startup error: it looks configured
		// and offers no single sign-on at all.
		name:    "oidc rejects a client id with no issuer",
		env:     map[string]string{envOIDCClientID: "blacklight"},
		wantErr: `BLACKLIGHT_OIDC_CLIENT_ID: is set, but BLACKLIGHT_OIDC_ISSUER is not`,
	}, {
		name:    "oidc rejects a role map with no issuer",
		env:     map[string]string{envOIDCRoleMap: "staff=member"},
		wantErr: `BLACKLIGHT_OIDC_ROLE_MAP: is set, but BLACKLIGHT_OIDC_ISSUER is not`,
	}, {
		name:    "oidc rejects auto-provisioning with no issuer",
		env:     map[string]string{envOIDCProvision: "true"},
		wantErr: `BLACKLIGHT_OIDC_AUTO_PROVISION: is set, but BLACKLIGHT_OIDC_ISSUER is not`,
	}, {
		name: "oidc scopes accept a space-separated list",
		env:  map[string]string{envOIDCScopes: "openid email"},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.OIDC.Scopes.String(), "openid email"; got != want {
				t.Errorf("OIDC.Scopes = %q, want %q", got, want)
			}
		},
	}, {
		name:    "oidc scopes must include openid",
		env:     map[string]string{envOIDCScopes: "profile email"},
		wantErr: `BLACKLIGHT_OIDC_SCOPES: must include the "openid" scope`,
	}, {
		name: "oidc role map parses group=role pairs",
		env:  map[string]string{envOIDCIssuer: "https://idp.example.com", envOIDCClientID: "x", envOIDCRoleMap: "admins=admin, staff=member"},
		check: func(t *testing.T, cfg Config) {
			role, ok := cfg.OIDC.RoleMap.Role([]string{"staff"})
			if !ok || role.Valid() != true || string(role) != "member" {
				t.Errorf("Role([staff]) = %q, %v, want the member role", role, ok)
			}
			if _, ok := cfg.OIDC.RoleMap.Role([]string{"nobody"}); ok {
				t.Error("Role([nobody]) matched, want no mapping for an unlisted group")
			}
		},
	}, {
		name:    "oidc role map rejects an unknown role",
		env:     map[string]string{envOIDCIssuer: "https://idp.example.com", envOIDCClientID: "x", envOIDCRoleMap: "admins=superuser"},
		wantErr: `BLACKLIGHT_OIDC_ROLE_MAP: maps the group "admins" onto the role "superuser", which is not one of`,
	}, {
		name:    "oidc role map rejects a pair with no role",
		env:     map[string]string{envOIDCIssuer: "https://idp.example.com", envOIDCClientID: "x", envOIDCRoleMap: "admins"},
		wantErr: `BLACKLIGHT_OIDC_ROLE_MAP: must be a comma-separated list of group=role pairs`,
	}, {
		name:    "oidc role map rejects a group listed twice",
		env:     map[string]string{envOIDCIssuer: "https://idp.example.com", envOIDCClientID: "x", envOIDCRoleMap: "admins=admin,admins=member"},
		wantErr: `BLACKLIGHT_OIDC_ROLE_MAP: maps the group "admins" twice`,
	}, {
		name: "oidc auto-provision reads a boolean",
		env:  map[string]string{envOIDCIssuer: "https://idp.example.com", envOIDCClientID: "x", envOIDCProvision: "true"},
		check: func(t *testing.T, cfg Config) {
			if !cfg.OIDC.AutoProvision {
				t.Error("OIDC.AutoProvision is false, want true")
			}
		},
	}, {
		name:    "oidc auto-provision rejects anything else",
		env:     map[string]string{envOIDCProvision: "yes please"},
		wantErr: `BLACKLIGHT_OIDC_AUTO_PROVISION: must be "true" or "false", got "yes please"`,
	}, {
		name: "no bootstrap administrator by default",
		env:  map[string]string{},
		check: func(t *testing.T, cfg Config) {
			if cfg.Bootstrap.Enabled() {
				t.Errorf("Bootstrap.Enabled() = true with no address configured (%+v)", cfg.Bootstrap)
			}
		},
	}, {
		name: "bootstrap administrator with a password file",
		env: map[string]string{
			envBootstrapEmail:        "admin@example.com",
			envBootstrapPasswordFile: "/run/secrets/bootstrap-admin-password",
		},
		check: func(t *testing.T, cfg Config) {
			if !cfg.Bootstrap.Enabled() {
				t.Error("Bootstrap.Enabled() = false, want true when an address is set")
			}
			if got, want := cfg.Bootstrap.Name, "Administrator"; got != want {
				t.Errorf("Bootstrap.Name = %q, want the default %q", got, want)
			}
			if !cfg.Bootstrap.Password.IsZero() {
				t.Error("Bootstrap.Password is set, want the file to be the only source")
			}
		},
	}, {
		name: "bootstrap administrator with a password variable",
		env: map[string]string{
			envBootstrapEmail:    "admin@example.com",
			envBootstrapName:     "Ada Lovelace",
			envBootstrapPassword: "correct horse battery staple",
		},
		check: func(t *testing.T, cfg Config) {
			if got, want := cfg.Bootstrap.Name, "Ada Lovelace"; got != want {
				t.Errorf("Bootstrap.Name = %q, want %q", got, want)
			}
			// The policy is not applied here — that happens where the account
			// is created — but the value has to arrive intact.
			if got, want := string(cfg.Bootstrap.Password.Reveal()), "correct horse battery staple"; got != want {
				t.Errorf("Bootstrap.Password = %q, want %q", got, want)
			}
		},
	}, {
		name: "bootstrap administrator needs a password",
		env:  map[string]string{envBootstrapEmail: "admin@example.com"},
		wantErr: envBootstrapPasswordFile + ": must be set when " + envBootstrapEmail +
			" is, or " + envBootstrapPassword + " must be",
	}, {
		name: "bootstrap administrator takes one password, not two",
		env: map[string]string{
			envBootstrapEmail:        "admin@example.com",
			envBootstrapPassword:     "correct horse battery staple",
			envBootstrapPasswordFile: "/run/secrets/bootstrap-admin-password",
		},
		wantErr: envBootstrapPasswordFile + ": is set and so is " + envBootstrapPassword,
	}, {
		name:    "bootstrap password without an address does nothing",
		env:     map[string]string{envBootstrapPassword: "correct horse battery staple"},
		wantErr: envBootstrapPassword + ": is set, but " + envBootstrapEmail + " is not",
	}, {
		name: "bootstrap address must be an address",
		env: map[string]string{
			envBootstrapEmail:    "admin",
			envBootstrapPassword: "correct horse battery staple",
		},
		wantErr: envBootstrapEmail + `: must be an email address, with exactly one "@"`,
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

	for _, want := range []string{
		envBaseURL + ": must be set",
		envSessionSecret + ": must be set",
		envEncryptionKey + ": must be set",
	} {
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
	if got, want := fieldErr.Error(), `BLACKLIGHT_BASE_URL: must be an absolute URL, got "localhost:8080"`; got != want {
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
		{name: "production over https", env: EnvProduction, baseURL: "https://blacklight.internal", wantErr: false},
		{name: "production over http", env: EnvProduction, baseURL: "http://blacklight.internal", wantErr: true},
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
//
// Both redacting types are checked. A ForeignSecret holds a credential this
// deployment did not choose rather than one it generated, which changes what is
// validated and changes nothing at all about where the value may appear.
func TestEverySecretFieldIsMarkedSensitive(t *testing.T) {
	var cfg Config
	for _, b := range cfg.bindings() {
		switch b.target.(type) {
		case *Secret, *ForeignSecret:
			if !b.sensitive {
				t.Errorf("binding %s holds a secret but is not marked sensitive", b.name)
			}
		}
	}
}

// TestTheClientSecretNeverLeaves is M1-009's acceptance criterion about the
// client secret, as an assertion: it may reach the provider's token endpoint and
// nowhere else. Rendering a Config is the way it would escape by accident — a
// startup log line, a debug endpoint, an error — so every ordinary rendering is
// checked here rather than trusted to the type's tests.
func TestTheClientSecretNeverLeaves(t *testing.T) {
	const secret = "s3cr3t-from-the-identity-provider"

	cfg, errs := parse(envWith(map[string]string{
		envOIDCIssuer:   "https://idp.example.com",
		envOIDCClientID: "blacklight",
		envOIDCSecret:   secret,
	}))
	if len(errs) > 0 {
		t.Fatalf("parse() = %v, want no errors", errs)
	}
	if got := string(cfg.OIDC.ClientSecret.Reveal()); got != secret {
		t.Fatalf("ClientSecret.Reveal() = %q, want the configured value", got)
	}

	for _, rendering := range []string{
		fmt.Sprintf("%v", cfg),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%#v", cfg),
		cfg.OIDC.ClientSecret.String(),
		mustMarshal(t, cfg),
		logLine(t, cfg),
	} {
		if strings.Contains(rendering, secret) {
			t.Errorf("the client secret appears in a rendering of the config:\n\t%s", rendering)
		}
	}
}

// TestAWeakClientSecretIsAccepted records the deliberate difference between the
// two secret types. The provider generated this value; refusing to start because
// it is short, or because it contains a word on the weak list, would be this
// server rejecting a credential nobody here can regenerate.
func TestAWeakClientSecretIsAccepted(t *testing.T) {
	_, errs := parse(envWith(map[string]string{
		envOIDCIssuer:   "https://idp.example.com",
		envOIDCClientID: "blacklight",
		envOIDCSecret:   "example-secret",
	}))
	if len(errs) > 0 {
		t.Errorf("parse() = %v, want a provider-issued secret to be accepted as it is", errs)
	}
}

func mustMarshal(t *testing.T, cfg Config) string {
	t.Helper()

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshalling the config: %v", err)
	}
	return string(encoded)
}

// logLine renders the config the way a startup log would, which is the rendering
// most likely to be added later by somebody who has not read this file.
func logLine(t *testing.T, cfg Config) string {
	t.Helper()

	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("configuration", slog.Any("config", cfg))
	return buf.String()
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

	t.Setenv(envBaseURL, "https://blacklight.internal")
	t.Setenv(envSessionSecret, testSecret)
	t.Setenv(envEncryptionKey, testKey)
	t.Setenv(envDBPath, filepath.Join(dir, "blacklight.duckdb"))
	t.Setenv(envEvidenceDir, evidence)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v, want a valid config", err)
	}
	if got, want := cfg.Server.BaseURL.String(), "https://blacklight.internal"; got != want {
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
		env:     map[string]string{envDBPath: filepath.Join(dir, "missing", "blacklight.duckdb")},
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
	t.Setenv(envEncryptionKey, testKey)
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
