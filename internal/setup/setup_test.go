package setup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/setup"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// The first-run state. What is worth testing here is not that a string round
// trips through a map — it is the three decisions the package makes on behalf
// of the interface above it: that an unconfigured installation needs setup,
// that finishing twice does not move the record of when it was finished, and
// that a row nobody can read is a failure rather than a wizard somebody has
// already been through appearing again.

func TestAFreshInstallationNeedsSetup(t *testing.T) {
	t.Parallel()

	state, err := newService(t, &fakeSettings{}).State(t.Context())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if state.Completed {
		t.Error("State reported a fresh installation as already set up; the wizard would never be shown")
	}
	if !state.CompletedAt.IsZero() {
		t.Errorf("CompletedAt = %v on an installation nobody has configured, want the zero time", state.CompletedAt)
	}
}

func TestCompleteRecordsWhenAndWhoAndIsReadBack(t *testing.T) {
	t.Parallel()

	store := &fakeSettings{}
	svc := newService(t, store)
	before := time.Now().UTC().Add(-time.Second)

	written, err := svc.Complete(t.Context(), authn.Subject{UserID: "admin-1"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !written.Completed {
		t.Fatal("Complete returned a state that is not completed")
	}
	if written.CompletedAt.Before(before) {
		t.Errorf("CompletedAt = %v, want a time after %v", written.CompletedAt, before)
	}
	if written.CompletedBy != "admin-1" {
		t.Errorf("CompletedBy = %q, want the subject that finished the wizard", written.CompletedBy)
	}

	// The answer a later request gets has to be the answer this one got —
	// otherwise the wizard reappears for the next administrator to sign in.
	read, err := svc.State(t.Context())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if !read.Completed || !read.CompletedAt.Equal(written.CompletedAt) || read.CompletedBy != "admin-1" {
		t.Errorf("State = %+v after Complete returned %+v; the two disagree", read, written)
	}
}

func TestCompletingTwiceKeepsTheFirstAnswer(t *testing.T) {
	t.Parallel()

	store := &fakeSettings{}
	svc := newService(t, store)

	first, err := svc.Complete(t.Context(), authn.Subject{UserID: "admin-1"})
	if err != nil {
		t.Fatalf("first Complete: %v", err)
	}

	second, err := svc.Complete(t.Context(), authn.Subject{UserID: "admin-2"})
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if !second.CompletedAt.Equal(first.CompletedAt) {
		t.Errorf("CompletedAt moved from %v to %v; when the installation was set up is not new information",
			first.CompletedAt, second.CompletedAt)
	}
	if second.CompletedBy != "admin-1" {
		t.Errorf("CompletedBy = %q after a second Complete, want the administrator who actually did it", second.CompletedBy)
	}
	if store.writes != 1 {
		t.Errorf("the settings store was written %d times, want 1: a retried Finish should not rewrite the row", store.writes)
	}
}

func TestAnUnreadableRowIsAnErrorRatherThanAWizard(t *testing.T) {
	t.Parallel()

	store := &fakeSettings{stored: map[string]settings.Setting{
		setup.KeyCompletedAt: {Key: setup.KeyCompletedAt, Value: "yesterday"},
	}}

	_, err := newService(t, store).State(t.Context())
	if err == nil {
		t.Fatal("State accepted a timestamp it could not parse; a running installation would be sent back to the wizard")
	}
	if !strings.Contains(err.Error(), setup.KeyCompletedAt) {
		t.Errorf("error = %q, want it to name the setting that is unreadable", err)
	}
}

func TestNewRefusesAServiceItCannotRead(t *testing.T) {
	t.Parallel()

	if _, err := setup.New(setup.Deps{}); err == nil {
		t.Fatal("New accepted a Service with no settings store")
	}
}

func newService(t *testing.T, store setup.Settings) *setup.Service {
	t.Helper()
	svc, err := setup.New(setup.Deps{Settings: store})
	if err != nil {
		t.Fatalf("setup.New: %v", err)
	}
	return svc
}

// fakeSettings is the platform setting table, in a map. The real store has its
// own tests against DuckDB; what these tests need from it is the ability to see
// how many times it was written.
type fakeSettings struct {
	stored map[string]settings.Setting
	writes int
	err    error
}

func (f *fakeSettings) All(context.Context) (map[string]settings.Setting, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]settings.Setting{}
	for k, v := range f.stored {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSettings) Put(_ context.Context, values map[string]string, updatedBy string) error {
	if f.err != nil {
		return f.err
	}
	f.writes++
	if f.stored == nil {
		f.stored = map[string]settings.Setting{}
	}
	at := time.Now().UTC()
	for k, v := range values {
		f.stored[k] = settings.Setting{Key: k, Value: v, UpdatedAt: at, UpdatedBy: updatedBy}
	}
	return nil
}

var _ setup.Settings = (*fakeSettings)(nil)

// errStore is here so the compiler keeps errors imported if the failure-path
// test above changes shape; it also documents the one error State can raise
// that is not the caller's fault.
var errStore = errors.New("settings unavailable")

func TestStateReportsAStoreThatCannotBeRead(t *testing.T) {
	t.Parallel()

	_, err := newService(t, &fakeSettings{err: errStore}).State(t.Context())
	if !errors.Is(err, errStore) {
		t.Errorf("State error = %v, want the store's own error", err)
	}
}
