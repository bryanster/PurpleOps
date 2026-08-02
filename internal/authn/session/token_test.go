package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// The token is a credential. These are the ways a value ordinarily escapes into
// somewhere it is kept — a log, a response, an error message — and every one of
// them has to produce the placeholder instead.

func TestATokenRedactsItselfHoweverItIsPrinted(t *testing.T) {
	t.Parallel()

	token := Token("a-real-looking-session-token")

	// %v and %s go through Format; %q must quote the placeholder rather than the
	// value; %#v and %x are the ones a Stringer alone would miss.
	for _, verb := range []string{"%v", "%s", "%q", "%#v", "%x", "%+v"} {
		got := fmt.Sprintf(verb, token)
		if strings.Contains(got, "a-real-looking") {
			t.Errorf("fmt.Sprintf(%q, token) = %s, which contains the token", verb, got)
		}
	}

	// Inside a struct, which is how it will usually reach a log line.
	type wrapper struct {
		Session Token
		Where   string
	}
	if got := fmt.Sprintf("%+v", wrapper{Session: token, Where: "cookie"}); strings.Contains(got, "a-real-looking") {
		t.Errorf("a struct holding a token printed as %s", got)
	}
}

func TestATokenIsRedactedInTheLogAndInJSON(t *testing.T) {
	t.Parallel()

	token := Token("a-real-looking-session-token")

	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("issued", slog.Any("token", token))
	if strings.Contains(buf.String(), "a-real-looking") {
		t.Errorf("slog wrote the token: %s", buf.String())
	}

	encoded, err := json.Marshal(struct {
		Token Token `json:"token"`
	}{token})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if strings.Contains(string(encoded), "a-real-looking") {
		t.Errorf("JSON encoding wrote the token: %s", encoded)
	}
}

func TestRevealIsTheOnlyWayToTheCharacters(t *testing.T) {
	t.Parallel()

	if got, want := Token("value").Reveal(), "value"; got != want {
		t.Errorf("Reveal() = %q, want %q", got, want)
	}
}

func TestANewTokenIsRandomAndTheRightShape(t *testing.T) {
	t.Parallel()

	seen := map[Token]bool{}
	for range 100 {
		token, err := newToken()
		if err != nil {
			t.Fatalf("newToken() = %v", err)
		}
		if len(token) != tokenLength {
			t.Fatalf("newToken() is %d characters, want %d", len(token), tokenLength)
		}
		if seen[token] {
			t.Fatalf("newToken() returned a value it had already returned; it is not random")
		}
		seen[token] = true
	}
}

// TestTheHashIsKeyedByTheSecret is what makes a stolen database insufficient,
// and what makes rotating BLACKLIGHT_SESSION_SECRET a way to log everybody out.
func TestTheHashIsKeyedByTheSecret(t *testing.T) {
	t.Parallel()

	token := Token("the-same-token-either-way")
	first := newTestManager(t, func(o *Options) { o.Secret = []byte("first-secret-first-secret-first!") })
	second := newTestManager(t, func(o *Options) { o.Secret = []byte("second-secret-second-secret-sec!") })

	if first.hash(token) == second.hash(token) {
		t.Error("two deployments with different secrets store the same hash for a token")
	}
	stored := first.hash(token)
	if again := first.hash(token); stored != again {
		t.Error("hashing the same token twice gave two answers; no session would ever resolve")
	}
	if strings.Contains(stored, token.Reveal()) {
		t.Errorf("the stored hash contains the token itself: %s", stored)
	}
}
