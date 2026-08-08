package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// `blctl user create` is the bootstrap path: it is how the first
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

	got := runWithInput(t, map[string]string{"BLACKLIGHT_LOG_LEVEL": "debug"}, testPassword,
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
			if !strings.HasPrefix(got.stderr, "blctl: the password ") {
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

// TestUserCreateStripsOneTrailingNewline, because `echo secret | blctl …` adds
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
	if !strings.Contains(got.stderr, "blctl migrate up") {
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
	users, _, err := identity.NewUsers(db).Page(t.Context(), identity.PageFilter{Limit: 200})
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

// `blctl user reset-mfa` is the break-glass path of M1-007. It is the only way
// back into an account whose authenticator and codes are both gone, and it is a
// lock being deliberately broken — so what it removes, what it leaves alone and
// what it says about it are all worth pinning down.

// enrolMFA gives a user an authenticator and a set of recovery codes, writing
// the rows directly. The command under test does not create them and there is no
// CLI path that does; what matters here is that it removes them.
func enrolMFA(t *testing.T, path, userID string, codes int) {
	t.Helper()

	db := openDB(t, path)
	if _, err := identity.NewTOTPs(db).Enroll(t.Context(), identity.NewTOTP{
		UserID:          userID,
		SecretEncrypted: "sealed-placeholder",
	}); err != nil {
		t.Fatalf("enrolling an authenticator: %v", err)
	}
	if _, err := identity.NewTOTPs(db).Accept(t.Context(), userID, 1, time.Now()); err != nil {
		t.Fatalf("confirming the enrolment: %v", err)
	}

	hashes := make([]string, codes)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("hash-%s-%02d", userID, i)
	}
	if _, err := identity.NewRecoveryCodes(db).Replace(t.Context(), userID, hashes); err != nil {
		t.Fatalf("storing recovery codes: %v", err)
	}
}

func mfaState(t *testing.T, path, userID string) (enrolled bool, codes int) {
	t.Helper()

	db := openDB(t, path)
	_, err := identity.NewTOTPs(db).ByUserID(t.Context(), userID)
	switch {
	case err == nil:
		enrolled = true
	case !errors.Is(err, apierr.ErrNotFound):
		t.Fatalf("reading the enrolment: %v", err)
	}

	codes, err = identity.NewRecoveryCodes(db).CountUnused(t.Context(), userID)
	if err != nil {
		t.Fatalf("counting recovery codes: %v", err)
	}
	return enrolled, codes
}

func TestUserResetMFARemovesBothHalvesOfTheSecondFactor(t *testing.T) {
	db := migratedDB(t)
	seedUser(t, db)
	user := readUser(t, db, testEmail)
	enrolMFA(t, db, user.ID, 7)

	got := runIn(t, db, "user", "reset-mfa", "--email", testEmail, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	result := decodeJSON(t, got.stdout)
	if result["authenticatorRemoved"] != true {
		t.Errorf("authenticatorRemoved = %v, want true", result["authenticatorRemoved"])
	}
	if result["recoveryCodesRemoved"] != float64(7) {
		t.Errorf("recoveryCodesRemoved = %v, want 7", result["recoveryCodesRemoved"])
	}
	if result["email"] != testEmail {
		t.Errorf("email = %v, want %q", result["email"], testEmail)
	}

	enrolled, codes := mfaState(t, db, user.ID)
	if enrolled {
		t.Error("the authenticator enrolment survived the reset")
	}
	if codes != 0 {
		t.Errorf("%d recovery codes survived the reset, want 0", codes)
	}
}

// TestUserResetMFALeavesTheAccountItself. It removes a second factor and
// nothing else: the account, its password and its role are not this command's
// business, and an operator reaching for it in a hurry must not find they have
// also reset something they did not mean to.
func TestUserResetMFALeavesTheAccountItself(t *testing.T) {
	db := migratedDB(t)
	seedUser(t, db)
	before := readUser(t, db, testEmail)
	enrolMFA(t, db, before.ID, 10)

	if got := runIn(t, db, "user", "reset-mfa", "--email", testEmail); got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	after := readUser(t, db, testEmail)
	switch {
	case after.PasswordHash != before.PasswordHash:
		t.Error("the password hash changed")
	case after.PlatformRole != before.PlatformRole:
		t.Error("the platform role changed")
	case after.Status != before.Status:
		t.Error("the account status changed")
	case after.MFAEnforced != before.MFAEnforced:
		t.Error("mfa_enforced changed; whether an administrator requires a factor " +
			"is a different question from whether one is enrolled")
	}
}

// TestUserResetMFAWarnsAboutWhatItDid is the acceptance criterion. The output
// is the only thing standing between an operator and forgetting that they have
// just made a password sufficient.
func TestUserResetMFAWarnsAboutWhatItDid(t *testing.T) {
	db := migratedDB(t)
	seedUser(t, db)
	enrolMFA(t, db, readUser(t, db, testEmail).ID, 4)

	got := runIn(t, db, "user", "reset-mfa", "--email", testEmail)
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	for _, want := range []string{"WARNING", "password and nothing else", "enrol an", testEmail} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the output does not mention %q:\n%s", want, got.stdout)
		}
	}
	// And the log line an audit trail is built from (M1-015 gives it a durable
	// home; until then this is the record).
	if !strings.Contains(got.stderr, "second factor reset") {
		t.Errorf("nothing was logged about the reset:\n%s", got.stderr)
	}
}

// TestUserResetMFASaysWhenAFactorIsStillRequired is the other half of the same
// output (M1-008). On an account a policy still covers, this command has not
// made a password sufficient — it has turned a lockout into an enrolment — and
// printing the warning above would tell an operator their deployment is less
// protected than it is.
func TestUserResetMFASaysWhenAFactorIsStillRequired(t *testing.T) {
	db := migratedDB(t)
	seedUser(t, db)

	user := readUser(t, db, testEmail)
	enrolMFA(t, db, user.ID, 4)
	requireMFAOf(t, db, user)

	got := runIn(t, db, "user", "reset-mfa", "--email", testEmail)
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}

	if !strings.Contains(got.stdout, "still required") {
		t.Errorf("the output does not say a factor is still required:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "password and nothing else") {
		t.Errorf("the output claims a password is now sufficient, which it is not:\n%s", got.stdout)
	}
}

// requireMFAOf sets the per-user flag directly, which is what the user
// administration API will do (M1-016). The platform policy would do just as
// well; the flag is the half that needs no second row.
func requireMFAOf(t *testing.T, db string, user identity.User) {
	t.Helper()

	store := openDB(t, db)
	user.MFAEnforced = true
	if _, err := identity.NewUsers(store).Update(t.Context(), user); err != nil {
		t.Fatalf("requiring MFA of %s: %v", user.Email, err)
	}
}

// TestUserResetMFAOnAnAccountWithoutOneIsNotAnError: the operator wanted the
// account to have no second factor, and it does.
func TestUserResetMFAOnAnAccountWithoutOneIsNotAnError(t *testing.T) {
	db := migratedDB(t)
	seedUser(t, db)

	got := runIn(t, db, "user", "reset-mfa", "--email", testEmail)
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "had no second factor") {
		t.Errorf("the output does not say nothing was removed:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "WARNING") {
		t.Error("a reset that removed nothing warned as though it had")
	}
}

func TestUserResetMFANeedsAnAccountThatExists(t *testing.T) {
	db := migratedDB(t)

	got := runIn(t, db, "user", "reset-mfa", "--email", "nobody@example.com")
	if got.code != ExitFailure {
		t.Fatalf("exited %d, want %d\nstderr: %s", got.code, ExitFailure, got.stderr)
	}
	if !strings.Contains(got.stderr, "nobody@example.com") {
		t.Errorf("the error does not name the address:\n%s", got.stderr)
	}
}

func TestUserResetMFANeedsAnEmail(t *testing.T) {
	db := migratedDB(t)

	got := runIn(t, db, "user", "reset-mfa")
	if got.code != ExitUsage {
		t.Fatalf("exited %d, want %d\nstderr: %s", got.code, ExitUsage, got.stderr)
	}
}

// seedUser creates the account the reset-mfa tests act on, through the command
// that exists for it.
func seedUser(t *testing.T, db string) {
	t.Helper()

	got := runWithInput(t, nil, testPassword,
		"user", "create", "--email", testEmail, "--name", "Alice", "--db", db)
	if got.code != ExitOK {
		t.Fatalf("seeding the user exited %d: %s", got.code, got.stderr)
	}
}
