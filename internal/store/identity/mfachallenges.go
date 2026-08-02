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

const mfaChallengeColumns = `id, user_id, token_hash, created_at, expires_at, consumed_at`

const selectMFAChallenge = `SELECT ` + mfaChallengeColumns + ` FROM app.mfa_challenge `

// MFAChallenges reads and writes the pending state between a correct password
// and a presented second factor. Construct it with [NewMFAChallenges].
//
// As with [Sessions], it stores and returns and does not judge: whether a
// challenge has expired is decided by the one clock in internal/authn, not by
// half a rule here and half a rule there.
type MFAChallenges struct {
	db DB
}

// NewMFAChallenges returns a repository over db.
func NewMFAChallenges(db DB) *MFAChallenges { return &MFAChallenges{db: db} }

// Open stores a challenge, first clearing whatever the same user had pending.
//
// Clearing is not housekeeping, it is the rule: starting a sign-in must
// invalidate the challenge left behind by the previous attempt, or an abandoned
// one stays answerable for its whole window while its owner is signing in
// somewhere else. It also bounds the table, which nothing else would — these
// rows have no other reason to be deleted.
func (r *MFAChallenges) Open(ctx context.Context, in NewMFAChallenge) (MFAChallenge, error) {
	id, err := newID()
	if err != nil {
		return MFAChallenge{}, err
	}
	at := now()

	var created MFAChallenge
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, in.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM app.mfa_challenge WHERE user_id = ?`, in.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.mfa_challenge
				(id, user_id, token_hash, created_at, expires_at, consumed_at)
				VALUES (?, ?, ?, ?, ?, NULL)`,
			id, in.UserID, in.TokenHash, at, toStorage(in.ExpiresAt)); err != nil {
			return err
		}
		created, err = scanMFAChallenge(
			tx.QueryRowContext(ctx, selectMFAChallenge+`WHERE id = ?`, id))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		return MFAChallenge{}, apierr.Conflict("that challenge token is already in use")
	case err != nil:
		return MFAChallenge{}, fmt.Errorf("identity: open an MFA challenge for user %q: %w",
			in.UserID, err)
	}
	return created, nil
}

// ByTokenHash returns the challenge with this token hash, or [apierr.NotFound].
func (r *MFAChallenges) ByTokenHash(ctx context.Context, hash string) (MFAChallenge, error) {
	c, err := scanMFAChallenge(r.db.Read().QueryRowContext(ctx,
		selectMFAChallenge+`WHERE token_hash = ?`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		// The hash is a credential in the log as well as in the response, so it
		// is not the identifier this reports.
		return MFAChallenge{}, apierr.NotFound("MFA challenge", "(token hash)")
	}
	if err != nil {
		return MFAChallenge{}, fmt.Errorf("identity: read an MFA challenge by token: %w", err)
	}
	return c, nil
}

// Consume marks a challenge spent and reports whether this call was the one
// that spent it.
//
// The guard is in the statement, so a challenge cannot be consumed twice even by
// two requests arriving together: the second finds consumed_at already set and
// is told false. A caller that gets false has lost that race and must not issue
// a session.
func (r *MFAChallenges) Consume(ctx context.Context, id string, at time.Time) (bool, error) {
	consumed := false
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.mfa_challenge SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`,
			toStorage(at), id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("counting the affected rows: %w", err)
		}
		if affected == 1 {
			consumed = true
			return nil
		}
		return confirmMFAChallengeExists(ctx, tx, id)
	})
	if err != nil {
		return false, fmt.Errorf("identity: consume the MFA challenge %q: %w", id, err)
	}
	return consumed, nil
}

// DeleteForUser removes every challenge a user has. It is what an administrator
// disabling an account, and a password change, reach for: a pending challenge
// outliving the credentials that opened it would be a way back in.
func (r *MFAChallenges) DeleteForUser(ctx context.Context, userID string) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM app.mfa_challenge WHERE user_id = ?`, userID)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity: delete the MFA challenges of user %q: %w", userID, err)
	}
	return nil
}

func confirmMFAChallengeExists(ctx context.Context, tx *sql.Tx, id string) error {
	var found int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM app.mfa_challenge WHERE id = ?`, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("MFA challenge", id)
	}
	return err
}

func scanMFAChallenge(row interface{ Scan(...any) error }) (MFAChallenge, error) {
	var (
		c        MFAChallenge
		consumed sql.NullTime
	)
	if err := row.Scan(&c.ID, &c.UserID, &c.TokenHash, &c.CreatedAt,
		&c.ExpiresAt, &consumed); err != nil {
		return MFAChallenge{}, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	c.ExpiresAt = c.ExpiresAt.UTC()
	c.ConsumedAt = fromNullTime(consumed)
	return c, nil
}
