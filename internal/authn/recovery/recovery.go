// Package recovery is how a recovery code is spelled, read back and hashed
// (M1-007), and nothing else.
//
// It holds no state, touches no database and knows nothing about users or
// sessions — the same division internal/authn/totp keeps, and for the same
// reason: what a code *is* can then be tested exhaustively against the one
// thing that matters about it, which is that a person can copy it off a screen
// onto paper and type it back in six months later without the server caring how
// they wrote it down.
//
// Three decisions are made here and nowhere else.
//
//   - **The alphabet.** Crockford's base32: the digits and the uppercase
//     letters, less I, L, O and U. No two characters in it look alike, which is
//     what M1-007 asks for — and better, the four that were left out are
//     *accepted* on the way in and folded onto the ones they resemble, so
//     somebody who writes down O for 0 is not locked out by their own
//     handwriting. U is absent for Crockford's own reason: it keeps the
//     accidental obscenities out of a string nobody chose.
//   - **The size.** Twenty characters is exactly 100 bits, comfortably past the
//     80 the ticket asks for, and it groups into five blocks of four.
//   - **The hash.** Keyed HMAC-SHA256, not a password KDF. See [Hasher].
package recovery

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	// SetSize is how many codes a person is given at once. Ten is what M1-007
	// specifies: enough that losing a phone is survivable more than once, few
	// enough to fit on the piece of paper somebody is going to write them on.
	SetSize = 10

	// Length is the number of characters in a code, without separators. Twenty
	// characters over a 32-symbol alphabet is 100 bits — an attacker guessing
	// one is not a threat model, they are a rounding error.
	Length = 20

	// group is how many characters go between separators, and separator is what
	// goes between them. Five groups of four is the shape of every product key
	// and every backup code anybody has typed before, which is the entire
	// argument for it.
	group     = 4
	separator = "-"

	// alphabet is Crockford's base32, in the order that makes index arithmetic
	// the same as his: 0-9 then A-Z without I, L, O or U.
	alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	// mask selects a symbol from a random byte. The alphabet is exactly 32
	// symbols, a power of two, so masking the low five bits is uniform — there
	// is no modulo bias to reject samples for.
	mask = 0x1f
)

// hashDomain separates this HMAC from every other one keyed with material
// derived from the deployment's secrets. Without it a value that hashed alike
// under two constructions would be usable in both.
const hashDomain = "blacklight/recovery-code\x00"

// derivationInfo binds the MAC key to this use. It differs from the info string
// internal/authn/secrets passes, so the key that authenticates codes and the key
// that encrypts TOTP secrets are different keys derived from the same
// configured value — a bug in one can never read or forge the other.
const derivationInfo = "blacklight/recovery-code/hmac-sha256/v1"

// keyBytes is 32, the natural key size for HMAC-SHA256: the block is 64 bytes,
// so a longer key would be hashed down to 32 anyway.
const keyBytes = 32

// encoding is base64url without padding, matching how every other opaque value
// in this tree is spelled in a TEXT column.
var encoding = base64.RawURLEncoding

const redacted = "[redacted]"

// ErrMalformed reports input that is not a recovery code at all: the wrong
// length, or a character the alphabet does not have and the fold-in does not
// rescue.
//
// It is deliberately not the same as "that is not one of your codes". A caller
// answers both with the same refusal — see internal/authn — but only one of
// them is worth counting as a guess, and telling them apart is this package's
// job rather than a regular expression's somewhere else.
var ErrMalformed = errors.New("recovery: the value is not a recovery code")

// Code is one recovery code in canonical form: [Length] characters from the
// alphabet, uppercase, no separators. It is a credential as much as a password
// is, so every ordinary way of rendering one produces [redacted] — reading it
// takes [Code.Reveal] or [Code.Printed].
type Code string

// Reveal returns the canonical characters. It is what [Hasher.Hash] consumes;
// what a person is shown is [Code.Printed].
func (c Code) Reveal() string { return string(c) }

// Printed returns the code as somebody should see it: the same characters in
// groups of four, hyphen-separated. It is a rendering and not a second value —
// [Parse] maps it back to exactly this code.
func (c Code) Printed() string {
	raw := string(c)
	if len(raw) == 0 {
		return ""
	}

	var out strings.Builder
	out.Grow(len(raw) + len(raw)/group)
	for i := 0; i < len(raw); i += group {
		if i > 0 {
			out.WriteString(separator)
		}
		out.WriteString(raw[i:min(i+group, len(raw))])
	}
	return out.String()
}

func (Code) String() string   { return redacted }
func (Code) GoString() string { return redacted }

// Format implements fmt.Formatter, which is what makes the redaction total: fmt
// consults it for every verb rather than only the ones Stringer covers.
func (Code) Format(f fmt.State, verb rune) {
	switch verb {
	case 'q':
		fmt.Fprintf(f, "%q", redacted)
	default:
		fmt.Fprint(f, redacted)
	}
}

