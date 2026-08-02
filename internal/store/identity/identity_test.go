package identity_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// These tests run against a real migrated DuckDB file (see storetest), because
// most of what this package promises is a promise the schema makes: that an
// unknown role cannot be stored, that two spellings of one email are one
// account, that a user with sessions cannot be deleted out from under them. A
// mock would agree with whatever the Go code happened to do.

// repos is every repository over one database, which is how the application
// wires them and how a test that spans two of them needs them.
type repos struct {
	db          *store.DB
	users       *identity.Users
	identities  *identity.Identities
	sessions    *identity.Sessions
	memberships *identity.Memberships
	totp        *identity.TOTPs
	challenges  *identity.MFAChallenges
	recovery    *identity.RecoveryCodes
}

func newRepos(t *testing.T) repos {
	t.Helper()

	db := storetest.Migrated(t)
	return repos{
		db:          db,
		users:       identity.NewUsers(db),
		identities:  identity.NewIdentities(db),
		sessions:    identity.NewSessions(db),
		memberships: identity.NewMemberships(db),
		totp:        identity.NewTOTPs(db),
		challenges:  identity.NewMFAChallenges(db),
		recovery:    identity.NewRecoveryCodes(db),
	}
}

// member is a plain active user, for the tests that need somebody to exist
// rather than somebody in particular.
func member(email string) identity.NewUser {
	return identity.NewUser{
		Email:        email,
		DisplayName:  email,
		PasswordHash: "argon2id$placeholder",
		PlatformRole: identity.PlatformRoleMember,
		Status:       identity.StatusActive,
	}
}

func mustCreateUser(t *testing.T, r repos, email string) identity.User {
	t.Helper()

	u, err := r.users.Create(t.Context(), member(email))
	if err != nil {
		t.Fatalf("creating user %q: %v", email, err)
	}
	return u
}

// TestNoRepositoryOwnsADatabaseHandle is the structural half of the
// single-writer rule (PLAN.md §6, M0B-003). A repository holding a *sql.DB
// could begin its own transaction and write outside store.Write, and the
// failure that causes — a lost write when two people save at once — does not
// show up in a test. So the shape is asserted instead of the symptom.
func TestNoRepositoryOwnsADatabaseHandle(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	forbidden := []reflect.Type{
		reflect.TypeFor[*sql.DB](),
		reflect.TypeFor[*sql.Tx](),
		reflect.TypeFor[*sql.Conn](),
	}

	for _, repo := range []any{r.users, r.identities, r.sessions, r.memberships} {
		repoType := reflect.TypeOf(repo).Elem()
		for i := range repoType.NumField() {
			field := repoType.Field(i)
			for _, banned := range forbidden {
				if field.Type == banned {
					t.Errorf("%s.%s is a %s; repositories take a store, not a database handle",
						repoType.Name(), field.Name, banned)
				}
			}
		}
	}
}

// TestEveryRepositoryMethodTakesAContext is the other convention that is easy
// to break one method at a time and impossible to notice afterwards: without a
// context, a query outlives the request that wanted it.
func TestEveryRepositoryMethodTakesAContext(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ctxType := reflect.TypeFor[context.Context]()

	for _, repo := range []any{r.users, r.identities, r.sessions, r.memberships} {
		repoType := reflect.TypeOf(repo)
		for i := range repoType.NumMethod() {
			method := repoType.Method(i)
			// In[0] is the receiver, so the first declared argument is In[1].
			if method.Type.NumIn() < 2 || !method.Type.In(1).Implements(ctxType) {
				t.Errorf("%s.%s does not take a context.Context first",
					repoType.Elem().Name(), method.Name)
			}
		}
	}
}

