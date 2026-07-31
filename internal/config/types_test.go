package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

// TestSecretIsRedactedInEveryRendering covers the ways a Config actually
// reaches a log or a support bundle. Each one goes through a different method,
// so each one is a separate way to leak the session key.
func TestSecretIsRedactedInEveryRendering(t *testing.T) {
	const value = "a-secret-that-must-never-be-printed-anywhere"

	cfg, errs := parse(envWith(map[string]string{envSessionSecret: value}))
	if len(errs) > 0 {
		t.Fatalf("parse() = %v, want no errors", errs)
	}

	renderings := map[string]func() string{
		`fmt "%v" of the config`:  func() string { return fmt.Sprintf("%v", cfg) },
		`fmt "%+v" of the config`: func() string { return fmt.Sprintf("%+v", cfg) },
		`fmt "%#v" of the config`: func() string { return fmt.Sprintf("%#v", cfg) },
		`fmt "%s" of the config`:  func() string { return fmt.Sprintf("%s", cfg) },
		`fmt "%v" of the secret`:  func() string { return fmt.Sprintf("%v", cfg.Session.Secret) },
		`fmt "%#v" of the secret`: func() string { return fmt.Sprintf("%#v", cfg.Session.Secret) },
		"json of the config": func() string {
			b, err := json.Marshal(cfg)
			if err != nil {
				t.Fatalf("json.Marshal(cfg): %v", err)
			}
			return string(b)
		},
		"slog json handler": func() string {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("startup", "config", cfg)
			return buf.String()
		},
		"slog text handler": func() string {
			var buf bytes.Buffer
			slog.New(slog.NewTextHandler(&buf, nil)).Info("startup", "config", cfg)
			return buf.String()
		},
		"slog attribute of the secret": func() string {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, nil)).Info("startup", "secret", cfg.Session.Secret)
			return buf.String()
		},
	}

	for name, render := range renderings {
		t.Run(name, func(t *testing.T) {
			got := render()
			if strings.Contains(got, value) {
				t.Errorf("the secret appears in %s:\n\t%s", name, got)
			}
			if !strings.Contains(got, redactedPlaceholder) {
				t.Errorf("%s does not show %s, so the redaction may not have run:\n\t%s",
					name, redactedPlaceholder, got)
			}
		})
	}
}

func TestSecretReveal(t *testing.T) {
	secret := NewSecret([]byte("the-value"))

	if got, want := string(secret.Reveal()), "the-value"; got != want {
		t.Errorf("Reveal() = %q, want %q", got, want)
	}

	// Reveal hands out a copy: a caller that scribbles on the slice must not
	// be able to change the key every other caller sees.
	revealed := secret.Reveal()
	revealed[0] = 'X'
	if got, want := string(secret.Reveal()), "the-value"; got != want {
		t.Errorf("after mutating a revealed copy, Reveal() = %q, want %q", got, want)
	}
}

func TestZeroSecretRendersAsUnset(t *testing.T) {
	var secret Secret

	if !secret.IsZero() {
		t.Error("IsZero() = false for the zero Secret")
	}
	if got := fmt.Sprintf("%v", secret); got != unsetPlaceholder {
		t.Errorf("%%v of the zero Secret = %q, want %q", got, unsetPlaceholder)
	}
}

func TestSecretStrength(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool // accepted?
	}{
		{name: "openssl rand -base64 32", value: testSecret, want: true},
		{name: "openssl rand -hex 32", value: "3f7a1c9e5b2d8046af13c5e7920bd4681fa3c05d9e7b264180ac539fe62d7b41", want: true},
		{name: "32 raw bytes", value: "Xq7!vP2#mL9$wR4%tK6^zN8&cB3*hJ5(", want: true},
		{name: "31 raw bytes", value: "Xq7!vP2#mL9$wR4%tK6^zN8&cB3*hJ5", want: false},
		{name: "base64 of 31 bytes", value: "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MA==", want: false},
		{name: "empty", value: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var secret Secret
			err := secret.UnmarshalText([]byte(tt.value))

			if got := err == nil; got != tt.want {
				t.Fatalf("UnmarshalText(%q) error = %v, want accepted = %v", tt.value, err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), tt.value) && tt.value != "" {
				t.Errorf("the error repeats the secret back: %v", err)
			}
		})
	}
}

