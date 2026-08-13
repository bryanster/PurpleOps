package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// ownershipFixture is one blind engagement with the full parent chain Facts
// walks. Revealed and unrevealed siblings are both present so the reveal flag
// can be asserted for step-shaped resources.
type ownershipFixture struct {
	engagement storengagement.Engagement
	scenario   storengagement.Scenario

	unrevealedStep storengagement.Step
	unrevealedExec storengagement.Execution
	revealedStep   storengagement.Step
	revealedExec   storengagement.Execution

	evidence storengagement.Evidence
	finding  storengagement.Finding
	report   storereport.Report
	template storereport.Template
	version  storereport.ReportVersion
	share    storereport.ReportShare
}

func seedOwnershipFixture(t *testing.T, db *store.DB) ownershipFixture {
	t.Helper()
	ctx := t.Context()

	engagements := storengagement.NewEngagements(db)
	eng, err := engagements.Create(ctx, storengagement.NewEngagement{
		Name:          "blind-fixture",
		Client:        "test",
		StartsOn:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AttackVersion: "15.1",
		Mode:          storengagement.EngagementModeBlind,
		CreatedBy:     "seed",
	})
	if err != nil {
		t.Fatalf("seed engagement: %v", err)
	}

	scenarios := storengagement.NewScenarios(db)
	sc, err := scenarios.Create(ctx, storengagement.NewScenario{
		EngagementID: eng.ID,
		Ordinal:      1,
		Name:         "scenario",
		Source:       storengagement.ScenarioSourceManual,
	})
	if err != nil {
		t.Fatalf("seed scenario: %v", err)
	}

	newStep := func(name string, ordinal int) storengagement.NewStep {
		return storengagement.NewStep{
			ScenarioID:    sc.ID,
			Ordinal:       ordinal,
			Name:          name,
			TechniqueID:   "T1003",
			AttackVersion: "15.1",
		}
	}

	steps := storengagement.NewSteps(db)
	unrevealed, unrevealedExec, err := steps.CreateWithExecution(ctx, newStep("unrevealed", 1))
	if err != nil {
		t.Fatalf("seed unrevealed step: %v", err)
	}
	revealed, revealedExec, err := steps.CreateWithExecution(ctx, newStep("revealed", 2))
	if err != nil {
		t.Fatalf("seed revealed step: %v", err)
	}
	if _, err := steps.Reveal(ctx, revealed.ID); err != nil {
		t.Fatalf("reveal step: %v", err)
	}
	blobSHA := strings.Repeat("a", 64)
	if err := storengagement.NewEvidenceBlobRepo(db).InsertBlob(ctx, blobSHA, "text/plain", "/tmp/unused", 4); err != nil {
		t.Fatalf("seed blob: %v", err)
	}

	evidence, err := storengagement.NewEvidenceRepo(db).Create(ctx, storengagement.NewEvidence{
		BlobSHA256:  blobSHA,
		Filename:    "evidence.txt",
		Caption:     "evidence",
		Side:        storengagement.EvidenceSideRed,
		ExecutionID: unrevealedExec.ID,
		UploadedBy:  "seed",
		Size:        4,
		MIME:        "text/plain",
	})
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	finding, err := storengagement.NewFindings(db).Create(ctx, storengagement.NewFinding{
		EngagementID: eng.ID,
		Title:        "finding",
		Severity:     "high",
	})
	if err != nil {
		t.Fatalf("seed finding: %v", err)
	}

	report, err := storereport.NewReports(db).Create(ctx, storereport.NewReport{
		EngagementID: eng.ID,
		Title:        "report",
		CreatedBy:    "seed",
	})
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}

	tmpl, err := storereport.NewTemplates(db).Create(ctx, storereport.NewTemplate{
		EngagementID: eng.ID,
		Name:         "template",
		CreatedBy:    "seed",
	})
	if err != nil {
		t.Fatalf("seed template: %v", err)
	}

	version, err := storereport.NewVersions(db).Insert(ctx, storereport.NewVersion{
		ReportID:      report.ID,
		Ordinal:       1,
		Title:         "version",
		PublishedBy:   "seed",
		BlindScope:    "lead",
		BlocksJSON:    "[]",
		BrandingJSON:  "{}",
		HTML:          "<html></html>",
		ContentSHA256: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("seed version: %v", err)
	}

	share, err := storereport.NewShares(db).Insert(ctx, storereport.NewShare{
		VersionID: version.ID,
		TokenHash: strings.Repeat("c", 64),
		CreatedBy: "seed",
	})
	if err != nil {
		t.Fatalf("seed share: %v", err)
	}

	return ownershipFixture{
		engagement: eng, scenario: sc,
		unrevealedStep: unrevealed, unrevealedExec: unrevealedExec,
		revealedStep: revealed, revealedExec: revealedExec,
		evidence: evidence, finding: finding, report: report,
		template: tmpl, version: version, share: share,
	}
}

