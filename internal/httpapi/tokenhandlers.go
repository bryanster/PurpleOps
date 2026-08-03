package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The service token endpoints (M1-011). Like every other handler in this
// package they translate and nothing else: what a token is, how long it may
// live and what happens when one is presented all live in
// internal/authn/servicetoken.
//
// *Who* may call these is not decided here either. api/openapi.yaml maps the
// three operations to `token.read` and `token.manage`, and the authorization
// middleware refuses anybody who does not hold them before any function below
// is entered (M1-013) — including a caller presenting a service token, whom the
// session-only guard in internal/authz refuses whatever scopes they carry.
//
// What *is* here is the scoping, and it is one word repeated: every call below
// passes the caller as the owner, so there is no argument any of these functions
// could be given that would reach somebody else's token.

// ListServiceTokens returns the caller's own tokens.
func (h *handlers) ListServiceTokens(ctx context.Context,
	_ gen.ListServiceTokensRequestObject) (gen.ListServiceTokensResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := h.auth.ListTokens(ctx, subject)
	if err != nil {
		return nil, err
	}

	// Non-nil, so the list renders as `[]` rather than `null` for somebody who
	// holds none. The generated client types the field as an array either way,
	// and a client that trusted that would find out otherwise on this response.
	items := make([]gen.ServiceToken, 0, len(tokens))
	for _, token := range tokens {
		rendered, err := serviceToken(token, time.Now())
		if err != nil {
			return nil, err
		}
		items = append(items, rendered)
	}
	return gen.ListServiceTokens200JSONResponse{Items: items}, nil
}

// CreateServiceToken mints one and returns it with its secret, once.
func (h *handlers) CreateServiceToken(ctx context.Context,
	request gen.CreateServiceTokenRequestObject) (gen.CreateServiceTokenResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	issued, err := h.auth.CreateToken(ctx, subject, authn.NewToken{
		Name:         request.Body.Name,
		Scopes:       tokenScopes(request.Body.Scopes),
		EngagementID: uuidOrEmpty(request.Body.EngagementId),
		ExpiresAt:    request.Body.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	// The one response in this API that carries a token, and the only place
	// [servicetoken.Token.Reveal] is called outside the package that defines
	// it. Everywhere else the type renders as "[redacted]", which is what makes
	// "exactly one response, ever" a property of the code rather than a rule.
	rendered, err := serviceToken(issued.ServiceToken, time.Now())
	if err != nil {
		return nil, err
	}
	return gen.CreateServiceToken201JSONResponse{
		ServiceToken: rendered,
		Token:        issued.Token.Reveal(),
	}, nil
}

// RevokeServiceToken ends one of the caller's own tokens.
//
// A token belonging to somebody else answers 404, the same as an identifier
// that names nothing — the ownership is part of the statement that does the
// revoking, so the two cannot be told apart by trying.
func (h *handlers) RevokeServiceToken(ctx context.Context,
	request gen.RevokeServiceTokenRequestObject) (gen.RevokeServiceTokenResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.auth.RevokeToken(ctx, subject, request.TokenId.String()); err != nil {
		return nil, err
	}
	return gen.RevokeServiceToken204Response{}, nil
}

// serviceToken renders one row for the wire, in one place, so the two responses
// that carry a token cannot describe it differently.
//
// It never carries a secret and could not: the row holds a hash, and the hash is
// not a field of the wire type.
func serviceToken(token identity.ServiceToken, now time.Time) (gen.ServiceToken, error) {
	// Parsed rather than asserted: the column is TEXT, and a row whose
	// identifier is not a UUID is a damaged row rather than a request anybody
	// can act on. It is answered as the fault it is, not with a panic and not
	// with a zero identifier a client would go on to send back.
	id, err := uuid.Parse(token.ID)
	if err != nil {
		return gen.ServiceToken{}, apierr.Internal(
			fmt.Errorf("service token %q does not have a UUID identifier: %w", token.ID, err))
	}

	out := gen.ServiceToken{
		Id:        id,
		Name:      token.Name,
		Prefix:    token.Prefix,
		Scopes:    make([]gen.TokenScope, 0, len(token.Scopes)),
		Status:    serviceTokenStatus(token, now),
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
	}
	for _, scope := range token.Scopes {
		out.Scopes = append(out.Scopes, gen.TokenScope(scope))
	}
	if token.EngagementID != "" {
		engagement, err := uuid.Parse(token.EngagementID)
		if err != nil {
			return gen.ServiceToken{}, apierr.Internal(
				fmt.Errorf("service token %s is bound to %q, which is not a UUID: %w", token.ID, token.EngagementID, err))
		}
		out.EngagementId = &engagement
	}
	if !token.LastUsedAt.IsZero() {
		out.LastUsedAt = &token.LastUsedAt
	}
	if !token.RevokedAt.IsZero() {
		out.RevokedAt = &token.RevokedAt
	}
	return out, nil
}

// serviceTokenStatus derives what a client renders in a list, so that every
// client does not compare two timestamps and one of them gets it wrong.
//
// Revoked beats expired where both are true. A token that was revoked and has
// since run out is still a token somebody decided to end, and that is the fact
// worth showing.
func serviceTokenStatus(token identity.ServiceToken, now time.Time) gen.ServiceTokenStatus {
	switch {
	case !token.RevokedAt.IsZero():
		return gen.ServiceTokenStatusRevoked
	case !now.Before(token.ExpiresAt):
		return gen.ServiceTokenStatusExpired
	default:
		return gen.ServiceTokenStatusActive
	}
}

// tokenScopes hands the requested scopes down as the words they arrived as.
//
// It does not check them, and this file could not: internal/authz owns the
// vocabulary, and a handler that imported it would be a handler that could name
// a role — which TestNoHandlerDecidesForItself fails the build over, for the
// reason PLAN.md §4 gives. The words are turned into scopes, or refused with a
// field error naming the one that is not, in internal/authn/servicetoken.
func tokenScopes(scopes []gen.TokenScope) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, string(scope))
	}
	return out
}

// uuidOrEmpty renders an optional identifier as the empty string the domain
// uses for "none", so that no layer below this one has to handle a nil pointer.
func uuidOrEmpty(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
