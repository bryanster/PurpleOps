package httpapi

import (
	"context"

	"github.com/bryanster/purpleops/internal/authn"
	"github.com/bryanster/purpleops/internal/authn/challenge"
	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

// The four TOTP endpoints (M1-006). Like the login handlers next door they
// translate and nothing else: what a confirmed factor gates, what a code being
// accepted means and what may remove one all live in internal/authn.

// mfaPathPrefix is where these endpoints live, relative to [BasePath]. It is a
// constant because two other things are derived from it and must not drift:
// the pending cookie's Path, which scopes the cookie to exactly these routes,
// and the throttle's table of credential routes.
const mfaPathPrefix = "/auth/mfa"

// EnrollTotp mints a new, unconfirmed authenticator secret for the caller.
func (h *handlers) EnrollTotp(ctx context.Context, _ gen.EnrollTotpRequestObject) (gen.EnrollTotpResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	enrolment, err := h.auth.EnrollTOTP(ctx, subject)
	if err != nil {
		return nil, err
	}
	return gen.EnrollTotp200JSONResponse{
		OtpauthUri: enrolment.URI,
		Secret:     enrolment.Secret,
		QrCode:     enrolment.QRCode,
	}, nil
}

// ConfirmTotp finishes an enrolment and rotates the caller's session onto a new
// token, now marked as having satisfied MFA.
func (h *handlers) ConfirmTotp(ctx context.Context, request gen.ConfirmTotpRequestObject) (gen.ConfirmTotpResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	issued, err := h.auth.ConfirmTOTP(ctx, subject, request.Body.Code)
	if err != nil {
		return nil, err
	}
	return gen.ConfirmTotp204Response{
		Headers: gen.ConfirmTotp204ResponseHeaders{
			SetCookie: h.sessions.Cookie(issued.Token, issued.Session.ExpiresAt).String(),
		},
	}, nil
}

// VerifyTotp completes a sign-in that answered mfa_required.
//
// The pending token comes from the cookie on the request rather than from the
// body: it is a credential, and the one place a credential belongs in a browser
// is a cookie script cannot read. That also means this handler cannot be reached
// with a token a caller typed in from somewhere else.
func (h *handlers) VerifyTotp(ctx context.Context, request gen.VerifyTotpRequestObject) (gen.VerifyTotpResponseObject, error) {
	result, err := h.auth.VerifyTOTP(ctx,
		pendingTokenFrom(ctx),
		request.Body.Code,
		originFrom(ctx))
	if err != nil {
		return nil, err
	}

	profile, err := h.auth.Profile(ctx, result.Subject)
	if err != nil {
		return nil, err
	}
	user := currentUser(profile, h.sessions.CSRFToken(result.Issued.Token))
	cookie := h.sessions.Cookie(result.Issued.Token, result.Issued.Session.ExpiresAt).String()

	// The session cookie goes in the response's own Set-Cookie; the CSRF cookie
	// is added by csrfWriter and the spent pending cookie is cleared by
	// challengeWriter, both on the way out. Three cookies cannot be folded into
	// the one header the generated type carries — see those two writers.
	return gen.VerifyTotp200JSONResponse{
		Body: gen.LoginResult{
			Status: gen.LoginStatusAuthenticated,
			User:   &user,
		},
		Headers: gen.VerifyTotp200ResponseHeaders{SetCookie: cookie},
	}, nil
}

// DisableTotp removes the caller's authenticator. The current password is
// checked by the service, which also refuses while MFA is enforced.
func (h *handlers) DisableTotp(ctx context.Context, request gen.DisableTotpRequestObject) (gen.DisableTotpResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.auth.DisableTOTP(ctx, subject,
		password.Plaintext(request.Body.CurrentPassword)); err != nil {
		return nil, err
	}
	return gen.DisableTotp204Response{}, nil
}

// mfaState renders the three facts about a person's second factor. It is here
// rather than inline in currentUser so that "enforced", "enrolled" and
// "satisfied" are filled from three different sources in one place, where the
// difference between them is visible.
func mfaState(profile authn.Profile) gen.MFAState {
	return gen.MFAState{
		// The administrator's requirement, off the user row.
		Enforced: profile.User.MFAEnforced,
		// Whether there is a confirmed enrolment, off the authenticator row.
		Enrolled: profile.MFAEnrolled,
		// Whether *this session* presented one, off the session row.
		Satisfied: profile.MFASatisfied,
	}
}

// pendingTokenKey is this file's context key, distinct from every other one in
// the package for the same reason theirs are.
type pendingTokenKey struct{}

// withPendingToken records the MFA challenge cookie, so that a strict handler —
// which is handed a context and not a request — can present it. It is set by
// [authenticate], which is the middleware that already turns a request into what
// a handler is allowed to know about it.
func withPendingToken(ctx context.Context, token challenge.Token) context.Context {
	return context.WithValue(ctx, pendingTokenKey{}, token)
}

// pendingTokenFrom returns the pending token the request arrived with, or the
// empty token. An empty one resolves to nothing, so a request that never went
// through the middleware fails the same way as one with no cookie.
func pendingTokenFrom(ctx context.Context) challenge.Token {
	token, ok := ctx.Value(pendingTokenKey{}).(challenge.Token)
	if !ok {
		return ""
	}
	return token
}