// LogValue implements slog.LogValuer, so a code logged as an attribute — or as
// a field of a struct being logged — records the placeholder.
func (Code) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON and MarshalText cover the encoders. A code reaches a response
// body exactly once, through a handler that spells [Code.Printed] out; anything
// that serializes one by accident is sending a credential somewhere nobody
// decided it should go.
func (Code) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }
func (Code) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// Generate mints a fresh set of [SetSize] codes.
//
// Every code is independent: there is no seed, no derivation and no order, so
// holding one says nothing about the other nine. A failure from crypto/rand is
// returned rather than worked around — codes from a weaker source would be
// codes somebody could guess, and the caller's alternative is to fail the
// enrolment, which is the right outcome.
func Generate() ([]Code, error) {
	codes := make([]Code, SetSize)
	for i := range codes {
		code, err := generateOne()
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

func generateOne() (Code, error) {
	raw := make([]byte, Length)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("recovery: read %d random bytes: %w", Length, err)
	}
	for i, b := range raw {
		raw[i] = alphabet[b&mask]
	}
	return Code(raw), nil
}

// Parse reads a code back from whatever a person typed.
//
// It is forgiving about everything that is not the code: case, surrounding
// space, the hyphens this package prints and any other spacing somebody added,
// and the four characters Crockford leaves out — O becomes 0, and I and L
// become 1. It is not forgiving about length, because a code of the wrong
// length is not a code and there is nothing to compare it against.
func Parse(input string) (Code, error) {
	var out strings.Builder
	out.Grow(Length)

	for _, r := range strings.ToUpper(strings.TrimSpace(input)) {
		switch {
		case r == '-' || r == ' ':
			// The separator this package prints, and the one somebody used
			// instead. Neither carries information.
			continue
		case r == 'O':
			out.WriteByte('0')
		case r == 'I' || r == 'L':
			out.WriteByte('1')
		case strings.ContainsRune(alphabet, r):
			out.WriteRune(r)
		default:
			return "", fmt.Errorf("%w: %q is not one of its characters", ErrMalformed, r)
		}
	}

	if out.Len() != Length {
		return "", fmt.Errorf("%w: it has %d characters, want %d",
			ErrMalformed, out.Len(), Length)
	}
	return Code(out.String()), nil
}

// Hasher turns a code into the value stored for it. Construct it with
// [NewHasher]; it is safe for concurrent use, which is how it is used — one per
// process, shared by every request.
//
// It is an HMAC and not Argon2id, which is the choice M1-007 asks to be
// justified. Three things decide it:
//
//   - A code carries 100 bits from crypto/rand. There is no dictionary and no
//     human-chosen pattern for a slow KDF to slow down; work factor buys
//     nothing against a search space nobody can enter.
//   - Verification has to compare a presented code against every unused code the
//     person holds, because the codes are not ordered and the caller does not
//     know which one arrived. Under Argon2id that is ten sequential derivations
//     — most of a second — on an endpoint reachable before authentication,
//     which is a denial-of-service lever aimed at the login path.
//   - Being keyed is worth more here than being slow. A copy of the database
//     alone yields nothing: forging a code needs the key as well, which lives
//     in the environment rather than in the file somebody exfiltrated.
//
// The key comes from BLACKLIGHT_ENCRYPTION_KEY and deliberately not from
// BLACKLIGHT_SESSION_SECRET, for the reason M1-006 gives: rotating the session
// secret is the documented way to sign everybody out, and it must not also
// destroy every recovery code in the deployment — silently, with the only
// symptom being that the way back in stopped working at the moment somebody
// needed it.
type Hasher struct {
	key []byte
}

// NewHasher returns a Hasher over the given key material, or an error
// describing why the material cannot produce one.
//
// key is whatever the operator configured, not a MAC key: it is stretched to 32
// bytes by HKDF-SHA256 under [derivationInfo]. The length check is on the input,
// because HKDF produces a well-formed key from four bytes of entropy and the
// result would be a well-formed key nobody has to guess very hard.
func NewHasher(key []byte) (*Hasher, error) {
	if len(key) < keyBytes {
		return nil, fmt.Errorf("recovery: the encryption key carries %d bytes, want at least %d",
			len(key), keyBytes)
	}

	// No salt: there is one key, it is already high-entropy, and a salt would
	// be a constant with nothing to do. The info string is what separates this
	// derivation from internal/authn/secrets'.
	derived, err := hkdf.Key(sha256.New, key, nil, derivationInfo, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("recovery: derive the code-hashing key: %w", err)
	}
	return &Hasher{key: derived}, nil
}

// Hash returns what is stored for a code: HMAC-SHA256 over the canonical
// characters under the derived key, base64url.
//
// It is deterministic and unsalted, which is what makes a set of ten comparable
// in one pass — and is safe for exactly the reason the doc comment on [Hasher]
// gives: there is nothing to precompute a rainbow table over when the input is
// 100 random bits and the key is not in the table builder's hands.
func (h *Hasher) Hash(code Code) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(hashDomain))
	mac.Write([]byte(code.Reveal()))
	return encoding.EncodeToString(mac.Sum(nil))
}

// HashAll returns the stored value for each of codes, in order.
func (h *Hasher) HashAll(codes []Code) []string {
	hashes := make([]string, len(codes))
	for i, code := range codes {
		hashes[i] = h.Hash(code)
	}
	return hashes
}

// Equal compares two stored hashes in constant time.
//
// The values it compares are hashes rather than secrets, so a timing signal
// here leaks the shape of a hash and not a code. It costs one function call to
// not have to make that argument every time somebody reads the verification
// loop.
func Equal(a, b string) bool { return hmac.Equal([]byte(a), []byte(b)) }
