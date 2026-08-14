package identity

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
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
//
// after runs inside the same transaction after the insert, so a side effect
// that fails — the activity row (M1-015), today — rolls the account back with
// it.
func (r *Users) Create(ctx context.Context, u NewUser, after ...After) (User, error) {
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
		if err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, created.ID), tx, after)
	})
	switch {
	case store.IsUniqueViolation(err):
		return User{}, apierr.Conflict("that email address is already in use")
	case err != nil:
		return User{}, fmt.Errorf("identity: create user %q: %w", u.Email, err)
	}
	return created, nil
}

// CreateWithLocalLogin writes an account and the local login method that points
// at it — the pair that makes somebody able to sign in with a password, and
// what both paths that create an account outside the API build (`blctl user
// create`, and the first-administrator bootstrap in internal/bootstrap).
//
// The two are separate writes because they are separate repositories, and there
// is a window between them. It is reported rather than hidden: an account
// without its identity row can still sign in — local login resolves by email —
// but account linking (M1-009) reads that table, so a deployment should not be
// left with a gap in it quietly.
//
// A conflicting address comes back as [apierr.ErrConflict], for the caller to
// word for whoever is reading.
func CreateWithLocalLogin(ctx context.Context, db DB, in NewUser) (User, error) {
	created, err := NewUsers(db).Create(ctx, in)
	if err != nil {
		return User{}, err
	}

	// The subject of a local identity is the normalized address, which is what
	// the database stores as email_normalized and what every lookup uses.
	_, err = NewIdentities(db).Create(ctx, NewIdentity{
		UserID:   created.ID,
		Provider: ProviderLocal,
		Subject:  strings.ToLower(strings.TrimSpace(created.Email)),
	})
	if err != nil {
		return User{}, fmt.Errorf(
			"the account %s was created (id %s) but its local login method was not: %w",
			created.Email, created.ID, err)
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

// PageFilter narrows and pages through accounts (M1-016).
//
// The zero value selects everything, one default page at a time. Each field
// that is set is one more AND: there is no "match any of these" form, because
// the interface this backs has one filter bar and no way to express one.
type PageFilter struct {
	// Status and Role restrict the page to accounts in that state or holding
	// that role. The zero value of either means "any".
	Status Status
	Role   authz.PlatformRole

	// Search matches the display name or the email address, without regard to
	// case, anywhere in either — an administrator looking somebody up usually
	// remembers a fragment rather than the beginning.
	Search string

	// Cursor is the opaque value from a previous page, and Limit is how many
	// rows to return. Limit is clamped to [defaultPageSize, maxPageSize] rather
	// than refused: the request validator already holds the wire parameter to
	// the same bounds, and a caller reaching this with something outside them is
	// not one an error message would help.
	Cursor string
	Limit  int
}

// The page bounds. They match components/parameters/Limit in api/openapi.yaml,
// which is what a request is actually held to; these are the backstop for the
// callers that do not come through it — blctl, and the tests.
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// Page returns one page of accounts, oldest first, and the cursor for the next
// one — empty when there is no next one.
//
// Oldest first because identifiers are UUIDv7: `ORDER BY id` is creation order
// and a total order at the same time, so paging cannot skip or repeat a row
// when accounts are created while somebody is reading. An alphabetical listing
// would need a second column in the cursor and would still reshuffle under a
// rename.
func (r *Users) Page(ctx context.Context, f PageFilter) (users []User, nextCursor string, err error) {
	limit := f.Limit
	switch {
	case limit <= 0:
		limit = defaultPageSize
	case limit > maxPageSize:
		limit = maxPageSize
	}

	var (
		conditions []string
		args       []any
	)
	if f.Status != "" {
		conditions = append(conditions, `status = ?`)
		args = append(args, f.Status)
	}
	if f.Role != "" {
		conditions = append(conditions, `platform_role = ?`)
		args = append(args, f.Role)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		// Lowered by the database on both sides, for the reason every email
		// comparison in this package is: one definition of "the same letter",
		// and it is not Go's. email_normalized is already lower(trim(...)), so
		// only the pattern needs it there.
		pattern := "%" + escapeLike(search) + "%"
		conditions = append(conditions,
			`(lower(display_name) LIKE lower(?) ESCAPE '\' OR email_normalized LIKE lower(?) ESCAPE '\')`)
		args = append(args, pattern, pattern)
	}
	if f.Cursor != "" {
		after, cerr := decodeUserCursor(f.Cursor)
		if cerr != nil {
			return nil, "", apierr.Validation(apierr.Field("cursor", "is not a cursor this server issued"))
		}
		conditions = append(conditions, `id > ?`)
		args = append(args, after)
	}

	var b strings.Builder
	b.WriteString(selectUser)
	if len(conditions) > 0 {
		b.WriteString(`WHERE ` + strings.Join(conditions, ` AND `) + ` `)
	}
	// One more than asked for: the extra row is how "there is another page" is
	// known without a second count query, and it is dropped before returning.
	b.WriteString(`ORDER BY id LIMIT ?`)
	args = append(args, limit+1)

	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("identity: list users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, "", fmt.Errorf("identity: list users: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("identity: list users: %w", err)
	}

	if len(users) > limit {
		users = users[:limit]
		nextCursor = encodeUserCursor(users[limit-1].ID)
	}
	if users == nil {
		users = []User{}
	}
	return users, nextCursor, nil
}

// Count reports how many accounts exist, in every status and every role.
//
// It answers one question, asked once at startup by internal/bootstrap: is this
// a database nobody has an account on? Zero is the only interesting answer, and
// it is read through the pool rather than in a transaction because the process
// asking holds the database file — DuckDB gives it to one writer — so there is
// nothing for the count to race with.
func (r *Users) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT count(*) FROM app."user"`).Scan(&count); err != nil {
		return 0, fmt.Errorf("identity: count the accounts: %w", err)
	}
	return count, nil
}

// CountActiveAdmins reports how many accounts are both [authz.PlatformRoleAdmin]
// and [StatusActive], on the caller's write transaction.
//
// It takes the transaction rather than reading through the pool because of what
// it is for: an installation must never end up with no administrator, and the
// only way to be sure of that is to make the change, count inside the same
// transaction, and roll back if the answer is zero. A count read beforehand
// through the pooled reader would be a check somebody could race — see the
// guard in internal/authn.
func CountActiveAdmins(ctx context.Context, tx *sql.Tx) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM app."user" WHERE platform_role = ? AND status = ?`,
		authz.PlatformRoleAdmin, StatusActive).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("identity: count the active administrators: %w", err)
	}
	return count, nil
}

