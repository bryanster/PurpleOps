package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const customImportPath = BasePath + "/content/custom/import"

func v1ImportFixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	p := filepath.Join(filepath.Dir(file), "..", "content", "testdata", "v1import", name)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (s *authServer) postCustomImport(t *testing.T, target string, data []byte, filename, format string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if format != "" {
		if err := w.WriteField("format", format); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		s.attachCSRF(req, cookie)
	}
	return do(s.handler, req)
}

func TestImportCustomContentJSONAndDryRun(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	raw := v1ImportFixture(t, "testcases.json")
	rec := server.postCustomImport(t, customImportPath, raw, "testcases.json", "testcases_json", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d\n%s", rec.Code, rec.Body)
	}
	var report gen.ContentImportReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.ProceduresCreated != 2 {
		t.Fatalf("created=%d report=%+v", report.ProceduresCreated, report)
	}

	rec = server.postCustomImport(t, customImportPath+"?dryRun=true", raw, "testcases.json", "testcases_json", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run: %d\n%s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || report.ProceduresUpdated != 2 || report.ProceduresCreated != 0 {
		t.Fatalf("dry report=%+v", report)
	}

	// Library should still have exactly 2 after dry-run.
	list := server.get(BasePath+"/content/custom/procedure-templates", admin)
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d", list.Code)
	}
	var items gen.ContentProcedureTemplateList
	if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 2 {
		t.Fatalf("list len=%d", len(items.Items))
	}
}

func TestImportCustomContentMemberForbidden(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "member-import@example.com"
		in.DisplayName = "Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))

	raw := v1ImportFixture(t, "testcases.json")
	rec := server.postCustomImport(t, customImportPath, raw, "testcases.json", "auto", memberSession)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member import: %d, want 403\n%s", rec.Code, rec.Body)
	}
}

func TestImportCustomContentKB(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	raw := v1ImportFixture(t, "knowledgebase/T1003.yaml")
	rec := server.postCustomImport(t, customImportPath, raw, "T1003.yaml", "knowledgebase_yaml", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("kb import: %d\n%s", rec.Code, rec.Body)
	}
	var report gen.ContentImportReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.NotesCreated != 1 {
		t.Fatalf("notes created=%d report=%+v", report.NotesCreated, report)
	}
}
