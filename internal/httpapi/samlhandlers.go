package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/returnto"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The three SAML endpoints (M1-010). Like their OIDC counterparts they translate
// and decide nothing: internal/authn/saml owns the protocol, internal/authn owns
// what a verified assertion means for an account, and what is left here is
// reading a form, setting cookies and choosing where to send the browser.

// GetSamlMetadata serves this deployment's service provider metadata.
//
// It answers whether or not the identity provider is reachable, which matters
// more than it looks: the moment somebody fetches this is the moment they are
// setting the registration up, which is exactly when the other half of it does
// not work yet.
func (h *handlers) GetSamlMetadata(_ context.Context, _ gen.GetSamlMetadataRequestObject) (gen.GetSamlMetadataResponseObject, error) {
	provider, err := h.samlSignOn()
	if err != nil {
		return nil, err
	}

	document, err := provider.Metadata()
	if err != nil {
		return nil, err
	}
	return gen.GetSamlMetadata200ApplicationsamlmetadataXmlResponse{
		Body:          bytes.NewReader(document),
		ContentLength: int64(len(document)),
	}, nil
}

// StartSamlSignIn redirects the browser to the identity provider.
func (h *handlers) StartSamlSignIn(ctx context.Context, request gen.StartSamlSignInRequestObject) (gen.StartSamlSignInResponseObject, error) {
	provider, err := h.samlSignOn()
	if err != nil {
		return nil, err
	}

	var target string
	if request.Params.ReturnTo != nil {
		target = *request.Params.ReturnTo
	}

	start, err := provider.Start(ctx, target)
	switch {
	case errors.Is(err, returnto.ErrUnsafe):
		// A 400 naming the parameter, not a redirect to somewhere safe — the
		// same answer the OIDC start gives, for the same reason: quietly
		// substituting "/" would hide somebody else's link from whoever is
		// reading the logs.
		h.log.WarnContext(ctx, "refused a SAML sign-in with an unsafe return path",
			slog.String("return_to", target),
			slog.String("error", err.Error()))
		return nil, apierr.Validation(apierr.Field("return_to",
			`must be a path within this application, beginning with a single "/"`))
	case errors.Is(err, saml.ErrUnavailable):
		return nil, unavailable(err)
	case err != nil:
		return nil, err
	}

	return redirectTo(start.RedirectURL, []*http.Cookie{provider.Cookie(start.Sealed)}), nil
}

// CompleteSamlSignIn consumes the assertion the identity provider posted, and
// signs somebody in.
func (h *handlers) CompleteSamlSignIn(ctx context.Context, request gen.CompleteSamlSignInRequestObject) (gen.CompleteSamlSignInResponseObject, error) {
	provider, err := h.samlSignOn()
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		// Unreachable: the specification marks the body required and the
		// validator runs first. Answered rather than dereferenced, because the
		// alternative on a reachable path would be a panic in the login flow.
		return nil, apierr.Validation(apierr.Field("SAMLResponse", "is required"))
	}

	verified, err := provider.Complete(ctx, saml.Callback{
		Response:   request.Body.SAMLResponse,
		RelayState: value(request.Body.RelayState),
		Sealed:     samlStateFrom(ctx),
	})
	switch {
	case errors.Is(err, saml.ErrNoPendingSignIn), errors.Is(err, saml.ErrRejected):
		// One answer for every way of an assertion not being acceptable, for the
		// reason apierr.BadCredentials gives: a caller who can tell "your
		// signature is wrong" from "that assertion has been used" is a caller
		// mapping which of the checks they have got past. The specific reason —
		// and it is a specific reason, dug out of the library's deliberately
		// opaque error — is in the log.
		h.log.WarnContext(ctx, "refused a SAML assertion",
			slog.String("error", err.Error()))
		return nil, apierr.Unauthenticated(err.Error())
	case errors.Is(err, returnto.ErrUnsafe):
		return nil, apierr.Validation(apierr.Field("return_to",
			`must be a path within this application, beginning with a single "/"`))
	case errors.Is(err, saml.ErrUnavailable):
		return nil, unavailable(err)
	case err != nil:
		return nil, err
	}

	// Read here, from the provider holding this deployment's configuration, and
	// applied by internal/authn, which is the only thing that may change an
	// account. Every sign-in, so a group removed at the provider takes effect at
	// the next one — including a demotion out of admin.
	role, mapped := provider.Role(verified.Groups)

	result, err := h.auth.SignInWithFederatedIdentity(ctx, authn.FederatedLogin{
		Provider:    identity.ProviderSAML,
		Subject:     verified.Subject,
		Email:       verified.Email,
		DisplayName: verified.DisplayName,
		// SAML has no `email_verified`, and this is the honest reading of that
		// rather than a shortcut. The check exists on the OIDC path because a
		// provider may vouch for an address nobody proved — self-service signup
		// at a permissive tenant, a directory that lets people edit their own
		// mail attribute — and linking an existing local account to such an
		// address is an account takeover.
		//
		// A SAML assertion is a document signed by the one identity provider
		// this deployment was configured against, carrying an attribute that
		// provider's administrator chose to send. There is no self-service path
		// into it: somebody who can set the mail attribute in that directory can
		// already mint an assertion for any NameID, so the address is exactly as
		// trustworthy as the subject beside it. Treating it as unverified would
		// not close a hole; it would only stop an enterprise's existing local
		// accounts from ever being reachable by single sign-on.
		EmailVerified: true,
		Role:          role,
		RoleMapped:    mapped,
		AutoProvision: provider.AutoProvision(),
		Request:       originFrom(ctx),
	})
	if err != nil {
		return nil, err
	}

	// The pending-request cookie is spent whatever happened next: one
	// authentication request, one assertion.
	cookies := []*http.Cookie{provider.ClearCookie()}

	if result.Status == authn.LoginMFARequired {
		// Signed in at the provider and not signed in here: what they get is the
		// pending cookie and the code entry page, exactly as a local sign-in
		// with an authenticator does (M1-006).
		cookies = append(cookies, h.challenges.Cookie(
			result.Challenge.Token, result.Challenge.Challenge.ExpiresAt))
		return redirectTo(signInPath, cookies), nil
	}

	cookies = append(cookies, h.sessions.Cookie(
		result.Issued.Token, result.Issued.Session.ExpiresAt))

	// An identity-provider-initiated sign-in named no return path — there was no
	// page here it started from — so it lands on the front page.
	target := verified.ReturnTo
	if target == "" {
		target = homePath
	}
	return redirectTo(target, cookies), nil
}

// samlSignOn returns the configured provider, or the 404 an endpoint answers
// when this deployment has no SAML. The argument is the one [handlers.signOn]
// makes for OIDC: from the caller's side there is no such endpoint here.
func (h *handlers) samlSignOn() (*saml.Provider, error) {
	if h.saml == nil {
		return nil, apierr.NotFound("single sign-on provider", "saml")
	}
	return h.saml, nil
}
