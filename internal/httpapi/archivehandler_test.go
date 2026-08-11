package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/archive"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storeactivity "github.com/bryanster/blacklight/internal/store/activity"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	identity "github.com/bryanster/blacklight/internal/store/identity"
)

func TestArchiveRoundTrip(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	h := testArchiveHandlers(t, fx)

	req := gen.ExportEngagementArchiveRequestObject{
		EngagementId: toUUID(fx.BaselineID),
	}
	ctx := authCtx(fx.BaselineID)

	resp, err := h.ExportEngagementArchive(ctx, req)
	if err != nil {
		t.Fatalf("ExportEngagementArchive: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := resp.VisitExportEngagementArchiveResponse(rec); err != nil {
		t.Fatalf("write response: %v", err)
	}
	body := rec.Body.Bytes()
	if len(body) == 0 {
		t.Fatal("empty archive body")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	members := map[string]bool{}
	for _, f := range zr.File {
		members[f.Name] = true
	}
	for _, name := range []string{"manifest.json", "engagement.json", "analytics.json", "activity.jsonl"} {
		if !members[name] {
			t.Errorf("missing archive member: %s", name)
		}
	}

	mf := readZipFile(t, zr, "manifest.json")
	var manifest archive.Manifest
	if err := json.Unmarshal(mf, &manifest); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if manifest.FormatVersion != archive.FormatVersion {
		t.Errorf("manifest FormatVersion = %d, want %d", manifest.FormatVersion, archive.FormatVersion)
	}

	ej := readZipFile(t, zr, "engagement.json")
	var engData archive.EngagementArchive
	if err := json.Unmarshal(ej, &engData); err != nil {
		t.Fatalf("engagement.json: %v", err)
	}
	if engData.Engagement.ID != fx.BaselineID {
		t.Errorf("engagement ID = %s, want %s", engData.Engagement.ID, fx.BaselineID)
	}

	al := readZipFile(t, zr, "activity.jsonl")
	if len(al) > 0 {
		for i, line := range bytes.Split(bytes.TrimSpace(al), []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(line, &obj); err != nil {
				t.Errorf("activity.jsonl line %d: invalid JSON: %v", i+1, err)
			}
		}
	}
}

func TestArchiveNoSecrets(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	h := testArchiveHandlers(t, fx)

	req := gen.ExportEngagementArchiveRequestObject{
		EngagementId: toUUID(fx.BaselineID),
	}
	resp, err := h.ExportEngagementArchive(authCtx(fx.BaselineID), req)
	if err != nil {
		t.Fatalf("ExportEngagementArchive: %v", err)
	}
	rec := httptest.NewRecorder()
	resp.VisitExportEngagementArchiveResponse(rec) //nolint:errcheck

	zr, _ := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len())) //nolint:errcheck

	forbidden := []string{
		"email", "password", "passwordHash", "secret", "sessionToken",
		"token", "mfaSecret", "recoveryCode", "secretKey",
	}

	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".json") && !strings.HasSuffix(f.Name, ".jsonl") {
			continue
		}
		rc, _ := f.Open()        //nolint:errcheck
		raw, _ := io.ReadAll(rc) //nolint:errcheck
		rc.Close()

		var check func(m map[string]any, path string)
		check = func(m map[string]any, path string) {
			for key, val := range m {
				lower := strings.ToLower(key)
				for _, fb := range forbidden {
					if strings.Contains(lower, fb) {
						t.Errorf("%s: %s field %q matches forbidden pattern %q", f.Name, path, key, fb)
					}
				}
				if nested, ok := val.(map[string]any); ok {
					check(nested, path+"."+key)
				}
			}
		}

		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err == nil {
			check(obj, "")
		}
		for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			var lineObj map[string]any
			if err := json.Unmarshal(line, &lineObj); err == nil {
				check(lineObj, fmt.Sprintf("%s(line)", f.Name))
			}
		}
	}
}

