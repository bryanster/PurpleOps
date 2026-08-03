package authn

import (
	"context"

	"github.com/bryanster/blacklight/internal/authz"
)

// Subject is who the caller is, as far as this request is concerned: enough to
// decide with, and nothing that has to be looked up again to decide.
//
// It deliberately carries no engagement memberships. Loading them costs a query
// on every request, and the policy that needs them (M1-012) can ask for the one
// engagement it is being asked about. Adding them here would be a query per
// request in exchange for a saving on a handful of them.
//
// The zero value is an unauthenticated caller. There is no "anonymous subject"
// with a role that means nothing — absence is the state, and [SubjectFrom]
// reports it.
type Subject struct {
	UserID      string
	Email       string
	DisplayName string

	PlatformRole authz.PlatformRole

	// Method is how this request proved who it is. It is here because CSRF
	// protection turns on it and must not turn on anything a caller controls
	// directly (M1-005): a request is exempt because it *arrived* bearing a
	// service token that this server checked, not because it sent a header
	// saying so.
	//
	// The vocabulary is authz's, because the policy reads it too — a service
	// token is fenced by its scopes as well as by its owner's role — and one
	// distinction deserves one definition.
	Method authz.Method

	// SessionID is the session this request arrived on. It is what logout
	// revokes and what a password change rotates, and it is never the token —
	// that value exists in the cookie and nowhere else.
	SessionID string

	// MFASatisfied is whether this session presented a second factor. It is a
	// fact about the session and not about the person: that they are *required*
	// to hold one is a different fact, and conflating the two is the hole
	// M1-008 closes.
	MFASatisfied bool

	// MFAEnrolmentRequired is set when this session may do exactly one thing:
	// enrol a second factor. It means a factor is required of this person, this
	// session has not presented one, and there is none enrolled to present
	// (M1-008).
	//
	// It is a state rather than the three booleans it is derived from, because
	// the decision is made once — in [Service.Authenticate], which is the only
	// place that has all three — and read by a middleware whose job is to
	// refuse, not to re-derive. The three-booleans version is how v1 arrived at
	// a combination nobody had thought about.
	//
	// False on a satisfied session whatever the policy says, and false where no
	// factor is required. "Is a factor required of this person at all" is
	// [Profile.MFARequired], which is a different question with a different
	// answer.
	MFAEnrolmentRequired bool
}

// contextKey is unexported so that nothing outside this package can put a
// Subject into a context, or take one out, without going through the two
// functions below.
type contextKey struct{}

// WithSubject returns a context carrying the authenticated caller. The
// authentication middleware is the only caller in the server; a test may use it
// to build a request as though it had arrived signed in.
func WithSubject(ctx context.Context, subject Subject) context.Context {
	return context.WithValue(ctx, contextKey{}, subject)
}

// SubjectFrom returns the authenticated caller, and false when the request is
// anonymous.
//
// Both halves matter. The middleware lets an unauthenticated request through on
// purpose — refusing is authorization's job, in one place (M1-013) — so every
// reader has to handle "nobody" rather than assuming a subject is there.
func SubjectFrom(ctx context.Context) (Subject, bool) {
	subject, ok := ctx.Value(contextKey{}).(Subject)
	return subject, ok && subject.UserID != ""
}
