package attackpin_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// The release catalog behind the first-run version picker. Three things are
// worth holding: upstream's order survives, what is installed is marked, and an
// upstream nobody can reach is an answer rather than a failure.

func TestReleaseCatalogMarksWhatIsInstalled(t *testing.T) {
	t.Parallel()

	rt := newPinRuntime(t)
	rt.adapter.IndexBytes = readAttackFixture(t, "index.json")
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	catalog, err := rt.pin.ListReleases(t.Context())
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if !catalog.Reachable {
		t.Fatalf("catalog reported unreachable (%q) with an index in hand", catalog.Unreachable)
	}
	if len(catalog.Items) != 2 {
		t.Fatalf("items = %+v, want the two Enterprise releases the index offers", catalog.Items)
	}

	// Upstream order, newest first, and exactly one release marked latest.
	if catalog.Items[0].Version != "15.1" || catalog.Items[1].Version != "14.1" {
		t.Errorf("items = %q, %q; want the index's own order preserved",
			catalog.Items[0].Version, catalog.Items[1].Version)
	}
	if !catalog.Items[0].Latest || catalog.Items[1].Latest {
		t.Errorf("latest flags = %v, %v; want it on the first release only",
			catalog.Items[0].Latest, catalog.Items[1].Latest)
	}

	if !catalog.Items[0].Installed {
		t.Error("15.1 was synced and is not marked installed; the picker would offer to install it again")
	}
	if got := catalog.Items[0].Status; got != storecontent.VersionStatusReady {
		t.Errorf("status = %q, want %q", got, storecontent.VersionStatusReady)
	}
	if catalog.Items[1].Installed {
		t.Error("14.1 was never synced and is marked installed")
	}
	if !catalog.SourceEnabled {
		t.Error("SourceEnabled = false on an enabled attack source")
	}
}

// An offline bundle can carry any label. Hiding a label upstream no longer
// lists would tell an administrator they have nothing installed when they do.
func TestReleaseCatalogKeepsInstalledVersionsUpstreamDoesNotOffer(t *testing.T) {
	t.Parallel()

	rt := newPinRuntime(t)
	// An index that has moved on: 14.1 is installed here and no longer offered.
	rt.adapter.IndexBytes = []byte(`{"collections":[{"name":"Enterprise ATT&CK",
		"versions":[{"version":"15.1","url":"https://example.test/15.1.json"}]}]}`)
	mustSyncFixture(t, rt, "14.1", "enterprise-mini-14.1.json")

	catalog, err := rt.pin.ListReleases(t.Context())
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}

	var found *attackpin.ReleaseInfo
	for i := range catalog.Items {
		if catalog.Items[i].Version == "14.1" {
			found = &catalog.Items[i]
		}
	}
	if found == nil {
		t.Fatalf("items = %+v, want the installed 14.1 among them", catalog.Items)
	}
	if !found.Installed || found.Latest {
		t.Errorf("14.1 = %+v, want installed and not latest: upstream no longer offers it", *found)
	}
	if catalog.Items[len(catalog.Items)-1].Version != "14.1" {
		t.Error("the label upstream does not offer should come after the ones it does")
	}
}

func TestAnUnreachableUpstreamIsACatalogRatherThanAnError(t *testing.T) {
	t.Parallel()

	rt := newPinRuntime(t)
	rt.adapter.IndexBytes = readAttackFixture(t, "index.json")
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	pin := newPinWithUpstream(t, rt, failingUpstream{errors.New("dial tcp: no route to host")})

	catalog, err := pin.ListReleases(t.Context())
	if err != nil {
		t.Fatalf("ListReleases returned an error for an air-gapped installation: %v", err)
	}
	if catalog.Reachable {
		t.Error("Reachable = true after upstream refused to answer")
	}
	if !strings.Contains(catalog.Unreachable, "no route to host") {
		t.Errorf("Unreachable = %q, want it to carry what actually went wrong", catalog.Unreachable)
	}
	// What is installed is local knowledge and survives the network being gone.
	if len(catalog.Items) != 1 || catalog.Items[0].Version != "15.1" || !catalog.Items[0].Installed {
		t.Errorf("items = %+v, want the installed release still listed", catalog.Items)
	}
	if catalog.Items[0].Latest {
		t.Error("Latest was claimed with no index to claim it from")
	}
}

func TestNoUpstreamWiredReadsAsUnreachable(t *testing.T) {
	t.Parallel()

	rt := newPinRuntime(t)
	pin := newPinWithUpstream(t, rt, nil)

	catalog, err := pin.ListReleases(t.Context())
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if catalog.Reachable {
		t.Error("Reachable = true in a process that cannot contact anything")
	}
	if catalog.Unreachable == "" {
		t.Error("Unreachable is empty; the caller is owed a reason it has nothing to show")
	}
	if catalog.Items == nil {
		t.Error("Items is nil; a caller ranging over it should not have to check")
	}
}

// A rolling source has no releases to choose between, and the runner says so
// with an empty list rather than an error.
func TestARollingSourceHasNoReleases(t *testing.T) {
	t.Parallel()

	rt := newPinRuntime(t)
	releases, err := rt.runner.ListReleases(t.Context(), storecontent.SourceIDAtomic)
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("releases = %+v, want none: no adapter registered for that kind here", releases)
	}
}

func newPinWithUpstream(t *testing.T, rt *pinRuntime, up attackpin.Upstream) *attackpin.Service {
	t.Helper()
	pin, err := attackpin.New(attackpin.Deps{
		Sources:  rt.sources,
		Versions: rt.versions,
		Objects:  rt.objects,
		Upstream: up,
	})
	if err != nil {
		t.Fatalf("attackpin.New: %v", err)
	}
	return pin
}

type failingUpstream struct{ err error }

func (f failingUpstream) ListReleases(context.Context, string) ([]content.Release, error) {
	return nil, f.err
}
