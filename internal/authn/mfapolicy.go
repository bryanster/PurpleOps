package authn

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// Enforcement (M1-008). The defect it closes, from PLAN.md §4:
//
//	today MFA=True only redirects users who already enrolled, so anyone who
//	skips /mfa/register logs in with a password alone.
//
// The bug is the order of two questions. v1 asked "have they enrolled?" and
// enforced only if the answer was yes, which makes enrolment optional by
// construction — the people who skipped it were exactly the people it stopped
// applying to. Everything here asks about *policy* first and about enrolment
// second, and the answer to "required, but nothing enrolled" is a session that
// can do one thing rather than a session that can do everything.

// The keys this package owns in app.platform_setting. They are here, next to
// the type that encodes and decodes them, because a key written in one file and
// read in another is two spellings waiting to diverge.
const (
	keyMFARequiredForAll    = "mfa.required_for_all"
	keyMFARequiredForAdmins = "mfa.required_for_admins"
)

// Settings is the part of the settings store this package needs.
// [*settings.Store] satisfies it.
type Settings interface {
	All(ctx context.Context) (map[string]settings.Setting, error)
	Put(ctx context.Context, values map[string]string, updatedBy string) error
}

// MFAPolicy is the platform-wide half of the requirement. The other half is the
// per-user [identity.User.MFAEnforced] flag, and [MFAPolicy.Requires] is where
// the two meet.
//
// The zero value is the policy of a deployment nobody has configured: nothing is
// required of anybody. That is deliberate — a fresh installation must not lock
// its first administrator into enrolling before they have seen the product —
// and it is why an absent row reads as false rather than as an error.
type MFAPolicy struct {
	// RequiredForAll requires a second factor of every account that signs in
	// with a local password.
	RequiredForAll bool

	// RequiredForAdmins requires one of every platform administrator. It is
	// implied by RequiredForAll and stored separately anyway, so that turning
	// the wider requirement off does not silently release administrators with
	// it — which is the one group whose accounts are worth the most to take
	// over.
	RequiredForAdmins bool
}

// Requires reports whether user must hold a second factor.
//
// It is the **or** of everything that applies, which is what makes the per-user
// flag meaningful with the platform policy off, and the platform policy
// meaningful for somebody nobody has flagged.
//
// An account with no local password is exempt. Those are the SSO-only accounts
// (M1-009, M1-010): they do not present a password to this server, so there is
// no local sign-in for a local second factor to stand behind, and the identity
// provider is where their factors are asserted and enforced. docs/security.md
// states the rule and what changes when an IdP tells us what it verified.
func (p MFAPolicy) Requires(user identity.User) bool {
	if user.PasswordHash == "" {
		return false
	}
	return p.RequiredForAll ||
		(p.RequiredForAdmins && user.PlatformRole == authz.PlatformRoleAdmin) ||
		user.MFAEnforced
}

// MFAPolicy returns the platform policy. Administrators only — enforced by the
// authorization middleware, which refuses anybody without `settings.read` before
// the handler that calls this is entered (M1-013). There is deliberately no
// check here: a second one would be a second definition of who an administrator
// is, and PLAN.md §4's whole complaint about v1 is that it had forty of them.
//
// What an ordinary caller needs — whether the requirement applies to *them* — is
// on their profile as [Profile.MFARequired], and needs no permission to read
// because it is a fact about themselves.
func (s *Service) MFAPolicy(ctx context.Context) (MFAPolicy, error) {
	return s.mfaPolicy(ctx)
}

// SetMFAPolicy replaces the platform policy and returns it as stored.
//
// Administrators only, for the reason [Service.MFAPolicy] is, and by the same
// mechanism: `settings.manage` is required by the middleware in front of the
// handler. subject is still here, and is not a permission — it is who to record
// as having changed it, on the row and in the log.
//
// Both fields, always: the request carries a whole policy rather than a change
// to one field of it, so two administrators editing at once cannot each keep
// half of what the other did.
//
// Nothing is revoked, deleted or migrated here, in either direction. Turning a
// requirement on takes effect the next time each session is used — see
// [Service.Authenticate], which is where "on their next request" is actually
// implemented — and turning one off leaves every enrolment and every recovery
// code exactly where it was.
func (s *Service) SetMFAPolicy(ctx context.Context, subject Subject, policy MFAPolicy) (MFAPolicy, error) {
	if err := s.settings.Put(ctx, map[string]string{
		keyMFARequiredForAll:    strconv.FormatBool(policy.RequiredForAll),
		keyMFARequiredForAdmins: strconv.FormatBool(policy.RequiredForAdmins),
	}, subject.UserID); err != nil {
		return MFAPolicy{}, err
	}

	// Warn rather than info, for the reason removing an authenticator is a
	// warning: this is the line somebody reads back when asking how an account
	// came to be reachable with a password alone. M1-015 gives it a durable
	// home in the activity log.
	s.log.WarnContext(ctx, "the platform MFA policy was changed",
		slog.String("user_id", subject.UserID),
		slog.Bool("required_for_all", policy.RequiredForAll),
		slog.Bool("required_for_admins", policy.RequiredForAdmins))

	return policy, nil
}

