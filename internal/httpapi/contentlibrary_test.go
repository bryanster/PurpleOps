package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const contentTechniquesPath = BasePath + "/content/techniques"

func TestLibraryTechniquesFiltersAndDisabled(t *testing.T) {
	t.Parallel()

	fixture := mustReadAttackFixture(t, "enterprise-mini-15.1.json")
	adapter := attack.New()
	adapter.FetchBytes = fixture

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAttack: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/sync", `{"version":"15.1"}`, admin)
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

	rec = server.get(contentTechniquesPath+"?version=15.1&q=t1059", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list q: %d %s", rec.Code, rec.Body)
	}
	var list gen.ContentTechniqueList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 1 {
		t.Fatal("expected techniques for q=t1059")
	}

	rec = server.get(contentTechniquesPath+"?version=15.1&isSubtechnique=true", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list sub: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ExternalId != "T1059.001" {
		t.Fatalf("subtechnique list = %+v", list.Items)
	}
	techID := list.Items[0].Id.String()

	rec = server.get(contentTechniquesPath+"/"+techID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	var detail gen.ContentTechniqueDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ParentExternalId != "T1059" || !detail.IsSubtechnique {
		t.Fatalf("detail = %+v", detail)
	}
	if len(detail.Tactics) != 1 || detail.Tactics[0] != "TA0002" {
		t.Fatalf("tactics = %v", detail.Tactics)
	}

	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "lib-member@example.com"
		in.DisplayName = "Lib Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	rec = server.get(contentTechniquesPath+"?version=15.1", memberSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d %s", rec.Code, rec.Body)
	}

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/disable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	rec = server.get(contentTechniquesPath+"?version=15.1", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list disabled: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("disabled source leaked %d techniques", len(list.Items))
	}
	rec = server.get(contentTechniquesPath+"/"+techID, admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get disabled: %d, want 404", rec.Code)
	}
}

func mustReadAttackFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "content", "attack", "testdata", name),
		filepath.Join("internal", "content", "attack", "testdata", name),
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
