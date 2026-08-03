package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// DB is the part of the store these repositories need: pooled reads, and writes
// serialized into one transaction at a time. [store.Store] satisfies it.
//
// It is declared here, in the package that consumes it, rather than exported by
// the store — so a test can substitute one, and so this package's dependency is
// the two methods it calls rather than everything a database happens to offer.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// Status is whether an account can be used. Retirement is a status change
// rather than a deletion: the executions, comments and findings somebody wrote
// keep their author.
type Status string

const (
	// StatusInvited is an account that exists but has never been claimed.
	StatusInvited  Status = "invited"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Provider names a way of proving who you are. One user may hold one of each.
type Provider string

const (
	ProviderLocal Provider = "local"
	ProviderOIDC  Provider = "oidc"
	ProviderSAML  Provider = "saml"
)

// User is a person. The zero value is not a user; obtain one from [Users].
type User struct {
	ID string

	// Email is the address as it was typed, for display. Lookups take any
	// casing — see [Users.ByEmail].
	Email string

	DisplayName string

	// PasswordHash is empty for an account that has no local password, which is
	// every SSO-only account. It is a hash and never a password; producing it
	// is M1-002's job.
	PasswordHash string

	PlatformRole authz.PlatformRole
	Status       Status

	// MFAEnforced is whether an administrator requires a second factor of this
	// person. Whether they have enrolled one is a different question, and
	// conflating the two is the v1 hole M1-008 closes.
	MFAEnforced bool

	CreatedAt time.Time
	UpdatedAt time.Time

	// LastLoginAt is the zero time for an account that has never logged in.
	LastLoginAt time.Time
}

// NewUser is the caller's half of creating a user: the fields that are not
// assigned by the store. The identifier and the timestamps are the store's.
type NewUser struct {
	Email        string
	DisplayName  string
	PasswordHash string
	PlatformRole authz.PlatformRole
	Status       Status
	MFAEnforced  bool
}

// Identity is one login method belonging to one user.
type Identity struct {
	ID     string
	UserID string

	Provider Provider

	// Subject is whatever the provider calls this person: the normalized email
	// for local, the "sub" claim for OIDC, the NameID for SAML. It is unique
	// per provider.
	Subject string

	CreatedAt time.Time
}

// NewIdentity is the caller's half of creating an identity.
type NewIdentity struct {
	UserID   string
	Provider Provider
	Subject  string
}

// Session is one logged-in browser. The token itself is never stored; see
// [Session.TokenHash].
type Session struct {
	ID     string
	UserID string

	// TokenHash is the hash of the value in the cookie. The hashing, and the
	// question of whether a session is still usable, belong to M1-003 — this
	// package neither computes it nor interprets the timestamps around it.
	TokenHash string

	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time

	// RevokedAt is the zero time unless the session was ended early. Expired
	// and revoked are different facts and are worth telling apart in an audit
	// trail.
	RevokedAt time.Time

	// IP and UserAgent are empty when the request did not carry them.
	IP        string
	UserAgent string

	// MFASatisfied is whether a second factor was presented for this session,
	// as opposed to at some point in this user's past.
	MFASatisfied bool
}

// NewSession is the caller's half of creating a session.
type NewSession struct {
	UserID       string
	TokenHash    string
	ExpiresAt    time.Time
	IP           string
	UserAgent    string
	MFASatisfied bool
}

// TOTP is one person's authenticator enrolment. There is at most one per user.
type TOTP struct {
	UserID string

	// SecretEncrypted is the ciphertext exactly as stored. This package never
	// looks inside it: producing and reading it belongs to
	// internal/authn/secrets, and a repository that could decrypt would be a
	// second place the key had to reach.
	SecretEncrypted string

	// ConfirmedAt is the zero time until the person has produced a code from
	// this secret. An unconfirmed enrolment gates nothing — see 0004_mfa.sql.
	ConfirmedAt time.Time

	// LastUsedStep is the last TOTP time step this enrolment accepted, and 0
	// before the first. It is the replay window; [TOTPs.Accept] is what advances
	// it, and only ever forwards.
	LastUsedStep int64

	CreatedAt time.Time
}

// Confirmed reports whether this enrolment is one that may be required at
// sign-in.
func (t TOTP) Confirmed() bool { return !t.ConfirmedAt.IsZero() }

// NewTOTP is the caller's half of enrolling: the ciphertext, and nothing else.
// A new enrolment is always unconfirmed and has spent no step.
type NewTOTP struct {
	UserID          string
	SecretEncrypted string
}

// RecoveryCode is one single-use way past a lost authenticator (M1-007). A
// person holds a set of them, minted together and replaced together.
type RecoveryCode struct {
	ID     string
	UserID string

	// CodeHash is the hash of the code that was printed. As with a session or a
	// challenge token, the value itself is never stored: it was shown once, and
	// this package could not produce it again if asked.
	CodeHash string

	// UsedAt is the zero time until the code is spent. A spent code is kept
	// rather than deleted, so that "seven of ten left" is answerable and so
	// that presenting one twice is refused by a row that exists.
	UsedAt time.Time

	CreatedAt time.Time
}

// Used reports whether this code has been spent.
func (c RecoveryCode) Used() bool { return !c.UsedAt.IsZero() }

// MFAChallenge is the pending state between a correct password and a presented
// second factor. It is not a session and nothing resolves it into a caller; see
// 0004_mfa.sql.
type MFAChallenge struct {
	ID     string
	UserID string

	// TokenHash is the hash of the value in the cookie. As with a session, the
	// token itself is never stored.
	TokenHash string

	CreatedAt time.Time
	ExpiresAt time.Time

	// ConsumedAt is the zero time until a code was accepted against this
	// challenge. One correct code buys exactly one session.
	ConsumedAt time.Time
}

// NewMFAChallenge is the caller's half of opening a challenge.
type NewMFAChallenge struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// ServiceToken is one bearer credential somebody automates with (M1-011). As
// with a session, the value that was handed out is not here: only the hash of
// its secret half, and the clear prefix that finds the row.
type ServiceToken struct {
	ID   string
	Name string

	// Prefix is the public identifier in the middle of the token, and is what
	// every authenticated request looks the row up by.
	Prefix string

	// TokenHash is the hash of the secret half. Producing and comparing it
	// belongs to internal/authn/servicetoken; this package neither computes it
	// nor decides what it means.
	TokenHash string

	// OwnerUserID is whose authority this token spends, and CreatedBy is who
	// issued it. The permissions read at request time are the owner's, live.
	OwnerUserID string
	CreatedBy   string

	// Scopes is what the token carries, exactly as stored. An entry this build
	// does not recognise is kept rather than dropped: the policy grants on
	// scopes it knows and ignores the rest, so a scope written by a newer
	// binary is harmless here and would be a silent loss if this package
	// filtered it.
	Scopes []authz.TokenScope

	// EngagementID is the one engagement this token may touch, and is empty for
	// a token that may touch every engagement its owner can.
	EngagementID string

	CreatedAt time.Time
	ExpiresAt time.Time

	// LastUsedAt is the zero time until the token is first used, and is written
	// back at most once per interval afterwards — see servicetoken.Manager, for
	// the reason a session's last_seen_at is.
	LastUsedAt time.Time

	// RevokedAt is the zero time unless somebody ended it early. Expired and
	// revoked are different facts and are worth telling apart in an audit
	// trail.
	RevokedAt time.Time
}

// NewServiceToken is the caller's half of creating one: the identifier and
// created_at are the store's, and the secret is the caller's to show once and
// forget.
type NewServiceToken struct {
	Name         string
	Prefix       string
	TokenHash    string
	OwnerUserID  string
	CreatedBy    string
	Scopes       []authz.TokenScope
	EngagementID string
	ExpiresAt    time.Time
}

// Membership places one user in one engagement with one role.
type Membership struct {
	EngagementID string
	UserID       string
	Role         authz.EngagementRole

	// AddedBy is empty when nobody added them — a seeded or imported
	// membership. Otherwise it is a user identifier, and the database holds it
	// to one.
	AddedBy string

	AddedAt time.Time
}

// NewMembership is the caller's half of adding somebody to an engagement.
type NewMembership struct {
	EngagementID string
	UserID       string
	Role         authz.EngagementRole
	AddedBy      string
}

// After runs inside a write transaction after the primary mutation succeeds.
// Activity recording (M1-015) is the first consumer: the log row and the change
// share one commit, so a failure here rolls both back.
//
// The entity the mutation just wrote is available to the hook as
// [AfterEntityID] — Create and Revoke put it on the context before calling.
type After func(ctx context.Context, tx *sql.Tx) error

type afterEntityKey struct{}

// AfterEntityID is the primary key of the row Create or Revoke just wrote,
// when called from inside an [After] hook. Outside a hook it is "".
func AfterEntityID(ctx context.Context) string {
	id, ok := ctx.Value(afterEntityKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

// WithAfterEntity puts id on ctx for [AfterEntityID]. Real repositories call
// this before running After hooks; in-memory test fakes do the same.
func WithAfterEntity(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, afterEntityKey{}, id)
}

// runAfter invokes every non-nil side effect in order.
func runAfter(ctx context.Context, tx *sql.Tx, after []After) error {
	for _, fn := range after {
		if fn == nil {
			continue
		}
		if err := fn(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// requireOneRow turns "the statement matched nothing" into [apierr.NotFound],
// which is what an update or a delete against a row somebody else has already
// removed means. Callers wrap the result; apierr classifies through wrapping,
// so the status survives.
//
// Any count other than one is a bug in the statement rather than a missing row
// — every write in this package is keyed on a primary key — so it is reported
// as itself and not flattened into not-found.
func requireOneRow(result sql.Result, resource, id string) error {
	affected, err := result.RowsAffected()
	switch {
	case err != nil:
		return fmt.Errorf("counting the affected rows: %w", err)
	case affected == 0:
		return apierr.NotFound(resource, id)
	case affected > 1:
		return fmt.Errorf("the write matched %d %s rows keyed on %q, want at most 1",
			affected, resource, id)
	}
	return nil
}

// requireUser reports [apierr.NotFound] unless the user exists, and is how a
// session, an identity or a membership is held to a real person.
//
// Until 0003_user_updatable that was a foreign key. DuckDB implements an UPDATE
// as a delete and an insert, and the delete half runs the RESTRICT check — so
// with the constraint in place no user who had ever signed in could be edited at
// all, not even to record the login. The constraint had to go; the rule it
// enforced did not, so it lives here.
//
// It runs inside the caller's write transaction, which is the only way the
// answer cannot change between the check and the insert — and, since the
// serialized writer admits one transaction at a time (PLAN.md §1), that is as
// strong as the constraint was.
func requireUser(ctx context.Context, tx *sql.Tx, userID string) error {
	var found int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM app."user" WHERE id = ?`, userID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("user", userID)
	}
	return err
}

// newID mints a UUIDv7: sortable by creation time, so "ORDER BY id" is a stable
// tiebreaker and rows arrive in the order they were made.
func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("identity: generating an identifier: %w", err)
	}
	return id.String(), nil
}

// now is the timestamp written to a row: UTC, and truncated to the microsecond
// DuckDB's TIMESTAMP stores. Without the truncation a value read back would
// differ from the one written by up to a microsecond, which is the sort of
// thing that is only ever discovered by a test that compares them.
func now() time.Time { return toStorage(time.Now()) }

// toStorage prepares a caller's timestamp the same way, so that a time supplied
// to a repository and a time generated by it behave alike.
func toStorage(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// nullString maps Go's "" to SQL NULL for the columns where the schema says
// absence is a real state — a user with no password, a membership nobody added.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fromNullTime maps SQL NULL to the zero time, for "has never logged in" and
// "was not revoked". There is no inverse: the two columns that can be null are
// written as literal NULL on insert and set to a real time afterwards, so
// nothing ever binds a possibly-zero time.
//
// Stored UTC, but a driver may hand back whatever zone it likes, and callers
// compare these.
func fromNullTime(t sql.NullTime) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
