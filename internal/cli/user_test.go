package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// `popsctl user create` is the bootstrap path: it is how the first
// administrator of a deployment exists at all, so its failure modes matter as
// much as its success.

const (
	testPassword = "correct horse battery staple"
	testEmail    = "alice@example.com"
)

// migratedDB returns a database with the schema applied, which is what this
// command needs and does not apply for itself.
func migratedDB(t *testing.T) string {
	t.Helper()

	db := tempDB(t)
	if got := runIn(t, db, "migrate", "up"); got.code != ExitOK {
		t.Fatalf("migrate up exited %d: %s", got.code, got.stderr)
	}
	return db
}

func TestUserCreateMakesAnAccountThatCanSignIn(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--admin", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	result := decodeJSON(t, got.stdout)
	if result["email"] != testEmail {
		t.Errorf("email = %v, want %q", result["email"], testEmail)
	}
	if result["platformRole"] != "admin" {
		t.Errorf("platformRole = %v, want %q — --admin was given", result["platformRole"], "admin")
	}
	if result["status"] != "active" {
		t.Errorf("status = %v, want %q; an account nobody can sign in to is not a bootstrap", result["status"], "active")
	}

	// The account is one that logging in would accept: the stored hash verifies
	// the password that was typed, and it is a hash rather than the password.
	user := readUser(t, db, testEmail)
	if user.PasswordHash == testPassword {
		t.Fatal("the password was stored in place of a hash")
	}
	ok, _, err := password.Verify(testPassword, user.PasswordHash)
	if err != nil || !ok {
		t.Errorf("the stored hash does not verify the password that was given (ok=%v, err=%v)", ok, err)
	}

	// And the local login method that M1-009's account linking reads.
	identities := readIdentities(t, db, user.ID)
	if len(identities) != 1 {
		t.Fatalf("%d identities, want 1", len(identities))
	}
	if identities[0].Provider != identity.ProviderLocal {
		t.Errorf("provider = %q, want %q", identities[0].Provider, identity.ProviderLocal)
	}
	if identities[0].Subject != testEmail {
		t.Errorf("subject = %q, want the normalized address %q", identities[0].Subject, testEmail)
	}
}

func TestUserCreateDefaultsToAMember(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if result := decodeJSON(t, got.stdout); result["platformRole"] != "member" {
		t.Errorf("platformRole = %v, want %q without --admin", result["platformRole"], "member")
	}
}

// TestUserCreateNormalizesTheAddress: the account is one account however it is
// typed, and the address is kept as it was typed for display.
func TestUserCreateNormalizesTheAddress(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, nil, testPassword,
		"user", "create", "--email", "  Alice@Example.COM  ", "--name", "Alice", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if result := decodeJSON(t, got.stdout); result["email"] != "Alice@Example.COM" {
		t.Errorf("email = %v, want the address as typed with the surrounding space removed", result["email"])
	}

	// The same address in another casing is the same account, and is refused.
	again := runWithInput(t, nil, testPassword,
		"user", "create", "--email", "ALICE@example.com", "--name", "Alice", "--db", db)
	if again.code != ExitFailure {
		t.Errorf("a duplicate address exited %d, want %d", again.code, ExitFailure)
	}
	if !strings.Contains(again.stderr, "already belongs to an account") {
		t.Errorf("the error does not say the address is taken: %s", again.stderr)
	}
}

func TestUserCreateRefusesADuplicate(t *testing.T) {
	db := migratedDB(t)

	first := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db)
	if first.code != ExitOK {
		t.Fatalf("the first create exited %d: %s", first.code, first.stderr)
	}

	second := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Someone Else", "--db", db)
	if second.code != ExitFailure {
		t.Errorf("exited %d, want %d", second.code, ExitFailure)
	}
	if second.stdout != "" {
		t.Errorf("wrote to stdout although nothing was created:\n%s", second.stdout)
	}
	if users := readUsers(t, db); len(users) != 1 {
		t.Errorf("%d users, want 1", len(users))
	}
}

// TestUserCreateNeverEchoesThePassword covers both streams. Neither the result
// nor the diagnostics may carry it, and neither may the log.
func TestUserCreateNeverEchoesThePassword(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, map[string]string{"PURPLEOPS_LOG_LEVEL": "debug"}, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, testPassword) {
		t.Errorf("stdout contains the password:\n%s", got.stdout)
	}
	if strings.Contains(got.stderr, testPassword) {
		t.Errorf("stderr contains the password:\n%s", got.stderr)
	}
	if strings.Contains(got.stdout, "$argon2id$") {
		t.Errorf("stdout contains the password hash, which is not something to print:\n%s", got.stdout)
	}
}

