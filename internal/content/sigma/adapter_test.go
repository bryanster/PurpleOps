package sigma_test

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
	"github.com/bryanster/blacklight/internal/content/sigma"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestTechniqueFromTagNormalization(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := sigma.New()

	// Drive through Parse/Normalize with a tiny bare rule per case via tags
	// embedded in YAML is heavy; unit-test the exported path by normalizing
	// fixtures that cover the matrix.
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "parent and subtechnique mixed case",
			raw: `title: Mixed
id: mix-1
tags:
  - attack.t1059
  - ATTACK.T1059.001
  - attack.execution
logsource: {}
detection:
  condition: selection
level: high
`,
			want: []string{"T1059", "T1059.001"},
		},
		{
			name: "subtechnique only",
			raw: `title: Sub
id: sub-1
tags:
  - attack.t1059.001
logsource: {}
detection:
  condition: selection
level: medium
`,
			want: []string{"T1059.001"},
		},
		{
			name: "unmapped tactic only",
			raw: `title: Unmapped
id: un-1
tags:
  - attack.persistence
  - attack.defense_evasion
logsource: {}
detection:
  condition: selection
level: low
`,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ast, err := a.Parse(ctx, content.Bundle{Bytes: []byte(tc.raw)})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			objs, err := a.Normalize(ctx, ast)
			if err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			cat, ok := objs[0].(*sigma.Catalog)
			if !ok {
				t.Fatalf("object type %T", objs[0])
			}
			if tc.want == nil {
				if len(cat.Rules) != 0 {
					t.Fatalf("rules = %d, want 0 skipped", len(cat.Rules))
				}
				if cat.Skipped != 1 {
					t.Fatalf("skipped = %d, want 1", cat.Skipped)
				}
				return
			}
			if len(cat.Rules) != 1 {
				t.Fatalf("rules = %d, want 1", len(cat.Rules))
			}
			got := cat.Rules[0].TechniqueExternalIDs
			if len(got) != len(tc.want) {
				t.Fatalf("techniques = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("techniques = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestParseNormalizeFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := sigma.New()
	raw := readTestdata(t, "rules-mini.zip")

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
	cat, ok := objs[0].(*sigma.Catalog)
	if !ok {
		t.Fatalf("object type %T", objs[0])
	}
	// 2 mapped + 1 unmapped skipped
	if len(cat.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(cat.Rules))
	}
	if cat.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", cat.Skipped)
	}
	if cat.ItemCount() != 2 {
		t.Fatalf("ItemCount = %d", cat.ItemCount())
	}
	msg := cat.SuccessMessage()
	if !strings.Contains(msg, "skipped 1") {
		t.Fatalf("SuccessMessage = %q, want skip count", msg)
	}

	byExt := map[string]sigma.Rule{}
	for _, r := range cat.Rules {
		byExt[r.ExternalID] = r
	}
	ps, ok := byExt["5d2c185a-6d5a-4f3e-9c1a-0b7e6f4d2a11"]
	if !ok {
		t.Fatalf("missing powershell rule; have %v", keys(byExt))
	}
	if ps.Level != "high" || ps.Status != "test" {
		t.Fatalf("level/status = %q/%q", ps.Level, ps.Status)
	}
	if len(ps.TechniqueExternalIDs) != 2 {
		t.Fatalf("techniques = %v", ps.TechniqueExternalIDs)
	}
	if ps.TechniqueExternalIDs[0] != "T1059" || ps.TechniqueExternalIDs[1] != "T1059.001" {
		t.Fatalf("techniques = %v", ps.TechniqueExternalIDs)
	}
	if ps.RuleYAML == "" || !strings.Contains(ps.RuleYAML, "Encoded PowerShell") {
		t.Fatalf("rule_yaml missing body: %q", ps.RuleYAML)
	}
	var ls map[string]any
	if err := json.Unmarshal(ps.Logsource, &ls); err != nil {
		t.Fatalf("logsource json: %v", err)
	}
	if ls["product"] != "windows" {
		t.Fatalf("logsource = %v", ls)
	}

	bash, ok := byExt["a1b2c3d4-e5f6-7890-abcd-ef1234567890"]
	if !ok {
		t.Fatal("missing bash rule")
	}
	// ATTACK.T1059 + attack.t1059.004 → T1059, T1059.004
	if len(bash.TechniqueExternalIDs) != 2 {
		t.Fatalf("bash techniques = %v", bash.TechniqueExternalIDs)
	}
}

func TestParseBrokenFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := sigma.New()
	raw := readTestdata(t, "broken.yml")
	_, err := a.Parse(ctx, content.Bundle{Bytes: raw, Version: storecontent.VersionCurrent})
	if err == nil {
		t.Fatal("Parse broken fixture: want error")
	}
}

func TestBareYAMLFixture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := sigma.New()
	raw := readTestdata(t, "mapped.yml")
	ast, err := a.Parse(ctx, content.Bundle{Bytes: raw})
	if err != nil {
		t.Fatalf("Parse bare yaml: %v", err)
	}
	objs, err := a.Normalize(ctx, ast)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	cat, ok := objs[0].(*sigma.Catalog)
	if !ok {
		t.Fatalf("object type %T", objs[0])
	}
	if len(cat.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(cat.Rules))
	}
	if cat.Rules[0].ExternalID != "bare-mapped-0001" {
		t.Fatalf("external_id = %q", cat.Rules[0].ExternalID)
	}
	if len(cat.Rules[0].TechniqueExternalIDs) != 1 || cat.Rules[0].TechniqueExternalIDs[0] != "T1003.001" {
		t.Fatalf("techniques = %v", cat.Rules[0].TechniqueExternalIDs)
	}
}

func TestSyncReplaceFiltersAndSkips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newSigmaRuntime(t)

	rt.adapter.FetchBytes = readTestdata(t, "rules-mini.zip")
	job := mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("sync status=%s err=%q", job.Status, job.Error)
	}
	if !strings.Contains(job.Message, "skipped") {
		t.Fatalf("job message %q missing skip count", job.Message)
	}

	dets := storecontent.NewDetections(rt.db)
	all, err := dets.List(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("list all = %d, want 2 (unmapped skipped)", len(all))
	}

	// Technique filter finds by parent and subtechnique.
	byParent, err := dets.List(ctx, storecontent.DetectionListFilter{Technique: "T1059"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byParent) != 2 {
		t.Fatalf("technique=T1059 = %d, want 2", len(byParent))
	}
	bySub, err := dets.List(ctx, storecontent.DetectionListFilter{Technique: "T1059.001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bySub) != 1 {
		t.Fatalf("technique=T1059.001 = %d, want 1", len(bySub))
	}
	byLevel, err := dets.List(ctx, storecontent.DetectionListFilter{Level: "HIGH"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byLevel) != 1 {
		t.Fatalf("level=HIGH = %d, want 1", len(byLevel))
	}

	// Rule body sufficient to display/copy.
	var body *storecontent.DetectionRuleRef
	for i := range all {
		if all[i].ExternalID == "5d2c185a-6d5a-4f3e-9c1a-0b7e6f4d2a11" {
			body = &all[i]
			break
		}
	}
	if body == nil || body.RuleYAML == "" || !strings.Contains(body.RuleYAML, "detection:") {
		t.Fatalf("rule_yaml incomplete: %+v", body)
	}

	// Re-sync with bare mapped YAML replaces catalog.
	rt.adapter.FetchBytes = readTestdata(t, "mapped.yml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("re-sync status=%s err=%q", job.Status, job.Error)
	}
	all, err = dets.List(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after re-sync count = %d, want 1", len(all))
	}
	if all[0].ExternalID != "bare-mapped-0001" {
		t.Fatalf("external_id = %q", all[0].ExternalID)
	}
	if all[0].Version != storecontent.VersionCurrent {
		t.Fatalf("version = %q", all[0].Version)
	}

	// Broken fixture fails and leaves prior catalog intact.
	rt.adapter.FetchBytes = readTestdata(t, "broken.yml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusFailed {
		t.Fatalf("broken sync status=%s, want failed err=%q", job.Status, job.Error)
	}
	all, err = dets.List(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after broken sync count = %d, want prior 1", len(all))
	}

	// All-unmapped archive succeeds with zero rows and skip count.
	rt.adapter.FetchBytes = []byte(`title: Only Unmapped
id: only-unmapped
tags:
  - attack.persistence
logsource: {}
detection:
  condition: selection
level: low
`)
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("unmapped-only status=%s err=%q", job.Status, job.Error)
	}
	if !strings.Contains(job.Message, "skipped") {
		t.Fatalf("unmapped-only message %q", job.Message)
	}
	all, err = dets.List(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("unmapped-only left %d rows", len(all))
	}

	// Restore mapped catalog then hide via disable.
	rt.adapter.FetchBytes = readTestdata(t, "mapped.yml")
	job = mustSync(t, rt)
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("restore status=%s err=%q", job.Status, job.Error)
	}
	all, err = dets.List(ctx, storecontent.DetectionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("restore count = %d", len(all))
	}
	if _, err := rt.sources.SetEnabled(ctx, storecontent.SourceIDSigma, false); err != nil {
		t.Fatal(err)
	}
	hidden, err := dets.List(ctx, storecontent.DetectionListFilter{EnabledOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 0 {
		t.Fatalf("enabled-only list = %d, want 0", len(hidden))
	}
	if _, err := dets.ByIDEnabled(ctx, all[0].ID, true); err == nil {
		t.Fatal("ByIDEnabled on disabled source: want error")
	}
}

func TestNoNetworkWhenFetchBytesSet(t *testing.T) {
	t.Parallel()
	a := sigma.New()
	a.FetchBytes = readTestdata(t, "rules-mini.zip")
	_, err := a.Fetch(context.Background(), content.FetchRequest{
		Source: content.SourceInfo{URL: "https://should-not-dial.example"},
		HTTP:   panicHTTP{},
	})
	if err != nil {
		t.Fatalf("Fetch with FetchBytes: %v", err)
	}
}

func TestPackageHasNoExecutionAPI(t *testing.T) {
	t.Parallel()
	// Architectural guard: this package must not grow an execution surface.
	// Grep the package source for forbidden identifiers.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"Execute(", "RunRule(", "MatchEvents(", "Deploy("}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		src := string(body)
		for _, f := range forbidden {
			if strings.Contains(src, f) {
				t.Fatalf("%s contains forbidden execution API %q", e.Name(), f)
			}
		}
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

func keys(m map[string]sigma.Rule) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type sigmaRuntime struct {
	db      *store.DB
	sources *storecontent.Sources
	adapter *sigma.Adapter
	runner  *content.Runner
}

func newSigmaRuntime(t *testing.T) *sigmaRuntime {
	t.Helper()
	db := storetest.Migrated(t)
	dir := t.TempDir()
	paths := storecontent.NewPaths(dir)
	sources := storecontent.NewSources(db)
	versions := storecontent.NewVersions(db, paths)
	jobs := storecontent.NewJobs(db)
	adapter := sigma.New()

	if _, err := sources.SetEnabled(context.Background(), storecontent.SourceIDSigma, true); err != nil {
		t.Fatalf("enable sigma: %v", err)
	}

	r, err := content.NewRunner(content.RunnerDeps{
		DB:         db,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   events.New(activity.New(db)),
		Adapters:   map[storecontent.Kind]content.Adapter{storecontent.KindSigma: adapter},
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

	return &sigmaRuntime{
		db:      db,
		sources: sources,
		adapter: adapter,
		runner:  r,
	}
}

func mustSync(t *testing.T, rt *sigmaRuntime) storecontent.Job {
	t.Helper()
	job, err := rt.runner.StartSync(context.Background(), authn.Subject{UserID: "admin"}, content.StartSyncRequest{
		SourceID: storecontent.SourceIDSigma,
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