// Update writes u's email, display name, password hash, platform role, status
// and MFA requirement over the stored row, and returns it as stored. Everything
// else on u is ignored — see [updateUser].
//
// A user that no longer exists is [apierr.NotFound]; an email that another user
// already holds is [apierr.Conflict].
//
// after runs inside the same transaction after the update. Two callers use it:
// the activity log (M1-015), and the last-administrator guard in internal/authn
// — which is why a hook that returns an error must undo the write, and does,
// because the transaction rolls back with it.
func (r *Users) Update(ctx context.Context, u User, after ...After) (User, error) {
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
		if err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, updated.ID), tx, after)
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

// likeEscape is the escape character every LIKE in this package declares. A
// backslash is not special inside a standard SQL string literal, so `ESCAPE '\'`
// is one backslash and not an unterminated escape.
const likeEscape = `\`

// escapeLike neutralises the two LIKE metacharacters, and the escape character
// itself, in text a caller typed. Without it a search for "50%" matches every
// account, and one for "_" matches every account with at least one character —
// which is a search box that quietly ignores what was typed rather than one that
// finds nothing.
var escapeLike = strings.NewReplacer(
	likeEscape, likeEscape+likeEscape,
	"%", likeEscape+"%",
	"_", likeEscape+"_",
).Replace

// The page cursor is the identifier of the last row handed out, base64url'd.
//
// Encoded rather than sent bare so that it is opaque in fact and not only by
// convention: a client that cannot read one cannot come to depend on its
// contents, and this one is free to become a compound key later.
func encodeUserCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeUserCursor(cursor string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", errors.New("empty")
	}
	return string(raw), nil
}

// wrapUserErr turns the absence of a row into the API's not-found, and anything
// else into a wrapped failure naming what was being looked up.
func wrapUserErr(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("user", id)
	}
	return fmt.Errorf("identity: read user %q: %w", id, err)
}
