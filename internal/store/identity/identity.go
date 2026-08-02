package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store"
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

// PlatformRole is what somebody may do to this installation: manage users,
// content and every engagement, or take part in the ones they are a member of.
// It is deliberately two values — anything finer belongs to the engagement
// role, and v1's single fuzzy level is the mistake PLAN.md §4 is correcting.
type PlatformRole string

const (
	PlatformRoleAdmin  PlatformRole = "admin"
	PlatformRoleMember PlatformRole = "member"
)

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

// EngagementRole is what somebody may do inside one engagement. Red and blue
// are separate so that blind mode and the split write endpoints in PLAN.md §4
// have something to decide on.
type EngagementRole string

const (
	EngagementRoleLead     EngagementRole = "lead"
	EngagementRoleRed      EngagementRole = "red"
	EngagementRoleBlue     EngagementRole = "blue"
	EngagementRoleObserver EngagementRole = "observer"
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

	PlatformRole PlatformRole
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
	PlatformRole PlatformRole
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

// Membership places one user in one engagement with one role.
type Membership struct {
	EngagementID string
	UserID       string
	Role         EngagementRole

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
	Role         EngagementRole
	AddedBy      string
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
