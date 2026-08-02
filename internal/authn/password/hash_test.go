package password_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/password"
)

// correct is the password the round-trip tests use. It is a policy-legal
// passphrase, so these tests exercise the same input the login path will.
const correct = password.Plaintext("correct battery horse staple")

func TestHashingTheSamePasswordTwiceProducesDifferentHashes(t *testing.T) {
	t.Parallel()

	first, err := password.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}
	second, err := password.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}

	// The salt is per-hash. If these ever match, every user with the same
	// password shares a hash and the table tells an attacker who to target.
	if first == second {
		t.Fatal("hashing one password twice produced the same string, so the salt is not random")
	}

	for _, encoded := range []string{first, second} {
		ok, needsRehash, err := password.Verify(correct, encoded)
		if err != nil {
			t.Fatalf("Verify() error = %v, want nil", err)
		}
		if !ok {
			t.Error("Verify() = false for the password that was hashed")
		}
		if needsRehash {
			t.Error("Verify() asked for a rehash of a hash just made with the current parameters")
		}
	}
}

func TestAHashIsInPHCFormatAndCarriesItsParameters(t *testing.T) {
	t.Parallel()

	encoded, err := password.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}

	// The prefix is what tells any other tool — or a future version of this
	// package — how to read the rest.
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Errorf("hash = %q, want it to start with $argon2id$v=19$", encoded)
	}

	stored, salt, key, err := password.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() = %v, want nil", err)
	}
	if want := password.Default(); stored != want {
		t.Errorf("stored parameters = %+v, want %+v", stored, want)
	}
	if len(salt) != int(password.Default().SaltLength) {
		t.Errorf("salt is %d bytes, want %d", len(salt), password.Default().SaltLength)
	}
	if len(key) != int(password.Default().KeyLength) {
		t.Errorf("key is %d bytes, want %d", len(key), password.Default().KeyLength)
	}
}

func TestTheWrongPasswordIsFalseAndNotAnError(t *testing.T) {
	t.Parallel()

	encoded, err := password.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}

	wrong := []password.Plaintext{
		"correct battery horse stapl",  // one character short
		correct + " ",                  // the right one, with a trailing space
		"Correct battery horse staple", // the right one, in a different case
		"",
		password.Plaintext(strings.Repeat("x", password.MaxPlaintextBytes+1)),
	}

	for _, plaintext := range wrong {
		ok, needsRehash, err := password.Verify(plaintext, encoded)
		if err != nil {
			// A failed login is not an error: an error here would push the
			// login handler towards a 500 for a mistyped password.
			t.Errorf("Verify(%v) error = %v, want nil", plaintext, err)
		}
		if ok {
			t.Errorf("Verify(%v) = true, want false", plaintext)
		}
		if needsRehash {
			t.Errorf("Verify(%v) asked for a rehash after a failed verification", plaintext)
		}
	}
}

func TestAMalformedStoredHashIsAnError(t *testing.T) {
	t.Parallel()

	valid, err := password.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}
	fields := strings.Split(valid, "$")

	tests := map[string]string{
		"empty":              "",
		"not PHC at all":     "hunter2",
		"bcrypt":             "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		"too few fields":     "$argon2id$v=19$m=65536,t=3,p=1$" + fields[4],
		"too many fields":    valid + "$extra",
		"argon2i not id":     "$argon2i$" + strings.Join(fields[2:], "$"),
		"no version":         "$argon2id$m=65536,t=3,p=1$" + strings.Join(fields[4:], "$"),
		"unknown version":    "$argon2id$v=16$" + strings.Join(fields[3:], "$"),
		"unreadable version": "$argon2id$v=nineteen$" + strings.Join(fields[3:], "$"),
		"missing t":          "$argon2id$v=19$m=65536,p=1$" + strings.Join(fields[4:], "$"),
		"repeated m":         "$argon2id$v=19$m=65536,m=8,t=3,p=1$" + strings.Join(fields[4:], "$"),
		"unknown parameter":  "$argon2id$v=19$m=65536,t=3,p=1,z=9$" + strings.Join(fields[4:], "$"),
		"non-numeric m":      "$argon2id$v=19$m=lots,t=3,p=1$" + strings.Join(fields[4:], "$"),
		"zero memory":        "$argon2id$v=19$m=0,t=3,p=1$" + strings.Join(fields[4:], "$"),
		"zero parallelism":   "$argon2id$v=19$m=65536,t=3,p=0$" + strings.Join(fields[4:], "$"),
		"absurd memory":      "$argon2id$v=19$m=4294967295,t=3,p=1$" + strings.Join(fields[4:], "$"),
		"salt not base64":    "$argon2id$v=19$m=65536,t=3,p=1$not base64!$" + fields[5],
		"key not base64":     "$argon2id$v=19$m=65536,t=3,p=1$" + fields[4] + "$not base64!",
		"empty salt":         "$argon2id$v=19$m=65536,t=3,p=1$$" + fields[5],
		"one-byte key":       "$argon2id$v=19$m=65536,t=3,p=1$" + fields[4] + "$AA",
	}

	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ok, needsRehash, err := password.Verify(correct, encoded)
			if !errors.Is(err, password.ErrMalformedHash) {
				t.Errorf("Verify() error = %v, want ErrMalformedHash", err)
			}
			if ok || needsRehash {
				t.Errorf("Verify() = (%t, %t), want (false, false)", ok, needsRehash)
			}
			// Nothing about a stored credential belongs in an error string,
			// even a damaged one: this value reaches a log.
			if message := err.Error(); strings.Contains(message, fields[5]) {
				t.Errorf("error message %q contains the stored hash", message)
			}
		})
	}
}