// mfaPolicy reads the policy with no permission check, for the paths inside this
// package that have to evaluate it on somebody else's behalf: signing in, and
// every authenticated request.
func (s *Service) mfaPolicy(ctx context.Context) (MFAPolicy, error) {
	stored, err := s.settings.All(ctx)
	if err != nil {
		return MFAPolicy{}, err
	}
	return DecodeMFAPolicy(stored)
}

// DecodeMFAPolicy reads a policy out of stored settings.
//
// Exported for blctl, which has the database but no [Service]: `user reset-mfa`
// has to tell an operator whether the account it just reset will be asked to
// enrol again, and the honest answer depends on the policy. One decoder rather
// than two, so the command line and the server cannot disagree about what a row
// means.
func DecodeMFAPolicy(stored map[string]settings.Setting) (MFAPolicy, error) {
	var (
		policy MFAPolicy
		err    error
	)
	if policy.RequiredForAll, err = storedBool(stored, keyMFARequiredForAll); err != nil {
		return MFAPolicy{}, err
	}
	if policy.RequiredForAdmins, err = storedBool(stored, keyMFARequiredForAdmins); err != nil {
		return MFAPolicy{}, err
	}
	return policy, nil
}

// storedBool reads one boolean setting. An absent key is false — the default of
// a deployment nobody has configured.
//
// A value that is present and unreadable is an error rather than a false, and
// the difference matters: this decides whether a requirement applies, so
// guessing would answer "not required" for a row somebody wrote to mean the
// opposite. The only writer is [Service.SetMFAPolicy], so an unreadable value
// means the database was edited by hand, and the failure it causes — every
// authenticated request answering 500 — is loud in the way that deserves.
func storedBool(stored map[string]settings.Setting, key string) (bool, error) {
	setting, ok := stored[key]
	if !ok {
		return false, nil
	}
	value, err := strconv.ParseBool(setting.Value)
	if err != nil {
		return false, fmt.Errorf(
			"authn: the platform setting %q holds %q, which is not true or false: %w",
			key, setting.Value, err)
	}
	return value, nil
}

// loginOutcome is the row of M1-008's table that a sign-in landed on. It is an
// enumeration rather than two booleans a caller reads in the right order,
// because the ticket's table has four rows and three of them end somewhere
// different — and scattered booleans are how v1 arrived at a fourth outcome
// nobody designed.
type loginOutcome int

const (
	// outcomeSession is an ordinary sign-in: nothing further is required.
	outcomeSession loginOutcome = iota

	// outcomeChallenge is a second factor this person holds and has not yet
	// presented. No session exists yet; the challenge is the only thing issued.
	outcomeChallenge

	// outcomeEnrolment is a second factor this person is required to hold and
	// does not. A session is issued and it may do exactly one thing.
	outcomeEnrolment
)

// decideLogin is M1-008's state machine, in one place, over the two facts that
// remain once the password has been checked. A wrong password is the table's
// first row and never reaches here — it is answered before this is called.
//
//	| MFA required | enrolled | outcome          |
//	|--------------|----------|------------------|
//	| no           | no       | session          |
//	| no           | yes      | challenge        |  <- stricter than the ticket
//	| yes          | yes      | challenge        |
//	| yes          | no       | enrolment        |
//
// The second row is the one deviation, and it is M1-006's rule kept: somebody
// who set up an authenticator meant it to be asked for, so a confirmed factor
// gates the sign-in whether or not an administrator requires one. The ticket's
// table writes that row as "—", and refusing to ask for a factor the person
// enrolled would be the wrong way to read it.
func decideLogin(required, enrolled bool) loginOutcome {
	switch {
	case enrolled:
		return outcomeChallenge
	case required:
		return outcomeEnrolment
	default:
		return outcomeSession
	}
}
