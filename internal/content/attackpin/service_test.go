package attackpin_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestResolveTechniqueVersionIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newPinRuntime(t)

	mustSyncFixture(t, rt, "14.1", "enterprise-mini-14.1.json")
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	tech, err := rt.pin.ResolveTechnique(ctx, "15.1", "T1059.001")
	if err != nil {
		t.Fatalf("ResolveTechnique 15.1: %v", err)
	}
	if tech.Version != "15.1" || tech.ExternalID != "T1059.001" {
		t.Fatalf("got %+v", tech)
	}

	_, err = rt.pin.ResolveTechnique(ctx, "14.1", "T1059.001")
	if err == nil {
		t.Fatal("14.1 must not resolve 15.1-only T1059.001")
	}
	if !errors.Is(err, apierr.ErrNotFound) {
		// Technique missing is store NotFound, not version not found.
		if !errors.Is(err, apierr.ErrNotFound) {
			var ae *apierr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("want not-found, got %v", err)
			}
		}
	}

	// Shared external id keeps the version-local description.
	t14, err := rt.pin.ResolveTechnique(ctx, "14.1", "T1059")
	if err != nil {
		t.Fatalf("14.1 T1059: %v", err)
	}
	t15, err := rt.pin.ResolveTechnique(ctx, "15.1", "T1059")
	if err != nil {
		t.Fatalf("15.1 T1059: %v", err)
	}
	if t14.Description == t15.Description {
		t.Fatalf("descriptions not isolated: %q", t14.Description)
	}
	if t14.Version != "14.1" || t15.Version != "15.1" {
		t.Fatalf("version leak: 14=%q 15=%q", t14.Version, t15.Version)
	}
}

func TestAssertPinned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newPinRuntime(t)
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	if err := rt.pin.AssertPinned(ctx, "15.1"); err != nil {
		t.Fatalf("ready version: %v", err)
	}
	if err := rt.pin.AssertPinned(ctx, ""); err == nil {
		t.Fatal("empty version should fail")
	}
	errMiss := rt.pin.AssertPinned(ctx, "99.9")
	if errMiss == nil {
		t.Fatal("missing version should fail")
	}
	if !errors.Is(errMiss, attackpin.ErrVersionNotFound) {
		t.Fatalf("missing: %v", errMiss)
	}

	if _, err := rt.sources.SetEnabled(ctx, storecontent.SourceIDAttack, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	err := rt.pin.AssertPinned(ctx, "15.1")
	if err == nil {
		t.Fatal("disabled source should fail AssertPinned")
	}
	if !errors.Is(err, apierr.ErrConflict) {
		var ae *apierr.Error
		if !errors.As(err, &ae) || ae.Code() != apierr.Code("conflict") {
			t.Fatalf("disabled want conflict, got %v", err)
		}
	}
}

func TestDeleteVersionIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newPinRuntime(t)
	mustSyncFixture(t, rt, "14.1", "enterprise-mini-14.1.json")
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	if err := rt.pin.DeleteVersion(ctx, authn.Subject{UserID: "admin"}, "14.1"); err != nil {
		t.Fatalf("DeleteVersion 14.1: %v", err)
	}

	if _, err := rt.pin.Resolve(ctx, "14.1"); !errors.Is(err, attackpin.ErrVersionNotFound) {
		t.Fatalf("14.1 resolve after delete: %v", err)
	}
	info, err := rt.pin.Resolve(ctx, "15.1")
	if err != nil {
		t.Fatalf("15.1 resolve: %v", err)
	}
	if info.ItemCount == 0 {
		t.Fatal("15.1 emptied by 14.1 delete")
	}

	list15, err := rt.objects.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list15) != 2 {
		t.Fatalf("15.1 techniques = %d, want 2", len(list15))
	}
	list14, err := rt.objects.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "14.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list14) != 0 {
		t.Fatalf("14.1 techniques residual = %d", len(list14))
	}

	// Activity verb recorded.
	entries, _, err := activity.New(rt.db).List(ctx, activity.ListFilter{
		ScopePlatform: true,
		Verb:          string(events.VerbContentVersionDeleted),
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected content.version.deleted activity")
	}
	if entries[0].ObjectType != events.ObjectContentSourceVersion {
		t.Fatalf("object type = %q", entries[0].ObjectType)
	}
}

