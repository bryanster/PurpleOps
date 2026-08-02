package config

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// The types in this file own the validation that can be done from the value
// alone, via encoding.TextUnmarshaler. Checks that need the filesystem or a
// second variable live in validate() and ensurePaths() instead, so that parsing
// a Config stays free of side effects.

// Environment is the deployment posture. It is the only switch that may relax a
// security control (PLAN.md §4), so it is spelled out rather than inferred from
// whether some other value happens to be set.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

var environments = []Environment{EnvDevelopment, EnvProduction}

func (e Environment) String() string { return string(e) }

// IsDevelopment reports whether relaxations meant for a developer's laptop are
// permitted. Read it as "this deployment is not protecting anything".
func (e Environment) IsDevelopment() bool { return e == EnvDevelopment }

func (e *Environment) UnmarshalText(text []byte) error { return parseEnum(text, environments, e) }

// LogLevel is the minimum severity that reaches the log.
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

var logLevels = []LogLevel{LevelDebug, LevelInfo, LevelWarn, LevelError}

func (l LogLevel) String() string { return string(l) }

func (l *LogLevel) UnmarshalText(text []byte) error { return parseEnum(text, logLevels, l) }

// Slog maps the configured level onto log/slog's scale. It lives next to the
// constants so that adding a level is a one-file change.
func (l LogLevel) Slog() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// LogFormat selects the log encoding.
type LogFormat string

const (
	FormatJSON LogFormat = "json"
	FormatText LogFormat = "text"
)

var logFormats = []LogFormat{FormatJSON, FormatText}

func (f LogFormat) String() string { return string(f) }

func (f *LogFormat) UnmarshalText(text []byte) error { return parseEnum(text, logFormats, f) }

// parseEnum accepts any of allowed, case-insensitively, and otherwise returns
// an error naming every accepted value: an operator who mistyped an enum should
// not have to go and find the documentation.
func parseEnum[T ~string](text []byte, allowed []T, dst *T) error {
	v := T(strings.ToLower(strings.TrimSpace(string(text))))
	if slices.Contains(allowed, v) {
		*dst = v
		return nil
	}
	quoted := make([]string, len(allowed))
	for i, a := range allowed {
		quoted[i] = strconv.Quote(string(a))
	}
	return fmt.Errorf("must be one of %s", strings.Join(quoted, ", "))
}

// URL is an absolute http(s) URL in canonical form: no trailing slash, no
// credentials, no query or fragment. Every consumer joins paths onto it
// (redirect URIs, share links), so ".../x" and ".../x/" must not be able to
// produce two different absolute URLs for the same resource.
//
// The embedded pointer is nil until UnmarshalText succeeds; the methods below
// tolerate that so a failed load can still be printed.
type URL struct {
	*url.URL
}

// IsZero reports whether the URL was never set.
func (u URL) IsZero() bool { return u.URL == nil }

// String shadows the promoted method, which panics on a nil receiver.
func (u URL) String() string {
	if u.URL == nil {
		return ""
	}
	return u.URL.String()
}

// MarshalText is the inverse of UnmarshalText. Without it a config dumped to
// JSON shows the eleven fields of net/url.URL instead of the URL.
func (u URL) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

func (u *URL) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	parsed, err := url.Parse(raw)
	// A bare "localhost:8080" parses as scheme "localhost", opaque "8080" —
	// hence the Host check rather than trusting err alone.
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("must use scheme http or https")
	}
	if parsed.User != nil {
		return errors.New("must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("must not contain a query string or fragment")
	}
	if port := parsed.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return errors.New("must have a port between 1 and 65535")
		}
	}

	// Trailing slashes are stripped rather than rejected: a browser adds one
	// when you copy the address bar, and the value is unambiguous either way.
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	u.URL = parsed
	return nil
}

// CIDRs is a set of network ranges, parsed from a comma-separated list.
//
// It exists for one job — deciding whether the peer on the other end of a
// connection is a reverse proxy this deployment installed — so it answers
// "does this set contain this address" and nothing else. An empty set contains
// nothing, which is the safe answer for a server reachable directly.
//
// A bare address is accepted and means that address alone: an operator naming
// one proxy should not have to remember to write "/32".
type CIDRs struct {
	prefixes []netip.Prefix
}

// IsZero reports whether the set is empty — no proxy is trusted.
func (c CIDRs) IsZero() bool { return len(c.prefixes) == 0 }

// Contains reports whether addr falls in any of the ranges.
//
// The address is unmapped first, so a peer that arrives on a dual-stack
// listener as ::ffff:10.0.0.1 is matched by the 10.0.0.0/8 an operator wrote,
// rather than silently failing to be a trusted proxy.
func (c CIDRs) Contains(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range c.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// String renders the set the way it is written in the environment.
func (c CIDRs) String() string {
	parts := make([]string, len(c.prefixes))
	for i, prefix := range c.prefixes {
		parts[i] = prefix.String()
	}
	return strings.Join(parts, ",")
}

// MarshalText is the inverse of UnmarshalText, so a config dumped to JSON shows
// the list rather than the internals of netip.Prefix.
func (c CIDRs) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

func (c *CIDRs) UnmarshalText(text []byte) error {
	var prefixes []netip.Prefix
	for _, field := range strings.Split(string(text), ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue // a trailing comma, or "a, b" — not worth an error
		}
		prefix, err := parsePrefix(field)
		if err != nil {
			return err
		}
		prefixes = append(prefixes, prefix)
	}
	if len(prefixes) == 0 {
		return errors.New("must list at least one address or CIDR range, or be left unset")
	}
	c.prefixes = prefixes
	return nil
}