func assertFacts(t *testing.T, own Ownership, ref ResourceRef, want ResourceFacts) {
	t.Helper()
	got, err := own.Facts(t.Context(), ref)
	if err != nil {
		t.Fatalf("Facts(%+v) error: %v", ref, err)
	}
	if got != want {
		t.Errorf("Facts(%+v) = %+v, want %+v", ref, got, want)
	}
}

func assertFactsNotFound(t *testing.T, own Ownership, ref ResourceRef) {
	t.Helper()
	_, err := own.Facts(t.Context(), ref)
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Facts(%+v) error = %v, want apierr.ErrNotFound", ref, err)
	}
}

// TestOwnershipFactsResolvesEachResourceType proves the loader walks every
// resource shape to its owning engagement, carrying blind mode and the reveal
// flag for step-shaped resources.
func TestOwnershipFactsResolvesEachResourceType(t *testing.T) {
	db := storetest.Migrated(t)
	fx := seedOwnershipFixture(t, db)
	own := NewOwnership(db)
	eng := fx.engagement.ID
	blind := true

	revealedFacts := ResourceFacts{EngagementID: eng, Blind: blind, Revealed: true}

	t.Run("engagement", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceEngagement, ID: eng, EngagementID: eng}, revealedFacts)
	})
	t.Run("member", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceMember, EngagementID: eng}, revealedFacts)
	})
	t.Run("scenario", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceScenario, ID: fx.scenario.ID, EngagementID: eng}, revealedFacts)
	})
	t.Run("finding", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceFinding, ID: fx.finding.ID, EngagementID: eng}, revealedFacts)
	})
	t.Run("report", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceReport, ID: fx.report.ID, EngagementID: eng}, revealedFacts)
	})
	t.Run("template", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceReport, ID: fx.template.ID, EngagementID: eng, Kind: "template"}, revealedFacts)
	})
	t.Run("version", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceReport, ID: fx.version.ID, Kind: "version"}, revealedFacts)
	})
	t.Run("share", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceReport, ID: fx.share.ID, Kind: "share"}, revealedFacts)
	})
	t.Run("unrevealed execution", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceExecution, ID: fx.unrevealedExec.ID, EngagementID: eng},
			ResourceFacts{EngagementID: eng, Blind: blind, Revealed: false})
	})
	t.Run("revealed execution", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceExecution, ID: fx.revealedExec.ID, EngagementID: eng}, revealedFacts)
	})
	t.Run("unrevealed evidence", func(t *testing.T) {
		assertFacts(t, own, ResourceRef{Type: authz.ResourceEvidence, ID: fx.evidence.ID},
			ResourceFacts{EngagementID: eng, Blind: blind, Revealed: false})
	})
	t.Run("evidence by execution", func(t *testing.T) {
		// uploadEvidence addresses the parent execution, not an evidence id.
		assertFacts(t, own, ResourceRef{Type: authz.ResourceEvidence, ID: fx.unrevealedExec.ID, Kind: "execution"},
			ResourceFacts{EngagementID: eng, Blind: blind, Revealed: false})
	})
}

