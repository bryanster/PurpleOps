package servicetoken

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// The shape of a token: `bl_<prefix>_<secret>`.
//
// Three parts, and each earns its place.
//
//   - `bl_` says what this string is. A credential that announces itself is one
//     a secret scanner can find — GitHub's push protection, an operator
//     grepping a CI log, the pre-commit hook somebody wrote. A high-entropy
//     string with no marker is indistinguishable from a base64 blob, and the
//     leak is found by whoever is looking for it rather than by whoever is
//     protecting against it.
//   - The prefix is public and unique, and is what the row is found by. Without
//     it, authenticating means comparing a presented secret against every
//     stored hash — a scan whose cost grows with the number of tokens and whose
//     timing says how many there are.
//   - The secret is the credential, and only its hash is stored.
const (
	// Marker is the fixed opening. It is separated from the prefix by
	// [separator] like everything else, so the parse is one split.
	Marker = "bl"

	separator = "_"

	// PrefixBytes is how much randomness the public half carries. Six bytes is
	// ten characters: short enough to read out over a phone when somebody is
	// deciding which token to revoke, and far more than enough to keep prefixes
	// from colliding at any number of tokens a deployment will hold. It is not
	// a secret and is not required to be unguessable — the database's UNIQUE
	// constraint is what makes it unambiguous.
	PrefixBytes = 6

	// SecretBytes is 32: 256 bits from crypto/rand, the same size as the hash
	// it is stored under, so neither half is the weak one. It is the same
	// choice session.TokenBytes makes, for the same reasons.
	SecretBytes = 32
)

// tokenEncoding spells the two random halves: RFC 4648 base32, uppercase, no
// padding.
//
// Not base64url, which is what every other opaque value in this tree uses and
// which cannot be used here: its alphabet contains "_", which is [separator].
// A prefix that happened to encode one would split into a token with four parts
// — reliably, for a few in a thousand tokens, and only in production.
//
// Base32 also happens to be the friendlier alphabet for the half that gets read
// aloud: no case to get wrong, and nothing that looks like something else in a
// terminal font. Padding is omitted because "=" is the sort of character that
// gets helpfully re-encoded somewhere in a proxy chain.
var tokenEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// hashEncoding spells the stored hash, which is never parsed and never sent, so
// it stays base64url like every other stored digest here.
var hashEncoding = base64.RawURLEncoding

var (
	prefixLength = tokenEncoding.EncodedLen(PrefixBytes)
	secretLength = tokenEncoding.EncodedLen(SecretBytes)
)

// hashDomain separates this HMAC from every other one keyed with material
// derived from the deployment's secrets. Without it, a value that hashed alike
// under two constructions would be usable in both.
const hashDomain = "blacklight/service-token\x00"

// derivationInfo binds the MAC key to this use, and differs from the info
// strings internal/authn/secrets and internal/authn/recovery pass. Three
// derivations from one configured value, and a bug in any one of them can
// neither read nor forge the others.
const derivationInfo = "blacklight/service-token/hmac-sha256/v1"

// keyBytes is 32, the natural key size for HMAC-SHA256.
const keyBytes = 32

const redacted = "[redacted]"

// ErrMalformed reports a value that is not a service token at all: the wrong
// marker, the wrong number of parts, or a part of the wrong length.
//
// It is deliberately not the same as "no token has that prefix". A caller
// answers both with the same 401 — see [Manager.Resolve] — but only one of them
// is worth a database round trip, and telling them apart here is what keeps a
// request carrying a megabyte of Authorization header from costing a query.
var ErrMalformed = errors.New("servicetoken: the value is not a service token")

// Token is a service token as its holder has it: the whole `bl_…_…` string, and
// a credential exactly as much as a password is.
//
// It is a distinct type for the reason [session.Token] and [password.Plaintext]
// are — so the compiler decides where it may go rather than a reviewer's
// memory. Every ordinary way a value reaches a log or a response is overridden
// to produce [redacted]: fmt verbs, log/slog attributes and JSON encoding.
// Reading the characters takes [Token.Reveal], which the one handler that shows
// a token to its owner calls and nothing else does.
type Token string

// Reveal returns the token itself. Call it where the value is used — hashing
// it, or writing it into the one response that carries it — and do not store
// what it returns.
func (t Token) Reveal() string { return string(t) }

func (Token) String() string   { return redacted }
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

