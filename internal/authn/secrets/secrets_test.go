package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// A key of the shape an operator would produce with `openssl rand -base64 32`.
const testKey = "9Qd3JmE7uZpA0xTnCiL5wHrYbVsK2fGoP4jXeM8tUcR="

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()

	cipher, err := New([]byte(testKey))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return cipher
}

func TestSealAndOpenRoundTrip(t *testing.T) {
	t.Parallel()

	cipher := newTestCipher(t)
	want := []byte("JBSWY3DPEHPK3PXP")

	sealed, err := cipher.Seal(want)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if strings.Contains(sealed, string(want)) {
		t.Error("the sealed value contains the plaintext")
	}

	got, err := cipher.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Open(Seal(%q)) = %q", want, got)
	}
}

// TestSealingTheSameValueTwiceDiffers is the nonce test the ticket asks for.
// Two seals of one plaintext that matched would mean a fixed nonce, which under
// GCM is not weakened encryption but broken encryption — it leaks the XOR of
// the two messages and, worse, the authentication key.
func TestSealingTheSameValueTwiceDiffers(t *testing.T) {
	t.Parallel()

	cipher := newTestCipher(t)

	first, err := cipher.Seal([]byte("the same secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	second, err := cipher.Seal([]byte("the same secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if first == second {
		t.Fatal("two seals of one plaintext are identical, so the nonce is not fresh per record")
	}

	// Both still open, which is what says the difference is the nonce and not
	// something that lost the plaintext.
	for _, sealed := range []string{first, second} {
		got, err := cipher.Open(sealed)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if string(got) != "the same secret" {
			t.Errorf("Open = %q", got)
		}
	}
}

func TestOpenRefusesWhatThisKeyDidNotSeal(t *testing.T) {
	t.Parallel()

	cipher := newTestCipher(t)
	sealed, err := cipher.Seal([]byte("a TOTP secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	other, err := New([]byte("kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU="))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name   string
		cipher *Cipher
		value  string
	}{
		{"a different key", other, sealed},
		{"not base64", cipher, "not base64 at all!"},
		{"shorter than a nonce", cipher, base64.RawURLEncoding.EncodeToString([]byte("short"))},
		{"an altered ciphertext", cipher, flipLastChar(sealed)},
		{"empty", cipher, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tt.cipher.Open(tt.value); !errors.Is(err, ErrCorrupt) {
				t.Errorf("Open(%s) = %v, want ErrCorrupt — every way of not being this key's "+
					"value is one answer", tt.name, err)
			}
		})
	}
}

// TestARotatedKeyCannotReadTheOldValues is the operational consequence the
// documentation promises, asserted rather than only written down: changing
// BLACKLIGHT_ENCRYPTION_KEY makes every enrolled authenticator unreadable.
func TestARotatedKeyCannotReadTheOldValues(t *testing.T) {
	t.Parallel()

	before := newTestCipher(t)
	sealed, err := before.Seal([]byte("JBSWY3DPEHPK3PXP"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	after, err := New([]byte("Lp4Wq8Zn2Cv6Bx0Fj5Th9Rd3Ks7Gm1Ay4Ue6Io8=="))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := after.Open(sealed); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("a rotated key opened a value sealed under the old one (err = %v)", err)
	}
}

func TestNewRefusesKeyMaterialThatIsTooShort(t *testing.T) {
	t.Parallel()

	// 31 bytes: one short, and long enough that HKDF would happily stretch it
	// into a well-formed key nobody would have to guess very hard.
	if _, err := New(bytes.Repeat([]byte("a"), keyBytes-1)); err == nil {
		t.Fatal("New accepted 31 bytes of key material, want an error")
	}
}

// TestTheZeroCipherEncryptsNothing: a Cipher that was never constructed must
// fail loudly rather than write a value it cannot read back.
func TestTheZeroCipherEncryptsNothing(t *testing.T) {
	t.Parallel()

	var zero Cipher
	if _, err := zero.Seal([]byte("secret")); err == nil {
		t.Error("the zero Cipher sealed a value")
	}
	if _, err := zero.Open("anything"); err == nil {
		t.Error("the zero Cipher opened a value")
	}
}

// flipLastChar changes one character of a base64url string to a different legal
// one, which is an edit GCM's tag has to catch.
func flipLastChar(s string) string {
	last := s[len(s)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	return s[:len(s)-1] + string(replacement)
}
