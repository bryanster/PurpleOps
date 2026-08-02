package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store"
)

const sessionColumns = `id, user_id, token_hash, created_at, last_seen_at,
	expires_at, revoked_at, ip, user_agent, mfa_satisfied`

const selectSession = `SELECT ` + sessionColumns + ` FROM app.session `

const insertSession = `INSERT INTO app.session
	(id, user_id, token_hash, created_at, last_seen_at, expires_at,
	 revoked_at, ip, user_agent, mfa_satisfied)
	VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`

// Sessions reads and writes logged-in browsers. Construct it with
// [NewSessions].
//
// It stores and returns; it does not decide. Whether a session is still usable
// depends on expiry, revocation, the user's status and whether MFA is required
// of them — that judgement is one place, in M1-003, and duplicating a piece of
// it here is how the two come to disagree.
type Sessions struct {
	db DB
}

// NewSessions returns a repository over db.
func NewSessions(db DB) *Sessions { return &Sessions{db: db} }

// Create stores a new session and returns it as stored. created_at and
// last_seen_at are set to now; a session has been seen at the moment it is
// made.
func (r *Sessions) Create(ctx context.Context, in NewSession) (Session, error) {
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	at := now()

	var created Session
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, insertSession,
			id, in.UserID, in.TokenHash, at, at, toStorage(in.ExpiresAt),
			in.IP, in.UserAgent, in.MFASatisfied); err != nil {
			return err
		}
		created, err = scanSession(tx.QueryRowContext(ctx, selectSession+`WHERE id = ?`, id))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		// Two sessions cannot share a token hash. With a random token this
		// never happens, so it means the caller reused one — worth failing on
		// rather than quietly handing out a second cookie for one session.
		return Session{}, apierr.Conflict("that session token is already in use")
	case err != nil:
		return Session{}, fmt.Errorf("identity: create session for user %q: %w", in.UserID, err)
	}
	return created, nil
}

// ByTokenHash returns the session with this token hash, or [apierr.NotFound].
// It is the lookup on every authenticated request.
func (r *Sessions) ByTokenHash(ctx context.Context, hash string) (Session, error) {
	s, err := scanSession(r.db.Read().QueryRowContext(ctx,
		selectSession+`WHERE token_hash = ?`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		// The hash is not echoed anywhere: apierr keeps the identifier out of
		// the response, and this one is a credential in the log as well.
		return Session{}, apierr.NotFound("session", "(token hash)")
	}
	if err != nil {
		return Session{}, fmt.Errorf("identity: read session by token: %w", err)
	}
	return s, nil
}

// ListByUser returns every session row a user has, newest first, including
// expired and revoked ones — this is what backs "where am I signed in", which
// is only useful if it shows what ended and when.
func (r *Sessions) ListByUser(ctx context.Context, userID string) ([]Session, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectSession+`WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("identity: list sessions for user %q: %w", userID, err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("identity: list sessions for user %q: %w", userID, err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list sessions for user %q: %w", userID, err)
	}
	return sessions, nil
}

// SetLastSeenAt records that a session was used.
func (r *Sessions) SetLastSeenAt(ctx context.Context, id string, at time.Time) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.session SET last_seen_at = ? WHERE id = ?`, toStorage(at), id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "session", id)
	})
	if err != nil {
		return fmt.Errorf("identity: touch session %q: %w", id, err)
	}
	return nil
}

// Revoke ends one session. Revoking an already-revoked session keeps the
// original timestamp: the first revocation is the one that took effect, and
// overwriting it would lose when access actually stopped.
func (r *Sessions) Revoke(ctx context.Context, id string, at time.Time) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.session SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
			toStorage(at), id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("counting the affected rows: %w", err)
		}
		if affected == 1 {
			return nil
		}
		// Nothing was updated: either the session is gone, or it was already
		// revoked — which is not a failure, because the caller's intent is
		// satisfied.
		return confirmSessionExists(ctx, tx, id)
	})
	if err != nil {
		return fmt.Errorf("identity: revoke session %q: %w", id, err)
	}
	return nil
}

// RevokeAllForUser ends every live session a user has and reports how many it
// ended. It is what a password change, a role change and an administrator
// disabling an account all call — PLAN.md §4's "rotation on privilege change".
func (r *Sessions) RevokeAllForUser(ctx context.Context, userID string, at time.Time) (int64, error) {
	var revoked int64
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.session SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
			toStorage(at), userID)
		if err != nil {
			return err
		}
		revoked, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("identity: revoke sessions for user %q: %w", userID, err)
	}
	return revoked, nil
}

// DeleteExpired removes sessions that expired before the given time, and
// reports how many. Revoked sessions are removed on the same terms rather than
// immediately, so that "this session was revoked" stays answerable for as long
// as an expired one does.
//
// A caller passes a cutoff behind now — a retention window — rather than now
// itself: the rows are the record of who was signed in, and deleting them the
// instant they expire throws that away.
func (r *Sessions) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	var deleted int64
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM app.session WHERE expires_at < ?`, toStorage(before))
		if err != nil {
			return err
		}
		deleted, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("identity: delete sessions expired before %s: %w",
			before.UTC().Format(time.RFC3339), err)
	}
	return deleted, nil
}

// confirmSessionExists reports [apierr.NotFound] for a session that is not
// there, and nil for one that is. It runs inside the caller's transaction, so
// the answer cannot change between the write and the check.
func confirmSessionExists(ctx context.Context, tx *sql.Tx, id string) error {
	var found int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM app.session WHERE id = ?`, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("session", id)
	}
	return err
}

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var (
		s       Session
		revoked sql.NullTime
	)
	if err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CreatedAt, &s.LastSeenAt,
		&s.ExpiresAt, &revoked, &s.IP, &s.UserAgent, &s.MFASatisfied); err != nil {
		return Session{}, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.LastSeenAt = s.LastSeenAt.UTC()
	s.ExpiresAt = s.ExpiresAt.UTC()
	s.RevokedAt = fromNullTime(revoked)
	return s, nil
}
