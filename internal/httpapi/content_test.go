package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Content source registry over HTTP (M2-002).

const contentSourcesPath = BasePath + "/content/sources"

func contentSourcePath(id string) string { return contentSourcesPath + "/" + id }

func contentEndpoints() []struct {
	name   string
	method string
	path   func(id string) string
	body   string
	// manage is true for mutations that require content.manage.
	manage bool
} {
	return []struct {
		name   string
		method string
		path   func(id string) string
		body   string
		manage bool
	}{
		{"list sources", http.MethodGet, func(string) string { return contentSourcesPath }, "", false},
		{"read one source", http.MethodGet, contentSourcePath, "", false},
		{"list versions", http.MethodGet, func(id string) string { return contentSourcePath(id) + "/versions" }, "", false},
		{"rename a source", http.MethodPatch, contentSourcePath, `{"name":"Renamed"}`, true},
		{"enable a source", http.MethodPost, func(id string) string { return contentSourcePath(id) + "/enable" }, "", true},
		{"disable a source", http.MethodPost, func(id string) string { return contentSourcePath(id) + "/disable" }, "", true},
		// delete is tested separately against a disposable source — the seed
		// custom source must not be removed by the matrix.
	}
}

func TestMemberCanReadContentSourcesButNotManage(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // admin
	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "member@example.com"
		in.DisplayName = "Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	id := storecontent.SourceIDAttack

	for _, endpoint := range contentEndpoints() {
		target := endpoint.path(id)
		var recorder *httptest.ResponseRecorder
		if endpoint.method == http.MethodGet {
			recorder = server.get(target, memberSession)
		} else {
			recorder = server.send(endpoint.method, target, endpoint.body, memberSession)
		}
		if endpoint.manage {
			if recorder.Code != http.StatusForbidden {
				t.Errorf("member tried to %s: %d, want 403\nbody: %s",
					endpoint.name, recorder.Code, recorder.Body)
			}
			if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeForbidden {
				t.Errorf("member tried to %s: problem %q", endpoint.name, got)
			}
			continue
		}
		if recorder.Code != http.StatusOK {
			t.Errorf("member tried to %s: %d, want 200\nbody: %s",
				endpoint.name, recorder.Code, recorder.Body)
		}
	}
}

func TestUnauthenticatedCannotReadContentSources(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	recorder := server.get(contentSourcesPath)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
}

func TestAdminEnableDisableRoundTrip(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)
	id := storecontent.SourceIDAttack

	// Seed is disabled. Enable.
	enable := server.send(http.MethodPost, contentSourcePath(id)+"/enable", "", admin)
	if enable.Code != http.StatusOK {
		t.Fatalf("enable: %d\nbody: %s", enable.Code, enable.Body)
	}
	var enabled gen.ContentSource
	if err := json.Unmarshal(enable.Body.Bytes(), &enabled); err != nil {
		t.Fatal(err)
	}
	if !enabled.Enabled {
		t.Fatal("enable left source disabled")
	}

	// Disable.
	disable := server.send(http.MethodPost, contentSourcePath(id)+"/disable", "", admin)
	if disable.Code != http.StatusOK {
		t.Fatalf("disable: %d\nbody: %s", disable.Code, disable.Body)
	}
	var disabled gen.ContentSource
	if err := json.Unmarshal(disable.Body.Bytes(), &disabled); err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatal("disable left source enabled")
	}

	// Activity verbs landed.
	rows, _, err := activity.New(server.db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true,
		ObjectType:    "content_source",
		ObjectID:      id,
		Limit:         10,
	})
	if err != nil {
		t.Fatal(err)
	}
	verbs := map[string]bool{}
	for _, row := range rows {
		verbs[row.Verb] = true
	}
	if !verbs["content.source.enabled"] || !verbs["content.source.disabled"] {
		t.Fatalf("activity verbs = %v", verbs)
	}
}

func TestAdminCanPatchSourceMetadata(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)
	id := storecontent.SourceIDAtomic

	recorder := server.send(http.MethodPatch, contentSourcePath(id),
		`{"name":"Atomic mirror","url":"https://example.invalid/atomic","ref":"main"}`, admin)
	if recorder.Code != http.StatusOK {
		t.Fatalf("patch: %d\nbody: %s", recorder.Code, recorder.Body)
	}
	var src gen.ContentSource
	if err := json.Unmarshal(recorder.Body.Bytes(), &src); err != nil {
		t.Fatal(err)
	}
	if src.Name != "Atomic mirror" || src.Url != "https://example.invalid/atomic" || src.Ref != "main" {
		t.Fatalf("unexpected patch result: %+v", src)
	}
	if src.Kind != gen.ContentSourceKindAtomic {
		t.Fatalf("kind changed to %q", src.Kind)
	}
}

