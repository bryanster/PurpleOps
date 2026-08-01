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
	envRequestTimeout  = prefix + "REQUEST_TIMEOUT"
	envShutdownTimeout = prefix + "SHUTDOWN_TIMEOUT"
	envTrustedProxies  = prefix + "TRUSTED_PROXIES"
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

	// RequestTimeout is the deadline put on the context of every request. A
	// handler that respects its context gives up at that point and the caller
	// gets an error rather than a connection that never answers.
	RequestTimeout time.Duration

	// ShutdownTimeout is how long in-flight requests get to finish after a
	// termination signal before the process exits anyway.
	ShutdownTimeout time.Duration

	// TrustedProxies are the peers whose forwarded-for headers this server
	// believes. Empty — the default — means the client address is always the
	// address the connection came from, so nobody can change what is throttled
	// (M1-004) or logged by sending a header.
	TrustedProxies CIDRs
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

// Tool is the configuration of an administrative process — popsctl, which
// shares this repository's packages with the server but serves no HTTP and
// holds no sessions.
//
// It is a separate type rather than a half-filled [Config] because this package
// promises never to hand back a partially valid configuration: every field of a
// Tool is one [LoadTool] actually read and validated.
type Tool struct {
	Database Database
	Log      Log
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
	// tool marks a variable the admin CLI reads as well as the server, and so
	// the fields [Tool] carries. It is a small list on purpose: requiring a
	// base URL and a session secret before `popsctl db info` will open a file
	// is a checklist with nothing behind it, and an operator who hits it once
	// learns to export junk values, which is worse than not asking.
	tool bool
}

// bindings is the single source of truth for what this package reads: every
// name, default and requirement is here, and TestEveryConfigFieldIsBound fails
// if a Config field is missing from the list.
func (c *Config) bindings() []binding {
	return []binding{
		{name: envEnv, target: &c.Env, def: string(EnvProduction)},
		{name: envAddr, target: &c.Server.Addr, def: ":8080"},
		{name: envBaseURL, target: &c.Server.BaseURL, required: true},
		{name: envRequestTimeout, target: &c.Server.RequestTimeout, def: "30s"},
		{name: envShutdownTimeout, target: &c.Server.ShutdownTimeout, def: "15s"},
		{name: envTrustedProxies, target: &c.Server.TrustedProxies},
		{name: envDBPath, target: &c.Database.Path, def: "./purpleops.duckdb", tool: true},
		{name: envEvidenceDir, target: &c.Evidence.Dir, def: "./evidence"},
		{name: envSessionSecret, target: &c.Session.Secret, required: true, sensitive: true},
		{name: envLogLevel, target: &c.Log.Level, def: string(LevelInfo), tool: true},
		{name: envLogFormat, target: &c.Log.Format, def: string(FormatJSON), tool: true},
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

// LoadTool reads and validates the settings an administrative process shares
// with the server — the database it opens and the log it writes — from the same
// environment, parsed by the same code. A non-nil error is always a [LoadError].
//
// It deliberately does less than [Load]. It ignores the variables only a server
// uses, so a CLI invocation is not held up by a missing base URL or session
// secret, and it creates nothing: an admin command that made a directory as a
// side effect of starting up would be a surprising way to typo a path.
func LoadTool() (Tool, error) {
	cfg, errs := parseTool(environ())
	if len(errs) > 0 {
		return Tool{}, &LoadError{Errs: errs}
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
	cfg, errs := bind(env, func(binding) bool { return true })
	return cfg, append(errs, cfg.validate()...)
}

// parseTool is parse for an administrative process: the variables marked
// binding.tool, and only those.
//
// It runs no cross-field validation, because every check in validate() is about
// a variable a tool does not read — a listen address, a base URL, a browser to
// render PDFs with. Adding a tool-visible check means calling it from here too.
func parseTool(env map[string]string) (Tool, []error) {
	cfg, errs := bind(env, func(b binding) bool { return b.tool })
	return Tool{Database: cfg.Database, Log: cfg.Log}, errs
}

// bind fills the fields whose bindings want accepts, and reports every problem
// it found with them. A binding want rejects is not read, so an invalid value
// in a variable this process does not use cannot stop it starting.
func bind(env map[string]string, want func(binding) bool) (Config, []error) {
	var cfg Config
	var errs []error

	for _, b := range cfg.bindings() {
		if !want(b) {
			continue
		}
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

	return cfg, errs
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
