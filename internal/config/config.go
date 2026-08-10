package config

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// prefix is shared by every variable this package reads, so an operator can see
// at a glance which of the environment belongs to Blacklight.
const prefix = "BLACKLIGHT_"

// The variables this package reads. Each one appears exactly three times: here,
// in Config.bindings, and in .env.example — tests tie the three together.
const (
	envEnv                   = prefix + "ENV"
	envAddr                  = prefix + "ADDR"
	envBaseURL               = prefix + "BASE_URL"
	envRequestTimeout        = prefix + "REQUEST_TIMEOUT"
	envShutdownTimeout       = prefix + "SHUTDOWN_TIMEOUT"
	envTrustedProxies        = prefix + "TRUSTED_PROXIES"
	envDBPath                = prefix + "DB_PATH"
	envEvidenceMaxUpload     = prefix + "EVIDENCE_MAX_UPLOAD_BYTES"
	envEvidenceMaxEngagement = prefix + "EVIDENCE_MAX_ENGAGEMENT_BYTES"
	envEvidenceMIMEAllowlist = prefix + "EVIDENCE_MIME_ALLOWLIST"
	envEvidenceDir           = prefix + "EVIDENCE_DIR"
	envContentDir            = prefix + "CONTENT_DIR"
	envContentMaxBytes       = prefix + "CONTENT_MAX_BYTES"
	envContentJobTimeout     = prefix + "CONTENT_JOB_TIMEOUT"
	envContentWriteBatch     = prefix + "CONTENT_WRITE_BATCH"
	envContentNoteMaxBytes   = prefix + "CONTENT_NOTE_MAX_BYTES"

	envEventsMaxSubscribers = prefix + "EVENTS_MAX_SUBSCRIBERS"
	envEventsBuffer         = prefix + "EVENTS_BUFFER"
	envEventsHeartbeat      = prefix + "EVENTS_HEARTBEAT"
	envEventsMaxReplay      = prefix + "EVENTS_MAX_REPLAY"

	envSessionSecret   = prefix + "SESSION_SECRET"
	envEncryptionKey   = prefix + "ENCRYPTION_KEY"
	envSessionLifetime = prefix + "SESSION_LIFETIME"
	envSessionIdle     = prefix + "SESSION_IDLE_TIMEOUT"
	envMFAPending      = prefix + "MFA_PENDING_TTL"
	envOIDCIssuer      = prefix + "OIDC_ISSUER"
	envOIDCClientID    = prefix + "OIDC_CLIENT_ID"
	envOIDCSecret      = prefix + "OIDC_CLIENT_SECRET"
	envOIDCScopes      = prefix + "OIDC_SCOPES"
	envOIDCGroupsClaim = prefix + "OIDC_GROUPS_CLAIM"
	envOIDCRoleMap     = prefix + "OIDC_ROLE_MAP"
	envOIDCProvision   = prefix + "OIDC_AUTO_PROVISION"
	envSAMLMetaURL     = prefix + "SAML_IDP_METADATA_URL"
	envSAMLMetaFile    = prefix + "SAML_IDP_METADATA_FILE"
	envSAMLEntityID    = prefix + "SAML_ENTITY_ID"
	envSAMLCertFile    = prefix + "SAML_CERT_FILE"
	envSAMLKeyFile     = prefix + "SAML_KEY_FILE"
	envSAMLEmailAttr   = prefix + "SAML_EMAIL_ATTRIBUTE"
	envSAMLNameAttr    = prefix + "SAML_NAME_ATTRIBUTE"
	envSAMLGroupsAttr  = prefix + "SAML_GROUPS_ATTRIBUTE"
	envSAMLRoleMap     = prefix + "SAML_ROLE_MAP"
	envSAMLProvision   = prefix + "SAML_AUTO_PROVISION"
	envSAMLIDPInit     = prefix + "SAML_ALLOW_IDP_INITIATED"
	envSAMLClockSkew   = prefix + "SAML_CLOCK_SKEW"
	envAccountFailures = prefix + "LOGIN_ACCOUNT_FAILURES"
	envAccountLockout  = prefix + "LOGIN_ACCOUNT_LOCKOUT"
	envSourceFailures  = prefix + "LOGIN_SOURCE_FAILURES"
	envSourceLockout   = prefix + "LOGIN_SOURCE_LOCKOUT"
	envLogLevel        = prefix + "LOG_LEVEL"
	envLogFormat       = prefix + "LOG_FORMAT"
	envChromePath      = prefix + "CHROME_PATH"
	envBrandingDir = prefix + "BRANDING_DIR"
)

