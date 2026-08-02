package identity_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// The tests in this file go around the repositories and write SQL directly.
// That is the point: PLAN.md §4 says field safety comes from the schema rather
// than from if statements, and a test that could only reach the database
// through code that already validates would prove nothing about the schema.
// Each of these is a bug getting past the Go layer.

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

// TestDeletingAUserWhoOwnsAnythingIsRefused is the "does not orphan rows"
// criterion. DuckDB has no ON DELETE CASCADE, so RESTRICT is both the decision
// and the only option; either way an account is retired by status and its
// history stays attached to it.
func TestDeletingAUserWhoOwnsAnythingIsRefused(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ctx := t.Context()
	user := mustCreateUser(t, r, "alice@example.com")

	owners := map[string]func(t *testing.T){
		"identity": func(t *testing.T) {
			if _, err := r.identities.Create(ctx, identity.NewIdentity{
				UserID: user.ID, Provider: identity.ProviderLocal, Subject: "alice@example.com",
			}); err != nil {
				t.Fatal(err)
			}
		},
		"session": func(t *testing.T) {
			if _, err := r.sessions.Create(ctx, identity.NewSession{
				UserID: user.ID, TokenHash: "hash-restrict", ExpiresAt: time.Now().Add(time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
		},
		"membership": func(t *testing.T) {
			if _, err := r.memberships.Add(ctx, identity.NewMembership{
				EngagementID: "e1", UserID: user.ID, Role: identity.EngagementRoleLead,
			}); err != nil {
				t.Fatal(err)
			}
		},
	}

	// Every dependent row exists at once, then they are removed one kind at a
	// time: the delete must stay refused until the last of them is gone.
	for _, create := range owners {
		create(t)
	}
	for kind := range owners {
		if err := writeSQL(t, r.db, `DELETE FROM app."user" WHERE id = ?`, user.ID); err == nil {
			t.Fatalf("the user was deleted while a %s still referenced them", kind)
		}
		clearDependent(t, r.db, kind, user.ID)
	}

	// With nothing left pointing at them, the row can go — a mistaken account
	// that never did anything is still removable.
	if err := writeSQL(t, r.db, `DELETE FROM app."user" WHERE id = ?`, user.ID); err != nil {
		t.Errorf("deleting a user who owns nothing = %v, want nil", err)
	}
}

// TestAddedByIsHeldToARealUser: "who gave them access" is the first question an
// incident review asks, so it cannot point at nobody in particular.
func TestAddedByIsHeldToARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	err := writeSQL(t, r.db,
		`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_by, added_at)
		 VALUES ('e1', ?, 'red', 'nobody', ?)`, user.ID, time.Now().UTC())
	if err == nil {
		t.Fatal("added_by accepted an identifier belonging to no user")
	}
	if !strings.Contains(err.Error(), "foreign key") {
		t.Errorf("failed with %v, want a foreign key violation", err)
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

func clearDependent(t *testing.T, db *store.DB, kind, userID string) {
	t.Helper()

	stmts := map[string]string{
		"identity":   `DELETE FROM app.identity WHERE user_id = ?`,
		"session":    `DELETE FROM app.session WHERE user_id = ?`,
		"membership": `DELETE FROM app.engagement_member WHERE user_id = ?`,
	}
	if err := writeSQL(t, db, stmts[kind], userID); err != nil {
		t.Fatalf("clearing the %s rows: %v", kind, err)
	}
}
