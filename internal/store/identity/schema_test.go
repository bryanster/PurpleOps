package identity_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The tests in this file go around the repositories and write SQL directly.
// That is the point: PLAN.md §4 says field safety comes from the schema rather
// than from if statements, and a test that could only reach the database
// through code that already validates would prove nothing about the schema.
// Each of these is a bug getting past the Go layer.
//
// Two exceptions go through the repositories on purpose, and say so: the
// invariants that 0003_user_updatable moved out of the schema and into
// requireUser, because DuckDB's foreign keys made the rows they guard
// impossible to update.

// TestTheSchemaRefusesAnUnknownRoleOrStatus is the CHECK-constraint half. A
// role no policy knows how to judge must be impossible to store, not merely
// impossible to send.
func TestTheSchemaRefusesAnUnknownRoleOrStatus(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	cases := []struct {
		name string
		stmt string
		args []any
	}{
		{
			name: "platform role",
			stmt: `INSERT INTO app."user" (id, email, email_normalized, display_name, password_hash,
				platform_role, status, mfa_enforced, created_at, updated_at, last_login_at)
				VALUES ('bad-role', 'r@x.com', 'r@x.com', 'R', NULL, ?, 'active', false, ?, ?, NULL)`,
			args: []any{"superadmin", time.Now().UTC(), time.Now().UTC()},
		},
		{
			name: "status",
			stmt: `INSERT INTO app."user" (id, email, email_normalized, display_name, password_hash,
				platform_role, status, mfa_enforced, created_at, updated_at, last_login_at)
				VALUES ('bad-status', 's@x.com', 's@x.com', 'S', NULL, 'member', ?, false, ?, ?, NULL)`,
			args: []any{"suspended", time.Now().UTC(), time.Now().UTC()},
		},
		{
			name: "identity provider",
			stmt: `INSERT INTO app.identity (id, user_id, provider, subject, created_at)
				VALUES ('bad-provider', ?, ?, 'someone', ?)`,
			args: []any{user.ID, "ldap", time.Now().UTC()},
		},
		{
			name: "engagement role",
			stmt: `INSERT INTO app.engagement_member (engagement_id, user_id, role, added_by, added_at)
				VALUES ('e1', ?, ?, NULL, ?)`,
			args: []any{user.ID, "purple", time.Now().UTC()},
		},
		{
			name: "role changed to an unknown one after the fact",
			stmt: `UPDATE app."user" SET platform_role = ? WHERE id = ?`,
			args: []any{"root", user.ID},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := writeSQL(t, r.db, tc.stmt, tc.args...)
			if err == nil {
				t.Fatal("the write succeeded; the schema accepted a value no policy knows")
			}
			if !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Errorf("failed with %v, want a CHECK constraint failure", err)
			}
		})
	}
}

// TestTheSchemaRefusesAMismatchedNormalizedEmail is what stops uniqueness from
// being evaded by writing a normalized value that is not the lowercased
// address — the failure mode a plain second column would have.
func TestTheSchemaRefusesAMismatchedNormalizedEmail(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "alice@example.com")

	err := writeSQL(t, r.db,
		`INSERT INTO app."user" (id, email, email_normalized, display_name, password_hash,
			platform_role, status, mfa_enforced, created_at, updated_at, last_login_at)
		 VALUES ('sneaky', 'Alice@example.com', 'Alice@example.com', 'A', NULL,
			'member', 'active', false, ?, ?, NULL)`,
		time.Now().UTC(), time.Now().UTC())
	if err == nil {
		t.Fatal("a second Alice was stored by writing an unnormalized key")
	}
	if !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Errorf("failed with %v, want a CHECK constraint failure", err)
	}
}

// TestEmailUniquenessIsCaseInsensitive is the acceptance criterion in its
// literal form: alice@x.com first, then Alice@x.com, which must fail.
func TestEmailUniquenessIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "alice@x.com")

	err := writeSQL(t, r.db,
		`INSERT INTO app."user" (id, email, email_normalized, display_name, password_hash,
			platform_role, status, mfa_enforced, created_at, updated_at, last_login_at)
		 VALUES ('second', 'Alice@x.com', lower('Alice@x.com'), 'A', NULL,
			'member', 'active', false, ?, ?, NULL)`,
		time.Now().UTC(), time.Now().UTC())
	if err == nil {
		t.Fatal("Alice@x.com was stored alongside alice@x.com")
	}
	if !store.IsUniqueViolation(err) {
		t.Errorf("failed with %v, want a unique-constraint violation", err)
	}
}

