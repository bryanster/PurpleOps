package totp

import (
	"encoding/base32"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// at is the moment every test in this file pretends it is. It is a fixed point
// rather than time.Now so that "the current step" is the same number on every
// machine and every run — the whole reason [Validate] takes a clock.
var at = time.Date(2026, 3, 14, 15, 9, 26, 0, time.UTC)

// codeAt produces the code an authenticator would show at a given step, which
// is what a test has to have in order to present one.
func codeAt(t *testing.T, secret string, step int64) string {
	t.Helper()

	code, err := totp.GenerateCodeCustom(secret, stepTime(step), totp.ValidateOpts{
		Period:    uint(Period / time.Second),
		Digits:    Digits,
		Algorithm: Algorithm,
	})
	if err != nil {
		t.Fatalf("generating a code for step %d: %v", step, err)
	}
	return code
}

func newSecret(t *testing.T) string {
	t.Helper()

	enrolment, err := Generate("PurpleOps (test.internal)", "alice@example.com")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return enrolment.Secret
}

// TestValidateAcceptsTheWindowAndNothingElse is the skew table the ticket asks
// for: one step either side is accepted, two is not, and what comes back is the
// step the code belonged to rather than a bare yes.
func TestValidateAcceptsTheWindowAndNothingElse(t *testing.T) {
	t.Parallel()

	secret := newSecret(t)
	now := Step(at)

	tests := []struct {
		name   string
		step   int64
		accept bool
	}{
		{"two steps behind", now - 2, false},
		{"one step behind", now - 1, true},
		{"the current step", now, true},
		{"one step ahead", now + 1, true},
		{"two steps ahead", now + 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Validate(secret, codeAt(t, secret, tt.step), at, 0)
			switch {
			case tt.accept && err != nil:
				t.Fatalf("Validate = %v, want the code accepted at step %d", err, tt.step)
			case tt.accept && got != tt.step:
				t.Errorf("Validate = step %d, want %d — the step decides the replay window", got, tt.step)
			case !tt.accept && !errors.Is(err, ErrNoMatch):
				t.Errorf("Validate = (%d, %v), want ErrNoMatch: ±1 step is the whole tolerance", got, err)
			}
		})
	}
}

// TestValidateRefusesASpentStep is replay protection. The library has no
// opinion about this, which is why the window is ours to implement and ours to
// test.
func TestValidateRefusesASpentStep(t *testing.T) {
	t.Parallel()

	secret := newSecret(t)
	now := Step(at)
	code := codeAt(t, secret, now)

	step, err := Validate(secret, code, at, 0)
	if err != nil {
		t.Fatalf("the first use of a code was refused: %v", err)
	}

	// The same code, inside its own thirty seconds, against a caller that has
	// recorded the step. This is the replay.
	if _, err := Validate(secret, code, at, step); !errors.Is(err, ErrNoMatch) {
		t.Errorf("Validate replayed a code from step %d, err = %v", step, err)
	}

	// And the step before it, which an attacker who saw an earlier code holds.
	previous := codeAt(t, secret, now-1)
	if _, err := Validate(secret, previous, at, step); !errors.Is(err, ErrNoMatch) {
		t.Errorf("Validate accepted a code from before the last spent step, err = %v", err)
	}

	// The next one still works: spending a step must not end the enrolment.
	next := at.Add(Period)
	if _, err := Validate(secret, codeAt(t, secret, Step(next)), next, step); err != nil {
		t.Errorf("the next code was refused after a spend: %v", err)
	}
}

func TestValidateRefusesTheObviouslyWrong(t *testing.T) {
	t.Parallel()

	secret := newSecret(t)

	tests := map[string]struct{ secret, code string }{
		"a wrong code":  {secret, "000000"},
		"an empty code": {secret, ""},
		"no secret":     {"", codeAt(t, secret, Step(at))},
		"another secret's code": {
			secret, codeAt(t, newSecret(t), Step(at)),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Validate(tt.secret, tt.code, at, 0); !errors.Is(err, ErrNoMatch) {
				t.Errorf("Validate(%s) = %v, want ErrNoMatch", name, err)
			}
		})
	}
}

