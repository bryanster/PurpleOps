package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

const totpColumns = `user_id, secret_encrypted, confirmed_at, last_used_step, created_at`

const selectTOTP = `SELECT ` + totpColumns + ` FROM app.user_totp `

// TOTPs reads and writes authenticator enrolments. Construct it with
// [NewTOTPs].
//
// It stores ciphertext and a step counter. It does not decrypt, it does not
// know what a valid code looks like, and it does not decide whether a factor is
// required — those live in internal/authn/totp and internal/authn respectively,
// so that there is one answer to each rather than one here and one there.
type TOTPs struct {
	db DB
}

// NewTOTPs returns a repository over db.
func NewTOTPs(db DB) *TOTPs { return &TOTPs{db: db} }

// ByUserID returns a person's enrolment, or [apierr.NotFound] when they have
// none.
func (r *TOTPs) ByUserID(ctx context.Context, userID string) (TOTP, error) {
	t, err := scanTOTP(r.db.Read().QueryRowContext(ctx, selectTOTP+`WHERE user_id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return TOTP{}, apierr.NotFound("authenticator enrolment", userID)
	}
	if err != nil {
		return TOTP{}, fmt.Errorf("identity: read the authenticator of user %q: %w", userID, err)
	}
	return t, nil
}

// Enroll stores a new, unconfirmed enrolment, replacing whatever the user had.
//
// Replacing rather than refusing is what makes a re-scan work: somebody who
// abandoned an enrolment, or who is setting up a new phone, starts again from a
// fresh secret and a fresh QR code. Refusing to replace a *confirmed* enrolment
// is a policy decision and lives in internal/authn, which is where the current
// password that would have to accompany it is checked.
func (r *TOTPs) Enroll(ctx context.Context, in NewTOTP) (TOTP, error) {
	at := now()

	var created TOTP
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, in.UserID); err != nil {
			return err
		}
		// Delete then insert rather than an upsert: this has to clear
		// confirmed_at and last_used_step as well, and a new secret that
		// inherited the old one's spent step would refuse its own first code.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM app.user_totp WHERE user_id = ?`, in.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.user_totp
				(user_id, secret_encrypted, confirmed_at, last_used_step, created_at)
				VALUES (?, ?, NULL, 0, ?)`,
			in.UserID, in.SecretEncrypted, at); err != nil {
			return err
		}
		var err error
		created, err = scanTOTP(tx.QueryRowContext(ctx, selectTOTP+`WHERE user_id = ?`, in.UserID))
		return err
	})
	if err != nil {
		return TOTP{}, fmt.Errorf("identity: enrol an authenticator for user %q: %w", in.UserID, err)
	}
	return created, nil
}

// Accept records that a code was accepted: it advances the replay window to
// step, and — on the first acceptance — marks the enrolment confirmed.
//
// The two happen in one statement because they are one event. `last_used_step <
// ?` is the whole of the replay protection, and it is enforced by the database
// rather than by the caller reading and then writing: two verifications racing
// on the same code cannot both find a lower step and both win, because the
// serialized writer (PLAN.md §1) runs them one after the other and the second
// one's guard is false.
//
// It reports false when the step has already been spent, which is a replay and
// is the caller's to refuse. Nothing is written in that case.
func (r *TOTPs) Accept(ctx context.Context, userID string, step int64, at time.Time) (bool, error) {
	accepted := false
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.user_totp
			 SET last_used_step = ?, confirmed_at = coalesce(confirmed_at, ?)
			 WHERE user_id = ? AND last_used_step < ?`,
			step, toStorage(at), userID, step)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("counting the affected rows: %w", err)
		}
		if affected == 1 {
			accepted = true
			return nil
		}
		// Nothing was updated: either there is no enrolment, or the step has
		// been spent. The first is a caller bug and the second is a replay, and
		// only the first is an error.
		return confirmTOTPExists(ctx, tx, userID)
	})
	if err != nil {
		return false, fmt.Errorf("identity: accept a code for user %q: %w", userID, err)
	}
	return accepted, nil
}

// Delete removes a person's enrolment. Removing one that is not there is not an
// error: the caller wanted them to have no authenticator, and they do not.
func (r *TOTPs) Delete(ctx context.Context, userID string) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM app.user_totp WHERE user_id = ?`, userID)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity: delete the authenticator of user %q: %w", userID, err)
	}
	return nil
}

// confirmTOTPExists reports [apierr.NotFound] for a user with no enrolment, and
// nil for one that has it. It runs inside the caller's transaction, so the
// answer cannot change between the write and the check.
func confirmTOTPExists(ctx context.Context, tx *sql.Tx, userID string) error {
	var found int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM app.user_totp WHERE user_id = ?`, userID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("authenticator enrolment", userID)
	}
	return err
}

func scanTOTP(row interface{ Scan(...any) error }) (TOTP, error) {
	var (
		t         TOTP
		confirmed sql.NullTime
	)
	if err := row.Scan(&t.UserID, &t.SecretEncrypted, &confirmed,
		&t.LastUsedStep, &t.CreatedAt); err != nil {
		return TOTP{}, err
	}
	t.ConfirmedAt = fromNullTime(confirmed)
	t.CreatedAt = t.CreatedAt.UTC()
	return t, nil
}
