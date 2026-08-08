package identity_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The recovery code store (M1-007). What is tested here is what the statements
// promise and the Go layer above cannot: that a set replaces rather than
// accumulates, that a code cannot be spent twice even by two callers at once,
// and that the codes belonging to one person are invisible to another.

// hashes returns n placeholder hashes, distinct and prefixed so a failure names
// which one. This package neither computes nor interprets them, so opaque
// strings are exactly as good here as real HMACs.
func hashes(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s-hash-%02d", prefix, i)
	}
	return out
}

func TestReplaceStoresASetAndReturnsIt(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	stored, err := r.recovery.Replace(t.Context(), user.ID, hashes("first", 10))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if len(stored) != 10 {
		t.Fatalf("%d codes stored, want 10", len(stored))
	}

	for _, code := range stored {
		switch {
		case code.ID == "":
			t.Error("a stored code has no identifier")
		case code.UserID != user.ID:
			t.Errorf("a stored code belongs to %q, want %q", code.UserID, user.ID)
		case code.Used():
			t.Error("a freshly stored code is already used")
		case code.CreatedAt.IsZero():
			t.Error("a stored code has no created_at")
		}
	}
}

// TestReplaceInvalidatesEveryPreviousCode is the acceptance criterion about
// regenerating: the unused ones have to go too, because the whole reason
// somebody regenerates is that the old list is somewhere it should not be.
func TestReplaceInvalidatesEveryPreviousCode(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	first, err := r.recovery.Replace(t.Context(), user.ID, hashes("first", 10))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	// Spend one, so the set being replaced is a realistic mixture rather than
	// ten untouched rows.
	if _, err := r.recovery.Use(t.Context(), first[0].ID, time.Now()); err != nil {
		t.Fatalf("Use: %v", err)
	}

	if _, err := r.recovery.Replace(t.Context(), user.ID, hashes("second", 10)); err != nil {
		t.Fatalf("second Replace: %v", err)
	}

	unused, err := r.recovery.Unused(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("Unused: %v", err)
	}
	if len(unused) != 10 {
		t.Fatalf("%d unused codes after regenerating, want 10", len(unused))
	}
	for _, code := range unused {
		if code.CodeHash[:5] == "first" {
			t.Fatalf("the code %s survived a regeneration", code.ID)
		}
	}

	// And the rows themselves are gone, not merely unused: spending one of the
	// old identifiers must find nothing.
	for _, old := range first {
		spent, err := r.recovery.Use(t.Context(), old.ID, time.Now())
		if err != nil {
			t.Fatalf("Use on a replaced code: %v", err)
		}
		if spent {
			t.Fatalf("the replaced code %s could still be spent", old.ID)
		}
	}
}

// TestACodeIsSpentExactlyOnce is the reuse criterion, and the count that goes
// with it.
func TestACodeIsSpentExactlyOnce(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	stored, err := r.recovery.Replace(t.Context(), user.ID, hashes("set", 10))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	at := time.Now()
	spent, err := r.recovery.Use(t.Context(), stored[3].ID, at)
	if err != nil {
		t.Fatalf("Use: %v", err)
	}
	if !spent {
		t.Fatal("the first use of a fresh code reported false")
	}

	again, err := r.recovery.Use(t.Context(), stored[3].ID, at)
	if err != nil {
		t.Fatalf("second Use: %v", err)
	}
	if again {
		t.Error("the same code was spent twice")
	}

	remaining, err := r.recovery.CountUnused(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("CountUnused: %v", err)
	}
	if remaining != 9 {
		t.Errorf("%d codes remaining after spending one, want 9", remaining)
	}

	// The spent row is kept and marked, not deleted: "you have used one of your
	// ten" is only answerable if it is still there.
	unused, err := r.recovery.Unused(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("Unused: %v", err)
	}
	for _, code := range unused {
		if code.ID == stored[3].ID {
			t.Error("a spent code is still reported as unused")
		}
	}
}