// TestOwnershipFactsMissingRowIsNotFound proves a missing row reads as a
// concealed denial: NotFound, exactly like a non-member.
func TestOwnershipFactsMissingRowIsNotFound(t *testing.T) {
	db := storetest.Migrated(t)
	own := NewOwnership(db)
	missing := "01900000-ffff-7000-8000-000000000000"

	for _, ref := range []ResourceRef{
		{Type: authz.ResourceEngagement, ID: missing, EngagementID: missing},
		{Type: authz.ResourceScenario, ID: missing, EngagementID: missing},
		{Type: authz.ResourceExecution, ID: missing, EngagementID: missing},
		{Type: authz.ResourceEvidence, ID: missing},
		{Type: authz.ResourceFinding, ID: missing, EngagementID: missing},
		{Type: authz.ResourceReport, ID: missing, EngagementID: missing},
		{Type: authz.ResourceReport, ID: missing, EngagementID: missing, Kind: "template"},
		{Type: authz.ResourceReport, ID: missing, Kind: "version"},
		{Type: authz.ResourceReport, ID: missing, Kind: "share"},
	} {
		assertFactsNotFound(t, own, ref)
	}
}

// TestOwnershipFactsMismatchedEngagementIsNotFound proves a child id from one
// engagement read through another engagement's path is 404, not the child's
// contents.
func TestOwnershipFactsMismatchedEngagementIsNotFound(t *testing.T) {
	db := storetest.Migrated(t)
	fx := seedOwnershipFixture(t, db)
	own := NewOwnership(db)

	other, err := storengagement.NewEngagements(db).Create(t.Context(), storengagement.NewEngagement{
		Name:          "other",
		StartsOn:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AttackVersion: "15.1",
		Mode:          storengagement.EngagementModeStandard,
		CreatedBy:     "seed",
	})
	if err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}

	assertFactsNotFound(t, own, ResourceRef{Type: authz.ResourceScenario, ID: fx.scenario.ID, EngagementID: other.ID})
}

