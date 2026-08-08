package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
)

// TokenBytes is how much randomness a session token carries. Thirty-two bytes
// is 256 bits: far beyond guessing, and the same size as the hash it is stored
// under, so neither half is the weak one.
const TokenBytes = 32

// tokenEncoding is base64url without padding, which is safe in a cookie value
// without quoting or escaping. Padding is omitted because "=" is legal in a
// cookie value but is the sort of character that gets helpfully re-encoded
// somewhere in a proxy chain.
var tokenEncoding = base64.RawURLEncoding

// tokenLength is how many characters a well-formed token has. A cookie of any
// other length cannot be one this server issued, so it is rejected before
// anything is hashed or looked up.
var tokenLength = tokenEncoding.EncodedLen(TokenBytes)

// redacted is what a [Token] renders as, whoever asks.
const redacted = "[redacted]"

// Token is a session token: the value in the cookie, and a credential exactly
// as much as a password is.
//
// It is a distinct type for the same reason password.Plaintext is — so that the
// compiler decides where it may go, rather than a reviewer's memory. Every
// ordinary way a value reaches a log or a response is overridden to produce
// [redacted]: fmt verbs, log/slog attributes and JSON encoding. Reading the
// characters requires [Token.Reveal].
type Token string

// Reveal returns the token itself. Call it where the value is used — hashing it,
// or putting it in a Set-Cookie header — and do not store what it returns.
func (t Token) Reveal() string { return string(t) }

// String implements fmt.Stringer for callers that reach for it directly.
func (Token) String() string { return redacted }

// GoString covers "%#v", which ignores Stringer.
func (Token) GoString() string { return redacted }

// Format implements fmt.Formatter, which is what makes the redaction total: fmt
// consults it for every verb, so %s, %q, %x and the rest all produce the
// placeholder rather than only the verbs Stringer covers.
func (Token) Format(f fmt.State, verb rune) {
	switch verb {
	case 'q':
		fmt.Fprintf(f, "%q", redacted)
	default:
		fmt.Fprint(f, redacted)
	}
}

// LogValue implements slog.LogValuer, so slog.Any("token", t) records the
// placeholder — including when the token is a field of a struct being logged.
func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON implements json.Marshaler. A session token has exactly one
// destination, the Set-Cookie header, so anything serializing one is sending it
// somewhere it does not belong.
func (Token) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// MarshalText covers the encoders that prefer it to MarshalJSON.
func (Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// newToken mints a token from crypto/rand.
//
// A failure here is not recoverable and must never be papered over with a
// weaker source: every session this process goes on to issue would be
// predictable. It is returned so the caller fails the request.
func newToken() (Token, error) {
	raw := make([]byte, TokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("session: read %d random bytes: %w", TokenBytes, err)
	}
	return Token(tokenEncoding.EncodeToString(raw)), nil
}

// hash returns what is stored for a token: HMAC-SHA256 under the deployment's
// session secret, base64url.
//
// Keyed rather than a bare SHA-256 for two reasons. A stolen database is not
// enough to look a token up — the attacker needs the secret as well, which lives
// in the environment and not in the file they copied. And rotating the secret
// invalidates every session, which is the "log everybody out" lever an operator
// expects to have and which .env.example promises them.
//
// It is deliberately fast. A token is 256 bits of uniform randomness, so there
// is no dictionary to run against it and nothing for a slow hash to buy; this
// runs on every authenticated request.
func (m *Manager) hash(token Token) string {
	mac := hmac.New(sha256.New, m.secret)
	// hash.Hash never returns an error, as its own documentation states.
	mac.Write([]byte(token.Reveal()))
	return tokenEncoding.EncodeToString(mac.Sum(nil))
}
