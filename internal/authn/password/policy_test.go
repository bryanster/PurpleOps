package password_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

func TestValidateRejectsOnePasswordPerRule(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		plaintext   password.Plaintext
		wantMessage string
	}{
		"empty": {
			plaintext:   "",
			wantMessage: "is required",
		},
		"whitespace only": {
			// Long enough to pass the length rule, so this can only be the
			// whitespace rule failing.
			plaintext:   "                    ",
			wantMessage: "must be more than spaces",
		},
		"one character short": {
			plaintext:   "hunter2hunt",
			wantMessage: "must be at least 12 characters — a passphrase of a few words is ideal",
		},
		"a common password": {
			plaintext:   "correcthorsebatterystaple",
			wantMessage: "is one of the passwords attackers try first — choose something less predictable",
		},
		"a common password in another case": {
			// A capital letter is not a different guess to anyone running a
			// cracking rule set, so it must not step around the list.
			plaintext:   "CorrectHorseBatteryStaple",
			wantMessage: "is one of the passwords attackers try first — choose something less predictable",
		},
		"longer than the maximum": {
			plaintext:   password.Plaintext(strings.Repeat("a", password.MaxLength+1)),
			wantMessage: "must be at most 128 characters",
		},
		"far longer than the maximum": {
			// M1-002: 200 characters is refused cleanly, rather than truncated
			// to something its owner did not choose.
			plaintext:   password.Plaintext(strings.Repeat("correct horse ", 15)),
			wantMessage: "must be at most 128 characters",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := password.Validate("newPassword", test.plaintext)
			if err == nil {
				t.Fatalf("Validate(%v) = nil, want a validation error", test.plaintext)
			}

			// The sentinel, not the concrete type: this is how a handler that
			// wraps the error still returns a validation failure rather than a
			// 500.
			if !errors.Is(err, apierr.ErrValidation) {
				t.Errorf("Validate() error is not ErrValidation, so a handler would report it as a 500")
			}

			problem := apierr.Translate(err, "")
			if problem.Errors == nil || len(*problem.Errors) != 1 {
				t.Fatalf("problem.Errors = %v, want exactly the rule that failed", problem.Errors)
			}
			field := (*problem.Errors)[0]
			// The field name comes from the caller so a form can highlight the
			// input the password was typed into.
			if field.Field != "newPassword" {
				t.Errorf("field = %q, want %q", field.Field, "newPassword")
			}
			if field.Message != test.wantMessage {
				t.Errorf("message = %q, want %q", field.Message, test.wantMessage)
			}
		})
	}
}

func TestValidateAcceptsAPassphrase(t *testing.T) {
	t.Parallel()

	accepted := map[string]password.Plaintext{
		"a passphrase with spaces":   "correct battery horse staple",
		"exactly the minimum":        "twelve chars",
		"exactly the maximum":        password.Plaintext(strings.Repeat("a", password.MaxLength)),
		"no digits or symbols":       "the quick brown fox jumps",
		"leading and trailing space": "  a phrase with room  ",
		// Counted in characters, not bytes — though here the difference runs
		// the other way: a rule counting bytes would let a shorter password
		// through simply for being written in a script that needs three bytes
		// a character.
		"non-ASCII":                              "対数の暗号はとても良いですよ",
		"a common password with something added": "correcthorsebatterystaple and more",
	}

	for name, plaintext := range accepted {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := password.Validate("password", plaintext); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateNeverRepeatsThePassword(t *testing.T) {
	t.Parallel()

	// One string that fails a rule and is distinctive enough to find in any
	// output it leaked into.
	secret := password.Plaintext("swordfish-in-a-message")
	err := password.Validate("password", secret+password.Plaintext(strings.Repeat("!", password.MaxLength)))
	if err == nil {
		t.Fatal("Validate() = nil, want the too-long rule to fail")
	}

	if strings.Contains(err.Error(), "swordfish") {
		t.Errorf("the error names the password: %q", err.Error())
	}
	problem := apierr.Translate(err, "")
	if message := (*problem.Errors)[0].Message; strings.Contains(message, "swordfish") {
		t.Errorf("the message names the password: %q", message)
	}
}

func TestTheCommonPasswordListEarnsItsPlace(t *testing.T) {
	t.Parallel()

	// Every entry must fail Validate, which is the only claim the list makes.
	// An entry shorter than MinLength never gets consulted — the length rule
	// rejects it first — so it is dead weight, and this catches one being added.
	for _, entry := range password.CommonPasswordsForTest() {
		if entry != strings.ToLower(entry) || entry != strings.TrimSpace(entry) {
			t.Errorf("entry %q is not lowercase and trimmed, so it can never match", entry)
		}
		if n := utf8.RuneCountInString(entry); n < password.MinLength {
			t.Errorf("entry %q is %d characters, below the minimum of %d — the length rule already rejects it",
				entry, n, password.MinLength)
		}
		if err := password.Validate("password", password.Plaintext(entry)); err == nil {
			t.Errorf("Validate(%q) = nil for a listed password", entry)
		}
	}
}