// TestValidateToleratesSurroundingWhitespace: some apps show "492 817", and a
// person who copies one should not be told their code is wrong.
func TestValidateToleratesSurroundingWhitespace(t *testing.T) {
	t.Parallel()

	secret := newSecret(t)
	code := codeAt(t, secret, Step(at))

	if _, err := Validate(secret, " "+code+"\n", at, 0); err != nil {
		t.Errorf("Validate refused a code with surrounding whitespace: %v", err)
	}
}

// TestGenerateProducesAURIAnAppCanRead checks the parts an authenticator
// actually reads. The acceptance criterion is that it scans; what a test can
// assert is that every field it scans for is there and correct.
func TestGenerateProducesAURIAnAppCanRead(t *testing.T) {
	t.Parallel()

	const (
		issuer  = "PurpleOps (purpleops.internal)"
		account = "alice@example.com"
	)

	enrolment, err := Generate(issuer, account)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	parsed, err := url.Parse(enrolment.URI)
	if err != nil {
		t.Fatalf("parsing %q: %v", enrolment.URI, err)
	}
	if got, want := parsed.Scheme+"://"+parsed.Host, "otpauth://totp"; got != want {
		t.Errorf("the URI starts %q, want %q", got, want)
	}
	// The label is issuer:account, which is what puts the two names on the
	// entry an app creates.
	if got, want := parsed.Path, "/"+issuer+":"+account; got != want {
		t.Errorf("label = %q, want %q", got, want)
	}

	query := parsed.Query()
	for field, want := range map[string]string{
		"issuer":    issuer,
		"algorithm": "SHA1",
		"digits":    "6",
		"period":    "30",
		"secret":    enrolment.Secret,
	} {
		if got := query.Get(field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}

	// A secret an app can decode, of the size RFC 4226 asks for.
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrolment.Secret)
	if err != nil {
		t.Fatalf("the secret is not base32: %v", err)
	}
	if len(raw) != secretBytes {
		t.Errorf("the secret is %d bytes, want %d", len(raw), secretBytes)
	}

	if !strings.HasPrefix(enrolment.QRCode, "data:image/png;base64,") {
		t.Errorf("the QR code is not a PNG data URI: %.40q", enrolment.QRCode)
	}
	if len(enrolment.QRCode) < 200 {
		t.Errorf("the QR code is %d characters, which is too short to be an image", len(enrolment.QRCode))
	}
}

func TestGenerateProducesADifferentSecretEveryTime(t *testing.T) {
	t.Parallel()

	first, second := newSecret(t), newSecret(t)
	if first == second {
		t.Fatal("two enrolments share a secret")
	}
}

// TestGenerateRefusesALabelThatWouldBeAmbiguous: a colon in either half moves
// the boundary between them, so an app would show a different account than the
// one being enrolled.
func TestGenerateRefusesALabelThatWouldBeAmbiguous(t *testing.T) {
	t.Parallel()

	tests := map[string][2]string{
		"a colon in the issuer":  {"Purple:Ops", "alice@example.com"},
		"a colon in the account": {"PurpleOps", "alice:example.com"},
		"no issuer":              {"", "alice@example.com"},
		"no account":             {"PurpleOps", "  "},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Generate(args[0], args[1]); err == nil {
				t.Errorf("Generate(%q, %q) succeeded, want an error", args[0], args[1])
			}
		})
	}
}

func TestStepAdvancesOncePerPeriod(t *testing.T) {
	t.Parallel()

	// Measured from the start of a step, because that is the only place where
	// "a second short of a period" is unambiguous — anywhere else in the step,
	// a second short of thirty crosses the boundary and is supposed to.
	start := stepTime(Step(at))

	if got, want := Step(start.Add(Period))-Step(start), int64(1); got != want {
		t.Errorf("a period advanced the step by %d, want %d", got, want)
	}
	if got, want := Step(start.Add(Period-time.Second)), Step(start); got != want {
		t.Errorf("a second short of a period changed the step (%d vs %d)", got, want)
	}
}