func TestSecretBytesMeasuresTheSmallestReading(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{value: "", want: 0},
		{value: testSecret, want: 32}, // 44 base64 characters
		{value: "cmVhbGx5IG5vdCB0aGlydHktdHdvIGJ5dGVzIQ==", want: 28},    // long, thin
		{value: "correct horse battery staple, and then some", want: 43}, // decodes as nothing
		{value: "00112233445566778899aabbccddeeff", want: 16},            // 32 hex characters
	}

	for _, tt := range tests {
		if got := secretBytes(tt.value); got != tt.want {
			t.Errorf("secretBytes(%q) = %d, want %d", tt.value, got, tt.want)
		}
	}
}

func TestURLNormalisation(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "https://example.internal", want: "https://example.internal"},
		{raw: "https://example.internal/", want: "https://example.internal"},
		{raw: "https://example.internal///", want: "https://example.internal"},
		{raw: "http://example.internal:8080/base/", want: "http://example.internal:8080/base"},
		{raw: "  https://example.internal  ", want: "https://example.internal"},
	}

	for _, tt := range tests {
		var u URL
		if err := u.UnmarshalText([]byte(tt.raw)); err != nil {
			t.Errorf("UnmarshalText(%q) = %v", tt.raw, err)
			continue
		}
		if got := u.String(); got != tt.want {
			t.Errorf("UnmarshalText(%q) produced %q, want %q", tt.raw, got, tt.want)
		}
	}
}

// TestURLJoinsPathsCleanly is the reason a trailing slash is normalised away:
// every consumer builds an absolute URL from this value.
func TestURLJoinsPathsCleanly(t *testing.T) {
	for _, raw := range []string{"https://example.internal/base", "https://example.internal/base/"} {
		var u URL
		if err := u.UnmarshalText([]byte(raw)); err != nil {
			t.Fatalf("UnmarshalText(%q) = %v", raw, err)
		}
		if got, want := u.JoinPath("auth", "callback").String(), "https://example.internal/base/auth/callback"; got != want {
			t.Errorf("from %q, JoinPath gave %q, want %q", raw, got, want)
		}
	}
}

// TestURLRoundTripsThroughText keeps a dumped config readable: the URL is one
// string, not the innards of net/url.URL, and it parses back to itself.
func TestURLRoundTripsThroughText(t *testing.T) {
	var u URL
	if err := u.UnmarshalText([]byte("https://example.internal/base")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}

	encoded, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal(URL): %v", err)
	}
	if got, want := string(encoded), `"https://example.internal/base"`; got != want {
		t.Fatalf("json.Marshal(URL) = %s, want %s", got, want)
	}

	var back URL
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", encoded, err)
	}
	if back.String() != u.String() {
		t.Errorf("round trip gave %q, want %q", back.String(), u.String())
	}
}

func TestZeroURLIsPrintable(t *testing.T) {
	var u URL

	if !u.IsZero() {
		t.Error("IsZero() = false for the zero URL")
	}
	// A failed load is still printed by whoever reports the failure.
	if got := u.String(); got != "" {
		t.Errorf("String() = %q, want the empty string", got)
	}
	if got := fmt.Sprintf("%v", Config{}); !strings.Contains(got, unsetPlaceholder) {
		t.Errorf("printing the zero Config produced %q", got)
	}
}

func TestURLWrapsTheStandardType(t *testing.T) {
	var u URL
	if err := u.UnmarshalText([]byte("https://example.internal/base")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}

	// Consumers need the stdlib type for anything that takes a *url.URL; this
	// only compiles while URL keeps exposing it.
	hostOf := func(std *url.URL) string { return std.Host }
	if got, want := hostOf(u.URL), "example.internal"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
}

func TestLogLevelMapsToSlog(t *testing.T) {
	tests := map[LogLevel]slog.Level{
		LevelDebug: slog.LevelDebug,
		LevelInfo:  slog.LevelInfo,
		LevelWarn:  slog.LevelWarn,
		LevelError: slog.LevelError,
	}

	for level, want := range tests {
		if got := level.Slog(); got != want {
			t.Errorf("%q.Slog() = %v, want %v", level, got, want)
		}
	}
	if len(tests) != len(logLevels) {
		t.Errorf("%d levels are mapped but %d are accepted; every level needs a slog equivalent",
			len(tests), len(logLevels))
	}
}
