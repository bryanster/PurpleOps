package httpapi

import (
	"context"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
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

// ConfirmTotp finishes an enrolment, rotates the caller's session onto a new
// token now marked as having satisfied MFA, and hands back the recovery codes
// minted with it.
//
// This response and the one from RegenerateRecoveryCodes are the only two in the
// API that carry codes, and they are the only two that can be: the server keeps
// hashes, so there is nothing for a third to read.
func (h *handlers) ConfirmTotp(ctx context.Context, request gen.ConfirmTotpRequestObject) (gen.ConfirmTotpResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.auth.ConfirmTOTP(ctx, subject, request.Body.Code)
	if err != nil {
		return nil, err
	}
	return gen.ConfirmTotp200JSONResponse{
		Body: recoveryCodes(result.Recovery),
		Headers: gen.ConfirmTotp200ResponseHeaders{
			SetCookie: h.sessions.Cookie(result.Issued.Token, result.Issued.Session.ExpiresAt).String(),
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

// VerifyRecoveryCode completes a sign-in with a printed code instead of an
// authenticator (M1-007).
//
// It is VerifyTotp with a different second factor, down to where the pending
// token comes from and what the response sets — which is the point: a person
// reaching for a recovery code is having a bad enough day without also landing
// in a different sign-in flow.
func (h *handlers) VerifyRecoveryCode(ctx context.Context, request gen.VerifyRecoveryCodeRequestObject) (gen.VerifyRecoveryCodeResponseObject, error) {
	result, err := h.auth.VerifyRecoveryCode(ctx,
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

	return gen.VerifyRecoveryCode200JSONResponse{
		Body: gen.LoginResult{
			Status: gen.LoginStatusAuthenticated,
			User:   &user,
		},
		Headers: gen.VerifyRecoveryCode200ResponseHeaders{SetCookie: cookie},
	}, nil
}

// RegenerateRecoveryCodes replaces the caller's codes and returns the new set.
// The password and the requirement that this session has already satisfied MFA
// are both checked by the service.
func (h *handlers) RegenerateRecoveryCodes(ctx context.Context, request gen.RegenerateRecoveryCodesRequestObject) (gen.RegenerateRecoveryCodesResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	set, err := h.auth.RegenerateRecoveryCodes(ctx, subject,
		password.Plaintext(request.Body.CurrentPassword))
	if err != nil {
		return nil, err
	}
	return gen.RegenerateRecoveryCodes200JSONResponse(recoveryCodes(set)), nil
}

// recoveryCodes renders a minted set for the wire.
//
// [recovery.Code] redacts itself for fmt, slog and the JSON encoder alike, so
// this call to Reveal is the one place in the process where a code becomes an
// ordinary string — which is what makes leaking one a deliberate act rather
// than an available accident.
func recoveryCodes(set authn.RecoveryCodeSet) gen.RecoveryCodes {
	printed := make([]string, len(set.Codes))
	for i, code := range set.Codes {
		printed[i] = code.Printed()
	}
	return gen.RecoveryCodes{Codes: printed, GeneratedAt: set.GeneratedAt}
}

// mfaState renders the five facts about a person's second factor. It is here
// rather than inline in currentUser so that "enforced", "required", "enrolled",
// "satisfied" and what is left to fall back on are filled from five different
// sources in one place, where the difference between them is visible — which is
// the whole subject of M1-008, whose defect was two of these being treated as
// one.
func mfaState(profile authn.Profile) gen.MFAState {
	return gen.MFAState{
		// The administrator's requirement of this person specifically, off the
		// user row.
		Enforced: profile.User.MFAEnforced,
		// Whether one is required at all: the flag above, or the platform
		// policy applying to them (M1-008). This is the one an interface acts
		// on, and it is computed rather than stored.
		Required: profile.MFARequired,
		// Whether there is a confirmed enrolment, off the authenticator row.
		Enrolled: profile.MFAEnrolled,
		// Whether *this session* presented one, off the session row.
		Satisfied: profile.MFASatisfied,
		// How many ways back in remain, counted off the recovery code rows.
		RecoveryCodesRemaining: profile.RecoveryCodesRemaining,
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