// TestTwoCallersCannotBothSpendOneCode is the race the guard in the UPDATE
// exists for. The serialized writer (PLAN.md §1) runs them one at a time, so
// the second finds used_at already set — which is the behaviour, not an
// accident of scheduling.
func TestTwoCallersCannotBothSpendOneCode(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	stored, err := r.recovery.Replace(t.Context(), user.ID, hashes("set", 10))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	const racers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
	)
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			spent, err := r.recovery.Use(t.Context(), stored[0].ID, time.Now())
			if err != nil {
				t.Errorf("Use: %v", err)
				return
			}
			if spent {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Errorf("%d of %d callers spent the same code, want exactly 1", winners, racers)
	}
}

// TestCodesAreScopedToTheirOwner. Nothing above this package filters by user,
// so if these queries did not, one person's code would open another's account.
func TestCodesAreScopedToTheirOwner(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	if _, err := r.recovery.Replace(t.Context(), alice.ID, hashes("alice", 10)); err != nil {
		t.Fatalf("Replace for alice: %v", err)
	}
	if _, err := r.recovery.Replace(t.Context(), bob.ID, hashes("bob", 3)); err != nil {
		t.Fatalf("Replace for bob: %v", err)
	}

	// Bob's set did not disturb Alice's, and neither can see the other's.
	for _, tc := range []struct {
		user identity.User
		want int
	}{{alice, 10}, {bob, 3}} {
		unused, err := r.recovery.Unused(t.Context(), tc.user.ID)
		if err != nil {
			t.Fatalf("Unused for %s: %v", tc.user.Email, err)
		}
		if len(unused) != tc.want {
			t.Errorf("%s holds %d codes, want %d", tc.user.Email, len(unused), tc.want)
		}
		for _, code := range unused {
			if code.UserID != tc.user.ID {
				t.Errorf("%s's list contains a code belonging to %s", tc.user.Email, code.UserID)
			}
		}
	}

	// And deleting one person's codes leaves the other's alone.
	if err := r.recovery.DeleteForUser(t.Context(), bob.ID); err != nil {
		t.Fatalf("DeleteForUser: %v", err)
	}
	if got, err := r.recovery.CountUnused(t.Context(), alice.ID); err != nil || got != 10 {
		t.Errorf("alice holds %d codes after bob's were deleted (err=%v), want 10", got, err)
	}
	if got, err := r.recovery.CountUnused(t.Context(), bob.ID); err != nil || got != 0 {
		t.Errorf("bob holds %d codes after his were deleted (err=%v), want 0", got, err)
	}
}

// TestReplaceRefusesAUserThatIsNotThere is requireUser doing the job the
// foreign key cannot (0003_user_updatable). Without it a set of codes could
// outlive — or precede — the account it belongs to.
func TestReplaceRefusesAUserThatIsNotThere(t *testing.T) {
	t.Parallel()

	r := newRepos(t)

	_, err := r.recovery.Replace(t.Context(), "no-such-user", hashes("set", 10))
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Replace for an unknown user = %v, want not-found", err)
	}
}

// TestAnEmptySetIsALegalReplacement: it is what removing an authenticator does,
// and it must not be an error or a no-op.
func TestAnEmptySetIsALegalReplacement(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.recovery.Replace(t.Context(), user.ID, hashes("set", 10)); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	stored, err := r.recovery.Replace(t.Context(), user.ID, nil)
	if err != nil {
		t.Fatalf("Replace with no hashes: %v", err)
	}
	if len(stored) != 0 {
		t.Errorf("%d codes after replacing with an empty set, want 0", len(stored))
	}
	if got, err := r.recovery.CountUnused(t.Context(), user.ID); err != nil || got != 0 {
		t.Errorf("CountUnused = %d (err=%v), want 0", got, err)
	}
}

// TestTwoCodesCannotShareAHash. At a hundred bits this does not happen, which
// is exactly why it is worth failing loudly on: if it ever does, the generator
// has stopped being random and quietly resolving it would hide that.
func TestTwoCodesCannotShareAHash(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	if _, err := r.recovery.Replace(t.Context(), alice.ID, []string{"the-same-hash"}); err != nil {
		t.Fatalf("Replace for alice: %v", err)
	}
	_, err := r.recovery.Replace(t.Context(), bob.ID, []string{"the-same-hash"})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("storing a duplicate hash = %v, want a conflict", err)
	}
}
