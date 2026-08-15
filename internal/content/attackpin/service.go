package attackpin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// VersionInfo is one installed ATT&CK release as the pin surface exposes it.
type VersionInfo struct {
	Version   string
	Status    storecontent.VersionStatus
	ItemCount int64
	SyncedAt  time.Time
	// SourceEnabled mirrors the attack source's enabled flag at read time so
	// callers of List/Resolve can reason about pin-ability without a second
	// round trip. AssertPinned still re-checks.
	SourceEnabled bool
}

// VersionDetail is VersionInfo plus per-family object counts.
type VersionDetail struct {
	VersionInfo
	Counts storecontent.ObjectFamilyCounts
}

// Service is the ATT&CK version catalog and pin API (M2-007).
//
// Authorization is not decided here. Callers that reached these methods already
// hold content.read or content.manage (or are blctl / M3 domain code).
type Service struct {
	sources  *storecontent.Sources
	versions *storecontent.Versions
	objects  *storecontent.Objects
	paths    storecontent.Paths
	activity *events.Log
	refs     References
	upstream Upstream
}

// Deps is everything a [Service] is built from.
type Deps struct {
	Sources  *storecontent.Sources
	Versions *storecontent.Versions
	Objects  *storecontent.Objects
	Paths    storecontent.Paths // optional; empty root skips raw-file cleanup
	Activity *events.Log        // optional; nil skips durable activity rows
	// Refs counts external pins. Nil becomes [NopReferences].
	Refs References
	// Upstream reads the published release list for the version picker
	// (see releases.go). Nil is a process with no content runner — blctl, and
	// tests that never look upstream — and makes the catalog answer
	// "unreachable" rather than pretending upstream offers nothing.
	Upstream Upstream
}

// New returns a Service over deps, or an error naming what is missing.
func New(deps Deps) (*Service, error) {
	switch {
	case deps.Sources == nil:
		return nil, errors.New("attackpin: no source repository")
	case deps.Versions == nil:
		return nil, errors.New("attackpin: no version repository")
	case deps.Objects == nil:
		return nil, errors.New("attackpin: no objects repository")
	}
	refs := deps.Refs
	if refs == nil {
		refs = NopReferences{}
	}
	return &Service{
		sources:  deps.Sources,
		versions: deps.Versions,
		objects:  deps.Objects,
		paths:    deps.Paths,
		activity: deps.Activity,
		refs:     refs,
		upstream: deps.Upstream,
	}, nil
}

// ListVersions returns every ATT&CK version snapshot, version then id order
// from the store. Staging never appears (it is not a version row).
func (s *Service) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	src, err := s.attackSource(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.versions.ListBySource(ctx, src.ID)
	if err != nil {
		return nil, err
	}
	out := make([]VersionInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInfo(row, src.Enabled))
	}
	return out, nil
}

// Resolve returns the installed version matching version exactly, or
// [ErrVersionNotFound].
func (s *Service) Resolve(ctx context.Context, version string) (VersionInfo, error) {
	v, err := NormalizeVersion(version)
	if err != nil {
		return VersionInfo{}, err
	}
	src, err := s.attackSource(ctx)
	if err != nil {
		return VersionInfo{}, err
	}
	row, err := s.versions.BySourceVersion(ctx, src.ID, v)
	if err != nil {
		return VersionInfo{}, mapStoreNotFound(err, v)
	}
	return toInfo(row, src.Enabled), nil
}

// ResolveDetail is Resolve plus per-family counts for the version detail API.
func (s *Service) ResolveDetail(ctx context.Context, version string) (VersionDetail, error) {
	info, err := s.Resolve(ctx, version)
	if err != nil {
		return VersionDetail{}, err
	}
	counts, err := s.objects.CountFamilies(ctx, storecontent.SourceIDAttack, info.Version)
	if err != nil {
		return VersionDetail{}, err
	}
	return VersionDetail{VersionInfo: info, Counts: counts}, nil
}

// ResolveTechnique returns the technique with externalID in exactly version.
// It never falls back to another installed version.
func (s *Service) ResolveTechnique(ctx context.Context, version, externalID string) (storecontent.Technique, error) {
	v, err := NormalizeVersion(version)
	if err != nil {
		return storecontent.Technique{}, err
	}
	ext := strings.TrimSpace(externalID)
	if ext == "" {
		return storecontent.Technique{}, apierr.Validation(apierr.Field(
			"externalId", "externalId must not be empty",
		))
	}
	// Ensure the version row exists so a missing catalog answers version-not-
	// found rather than technique-not-found when the whole pin is unknown.
	if _, err := s.Resolve(ctx, v); err != nil {
		return storecontent.Technique{}, err
	}
	t, err := s.objects.TechniqueByExternal(ctx, storecontent.SourceIDAttack, v, ext)
	if err != nil {
		return storecontent.Technique{}, err
	}
	return t, nil
}

