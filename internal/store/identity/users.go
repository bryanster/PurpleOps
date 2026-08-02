package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// userColumns is the read projection, in the order [scanUser] expects. One
// constant so that adding a column cannot change one and not the other.
//
// email_normalized is not among them: it is the lookup key, derived from email,
// and a copy of it on the struct would be one more field a caller could set to
// something untrue.
const userColumns = `id, email, display_name, password_hash,
	platform_role, status, mfa_enforced, created_at, updated_at, last_login_at`

const selectUser = `SELECT ` + userColumns + ` FROM app."user" `

// insertUser normalizes in SQL, and trims the display column too, so that the
// stored pair always satisfies the table's CHECK. email is bound twice because
// DuckDB's placeholders are positional.
const insertUser = `INSERT INTO app."user"
	(id, email, email_normalized, display_name, password_hash,
	 platform_role, status, mfa_enforced, created_at, updated_at, last_login_at)
	VALUES (?, trim(?), lower(trim(?)), ?, ?, ?, ?, ?, ?, ?, NULL)`

// updateUser writes the fields a caller may change. Neither created_at nor
// last_login_at is among them: one is fixed, and the other belongs to logging
// in ([Users.SetLastLoginAt]), so a read-modify-write of a stale User cannot
// undo a login that happened while it was being edited.
const updateUser = `UPDATE app."user"
	SET email = trim(?), email_normalized = lower(trim(?)), display_name = ?,
	    password_hash = ?, platform_role = ?, status = ?, mfa_enforced = ?,
	    updated_at = ?
	WHERE id = ?`

// Users reads and writes people. Construct it with [NewUsers].
//
// There is deliberately no delete: an account is retired with
// [StatusDisabled], and the schema restricts hard deletion of a user who owns
// anything (0002_identity.sql). Removing that history is a database
// administrator's decision, not an API call.
type Users struct {
	db DB
}

// NewUsers returns a repository over db.
func NewUsers(db DB) *Users { return &Users{db: db} }

// Create stores a new user and returns it as stored, with the identifier and
// timestamps the store assigned.
//
// An email that is already in use — in any casing — is [apierr.Conflict]
// rather than a server error: it is the caller's to fix, and it is the most
// common way this call fails.
func (r *Users) Create(ctx context.Context, u NewUser) (User, error) {
	id, err := newID()
	if err != nil {
		return User{}, err
	}
	at := now()

	var created User
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, insertUser,
			id, u.Email, u.Email, u.DisplayName, nullString(u.PasswordHash),
			u.PlatformRole, u.Status, u.MFAEnforced, at, at); err != nil {
			return err
		}
		// Read back inside the same transaction rather than returning the
		// struct that went in: the database trims and normalizes, so this is
		// the only way the caller is handed what was actually stored.
		created, err = scanUser(tx.QueryRowContext(ctx, selectUser+`WHERE id = ?`, id))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		return User{}, apierr.Conflict("that email address is already in use")
	case err != nil:
		return User{}, fmt.Errorf("identity: create user %q: %w", u.Email, err)
	}
	return created, nil
}

// ByID returns the user with this identifier, or [apierr.NotFound].
func (r *Users) ByID(ctx context.Context, id string) (User, error) {
	u, err := scanUser(r.db.Read().QueryRowContext(ctx, selectUser+`WHERE id = ?`, id))
	if err != nil {
		return User{}, wrapUserErr(err, id)
	}
	return u, nil
}

// ByEmail returns the user with this address, matched without regard to case or
// surrounding whitespace, or [apierr.NotFound].
func (r *Users) ByEmail(ctx context.Context, email string) (User, error) {
	u, err := scanUser(r.db.Read().QueryRowContext(ctx,
		selectUser+`WHERE email_normalized = lower(trim(?))`, email))
	if err != nil {
		return User{}, wrapUserErr(err, email)
	}
	return u, nil
}

// List returns every user, ordered by normalized email — unique, so the order
// is total, and the order a person reading a list of accounts expects.
func (r *Users) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.Read().QueryContext(ctx, selectUser+`ORDER BY email_normalized`)
	if err != nil {
		return nil, fmt.Errorf("identity: list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("identity: list users: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list users: %w", err)
	}
	return users, nil
}

// Update writes u's email, display name, password hash, platform role, status
// and MFA requirement over the stored row, and returns it as stored. Everything
// else on u is ignored — see [updateUser].
//
// A user that no longer exists is [apierr.NotFound]; an email that another user
// already holds is [apierr.Conflict].
func (r *Users) Update(ctx context.Context, u User) (User, error) {
	var updated User
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, updateUser,
			u.Email, u.Email, u.DisplayName, nullString(u.PasswordHash),
			u.PlatformRole, u.Status, u.MFAEnforced, now(), u.ID)
		if err != nil {
			return err
		}
		if err := requireOneRow(result, "user", u.ID); err != nil {
			return err
		}
		updated, err = scanUser(tx.QueryRowContext(ctx, selectUser+`WHERE id = ?`, u.ID))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		return User{}, apierr.Conflict("that email address is already in use")
	case err != nil:
		// A not-found raised inside the transaction survives this: apierr
		// classifies through wrapping, on purpose, so adding context to an
		// error does not turn a 404 into a 500.
		return User{}, fmt.Errorf("identity: update user %q: %w", u.ID, err)
	}
	return updated, nil
}

// SetLastLoginAt records a successful login. It takes the time rather than
// reading the clock so that the moment stored is the moment the session was
// established, not the moment this row happened to be written.
func (r *Users) SetLastLoginAt(ctx context.Context, id string, at time.Time) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app."user" SET last_login_at = ? WHERE id = ?`, toStorage(at), id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "user", id)
	})
	if err != nil {
		return fmt.Errorf("identity: record login for user %q: %w", id, err)
	}
	return nil
}

// scanUser reads one row of [userColumns]. It takes the interface both *sql.Row
// and *sql.Rows satisfy, so the single-row and list paths cannot drift apart.
func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var (
		u         User
		hash      sql.NullString
		lastLogin sql.NullTime
	)
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &hash,
		&u.PlatformRole, &u.Status, &u.MFAEnforced,
		&u.CreatedAt, &u.UpdatedAt, &lastLogin); err != nil {
		return User{}, err
	}

	u.PasswordHash = hash.String
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	u.LastLoginAt = fromNullTime(lastLogin)
	return u, nil
}

// wrapUserErr turns the absence of a row into the API's not-found, and anything
// else into a wrapped failure naming what was being looked up.
func wrapUserErr(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("user", id)
	}
	return fmt.Errorf("identity: read user %q: %w", id, err)
}
