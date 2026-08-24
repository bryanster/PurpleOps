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

	// A sealed value is a nonce, then the ciphertext, then GCM's tag. Editing
	// any of the three has to be refused, so each gets a case: the tag is what
	// catches the first two, and the tag catching an edit to itself is what
	// stops an attacker simply recomputing it.
	raw, err := encoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decoding a value Seal produced: %v", err)
	}

	tests := []struct {
		name   string
		cipher *Cipher
		value  string
	}{
		{"a different key", other, sealed},
		{"not base64", cipher, "not base64 at all!"},
		{"shorter than a nonce", cipher, base64.RawURLEncoding.EncodeToString([]byte("short"))},
		{"an altered nonce", cipher, flipByte(t, sealed, 0)},
		{"an altered ciphertext", cipher, flipByte(t, sealed, cipher.aead.NonceSize())},
		{"an altered tag", cipher, flipByte(t, sealed, len(raw)-1)},
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

// TestEachPurposeGetsItsOwnKey is what [NewFor] is for: two uses of the same
// configured key material must not be able to read each other's values, so a
// mistake in one cannot become a way into the other.
func TestEachPurposeGetsItsOwnKey(t *testing.T) {
	t.Parallel()

	key := []byte("kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU=")

	totp, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	state, err := NewFor(key, "oidc-state")
	if err != nil {
		t.Fatalf("NewFor: %v", err)
	}
	other, err := NewFor(key, "something-else")
	if err != nil {
		t.Fatalf("NewFor: %v", err)
	}

	sealed, err := state.Seal([]byte("the pending single sign-on"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if opened, err := state.Open(sealed); err != nil || string(opened) != "the pending single sign-on" {
		t.Fatalf("Open under its own key = %q, %v", opened, err)
	}
	for name, cipher := range map[string]*Cipher{"the default purpose": totp, "another purpose": other} {
		if _, err := cipher.Open(sealed); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s opened a value sealed for oidc-state (err = %v)", name, err)
		}
	}
}

func TestNewForRefusesAnUnnamedPurpose(t *testing.T) {
	t.Parallel()

	if _, err := NewFor([]byte("kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU="), "  "); err == nil {
		t.Fatal("NewFor accepted an empty purpose, want an error")
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

// flipByte returns sealed with one bit of its index-th decoded byte inverted
// and the result re-encoded: an edit GCM's tag has to catch.
//
// It edits the decoded bytes. Editing a character of the base64 instead — which
// is what this helper did until it was found to be flaky — does not reliably
// edit anything at all. base64url is lenient about the bits at the end of a
// string that no byte uses: when the payload's length is not a multiple of
// three, its final character carries only two or four significant bits and the
// rest are ignored on the way back in. Replacing that character left the
// decoded bytes identical whenever its significant bits already matched the
// replacement's — four values in sixty-four for a sealed 13-byte secret, so
// about one run in sixteen — and then Open succeeded and the test failed over
// base64 rather than over GCM.
func flipByte(t *testing.T, sealed string, index int) string {
	t.Helper()

	raw, err := encoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decoding a value Seal produced: %v", err)
	}
	if index < 0 || index >= len(raw) {
		t.Fatalf("byte %d is outside a %d-byte sealed value", index, len(raw))
	}

	// One bit, not a whole byte: the smallest edit the tag has to notice.
	raw[index] ^= 1
	return encoding.EncodeToString(raw)
}
