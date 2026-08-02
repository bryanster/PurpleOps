// Package secrets encrypts the values this server stores on somebody else's
// behalf — today the TOTP shared secrets of M1-006, later an OIDC client secret
// or an SMTP password.
//
// It exists because those values are not credentials this server checks, they
// are credentials it *holds*: it has to be able to read them back, so hashing
// them is not an option and a copy of the database would otherwise be a set of
// working authenticators. Everything here is the standard construction with no
// choices left to a caller — AES-256-GCM, a fresh nonce per record, one key
// derived once at startup — because the ways to get authenticated encryption
// wrong are all in the parameters.
//
// The key comes from BLACKLIGHT_ENCRYPTION_KEY and deliberately not from
// BLACKLIGHT_SESSION_SECRET; internal/config says why, and docs/security.md says
// it again for the operator who has to keep both.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// keyBytes is 32: AES-256. The configured key material is run through HKDF to
// get exactly this many bytes, so an operator's 44-character base64 string and
// their colleague's 60-character passphrase both produce a key of the right
// size rather than one of them failing at startup.
const keyBytes = 32

// derivationInfo binds the derived key to this use. A second thing encrypted
// with the same configured value would pass its own label and get a different
// key, so a ciphertext from one can never be opened by the other — which is
// what stops a bug in a future caller turning into a way to read TOTP secrets.
const derivationInfo = "blacklight/secrets/aes-256-gcm/v1"

// encoding is base64url without padding, matching how every other opaque value
// in this tree is spelled in a TEXT column.
var encoding = base64.RawURLEncoding

// ErrCorrupt reports a value that cannot be opened: not this key's, not this
// construction's, or altered since it was written. The three are one error on
// purpose — nothing above this package can act differently on the difference,
// and the distinction is exactly what a padding-oracle attack asks for.
var ErrCorrupt = errors.New("secrets: the value cannot be decrypted")

// Cipher seals and opens values under one key. It is safe for concurrent use,
// which is how it is used: one per process, shared by every request.
//
// The zero value has no key and will not encrypt anything; construct it with
// [New].
type Cipher struct {
	aead cipher.AEAD
}

// New returns a Cipher over the given key material, or an error describing why
// the material cannot produce one.
//
// key is whatever the operator configured, not an AES key: it is stretched to
// 32 bytes by HKDF-SHA256. The length check is on the input, because HKDF will
// happily produce a well-formed key from four bytes of entropy and the result
// would be a well-formed key nobody has to guess very hard.
func New(key []byte) (*Cipher, error) {
	if len(key) < keyBytes {
		return nil, fmt.Errorf("secrets: the encryption key carries %d bytes, want at least %d",
			len(key), keyBytes)
	}

	// No salt: there is one key and it is already high-entropy, so the salt
	// would be a constant with nothing to do. The info string is what separates
	// this derivation from any other.
	derived, err := hkdf.Key(sha256.New, key, nil, derivationInfo, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("secrets: derive the encryption key: %w", err)
	}
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, fmt.Errorf("secrets: build the cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: build the AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext and returns the value to store: the nonce, then the
// ciphertext and its tag, base64url.
//
// The nonce is 12 fresh bytes from crypto/rand for every call, so sealing the
// same plaintext twice produces two different values — TestSealingTwiceDiffers
// asserts it. Reusing a nonce under GCM is not a weakened encryption, it is a
// broken one: two messages under the same nonce leak their XOR and, worse, the
// authentication key. That is the whole reason a caller is given no way to
// supply one.
func (c *Cipher) Seal(plaintext []byte) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("secrets: no cipher; nothing can be encrypted")
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		// Not recoverable and never to be papered over with a weaker source:
		// every value this process went on to seal would share a nonce space
		// somebody can predict.
		return "", fmt.Errorf("secrets: read %d random bytes: %w", len(nonce), err)
	}

	// The nonce is prepended to the ciphertext rather than stored beside it: it
	// is not a secret, it must never be lost, and one column cannot have half of
	// it.
	sealed := c.aead.Seal(nonce, nonce, plaintext, nil)
	return encoding.EncodeToString(sealed), nil
}

// Open reverses [Cipher.Seal]. Anything that is not a value this key sealed —
// including one that has been edited in the database — is [ErrCorrupt].
func (c *Cipher) Open(sealed string) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, errors.New("secrets: no cipher; nothing can be decrypted")
	}

	raw, err := encoding.DecodeString(sealed)
	if err != nil {
		return nil, fmt.Errorf("%w: it is not base64url", ErrCorrupt)
	}
	if len(raw) < c.aead.NonceSize() {
		return nil, fmt.Errorf("%w: it is shorter than a nonce", ErrCorrupt)
	}

	nonce, ciphertext := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// GCM's own error says only "message authentication failed", which is
		// all a caller should learn: whether the key is wrong or the bytes were
		// altered is not a distinction worth answering. It is wrapped so that a
		// log can still show what the AEAD said.
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	return plaintext, nil
}
