package httpapi

import (
	"context"
	"net/http"

	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/saml"
)

// Carrying the sealed single sign-on state from the request to the handler, for
// both protocols (M1-009, M1-010).
//
// The generated strict handlers are handed a context and the parameters the
// specification declares — never the *http.Request — so a handler cannot read a
// cookie for itself. The pending-MFA token has the same problem and the same
// answer (see withPendingToken): the middleware that has the request puts the
// value in the context, and the handler takes it out.
//
// Nothing here resolves or validates anything. Each value is an opaque sealed
// blob until the package holding the key for it opens one, and the two keys are
// different — a value from one protocol will not open in the other.

// oidcStateKey and samlStateKey are this file's context keys, each its own type
// for the reason every other one in this package is.
type (
	oidcStateKey struct{}
	samlStateKey struct{}
)

// withSSOState records both sealed cookies, or the empty string for whichever
// the request does not carry.
//
// Both on every request rather than one per route: this runs in the
// authentication middleware, which sees a request before anything has resolved
// which operation it is, and reading a cookie that is not there costs nothing.
func withSSOState(ctx context.Context, r *http.Request) context.Context {
	ctx = context.WithValue(ctx, oidcStateKey{}, oidc.SealedFrom(r))
	return context.WithValue(ctx, samlStateKey{}, saml.SealedFrom(r))
}

// oidcStateFrom returns the sealed OIDC state. A callback with no cookie is not
// an error here — it is a callback that will be refused, and refusing it is
// [oidc.Provider.Complete]'s job rather than a middleware's.
func oidcStateFrom(ctx context.Context) string {
	sealed, ok := ctx.Value(oidcStateKey{}).(string)
	if !ok {
		return ""
	}
	return sealed
}

// samlStateFrom returns the sealed SAML pending request. Empty is meaningful
// here rather than merely refusable: it is what an identity-provider-initiated
// sign-in looks like, and whether this deployment accepts one is
// [saml.Provider.Complete]'s decision and not a middleware's.
func samlStateFrom(ctx context.Context) string {
	sealed, ok := ctx.Value(samlStateKey{}).(string)
	if !ok {
		return ""
	}
	return sealed
}