// Config is the whole configuration of a Blacklight process. It is grouped by
// the thing being configured rather than flattened, so a constructor can take
// the section it needs (cfg.Database) instead of the world.
type Config struct {
	// Env is the deployment posture. Only EnvDevelopment relaxes a security
	// control; see the individual controls for what each relaxation is.
	Env Environment

	Server   Server
	Database Database
	Evidence Evidence
	Content  Content
	Events   Events

	Session    Session
	Encryption Encryption
	MFA        MFA
	OIDC       OIDC
	SAML       SAML
	Throttle   Throttle
	Log        Log
	Report     Report
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

// Evidence locates the on-disk blob store and sets the upload policy.
type Evidence struct {
	// Dir holds uploaded evidence. It is created at startup if absent.
	Dir string

	// MaxUploadBytes is the largest single file accepted on upload. Post body
	// is streamed to a temp file before this is checked; the gate is the
	// configured bytes plus one — "read one more byte and reject past limit"
	// is the contract, and every streaming reader in this codebase upholds it.
	MaxUploadBytes ByteSize

	// MaxEngagementBytes is the soft cap on total evidence bytes linked to one
	// engagement, counting unique blobs once. Exceeding it returns 413.
	MaxEngagementBytes ByteSize

	// MIMEAllowlist is a comma-separated list of accepted Content-Types for
	// uploads. Types matching the list pass; everything else is rejected with
	// a problem detail naming the allowed types.
	MIMEAllowlist string
}

// Content locates on-disk raw upstream snapshots and offline bundles (M2),
// and holds the knobs the content job runner (M2-003) reads.
type Content struct {
	// Dir is the root for content artifacts. Raw snapshots are stored under
	// Dir/raw/{source_id}/{version}/{sha256}. Repositories reject any stored
	// path that would escape this root.
	Dir string

	// MaxBytes is the largest upstream download Fetch may accept. Oversized
	// payloads fail the job with an error that names this limit.
	MaxBytes ByteSize

	// JobTimeout is the wall-clock budget for one content job. On expiry the
	// job is marked failed and the adapter context is cancelled.
	JobTimeout time.Duration

	// WriteBatch is how many normalized objects Apply writes per store.Write
	// transaction. Default 250: M2-016 loadtest (20k fixture notes, local SSD)
	// kept interactive session-touch p95 under 200ms at this size; larger
	// batches risk starving interactive writers on the serialized lock.
	WriteBatch int

	// NoteMaxBytes is the largest markdown body a custom knowledge-base note
	// may carry (M2-011). Oversized bodies are refused with a field error.
	NoteMaxBytes ByteSize
}

// Events bounds the shared SSE hub (M2-004). The hub is pure fan-out; these
// knobs keep memory and idle proxies under control.
type Events struct {
	// MaxSubscribers caps concurrent live SSE subscriptions installation-wide.
	MaxSubscribers int

	// Buffer is the per-subscriber event channel capacity. A full buffer drops
	// that subscriber rather than blocking publishers.
	Buffer int

	// Heartbeat is how often the SSE handler writes a comment frame so idle
	// proxies do not close the connection. Must be well under typical proxy
	// MaxReplayEvents caps how many activity rows the SSE handler replays on
	// a reconnect with Last-Event-ID before sending stream.gap. Default 500;
	// 0 disables replay (only live tail).
	MaxReplayEvents int
	Heartbeat       time.Duration
}

// Session holds the secrets and the timings behind cookie sessions.
type Session struct {
	// Secret keys the hash a session token is stored under. Rotating it
	// invalidates every session, because no stored hash can be reproduced from
	// the cookies people are holding.
	Secret Secret

	// Lifetime is how long a session may live at most, counted from when it was
	// issued. Reaching it ends the session whatever the person was in the middle
	// of; that is the point of an absolute expiry, and no amount of activity
	// extends it.
	Lifetime time.Duration

	// IdleTimeout is how long a session may go unused before it ends. It is the
	// one that protects an unattended browser, and it is necessarily shorter
	// than Lifetime.
	IdleTimeout time.Duration
}

// Encryption holds the key that protects the secrets this server stores on
// behalf of somebody else — today the TOTP shared secrets of M1-006, later the
// OIDC and SMTP credentials an operator hands over.
//
// It is deliberately *not* [Session.Secret]. Rotating the session secret is the
// documented way to sign everybody out, and if these secrets were derived from
// it that lever would also, silently, destroy every enrolled authenticator: the
// database would still hold ciphertext that no key could open, and the only
// symptom would be everyone's codes being wrong at once. Two keys, two blast
// radii — see docs/security.md.
type Encryption struct {
	// Key is the input to the key derivation behind
	// internal/authn/secrets. Losing it means the TOTP enrolments it protects
	// cannot be read; changing it means the same. Neither is recoverable, which
	// is why it has no default and is never generated per process.
	Key Secret
}

// MFA is the timing of a second-factor challenge.
type MFA struct {
	// PendingTTL is how long the pending state between a correct password and a
	// presented second factor lasts. It is short by design: it is not a session,
	// it authorizes nothing except the verification endpoint, and a person who
	// has their authenticator in their hand does not need five minutes.
	PendingTTL time.Duration
}

// OIDC configures single sign-on against an OpenID Connect provider (M1-009).
//
// The whole section is optional, and Issuer is the switch: with no issuer there
// is no OIDC, the start endpoint answers 404 and `GET /auth/providers` offers
// local login alone. That is deliberate and it is what stops a broken identity
// provider from being an outage — v1's SSO could not be turned off without a
// redeploy, and PLAN.md §4 asks for a login page that still works when the
// provider does not.
//
// Everything here is read once at startup. Rotating a client secret is a
// restart, which is the same as every other secret this process holds.
type OIDC struct {
	// Issuer is the provider's issuer identifier — the URL its discovery
	// document lives under, and the exact string every ID token's `iss` claim
	// must equal. Discovery is `<issuer>/.well-known/openid-configuration`;
	// nothing here is hand-configured per provider, which is the point of using
	// discovery at all.
	Issuer IssuerURL

	// ClientID is this deployment's registration at the provider. It is not a
	// secret — it travels in the authorization URL a browser follows — and it is
	// what an ID token's `aud` claim must contain.
	ClientID string

	// ClientSecret authenticates the token exchange, which happens server to
	// server. It never reaches a browser, a log or a response; see
	// [ForeignSecret], and TestTheClientSecretNeverLeaves.
	//
	// Empty means a public client: the authorization code is bound to this
	// deployment by PKCE alone. That is a supported configuration — some
	// providers issue no secret for a confidential client — and PKCE is required
	// either way, so nothing weakens by leaving it out.
	ClientSecret ForeignSecret

	// Scopes are what the authorization request asks for. The default asks for
	// the profile, the address and the groups, because role mapping needs the
	// last of those and there is no second round trip to fetch them later.
	Scopes Scopes

	// GroupsClaim is where in the ID token the groups are. Providers disagree:
	// Keycloak and Authentik call it `groups`, Entra calls it `roles` unless you
	// configure a groups claim, Okta calls it whatever the claim you added is
	// called.
	GroupsClaim string

	// RoleMap turns those groups into a platform role, re-evaluated at every
	// login. Empty means every federated user is an ordinary member.
	//
	// A mapping that produces no administrators is allowed. It is a mapping, not
	// a safety mechanism: the local administrator account is what a deployment
	// falls back on, and refusing this configuration would only teach an
	// operator to write a mapping they did not mean.
	RoleMap RoleMap

	// AutoProvision creates an account the first time somebody the provider
	// vouches for arrives. Off by default: on a deployment whose provider hosts
	// the whole company, on means the whole company has an account here.
	AutoProvision bool
}

// Enabled reports whether this deployment offers single sign-on at all.
func (o OIDC) Enabled() bool { return !o.Issuer.IsZero() }

// SAML configures single sign-on against a SAML 2.0 identity provider
// (M1-010). Nobody chooses SAML in 2026; enterprises still require it, which is
// the whole argument for this section existing beside [OIDC].
//
// It is optional in exactly the way OIDC is, and the identity provider's
// metadata is the switch: with neither [SAML.MetadataURL] nor
// [SAML.MetadataFile] there is no SAML, the endpoints answer 404 and
// `GET /auth/providers` does not draw the button. A deployment may configure
// both protocols, one, or neither, and none of the three affects local login.
//
// Read once at startup, including the two files: rotating the service provider
// key is a restart, and so is a change of the identity provider's certificate
// when the metadata is a file rather than a URL.
type SAML struct {
	// MetadataURL is where the identity provider publishes its metadata, and
	// MetadataFile is that same document saved to disk. Exactly one of them is
	// set; [Config.validate] refuses both and refuses a half-configured
	// section.
	//
	// The URL is the better of the two — it is how a rotated signing
	// certificate reaches this deployment without anybody editing a file — and
	// it is also the one that puts this deployment's ability to offer single
	// sign-on at the mercy of somebody else's web server. Both are supported
	// because both are what real identity providers hand out: some publish a
	// URL, and some give you an XML document in a browser download and nothing
	// else.
	MetadataURL  URL
	MetadataFile string

	// EntityID is what this deployment calls itself to the identity provider,
	// and is the value every assertion's `Audience` must carry. Empty means the
	// metadata URL of this deployment, which is the conventional default and
	// the one the metadata document advertises.
	EntityID string

	// CertFile and KeyFile are the PEM files holding this service provider's
	// certificate and its private key. The certificate is published in the
	// metadata and used by the identity provider to check the signature on an
	// authentication request; the key signs those requests and never leaves
	// this process — it is not logged, not served, and not in the environment,
	// which is the reason these are paths rather than pasted PEM.
	CertFile string
	KeyFile  string

	// EmailAttribute, NameAttribute and GroupsAttribute are the assertion
	// attributes to read each fact out of, best first. They are lists because
	// no two identity providers agree on a spelling: the same address is `mail`
	// at one, an OID at the next, and a schemas.xmlsoap.org URL at a third, and
	// an operator should not have to discover which before their first login
	// works. Both the `Name` and the `FriendlyName` of an attribute are
	// matched.
	EmailAttribute  Names
	NameAttribute   Names
	GroupsAttribute Names

	// RoleMap turns the groups in an assertion into a platform role, on every
	// sign-in, exactly as [OIDC.RoleMap] does — it is the same type, evaluated
	// by the same code.
	RoleMap RoleMap

	// AutoProvision creates an account the first time somebody the provider
	// vouches for arrives. Off by default, for the reason [OIDC.AutoProvision]
	// gives.
	AutoProvision bool

	// AllowIDPInitiated accepts an assertion that answers no authentication
	// request from here — somebody clicking a Blacklight tile in their
	// provider's application portal, which is how a great many enterprises
	// expect SAML to work.
	//
	// On by default, and it is a real tradeoff rather than an oversight. An
	// SP-initiated sign-in is bound to the browser that started it: the sealed
	// cookie names a request ID, and the assertion has to answer that ID, so an
	// assertion captured or minted for somebody else cannot be delivered into
	// your browser to sign you in as them. An IdP-initiated sign-in has no
	// request to answer, so it has no such binding — that is inherent in the
	// profile and not something an implementation can fix. Turn it off on a
	// deployment nobody reaches from a portal.
	AllowIDPInitiated bool

	// ClockSkew is how far the identity provider's clock may be from this
	// server's before an assertion inside its stated validity window is refused
	// for being outside it. Bounded rather than open-ended: it widens every
	// window in the profile, so a generous value here is a replay window that
	// stays open after the assertion was supposed to have expired.
	ClockSkew time.Duration
}

// Enabled reports whether this deployment offers SAML at all: the identity
// provider's metadata is the switch, in either of its two spellings.
func (s SAML) Enabled() bool { return !s.MetadataURL.IsZero() || s.MetadataFile != "" }

// Throttle is how many failed sign-in attempts the server answers before it
// starts refusing them (M1-004). Two independent limits, both enforced.
type Throttle struct {
	// AccountFailures is how many consecutive failures against one email
	// address close it, and AccountLockout is how long the first closure lasts.
	// Each further closure of the same account doubles the wait, three times.
	AccountFailures int
	AccountLockout  time.Duration

	// SourceFailures and SourceLockout are the same two things keyed on the
	// client address instead. The threshold is much higher: this limit is not
	// about one password being guessed, which the account limit sees, but about
	// one host trying one password against every account, which it does not.
	SourceFailures int
	SourceLockout  time.Duration
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

	// BrandingDir is the directory for content-addressed branding logo files.
	// Created at startup if absent. Defaults to ./branding.
	BrandingDir string
}

// Tool is the configuration of an administrative process — blctl, which
// shares this repository's packages with the server but serves no HTTP and
// holds no sessions.
//
// It is a separate type rather than a half-filled [Config] because this package
// promises never to hand back a partially valid configuration: every field of a
// Tool is one [LoadTool] actually read and validated.
type Tool struct {
	Database Database
	Log      Log
	// Content is the on-disk content root and runner knobs. blctl content sync
	// writes raw snapshots here the same way the server does.
	Content  Content
	Evidence Evidence
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
	// base URL and a session secret before `blctl db info` will open a file
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
		{name: envDBPath, target: &c.Database.Path, def: "./blacklight.duckdb", tool: true},
		{name: envEvidenceDir, target: &c.Evidence.Dir, def: "./evidence", tool: true},
		{name: envEvidenceMaxUpload, target: &c.Evidence.MaxUploadBytes, def: "25MiB", tool: true},
		{name: envEvidenceMaxEngagement, target: &c.Evidence.MaxEngagementBytes, def: "2GiB", tool: true},
		{name: envEvidenceMIMEAllowlist, target: &c.Evidence.MIMEAllowlist, def: "image/png,image/jpeg,image/gif,image/webp,application/pdf,text/plain,text/csv,application/json,application/zip,application/x-tar,application/gzip,application/x-7z-compressed,text/x-log", tool: true},
		{name: envContentDir, target: &c.Content.Dir, def: "./content", tool: true},
		{name: envContentMaxBytes, target: &c.Content.MaxBytes, def: "512MiB", tool: true},
		{name: envContentJobTimeout, target: &c.Content.JobTimeout, def: "30m", tool: true},
		{name: envContentWriteBatch, target: &c.Content.WriteBatch, def: "250", tool: true},
		{name: envContentNoteMaxBytes, target: &c.Content.NoteMaxBytes, def: "256KiB", tool: true},

		{name: envEventsMaxSubscribers, target: &c.Events.MaxSubscribers, def: "256"},
		{name: envEventsBuffer, target: &c.Events.Buffer, def: "16"},
		{name: envEventsHeartbeat, target: &c.Events.Heartbeat, def: "15s"},
		{name: envEventsMaxReplay, target: &c.Events.MaxReplayEvents, def: "500"},

		{name: envSessionSecret, target: &c.Session.Secret, required: true, sensitive: true},
		{name: envEncryptionKey, target: &c.Encryption.Key, required: true, sensitive: true},
		{name: envSessionLifetime, target: &c.Session.Lifetime, def: "12h"},
		{name: envSessionIdle, target: &c.Session.IdleTimeout, def: "2h"},
		{name: envMFAPending, target: &c.MFA.PendingTTL, def: "5m"},
		{name: envOIDCIssuer, target: &c.OIDC.Issuer},
		{name: envOIDCClientID, target: &c.OIDC.ClientID},
		{name: envOIDCSecret, target: &c.OIDC.ClientSecret, sensitive: true},
		{name: envOIDCScopes, target: &c.OIDC.Scopes, def: "openid profile email groups"},
		{name: envOIDCGroupsClaim, target: &c.OIDC.GroupsClaim, def: "groups"},
		{name: envOIDCRoleMap, target: &c.OIDC.RoleMap},
		{name: envOIDCProvision, target: &c.OIDC.AutoProvision, def: "false"},
		{name: envSAMLMetaURL, target: &c.SAML.MetadataURL},
		{name: envSAMLMetaFile, target: &c.SAML.MetadataFile},
		{name: envSAMLEntityID, target: &c.SAML.EntityID},
		{name: envSAMLCertFile, target: &c.SAML.CertFile},
		{name: envSAMLKeyFile, target: &c.SAML.KeyFile},
		// The defaults are a tour of how little the identity providers agree.
		// Ordered best first, and generous on purpose: the cost of an extra
		// name nobody's provider sends is nothing, and the cost of a missing one
		// is an operator debugging an empty display name on their first login.
		{name: envSAMLEmailAttr, target: &c.SAML.EmailAttribute,
			def: "email,mail,urn:oid:0.9.2342.19200300.100.1.3," +
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"},
		{name: envSAMLNameAttr, target: &c.SAML.NameAttribute,
			def: "displayName,name,cn,urn:oid:2.16.840.1.113730.3.1.241," +
				"http://schemas.microsoft.com/identity/claims/displayname," +
				"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"},
		{name: envSAMLGroupsAttr, target: &c.SAML.GroupsAttribute,
			def: "groups,memberOf,Group,http://schemas.xmlsoap.org/claims/Group," +
				"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"},
		{name: envSAMLRoleMap, target: &c.SAML.RoleMap},
		{name: envSAMLProvision, target: &c.SAML.AutoProvision, def: "false"},
		{name: envBrandingDir, target: &c.Report.BrandingDir, def: "./branding"},
		{name: envSAMLIDPInit, target: &c.SAML.AllowIDPInitiated, def: "true"},
		{name: envSAMLClockSkew, target: &c.SAML.ClockSkew, def: "2m"},
		{name: envAccountFailures, target: &c.Throttle.AccountFailures, def: "5"},
		{name: envAccountLockout, target: &c.Throttle.AccountLockout, def: "15m"},
		{name: envSourceFailures, target: &c.Throttle.SourceFailures, def: "50"},
		{name: envSourceLockout, target: &c.Throttle.SourceLockout, def: "15m"},
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
	return Tool{Database: cfg.Database, Log: cfg.Log, Content: cfg.Content, Evidence: cfg.Evidence}, errs
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
	case *bool:
		// strconv.ParseBool's vocabulary: 1/t/T/TRUE/true/True and the same for
		// false. Wider than an operator needs and narrower than "anything that is
		// not empty is true", which is the reading that turns a typo into a
		// setting nobody meant to change.
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return errors.New(`must be "true" or "false"`)
		}
		*target = v
	case *int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return errors.New("must be a whole number")
		}
		// Every count this package reads is a threshold, and none of them mean
		// anything at zero or below. A binding that wants to allow zero needs its
		// own case rather than a relaxation of this one.
		if n <= 0 {
			return errors.New("must be a positive whole number")
		}
		*target = n
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

// LoadTestEnabled reports whether the BLACKLIGHT_LOADTEST env var is set to
// a truthy value. Load tests that are too expensive for CI gate on this.
func LoadTestEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BLACKLIGHT_LOADTEST"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
