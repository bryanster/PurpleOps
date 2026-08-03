package identity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The replay cache (M1-010). SAML has no nonce, so this table is the only thing
// standing between a captured assertion and a working sign-in for as long as it
// remains inside its validity window.

func newAssertions(t *testing.T) *identity.SAMLAssertions {
	t.Helper()
	return identity.NewSAMLAssertions(storetest.Migrated(t))
}

func TestAnAssertionCanBeConsumedOnce(t *testing.T) {
	t.Parallel()

	r := newAssertions(t)
	expires := time.Now().Add(5 * time.Minute)

	if err := r.Consume(t.Context(), "id-assertion-1", expires); err != nil {
		t.Fatalf("the first Consume = %v, want nil", err)
	}

	err := r.Consume(t.Context(), "id-assertion-1", expires)
	if err == nil {
		t.Fatal("the same assertion was consumed twice; every captured assertion is a working " +
			"sign-in for the rest of its validity window")
	}
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("the second Consume = %v, want a conflict — the layer above turns that into a "+
			"refusal and anything else into a 500", err)
	}
}

func TestTwoDifferentAssertionsBothConsume(t *testing.T) {
	t.Parallel()

	r := newAssertions(t)
	expires := time.Now().Add(5 * time.Minute)

	for _, id := range []string{"id-a", "id-b", "id-c"} {
		if err := r.Consume(t.Context(), id, expires); err != nil {
			t.Fatalf("Consume(%q) = %v, want nil", id, err)
		}
	}
	count, err := r.Count(t.Context())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestConsumingSweepsWhatCanNoLongerBeReplayed. The table is bounded by the
// writes to it rather than by a background job, so there is nothing to schedule
// and nothing to forget to run.
func TestConsumingSweepsWhatCanNoLongerBeReplayed(t *testing.T) {
	t.Parallel()

	r := newAssertions(t)

	// One that could still be replayed, and one that could not.
	if err := r.Consume(t.Context(), "still-live", time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := r.Consume(t.Context(), "long-gone", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	// The next write sweeps.
	if err := r.Consume(t.Context(), "the-next-one", time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	count, err := r.Count(t.Context())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 — the expired row was not swept", count)
	}

	// And the swept one may be consumed again, which is the point of sweeping
	// rather than keeping forever: it protects nothing now, and a stale row that
	// refused a reused ID for ever would be a cache that grows and then lies.
	if err := r.Consume(t.Context(), "long-gone", time.Now().Add(5*time.Minute)); err != nil {
		t.Errorf("re-consuming a swept assertion ID = %v, want nil", err)
	}

	// The one still inside its window is untouched by all of it.
	if err := r.Consume(t.Context(), "still-live", time.Now().Add(5*time.Minute)); err == nil {
		t.Error("an assertion still inside its validity window was swept and accepted again")
	}
}
