package authn

import (
	"context"

	"github.com/bryanster/purpleops/internal/store/identity"
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

	PlatformRole identity.PlatformRole

	// SessionID is the session this request arrived on. It is what logout
	// revokes and what a password change rotates, and it is never the token —
	// that value exists in the cookie and nowhere else.
	SessionID string

	// MFAEnforced is whether an administrator requires a second factor of this
	// person; MFASatisfied is whether this session presented one. Two facts, not
	// one: conflating them is the hole M1-008 closes.
	MFAEnforced  bool
	MFASatisfied bool
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
