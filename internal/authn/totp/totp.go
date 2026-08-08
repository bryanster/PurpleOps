// Package totp is the time-based one-time password algorithm and nothing else:
// minting an enrolment, and deciding whether six digits are the right six
// digits at a given moment.
//
// It holds no state, touches no database and knows nothing about users or
// sessions. What it returns instead is the *step* a code belonged to, which is
// what makes replay protection possible one layer up: the caller remembers the
// last step it accepted and refuses anything at or before it. RFC 6238 has
// nothing to say about that, and neither do the libraries — it is the half
// everybody has to write, so it is written here where it can be tested with a
// clock rather than a sleep.
//
// The parameters are fixed, not configurable. Every authenticator app in
// circulation implements SHA-1, six digits and thirty seconds; the others are a
// compatibility problem in exchange for no security a longer secret does not
// already give.
package totp

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// Period is the length of a step. Thirty seconds is what the URI's
	// `period` says and what every app assumes when it is absent.
	Period = 30 * time.Second

	// Digits is the length of a code.
	Digits = otp.DigitsSix

	// Algorithm is the HMAC behind it. SHA-1 here is not a hash of anything an
	// attacker can attack: HOTP uses it as a PRF over a counter under a
	// 160-bit key, where its collision weakness does not apply, and it is the
	// only algorithm the installed base agrees on.
	Algorithm = otp.AlgorithmSHA1

	// secretBytes is 20, the size RFC 4226 §4 requires and the size every app
	// expects to be handed. Base32 of 20 bytes is the 32-character string
	// somebody can type in when the camera will not focus.
	secretBytes = 20

	// Skew is how many steps either side of the current one are accepted. One
	// step is thirty seconds of tolerance in each direction, which covers a
	// phone whose clock has drifted and a person who typed slowly. Two would
	// double the window an attacker gets to guess in for no case anybody has.
	Skew = 1

	// qrPixels is the size of the rendered QR code. It is scaled here rather
	// than in CSS so that the image is not resampled from something smaller,
	// which is how a QR code becomes one a camera cannot read.
	qrPixels = 256
)

// ErrNoMatch reports that a code is not one this secret produces in the
// accepted window — wrong digits, too old, too new, or already spent.
//
// One error for all four, because the caller answers them identically: telling
// somebody that their code was *right but stale* narrows the search for anybody
// who is guessing.
var ErrNoMatch = errors.New("totp: the code does not match")

// Enrolment is a newly minted authenticator, as the person setting it up needs
// to see it.
type Enrolment struct {
	// Secret is the base32 shared secret. It is the only field here that must
	// never be stored in the clear or logged, and it leaves this struct exactly
	// twice: into the cipher, and into URI below.
	Secret string

	// URI is the otpauth:// URI an authenticator app consumes.
	URI string

	// QRCode is URI rendered as a PNG in a data: URI, ready for an <img src>.
	// A data URI rather than SVG markup so that a client displays it without
	// putting server-supplied markup into its DOM, and because the page's
	// content security policy already allows `img-src data:`.
	QRCode string
}

// Generate mints an enrolment for one account.
//
// issuer is the deployment, and appears in the app as the name of the entry;
// account is the person, and is what tells two entries from the same deployment
// apart. Neither may contain a colon: the label is `issuer:account`, and a
// colon inside either half moves the boundary.
func Generate(issuer, account string) (Enrolment, error) {
	switch {
	case strings.TrimSpace(issuer) == "":
		return Enrolment{}, errors.New("totp: no issuer; the app would show an unnamed entry")
	case strings.TrimSpace(account) == "":
		return Enrolment{}, errors.New("totp: no account name; two entries would be indistinguishable")
	case strings.ContainsRune(issuer, ':') || strings.ContainsRune(account, ':'):
		return Enrolment{}, fmt.Errorf("totp: the issuer %q and account %q must not contain a colon, "+
			"which separates them in the otpauth label", issuer, account)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      uint(Period / time.Second),
		SecretSize:  secretBytes,
		Digits:      Digits,
		Algorithm:   Algorithm,
	})
	if err != nil {
		return Enrolment{}, fmt.Errorf("totp: generate a key: %w", err)
	}

	qr, err := qrDataURI(key)
	if err != nil {
		return Enrolment{}, err
	}
	return Enrolment{Secret: key.Secret(), URI: key.URL(), QRCode: qr}, nil
}

// Step is the TOTP time step a moment falls in: Unix seconds divided by the
// period. It is exported because it is what a caller stores as "the last code I
// accepted", and a caller that computed it a second way could compute it wrong.
func Step(at time.Time) int64 { return at.Unix() / int64(Period/time.Second) }

// Validate reports which step a code belongs to.
//
// It tries the current step and [Skew] either side, oldest first, and returns
// the first that matches — oldest first so that a code presented at the edge of
// its window is attributed to the step it was generated in rather than to a
// later one, which would spend a step that has not happened yet.
//
// after is the last step this secret has already accepted; a match at or before
// it is a replay and is refused as [ErrNoMatch]. Pass 0 for a secret that has
// never accepted one. The caller is responsible for storing the returned step,
// and for doing it in a way two concurrent verifications cannot both win — see
// identity.TOTPs.Accept.
//
// The comparison is constant-time. A code is six digits and an attacker gets a
// small number of guesses, so a timing side channel is not the likely way in;
// it costs one function call to not have to argue about that.
func Validate(secret, code string, at time.Time, after int64) (int64, error) {
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return 0, fmt.Errorf("%w: the secret or the code is empty", ErrNoMatch)
	}

	now := Step(at)
	for step := now - Skew; step <= now+Skew; step++ {
		if step <= after {
			// Spent, or older than something already spent. Nothing about this
			// step can make it acceptable, so it is skipped rather than
			// compared — and the skip is on the step, not on the code, so it
			// leaks nothing about whether the digits were right.
			continue
		}
		expected, err := totp.GenerateCodeCustom(secret, stepTime(step), totp.ValidateOpts{
			Period:    uint(Period / time.Second),
			Digits:    Digits,
			Algorithm: Algorithm,
		})
		if err != nil {
			// The stored secret is not base32. That is a damaged row, not a
			// failed code, and reporting it as one would leave nobody looking
			// at it.
			return 0, fmt.Errorf("totp: generate the expected code: %w", err)
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, nil
		}
	}
	return 0, ErrNoMatch
}

// stepTime returns a moment inside a step, which is what the library's
// generator takes instead of the step itself.
func stepTime(step int64) time.Time {
	return time.Unix(step*int64(Period/time.Second), 0).UTC()
}

// qrDataURI renders a key as a PNG data URI.
func qrDataURI(key *otp.Key) (string, error) {
	image, err := key.Image(qrPixels, qrPixels)
	if err != nil {
		return "", fmt.Errorf("totp: render the QR code: %w", err)
	}

	// Encoded straight into the builder rather than through an intermediate
	// []byte: the PNG is a few kilobytes and this saves holding two copies of
	// it, which matters not at all — but it also means there is no plain-bytes
	// variable for a later change to log.
	var out strings.Builder
	out.WriteString("data:image/png;base64,")
	encoder := base64.NewEncoder(base64.StdEncoding, &out)
	if err := png.Encode(encoder, image); err != nil {
		return "", fmt.Errorf("totp: encode the QR code as PNG: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("totp: finish encoding the QR code: %w", err)
	}
	return out.String(), nil
}
