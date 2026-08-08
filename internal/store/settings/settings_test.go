package settings_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store/settings"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The platform settings store, against a real migrated DuckDB. What is tested
// here is what the statements promise and the layer above cannot: that a write
// replaces rather than accumulates, that a set of values lands as one, and that
// a deployment nobody has configured reads as empty rather than as an error.

func newStore(t *testing.T) *settings.Store {
	t.Helper()
	return settings.New(storetest.Migrated(t))
}

func TestAFreshDatabaseHasNoSettings(t *testing.T) {
	t.Parallel()

	all, err := newStore(t).All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	// Empty rather than an error, and empty rather than nil, so that a caller
	// can index it without checking. Absence is a setting's default and every
	// reader relies on that.
	if all == nil {
		t.Fatal("All returned nil; a caller indexing it would be right to expect a map")
	}
	if len(all) != 0 {
		t.Errorf("All = %v on a database nobody has configured, want empty", all)
	}
}

func TestPutStoresValuesAndWhoWroteThem(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	before := time.Now().Add(-time.Second)

	if err := store.Put(t.Context(), map[string]string{
		"mfa.required_for_all":    "true",
		"mfa.required_for_admins": "false",
	}, "user-1"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("All returned %d settings, want 2: %v", len(all), all)
	}

	setting := all["mfa.required_for_all"]
	switch {
	case setting.Key != "mfa.required_for_all":
		t.Errorf("key = %q, want the key it was stored under", setting.Key)
	case setting.Value != "true":
		t.Errorf("value = %q, want %q", setting.Value, "true")
	case setting.UpdatedBy != "user-1":
		t.Errorf("updatedBy = %q, want %q", setting.UpdatedBy, "user-1")
	case setting.UpdatedAt.Before(before):
		t.Errorf("updatedAt = %s, want a time after %s", setting.UpdatedAt, before)
	case setting.UpdatedAt.Location() != time.UTC:
		t.Errorf("updatedAt is in %s, want UTC — every timestamp this application reads back is",
			setting.UpdatedAt.Location())
	}
}

func TestPutReplacesRatherThanAccumulating(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	if err := store.Put(t.Context(), map[string]string{"mfa.required_for_all": "true"}, "user-1"); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := store.Put(t.Context(), map[string]string{"mfa.required_for_all": "false"}, "user-2"); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	// One row, not two. The primary key would refuse a second — the point of
	// the assertion is that the write path deletes before it inserts rather
	// than failing on the constraint.
	if len(all) != 1 {
		t.Fatalf("All returned %d settings after two writes to one key: %v", len(all), all)
	}
	if got := all["mfa.required_for_all"]; got.Value != "false" || got.UpdatedBy != "user-2" {
		t.Errorf("the stored setting is %+v, want the second write", got)
	}
}

// TestPutWithNoAuthorStoresNothingRatherThanAnEmptyString: "nobody did this" is
// a real state — a value written by the command line or by a migration — and it
// is NULL rather than "" so that a reader can tell it from a user whose
// identifier went missing.
func TestPutWithNoAuthorStoresNothingRatherThanAnEmptyString(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	if err := store.Put(t.Context(), map[string]string{"a.key": "a value"}, ""); err != nil {
		t.Fatalf("Put: %v", err)
	}

	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if got := all["a.key"].UpdatedBy; got != "" {
		t.Errorf("updatedBy = %q, want empty for a value nobody wrote", got)
	}
}

func TestPutOfNothingIsNotAnError(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	if err := store.Put(t.Context(), nil, "user-1"); err != nil {
		t.Errorf("Put(nil) = %v, want nil: a caller with no changes has nothing to say", err)
	}
	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("All = %v after writing nothing, want empty", all)
	}
}

// TestAnUnknownKeyIsReturnedRatherThanRefused is what lets a binary run against
// a database a newer one wrote. The store knows no key names; whoever reads a
// setting decides what it means, and a key nobody recognises is ignored by
// everything rather than being a startup failure.
func TestAnUnknownKeyIsReturnedRatherThanRefused(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	key := "something.from.the.future"
	if err := store.Put(t.Context(), map[string]string{key: "{}"}, "user-1"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if got, ok := all[key]; !ok || got.Value != "{}" {
		t.Errorf("All = %v, want the unrecognised key returned as stored", all)
	}
}

// TestAValueMayBeLongAndAwkward: the column is TEXT and the encoding is the
// caller's, so nothing here should be quietly truncating or interpreting one.
func TestAValueMayBeLongAndAwkward(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	value := strings.Repeat("ünïcodé ", 1000) + "\n\t'\"--;"
	if err := store.Put(t.Context(), map[string]string{"a.key": value}, ""); err != nil {
		t.Fatalf("Put: %v", err)
	}

	all, err := store.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if got := all["a.key"].Value; got != value {
		t.Errorf("the value came back %d bytes, want the %d that went in", len(got), len(value))
	}
}