// TestTimestampsAreUTC covers every timestamp the package writes. The store
// sets the session time zone to UTC, but a driver is free to hand back a time
// in any zone, and a report that formats one is entitled to assume it did not.
func TestTimestampsAreUTC(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ctx := t.Context()

	// Deliberately not UTC: what a caller passes in must not decide what comes
	// back out.
	elsewhere := time.FixedZone("UTC+7", 7*60*60)
	loginAt := time.Now().In(elsewhere)

	user := mustCreateUser(t, r, "utc@example.com")
	if err := r.users.SetLastLoginAt(ctx, user.ID, loginAt); err != nil {
		t.Fatalf("SetLastLoginAt() = %v", err)
	}
	user, err := r.users.ByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ByID() = %v", err)
	}

	session, err := r.sessions.Create(ctx, identity.NewSession{
		UserID: user.ID, TokenHash: "hash-utc", ExpiresAt: loginAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("sessions.Create() = %v", err)
	}
	ident, err := r.identities.Create(ctx, identity.NewIdentity{
		UserID: user.ID, Provider: identity.ProviderLocal, Subject: "utc@example.com",
	})
	if err != nil {
		t.Fatalf("identities.Create() = %v", err)
	}
	membership, err := r.memberships.Add(ctx, identity.NewMembership{
		EngagementID: "e-utc", UserID: user.ID, Role: identity.EngagementRoleRed,
	})
	if err != nil {
		t.Fatalf("memberships.Add() = %v", err)
	}

	stamps := map[string]time.Time{
		"user.CreatedAt":     user.CreatedAt,
		"user.UpdatedAt":     user.UpdatedAt,
		"user.LastLoginAt":   user.LastLoginAt,
		"identity.CreatedAt": ident.CreatedAt,
		"session.CreatedAt":  session.CreatedAt,
		"session.LastSeenAt": session.LastSeenAt,
		"session.ExpiresAt":  session.ExpiresAt,
		"membership.AddedAt": membership.AddedAt,
	}
	for name, stamp := range stamps {
		if stamp.IsZero() {
			t.Errorf("%s is the zero time; this test is not checking what it claims", name)
			continue
		}
		if stamp.Location() != time.UTC {
			t.Errorf("%s is in %s, want UTC", name, stamp.Location())
		}
	}

	// The caller's zone changes nothing about the instant that was stored.
	if !user.LastLoginAt.Equal(loginAt.UTC().Truncate(time.Microsecond)) {
		t.Errorf("LastLoginAt = %s, want %s", user.LastLoginAt, loginAt.UTC())
	}
}

// TestTheIdentitySchemaExists is the migration-from-empty check for 0002: the
// tables and the indexes M1 queries through are all there after migrating an
// empty database, which is the state storetest.Migrated leaves behind.
func TestTheIdentitySchemaExists(t *testing.T) {
	t.Parallel()

	db := storetest.Migrated(t)
	ctx := t.Context()

	for _, table := range []string{"user", "identity", "session", "engagement_member"} {
		var n int
		if err := db.Read().QueryRowContext(ctx,
			`SELECT count(*) FROM information_schema.tables
			 WHERE table_schema = 'app' AND table_name = ?`, table).Scan(&n); err != nil {
			t.Fatalf("looking for app.%s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("app.%q does not exist after migrating", table)
		}
	}

	// Every access path M1 uses is indexed. v1 had none at all, which PLAN.md
	// calls out as a stack problem, so an index quietly dropped from the
	// migration should fail a test rather than show up as a slow deployment.
	//
	// duckdb_indexes() is engine-specific, and this is the one place that is
	// fine: the backlog's rule is about the schema and the queries staying
	// portable, and a test asserting what DuckDB did with the schema is not
	// something a different engine would run unchanged anyway.
	for _, index := range []string{
		"identity_user_id_idx",
		"session_user_id_expires_at_idx",
		"engagement_member_user_id_idx",
	} {
		var n int
		if err := db.Read().QueryRowContext(ctx,
			`SELECT count(*) FROM duckdb_indexes() WHERE index_name = ?`, index).Scan(&n); err != nil {
			t.Fatalf("looking for index %s: %v", index, err)
		}
		if n != 1 {
			t.Errorf("index %s does not exist after migrating", index)
		}
	}
}
