package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are Argon2id's cost settings: how much memory a single hash occupies,
// how many passes it makes over it, and how many lanes it splits into, plus the
// sizes of the salt and the derived key.
//
// They are a type rather than five constants because they are also read back
// out of a stored hash — see [Decode] — and comparing what a hash was made with
// against what [Default] says today is how [Verify] answers needsRehash.
type Params struct {
	// Memory is the size of the block array, in KiB. This is the parameter
	// that costs an attacker with custom hardware; raise it before Time.
	Memory uint32
	// Time is the number of passes over that memory.
	Time uint32
	// Parallelism is the number of lanes, and so the number of cores one hash
	// can occupy.
	Parallelism uint8
	// SaltLength is how many random bytes each hash gets, and KeyLength how
	// many bytes of output are kept. Both are in bytes.
	SaltLength uint32
	KeyLength  uint32
}

// Default returns the parameters new hashes are made with.
//
// OWASP's Password Storage Cheat Sheet (2025) gives m=19456 (19 MiB), t=2, p=1
// as its Argon2id minimum. That is a floor for hardware nobody controls; on the
// hardware PurpleOps actually runs on it completes in well under 50 ms, which
// buys less than it could. These settings sit above it — 64 MiB and three
// passes, single-lane so that a login costs one core rather than several — and
// land in the 100–500 ms band M1-002 asks for: slow enough that an offline
// attacker's throughput hurts, fast enough that a login is not noticeably
// delayed and a burst of them does not become a denial of service.
//
// Re-tune with BenchmarkHash when the target hardware changes. Nothing else in
// the codebase needs to know: raising these values does not invalidate stored
// hashes, because each one carries the settings it was made with and [Verify]
// asks for it to be replaced on the next successful login.
//
// A 16-byte salt and a 32-byte key are the reference implementation's
// recommendations, and are what the PHC format's own test vectors use.
func Default() Params {
	return Params{
		Memory:      64 * 1024,
		Time:        3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// MaxPlaintextBytes bounds what [Hash] and [Verify] will process.
//
// Argon2's own cost does not depend on the length of the password, but copying
// and hashing an unbounded body does, and this package sits on an unauthenticated
// path. [Validate] already rejects anything past [MaxLength] runes; this is the
// backstop for a caller that reaches [Hash] without asking policy first, and is
// generous enough that no policy-legal password can hit it.
const MaxPlaintextBytes = 1024

// ErrMalformedHash reports a stored hash this package cannot read: not PHC
// format, not Argon2id, an unknown version, unparseable parameters, or a salt
// or key that is too short to be one.
//
// It means the stored value is damaged or was written by something else — never
// that a password was wrong. [Verify] keeps the two apart on purpose: one is a
// failed login, the other is an operational problem someone has to look at.
var ErrMalformedHash = errors.New("password: malformed stored hash")

// ErrTooLong reports a plaintext over [MaxPlaintextBytes]. It carries no length
// and no value.
var ErrTooLong = errors.New("password: plaintext is too long to hash")

// ErrInvalidParams reports [Params] that cannot produce a usable hash. It is a
// programming error — [Default] never returns such a value — and exists so that
// hand-built parameters fail with a sentence instead of panicking inside the
// hashing library.
var ErrInvalidParams = errors.New("password: invalid Argon2id parameters")

// Hash derives a new Argon2id hash of plaintext under [Default], with a fresh
// random salt, and returns it in PHC string format.
//
// Hashing the same password twice returns different strings. That is the salt
// doing its job, and it is why a hash cannot be compared with ==.
func Hash(plaintext Plaintext) (string, error) { return Default().Hash(plaintext) }

// Verify reports whether plaintext is the password behind encoded, and whether
// the stored hash should be replaced with one made under [Default].
//
// A wrong password is (false, false, nil) — not an error. An error means
// encoded could not be read at all ([ErrMalformedHash]), which is not something
// the person logging in did.
//
// needsRehash is only ever true alongside ok: the plaintext has to be right
// before there is anything worth re-hashing. The login path (M1-003) is
// expected to act on it, hashing the password it already has and storing the
// result, so that costs raised in [Default] reach existing accounts as their
// owners sign in.
func Verify(plaintext Plaintext, encoded string) (ok bool, needsRehash bool, err error) {
	return Default().Verify(plaintext, encoded)
}

// Hash derives a hash of plaintext under p. Use the package-level [Hash] unless
// you are deliberately hashing under settings other than [Default] — a
// benchmark, or a test that needs a hash to look old.
func (p Params) Hash(plaintext Plaintext) (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	if len(plaintext) > MaxPlaintextBytes {
		return "", ErrTooLong
	}

	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		// crypto/rand does not fail on any supported platform, and if it has,
		// nothing this process goes on to do is safe.
		return "", fmt.Errorf("password: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(plaintext.Reveal()), salt, p.Time, p.Memory, p.Parallelism, p.KeyLength)
	return p.encode(salt, key), nil
}

// Verify checks plaintext against encoded, treating p as the current settings
// when deciding needsRehash. See the package-level [Verify].
func (p Params) Verify(plaintext Plaintext, encoded string) (ok bool, needsRehash bool, err error) {
	stored, salt, key, err := Decode(encoded)
	if err != nil {
		return false, false, err
	}
	if len(plaintext) > MaxPlaintextBytes {
		// No policy-legal password is this long, so this cannot be a real
		// login. Refusing before hashing keeps a large body from costing a
		// hash, and it is not an error the caller can act on differently from
		// a wrong password.
		return false, false, nil
	}

	// The stored parameters, not p's: the hash was made with them, and using
	// anything else would derive a different key and fail every login after a
	// cost change.
	derived := argon2.IDKey([]byte(plaintext.Reveal()), salt,
		stored.Time, stored.Memory, stored.Parallelism, stored.KeyLength)

	// Constant-time: comparing two derived keys with == or bytes.Equal leaks,
	// through timing, how many leading bytes matched, which is enough to
	// reconstruct the stored key one byte at a time.
	if subtle.ConstantTimeCompare(derived, key) != 1 {
		return false, false, nil
	}
	return true, p.strongerThan(stored), nil
}

// validate reports whether p can produce a hash, and one this package would
// read back. [Default] satisfies it; the check is for parameters written by
// hand — a benchmark, a test, or a future configuration path.
//
// Two of these would otherwise be a panic inside golang.org/x/crypto/argon2,
// which refuses zero rounds and zero lanes that way.
func (p Params) validate() error {
	switch {
	case p.Time == 0:
		return fmt.Errorf("%w: time must be at least 1", ErrInvalidParams)
	case p.Parallelism == 0:
		return fmt.Errorf("%w: parallelism must be at least 1", ErrInvalidParams)
	case p.Memory < 8*uint32(p.Parallelism):
		// Argon2 quietly raises a memory size this small to its own floor, so a
		// hash would come back made with parameters other than the ones asked
		// for — and the encoded string would claim the ones asked for.
		return fmt.Errorf("%w: memory must be at least 8 KiB per lane", ErrInvalidParams)
	case p.SaltLength < minStoredSaltBytes || p.KeyLength < minStoredKeyBytes:
		return fmt.Errorf("%w: salt or key is shorter than Decode accepts", ErrInvalidParams)
	case p.SaltLength > maxStoredBytes || p.KeyLength > maxStoredBytes:
		return fmt.Errorf("%w: salt or key is longer than Decode accepts", ErrInvalidParams)
	}
	return nil
}

// strongerThan reports whether a hash made with stored should be replaced by
// one made with p.
//
// Memory, time, salt and key are each stronger when larger, so anything below
// the current setting is worth upgrading. Parallelism is the exception and runs
// the other way: at a fixed memory size, splitting the work across more lanes
// gives an attacker more to parallelize, so a hash made with *fewer* lanes than
// today's setting is not weaker and is left alone.
func (p Params) strongerThan(stored Params) bool {
	return stored.Memory < p.Memory ||
		stored.Time < p.Time ||
		stored.Parallelism > p.Parallelism ||
		stored.SaltLength < p.SaltLength ||
		stored.KeyLength < p.KeyLength
}

// phcAlgorithm is the only algorithm identifier this package writes or accepts.
// argon2i and argon2d are real Argon2 variants and are not interchangeable with
// this one: reading a hash of either as though it were argon2id would silently
// fail every login.
const phcAlgorithm = "argon2id"

// b64 is PHC's encoding: standard base64, without padding.
var b64 = base64.RawStdEncoding

// encode renders a hash in PHC string format, the layout the Argon2 reference
// implementation prints and every other library reads:
//
//	$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>
//
// The parameters are in the string because a hash outlives the code that made
// it. Storing bare bytes and holding the settings in configuration is how an
// installation ends up unable to verify its own users after a tuning change.
func (p Params) encode(salt, key []byte) string {
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		phcAlgorithm, argon2.Version, p.Memory, p.Time, p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key))
}

// Bounds on the salt and key a stored hash may carry. The floors are what makes
// a value a hash at all rather than policy — a 4-byte key would verify a wrong
// password roughly once in four billion tries, and an empty salt is the
// shared-salt bug this package exists to remove. The ceiling is a sanity limit
// on a value read from storage, an order of magnitude above anything [Default]
// would produce.
const (
	minStoredSaltBytes = 8
	minStoredKeyBytes  = 16
	maxStoredBytes     = 256

	// maxStoredMemory is the largest m= a stored hash may ask [Verify] to
	// allocate, in KiB — 1 GiB, sixteen times [Default].
	maxStoredMemory = 1 << 20
)

// Decode reads a PHC-format Argon2id hash back into the parameters it was made
// with, its salt, and its derived key.
//
// It is exported because M1-003 and the tests both need to look at what is
// stored — how old a hash is, in cost terms — without re-deriving it. Every
// failure is [ErrMalformedHash]; the specifics are wrapped for the log, and no
// part of the input is echoed, because a value in this position is a credential
// even when it is not the one expected.
func Decode(encoded string) (p Params, salt, key []byte, err error) {
	// Six fields: the empty string before the leading $, then algorithm,
	// version, parameters, salt and key.
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return Params{}, nil, nil, fmt.Errorf("%w: want 5 $-separated fields, got %d",
			ErrMalformedHash, len(parts)-1)
	}
	if parts[1] != phcAlgorithm {
		return Params{}, nil, nil, fmt.Errorf("%w: algorithm is not %s", ErrMalformedHash, phcAlgorithm)
	}

	versionField, found := strings.CutPrefix(parts[2], "v=")
	if !found {
		return Params{}, nil, nil, fmt.Errorf("%w: no version field", ErrMalformedHash)
	}
	version, err := strconv.ParseUint(versionField, 10, 32)
	if err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: unreadable version field", ErrMalformedHash)
	}
	if version != argon2.Version {
		// A future Argon2 version would derive a different key from the same
		// inputs. Failing loudly beats verifying nobody.
		return Params{}, nil, nil, fmt.Errorf("%w: version %d, want %d",
			ErrMalformedHash, version, argon2.Version)
	}

	if p, err = decodeParams(parts[3]); err != nil {
		return Params{}, nil, nil, err
	}

	if salt, err = b64.DecodeString(parts[4]); err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: salt is not base64", ErrMalformedHash)
	}
	if key, err = b64.DecodeString(parts[5]); err != nil {
		return Params{}, nil, nil, fmt.Errorf("%w: hash is not base64", ErrMalformedHash)
	}
	if len(salt) < minStoredSaltBytes || len(key) < minStoredKeyBytes {
		return Params{}, nil, nil, fmt.Errorf("%w: salt or hash is too short to be one", ErrMalformedHash)
	}
	if len(salt) > maxStoredBytes || len(key) > maxStoredBytes {
		return Params{}, nil, nil, fmt.Errorf("%w: salt or hash is implausibly long", ErrMalformedHash)
	}

	// The lengths are what the stored value actually has, which is what
	// strongerThan must compare — a header claiming 32 bytes over a 16-byte key
	// would otherwise read as up to date.
	p.SaltLength = storedLength(len(salt))
	p.KeyLength = storedLength(len(key))
	return p, salt, key, nil
}

