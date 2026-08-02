package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store"
)

const identityColumns = `id, user_id, provider, subject, created_at`

const selectIdentity = `SELECT ` + identityColumns + ` FROM app.identity `

const insertIdentity = `INSERT INTO app.identity (id, user_id, provider, subject, created_at)
	VALUES (?, ?, ?, ?, ?)`

// Identities reads and writes login methods. Construct it with
// [NewIdentities].
//
// There is no update: a login method is added or removed, never edited. An
// identity whose subject changed is a different subject, and silently
// repointing one at another user is how an account gets taken over.
type Identities struct {
	db DB
}

// NewIdentities returns a repository over db.
func NewIdentities(db DB) *Identities { return &Identities{db: db} }

// Create attaches a login method to a user and returns it as stored.
//
// A (provider, subject) pair another user already holds is [apierr.Conflict]:
// it means two accounts are claiming to be the same person at the identity
// provider, which is a decision for an administrator and not something to
// resolve by overwriting.
func (r *Identities) Create(ctx context.Context, in NewIdentity) (Identity, error) {
	id, err := newID()
	if err != nil {
		return Identity{}, err
	}

	var created Identity
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, in.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, insertIdentity,
			id, in.UserID, in.Provider, in.Subject, now()); err != nil {
			return err
		}
		created, err = scanIdentity(tx.QueryRowContext(ctx, selectIdentity+`WHERE id = ?`, id))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		return Identity{}, apierr.Conflict("that login is already attached to an account")
	case err != nil:
		return Identity{}, fmt.Errorf("identity: create %s identity for user %q: %w",
			in.Provider, in.UserID, err)
	}
	return created, nil
}

// BySubject returns the identity a provider knows by this subject, or
// [apierr.NotFound]. It is the lookup every SSO login makes.
func (r *Identities) BySubject(ctx context.Context, provider Provider, subject string) (Identity, error) {
	found, err := scanIdentity(r.db.Read().QueryRowContext(ctx,
		selectIdentity+`WHERE provider = ? AND subject = ?`, provider, subject))
	if errors.Is(err, sql.ErrNoRows) {
		// The provider is part of the identifier because "no such subject" is
		// only meaningful alongside who was asked.
		return Identity{}, apierr.NotFound("identity", string(provider)+":"+subject)
	}
	if err != nil {
		return Identity{}, fmt.Errorf("identity: read %s identity %q: %w", provider, subject, err)
	}
	return found, nil
}

// ListByUser returns every login method attached to a user, oldest first.
func (r *Identities) ListByUser(ctx context.Context, userID string) ([]Identity, error) {
	rows, err := r.db.Read().QueryContext(ctx, selectIdentity+`WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("identity: list identities for user %q: %w", userID, err)
	}
	defer rows.Close()

	var found []Identity
	for rows.Next() {
		i, err := scanIdentity(rows)
		if err != nil {
			return nil, fmt.Errorf("identity: list identities for user %q: %w", userID, err)
		}
		found = append(found, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list identities for user %q: %w", userID, err)
	}
	return found, nil
}

// Delete detaches a login method, or reports [apierr.NotFound]. Whether this
// was somebody's only way in is a question for the layer that can answer it —
// see [Identities.ListByUser].
func (r *Identities) Delete(ctx context.Context, id string) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM app.identity WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "identity", id)
	})
	if err != nil {
		return fmt.Errorf("identity: delete identity %q: %w", id, err)
	}
	return nil
}

func scanIdentity(row interface{ Scan(...any) error }) (Identity, error) {
	var i Identity
	if err := row.Scan(&i.ID, &i.UserID, &i.Provider, &i.Subject, &i.CreatedAt); err != nil {
		return Identity{}, err
	}
	i.CreatedAt = i.CreatedAt.UTC()
	return i, nil
}
