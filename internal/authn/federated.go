package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Signing in with an identity somebody else vouched for (M1-009, and M1-010
// when SAML lands on the same door).
//
// Everything protocol-shaped happened before this file: internal/authn/oidc
// verified a signature, an issuer, an audience, an expiry and a nonce, and what
// arrives here is claims that have been proved. What is left is the four
// decisions this deployment owns, and they are the ones a hand-rolled
// integration usually gets wrong:
//
//  1. Which account is this? By subject, always — see [FederatedLogin.Subject].
//  2. If none, may one be created, or linked to an existing local account?
//  3. What may they do — which is asked *every* time, not at provisioning.
//  4. Does a second factor still apply? The same question a local sign-in asks,
//     answered by the same code (see completeSignIn).

// Identities is the part of the identity store the federated path needs.
// [*identity.Identities] satisfies it.
type Identities interface {
	BySubject(ctx context.Context, provider identity.Provider, subject string) (identity.Identity, error)
	Create(ctx context.Context, in identity.NewIdentity) (identity.Identity, error)
}

// FederatedLogin is a verified sign-in from an identity provider.
//
// Everything here has been checked by the package that produced it. This one
// neither knows nor cares which protocol it came from, which is what makes SAML
// (M1-010) a second caller rather than a second copy of this logic.
type FederatedLogin struct {
	// Provider is which kind of identity provider vouched for this, and Subject
	// is what that provider calls this person.
	Provider identity.Provider
	Subject  string

	// Email is the address the provider asserted, and EmailVerified is whether
	// it says the address has been proved. The second one decides whether this
	// login may be attached to an *existing* account — see [Service.SignInWithFederatedIdentity].
	Email         string
	EmailVerified bool

	// DisplayName is what to call them if an account has to be created.
	DisplayName string

	// Role is the platform role their groups map to, and RoleMapped is whether
	// any group mapped at all. The two are separate because "no mapped group" is
	// not the same fact as "mapped to member": the first means the deployment's
	// mapping says nothing about this person, which is what the default is for.
	Role       authz.PlatformRole
	RoleMapped bool

	// AutoProvision is whether this deployment creates accounts for people the
	// provider vouches for and this installation has never seen.
	AutoProvision bool

	// Request is what to record about where the session was created.
	Request session.Request
}

// SignInWithFederatedIdentity signs somebody in on a verified assertion from an
// identity provider, creating or linking the account if this deployment's
// configuration says to.
//
// It returns the same [LoginResult] a local sign-in returns, including the two
// outcomes that are not a session: a deployment that requires a second factor
// requires it of everybody, and an account reached through an identity provider
// is not an exception to a policy an administrator set here. (An SSO-only
// account — one with no local password — is exempt by
// [MFAPolicy.Requires], which is where that rule lives and is explained.)
func (s *Service) SignInWithFederatedIdentity(ctx context.Context, in FederatedLogin) (LoginResult, error) {
	if strings.TrimSpace(in.Subject) == "" {
		// Unreachable from the OIDC package, which refuses a token with no
		// subject. Checked anyway: an empty subject would match the empty subject
		// of the next person through the door.
		return LoginResult{}, apierr.Internal(errors.New(
			"authn: a federated sign-in arrived with no subject"))
	}

	user, err := s.federatedUser(ctx, in)
	if err != nil {
		return LoginResult{}, err
	}
	user, err = s.claimInvitation(ctx, user)
	if err != nil {
		return LoginResult{}, err
	}
	if user.Status != identity.StatusActive {
		// Said plainly rather than hidden behind the generic refusal. They have
		// just proved who they are at the provider, so there is nothing here they
		// do not already know, and "your account is disabled" is what stops them
		// from filing a bug about single sign-on being broken.
		return LoginResult{}, apierr.SignInRefused(
			"this account is not active here; ask an administrator",
			fmt.Sprintf("user %s is %s", user.ID, user.Status))
	}

	user, err = s.applyMappedRole(ctx, user, in)
	if err != nil {
		return LoginResult{}, err
	}

	result, err := s.completeSignIn(ctx, user, in.Request)
	if err != nil {
		return LoginResult{}, err
	}
	s.log.InfoContext(ctx, "federated sign-in",
		slog.String("user_id", user.ID),
		slog.String("provider", string(in.Provider)),
		slog.String("status", string(result.Status)))
	return result, nil
}

// federatedUser finds the account this assertion belongs to, linking or
// creating one where that is allowed.
//
// The order is the security decision in this file:
//
//  1. By (provider, subject). The subject is the only identifier a provider
//     promises not to reuse or reassign, so it is the only one an account is
//     ever *found* by.
//  2. By verified email, which links this login to an existing local account.
//     Only when the provider says the address is verified: without that check,
//     anybody who can set an unverified address at the provider — self-service
//     signup at a provider with a permissive tenant, or a directory that lets
//     users edit their own mail attribute — can claim any account here by
//     typing its address.
//  3. Provisioning, if this deployment does that.
//  4. Refusal, with a message that tells them what to do about it.
func (s *Service) federatedUser(ctx context.Context, in FederatedLogin) (identity.User, error) {
	linked, err := s.identities.BySubject(ctx, in.Provider, in.Subject)
	switch {
	case err == nil:
		return s.users.ByID(ctx, linked.UserID)
	case !errors.Is(err, apierr.ErrNotFound):
		return identity.User{}, err
	}

	if in.Email != "" && in.EmailVerified {
		existing, err := s.users.ByEmail(ctx, in.Email)
		switch {
		case err == nil:
			if _, err := s.identities.Create(ctx, identity.NewIdentity{
				UserID:   existing.ID,
				Provider: in.Provider,
				Subject:  in.Subject,
			}); err != nil {
				return identity.User{}, err
			}
			// Warn: attaching a new way into an existing account is the sort of
			// thing somebody reads back afterwards. It is also the only line that
			// records which provider subject now owns this account.
			s.log.WarnContext(ctx, "linked a federated identity to an existing account",
				slog.String("user_id", existing.ID),
				slog.String("provider", string(in.Provider)),
				slog.String("subject", in.Subject),
				slog.String("email", existing.Email))
			s.recordAlone(ctx, events.Entry{
				ActorID:    existing.ID,
				Verb:       events.VerbSSOLinked,
				ObjectType: events.ObjectIdentity,
				ObjectID:   existing.ID,
				Delta: events.Delta(map[string]any{
					"provider": string(in.Provider),
					"subject":  in.Subject,
					"email":    existing.Email,
				}),
			})
			return existing, nil
		case !errors.Is(err, apierr.ErrNotFound):
			return identity.User{}, err
		}
	}

	if !in.AutoProvision {
		// Nothing is written. The account does not exist, and a deployment that
		// has turned provisioning off means that.
		s.log.InfoContext(ctx, "refused a federated sign-in for an account that does not exist",
			slog.String("provider", string(in.Provider)),
			slog.String("subject", in.Subject),
			slog.Bool("email_verified", in.EmailVerified))
		return identity.User{}, apierr.SignInRefused(
			"you signed in successfully, but you have no account on this Blacklight; "+
				"ask an administrator to create one",
			fmt.Sprintf("no user is linked to %s subject %q and automatic provisioning is off",
				in.Provider, in.Subject))
	}
	return s.provision(ctx, in)
}

// claimInvitation activates an account an administrator created for somebody
// who had not signed in yet (M1-016), and leaves every other account alone.
//
// [identity.StatusInvited] means "exists but has never been claimed", and this
// is the moment it is claimed: an administrator created the account without a
// password precisely so that the identity provider would be the thing that let
// its owner in, and the provider has just vouched for them. Without this the
// invited state would be a dead end — no local password to sign in with, and a
// federated sign-in refused for not being active.
//
// A `disabled` account is not touched. Somebody turned it off deliberately, and
// proving who you are at the provider is not an argument against that.
func (s *Service) claimInvitation(ctx context.Context, user identity.User) (identity.User, error) {
	if user.Status != identity.StatusInvited {
		return user, nil
	}

	claimed := user
	claimed.Status = identity.StatusActive
	// The actor is the person themselves: nobody administered this, they turned
	// up. The row is a user.enabled, the same verb an administrator's enable
	// writes, because the fact recorded is the same one.
	claimed, err := s.users.Update(ctx, claimed,
		s.recordInTx(user.ID, events.VerbUserEnabled, map[string]any{
			"status": change(string(user.Status), string(identity.StatusActive)),
			"reason": "claimed by a federated sign-in",
		}))
	if err != nil {
		return identity.User{}, err
	}

	s.log.InfoContext(ctx, "an invited account was claimed by a federated sign-in",
		slog.String("user_id", claimed.ID),
		slog.String("email", claimed.Email))
	return claimed, nil
}

// provision creates the account for somebody arriving for the first time.
//
// The new account has no password hash, which is what makes it an SSO-only
// account: [Service.Login] refuses it, and the MFA policy exempts it because
// there is no local sign-in for a local second factor to stand behind.
func (s *Service) provision(ctx context.Context, in FederatedLogin) (identity.User, error) {
	if in.Email == "" {
		// An account is identified by an address everywhere in this application
		// — it is what a membership is offered to and what an operator searches
		// by — so there is nothing sensible to create here. It means the provider
		// sent no `email` claim, which is a scope or a claim-mapping problem at
		// the provider and is worth saying out loud.
		return identity.User{}, apierr.SignInRefused(
			"the identity provider sent no email address, so no account could be created; "+
				"ask an administrator to check the provider's claim mapping",
			fmt.Sprintf("%s subject %q arrived with no email claim", in.Provider, in.Subject))
	}

	role := authz.PlatformRoleMember
	if in.RoleMapped {
		role = in.Role
	}
	displayName := in.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = in.Email
	}

	created, err := s.users.Create(ctx, identity.NewUser{
		Email:       in.Email,
		DisplayName: displayName,
		// No password hash: see the comment above.
		PlatformRole: role,
		Status:       identity.StatusActive,
	})
	if err != nil {
		return identity.User{}, err
	}
	if _, err := s.identities.Create(ctx, identity.NewIdentity{
		UserID:   created.ID,
		Provider: in.Provider,
		Subject:  in.Subject,
	}); err != nil {
		return identity.User{}, err
	}

	s.log.WarnContext(ctx, "provisioned an account from a federated sign-in",
		slog.String("user_id", created.ID),
		slog.String("provider", string(in.Provider)),
		slog.String("subject", in.Subject),
		slog.String("email", created.Email),
		slog.String("platform_role", string(created.PlatformRole)))
	s.recordAlone(ctx, events.Entry{
		ActorID:    created.ID,
		Verb:       events.VerbUserCreated,
		ObjectType: events.ObjectUser,
		ObjectID:   created.ID,
		Delta: events.Delta(map[string]any{
			"email":         created.Email,
			"platform_role": string(created.PlatformRole),
			"provider":      string(in.Provider),
		}),
	})
	s.recordAlone(ctx, events.Entry{
		ActorID:    created.ID,
		Verb:       events.VerbSSOProvisioned,
		ObjectType: events.ObjectIdentity,
		ObjectID:   created.ID,
		Delta: events.Delta(map[string]any{
			"provider": string(in.Provider),
			"subject":  in.Subject,
			"email":    created.Email,
		}),
	})
	return created, nil
}

