package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/atomic"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const contentProcedureTemplatesPath = BasePath + "/content/procedure-templates"

func TestLibraryProcedureTemplatesFiltersAndDisabled(t *testing.T) {
	t.Parallel()

	fixture := mustReadAtomicFixture(t, "atomics-mini.zip")
	adapter := atomic.New()
	adapter.FetchBytes = fixture

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAtomic)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAtomic)+"/sync", `{}`, admin)
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

	rec = server.get(contentProcedureTemplatesPath+"?technique=T1059.001", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list technique: %d %s", rec.Code, rec.Body)
	}
	var list gen.ContentProcedureTemplateList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("technique filter = %d, want 2", len(list.Items))
	}

	// Structure preserved on the wire.
	var withArgs gen.ContentProcedureTemplate
	foundArgs := false
	for i := range list.Items {
		if list.Items[i].ExternalId == "11111111-1111-4111-8111-111111111111" {
			withArgs = list.Items[i]
			foundArgs = true
			break
		}
	}
	if !foundArgs {
		t.Fatal("missing guid template in list")
	}
	if withArgs.Command == "" || withArgs.Cleanup == "" {
		t.Fatalf("command/cleanup missing: %+v", withArgs)
	}
	if withArgs.Command == withArgs.Cleanup {
		t.Fatal("command and cleanup must stay distinct")
	}
	if len(withArgs.InputArgs) != 2 {
		t.Fatalf("inputArgs = %d, want 2", len(withArgs.InputArgs))
	}
	if len(withArgs.Platforms) != 1 || withArgs.Platforms[0] != "windows" {
		t.Fatalf("platforms = %v", withArgs.Platforms)
	}
	rec = server.get(contentProcedureTemplatesPath+"?platform=linux", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list platform: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("platform=linux = %d, want 2", len(list.Items))
	}

	rec = server.get(contentProcedureTemplatesPath+"?q=bash", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list q: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 1 {
		t.Fatal("q=bash found nothing")
	}

	tmplID := withArgs.Id.String()
	rec = server.get(contentProcedureTemplatesPath+"/"+tmplID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	var detail gen.ContentProcedureTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Executor != "powershell" || len(detail.InputArgs) != 2 {
		t.Fatalf("detail = %+v", detail)
	}

	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "atomic-member@example.com"
		in.DisplayName = "Atomic Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	rec = server.get(contentProcedureTemplatesPath, memberSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d %s", rec.Code, rec.Body)
	}

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAtomic)+"/disable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	rec = server.get(contentProcedureTemplatesPath, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list disabled: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("disabled source leaked %d templates", len(list.Items))
	}
	rec = server.get(contentProcedureTemplatesPath+"/"+tmplID, admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get disabled: %d, want 404", rec.Code)
	}
}

func mustReadAtomicFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "content", "atomic", "testdata", name),
		filepath.Join("internal", "content", "atomic", "testdata", name),
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
