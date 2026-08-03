package authn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/authn/servicetoken"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Service tokens as an *authentication method* and as something a person
// manages (M1-011).
//
// The first half is [Service.AuthenticateToken], and its shape is the point of
// the ticket: it produces the same [Subject] that [Service.Authenticate]
// produces from a cookie, so nothing above this line branches on how a request
// proved who it is. v1's API keys were checked — where they were checked at all
// — by code that ran nowhere near the session path, which is how they came to
// authenticate nothing.
//
// The second half is three thin wrappers. They exist so that the HTTP layer
// keeps translating and nothing else, and so that "a caller manages their own
// tokens and nobody else's" is stated once, here, as the owner argument every
// one of them passes.

// AuthenticateToken resolves a presented service token to the caller it belongs
// to.
//
// It reports [servicetoken.ErrNoToken] for every way of not being authenticated
// by one, including an owner whose account has been disabled since the token was
// created — disabling somebody must end their access now, and their tokens are
// their access. Any other error is the database failing, and the caller must not
// report it as a failure to authenticate.
//
// The permissions on the [Subject] are the owner's, read fresh on every request.
// That is the first of PLAN.md §9's two fences and it is enforced by there being
// nowhere else for them to come from: nothing about a role is stored on the
// token row, so a demotion applies at the demoted person's next request and
// there is no cached copy to invalidate.
func (s *Service) AuthenticateToken(ctx context.Context, presented servicetoken.Token) (Subject, error) {
	found, err := s.tokens.Resolve(ctx, presented)
	if err != nil {
		return Subject{}, err
	}

	user, err := s.users.ByID(ctx, found.OwnerUserID)
	if errors.Is(err, apierr.ErrNotFound) {
		return Subject{}, fmt.Errorf("%w: token %s belongs to user %s, which is gone",
			servicetoken.ErrNoToken, found.ID, found.OwnerUserID)
	}
	if err != nil {
		return Subject{}, err
	}
	if user.Status != identity.StatusActive {
		return Subject{}, fmt.Errorf("%w: token %s belongs to user %s, who is %s",
			servicetoken.ErrNoToken, found.ID, user.ID, user.Status)
	}

	// No MFA state and no session. A token presented a factor at no point and
	// belongs to no session, so MFASatisfied is false and stays false — and
	// MFAEnrolmentRequired is false as well, deliberately: the enrolment gate
	// exists to walk a *person* to the screen that fixes their account, and
	// confining a pipeline to an interactive enrolment flow would strand it
	// with nothing it could do. What holds the token's owner to the policy is
	// their own next sign-in.
	return Subject{
		UserID:            user.ID,
		Email:             user.Email,
		DisplayName:       user.DisplayName,
		PlatformRole:      user.PlatformRole,
		Method:            authz.MethodServiceToken,
		TokenID:           found.ID,
		TokenScopes:       found.Scopes,
		TokenEngagementID: found.EngagementID,
	}, nil
}

// CreateToken mints a token for the caller, and returns it with the one and
// only copy of its secret.
//
// The owner and the issuer are both the caller. There is no way through this
// function to create a token for anybody else, which is what makes the
// operation's `x-authz-action: token.manage` mapping honest: the policy says
// everybody holds it, and the scoping — over their own account and nothing
// else — is here rather than in a role.
func (s *Service) CreateToken(ctx context.Context, subject Subject, in NewToken) (servicetoken.Issued, error) {
	return s.tokens.Issue(ctx, servicetoken.NewToken{
		Name:         in.Name,
		OwnerUserID:  subject.UserID,
		CreatedBy:    subject.UserID,
		Scopes:       in.Scopes,
		EngagementID: in.EngagementID,
		ExpiresAt:    in.ExpiresAt,
	})
}

// NewToken is what a caller asked for, without the owner: that is the caller,
// and is not theirs to choose.
type NewToken struct {
	Name string

	// Scopes is the wire spelling, unchecked. See [servicetoken.NewToken],
	// which says why the words stay words until one place turns them into
	// scopes.
	Scopes       []string
	EngagementID string
	ExpiresAt    time.Time
}

// ListTokens returns the caller's own tokens, newest first. It never returns a
// secret, because no secret exists to return.
func (s *Service) ListTokens(ctx context.Context, subject Subject) ([]identity.ServiceToken, error) {
	return s.tokens.List(ctx, subject.UserID)
}

// RevokeToken ends one of the caller's own tokens.
//
// A token belonging to somebody else is [apierr.NotFound] and so is one that
// never existed — the ownership is part of the statement rather than a check in
// front of it, so the two cannot be told apart by trying.
func (s *Service) RevokeToken(ctx context.Context, subject Subject, id string) (identity.ServiceToken, error) {
	return s.tokens.Revoke(ctx, id, subject.UserID)
}
