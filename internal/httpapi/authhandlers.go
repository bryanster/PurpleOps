package httpapi

import (
	"context"

	"github.com/bryanster/purpleops/internal/authn"
	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

// The four local-login endpoints. They translate and nothing else: the rules
// about who may sign in, what ends a session and what a password change does to
// the other ones live in internal/authn, so that SSO (M1-009) and service tokens
// (M1-011) reach the same rules by a different route.

// Login checks an email address and password and, when nothing else is
// required, sets the session cookie.
//
// Every failure is the same 401 with the same body — see apierr.BadCredentials,
// which is written so that this handler has no way to make it vary.
func (h *handlers) Login(ctx context.Context, request gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	result, err := h.auth.Login(ctx, authn.Login{
		Email:    request.Body.Email,
		Password: password.Plaintext(request.Body.Password),
		Request:  originFrom(ctx),
	})
	if err != nil {
		return nil, err
	}

	if result.Status != authn.LoginAuthenticated {
		// No session cookie and no user: the caller has proved a password and
		// nothing more, and until the second factor is presented (M1-006) there
		// is no session to describe.
		//
		// The credential was right, so this is not a failed attempt — and it is
		// not a finished one either. Saying so keeps the throttle from clearing
		// the account's failure count, which would let somebody holding the
		// password reset their code-guessing budget between every guess.
		markCredentialIncomplete(ctx)

		body := gen.Login200JSONResponse{
			Body: gen.LoginResult{Status: gen.LoginStatusMfaRequired},
		}
		if result.Challenge.Token != "" {
			// A challenge the caller can actually answer. Its absence means MFA
			// is enforced with nothing enrolled — the dead end M1-008 removes —
			// and there is deliberately nothing to hand out in that case.
			cookie := h.challenges.Cookie(
				result.Challenge.Token, result.Challenge.Challenge.ExpiresAt).String()
			body.Headers = gen.Login200ResponseHeaders{SetCookie: &cookie}
		}
		return body, nil
	}

	profile, err := h.auth.Profile(ctx, result.Subject)
	if err != nil {
		return nil, err
	}
	// Derived from the token that is going into the cookie, so the body and the
	// two Set-Cookie headers describe the same session. The second cookie is
	// added on the way out, by the CSRF middleware (M1-005) — see csrfWriter.
	user := currentUser(profile, h.sessions.CSRFToken(result.Issued.Token))
	cookie := h.sessions.Cookie(result.Issued.Token, result.Issued.Session.ExpiresAt).String()

	return gen.Login200JSONResponse{
		Body: gen.LoginResult{
			Status: gen.LoginStatusAuthenticated,
			User:   &user,
		},
		Headers: gen.Login200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

// Logout revokes the current session and clears the cookie.
//
// A request with no usable session is a 204 as well. The caller asked to be
// signed out and they are, and answering 401 would leave a browser holding a
// dead cookie it was never told to drop.
func (h *handlers) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if subject, ok := authn.SubjectFrom(ctx); ok {
		if err := h.auth.Logout(ctx, subject); err != nil {
			return nil, err
		}
	}
	// Cleared whether or not there was anything to revoke: the point is the
	// browser's copy, and the server-side revocation above is what makes a
	// replay of it useless.
	return gen.Logout204Response{
		Headers: gen.Logout204ResponseHeaders{SetCookie: h.sessions.ClearCookie().String()},
	}, nil
}

// GetCurrentUser reports the signed-in caller. It is what the interface builds
// itself from, so it is read fresh rather than served out of the session.
func (h *handlers) GetCurrentUser(ctx context.Context, _ gen.GetCurrentUserRequestObject) (gen.GetCurrentUserResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.auth.Profile(ctx, subject)
	if err != nil {
		return nil, err
	}
	// Empty for a caller who did not authenticate by cookie, which is the one
	// case where the field is absent from the body — there is no CSRF check on
	// such a request to give it a token for.
	return gen.GetCurrentUser200JSONResponse(currentUser(profile, csrfTokenFrom(ctx))), nil
}

// ChangePassword replaces the caller's own password, rotates this session onto a
// new token and revokes every other one they have.
func (h *handlers) ChangePassword(ctx context.Context, request gen.ChangePasswordRequestObject) (gen.ChangePasswordResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	issued, err := h.auth.ChangePassword(ctx,
		subject,
		password.Plaintext(request.Body.CurrentPassword),
		password.Plaintext(request.Body.NewPassword))
	if err != nil {
		return nil, err
	}

	return gen.ChangePassword204Response{
		Headers: gen.ChangePassword204ResponseHeaders{
			SetCookie: h.sessions.Cookie(issued.Token, issued.Session.ExpiresAt).String(),
		},
	}, nil
}

// currentUser renders a profile for the wire. It is the only place a user
// becomes JSON, so there is one answer to what /auth/me and the login response
// contain — and nothing about the session token is in the struct it fills, so
// there is nothing to leave out by mistake.
//
// csrfToken is the one value here that is not part of the profile. It is "" for
// a caller with no cookie session, and omitted from the body when it is.
func currentUser(profile authn.Profile, csrfToken string) gen.CurrentUser {
	memberships := make([]gen.EngagementMembership, 0, len(profile.Memberships))
	for _, membership := range profile.Memberships {
		memberships = append(memberships, gen.EngagementMembership{
			EngagementId: membership.EngagementID,
			Role:         gen.EngagementRole(membership.Role),
			AddedAt:      membership.AddedAt,
		})
	}

	user := gen.CurrentUser{
		Id:           profile.User.ID,
		Email:        profile.User.Email,
		DisplayName:  profile.User.DisplayName,
		PlatformRole: gen.PlatformRole(profile.User.PlatformRole),
		Mfa:          mfaState(profile),
		Memberships:  memberships,
	}
	if csrfToken != "" {
		user.CsrfToken = &csrfToken
	}
	return user
}
