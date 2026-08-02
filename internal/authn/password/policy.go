package password

import (
	_ "embed"
	"strings"
	"unicode/utf8"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// The policy, in full.
//
// It follows NIST SP 800-63B §5.1.1.2: length is the requirement that matters,
// composition rules ("one uppercase, one digit, one symbol") are not required
// and are counterproductive, and chosen passwords are checked against a list of
// values known to be common. There is nothing here about expiry either — a
// password is changed when there is reason to believe it is known, not on a
// schedule.
const (
	// MinLength is the shortest password accepted, in characters. Twelve is
	// what PLAN.md §4 and M1-002 specify; it is above NIST's floor of eight
	// because this is an application people log into with a password they chose.
	MinLength = 12

	// MaxLength is the longest password accepted, in characters. It exists to
	// bound work on an unauthenticated path, not to discourage long
	// passphrases, and is set at the top of the range M1-002 allows so that no
	// passphrase anyone would type is refused. Nothing truncates: a longer
	// password is rejected and said so, because silently hashing a prefix means
	// a shorter password than its owner believes they have.
	MaxLength = 128
)

// Validate reports whether plaintext may be used as a password.
//
// field is the name of the request field being checked ("password",
// "newPassword"), and comes back in the [apierr.Validation] error so a form can
// put the message next to the right input. The messages describe the rule, not
// the value: none of them repeats any part of what was typed.
//
// The rules are checked in the order below, and the first failure is returned
// on its own. A list saying a password is both too short and too common is
// noise — the caller's next attempt is a different password either way.
func Validate(field string, plaintext Plaintext) error {
	fail := func(message string) error {
		return apierr.Validation(apierr.Field(field, message))
	}

	switch {
	case len(plaintext) == 0:
		return fail("is required")
	case strings.TrimSpace(plaintext.Reveal()) == "":
		return fail("must be more than spaces")
	case utf8.RuneCountInString(plaintext.Reveal()) < MinLength:
		// Counted in characters rather than bytes, so a passphrase in a script
		// that does not fit in one byte per character is not held to a longer
		// requirement than one in ASCII.
		return fail("must be at least 12 characters — a passphrase of a few words is ideal")
	case utf8.RuneCountInString(plaintext.Reveal()) > MaxLength:
		return fail("must be at most 128 characters")
	case isCommon(plaintext):
		return fail("is one of the passwords attackers try first — choose something less predictable")
	}
	return nil
}

// commonPasswordList is the embedded corpus. Its provenance and limits are
// documented at the top of the file itself.
//
//go:embed common_passwords.txt
var commonPasswordList string

// commonPasswords is the parsed form: lowercase, one key per entry. Built once,
// read-only afterwards, so it needs no synchronization.
var commonPasswords = parseCommonPasswords(commonPasswordList)

// parseCommonPasswords reads the embedded list. Blank lines and # comments are
// skipped; everything else is lowercased so that the lookup can be.
func parseCommonPasswords(list string) map[string]struct{} {
	common := make(map[string]struct{})
	for line := range strings.Lines(list) {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		common[strings.ToLower(entry)] = struct{}{}
	}
	return common
}

// isCommon reports whether plaintext is on the list, ignoring case.
//
// Case-insensitively, because Passw0rdPassw0rd is the same guess as
// passw0rdpassw0rd to anyone running a cracking rule set — matching only the
// exact casing would let a capital letter step around the whole list.
//
// This is a membership test and nothing more: no substring matching, no
// stripping of trailing digits. Guessing at the shape of a password is how a
// policy starts rejecting passphrases that happen to contain a common word,
// which is the sort of rule that sends people back to Password1!. Catching the
// long tail properly means a breach corpus — a HIBP range lookup, or an
// offline Pwned Passwords file — which M1-002 puts out of scope and which is
// worth a follow-up ticket.
func isCommon(plaintext Plaintext) bool {
	_, found := commonPasswords[strings.ToLower(plaintext.Reveal())]
	return found
}
