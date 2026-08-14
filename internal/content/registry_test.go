package content_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestRegistryUpdateSourceURLValidation(t *testing.T) {
	t.Parallel()

	db := storetest.Migrated(t)
	public := func(_ context.Context, _ string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("8.8.8.8")}, nil
	}
	reg, err := content.New(content.Deps{
		Sources:  storecontent.NewSources(db),
		Versions: storecontent.NewVersions(db, storecontent.Paths{}),
		Jobs:     storecontent.NewJobs(db),
		Activity: events.New(activity.New(db)),
		Policy:   content.URLPolicy{AllowHTTP: false, LookupIP: public},
	})
	if err != nil {
		t.Fatal(err)
	}
	actor := authn.Subject{UserID: "actor"}

	for _, bad := range []string{
		"http://127.0.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
	} {
		_, err := reg.UpdateSource(t.Context(), actor, storecontent.SourceIDAttack, content.SourceEdit{URL: &bad})
		if !errors.Is(err, apierr.ErrValidation) {
			t.Fatalf("UpdateSource(%q) = %v, want a validation error", bad, err)
		}
	}

	good := "https://github.com/mitre-attack/attack-stix-data"
	if _, err := reg.UpdateSource(t.Context(), actor, storecontent.SourceIDAttack, content.SourceEdit{URL: &good}); err != nil {
		t.Fatalf("UpdateSource(%q) = %v, want nil", good, err)
	}
}
func testRegistry(t *testing.T) (*content.Registry, *storecontent.Sources) {
	t.Helper()
	db := storetest.Migrated(t)
	sources := storecontent.NewSources(db)
	reg, err := content.New(content.Deps{
		Sources:  sources,
		Versions: storecontent.NewVersions(db, storecontent.Paths{}),
		Jobs:     storecontent.NewJobs(db),
		Activity: events.New(activity.New(db)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return reg, sources
}

func TestRegistryEnableDisableIdempotent(t *testing.T) {
	t.Parallel()
	reg, _ := testRegistry(t)
	actor := authn.Subject{UserID: "actor"}

	// Seed attack is disabled.
	src, err := reg.EnableSource(t.Context(), actor, storecontent.SourceIDAttack)
	if err != nil {
		t.Fatal(err)
	}
	if !src.Enabled {
		t.Fatal("not enabled")
	}
	// Second enable is a no-op.
	again, err := reg.EnableSource(t.Context(), actor, storecontent.SourceIDAttack)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Enabled {
		t.Fatal("second enable cleared it")
	}

	disabled, err := reg.DisableSource(t.Context(), actor, storecontent.SourceIDAttack)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatal("still enabled")
	}
	if err := content.AssertReferencable(disabled); err == nil {
		t.Fatal("disabled source still referencable")
	}
}

func TestRegistryDeleteCustomRefused(t *testing.T) {
	t.Parallel()
	reg, _ := testRegistry(t)
	err := reg.DeleteSource(t.Context(), authn.Subject{UserID: "actor"}, storecontent.SourceIDCustom)
	if err == nil {
		t.Fatal("custom delete accepted")
	}
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestRegistryDeleteCascade(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, storecontent.Paths{})
	jobs := storecontent.NewJobs(db)
	reg, err := content.New(content.Deps{
		Sources:  sources,
		Versions: versions,
		Jobs:     jobs,
		Activity: events.New(activity.New(db)),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := versions.Create(t.Context(), storecontent.NewSourceVersion{
		SourceID: storecontent.SourceIDCTID,
		Version:  storecontent.VersionCurrent,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.Create(t.Context(), storecontent.NewJob{
		SourceID:  storecontent.SourceIDCTID,
		Kind:      storecontent.JobKindSync,
		CreatedBy: "actor",
	}); err != nil {
		t.Fatal(err)
	}

	if err := reg.DeleteSource(t.Context(), authn.Subject{UserID: "actor"}, storecontent.SourceIDCTID); err != nil {
		t.Fatal(err)
	}
	if _, err := sources.ByID(t.Context(), storecontent.SourceIDCTID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("source still present: %v", err)
	}
	remaining, err := versions.ListBySource(t.Context(), storecontent.SourceIDCTID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("versions remain: %d", len(remaining))
	}
}
