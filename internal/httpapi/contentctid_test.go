package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/ctid"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const contentEmulationPlansPath = BasePath + "/content/emulation-plans"

func TestLibraryEmulationPlansDetailOrderAndDisabled(t *testing.T) {
	t.Parallel()

	fixture := mustReadCTIDFixture(t, "plans-mini.zip")
	adapter := ctid.New()
	adapter.FetchBytes = fixture

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindCTID: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDCTID)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDCTID)+"/sync", `{}`, admin)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body)
	}
	var job gen.ContentSyncJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	finished := waitContentJob(t, server, admin, job.Id.String())
	if finished.Status != gen.ContentSyncJobStatusSucceeded {
		t.Fatalf("sync job %s error=%q", finished.Status, finished.Error)
	}

	rec = server.get(contentEmulationPlansPath+"?technique=T1566.001", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list technique: %d %s", rec.Code, rec.Body)
	}
	var list gen.ContentEmulationPlanList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("technique filter = %d, want 1", len(list.Items))
	}
	plan := list.Items[0]
	if plan.AdversaryName != "Fixture Eagle" {
		t.Fatalf("adversary = %q", plan.AdversaryName)
	}
	if plan.ExternalId != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("externalId = %q", plan.ExternalId)
	}

	rec = server.get(contentEmulationPlansPath+"?q=eagle", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list q: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("q=eagle = %d", len(list.Items))
	}

	planID := plan.Id.String()
	rec = server.get(contentEmulationPlansPath+"/"+planID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	var detail gen.ContentEmulationPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(detail.Steps))
	}
	for i := range detail.Steps {
		if detail.Steps[i].Ordinal != i+1 {
			t.Fatalf("step[%d].ordinal = %d, want %d", i, detail.Steps[i].Ordinal, i+1)
		}
	}
	// Missing technique allowed as empty string.
	if detail.Steps[2].TechniqueExternalId != "" {
		t.Fatalf("step2 technique = %q", detail.Steps[2].TechniqueExternalId)
	}
	// Procedure payload present on a commanded step.
	if len(detail.Steps[1].Procedure) == 0 {
		t.Fatal("step1 procedure empty")
	}
	if execs, ok := detail.Steps[1].Procedure["executors"].([]any); !ok || len(execs) == 0 {
		t.Fatalf("step1 executors = %#v", detail.Steps[1].Procedure["executors"])
	}

	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "ctid-member@example.com"
		in.DisplayName = "CTID Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	rec = server.get(contentEmulationPlansPath, memberSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d %s", rec.Code, rec.Body)
	}

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDCTID)+"/disable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	rec = server.get(contentEmulationPlansPath, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list disabled: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("disabled source leaked %d plans", len(list.Items))
	}
	rec = server.get(contentEmulationPlansPath+"/"+planID, admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get disabled: %d, want 404", rec.Code)
	}
}

func mustReadCTIDFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "content", "ctid", "testdata", name),
		filepath.Join("internal", "content", "ctid", "testdata", name),
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