func TestDeleteVersionReferenced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newPinRuntime(t)
	rt.refs.count = 3
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	err := rt.pin.DeleteVersion(ctx, authn.Subject{UserID: "admin"}, "15.1")
	if err == nil {
		t.Fatal("want 409 when referenced")
	}
	if !errors.Is(err, attackpin.ErrNotReferencable) {
		t.Fatalf("got %v", err)
	}
	mapped := attackpin.MapError(err)
	if !errors.Is(mapped, apierr.ErrConflict) {
		t.Fatalf("mapped = %v", mapped)
	}
	// Catalog still present.
	if _, err := rt.pin.Resolve(ctx, "15.1"); err != nil {
		t.Fatalf("still installed: %v", err)
	}
}

func TestListAndDetail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newPinRuntime(t)
	mustSyncFixture(t, rt, "14.1", "enterprise-mini-14.1.json")
	mustSyncFixture(t, rt, "15.1", "enterprise-mini-15.1.json")

	list, err := rt.pin.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d, want 2", len(list))
	}
	detail, err := rt.pin.ResolveDetail(ctx, "15.1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Counts.Techniques != 2 {
		t.Fatalf("techniques count = %d", detail.Counts.Techniques)
	}
	if detail.Counts.Tactics < 1 {
		t.Fatalf("tactics count = %d", detail.Counts.Tactics)
	}
}

func TestNopReferencesCompilesAndReturnsZero(t *testing.T) {
	t.Parallel()
	var refs attackpin.References = attackpin.NopReferences{}
	n, err := refs.AttackVersion(context.Background(), "15.1")
	if err != nil || n != 0 {
		t.Fatalf("NopReferences = %d, %v", n, err)
	}
}

type countingRefs struct {
	count int64
	err   error
}

func (c *countingRefs) AttackVersion(context.Context, string) (int64, error) {
	return c.count, c.err
}

type pinRuntime struct {
	db       *store.DB
	sources  *storecontent.Sources
	versions *storecontent.Versions
	objects  *storecontent.Objects
	adapter  *attack.Adapter
	runner   *content.Runner
	pin      *attackpin.Service
	refs     *countingRefs
}

func newPinRuntime(t *testing.T) *pinRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	objects := storecontent.NewObjects(db)
	adapter := attack.New()
	refs := &countingRefs{}

	if _, err := sources.SetEnabled(context.Background(), storecontent.SourceIDAttack, true); err != nil {
		t.Fatalf("enable attack: %v", err)
	}

	r, err := content.NewRunner(content.RunnerDeps{
		DB:         db,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   events.New(activity.New(db)),
		Adapters:   map[storecontent.Kind]content.Adapter{storecontent.KindAttack: adapter},
		MaxBytes:   512 << 20,
		JobTimeout: time.Minute,
		WriteBatch: 50,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if err := r.Boot(context.Background()); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	r.Start(context.Background())
	t.Cleanup(r.Stop)

	pin, err := attackpin.New(attackpin.Deps{
		Sources:  sources,
		Versions: versions,
		Objects:  objects,
		Paths:    paths,
		Activity: events.New(activity.New(db)),
		Refs:     refs,
		Upstream: r,
	})
	if err != nil {
		t.Fatalf("attackpin.New: %v", err)
	}

	return &pinRuntime{
		db:       db,
		sources:  sources,
		versions: versions,
		objects:  objects,
		adapter:  adapter,
		runner:   r,
		pin:      pin,
		refs:     refs,
	}
}

func mustSyncFixture(t *testing.T, rt *pinRuntime, version, fixture string) {
	t.Helper()
	rt.adapter.FetchBytes = readAttackFixture(t, fixture)
	job, err := rt.runner.StartSync(context.Background(), authn.Subject{UserID: "admin"}, content.StartSyncRequest{
		SourceID: storecontent.SourceIDAttack,
		Version:  version,
	})
	if err != nil {
		t.Fatalf("StartSync %s: %v", version, err)
	}
	job, err = rt.runner.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Wait %s: %v", version, err)
	}
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("sync %s: status=%s err=%q", version, job.Status, job.Error)
	}
}

func readAttackFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "attack", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return b
		}
	}
	t.Fatalf("fixture %s not found", name)
	return nil
}
