package authz

import "log/slog"

// Subject is who is asking, reduced to the facts a rule can read.
//
// It is built once, by the authentication middleware, and handed to [Can]. Can
// never re-fetches any of it — that is what makes the decision a pure function
// of its arguments, and what lets M1-014 enumerate the whole model in
// milliseconds. It is also why a stale Subject is a caller's bug and not the
// policy's: the middleware loads it per request, so a demotion takes effect on
// the next one.
//
// The zero value is nobody, and holds nothing. There is no anonymous subject
// with a role that means "not much"; absence is the state, and [Can] denies it
// before reading anything else.
type Subject struct {
	UserID string

	// PlatformRole is authority over the installation. An unrecognised value
	// holds nothing, because no rule lists it — a role this build does not
	// know about is not a weaker role, it is no role.
	PlatformRole PlatformRole

	// Memberships is engagement ID → role, for every engagement this caller
	// belongs to. Absence from the map is non-membership, which is denial:
	// [PLAN.md §4] "Non-members get nothing on an engagement, including its
	// existence."
	//
	// A map rather than a slice because the only question ever asked of it is
	// "what is their role in *this* engagement", and a nil map answers it
	// correctly without a guard.
	Memberships map[string]EngagementRole

	// Method is how this request proved who it is. It selects whether the
	// token fence below applies at all.
	Method Method

	// TokenScopes is what the service token this request arrived on may do,
	// and is empty for a session. It is the *second* fence: an action needs
	// both the owner's role and the token's scope, so demoting the owner
	// constrains every token they hold without touching the tokens (M1-011).
	TokenScopes []TokenScope

	// MFASatisfied is whether this session presented a second factor.
	//
	// No rule reads it, deliberately. Whether a factor is *required* of this
	// person is a fact the Subject does not carry, and enforcing a requirement
	// against half of it is how v1 shipped an MFA setting that let anyone who
	// skipped enrolment sign in with a password alone. The requirement is
	// enforced in one place — the gate M1-008 built, ahead of this — and this
	// field is here so that the audit line for a decision records the strength
	// of the session that got it.
	MFASatisfied bool
}

// MembershipIn returns this caller's role in one engagement, and false when
// they are not a member of it.
func (s Subject) MembershipIn(engagementID string) (EngagementRole, bool) {
	if engagementID == "" {
		return "", false
	}
	role, ok := s.Memberships[engagementID]
	return role, ok
}

// authenticated reports whether anything actually established who this is.
// Both halves are required: a user ID with no method is a struct somebody
// filled in by hand, and a method with no user ID is a failed sign-in.
func (s Subject) authenticated() bool {
	return s.UserID != "" && s.Method != MethodNone
}

// holdsScope reports whether a service token carries the scope a rule requires.
func (s Subject) holdsScope(scope TokenScope) bool {
	for _, held := range s.TokenScopes {
		if held == scope {
			return true
		}
	}
	return false
}

// LogValue renders a subject for a log line.
//
// The token's scopes are logged; the token is not, and cannot be — a Subject
// has never held the secret, only what it resolved to. That is deliberate:
// M1-011 requires that a token's secret appear in exactly one response ever and
// in no log, and the cheapest way to keep a value out of the logs is for the
// logged type not to have it.
func (s Subject) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("user_id", s.UserID),
		slog.String("platform_role", string(s.PlatformRole)),
		slog.String("method", string(s.Method)),
		slog.Bool("mfa_satisfied", s.MFASatisfied),
	}
	if s.Method == MethodServiceToken {
		scopes := make([]string, 0, len(s.TokenScopes))
		for _, scope := range s.TokenScopes {
			scopes = append(scopes, string(scope))
		}
		attrs = append(attrs, slog.Any("token_scopes", scopes))
	}
	return slog.GroupValue(attrs...)
}