// applyMappedRole brings the account's platform role into line with what the
// provider's groups say, on every single sign-on.
//
// Every time, not only at provisioning: a group removed at the provider has to
// take effect here, and the only moment this deployment finds out about it is
// the next login. That includes demotion out of admin, which is the direction
// that matters — an integration that only ever promotes is one where revoking
// access at the directory does nothing at all.
//
// An unmapped person is left alone rather than reset to member. The two are
// different facts: "your groups say member" is a decision, and "none of your
// groups is in the mapping" is the configuration having nothing to say — and on
// a deployment with no mapping at all, resetting would quietly demote every
// administrator who signs in through the provider.
func (s *Service) applyMappedRole(ctx context.Context, user identity.User, in FederatedLogin) (identity.User, error) {
	if !in.RoleMapped || user.PlatformRole == in.Role {
		return user, nil
	}

	was := user.PlatformRole
	user.PlatformRole = in.Role
	updated, err := s.users.Update(ctx, user)
	if err != nil {
		return identity.User{}, err
	}

	// Warn in both directions. A promotion is worth reading back afterwards, and
	// a demotion is worth being able to point at when somebody asks why their
	// access changed without anybody here touching anything.
	s.log.WarnContext(ctx, "a federated sign-in changed a platform role",
		slog.String("user_id", updated.ID),
		slog.String("provider", string(in.Provider)),
		slog.String("from", string(was)),
		slog.String("to", string(updated.PlatformRole)))
	s.recordAlone(ctx, events.Entry{
		ActorID:    updated.ID,
		Verb:       events.VerbUserRoleChanged,
		ObjectType: events.ObjectUser,
		ObjectID:   updated.ID,
		Delta: events.Delta(map[string]any{
			"from":     string(was),
			"to":       string(updated.PlatformRole),
			"provider": string(in.Provider),
		}),
	})
	return updated, nil
}