// parsePrefix reads one entry of a [CIDRs] list.
func parsePrefix(field string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(field); err == nil {
		// Masked: 10.0.0.1/8 is a range with a host address written in it, and
		// Prefix.Contains reports false for every address unless the bits
		// outside the mask are clear.
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(field)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf(
			"must be a comma-separated list of addresses or CIDR ranges, such as "+
				`"10.0.0.0/8,192.168.1.7", but %s is neither`, strconv.Quote(field))
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// Redacted placeholders. They are what a [Secret] renders as everywhere a
// value could otherwise reach a log, an error or a JSON dump.
const (
	redactedPlaceholder = "[REDACTED]"
	unsetPlaceholder    = "[unset]"
)

// minSecretBytes is the floor for any signing key: 32 bytes is the output size
// of the hash behind the session MAC, so less than that adds no security.
const minSecretBytes = 32

// minDistinctRunes rejects "aaaa…" and "1234123412341234…": long, but with
// nowhere near the entropy the length suggests.
const minDistinctRunes = 8

// Secret is a configuration value that must never be printed. It implements
// [fmt.Stringer], [fmt.GoStringer], [json.Marshaler] and [slog.LogValuer], so
// every ordinary way of rendering a Config — "%v", "%#v", a JSON dump, a
// structured log attribute — redacts it. Reaching the bytes takes an explicit
// call to [Secret.Reveal], which is greppable in review.
type Secret struct {
	b []byte
}

// NewSecret is for tests and for callers that already hold validated key
// material; [Load] goes through UnmarshalText, which also enforces strength.
func NewSecret(b []byte) Secret { return Secret{b: bytes.Clone(b)} }

// Reveal returns a copy of the secret bytes. Every call site is a place where
// key material escapes this type — keep it to the few that need it.
func (s Secret) Reveal() []byte { return bytes.Clone(s.b) }

// IsZero reports whether the secret was never set.
func (s Secret) IsZero() bool { return len(s.b) == 0 }

func (s Secret) String() string {
	if len(s.b) == 0 {
		return unsetPlaceholder
	}
	return redactedPlaceholder
}

// GoString covers the "%#v" verb, which ignores String.
func (s Secret) GoString() string { return "config.Secret(" + s.String() + ")" }

// MarshalJSON covers a config dump serialised to JSON.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// LogValue covers slog, which would otherwise reflect over the struct.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

func (s *Secret) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if n := secretBytes(raw); n < minSecretBytes {
		return fmt.Errorf("must carry at least %d bytes of secret material, this carries %d "+
			"(generate one with `openssl rand -base64 32`)", minSecretBytes, n)
	}
	if reason, weak := weakSecret(raw); weak {
		return fmt.Errorf("is a placeholder or a guessable value (%s) "+
			"(generate one with `openssl rand -base64 32`)", reason)
	}
	s.b = []byte(raw)
	return nil
}

// decoders are the encodings a generated secret is usually pasted in.
var decoders = []func(string) ([]byte, error){
	base64.StdEncoding.DecodeString,
	base64.RawStdEncoding.DecodeString,
	base64.URLEncoding.DecodeString,
	base64.RawURLEncoding.DecodeString,
	hex.DecodeString,
}

// secretBytes reports how many bytes of secret material a value carries.
//
// Encoding always expands, so the honest measure is the *smallest* plausible
// reading of the value: `openssl rand -base64 32` is 44 characters but only 32
// bytes of entropy, and a 40-character base64 string is 30 bytes however long
// it looks. A value that decodes as nothing is measured as its raw bytes.
func secretBytes(raw string) int {
	n := len(raw)
	for _, decode := range decoders {
		if b, err := decode(raw); err == nil && len(b) < n {
			n = len(b)
		}
	}
	return n
}

// weakSecretMarkers are substrings that no generated secret contains and that
// every shipped placeholder does. A deployment running with one of these is not
// protected by session signing at all, so it is a startup error rather than a
// warning nobody reads.
var weakSecretMarkers = []string{
	"changeme", "change-me", "change_me",
	"replaceme", "replace-me", "replace_me",
	"placeholder", "example", "sample", "default",
	"insecure", "notsecure", "notsafe", "unsafe",
	"password", "passwd", "secretkey", "secret-key", "secret_key",
	"supersecret", "topsecret", "mysecret", "devsecret", "dev-secret",
	"blacklight", "testtest", "abcdef", "123456", "qwerty",
}

// weakSecret returns a short reason and true when raw must be rejected. The
// reason names the pattern, never the value: it is repeated back to the
// operator in an error that may be logged.
func weakSecret(raw string) (string, bool) {
	lower := strings.ToLower(raw)
	for _, marker := range weakSecretMarkers {
		if strings.Contains(lower, marker) {
			return "contains " + strconv.Quote(marker), true
		}
	}
	distinct := map[rune]struct{}{}
	for _, r := range raw {
		distinct[r] = struct{}{}
	}
	if len(distinct) < minDistinctRunes {
		return fmt.Sprintf("only %d distinct characters", len(distinct)), true
	}
	return "", false
}