// storedLength narrows a decoded byte count to the width [Params] holds it in.
// The bound is restated here rather than assumed from the caller's check, so
// the conversion is safe on its own terms and stays that way if it moves.
func storedLength(n int) uint32 {
	if n < 0 || n > maxStoredBytes {
		return maxStoredBytes
	}
	return uint32(n)
}

// decodeParams reads the m=,t=,p= field. Keys are matched by name rather than
// by position, but all three must be present exactly once: a missing one would
// otherwise default to zero, and argon2.IDKey panics on zero memory or time.
func decodeParams(field string) (Params, error) {
	var (
		p    Params
		seen = map[string]bool{}
	)
	for _, pair := range strings.Split(field, ",") {
		name, value, found := strings.Cut(pair, "=")
		if !found || seen[name] {
			return Params{}, fmt.Errorf("%w: parameters are not name=value pairs", ErrMalformedHash)
		}
		seen[name] = true

		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return Params{}, fmt.Errorf("%w: parameter %q is not a number", ErrMalformedHash, name)
		}
		switch name {
		case "m":
			p.Memory = uint32(n)
		case "t":
			p.Time = uint32(n)
		case "p":
			if n == 0 || n > 255 {
				return Params{}, fmt.Errorf("%w: parallelism out of range", ErrMalformedHash)
			}
			p.Parallelism = uint8(n)
		default:
			return Params{}, fmt.Errorf("%w: unknown parameter %q", ErrMalformedHash, name)
		}
	}
	if !seen["m"] || !seen["t"] || !seen["p"] {
		return Params{}, fmt.Errorf("%w: missing one of m, t, p", ErrMalformedHash)
	}
	if p.Memory == 0 || p.Time == 0 {
		return Params{}, fmt.Errorf("%w: zero memory or time", ErrMalformedHash)
	}
	if p.Memory > maxStoredMemory {
		// Verifying allocates whatever the string asks for. A stored hash
		// demanding more memory than any setting here would produce is more
		// likely damaged than genuine, and hashing it would let one login
		// exhaust the host.
		return Params{}, fmt.Errorf("%w: memory parameter is implausibly large", ErrMalformedHash)
	}
	return p, nil
}