func TestUserCreateHoldsThePasswordToThePolicy(t *testing.T) {
	tests := map[string]string{
		"too short":               "short",
		"one attackers try first": "password123456",
		"nothing but spaces":      "                    ",
	}
	for name, plaintext := range tests {
		t.Run(name, func(t *testing.T) {
			db := migratedDB(t)

			got := runWithInput(t, nil, plaintext,
				"user", "create", "--email", testEmail, "--name", "Alice", "--db", db)
			if got.code != ExitFailure {
				t.Fatalf("exited %d, want %d", got.code, ExitFailure)
			}
			if !strings.HasPrefix(got.stderr, "popsctl: the password ") {
				t.Errorf("the error does not say what is wrong with the password: %s", got.stderr)
			}
			if strings.Contains(got.stderr, plaintext) {
				t.Errorf("the error repeats the password back: %s", got.stderr)
			}
			if users := readUsers(t, db); len(users) != 0 {
				t.Errorf("%d users, want none", len(users))
			}
		})
	}
}

// TestUserCreateWithNothingOnStdin: there is no terminal to ask on, so the
// command has to say what to pipe rather than hang or create an account with an
// empty password.
func TestUserCreateWithNothingOnStdin(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, nil, "", "user", "create", "--email", testEmail, "--name", "Alice", "--db", db)
	if got.code != ExitFailure {
		t.Fatalf("exited %d, want %d", got.code, ExitFailure)
	}
	if !strings.Contains(got.stderr, "no password on stdin") {
		t.Errorf("the error does not say what is missing: %s", got.stderr)
	}
}

// TestUserCreateStripsOneTrailingNewline, because `echo secret | popsctl …` adds
// one and almost nobody remembers -n.
func TestUserCreateStripsOneTrailingNewline(t *testing.T) {
	db := migratedDB(t)

	if got := runWithInput(t, nil, testPassword+"\n",
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db); got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	user := readUser(t, db, testEmail)
	ok, _, err := password.Verify(testPassword, user.PasswordHash)
	if err != nil || !ok {
		t.Errorf("the trailing newline became part of the password (ok=%v, err=%v)", ok, err)
	}
}

func TestUserCreateRefusesAPasswordWithALineBreakInIt(t *testing.T) {
	db := migratedDB(t)

	got := runWithInput(t, nil, "first line of it\nsecond line of it\n",
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db)
	if got.code != ExitFailure {
		t.Fatalf("exited %d, want %d", got.code, ExitFailure)
	}
	if !strings.Contains(got.stderr, "line break") {
		t.Errorf("the error does not explain what was wrong with the input: %s", got.stderr)
	}
}

func TestUserCreateNeedsAnEmailAndAName(t *testing.T) {
	tests := map[string][]string{
		"no email":       {"user", "create", "--name", "Alice"},
		"no name":        {"user", "create", "--email", testEmail},
		"an empty email": {"user", "create", "--email", "  ", "--name", "Alice"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			db := migratedDB(t)

			got := runWithInput(t, nil, testPassword, append(args, "--db", db)...)
			if got.code != ExitUsage {
				t.Errorf("exited %d, want %d: a missing flag is a bad command line", got.code, ExitUsage)
			}
		})
	}
}

// TestUserCreateOnAnUnmigratedDatabase has to say what to run, because the
// driver's own error is about a table nobody asked for.
func TestUserCreateOnAnUnmigratedDatabase(t *testing.T) {
	got := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", tempDB(t))
	if got.code != ExitFailure {
		t.Fatalf("exited %d, want %d", got.code, ExitFailure)
	}
	if !strings.Contains(got.stderr, "popsctl migrate up") {
		t.Errorf("the error does not say how to fix it: %s", got.stderr)
	}
}

// openDB opens the database a command wrote and closes it when the test ends.
// DuckDB gives a file to one process at a time and this is the same process, so
// it can be opened after the command has finished with it — but not before.
func openDB(t *testing.T, path string) *store.DB {
	t.Helper()

	db, err := store.Open(context.Background(), config.Database{Path: path})
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing %s: %v", path, err)
		}
	})
	return db
}

// readUsers and the two helpers below open the database the command wrote, which
// is the only way to check what it actually stored.
func readUsers(t *testing.T, path string) []identity.User {
	t.Helper()

	db := openDB(t, path)
	users, err := identity.NewUsers(db).List(t.Context())
	if err != nil {
		t.Fatalf("listing users: %v", err)
	}
	return users
}

func readUser(t *testing.T, path, email string) identity.User {
	t.Helper()

	db := openDB(t, path)
	user, err := identity.NewUsers(db).ByEmail(t.Context(), email)
	if err != nil {
		t.Fatalf("reading %s: %v", email, err)
	}
	return user
}

func readIdentities(t *testing.T, path, userID string) []identity.Identity {
	t.Helper()

	db := openDB(t, path)
	found, err := identity.NewIdentities(db).ListByUser(t.Context(), userID)
	if err != nil {
		t.Fatalf("listing identities: %v", err)
	}
	return found
}
