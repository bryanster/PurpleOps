package httpapi

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestUploadBundleAndReprocessEndToEnd(t *testing.T) {
	t.Parallel()

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	fixture.FetchErr = fmt.Errorf("network disabled")

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	bundle := content.FixtureBundle(storecontent.VersionCurrent, []content.FixtureNote{
		{ExternalID: "http-1", Title: "HTTP One", Body: "b1"},
		{ExternalID: "http-2", Title: "HTTP Two", Body: "b2"},
	})
	rec := server.postMultipart(
		contentSourcePath(storecontent.SourceIDAtomic)+"/bundle",
		"file", "fixture.json", bundle, admin,
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("bundle upload = %d\n%s", rec.Code, rec.Body)
	}
	job := decodeJSON[gen.ContentSyncJob](t, rec)
	if job.Kind != gen.ContentSyncJobKindBundleImport {
		t.Fatalf("kind = %s, want bundle_import", job.Kind)
	}
	job = waitContentJob(t, server, admin, job.Id.String())
	if job.Status != gen.ContentSyncJobStatusSucceeded {
		t.Fatalf("bundle job status = %s err=%v", job.Status, job.Error)
	}

	// Break catalog then reprocess from raw (no network).
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`DELETE FROM content.content_note WHERE source_id = ?`,
			storecontent.SourceIDAtomic)
		return err
	}); err != nil {
		t.Fatalf("break catalog: %v", err)
	}

	rec = server.post(
		contentSourcePath(storecontent.SourceIDAtomic)+"/reprocess",
		`{}`,
		admin,
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("reprocess = %d\n%s", rec.Code, rec.Body)
	}
	job2 := decodeJSON[gen.ContentSyncJob](t, rec)
	if job2.Kind != gen.ContentSyncJobKindReprocess {
		t.Fatalf("kind = %s, want reprocess", job2.Kind)
	}
	job2 = waitContentJob(t, server, admin, job2.Id.String())
	if job2.Status != gen.ContentSyncJobStatusSucceeded {
		t.Fatalf("reprocess status = %s err=%v", job2.Status, job2.Error)
	}
}

func TestUploadBundleOversizedIsBadRequest(t *testing.T) {
	t.Parallel()

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
		}
		d.Config.Content.MaxBytes = 64
	})
	server.seedUser(t)
	admin := server.signIn(t)

	big := bytes.Repeat([]byte("x"), 128)
	rec := server.postMultipart(
		contentSourcePath(storecontent.SourceIDAtomic)+"/bundle",
		"file", "big.bin", big, admin,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized = %d, want 400\n%s", rec.Code, rec.Body)
	}
	problem := decodeProblem(t, rec)
	if !bundleProblemNamesLimit(problem, "64") {
		t.Fatalf("problem does not name the limit: detail=%v errors=%v", problem.Detail, problem.Errors)
	}
}

func TestReprocessWithoutRawIsConflict(t *testing.T) {
	t.Parallel()

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	rec := server.post(
		contentSourcePath(storecontent.SourceIDAtomic)+"/reprocess",
		`{}`,
		admin,
	)
	if rec.Code != http.StatusConflict {
		t.Fatalf("reprocess without raw = %d, want 409\n%s", rec.Code, rec.Body)
	}
	problem := decodeProblem(t, rec)
	if problem.Detail == nil || !strings.Contains(*problem.Detail, "no raw snapshot") {
		t.Fatalf("detail = %v, want no raw snapshot", problem.Detail)
	}
}

func TestMemberCannotUploadOrReprocess(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // admin default email
	server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "bundle-member@example.com"
		u.DisplayName = "Member"
		u.PlatformRole = authz.PlatformRoleMember
	})
	cookie := sessionCookie(t, server.login("bundle-member@example.com", testPassword))

	rec := server.postMultipart(
		contentSourcePath(storecontent.SourceIDAtomic)+"/bundle",
		"file", "f.json", []byte(`{}`), cookie,
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member bundle = %d, want 403\n%s", rec.Code, rec.Body)
	}

	rec = server.post(
		contentSourcePath(storecontent.SourceIDAtomic)+"/reprocess",
		`{}`,
		cookie,
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member reprocess = %d, want 403\n%s", rec.Code, rec.Body)
	}
}

func TestBundleWhileSyncActiveIsConflict(t *testing.T) {
	t.Parallel()

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	fixture.FetchBytes = content.FixtureBundle(storecontent.VersionCurrent, manyHTTPNotes(8))
	fixture.DelayBatch = 200 * time.Millisecond

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
			storecontent.KindSigma:  content.NewFixtureAdapter(storecontent.KindSigma),
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	syncRec := server.post(
		contentSourcePath(storecontent.SourceIDAtomic)+"/sync",
		`{}`,
		admin,
	)
	if syncRec.Code != http.StatusAccepted {
		t.Fatalf("sync = %d\n%s", syncRec.Code, syncRec.Body)
	}
	syncJob := decodeJSON[gen.ContentSyncJob](t, syncRec)

	bundle := content.FixtureBundle(storecontent.VersionCurrent, []content.FixtureNote{
		{ExternalID: "x", Title: "X", Body: "y"},
	})
	rec := server.postMultipart(
		contentSourcePath(storecontent.SourceIDSigma)+"/bundle",
		"file", "f.json", bundle, admin,
	)
	if rec.Code != http.StatusConflict {
		t.Fatalf("concurrent bundle = %d, want 409\n%s", rec.Code, rec.Body)
	}
	problem := decodeProblem(t, rec)
	if problem.Detail == nil || !strings.Contains(*problem.Detail, syncJob.Id.String()) {
		t.Fatalf("detail %v does not name jobId %s", problem.Detail, syncJob.Id)
	}

	_ = waitContentJob(t, server, admin, syncJob.Id.String())
}

func (s *authServer) postMultipart(target, field, filename string, data []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		panic(err)
	}
	if _, err := part.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == session.CookieName {
			s.attachCSRF(req, cookie)
		}
	}
	return do(s.handler, req)
}

func waitContentJob(t *testing.T, server *authServer, cookie *http.Cookie, jobID string) gen.ContentSyncJob {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		rec := server.get(BasePath+"/content/jobs/"+jobID, cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET job = %d\n%s", rec.Code, rec.Body)
		}
		job := decodeJSON[gen.ContentSyncJob](t, rec)
		switch job.Status {
		case gen.ContentSyncJobStatusSucceeded, gen.ContentSyncJobStatusFailed,
			gen.ContentSyncJobStatusCancelled, gen.ContentSyncJobStatusInterrupted:
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for job %s", jobID)
	return gen.ContentSyncJob{}
}
func bundleProblemNamesLimit(p gen.Problem, limit string) bool {
	if p.Detail != nil && strings.Contains(*p.Detail, limit) {
		return true
	}
	if p.Errors == nil {
		return false
	}
	for _, f := range *p.Errors {
		if strings.Contains(f.Message, limit) {
			return true
		}
	}
	return false
}

func manyHTTPNotes(n int) []content.FixtureNote {
	out := make([]content.FixtureNote, n)
	for i := range n {
		out[i] = content.FixtureNote{
			ExternalID: fmt.Sprintf("n-%d", i),
			Title:      fmt.Sprintf("Note %d", i),
			Body:       "b",
		}
	}
	return out
}