func TestDeleteCustomSourceIsConflict(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	recorder := server.send(http.MethodDelete, contentSourcePath(storecontent.SourceIDCustom), "", admin)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("delete custom: %d, want 409\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeConflict {
		t.Fatalf("problem code %q", got)
	}

	// Still there.
	get := server.get(contentSourcePath(storecontent.SourceIDCustom), admin)
	if get.Code != http.StatusOK {
		t.Fatalf("custom source gone after refused delete: %d", get.Code)
	}
}

func TestDeleteUpstreamSourceRemovesSubtree(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	// Plant a version + technique under ATT&CK, then delete the source.
	db := server.db
	versions := storecontent.NewVersions(db, storecontent.Paths{})
	v, err := versions.Create(t.Context(), storecontent.NewSourceVersion{
		SourceID: storecontent.SourceIDAttack,
		Version:  "15.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	objects := storecontent.NewObjects(db)
	if _, err := objects.CreateTechnique(t.Context(), storecontent.Technique{
		SourceID:   storecontent.SourceIDAttack,
		Version:    "15.1",
		ExternalID: "T1059",
		Name:       "Command and Scripting Interpreter",
	}); err != nil {
		t.Fatal(err)
	}

	recorder := server.send(http.MethodDelete, contentSourcePath(storecontent.SourceIDAttack), "", admin)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete attack: %d, want 204\nbody: %s", recorder.Code, recorder.Body)
	}

	// Source gone.
	get := server.get(contentSourcePath(storecontent.SourceIDAttack), admin)
	if get.Code != http.StatusNotFound {
		t.Fatalf("source still readable: %d", get.Code)
	}

	// Version gone.
	if _, err := versions.ByID(t.Context(), v.ID); err == nil {
		t.Fatal("version row survived cascade delete")
	}

	// No orphan techniques.
	var n int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT count(*) FROM content.content_technique WHERE source_id = ?`,
		storecontent.SourceIDAttack,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d orphan techniques remain", n)
	}

	// Activity.
	rows, _, err := activity.New(db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true,
		Verb:          "content.source.deleted",
		ObjectID:      storecontent.SourceIDAttack,
		Limit:         5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("deleted activity rows = %d", len(rows))
	}
}

func TestListContentSourcesFilters(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	// Enable custom is already true; enable attack so the enabled filter has more than one.
	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d", rec.Code)
	}

	onlyEnabled := server.get(contentSourcesPath+"?enabled=true", admin)
	if onlyEnabled.Code != http.StatusOK {
		t.Fatalf("enabled=true: %d\nbody: %s", onlyEnabled.Code, onlyEnabled.Body)
	}
	var enabledList gen.ContentSourceList
	if err := json.Unmarshal(onlyEnabled.Body.Bytes(), &enabledList); err != nil {
		t.Fatal(err)
	}
	for _, s := range enabledList.Items {
		if !s.Enabled {
			t.Fatalf("enabled=true returned disabled source %s", s.Id)
		}
	}

	onlyAttack := server.get(contentSourcesPath+"?kind=attack", admin)
	if onlyAttack.Code != http.StatusOK {
		t.Fatalf("kind=attack: %d", onlyAttack.Code)
	}
	var attackList gen.ContentSourceList
	if err := json.Unmarshal(onlyAttack.Body.Bytes(), &attackList); err != nil {
		t.Fatal(err)
	}
	if len(attackList.Items) != 1 || attackList.Items[0].Kind != gen.ContentSourceKindAttack {
		t.Fatalf("kind filter: %+v", attackList.Items)
	}
}

func TestServiceTokenContentScopes(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	readTok := server.createToken(t, admin, authz.TokenScopeContentRead).Token
	writeTok := server.createToken(t, admin, authz.TokenScopeContentWrite).Token
	syncTok := server.createToken(t, admin, authz.TokenScopeContentSync).Token

	// content:read lists.
	if rec := server.withToken(http.MethodGet, contentSourcesPath, readTok); rec.Code != http.StatusOK {
		t.Fatalf("content:read list: %d", rec.Code)
	}
	// content:read cannot enable.
	if rec := server.withToken(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/enable", readTok); rec.Code != http.StatusForbidden {
		t.Fatalf("content:read enable: %d, want 403", rec.Code)
	}
	// content:sync cannot enable (distinct from manage).
	if rec := server.withToken(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/enable", syncTok); rec.Code != http.StatusForbidden {
		t.Fatalf("content:sync enable: %d, want 403", rec.Code)
	}
	// content:write can enable.
	if rec := server.withToken(http.MethodPost, contentSourcePath(storecontent.SourceIDSigma)+"/enable", writeTok); rec.Code != http.StatusOK {
		t.Fatalf("content:write enable: %d\nbody: %s", rec.Code, rec.Body)
	}
}
