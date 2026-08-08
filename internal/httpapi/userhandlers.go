package httpapi

import (
	"context"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The user administration endpoints (M1-016) — the family v1 shipped with no
// gate on it at all, which is the defect PLAN.md §4 names.
//
// Like every other handler in this package they translate and nothing else.
// What an edit means, which combinations are refused, what disabling somebody
// stops and what goes in the activity log all live in internal/authn; *who* may
// call these is decided by the middleware from api/openapi.yaml, before any
// function below is entered (M1-013).
//
// Two things in this file are worth reading as design rather than as plumbing:
//
//   - The role and the status travel as the strings they arrived as.
//     internal/authn resolves them. A handler that could name a role is a
//     handler that could decide with one, and TestNoHandlerDecidesForItself
//     fails the build if one of these files so much as imports internal/authz.
//   - [handlers.UpdateCurrentUser] takes a different request type from
//     [handlers.UpdateUser], not the same type with fewer fields honoured. The
//     schema is the fence: there is no `platformRole` in `UpdateSelfRequest`, so
//     a body carrying one is rejected by the request validator with a 400
//     before this code runs.

// ListUsers returns a page of accounts.
func (h *handlers) ListUsers(ctx context.Context,
	request gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	users, next, err := h.auth.ListAccounts(ctx, authn.AccountFilter{
		Status: stringParam(request.Params.Status),
		Role:   stringParam(request.Params.Role),
		Search: stringParam(request.Params.Q),
		Cursor: stringParam(request.Params.Cursor),
		Limit:  limitParam(request.Params.Limit),
	})
	if err != nil {
		return nil, err
	}

	// Non-nil, so an installation with no matching account renders as `[]`
	// rather than `null` — the generated client types the field as an array
	// either way, and a client that trusted that would find out otherwise here.
	items := make([]gen.User, 0, len(users))
	for _, u := range users {
		rendered, err := user(u)
		if err != nil {
			return nil, err
		}
		items = append(items, rendered)
	}
	page := gen.UserPage{Items: items}
	if next != "" {
		page.NextCursor = nullable.NewNullableWithValue(next)
	}
	return gen.ListUsers200JSONResponse(page), nil
}

// CreateUser creates an account and says where to send the person it belongs to.
func (h *handlers) CreateUser(ctx context.Context,
	request gen.CreateUserRequestObject) (gen.CreateUserResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	created, err := h.auth.CreateAccount(ctx, subject, authn.NewAccount{
		Email:       string(request.Body.Email),
		DisplayName: request.Body.DisplayName,
		Password:    password.Plaintext(stringValue(request.Body.Password)),
		Role:        string(request.Body.PlatformRole),
		Status:      stringParam(request.Body.Status),
		MFAEnforced: boolValue(request.Body.MfaEnforced),
	})
	if err != nil {
		return nil, err
	}

	rendered, err := user(created)
	if err != nil {
		return nil, err
	}
	return gen.CreateUser201JSONResponse{User: rendered, InviteUrl: h.signInURL}, nil
}

// GetUser returns one account.
func (h *handlers) GetUser(ctx context.Context,
	request gen.GetUserRequestObject) (gen.GetUserResponseObject, error) {
	found, err := h.auth.Account(ctx, request.UserId.String())
	if err != nil {
		return nil, err
	}
	rendered, err := user(found)
	if err != nil {
		return nil, err
	}
	return gen.GetUser200JSONResponse(rendered), nil
}

// UpdateUser applies an administrator's patch to one account.
func (h *handlers) UpdateUser(ctx context.Context,
	request gen.UpdateUserRequestObject) (gen.UpdateUserResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	updated, err := h.auth.UpdateAccount(ctx, subject, request.UserId.String(), authn.AccountEdit{
		DisplayName: request.Body.DisplayName,
		Role:        wireWord(request.Body.PlatformRole),
		Status:      wireWord(request.Body.Status),
		MFAEnforced: request.Body.MfaEnforced,
	})
	if err != nil {
		return nil, err
	}
	rendered, err := user(updated)
	if err != nil {
		return nil, err
	}
	return gen.UpdateUser200JSONResponse(rendered), nil
}

// UpdateCurrentUser changes the caller's own display name.
//
// It names no account: the one it edits is the one the request authenticated
// as. That is what makes `x-authz-self` honest here, and it is why there is no
// path parameter for api.Requirements to refuse.
func (h *handlers) UpdateCurrentUser(ctx context.Context,
	request gen.UpdateCurrentUserRequestObject) (gen.UpdateCurrentUserResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	updated, err := h.auth.RenameSelf(ctx, subject, request.Body.DisplayName)
	if err != nil {
		return nil, err
	}
	rendered, err := user(updated)
	if err != nil {
		return nil, err
	}
	return gen.UpdateCurrentUser200JSONResponse(rendered), nil
}

// DisableUser ends an account's access, and DeleteUser is the same operation
// under the name a client that thinks in resources reaches for. Both answer with
// the account, which is still there — the delete is soft, per M1-001.
func (h *handlers) DisableUser(ctx context.Context,
	request gen.DisableUserRequestObject) (gen.DisableUserResponseObject, error) {
	updated, err := h.setStatus(ctx, request.UserId.String(), identity.StatusDisabled)
	if err != nil {
		return nil, err
	}
	return gen.DisableUser200JSONResponse(updated), nil
}

func (h *handlers) DeleteUser(ctx context.Context,
	request gen.DeleteUserRequestObject) (gen.DeleteUserResponseObject, error) {
	updated, err := h.setStatus(ctx, request.UserId.String(), identity.StatusDisabled)
	if err != nil {
		return nil, err
	}
	return gen.DeleteUser200JSONResponse(updated), nil
}

// EnableUser turns an account back on, and is also how an `invited` account that
// will never use single sign-on is claimed by hand.
func (h *handlers) EnableUser(ctx context.Context,
	request gen.EnableUserRequestObject) (gen.EnableUserResponseObject, error) {
	updated, err := h.setStatus(ctx, request.UserId.String(), identity.StatusActive)
	if err != nil {
		return nil, err
	}
	return gen.EnableUser200JSONResponse(updated), nil
}

// RevokeUserSessions signs an account out everywhere without disabling it.
func (h *handlers) RevokeUserSessions(ctx context.Context,
	request gen.RevokeUserSessionsRequestObject) (gen.RevokeUserSessionsResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	revoked, err := h.auth.RevokeAccountSessions(ctx, subject, request.UserId.String())
	if err != nil {
		return nil, err
	}
	return gen.RevokeUserSessions200JSONResponse{Revoked: int(revoked)}, nil
}

// setStatus is the body the three status endpoints share, so that disabling
// through `DELETE`, through `/disable` and through a `PATCH` cannot come to mean
// three different things.
func (h *handlers) setStatus(ctx context.Context, id string, status identity.Status) (gen.User, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return gen.User{}, err
	}
	updated, err := h.auth.SetAccountStatus(ctx, subject, id, status)
	if err != nil {
		return gen.User{}, err
	}
	return user(updated)
}