// TestBlindMiddlewareConcealsUnrevealedStep drives the real chain with the
// default (store-backed) loader: a blue member of a blind engagement gets 404
// for evidence.read and execution.read on an unrevealed step, while red is
// unaffected, and revealing the step un-conceals it for blue.
func TestBlindMiddlewareConcealsUnrevealedStep(t *testing.T) {
	server := newAuthServer(t)
	red := createUser(t, server, "red@blind.test", "Red")
	blue := createUser(t, server, "blue@blind.test", "Blue")

	engID := "01900000-c001-7000-8000-000000000001"
	scenarioID := "01900000-c001-7000-8000-000000000002"
	seedBlindEngagementDB(t, server, engID, scenarioID, red, blue)

	steps := storengagement.NewSteps(server.db)
	step, exec, err := steps.CreateWithExecution(t.Context(), storengagement.NewStep{
		ScenarioID:    scenarioID,
		Ordinal:       1,
		Name:          "unrevealed",
		TechniqueID:   "T1003",
		AttackVersion: "15.1",
	})
	if err != nil {
		t.Fatalf("seed step: %v", err)
	}
	blobSHA := strings.Repeat("a", 64)
	if err := storengagement.NewEvidenceBlobRepo(server.db).InsertBlob(t.Context(), blobSHA, "text/plain", "/tmp/unused", 1); err != nil {
		t.Fatalf("seed blob: %v", err)
	}

	evidence, err := storengagement.NewEvidenceRepo(server.db).Create(t.Context(), storengagement.NewEvidence{
		BlobSHA256:  blobSHA,
		Filename:    "e.txt",
		Caption:     "e",
		Side:        storengagement.EvidenceSideRed,
		ExecutionID: exec.ID,
		UploadedBy:  red.ID,
		Size:        1,
		MIME:        "text/plain",
	})
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	redCookie := sessionCookie(t, server.login(red.Email, testPassword))
	blueCookie := sessionCookie(t, server.login(blue.Email, testPassword))

	evidencePath := BasePath + "/evidence/" + evidence.ID
	executionPath := BasePath + "/engagements/" + engID + "/executions/" + exec.ID

	// Red holds evidence.read without the blind guard: 200.
	if rec := server.get(evidencePath, redCookie); rec.Code != http.StatusOK {
		t.Fatalf("red GET evidence = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	// Blue on an unrevealed step: 404, indistinguishable from a missing row.
	if rec := server.get(evidencePath, blueCookie); rec.Code != http.StatusNotFound {
		t.Errorf("blue GET evidence = %d, want 404\nbody: %s", rec.Code, rec.Body)
	}
	if rec := server.get(executionPath, blueCookie); rec.Code != http.StatusNotFound {
		t.Errorf("blue GET execution = %d, want 404\nbody: %s", rec.Code, rec.Body)
	}

	// Reveal the step, and blue can see both again.
	if _, err := steps.Reveal(t.Context(), step.ID); err != nil {
		t.Fatalf("reveal step: %v", err)
	}
	if rec := server.get(evidencePath, blueCookie); rec.Code != http.StatusOK {
		t.Errorf("blue GET evidence after reveal = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	if rec := server.get(executionPath, blueCookie); rec.Code != http.StatusOK {
		t.Errorf("blue GET execution after reveal = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
}

// TestAdminNotMemberOfBlindEngagementReadsEvidence proves the platform admin
// who holds no seat in a blind engagement is unaffected by the blind guard:
// the guard binds to the blue *seat*, not the platform role.
func TestAdminNotMemberOfBlindEngagementReadsEvidence(t *testing.T) {
	server := newAuthServer(t)

	admin, err := identity.NewUsers(server.db).Create(t.Context(), identity.NewUser{
		Email:        "admin@blind.test",
		DisplayName:  "Admin",
		PasswordHash: testPasswordHash(),
		PlatformRole: authz.PlatformRoleAdmin,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	red := createUser(t, server, "red2@blind.test", "Red")
	blue := createUser(t, server, "blue2@blind.test", "Blue")
	engID := "01900000-c002-7000-8000-000000000001"
	scenarioID := "01900000-c002-7000-8000-000000000002"
	seedBlindEngagementDB(t, server, engID, scenarioID, red, blue)

	steps := storengagement.NewSteps(server.db)
	_, exec, err := steps.CreateWithExecution(t.Context(), storengagement.NewStep{
		ScenarioID:    scenarioID,
		Ordinal:       1,
		Name:          "unrevealed",
		TechniqueID:   "T1003",
		AttackVersion: "15.1",
	})
	if err != nil {
		t.Fatalf("seed step: %v", err)
	}
	blobSHA := strings.Repeat("a", 64)
	if err := storengagement.NewEvidenceBlobRepo(server.db).InsertBlob(t.Context(), blobSHA, "text/plain", "/tmp/unused", 1); err != nil {
		t.Fatalf("seed blob: %v", err)
	}

	evidence, err := storengagement.NewEvidenceRepo(server.db).Create(t.Context(), storengagement.NewEvidence{
		BlobSHA256:  blobSHA,
		Filename:    "e.txt",
		Caption:     "e",
		Side:        storengagement.EvidenceSideRed,
		ExecutionID: exec.ID,
		UploadedBy:  red.ID,
		Size:        1,
		MIME:        "text/plain",
	})
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	adminCookie := sessionCookie(t, server.login(admin.Email, testPassword))
	if rec := server.get(BasePath+"/evidence/"+evidence.ID, adminCookie); rec.Code != http.StatusOK {
		t.Errorf("admin (non-member) GET evidence = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
}
