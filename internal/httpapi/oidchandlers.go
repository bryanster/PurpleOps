package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The three single sign-on endpoints (M1-009). They translate and decide
// nothing: internal/authn/oidc owns the protocol, internal/authn owns what an
// assertion means for an account, and what is left here is reading a query
// string, setting cookies and choosing where to send the browser.

// signInPath is where a browser lands when a single sign-on completed and a
// second factor is still required. There is no session at that point — only a
// pending challenge in a cookie — so the interface cannot work out what happened
// from `GET /auth/me`, and the query parameter is how it is told (M1-017 renders
// it).
const signInPath = "/login?mfa=required"

// homePath is where a sign-in that named no return path lands.
const homePath = "/"

// GetAuthProviders lists the ways this deployment can be signed in to, for the
// login page to draw buttons from.
//
// A provider that is configured but unreachable is left out. That is the whole
// point of the endpoint: PLAN.md §4 asks that a broken identity provider never
// lock anybody out, and a button leading to a provider that is down is worse
// than no button — it looks like Blacklight is broken.
func (h *handlers) GetAuthProviders(ctx context.Context, _ gen.GetAuthProvidersRequestObject) (gen.GetAuthProvidersResponseObject, error) {
	providers := gen.AuthProviders{
		Password: true,
		// Never nil: an empty array and a missing field are different values on
		// the wire, and a client mapping over the second one crashes.
		Sso: []gen.SSOProvider{},
	}
	if h.oidc != nil && h.oidc.Available(ctx) {
		providers.Sso = append(providers.Sso, gen.SSOProvider{
			Id: gen.SSOProviderIdOidc,
			// The protocol, not the issuer. This endpoint is unauthenticated, and
			// the issuer URL names the directory this organization uses.
			Label:    "Single sign-on",
			StartUrl: BasePath + "/auth/oidc/start",
		})
	}
	if h.saml != nil && h.saml.Available(ctx) {
		providers.Sso = append(providers.Sso, gen.SSOProvider{
			Id: gen.SSOProviderIdSaml,
			// "SAML" rather than the entity ID, for the reason above, and
			// spelled out rather than called "Single sign-on" a second time: a
			// deployment with both configured would otherwise draw two buttons
			// with the same words on them.
			Label:    "SAML single sign-on",
			StartUrl: BasePath + saml.StartPath,
		})
	}
	return gen.GetAuthProviders200JSONResponse(providers), nil
}

// StartOidcSignIn redirects the browser to the identity provider.
func (h *handlers) StartOidcSignIn(ctx context.Context, request gen.StartOidcSignInRequestObject) (gen.StartOidcSignInResponseObject, error) {
	provider, err := h.signOn()
	if err != nil {
		return nil, err
	}

	var returnTo string
	if request.Params.ReturnTo != nil {
		returnTo = *request.Params.ReturnTo
	}

	start, err := provider.Start(ctx, returnTo)
	switch {
	case errors.Is(err, oidc.ErrUnsafeReturnTo):
		// A 400 naming the parameter, not a redirect to somewhere safe. The only
		// thing that sets this is our own login page; a value it did not produce
		// came from somebody else's link, and quietly substituting "/" would hide
		// that from whoever is looking at the logs.
		h.log.WarnContext(ctx, "refused a single sign-on with an unsafe return path",
			slog.String("return_to", returnTo),
			slog.String("error", err.Error()))
		return nil, apierr.Validation(apierr.Field("return_to",
			`must be a path within this application, beginning with a single "/"`))
	case errors.Is(err, oidc.ErrUnavailable):
		return nil, unavailable(err)
	case err != nil:
		return nil, err
	}

	return redirectTo(start.AuthorizationURL, []*http.Cookie{provider.Cookie(start.Sealed)}), nil
}

