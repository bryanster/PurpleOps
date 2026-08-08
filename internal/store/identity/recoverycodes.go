package identity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

const recoveryCodeColumns = `id, user_id, code_hash, used_at, created_at`

const selectRecoveryCode = `SELECT ` + recoveryCodeColumns + ` FROM app.user_recovery_code `

// RecoveryCodes reads and writes the single-use codes that stand in for an
// authenticator (M1-007). Construct it with [NewRecoveryCodes].
//
// It stores hashes and spends rows. It does not know how a code is spelled, how
// one is hashed, or how many a person should have — those live in
// internal/authn/recovery, so that there is one answer to each rather than one
// here and one there.
type RecoveryCodes struct {
	db DB
}

// NewRecoveryCodes returns a repository over db.
func NewRecoveryCodes(db DB) *RecoveryCodes { return &RecoveryCodes{db: db} }

// Replace makes hashes the user's complete set of codes, discarding whatever
// they had.
//
// Replacing rather than adding is the rule M1-007 asks for: regenerating must
// invalidate every outstanding code, including the unused ones, or somebody who
// regenerated *because* a printout went missing would still be reachable
// through it. It is one transaction, so there is no moment in which a person
// holds both sets or neither.
//
// An empty slice is a legal call and means "this person has no codes", which is
// what removing an authenticator does.
func (r *RecoveryCodes) Replace(ctx context.Context, userID string, hashes []string) ([]RecoveryCode, error) {
	at := now()

	ids := make([]string, len(hashes))
	for i := range hashes {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}

	var created []RecoveryCode
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM app.user_recovery_code WHERE user_id = ?`, userID); err != nil {
			return err
		}
		for i, hash := range hashes {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.user_recovery_code (id, user_id, code_hash, used_at, created_at)
					VALUES (?, ?, ?, NULL, ?)`,
				ids[i], userID, hash, at); err != nil {
				return err
			}
		}
		rows, err := tx.QueryContext(ctx,
			selectRecoveryCode+`WHERE user_id = ? ORDER BY id`, userID)
		if err != nil {
			return err
		}
		created, err = scanRecoveryCodes(rows)
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		// Two codes hashing alike. At ~100 bits from crypto/rand this does not
		// happen, which is exactly why it is worth refusing loudly rather than
		// resolving quietly — it means the generator has stopped being random.
		return nil, apierr.Conflict("that recovery code is already in use")
	case err != nil:
		return nil, fmt.Errorf("identity: replace the recovery codes of user %q: %w", userID, err)
	}
	return created, nil
}

// Unused returns the codes a person can still present, oldest first.
//
// Only the unused ones: a spent code is not a candidate for anything, and
// leaving its hash out of the slice the caller compares against is one fewer
// place for it to be treated as live by mistake.
func (r *RecoveryCodes) Unused(ctx context.Context, userID string) ([]RecoveryCode, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectRecoveryCode+`WHERE user_id = ? AND used_at IS NULL ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("identity: read the recovery codes of user %q: %w", userID, err)
	}
	codes, err := scanRecoveryCodes(rows)
	if err != nil {
		return nil, fmt.Errorf("identity: read the recovery codes of user %q: %w", userID, err)
	}
	return codes, nil
}

// CountUnused reports how many codes a person has left. It is what the profile
// answers with, and it is a count rather than len(Unused) so that the ordinary
// case — every request to GET /auth/me — does not carry ten hashes across the
// process to discard them.
func (r *RecoveryCodes) CountUnused(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.Read().QueryRowContext(ctx,
		`SELECT count(*) FROM app.user_recovery_code WHERE user_id = ? AND used_at IS NULL`,
		userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("identity: count the recovery codes of user %q: %w", userID, err)
	}
	return count, nil
}

// Use spends a code and reports whether this call was the one that spent it.
//
// The guard is in the statement, as it is for a challenge and for a TOTP step:
// two requests arriving with the same code cannot both find it unused, so one
// code buys exactly one session. A caller told false has lost that race, or is
// presenting a code that was spent earlier, and must refuse either way.
func (r *RecoveryCodes) Use(ctx context.Context, id string, at time.Time) (bool, error) {
	used := false
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.user_recovery_code SET used_at = ? WHERE id = ? AND used_at IS NULL`,
			toStorage(at), id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("counting the affected rows: %w", err)
		}
		if affected == 1 {
			used = true
		}
		// A row that matched nothing is a code already spent, or one deleted by
		// a regeneration between the read and this write. Neither is an error:
		// the caller's answer to false is the same refusal in both cases.
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("identity: spend the recovery code %q: %w", id, err)
	}
	return used, nil
}

// DeleteForUser removes every code a person holds, spent or not. Removing an
// authenticator calls it, and so does `blctl user reset-mfa`: codes are the
// second half of a second factor, and leaving them behind would mean a factor
// that was removed is still presentable.
func (r *RecoveryCodes) DeleteForUser(ctx context.Context, userID string) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM app.user_recovery_code WHERE user_id = ?`, userID)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity: delete the recovery codes of user %q: %w", userID, err)
	}
	return nil
}

func scanRecoveryCodes(rows *sql.Rows) ([]RecoveryCode, error) {
	defer rows.Close()

	codes := make([]RecoveryCode, 0, defaultCodeSetSize)
	for rows.Next() {
		var (
			c    RecoveryCode
			used sql.NullTime
		)
		if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &used, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.UsedAt = fromNullTime(used)
		c.CreatedAt = c.CreatedAt.UTC()
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

// defaultCodeSetSize is how many codes a set holds, used here only to size a
// slice. The number itself is internal/authn/recovery's to decide; this is an
// allocation hint that costs nothing if it is wrong.
const defaultCodeSetSize = 10
