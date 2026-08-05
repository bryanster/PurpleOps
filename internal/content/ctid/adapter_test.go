package ctid_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/ctid"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestParseNormalizeFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := ctid.New()
	a.FetchBytes = readTestdata(t, "plans-mini.zip")

	bundle, err := a.Fetch(ctx, content.FetchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ast, err := a.Parse(ctx, bundle)
	if err != nil {
		t.Fatal(err)
	}
	objs, err := a.Normalize(ctx, ast)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 {
		t.Fatalf("objects = %d", len(objs))
	}
	cat, ok := objs[0].(*ctid.Catalog)
	if !ok {
		t.Fatalf("type %T", objs[0])
	}
	if len(cat.Plans) != 1 {
		t.Fatalf("plans = %d", len(cat.Plans))
	}
	p := cat.Plans[0]
	if p.ExternalID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("plan external_id = %q", p.ExternalID)
	}
	if p.AdversaryName != "Fixture Eagle" {
		t.Fatalf("adversary = %q", p.AdversaryName)
	}
	if len(p.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(p.Steps))
	}
	// Ordinals dense and ascending from document order.
	for i, s := range p.Steps {
		if s.Position != i+1 {
			t.Fatalf("step[%d].position = %d, want %d", i, s.Position, i+1)
		}
	}
	if p.Steps[0].TechniqueExternalID != "T1566.001" {
		t.Fatalf("step0 technique = %q", p.Steps[0].TechniqueExternalID)
	}
	if p.Steps[1].TechniqueExternalID != "T1087.002" {
		t.Fatalf("step1 technique = %q", p.Steps[1].TechniqueExternalID)
	}
	// Missing technique allowed (null/empty) and counted.
	if p.Steps[2].TechniqueExternalID != "" {
		t.Fatalf("step2 technique = %q, want empty", p.Steps[2].TechniqueExternalID)
	}
	if cat.MissingTechniques != 1 {
		t.Fatalf("missing techniques = %d", cat.MissingTechniques)
	}
	// Procedure JSON preserves command + cleanup distinctly.
	var proc map[string]any
	if err := json.Unmarshal(p.Steps[1].Procedure, &proc); err != nil {
		t.Fatal(err)
	}
	execs, ok := proc["executors"].([]any)
	if !ok || len(execs) != 1 {
		t.Fatalf("executors = %#v", proc["executors"])
	}
	ex0, ok := execs[0].(map[string]any)
	if !ok {
		t.Fatalf("executor[0] = %#v", execs[0])
	}
	cmd, ok := ex0["command"].(string)
	if !ok || !strings.Contains(cmd, "net user") {
		t.Fatalf("command = %v", ex0["command"])
	}
	cleanup, ok := ex0["cleanup"].(string)
	if !ok || !strings.Contains(cleanup, "cleaned") {
		t.Fatalf("cleanup = %v", ex0["cleanup"])
	}
}

func TestParseBrokenFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := ctid.New()
	a.FetchBytes = readTestdata(t, "broken.yaml")
	bundle, err := a.Fetch(ctx, content.FetchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Parse(ctx, bundle); err == nil {
		t.Fatal("broken plan: want parse error")
	}
}

func TestBareYAMLFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := ctid.New()
	a.FetchBytes = readTestdata(t, "mini-plan.yaml")
	bundle, err := a.Fetch(ctx, content.FetchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ast, err := a.Parse(ctx, bundle)
	if err != nil {
		t.Fatal(err)
	}
	objs, err := a.Normalize(ctx, ast)
	if err != nil {
		t.Fatal(err)
	}
	cat, ok := objs[0].(*ctid.Catalog)
	if !ok {
		t.Fatalf("type %T", objs[0])
	}
	if len(cat.Plans) != 1 || len(cat.Plans[0].Steps) != 3 {
		t.Fatalf("bare yaml catalog = %+v", cat)
	}
}

func TestEmptyCatalogFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := ctid.New()
	// Zip with only skipped noise.
	a.FetchBytes = []byte("PK\x03\x04") // invalid short zip → parse error
	// Better: a valid empty-ish payload that is not plan YAML.
	a.FetchBytes = []byte("title: not a plan\n")
	bundle, err := a.Fetch(ctx, content.FetchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Parse(ctx, bundle); err == nil {
		t.Fatal("non-plan payload: want parse error")
	}
}

func TestSyncReplaceAndDetailOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newCTIDRuntime(t)

	rt.adapter.FetchBytes = readTestdata(t, "plans-mini.zip")
	job := mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("sync status=%s err=%q", job.Status, job.Error)
	}
	if !strings.Contains(job.Message, "missing technique") {
		t.Fatalf("job message %q missing technique warning", job.Message)
	}

	plans := storecontent.NewEmulationPlans(rt.db)
	all, err := plans.List(ctx, storecontent.EmulationPlanListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("list all = %d, want 1", len(all))
	}
	if all[0].ExternalID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("external_id = %q", all[0].ExternalID)
	}
	if all[0].Version != storecontent.VersionCurrent {
		t.Fatalf("version = %q", all[0].Version)
	}
	if all[0].AdversaryName != "Fixture Eagle" {
		t.Fatalf("adversary = %q", all[0].AdversaryName)
	}

	detail, err := plans.DetailByIDEnabled(ctx, all[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 3 {
		t.Fatalf("detail steps = %d", len(detail.Steps))
	}
	for i := 1; i < len(detail.Steps); i++ {
		if detail.Steps[i].Position <= detail.Steps[i-1].Position {
			t.Fatalf("steps not sorted by ordinal: %+v", detail.Steps)
		}
	}
	if detail.Steps[0].Position != 1 || detail.Steps[2].Position != 3 {
		t.Fatalf("positions = %d,%d,%d", detail.Steps[0].Position, detail.Steps[1].Position, detail.Steps[2].Position)
	}

	// Technique filter finds the plan via a step.
	byTech, err := plans.List(ctx, storecontent.EmulationPlanListFilter{Technique: "T1566.001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTech) != 1 {
		t.Fatalf("technique filter = %d", len(byTech))
	}

	// Re-sync same fixture: no duplicate external_ids, still one plan.
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("re-sync status=%s err=%q", job.Status, job.Error)
	}
	all, err = plans.List(ctx, storecontent.EmulationPlanListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after re-sync count = %d, want 1", len(all))
	}
	// No duplicate step external ids under the plan.
	detail, err = plans.DetailByIDEnabled(ctx, all[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range detail.Steps {
		if seen[s.ExternalID] {
			t.Fatalf("duplicate step external_id %q", s.ExternalID)
		}
		seen[s.ExternalID] = true
	}

	// Broken fixture fails and leaves prior catalog intact.
	rt.adapter.FetchBytes = readTestdata(t, "broken.yaml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("broken sync status=%s, want failed err=%q", job.Status, job.Error)
	}
	all, err = plans.List(ctx, storecontent.EmulationPlanListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after broken sync count = %d, want prior 1", len(all))
	}

	// Disable hides from default list.
	if _, err := rt.sources.SetEnabled(ctx, storecontent.SourceIDCTID, false); err != nil {
		t.Fatal(err)
	}
	hidden, err := plans.List(ctx, storecontent.EmulationPlanListFilter{EnabledOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 0 {
		t.Fatalf("enabled-only list = %d, want 0", len(hidden))
	}
	if _, err := plans.ByIDEnabled(ctx, all[0].ID, true); err == nil {
		t.Fatal("ByIDEnabled on disabled source: want error")
	}
}

func TestNoNetworkWhenFetchBytesSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := ctid.New()
	a.FetchBytes = readTestdata(t, "mini-plan.yaml")
	req := content.FetchRequest{
		HTTP: panicHTTP{},
	}
	if _, err := a.Fetch(ctx, req); err != nil {
		t.Fatal(err)
	}
}

type panicHTTP struct{}

func (panicHTTP) Do(*http.Request) (*http.Response, error) {
	panic("network I/O in test")
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type ctidRuntime struct {
	db      store.Store
	sources *storecontent.Sources
	adapter *ctid.Adapter
	runner  *content.Runner
}

func newCTIDRuntime(t *testing.T) *ctidRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	adapter := ctid.New()

	if _, err := sources.SetEnabled(context.Background(), storecontent.SourceIDCTID, true); err != nil {
		t.Fatalf("enable ctid: %v", err)
	}

	r, err := content.NewRunner(content.RunnerDeps{
		DB:         db,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   events.New(activity.New(db)),
		Adapters:   map[storecontent.Kind]content.Adapter{storecontent.KindCTID: adapter},
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

	return &ctidRuntime{
		db:      db,
		sources: sources,
		adapter: adapter,
		runner:  r,
	}
}

func mustSync(t *testing.T, rt *ctidRuntime) storecontent.Job {
	t.Helper()
	job, err := rt.runner.StartSync(context.Background(), authn.Subject{UserID: "admin"}, content.StartSyncRequest{
		SourceID: storecontent.SourceIDCTID,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	job, err = rt.runner.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	return job
}
