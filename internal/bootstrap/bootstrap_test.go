package bootstrap_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/bootstrap"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The bootstrap is the one thing in this system that creates an account with
// nobody signed in, so the tests are mostly about what it refuses to do: touch a
// database that already has accounts, and start a deployment nobody could sign
// in to.

const testPassword = "correct horse battery staple"

func TestItCreatesTheFirstAdministrator(t *testing.T) {
	db := storetest.Migrated(t)

	if err := bootstrap.Apply(t.Context(), db, config.Bootstrap{
		Email:    "  Admin@Example.com  ",
		Name:     "Ada Lovelace",
		Password: foreignSecret(t, testPassword),
	}, quiet()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	user := onlyUser(t, db)
	if got, want := user.Email, "Admin@Example.com"; got != want {
		t.Errorf("Email = %q, want %q — the address is stored trimmed, as typed", got, want)
	}
	if got, want := user.DisplayName, "Ada Lovelace"; got != want {
		t.Errorf("DisplayName = %q, want %q", got, want)
	}
	if got, want := user.PlatformRole, authz.PlatformRoleAdmin; got != want {
		t.Errorf("PlatformRole = %q, want %q; a member could not administer the deployment", got, want)
	}
	if got, want := user.Status, identity.StatusActive; got != want {
		t.Errorf("Status = %q, want %q; an account nobody can sign in to is not a bootstrap", got, want)
	}

	ok, _, err := password.Verify(testPassword, user.PasswordHash)
	if err != nil {
		t.Fatalf("verifying the stored hash: %v", err)
	}
	if !ok {
		t.Error("the configured password does not verify against the stored hash")
	}

	// The local login method is what account linking reads; an account without
	// one is the gap identity.CreateWithLocalLogin exists to avoid.
	if _, err := identity.NewIdentities(db).BySubject(
		t.Context(), identity.ProviderLocal, "admin@example.com"); err != nil {
		t.Errorf("no local login method for the account: %v", err)
	}
}

func TestItReadsThePasswordFromAFile(t *testing.T) {
	db := storetest.Migrated(t)
	// With the trailing newline almost everything that writes a file adds, and
	// which is not part of the password.
	path := writeFile(t, testPassword+"\n")

	if err := bootstrap.Apply(t.Context(), db, config.Bootstrap{
		Email:        "admin@example.com",
		Name:         "Administrator",
		PasswordFile: path,
	}, quiet()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	ok, _, err := password.Verify(testPassword, onlyUser(t, db).PasswordHash)
	if err != nil {
		t.Fatalf("verifying the stored hash: %v", err)
	}
	if !ok {
		t.Error("the password in the file does not verify; the trailing newline is not part of it")
	}
}

func TestItDoesNothingWithoutAnAddress(t *testing.T) {
	db := storetest.Migrated(t)

	if err := bootstrap.Apply(t.Context(), db, config.Bootstrap{Name: "Administrator"}, quiet()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := countUsers(t, db); got != 0 {
		t.Errorf("%d accounts exist, want none: an unconfigured bootstrap must create nobody", got)
	}
}

// The precondition the whole design rests on: a configured bootstrap is inert
// on a database that has accounts, whatever it says. If this ever passes an
// existing account through, an environment variable becomes a way to reset an
// administrator's password.
func TestItLeavesADatabaseWithAccountsAlone(t *testing.T) {
	db := storetest.Migrated(t)

	existing, err := identity.CreateWithLocalLogin(t.Context(), db, identity.NewUser{
		Email:        "someone@example.com",
		DisplayName:  "Someone",
		PasswordHash: hash(t, "a password nobody is changing"),
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	for _, cfg := range []config.Bootstrap{
		{Email: "admin@example.com", Name: "Administrator", Password: foreignSecret(t, testPassword)},
		// The same address as the account that is already there: still nothing,
		// because the rule is "no accounts", not "not this account".
		{Email: "someone@example.com", Name: "Administrator", Password: foreignSecret(t, testPassword)},
	} {
		if err := bootstrap.Apply(t.Context(), db, cfg, quiet()); err != nil {
			t.Fatalf("Apply(%s): %v", cfg.Email, err)
		}
	}

	if got := countUsers(t, db); got != 1 {
		t.Fatalf("%d accounts exist, want the 1 that was there before", got)
	}
	after := onlyUser(t, db)
	if after.PasswordHash != existing.PasswordHash {
		t.Error("the existing account's password was changed")
	}
	if after.PlatformRole != authz.PlatformRoleMember {
		t.Errorf("PlatformRole = %q, want %q; the bootstrap promoted somebody",
			after.PlatformRole, authz.PlatformRoleMember)
	}
}

// A password the policy refuses fails the start, rather than leaving a
// deployment that comes up healthy and cannot be signed in to.
func TestItRefusesAPasswordThePolicyRefuses(t *testing.T) {
	db := storetest.Migrated(t)

	err := bootstrap.Apply(t.Context(), db, config.Bootstrap{
		Email:    "admin@example.com",
		Name:     "Administrator",
		Password: foreignSecret(t, "short"),
	}, quiet())
	if err == nil {
		t.Fatal("Apply succeeded with a password the policy refuses")
	}
	if !strings.Contains(err.Error(), "at least 12 characters") {
		t.Errorf("error does not say what is wrong with the password: %v", err)
	}
	if !strings.Contains(err.Error(), config.EnvBootstrapPassword) {
		t.Errorf("error does not name the variable to change: %v", err)
	}
	if got := countUsers(t, db); got != 0 {
		t.Errorf("%d accounts exist, want none: a refused password must create nothing", got)
	}
}

func TestItReportsAnUnreadablePasswordFile(t *testing.T) {
	db := storetest.Migrated(t)

	err := bootstrap.Apply(t.Context(), db, config.Bootstrap{
		Email:        "admin@example.com",
		Name:         "Administrator",
		PasswordFile: filepath.Join(t.TempDir(), "not-mounted"),
	}, quiet())
	if err == nil {
		t.Fatal("Apply succeeded with a password file that does not exist")
	}
	if !strings.Contains(err.Error(), config.EnvBootstrapPasswordFile) {
		t.Errorf("error does not name the variable that points at the file: %v", err)
	}
}

// A file holding more than the password is a mistake worth naming: the account
// would end up with a password whose owner does not know what it is.
func TestItRefusesAPasswordFileWithMoreThanAPasswordInIt(t *testing.T) {
	db := storetest.Migrated(t)

	err := bootstrap.Apply(t.Context(), db, config.Bootstrap{
		Email:        "admin@example.com",
		Name:         "Administrator",
		PasswordFile: writeFile(t, testPassword+"\nand another line\n"),
	}, quiet())
	if err == nil {
		t.Fatal("Apply succeeded with a file holding two lines")
	}
	if !strings.Contains(err.Error(), "line break") {
		t.Errorf("error does not say what is wrong with the file: %v", err)
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func foreignSecret(t *testing.T, value string) config.ForeignSecret {
	t.Helper()

	var secret config.ForeignSecret
	if err := secret.UnmarshalText([]byte(value)); err != nil {
		t.Fatalf("building a test password: %v", err)
	}
	return secret
}

func hash(t *testing.T, plaintext password.Plaintext) string {
	t.Helper()

	encoded, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("hashing a test password: %v", err)
	}
	return encoded
}

func writeFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bootstrap-admin-password")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func countUsers(t *testing.T, db *store.DB) int {
	t.Helper()

	count, err := identity.NewUsers(db).Count(t.Context())
	if err != nil {
		t.Fatalf("counting accounts: %v", err)
	}
	return count
}

func onlyUser(t *testing.T, db *store.DB) identity.User {
	t.Helper()

	users, _, err := identity.NewUsers(db).Page(t.Context(), identity.PageFilter{Limit: 10})
	if err != nil {
		t.Fatalf("listing accounts: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("%d accounts exist, want exactly 1", len(users))
	}
	return users[0]
}