// TestOneSubjectBelongsToOneAccount: without this, a second account could claim
// an existing OIDC subject and start receiving its logins.
func TestOneSubjectBelongsToOneAccount(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	mallory := mustCreateUser(t, r, "mallory@example.com")

	if _, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: alice.ID, Provider: identity.ProviderOIDC, Subject: "sub-123",
	}); err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	_, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: mallory.ID, Provider: identity.ProviderOIDC, Subject: "sub-123",
	})
	if err == nil {
		t.Fatal("a second account claimed an existing OIDC subject")
	}

	// The same subject at a different provider is a different person, and must
	// still be allowed.
	if _, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: mallory.ID, Provider: identity.ProviderSAML, Subject: "sub-123",
	}); err != nil {
		t.Errorf("Create() with the same subject at another provider = %v, want nil", err)
	}
}

// TestADependentRowMustBelongToARealUser is the invariant 0002_identity gave to
// a foreign key and 0003_user_updatable had to take back: a session, an identity
// or a membership pointing at nobody.
//
// DuckDB runs an UPDATE as a delete and an insert, so the ON DELETE RESTRICT
// made every user who owned any of these permanently uneditable — including by
// the statement that records their login. The constraints are gone and the rule
// is enforced by requireUser inside each repository's write transaction; this is
// what proves it still holds.
func TestADependentRowMustBelongToARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ctx := t.Context()

	writes := map[string]func() error{
		"identity": func() error {
			_, err := r.identities.Create(ctx, identity.NewIdentity{
				UserID: "no-such-user", Provider: identity.ProviderLocal, Subject: "ghost@example.com",
			})
			return err
		},
		"session": func() error {
			_, err := r.sessions.Create(ctx, identity.NewSession{
				UserID: "no-such-user", TokenHash: "hash-ghost", ExpiresAt: time.Now().Add(time.Hour),
			})
			return err
		},
		"membership": func() error {
			_, err := r.memberships.Add(ctx, identity.NewMembership{
				EngagementID: "e1", UserID: "no-such-user", Role: identity.EngagementRoleLead,
			})
			return err
		},
	}

	for kind, write := range writes {
		if err := write(); !errors.Is(err, apierr.ErrNotFound) {
			t.Errorf("creating a %s for a user who does not exist = %v, want a not-found", kind, err)
		}
	}
}

// TestAUserWhoOwnsRowsCanStillBeUpdated is the regression case behind
// 0003_user_updatable, and the reason its comment is as long as it is: with the
// foreign keys in place every one of these updates failed with a constraint
// error, which made signing in impossible.
func TestAUserWhoOwnsRowsCanStillBeUpdated(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ctx := t.Context()
	user := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.identities.Create(ctx, identity.NewIdentity{
		UserID: user.ID, Provider: identity.ProviderLocal, Subject: "alice@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.sessions.Create(ctx, identity.NewSession{
		UserID: user.ID, TokenHash: "hash-updatable", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.memberships.Add(ctx, identity.NewMembership{
		EngagementID: "e1", UserID: user.ID, Role: identity.EngagementRoleLead, AddedBy: user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	if err := r.users.SetLastLoginAt(ctx, user.ID, time.Now()); err != nil {
		t.Errorf("recording a login for a user who owns rows = %v, want nil", err)
	}
	user.DisplayName = "Alice Renamed"
	if updated, err := r.users.Update(ctx, user); err != nil {
		t.Errorf("updating a user who owns rows = %v, want nil", err)
	} else if updated.DisplayName != "Alice Renamed" {
		t.Errorf("DisplayName = %q, want %q", updated.DisplayName, "Alice Renamed")
	}
}

// TestAddedByIsHeldToARealUser: "who gave them access" is the first question an
// incident review asks, so it cannot point at nobody in particular.
func TestAddedByIsHeldToARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	_, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "e1", UserID: user.ID, Role: identity.EngagementRoleRed, AddedBy: "nobody",
	})
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("added_by pointing at no user = %v, want a not-found", err)
	}
}

// writeSQL runs one statement through the store's serialized writer, which is
// the only way to write — including for a test that is deliberately reaching
// past the repositories.
func writeSQL(t *testing.T, db *store.DB, stmt string, args ...any) error {
	t.Helper()

	return db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), stmt, args...)
		return err
	})
}
