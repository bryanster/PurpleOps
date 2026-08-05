package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/sigma"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const contentDetectionRulesPath = BasePath + "/content/detection-rules"

func TestLibraryDetectionRulesFiltersAndDisabled(t *testing.T) {
	t.Parallel()

	fixture := mustReadSigmaFixture(t, "rules-mini.zip")
	adapter := sigma.New()
	adapter.FetchBytes = fixture

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindSigma: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/sync", `{}`, admin)
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
	if !strings.Contains(finished.Message, "skipped") {
		t.Fatalf("job message missing skip count: %q", finished.Message)
	}

	rec = server.get(contentDetectionRulesPath+"?technique=T1059.001", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list technique: %d %s", rec.Code, rec.Body)
	}
	var list gen.ContentDetectionRuleList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("technique filter = %d, want 1", len(list.Items))
	}
	rule := list.Items[0]
	if rule.RuleYaml == "" || !strings.Contains(rule.RuleYaml, "detection:") {
		t.Fatalf("ruleYaml incomplete: %+v", rule)
	}
	if rule.Level != "high" {
		t.Fatalf("level = %q", rule.Level)
	}
	if len(rule.TechniqueExternalIds) != 2 {
		t.Fatalf("techniques = %v", rule.TechniqueExternalIds)
	}

	rec = server.get(contentDetectionRulesPath+"?technique=T1059", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list parent technique: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("technique=T1059 = %d, want 2", len(list.Items))
	}

	rec = server.get(contentDetectionRulesPath+"?level=medium", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list level: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("level=medium = %d, want 1", len(list.Items))
	}

	rec = server.get(contentDetectionRulesPath+"?q=powershell", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list q: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 1 {
		t.Fatal("q=powershell found nothing")
	}

	ruleID := rule.Id.String()
	rec = server.get(contentDetectionRulesPath+"/"+ruleID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	var detail gen.ContentDetectionRule
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.RuleYaml == "" || detail.Name == "" {
		t.Fatalf("detail incomplete: %+v", detail)
	}
	if detail.Logsource["product"] != "windows" {
		t.Fatalf("logsource = %v", detail.Logsource)
	}

	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "sigma-member@example.com"
		in.DisplayName = "Sigma Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	rec = server.get(contentDetectionRulesPath, memberSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d %s", rec.Code, rec.Body)
	}

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/disable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	rec = server.get(contentDetectionRulesPath, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list disabled: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("disabled source leaked %d rules", len(list.Items))
	}
	rec = server.get(contentDetectionRulesPath+"/"+ruleID, admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get disabled: %d, want 404", rec.Code)
	}
}

func mustReadSigmaFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "content", "sigma", "testdata", name),
		filepath.Join("internal", "content", "sigma", "testdata", name),
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