// user renders one account for the wire, in one place, so the seven responses
// that carry one cannot describe it differently.
//
// It could not carry a secret if it tried: the password hash, the authenticator
// secret and the session token are not fields of [gen.User], and this is the
// only function that builds one.
func user(u identity.User) (gen.User, error) {
	// Parsed rather than asserted, for the reason serviceToken parses its own:
	// the column is TEXT, and a row whose identifier is not a UUID is a damaged
	// row rather than something to hand a client and let it send back.
	id, err := parseUUID(u.ID)
	if err != nil {
		return gen.User{}, err
	}

	out := gen.User{
		Id:           id,
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		PlatformRole: gen.PlatformRole(u.PlatformRole),
		Status:       gen.UserStatus(u.Status),
		MfaEnforced:  u.MFAEnforced,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	if !u.LastLoginAt.IsZero() {
		out.LastLoginAt = &u.LastLoginAt
	}
	return out, nil
}

// wireWord hands an optional enum down as the word it arrived as, leaving nil as
// nil so that "the request did not mention this field" survives the trip.
//
// It exists because a handler in this package may not name a role: the string is
// resolved into internal/authz's vocabulary in internal/authn, where the one
// definition of the word lives.
func wireWord[T ~string](p *T) *string {
	if p == nil {
		return nil
	}
	word := string(*p)
	return &word
}

// stringValue and boolValue read an optional field with the zero value standing
// for "absent", which is what the fields using them mean by it: no password, and
// MFA not enforced.
func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolValue(p *bool) bool {
	return p != nil && *p
}
