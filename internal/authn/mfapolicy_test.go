package authn

import (
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The two pure functions M1-008 turns on. They are tested here rather than only
// through the HTTP layer for two reasons: they are total over a small space, so
// a table can cover every case rather than a representative few, and one of the
// cases — the SSO-only account — cannot be reached through a sign-in at all,
// because an account with no local password cannot present one.

func TestMFAPolicyRequires(t *testing.T) {
	t.Parallel()

	// local is an ordinary account: a password, no per-user flag, no role that
	// attracts a policy of its own.
	local := identity.User{
		PasswordHash: "argon2id$whatever",
		PlatformRole: authz.PlatformRoleMember,
	}
	admin := local
	admin.PlatformRole = authz.PlatformRoleAdmin

	tests := map[string]struct {
		policy MFAPolicy
		user   func(identity.User) identity.User
		on     identity.User
		want   bool
	}{
		"no policy, no flag": {on: local},

		"required for all": {
			policy: MFAPolicy{RequiredForAll: true},
			on:     local,
			want:   true,
		},
		"required for admins, and they are one": {
			policy: MFAPolicy{RequiredForAdmins: true},
			on:     admin,
			want:   true,
		},
		// The half that would be invisible without it: the narrower policy has
		// to be narrower.
		"required for admins, and they are not": {
			policy: MFAPolicy{RequiredForAdmins: true},
			on:     local,
		},
		// The per-user flag is the other half of the **or**, and it is the one
		// that must keep working with the platform policy off — otherwise
		// enforcing MFA of one person would mean enforcing it of everybody.
		"flagged individually with no policy": {
			on:   local,
			user: func(u identity.User) identity.User { u.MFAEnforced = true; return u },
			want: true,
		},
		"flagged individually and required anyway": {
			policy: MFAPolicy{RequiredForAll: true},
			on:     local,
			user:   func(u identity.User) identity.User { u.MFAEnforced = true; return u },
			want:   true,
		},

		// An account with no local password is one that signs in through an
		// identity provider (M1-009, M1-010). It presents no password here, so
		// there is no local sign-in for a local second factor to stand behind,
		// and the provider is where its factors are asserted. Exempt from every
		// one of the three inputs, deliberately — including the flag, because a
		// requirement they have no way to satisfy is a lockout rather than a
		// policy. docs/security.md states this and what changes when an IdP
		// tells us what it verified.
		"SSO-only account, required for all": {
			policy: MFAPolicy{RequiredForAll: true},
			on:     local,
			user:   func(u identity.User) identity.User { u.PasswordHash = ""; return u },
		},
		"SSO-only administrator, required for admins": {
			policy: MFAPolicy{RequiredForAdmins: true},
			on:     admin,
			user:   func(u identity.User) identity.User { u.PasswordHash = ""; return u },
		},
		"SSO-only account, flagged individually": {
			on: local,
			user: func(u identity.User) identity.User {
				u.PasswordHash = ""
				u.MFAEnforced = true
				return u
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			user := test.on
			if test.user != nil {
				user = test.user(user)
			}
			if got := test.policy.Requires(user); got != test.want {
				t.Errorf("MFAPolicy%+v.Requires(%s, enforced=%t, local=%t) = %t, want %t",
					test.policy, user.PlatformRole, user.MFAEnforced,
					user.PasswordHash != "", got, test.want)
			}
		})
	}
}

// TestDecideLogin is M1-008's state machine over the space it is total on. The
// password has already been checked by the time it is called — a wrong one is
// the table's first row and is answered before this is reached.
func TestDecideLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		required bool
		enrolled bool
		want     loginOutcome
	}{
		{required: false, enrolled: false, want: outcomeSession},
		// Stricter than the ticket's table on purpose: a confirmed factor gates
		// a sign-in whether or not anybody requires one (M1-006).
		{required: false, enrolled: true, want: outcomeChallenge},
		{required: true, enrolled: true, want: outcomeChallenge},
		// The row this ticket exists for. v1 answered it with a full session.
		{required: true, enrolled: false, want: outcomeEnrolment},
	}

	for _, test := range tests {
		if got := decideLogin(test.required, test.enrolled); got != test.want {
			t.Errorf("decideLogin(required=%t, enrolled=%t) = %d, want %d",
				test.required, test.enrolled, got, test.want)
		}
	}
}
