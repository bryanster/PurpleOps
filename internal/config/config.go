package config

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// prefix is shared by every variable this package reads, so an operator can see
// at a glance which of the environment belongs to PurpleOps.
const prefix = "PURPLEOPS_"

// The variables this package reads. Each one appears exactly three times: here,
// in Config.bindings, and in .env.example — tests tie the three together.
const (
	envEnv             = prefix + "ENV"
	envAddr            = prefix + "ADDR"
	envBaseURL         = prefix + "BASE_URL"
	envShutdownTimeout = prefix + "SHUTDOWN_TIMEOUT"
	envDBPath          = prefix + "DB_PATH"
	envEvidenceDir     = prefix + "EVIDENCE_DIR"
	envSessionSecret   = prefix + "SESSION_SECRET"
	envLogLevel        = prefix + "LOG_LEVEL"
	envLogFormat       = prefix + "LOG_FORMAT"
	envChromePath      = prefix + "CHROME_PATH"
)

// Config is the whole configuration of a PurpleOps process. It is grouped by
// the thing being configured rather than flattened, so a constructor can take
// the section it needs (cfg.Database) instead of the world.
type Config struct {
	// Env is the deployment posture. Only EnvDevelopment relaxes a security
	// control; see the individual controls for what each relaxation is.
	Env Environment

	Server   Server
	Database Database
	Evidence Evidence
	Session  Session
	Log      Log
	Report   Report
}

// Server is the HTTP listener and the public identity of this deployment.
type Server struct {
	// Addr is the listen address as host:port. An empty host means every
	// interface; port 0 means "ask the kernel", which is what tests use.
	Addr string

	// BaseURL is the URL clients reach this deployment on. It cannot be
	// derived from an incoming request without trusting a proxy header, and
	// OIDC redirect URIs and report share links must be stable, so it is
	// configured rather than guessed.
	BaseURL URL

	// ShutdownTimeout is how long in-flight requests get to finish after a
	// termination signal before the process exits anyway.
	ShutdownTimeout time.Duration
}

// Database locates the embedded DuckDB database.
type Database struct {
	// Path is the database file. Its parent directory must exist and be
	// writable; DuckDB creates the file itself, plus a WAL beside it.
	Path string
}

// Evidence locates the on-disk blob store.
type Evidence struct {
	// Dir holds uploaded evidence. It is created at startup if absent.
	Dir string
}

// Session holds the secrets behind cookie sessions.
type Session struct {
	// Secret signs session cookies. Rotating it invalidates every session.
	Secret Secret
}

// Log configures the process logger.
type Log struct {
	Level  LogLevel
	Format LogFormat
}

// Report configures report rendering (M6).
type Report struct {
	// ChromePath is the Chromium/Chrome binary used to render PDFs. Empty
	// means PDF rendering is unavailable; the rest of the server runs.
	ChromePath string
}

// binding is one environment variable and the field it fills.
type binding struct {
	name     string
	target   any    // pointer to the field; see binding.set for the types allowed
	def      string // the default, parsed through the same path as a real value
	required bool
	// sensitive keeps the value out of the error message when parsing fails.
	// Only secrets set it — a redacted error is harder to act on, so it is not
	// the default.
	sensitive bool
}

// bindings is the single source of truth for what this package reads: every
// name, default and requirement is here, and TestEveryConfigFieldIsBound fails
// if a Config field is missing from the list.
func (c *Config) bindings() []binding {
	return []binding{
		{name: envEnv, target: &c.Env, def: string(EnvProduction)},
		{name: envAddr, target: &c.Server.Addr, def: ":8080"},
		{name: envBaseURL, target: &c.Server.BaseURL, required: true},
		{name: envShutdownTimeout, target: &c.Server.ShutdownTimeout, def: "15s"},
		{name: envDBPath, target: &c.Database.Path, def: "./purpleops.duckdb"},
		{name: envEvidenceDir, target: &c.Evidence.Dir, def: "./evidence"},
		{name: envSessionSecret, target: &c.Session.Secret, required: true, sensitive: true},
		{name: envLogLevel, target: &c.Log.Level, def: string(LevelInfo)},
		{name: envLogFormat, target: &c.Log.Format, def: string(FormatJSON)},
		{name: envChromePath, target: &c.Report.ChromePath},
	}
}

// Load reads, parses and validates the configuration from the process
// environment, and creates the directories it promises exist.
//
// It reports every problem it found, not the first: fixing a deployment one
// restart per mistake is what a startup check is supposed to prevent. A
// non-nil error is always a [LoadError].
func Load() (Config, error) {
	cfg, errs := parse(environ())
	if len(errs) > 0 {
		// Nothing has been created yet, so a rejected configuration leaves no
		// directories behind.
		return Config{}, &LoadError{Errs: errs}
	}
	if errs := cfg.ensurePaths(); len(errs) > 0 {
		return Config{}, &LoadError{Errs: errs}
	}
	return cfg, nil
}

// environ snapshots the process environment. It is the only place in the tree
// that touches it; TestOnlyConfigReadsTheEnvironment keeps it that way.
func environ() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	return env
}

// parse fills a Config from env and checks everything that can be checked
// without touching the filesystem. It has no side effects, so a caller that is
// going to reject the result has not already changed the machine.
func parse(env map[string]string) (Config, []error) {
	var cfg Config
	var errs []error

	for _, b := range cfg.bindings() {
		raw, ok := lookup(env, b.name)
		if !ok {
			if b.required {
				errs = append(errs, &FieldError{Name: b.name, Msg: "must be set"})
				continue
			}
			if b.def == "" {
				continue // optional and unset: the zero value is the answer
			}
			raw = b.def
		}
		if err := b.set(raw); err != nil {
			fe := &FieldError{Name: b.name, Msg: err.Error()}
			if !b.sensitive {
				fe.Value = raw
			}
			errs = append(errs, fe)
		}
	}

	return cfg, append(errs, cfg.validate()...)
}

// lookup returns the value of name, treating set-but-empty as unset. An empty
// string is what a compose file produces for a variable nobody filled in, and
// it is never a valid value for anything here. Surrounding whitespace is
// trimmed: a trailing space in a .env file is a typo, not key material.
func lookup(env map[string]string, name string) (string, bool) {
	v, ok := env[name]
	if !ok {
		return "", false
	}
	if v = strings.TrimSpace(v); v == "" {
		return "", false
	}
	return v, true
}

// set parses raw into the bound field. The error it returns is the requirement
// alone ("must be an absolute URL"); the caller adds the variable name and the
// offending value.
func (b binding) set(raw string) error {
	switch target := b.target.(type) {
	case *string:
		*target = raw
	case *time.Duration:
		d, err := time.ParseDuration(raw)
		if err != nil {
			return errors.New(`must be a duration with a unit, such as "15s" or "2m"`)
		}
		if d <= 0 {
			return errors.New("must be a positive duration")
		}
		*target = d
	case encoding.TextUnmarshaler:
		return target.UnmarshalText([]byte(raw))
	default:
		// Unreachable unless a field is added with a type nothing can parse,
		// which TestEveryConfigFieldIsBound catches first.
		return fmt.Errorf("internal: no parser for %T", b.target)
	}
	return nil
}