func TestAHashMadeWithWeakerParametersVerifiesAndAsksToBeReplaced(t *testing.T) {
	t.Parallel()

	current := password.Default()
	weaker := map[string]password.Params{
		"less memory":    {Memory: current.Memory / 4, Time: current.Time, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		"fewer passes":   {Memory: current.Memory, Time: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		"shorter salt":   {Memory: current.Memory, Time: current.Time, Parallelism: 1, SaltLength: 8, KeyLength: 32},
		"shorter key":    {Memory: current.Memory, Time: current.Time, Parallelism: 1, SaltLength: 16, KeyLength: 16},
		"more lanes":     {Memory: current.Memory, Time: current.Time, Parallelism: 4, SaltLength: 16, KeyLength: 32},
		"v1 era default": {Memory: 4096, Time: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16},
	}

	for name, params := range weaker {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			encoded, err := params.Hash(correct)
			if err != nil {
				t.Fatalf("Hash() = %v, want nil", err)
			}

			ok, needsRehash, err := password.Verify(correct, encoded)
			if err != nil {
				t.Fatalf("Verify() error = %v, want nil", err)
			}
			// Verifying must use the parameters in the string, not today's, or
			// raising a cost would lock every existing account out.
			if !ok {
				t.Error("Verify() = false for a hash made under older parameters")
			}
			if !needsRehash {
				t.Error("Verify() needsRehash = false, want true so M1-003 upgrades it on login")
			}

			// And the wrong password is still wrong, whatever it was hashed with.
			if ok, _, err := password.Verify("something else entirely", encoded); ok || err != nil {
				t.Errorf("Verify(wrong) = (%t, _, %v), want (false, _, nil)", ok, err)
			}
		})
	}
}

func TestAStrongerStoredHashIsLeftAlone(t *testing.T) {
	t.Parallel()

	current := password.Default()
	stronger := current
	stronger.Memory *= 2
	stronger.Time++

	encoded, err := stronger.Hash(correct)
	if err != nil {
		t.Fatalf("Hash() = %v, want nil", err)
	}

	ok, needsRehash, err := password.Verify(correct, encoded)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if !ok {
		t.Error("Verify() = false for a hash made under stronger parameters")
	}
	// Downgrading somebody's hash because an operator lowered the cost is not
	// an upgrade, and re-hashing costs a login's latency for nothing.
	if needsRehash {
		t.Error("Verify() asked to replace a hash that is stronger than the current setting")
	}
}

func TestAnAbsurdlyLongPlaintextIsRefusedRatherThanHashed(t *testing.T) {
	t.Parallel()

	// Well past anything Validate would allow, and past what this package will
	// copy into a hashing call.
	huge := password.Plaintext(strings.Repeat("a", password.MaxPlaintextBytes+1))

	if _, err := password.Hash(huge); !errors.Is(err, password.ErrTooLong) {
		t.Errorf("Hash(huge) error = %v, want ErrTooLong", err)
	}
	// Nothing is truncated on the way in: a password one byte over the limit is
	// refused, not silently shortened to the limit.
	atLimit := password.Plaintext(strings.Repeat("a", password.MaxPlaintextBytes))
	encoded, err := password.Hash(atLimit)
	if err != nil {
		t.Fatalf("Hash(at limit) = %v, want nil", err)
	}
	if ok, _, err := password.Verify(huge, encoded); ok || err != nil {
		t.Errorf("Verify(over the limit) = (%t, _, %v), want (false, _, nil)", ok, err)
	}
}

func TestParametersThatCannotHashAreAnErrorAndNotAPanic(t *testing.T) {
	t.Parallel()

	current := password.Default()
	invalid := map[string]password.Params{
		"the zero value":                  {},
		"no passes":                       {Memory: current.Memory, Time: 0, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		"no lanes":                        {Memory: current.Memory, Time: current.Time, Parallelism: 0, SaltLength: 16, KeyLength: 32},
		"too little memory for the lanes": {Memory: 16, Time: 1, Parallelism: 4, SaltLength: 16, KeyLength: 32},
		"no salt":                         {Memory: current.Memory, Time: 1, Parallelism: 1, SaltLength: 0, KeyLength: 32},
		"a two-byte key":                  {Memory: current.Memory, Time: 1, Parallelism: 1, SaltLength: 16, KeyLength: 2},
		"a huge key":                      {Memory: current.Memory, Time: 1, Parallelism: 1, SaltLength: 16, KeyLength: 1 << 20},
	}

	for name, params := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// argon2.IDKey panics on zero rounds and zero lanes; a panic in a
			// login handler is a 500 with a stack trace, so these are errors.
			encoded, err := params.Hash(correct)
			if !errors.Is(err, password.ErrInvalidParams) {
				t.Errorf("Hash() error = %v, want ErrInvalidParams", err)
			}
			if encoded != "" {
				t.Errorf("Hash() = %q, want no hash alongside an error", encoded)
			}
		})
	}
}

// BenchmarkHash measures one hash at the current [password.Default] settings.
// M1-002 wants roughly 100–500 ms: below that the parameters are too cheap to
// slow an offline attacker, above it a login is slow and a burst of logins is a
// way to exhaust the server.
//
// Run it on the target hardware before changing Default:
//
//	go test ./internal/authn/password -run '^$' -bench BenchmarkHash -benchtime 20x
func BenchmarkHash(b *testing.B) {
	for b.Loop() {
		if _, err := password.Hash(correct); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVerify is the number that matters for login latency, and should be
// within noise of BenchmarkHash — the derivation is the same work.
func BenchmarkVerify(b *testing.B) {
	encoded, err := password.Hash(correct)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for b.Loop() {
		ok, _, err := password.Verify(correct, encoded)
		if err != nil || !ok {
			b.Fatalf("Verify() = (%t, _, %v)", ok, err)
		}
	}
}
