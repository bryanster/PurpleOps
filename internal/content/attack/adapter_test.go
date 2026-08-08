package attack_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestParseNormalizeFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := attack.New()
	raw := readTestdata(t, "enterprise-mini-15.1.json")

	ast, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: "15.1"})
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
	cat, ok := objs[0].(*attack.Catalog)
	if !ok {
		t.Fatalf("object type %T", objs[0])
	}
	if cat.Version != "15.1" {
		t.Fatalf("version = %q", cat.Version)
	}
	if len(cat.Tactics) != 2 {
		t.Fatalf("tactics = %d, want 2", len(cat.Tactics))
	}
	if len(cat.Techniques) != 2 {
		t.Fatalf("techniques = %d, want 2", len(cat.Techniques))
	}
	var sub *attack.Technique
	for i := range cat.Techniques {
		if cat.Techniques[i].ExternalID == "T1059.001" {
			sub = &cat.Techniques[i]
		}
	}
	if sub == nil {
		t.Fatal("missing T1059.001")
	}
	if !sub.IsSubtechnique || sub.ParentExternalID != "T1059" {
		t.Fatalf("subtechnique parent = %q is_sub=%v", sub.ParentExternalID, sub.IsSubtechnique)
	}
	if got := cat.TechTactics["T1059"]; len(got) != 1 || got[0] != "TA0002" {
		t.Fatalf("T1059 tactics = %v, want [TA0002]", got)
	}
	if got := cat.TechMitigations["T1059"]; len(got) != 1 || got[0] != "M1049" {
		t.Fatalf("T1059 mitigations = %v, want [M1049]", got)
	}
	if len(cat.Groups) != 1 || cat.Groups[0].ExternalID != "G0016" {
		t.Fatalf("groups = %+v", cat.Groups)
	}
	if len(cat.Software) != 2 {
		t.Fatalf("software = %d, want 2", len(cat.Software))
	}
	if len(cat.DataSources) != 1 {
		t.Fatalf("data sources = %d", len(cat.DataSources))
	}
}

func TestParseBrokenFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := attack.New()
	raw := readTestdata(t, "enterprise-broken.json")
	ast, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: "15.1"})
	if err != nil {
		return
	}
	_, err = a.Normalize(ctx, ast)
	if err == nil {
		t.Fatal("Normalize broken fixture: want error")
	}
	if !strings.Contains(err.Error(), "external_id") && !strings.Contains(err.Error(), "empty technique") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVersionMismatchFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := attack.New()
	raw := readTestdata(t, "enterprise-mini-15.1.json")
	_, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: "14.1"})
	if err == nil || !strings.Contains(err.Error(), "version label mismatch") {
		t.Fatalf("want version mismatch, got %v", err)
	}
}

func TestDiscoverLatestFromIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := attack.New()
	a.IndexBytes = readTestdata(t, "index.json")
	a.FetchBytes = readTestdata(t, "enterprise-mini-15.1.json")

	b, err := a.Fetch(ctx, content.FetchRequest{
		Source: content.SourceInfo{
			URL: "https://example.test/base",
			Ref: "enterprise-attack/enterprise-attack-{version}.json",
		},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if b.Version != "15.1" {
		t.Fatalf("version = %q, want 15.1 from index", b.Version)
	}
}

func TestSyncVersionIsolationAndResync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newAttackRuntime(t)

	rt.adapter.FetchBytes = readTestdata(t, "enterprise-mini-14.1.json")
	job := mustSync(t, rt, "14.1")
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("14.1 status=%s err=%q", job.Status, job.Error)
	}

	rt.adapter.FetchBytes = readTestdata(t, "enterprise-mini-15.1.json")
	job = mustSync(t, rt, "15.1")
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("15.1 status=%s err=%q", job.Status, job.Error)
	}

	objs := storecontent.NewObjects(rt.db)

	list14, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "14.1"})
	if err != nil {
		t.Fatalf("list 14.1: %v", err)
	}
	if len(list14) != 2 {
		t.Fatalf("14.1 techniques = %d, want 2", len(list14))
	}
	for _, tech := range list14 {
		if tech.Version != "14.1" {
			t.Fatalf("14.1 list leaked version %q", tech.Version)
		}
		if tech.ExternalID == "T1059.001" {
			t.Fatal("14.1 list contains 15.1-only subtechnique")
		}
	}

	list15, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1"})
	if err != nil {
		t.Fatalf("list 15.1: %v", err)
	}
	if len(list15) != 2 {
		t.Fatalf("15.1 techniques = %d, want 2", len(list15))
	}

	var desc14, desc15 string
	for _, tech := range list14 {
		if tech.ExternalID == "T1059" {
			desc14 = tech.Description
		}
	}
	for _, tech := range list15 {
		if tech.ExternalID == "T1059" {
			desc15 = tech.Description
		}
	}
	if desc14 == "" || desc15 == "" || desc14 == desc15 {
		t.Fatalf("T1059 descriptions not isolated: 14=%q 15=%q", desc14, desc15)
	}

	edited := strings.Replace(
		string(readTestdata(t, "enterprise-mini-14.1.json")),
		"v14.1 description of T1059 — distinct from 15.1.",
		"v14.1 REVISED description",
		1,
	)
	rt.adapter.FetchBytes = []byte(edited)
	job = mustSync(t, rt, "14.1")
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("re-sync 14.1: %s %q", job.Status, job.Error)
	}

	list14, err = objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "14.1", Q: "T1059"})
	if err != nil {
		t.Fatalf("list after resync: %v", err)
	}
	found := false
	for _, tech := range list14 {
		if tech.ExternalID == "T1059" {
			found = true
			if !strings.Contains(tech.Description, "REVISED") {
				t.Fatalf("expected revised description, got %q", tech.Description)
			}
		}
	}
	if !found {
		t.Fatal("T1059 missing after resync")
	}

	list15, err = objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1", Q: "T1059"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tech := range list15 {
		if tech.ExternalID == "T1059" && strings.Contains(tech.Description, "REVISED") {
			t.Fatal("15.1 T1059 was mutated by 14.1 resync")
		}
	}

	rt.adapter.FetchBytes = readTestdata(t, "enterprise-broken.json")
	job = mustSync(t, rt, "15.1")
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("broken sync status=%s, want failed err=%q", job.Status, job.Error)
	}
	list15, err = objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list15) != 2 {
		t.Fatalf("after broken sync techniques = %d, want prior 2", len(list15))
	}

	yes := true
	subs, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{
		Version:        "15.1",
		IsSubtechnique: &yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].ExternalID != "T1059.001" || subs[0].ParentExternalID != "T1059" {
		t.Fatalf("subtechnique filter = %+v", subs)
	}

	byID, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1", Q: "t1059"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byID) < 1 {
		t.Fatal("q=t1059 found nothing")
	}
	byName, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1", Q: "powershell"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byName) != 1 || byName[0].ExternalID != "T1059.001" {
		t.Fatalf("q=powershell = %+v", byName)
	}

	byTac, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{Version: "15.1", Tactic: "TA0002"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTac) != 2 {
		t.Fatalf("tactic filter = %d, want 2", len(byTac))
	}

	if _, err := rt.sources.SetEnabled(ctx, storecontent.SourceIDAttack, false); err != nil {
		t.Fatal(err)
	}
	hidden, err := objs.ListTechniques(ctx, storecontent.ObjectListFilter{
		Version:     "15.1",
		EnabledOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 0 {
		t.Fatalf("enabled-only list = %d, want 0", len(hidden))
	}
}

func TestNoNetworkWhenFetchBytesSet(t *testing.T) {
	t.Parallel()
	a := attack.New()
	a.FetchBytes = readTestdata(t, "enterprise-mini-15.1.json")
	_, err := a.Fetch(context.Background(), content.FetchRequest{
		Source:  content.SourceInfo{URL: "https://should-not-dial.example"},
		Version: "15.1",
		HTTP:    panicHTTP{},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
}

type panicHTTP struct{}

func (panicHTTP) Do(*http.Request) (*http.Response, error) {
	panic("network I/O in test")
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("testdata", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

type attackRuntime struct {
	db       *store.DB
	sources  *storecontent.Sources
	versions *storecontent.Versions
	objects  *storecontent.Objects
	adapter  *attack.Adapter
	runner   *content.Runner
}

func newAttackRuntime(t *testing.T) *attackRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	adapter := attack.New()

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

	return &attackRuntime{
		db:       db,
		sources:  sources,
		versions: versions,
		objects:  storecontent.NewObjects(db),
		adapter:  adapter,
		runner:   r,
	}
}

func mustSync(t *testing.T, rt *attackRuntime, version string) storecontent.Job {
	t.Helper()
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
	return job
}