// LogValue implements slog.LogValuer, so a token logged as an attribute — or as
// a field of a struct being logged — records the placeholder.
func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON and MarshalText cover the encoders. The one response that carries
// a token spells [Token.Reveal] out; anything that serializes one by accident is
// sending a credential somewhere nobody decided it should go.
func (Token) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }
func (Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// parts is a token taken apart: the public half that finds the row, and the
// secret half that proves it.
type parts struct {
	prefix string
	secret string
}

// mint returns a fresh token and its two halves.
//
// A failure from crypto/rand is returned rather than worked around: a token
// from a weaker source is a token somebody can guess, and the caller's
// alternative is to fail the creation, which is the right outcome.
func mint() (Token, parts, error) {
	prefix, err := randomString(PrefixBytes)
	if err != nil {
		return "", parts{}, err
	}
	secret, err := randomString(SecretBytes)
	if err != nil {
		return "", parts{}, err
	}
	return Token(Marker + separator + prefix + separator + secret),
		parts{prefix: prefix, secret: secret}, nil
}

func randomString(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("servicetoken: read %d random bytes: %w", n, err)
	}
	return tokenEncoding.EncodeToString(raw), nil
}

// parse takes a presented token apart, or reports [ErrMalformed].
//
// It is strict about lengths, and that strictness is the point: no token this
// server issued has any other shape, so anything else is refused before a query
// runs. The reason it names is for the log — a caller is told nothing beyond
// 401 either way.
func parse(token Token) (parts, error) {
	// SplitN rather than Split: base32 has no underscore in its alphabet (see
	// tokenEncoding, which is why it is base32), so a well-formed token has
	// exactly two separators — and bounding the split means a pathological
	// value costs a fixed number of allocations rather than one per underscore
	// somebody put in it.
	fields := strings.SplitN(token.Reveal(), separator, 4)
	switch {
	case len(fields) != 3:
		return parts{}, fmt.Errorf("%w: it has %d parts, want 3", ErrMalformed, len(fields))
	case fields[0] != Marker:
		return parts{}, fmt.Errorf("%w: it does not begin with %q", ErrMalformed, Marker+separator)
	case len(fields[1]) != prefixLength:
		return parts{}, fmt.Errorf("%w: its prefix has %d characters, want %d",
			ErrMalformed, len(fields[1]), prefixLength)
	case len(fields[2]) != secretLength:
		return parts{}, fmt.Errorf("%w: its secret has %d characters, want %d",
			ErrMalformed, len(fields[2]), secretLength)
	}
	return parts{prefix: fields[1], secret: fields[2]}, nil
}

// Hasher turns the secret half of a token into the value stored for it.
// Construct it with [NewHasher]; it is safe for concurrent use, which is how it
// is used — one per process, shared by every request.
//
// HMAC-SHA256 and not Argon2id, for the reasons internal/authn/recovery gives
// at length and one more that matters here: this runs on every
// token-authenticated request, which is every request an integration makes. A
// work factor would buy nothing against 256 bits of uniform randomness and
// would put a hundred milliseconds on the critical path of the API's only
// supported automation surface.
//
// The key comes from BLACKLIGHT_ENCRYPTION_KEY and deliberately not from
// BLACKLIGHT_SESSION_SECRET. Rotating the session secret is the documented way
// to sign every browser out, and it must not also break every integration in
// the deployment — silently, at whatever hour the rotation happened, with the
// only symptom being pipelines failing on 401.
type Hasher struct {
	key []byte
}

// NewHasher returns a Hasher over the given key material, or an error
// describing why the material cannot produce one.
//
// key is whatever the operator configured, not a MAC key: it is stretched to 32
// bytes by HKDF-SHA256 under [derivationInfo]. The length check is on the
// input, because HKDF produces a well-formed key from four bytes of entropy and
// the result would be a well-formed key nobody has to guess very hard.
func NewHasher(key []byte) (*Hasher, error) {
	if len(key) < keyBytes {
		return nil, fmt.Errorf("servicetoken: the encryption key carries %d bytes, want at least %d",
			len(key), keyBytes)
	}
	derived, err := hkdf.Key(sha256.New, key, nil, derivationInfo, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("servicetoken: derive the token-hashing key: %w", err)
	}
	return &Hasher{key: derived}, nil
}

// Hash returns what is stored for a secret: HMAC-SHA256 over it under the
// derived key, base64url.
//
// Deterministic and unsalted, which is what lets the row be found by prefix and
// confirmed by one comparison — and is safe for the reason the doc comment on
// [Hasher] gives: there is nothing to precompute against 256 random bits when
// the key is not in the table builder's hands.
func (h *Hasher) Hash(secret string) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(hashDomain))
	mac.Write([]byte(secret))
	return hashEncoding.EncodeToString(mac.Sum(nil))
}

// equal compares two stored hashes in constant time.
//
// The values compared are hashes rather than secrets, so a timing signal here
// leaks the shape of a hash and not a token. It is constant-time anyway,
// because the alternative is making that argument every time somebody reads
// the resolution path — and because the argument stops holding the moment
// somebody compares something else with it.
func equal(a, b string) bool { return hmac.Equal([]byte(a), []byte(b)) }