// CompleteOidcSignIn finishes what the provider sent back, and signs somebody in.
func (h *handlers) CompleteOidcSignIn(ctx context.Context, request gen.CompleteOidcSignInRequestObject) (gen.CompleteOidcSignInResponseObject, error) {
	provider, err := h.signOn()
	if err != nil {
		return nil, err
	}

	verified, err := provider.Complete(ctx, oidc.Callback{
		State:            value(request.Params.State),
		Code:             value(request.Params.Code),
		Error:            value(request.Params.Error),
		ErrorDescription: value(request.Params.ErrorDescription),
		Sealed:           oidcStateFrom(ctx),
	})
	switch {
	case errors.Is(err, oidc.ErrNoPendingSignIn), errors.Is(err, oidc.ErrRejected):
		// One answer for every way of the callback not being acceptable, for the
		// reason apierr.BadCredentials gives: a caller who can tell "your state is
		// wrong" from "your token did not verify" is a caller probing which half
		// of a forgery failed. The specific reason is in the log.
		h.log.WarnContext(ctx, "refused a single sign-on callback",
			slog.String("error", err.Error()))
		return nil, apierr.Unauthenticated(err.Error())
	case errors.Is(err, oidc.ErrUnsafeReturnTo):
		return nil, apierr.Validation(apierr.Field("return_to",
			`must be a path within this application, beginning with a single "/"`))
	case errors.Is(err, oidc.ErrUnavailable):
		return nil, unavailable(err)
	case err != nil:
		return nil, err
	}

	// The mapping is read here, from the provider that holds this deployment's
	// configuration, and applied by internal/authn, which is the only thing that
	// may change an account. Every login, so that a group removed at the provider
	// takes effect at the next one.
	role, mapped := provider.Role(verified.Groups)

	result, err := h.auth.SignInWithFederatedIdentity(ctx, authn.FederatedLogin{
		Provider:      identity.ProviderOIDC,
		Subject:       verified.Subject,
		Email:         verified.Email,
		EmailVerified: verified.EmailVerified,
		DisplayName:   verified.DisplayName,
		Role:          role,
		RoleMapped:    mapped,
		AutoProvision: provider.AutoProvision(),
		Request:       originFrom(ctx),
	})
	if err != nil {
		return nil, err
	}

	// The state cookie is spent whatever happened next: one authorization code,
	// one callback, one state.
	cookies := []*http.Cookie{provider.ClearCookie()}

	if result.Status == authn.LoginMFARequired {
		// Signed in at the provider and not signed in here: what they get is the
		// pending cookie and the code entry page, exactly as a local sign-in with
		// an authenticator does (M1-006).
		cookies = append(cookies, h.challenges.Cookie(
			result.Challenge.Token, result.Challenge.Challenge.ExpiresAt))
		return redirectTo(signInPath, cookies), nil
	}

	// Both remaining outcomes issued a session. The difference between them —
	// whether it may do anything but enrol a second factor — is enforced on every
	// request by the gate in mfagate.go, and reported to the interface by
	// GET /auth/me, so there is nothing for this redirect to say about it.
	cookies = append(cookies, h.sessions.Cookie(
		result.Issued.Token, result.Issued.Session.ExpiresAt))

	// The CSRF cookie is not added here: csrfWriter adds it on the way out, from
	// the session cookie above, so every path that issues a session gets it
	// without knowing that it exists.
	target := verified.ReturnTo
	if target == "" {
		target = homePath
	}
	return redirectTo(target, cookies), nil
}

// signOn returns the configured provider, or the 404 an endpoint answers when
// this deployment has no single sign-on.
//
// A 404 rather than a 501 or a 503: from the caller's side there is no such
// endpoint on this deployment, and saying "not implemented" would suggest the
// software cannot do it rather than that this installation has not configured
// it.
func (h *handlers) signOn() (*oidc.Provider, error) {
	if h.oidc == nil {
		return nil, apierr.NotFound("single sign-on provider", "oidc")
	}
	return h.oidc, nil
}
