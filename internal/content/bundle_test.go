package content

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "embed"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// fixtureBundleJSON is the golden payload the fixture adapter expects. Kept as
// a file so CI needs no network and parity tests share one byte-identical input
// with the online Fetch path.
//
//go:embed testdata/fixture-bundle.json
var fixtureBundleJSON []byte

func TestBundleImportParityWithFetch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Online path.
	rtFetch := newTestRunner(t, storecontent.KindAtomic, 250)
	rtFetch.fixture.FetchBytes = append([]byte(nil), fixtureBundleJSON...)
	jobF, err := rtFetch.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	jobF, err = rtFetch.runner.Wait(ctx, jobF.ID)
	if err != nil {
		t.Fatalf("Wait fetch: %v", err)
	}
	if jobF.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("fetch status = %s err=%q", jobF.Status, jobF.Error)
	}
	verF, err := rtFetch.versions.BySourceVersion(ctx, storecontent.SourceIDAtomic, storecontent.VersionCurrent)
	if err != nil {
		t.Fatalf("fetch version: %v", err)
	}

	// Offline bundle path with identical bytes. Fetch must not run.
	rtBundle := newTestRunner(t, storecontent.KindAtomic, 250)
	rtBundle.fixture.FetchErr = errors.New("network must not be used for bundle import")
	path, sha, size, err := rtBundle.runner.SpoolUpload(ctx, bytes.NewReader(fixtureBundleJSON), "fixture.json")
	if err != nil {
		t.Fatalf("SpoolUpload: %v", err)
	}
	if size != int64(len(fixtureBundleJSON)) {
		t.Fatalf("spool size = %d, want %d", size, len(fixtureBundleJSON))
	}
	jobB, err := rtBundle.runner.StartBundleImport(ctx, testActor(), StartBundleImportRequest{
		SourceID:     storecontent.SourceIDAtomic,
		BundlePath:   path,
		BundleSHA256: sha,
	})
	if err != nil {
		t.Fatalf("StartBundleImport: %v", err)
	}
	if jobB.Kind != storecontent.JobKindBundleImport {
		t.Fatalf("kind = %s, want bundle_import", jobB.Kind)
	}
	jobB, err = rtBundle.runner.Wait(ctx, jobB.ID)
	if err != nil {
		t.Fatalf("Wait bundle: %v", err)
	}
	if jobB.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("bundle status = %s err=%q", jobB.Status, jobB.Error)
	}
	verB, err := rtBundle.versions.BySourceVersion(ctx, storecontent.SourceIDAtomic, storecontent.VersionCurrent)
	if err != nil {
		t.Fatalf("bundle version: %v", err)
	}

	if verF.RawSHA256 != verB.RawSHA256 {
		t.Fatalf("raw sha mismatch: fetch=%s bundle=%s", verF.RawSHA256, verB.RawSHA256)
	}
	wantSum := sha256.Sum256(fixtureBundleJSON)
	wantSHA := hex.EncodeToString(wantSum[:])
	if verB.RawSHA256 != wantSHA {
		t.Fatalf("raw sha = %s, want %s", verB.RawSHA256, wantSHA)
	}

	var n int
	if err := rtBundle.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ? AND version = ?`,
		storecontent.SourceIDAtomic, storecontent.VersionCurrent,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("notes = %d, want 2", n)
	}

	// Spooled upload is cleaned up after the job.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("upload path still exists after success: %v", err)
	}
}

func TestBundleImportOversizedFailsBeforeJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)
	rt.runner.maxBytes = 64

	_, _, _, err := rt.runner.SpoolUpload(ctx, bytes.NewReader(bytes.Repeat([]byte("x"), 128)), "big.bin")
	if err == nil {
		t.Fatal("expected oversized spool to fail")
	}
	var tooLarge *ErrTooLarge
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err = %v (%T), want ErrTooLarge", err, err)
	}
	if tooLarge.Limit != 64 {
		t.Fatalf("limit = %d, want 64", tooLarge.Limit)
	}
	if !strings.Contains(err.Error(), "64") {
		t.Fatalf("err = %q does not name the limit", err)
	}

	jobs, err := rt.runner.ListJobs(ctx, storecontent.ListFilter{})
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %d, want 0 (oversized must not enqueue)", len(jobs))
	}
}

func TestReprocessRestoresBrokenCatalog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)

	rt.fixture.FetchBytes = append([]byte(nil), fixtureBundleJSON...)
	job, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("status = %s err=%q", job.Status, job.Error)
	}

	// Intentionally break normalized rows (test-only corruption).
	if err := rt.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM content.content_note WHERE source_id = ?`,
			storecontent.SourceIDAtomic)
		return err
	}); err != nil {
		t.Fatalf("break catalog: %v", err)
	}
	var n int
	if err := rt.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ?`,
		storecontent.SourceIDAtomic,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("notes after break = %d, want 0", n)
	}

	// Fetch must not run on reprocess.
	rt.fixture.FetchErr = errors.New("network must not be used for reprocess")
	rt.fixture.FetchBytes = nil
	rt.runner.http = httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("http disabled")
	})

	job2, err := rt.runner.StartReprocess(ctx, testActor(), StartReprocessRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartReprocess: %v", err)
	}
	if job2.Kind != storecontent.JobKindReprocess {
		t.Fatalf("kind = %s, want reprocess", job2.Kind)
	}
	job2, err = rt.runner.Wait(ctx, job2.ID)
	if err != nil {
		t.Fatalf("Wait reprocess: %v", err)
	}
	if job2.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("reprocess status = %s err=%q", job2.Status, job2.Error)
	}
	if err := rt.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content.content_note WHERE source_id = ? AND version = ?`,
		storecontent.SourceIDAtomic, storecontent.VersionCurrent,
	).Scan(&n); err != nil {
		t.Fatalf("count after reprocess: %v", err)
	}
	if n != 2 {
		t.Fatalf("notes after reprocess = %d, want 2", n)
	}
}

func TestReprocessMissingRawIsConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)

	_, err := rt.runner.StartReprocess(ctx, testActor(), StartReprocessRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), "no raw snapshot") {
		t.Fatalf("err = %q, want no raw snapshot", err)
	}
}

func TestBundleImportWhileJobActiveIsConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)
	rt.fixture.FetchBytes = FixtureBundle(storecontent.VersionCurrent, manyNotes(5))
	rt.fixture.DelayBatch = 200 * time.Millisecond

	job1, err := rt.runner.StartSync(ctx, testActor(), StartSyncRequest{
		SourceID: storecontent.SourceIDAtomic,
	})
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}

	path, sha, _, err := rt.runner.SpoolUpload(ctx, bytes.NewReader(fixtureBundleJSON), "f.json")
	if err != nil {
		t.Fatalf("SpoolUpload: %v", err)
	}
	_, err = rt.runner.StartBundleImport(ctx, testActor(), StartBundleImportRequest{
		SourceID:     storecontent.SourceIDSigma,
		BundlePath:   path,
		BundleSHA256: sha,
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}
	if !strings.Contains(err.Error(), job1.ID) {
		t.Fatalf("conflict %q does not name jobId %s", err.Error(), job1.ID)
	}
	// Spooled file removed on enqueue failure.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("upload should be removed after failed start: %v", err)
	}

	if _, err := rt.runner.Wait(ctx, job1.ID); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestReadBundleMultipartAirGap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)
	rt.fixture.FetchErr = errors.New("http must not run")
	rt.runner.http = httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("http disabled")
	})

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(fixtureBundleJSON); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(&body, w.Boundary())
	path, sha, version, size, err := rt.runner.ReadBundleMultipart(ctx, mr)
	if err != nil {
		t.Fatalf("ReadBundleMultipart: %v", err)
	}
	if version != "" {
		t.Fatalf("version = %q, want empty", version)
	}
	if size != int64(len(fixtureBundleJSON)) {
		t.Fatalf("size = %d", size)
	}

	job, err := rt.runner.StartBundleImport(ctx, testActor(), StartBundleImportRequest{
		SourceID:     storecontent.SourceIDAtomic,
		BundlePath:   path,
		BundleSHA256: sha,
	})
	if err != nil {
		t.Fatalf("StartBundleImport: %v", err)
	}
	job, err = rt.runner.Wait(ctx, job.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if job.Status != storecontent.JobStatusSucceeded {
		t.Fatalf("status = %s err=%q", job.Status, job.Error)
	}
}

func TestSpoolUploadRejectsEmpty(t *testing.T) {
	t.Parallel()
	rt := newTestRunner(t, storecontent.KindAtomic, 250)
	_, _, _, err := rt.runner.SpoolUpload(context.Background(), bytes.NewReader(nil), "empty.bin")
	if err == nil {
		t.Fatal("expected empty spool to fail")
	}
	if !errors.Is(err, apierr.ErrValidation) {
		t.Fatalf("err = %v, want validation", err)
	}
	var ae *apierr.Error
	if !errors.As(err, &ae) || len(ae.Fields()) == 0 {
		t.Fatalf("err = %v, want field detail", err)
	}
	if got := ae.Fields()[0].Message; !strings.Contains(strings.ToLower(got), "empty") {
		t.Fatalf("field message = %q, want empty", got)
	}
}

func TestMapUploadErrNamesLimit(t *testing.T) {
	t.Parallel()
	err := mapUploadErr(&ErrTooLarge{Limit: 1024, Got: 2048})
	if !errors.Is(err, apierr.ErrValidation) {
		t.Fatalf("err = %v, want validation", err)
	}
	var ae *apierr.Error
	if !errors.As(err, &ae) || len(ae.Fields()) == 0 {
		t.Fatalf("err = %v, want field detail", err)
	}
	if got := ae.Fields()[0].Message; !strings.Contains(got, "1024") {
		t.Fatalf("field message = %q does not name limit", got)
	}
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
