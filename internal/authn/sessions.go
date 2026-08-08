package authn

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Self-service session management (M1-017): the three things somebody can do
// about the browsers they are signed in on, without an administrator.
//
// Every function here takes the caller as the account being acted on, and there
// is no argument that could name another. That is the same scoping the service
// token functions use, and it is what makes "everybody holds session.read" a
// statement about their own rows rather than about everybody's — see the rule
// table in internal/authz, which says the same thing from the other side.

// Sessions returns the caller's own live sessions, newest first.
func (s *Service) Sessions(ctx context.Context, subject Subject) ([]identity.Session, error) {
	return s.sessions.Live(ctx, subject.UserID)
}

// RevokeSession ends one of the caller's own sessions. One belonging to
// somebody else is a not-found, because the lookup that finds it is scoped to
// the caller — see [session.Manager.RevokeOwned].
func (s *Service) RevokeSession(ctx context.Context, subject Subject, id string) error {
	if err := s.sessions.RevokeOwned(ctx, subject.UserID, id); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "revoked one of the caller's own sessions",
		slog.String("user_id", subject.UserID),
		slog.String("session_id", id))
	return nil
}

// RevokeOtherSessions ends every session the caller holds except the one they
// are asking on, and reports how many.
//
// The current session is what makes this different from signing out, so a
// caller that does not have one is refused rather than served: keeping "no
// session" would revoke everything, which is the opposite of what was asked and
// is not a mistake worth making quietly. The authorization policy already
// refuses a service token here (GuardSessionOnly on session.manage), so this is
// the second fence rather than the first — and the one that would still hold if
// somebody moved the endpoint.
func (s *Service) RevokeOtherSessions(ctx context.Context, subject Subject) (int64, error) {
	if subject.SessionID == "" {
		return 0, apierr.Internal(fmt.Errorf(
			"authn: %q asked to revoke their other sessions on a request that has no session to keep",
			subject.UserID))
	}

	revoked, err := s.sessions.RevokeOthers(ctx, subject.UserID, subject.SessionID)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "revoked the caller's other sessions",
		slog.String("user_id", subject.UserID),
		slog.String("kept_session_id", subject.SessionID),
		slog.Int64("sessions_revoked", revoked))
	// One row for one act, with the count in the delta — the same shape
	// RevokeAccountSessions records, so the feed reads consistently whether an
	// administrator or the account holder did it.
	s.recordAlone(ctx, events.Entry{
		ActorID:    subject.UserID,
		Verb:       events.VerbSessionOthersRevoked,
		ObjectType: events.ObjectUser,
		ObjectID:   subject.UserID,
		Delta:      events.Delta(map[string]any{"revoked": revoked}),
	})
	return revoked, nil
}