// AssertPinned refuses a pin that is missing, empty after normalize, on a
// disabled source, not ready, or empty. Healthy means: last sync succeeded
// (status ready) and item_count > 0, and the attack source is enabled.
func (s *Service) AssertPinned(ctx context.Context, version string) error {
	v, err := NormalizeVersion(version)
	if err != nil {
		return err
	}
	src, err := s.attackSource(ctx)
	if err != nil {
		return err
	}
	if err := content.AssertReferencable(src); err != nil {
		// Preserve 409 Conflict detail from the source-level helper.
		return err
	}
	row, err := s.versions.BySourceVersion(ctx, src.ID, v)
	if err != nil {
		return mapStoreNotFound(err, v)
	}
	if row.Status != storecontent.VersionStatusReady {
		return notReferencable(v, fmt.Sprintf(
			"ATT&CK version %q is not ready to pin (status %s); wait for a successful sync",
			v, row.Status,
		))
	}
	if row.ItemCount <= 0 {
		return notReferencable(v, fmt.Sprintf(
			"ATT&CK version %q has no catalog objects; re-sync before pinning",
			v,
		))
	}
	return nil
}

// DeleteVersion removes one ATT&CK version's object rows and version snapshot
// when nothing external references the pin. M2 refs always answer zero via
// [NopReferences]; M3 plugs real counts through [References].
//
// Sync of version X never mutates version Y — this delete is the same isolation
// guarantee in reverse: only the named version's rows are touched.
func (s *Service) DeleteVersion(ctx context.Context, actor authn.Subject, version string) error {
	v, err := NormalizeVersion(version)
	if err != nil {
		return err
	}
	src, err := s.attackSource(ctx)
	if err != nil {
		return err
	}
	// Confirm the row exists before asking refs — missing → 404, not 409.
	before, err := s.versions.BySourceVersion(ctx, src.ID, v)
	if err != nil {
		return mapStoreNotFound(err, v)
	}

	count, err := s.refs.AttackVersion(ctx, v)
	if err != nil {
		return fmt.Errorf("attackpin: reference count for %s: %w", v, err)
	}
	if count > 0 {
		return notReferencable(v, fmt.Sprintf(
			"ATT&CK version %q is still referenced by %d engagement(s) or other rows; remove those pins before deleting",
			v, count,
		))
	}

	rawPath := before.RawPath
	deleted, err := s.versions.DeleteAttackCatalog(ctx, src.ID, v, s.recordDeleted(actor.UserID, v, before))
	if err != nil {
		return err
	}
	_ = deleted

	// Best-effort raw snapshot cleanup outside the DB txn. A leftover file is
	// harmless; the version row that pointed at it is already gone.
	s.removeRaw(rawPath)

	// Recompute source bookkeeping from remaining versions so the registry
	// item_count is not stuck on a deleted catalog.
	if err := s.refreshSourceCount(ctx, src); err != nil {
		return err
	}
	return nil
}

func (s *Service) attackSource(ctx context.Context) (storecontent.Source, error) {
	src, err := s.sources.ByID(ctx, storecontent.SourceIDAttack)
	if err != nil {
		return storecontent.Source{}, err
	}
	return src, nil
}

func (s *Service) recordDeleted(actorID, version string, before storecontent.SourceVersion) storecontent.After {
	if s.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return s.activity.Record(ctx, tx, events.Entry{
			ActorID:    actorID,
			Verb:       events.VerbContentVersionDeleted,
			ObjectType: events.ObjectContentSourceVersion,
			ObjectID:   storecontent.AfterEntityID(ctx),
			Delta: events.Delta(map[string]any{
				"version":    version,
				"source_id":  before.SourceID,
				"item_count": before.ItemCount,
				"status":     string(before.Status),
			}),
		})
	}
}

func (s *Service) removeRaw(rel string) {
	if rel == "" || s.paths.Root() == "" {
		return
	}
	abs, err := s.paths.Abs(rel)
	if err != nil {
		return
	}
	_ = os.Remove(abs)
}

func (s *Service) refreshSourceCount(ctx context.Context, src storecontent.Source) error {
	rows, err := s.versions.ListBySource(ctx, src.ID)
	if err != nil {
		return err
	}
	var total int64
	var lastSync time.Time
	for _, row := range rows {
		if row.Status == storecontent.VersionStatusReady {
			total += row.ItemCount
		}
		if row.SyncedAt.After(lastSync) {
			lastSync = row.SyncedAt
		}
	}
	return s.sources.SetSyncState(ctx, src.ID, storecontent.SourceStatusIdle, total, "", lastSync)
}

func toInfo(row storecontent.SourceVersion, sourceEnabled bool) VersionInfo {
	return VersionInfo{
		Version:       row.Version,
		Status:        row.Status,
		ItemCount:     row.ItemCount,
		SyncedAt:      row.SyncedAt,
		SourceEnabled: sourceEnabled,
	}
}

func mapStoreNotFound(err error, version string) error {
	if errors.Is(err, apierr.ErrNotFound) {
		return notFound(version)
	}
	return err
}
