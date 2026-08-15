package attackpin

import (
	"context"
	"time"

	"github.com/bryanster/blacklight/internal/content"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// The release catalog: what upstream offers, next to what is installed.
//
// [Service.ListVersions] answers "what does this installation hold", which is
// the question the sources screen asks. The first-run wizard asks the other
// one — "what could it hold" — and a picker needs both in one answer, because
// the useful thing to show beside ATT&CK 17.1 is whether it is already here.

// ReleaseInfo is one ATT&CK release as a picker sees it.
type ReleaseInfo struct {
	// Version is upstream's label, and the string a caller pins with.
	Version string

	// Released is when upstream published it, or the zero time when upstream
	// did not say, or when this release is only known because it is installed.
	Released time.Time

	// Installed is whether this installation already holds a version row for
	// this label — in any state. Status says which.
	Installed bool

	// Status is the installed version's state, and "" when Installed is false.
	// A release that is installed but `failed` is worth offering again, which
	// is why this is a status and not a second boolean.
	Status storecontent.VersionStatus

	// Latest marks upstream's newest release. Exactly one release carries it
	// when upstream was reachable, and none do when it was not: without the
	// index there is nothing that knows which is newest, and guessing from
	// labels is how "4.0" becomes newer than "17.1".
	Latest bool
}

// ReleaseCatalog is the answer to "what can this installation install".
type ReleaseCatalog struct {
	// Items are the releases, newest first, in upstream's own order. Installed
	// releases upstream no longer lists are appended after them: an offline
	// bundle can carry any label, and a catalog that omitted it would offer to
	// install something that is already here.
	Items []ReleaseInfo

	// Reachable is whether upstream answered. False is a normal outcome, not a
	// failure of the request: an air-gapped installation is a supported
	// deployment, and it should be told that the picker is empty because
	// nothing answered rather than because there is nothing to install.
	Reachable bool

	// Unreachable is why, when Reachable is false: the transport or index error,
	// for an administrator reading it beside the source URL they configured.
	// Empty when Reachable is true.
	Unreachable string

	// SourceEnabled mirrors the attack source's enabled flag, because a picker
	// that offers a version on a disabled source is offering a sync that will
	// be refused.
	SourceEnabled bool
}

// Upstream is the part of the content runner this package needs: one read of a
// source's published releases, with no job and no writes.
//
// Declared here, in the consumer, so that a test can answer it with a slice and
// so that this package does not depend on the runner's construction.
// [*content.Runner] satisfies it.
type Upstream interface {
	ListReleases(ctx context.Context, sourceID string) ([]content.Release, error)
}

// ListReleases returns upstream's ATT&CK releases merged with what is
// installed.
//
// An upstream that cannot be reached is reported in the catalog rather than
// raised: the caller is a picker, and "nothing answered, here is what you
// already have, the offline path is over there" is a screen. An error would be
// a dead end, and the deployments most likely to hit it — air-gapped ones — are
// exactly the deployments for which it is not a fault.
//
// What is *not* forgiving is the local half. If the installed versions cannot
// be read, that is this installation's own database failing, and it is raised.
func (s *Service) ListReleases(ctx context.Context) (ReleaseCatalog, error) {
	src, err := s.attackSource(ctx)
	if err != nil {
		return ReleaseCatalog{}, err
	}
	rows, err := s.versions.ListBySource(ctx, src.ID)
	if err != nil {
		return ReleaseCatalog{}, err
	}
	installed := make(map[string]storecontent.SourceVersion, len(rows))
	for _, row := range rows {
		installed[row.Version] = row
	}

	catalog := ReleaseCatalog{SourceEnabled: src.Enabled, Reachable: true}

	var upstream []content.Release
	if s.upstream == nil {
		// No runner wired — blctl, and tests that never reach the network.
		// Treating that as "unreachable" rather than as an empty upstream keeps
		// the two apart for whoever reads the answer.
		catalog.Reachable = false
		catalog.Unreachable = "this process has no content runner, so upstream was not contacted"
	} else if upstream, err = s.upstream.ListReleases(ctx, src.ID); err != nil {
		catalog.Reachable = false
		catalog.Unreachable = err.Error()
	}

	seen := make(map[string]bool, len(upstream)+len(rows))
	for i, rel := range upstream {
		if seen[rel.Version] {
			continue
		}
		seen[rel.Version] = true
		item := ReleaseInfo{
			Version:  rel.Version,
			Released: rel.Released,
			Latest:   i == 0,
		}
		if row, ok := installed[rel.Version]; ok {
			item.Installed = true
			item.Status = row.Status
		}
		catalog.Items = append(catalog.Items, item)
	}

	// Installed labels upstream did not offer, in the store's order. A release
	// that was withdrawn upstream, or a bundle labelled by hand, is still a
	// thing this installation holds and a picker that hid it would be lying
	// about what is here.
	for _, row := range rows {
		if seen[row.Version] || row.Version == storecontent.VersionCurrent {
			continue
		}
		seen[row.Version] = true
		catalog.Items = append(catalog.Items, ReleaseInfo{
			Version:   row.Version,
			Released:  row.SyncedAt,
			Installed: true,
			Status:    row.Status,
		})
	}

	if catalog.Items == nil {
		catalog.Items = []ReleaseInfo{}
	}
	return catalog, nil
}

// SetUpstream wires the release reader after construction.
//
// It exists because of an ordering: this service is a dependency of the
// engagement service, and the content runner that reads upstream is built after
// both. A setter keeps that construction order and is safe for the one caller
// that has it — internal/httpapi, wiring at start-up before a request can
// arrive. Nothing calls it twice, and nothing calls it while serving.
func (s *Service) SetUpstream(u Upstream) { s.upstream = u }
