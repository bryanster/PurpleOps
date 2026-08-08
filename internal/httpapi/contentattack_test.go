package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const contentAttackVersionsPath = BasePath + "/content/attack/versions"

func TestAttackPinSurfaceHTTP(t *testing.T) {
	t.Parallel()

	adapter := attack.New()
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

	syncAttack(t, server, admin, adapter, "14.1", "enterprise-mini-14.1.json")
	syncAttack(t, server, admin, adapter, "15.1", "enterprise-mini-15.1.json")

	// List
	rec := server.get(contentAttackVersionsPath, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var list gen.ContentAttackVersionList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("list items = %d", len(list.Items))
	}

	// Detail + counts
	rec = server.get(contentAttackVersionsPath+"/15.1", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body)
	}
	var detail gen.ContentAttackVersionDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Version != "15.1" || detail.Counts.Techniques != 2 {
		t.Fatalf("detail = %+v", detail)
	}

	// Natural-key resolve
	rec = server.get(contentAttackVersionsPath+"/15.1/techniques/T1059.001", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve 15.1: %d %s", rec.Code, rec.Body)
	}
	var tech gen.ContentTechnique
	if err := json.Unmarshal(rec.Body.Bytes(), &tech); err != nil {
		t.Fatal(err)
	}
	if tech.ExternalId != "T1059.001" || tech.Version != "15.1" {
		t.Fatalf("tech = %+v", tech)
	}

	// Cross-version not found
	rec = server.get(contentAttackVersionsPath+"/14.1/techniques/T1059.001", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-version: %d, want 404", rec.Code)
	}

	// v-prefix is a different pin string → not found
	rec = server.get(contentAttackVersionsPath+"/v15.1", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("v15.1: %d, want 404", rec.Code)
	}

	// Member can read, not delete
	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "pin-member@example.com"
		in.DisplayName = "Pin Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	rec = server.get(contentAttackVersionsPath, memberSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d", rec.Code)
	}
	rec = server.send(http.MethodDelete, contentAttackVersionsPath+"/14.1", "", memberSession)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member delete: %d, want 403", rec.Code)
	}

	// Admin delete 14.1 leaves 15.1
	rec = server.send(http.MethodDelete, contentAttackVersionsPath+"/14.1", "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 14.1: %d %s", rec.Code, rec.Body)
	}
	rec = server.get(contentAttackVersionsPath+"/14.1", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("14.1 after delete: %d", rec.Code)
	}
	rec = server.get(contentAttackVersionsPath+"/15.1/techniques/T1059.001", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("15.1 after delete 14.1: %d %s", rec.Code, rec.Body)
	}

	entries, _, err := activity.New(server.db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true,
		Verb:          string(events.VerbContentVersionDeleted),
		Limit:         5,
	})
	if err != nil {
		t.Fatalf("activity: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected content.version.deleted")
	}
}
func syncAttack(t *testing.T, server *authServer, admin *http.Cookie, adapter *attack.Adapter, version, fixture string) {
	t.Helper()
	adapter.FetchBytes = mustReadAttackFixture(t, fixture)
	rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/sync", `{"version":"`+version+`"}`, admin)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("sync %s: %d %s", version, rec.Code, rec.Body)
	}
	var job gen.ContentSyncJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	finished := waitContentJob(t, server, admin, job.Id.String())
	if finished.Status != gen.ContentSyncJobStatusSucceeded {
		t.Fatalf("sync %s: %s %q", version, finished.Status, finished.Error)
	}
}
