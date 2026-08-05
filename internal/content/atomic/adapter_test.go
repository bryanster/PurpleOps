package atomic_test

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
	"github.com/bryanster/blacklight/internal/content/atomic"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestParseNormalizeFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := atomic.New()
	raw := readTestdata(t, "atomics-mini.zip")

	ast, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: storecontent.VersionCurrent})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	objs, err := a.Normalize(ctx, ast)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("objects = %d, want 1", len(objs))
	}
	cat, ok := objs[0].(*atomic.Catalog)
	if !ok {
		t.Fatalf("object type %T", objs[0])
	}
	// 2 (T1059.001) + 2 (T1059.004) + 1 (T1003) = 5
	if len(cat.Templates) != 5 {
		t.Fatalf("templates = %d, want 5", len(cat.Templates))
	}

	byExt := map[string]atomic.Template{}
	for _, tmpl := range cat.Templates {
		byExt[tmpl.ExternalID] = tmpl
	}

	win, ok := byExt["11111111-1111-4111-8111-111111111111"]
	if !ok {
		t.Fatal("missing windows guid template")
	}
	if win.Executor != "powershell" {
		t.Fatalf("executor = %q", win.Executor)
	}
	if win.Command == "" || win.Cleanup == "" {
		t.Fatal("command/cleanup must stay distinct non-empty fields")
	}
	if win.Cleanup != "" && strings.Contains(win.Command, win.Cleanup) {
		t.Fatal("command must not embed cleanup as a flattened blob")
	}
	if len(win.InputArgs) != 2 {
		t.Fatalf("input args = %d, want 2", len(win.InputArgs))
	}
	argNames := map[string]bool{}
	for _, arg := range win.InputArgs {
		argNames[arg.Name] = true
		if arg.Name == "message" && arg.Default != "hello-atomic" {
			t.Fatalf("message default = %q", arg.Default)
		}
	}
	if !argNames["message"] || !argNames["output_file"] {
		t.Fatalf("input arg names = %v", argNames)
	}
	if len(win.TechniqueExternalIDs) != 1 || win.TechniqueExternalIDs[0] != "T1059.001" {
		t.Fatalf("techniques = %v", win.TechniqueExternalIDs)
	}
	if len(win.Platforms) != 1 || win.Platforms[0] != "windows" {
		t.Fatalf("platforms = %v", win.Platforms)
	}

	// Derived external id when guid missing.
	derived, ok := byExt["T1059.004/1"]
	if !ok {
		t.Fatalf("missing derived external_id; have %v", keys(byExt))
	}
	if derived.Executor != "sh" {
		t.Fatalf("derived executor = %q", derived.Executor)
	}
	if derived.Cleanup != "" {
		t.Fatalf("derived cleanup should be empty, got %q", derived.Cleanup)
	}

	mac, ok := byExt["44444444-4444-4444-8444-444444444444"]
	if !ok {
		t.Fatal("missing macos template")
	}
	if mac.DependencyExecutorName != "sh" || mac.Dependencies == "" {
		t.Fatalf("dependencies not preserved: executor=%q deps=%q", mac.DependencyExecutorName, mac.Dependencies)
	}
	if cat.ItemCount() != 5 {
		t.Fatalf("ItemCount = %d", cat.ItemCount())
	}
}

func TestParseBrokenFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := atomic.New()
	raw := readTestdata(t, "broken.yaml")
	_, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: storecontent.VersionCurrent})
	if err == nil {
		t.Fatal("Parse broken fixture: want error")
	}
	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "atomic_tests") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBareYAMLFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := atomic.New()
	raw := readTestdata(t, "T1059.001.yaml")
	ast, err := a.Parse(ctx, content.Bundle{Bytes: raw})
	if err != nil {
		t.Fatalf("Parse bare yaml: %v", err)
	}
	objs, err := a.Normalize(ctx, ast)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	cat, ok := objs[0].(*atomic.Catalog)
	if !ok {
		t.Fatalf("object type %T", objs[0])
	}
	if len(cat.Templates) != 2 {
		t.Fatalf("templates = %d, want 2", len(cat.Templates))
	}
}

func TestSyncReplaceAndFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newAtomicRuntime(t)

	rt.adapter.FetchBytes = readTestdata(t, "atomics-mini.zip")
	job := mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("sync status=%s err=%q", job.Status, job.Error)
	}

	procs := storecontent.NewProcedures(rt.db)
	all, err := procs.List(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("list all = %d, want 5", len(all))
	}

	// command and cleanup are distinct fields post-sync.
	var echo *storecontent.ProcedureTemplate
	for i := range all {
		if all[i].ExternalID == "11111111-1111-4111-8111-111111111111" {
			echo = &all[i]
			break
		}
	}
	if echo == nil {
		t.Fatal("echo template missing after sync")
	}
	if echo.Command == "" || echo.Cleanup == "" {
		t.Fatalf("command/cleanup flattened: cmd=%q cleanup=%q", echo.Command, echo.Cleanup)
	}
	if echo.Command == echo.Cleanup {
		t.Fatal("command and cleanup must not be identical")
	}
	var args []atomic.InputArg
	if err := json.Unmarshal(echo.InputArgs, &args); err != nil {
		t.Fatalf("input_args json: %v raw=%s", err, echo.InputArgs)
	}
	if len(args) != 2 {
		t.Fatalf("input_args round-trip len=%d raw=%s", len(args), echo.InputArgs)
	}

	byTech, err := procs.List(ctx, storecontent.ProcedureListFilter{Technique: "T1059.001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTech) != 2 {
		t.Fatalf("technique filter = %d, want 2", len(byTech))
	}

	byPlat, err := procs.List(ctx, storecontent.ProcedureListFilter{Platform: "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byPlat) != 2 { // T1059.004/sh + T1003
		t.Fatalf("platform=macos = %d, want 2", len(byPlat))
	}

	// Re-sync with a single-technique bare YAML replaces the catalog.
	rt.adapter.FetchBytes = readTestdata(t, "T1059.001.yaml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("re-sync status=%s err=%q", job.Status, job.Error)
	}
	all, err = procs.List(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("after re-sync count = %d, want 2", len(all))
	}
	seen := map[string]int{}
	for _, p := range all {
		seen[p.ExternalID]++
		if p.Version != storecontent.VersionCurrent {
			t.Fatalf("version = %q, want current", p.Version)
		}
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("duplicate external_id %s count=%d", id, n)
		}
	}

	// Broken fixture fails and leaves prior catalog intact.
	rt.adapter.FetchBytes = readTestdata(t, "broken.yaml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("broken sync status=%s, want failed err=%q", job.Status, job.Error)
	}
	all, err = procs.List(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("after broken sync count = %d, want prior 2", len(all))
	}

	// Disabled source hides templates from default list.
	if _, err := rt.sources.SetEnabled(ctx, storecontent.SourceIDAtomic, false); err != nil {
		t.Fatal(err)
	}
	hidden, err := procs.List(ctx, storecontent.ProcedureListFilter{EnabledOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 0 {
		t.Fatalf("enabled-only list = %d, want 0", len(hidden))
	}
	// ByIDEnabled also hides.
	if _, err := procs.ByIDEnabled(ctx, all[0].ID, true); err == nil {
		t.Fatal("ByIDEnabled on disabled source: want error")
	}
}

func TestNoNetworkWhenFetchBytesSet(t *testing.T) {
	t.Parallel()
	a := atomic.New()
	a.FetchBytes = readTestdata(t, "atomics-mini.zip")
	_, err := a.Fetch(context.Background(), content.FetchRequest{
		Source: content.SourceInfo{URL: "https://should-not-dial.example"},
		HTTP:   panicHTTP{},
	})
	if err != nil {
		t.Fatalf("Fetch with FetchBytes: %v", err)
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
		t.Fatalf("testdata %s: %v", name, err)
	}
	return b
}

func keys(m map[string]atomic.Template) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type atomicRuntime struct {
	db      *store.DB
	sources *storecontent.Sources
	adapter *atomic.Adapter
	runner  *content.Runner
}

func newAtomicRuntime(t *testing.T) *atomicRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	adapter := atomic.New()

	if _, err := sources.SetEnabled(context.Background(), storecontent.SourceIDAtomic, true); err != nil {
		t.Fatalf("enable atomic: %v", err)
	}

	r, err := content.NewRunner(content.RunnerDeps{
		DB:         db,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   events.New(activity.New(db)),
		Adapters:   map[storecontent.Kind]content.Adapter{storecontent.KindAtomic: adapter},
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

	return &atomicRuntime{
		db:      db,
		sources: sources,
		adapter: adapter,
		runner:  r,
	}
}

func mustSync(t *testing.T, rt *atomicRuntime) storecontent.Job {
	t.Helper()
	job, err := rt.runner.StartSync(context.Background(), authn.Subject{UserID: "admin"}, content.StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
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