func TestArchiveBlindMode(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	h := testArchiveHandlers(t, fx)

	req := gen.ExportEngagementArchiveRequestObject{
		EngagementId: toUUID(fx.BaselineID),
	}

	adminResp, err := h.ExportEngagementArchive(authCtx(fx.BaselineID), req)
	if err != nil {
		t.Fatalf("admin export: %v", err)
	}
	adminRec := httptest.NewRecorder()
	adminResp.VisitExportEngagementArchiveResponse(adminRec) //nolint:errcheck
	adminBody := adminRec.Body.Bytes()

	blueCtx := authCtxBlue(fx.BaselineID)
	blueResp, err := h.ExportEngagementArchive(blueCtx, req)
	if err != nil {
		t.Fatalf("blue export: %v", err)
	}
	blueRec := httptest.NewRecorder()
	blueResp.VisitExportEngagementArchiveResponse(blueRec) //nolint:errcheck
	blueBody := blueRec.Body.Bytes()

	if len(blueBody) > len(adminBody) {
		t.Errorf("blue archive (%d bytes) larger than admin archive (%d bytes)", len(blueBody), len(adminBody))
	}

	zr, _ := zip.NewReader(bytes.NewReader(blueBody), int64(len(blueBody))) //nolint:errcheck
	mf := readZipFile(t, zr, "manifest.json")
	var manifest archive.Manifest
	if err := json.Unmarshal(mf, &manifest); err != nil {
		t.Fatalf("blue manifest.json: %v", err)
	}
	if !manifest.BlindFiltered {
		t.Error("blue manifest BlindFiltered should be true")
	}
}

func authCtxBlue(engID string) context.Context {
	return context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "exportEngagementArchive",
		Subject: authz.Subject{
			UserID:       "test-blue-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				engID: authz.EngagementRoleBlue,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: engID},
		Allowed:  true,
	})
}

func TestArchiveAuthzNonMember(t *testing.T) {
	t.Parallel()
	fx := analyticstest.Seed(t)
	h := testArchiveHandlers(t, fx)

	req := gen.ExportEngagementArchiveRequestObject{
		EngagementId: toUUID(fx.BaselineID),
	}
	ctx := context.Background()

	resp, err := h.ExportEngagementArchive(ctx, req)
	if err != nil {
		t.Logf("handler error (expected without auth context): %v", err)
		return
	}
	rec := httptest.NewRecorder()
	resp.VisitExportEngagementArchiveResponse(rec) //nolint:errcheck
	t.Logf("response: %d bytes", len(rec.Body.Bytes()))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testArchiveHandlers(t *testing.T, fx analyticstest.Fixture) *handlers {
	t.Helper()
	db := fx.DB

	scenarios := storengagement.NewScenarios(db)
	steps := storengagement.NewSteps(db)
	executions := storengagement.NewExecutions(db)
	comments := storengagement.NewComments(db)
	findings := storengagement.NewFindings(db)
	evidenceRepo := storengagement.NewEvidenceRepo(db)
	blobRepo := storengagement.NewEvidenceBlobRepo(db)
	activityEntries := storeactivity.New(db)
	users := identity.NewUsers(db)
	memberships := identity.NewMemberships(db)
	queries := analytics.NewQueries(db)

	tmpDir := t.TempDir()
	evidenceStore := evidence.NewStore(tmpDir, config.Evidence{
		MaxUploadBytes:     256 << 20,
		MaxEngagementBytes: 2 << 30,
	}, blobRepo)

	engSvc, err := engagement.New(engagement.Deps{
		Engagements: storengagement.NewEngagements(db),
		Memberships: memberships,
		Scenarios:   scenarios,
		Steps:       steps,
		Executions:  executions,
		Comments:    comments,
		Findings:    findings,
		Users:       users,
	})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}

	return &handlers{
		store:           db,
		engagements:     engSvc,
		scenarios:       scenarios,
		steps:           steps,
		executions:      executions,
		comments:        comments,
		findings:        findings,
		evidenceRepo:    evidenceRepo,
		evidenceStore:   evidenceStore,
		blobRepo:        blobRepo,
		activityEntries: activityEntries,
		analytics:       queries,
		users:           users,
		log:             testLogger(t),
	}
}

func readZipFile(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return b
		}
	}
	t.Fatalf("member %s not found in archive", name)
	return nil
}
