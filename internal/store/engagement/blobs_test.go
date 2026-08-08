package engagement_test

import (
	"context"
	"testing"

	engagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestBlobRepo_RefCountGC(t *testing.T) {
	db := storetest.Migrated(t)
	repo := engagement.NewEvidenceBlobRepo(db)

	sha := "abc123def456"
	ctx := context.Background()

	// Insert a blob with ref_count=1.
	err := repo.InsertBlob(ctx, sha, "image/png", "ab/abc123def456", 1024)
	if err != nil {
		t.Fatalf("InsertBlob: %v", err)
	}

	// Read it back.
	blob, err := repo.GetBlob(ctx, sha)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if blob.RefCount != 1 {
		t.Errorf("RefCount = %d, want 1", blob.RefCount)
	}

	// Increment ref.
	err = repo.IncrementRef(ctx, sha)
	if err != nil {
		t.Fatalf("IncrementRef: %v", err)
	}

	blob, err = repo.GetBlob(ctx, sha)
	if err != nil {
		t.Fatalf("GetBlob after increment: %v", err)
	}
	if blob.RefCount != 2 {
		t.Errorf("RefCount after increment = %d, want 2", blob.RefCount)
	}

	// Decrement ref — should NOT gc.
	gc, err := repo.DecrementRef(ctx, sha)
	if err != nil {
		t.Fatalf("DecrementRef: %v", err)
	}
	if gc {
		t.Error("DecrementRef from 2 should not gc")
	}

	// Decrement again — SHOULD gc.
	gc, err = repo.DecrementRef(ctx, sha)
	if err != nil {
		t.Fatalf("DecrementRef (second): %v", err)
	}
	if !gc {
		t.Error("DecrementRef from 1 should gc")
	}

	// Delete the blob row.
	err = repo.DeleteBlob(ctx, sha)
	if err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}
}

func TestBlobRepo_QuotaQuery(t *testing.T) {
	db := storetest.Migrated(t)
	r := repos{
		Engagements: engagement.NewEngagements(db),
		Scenarios:   engagement.NewScenarios(db),
		Steps:       engagement.NewSteps(db),
		Executions:  engagement.NewExecutions(db),
		Evidence:    engagement.NewEvidenceRepo(db),
	}
	blobRepo := engagement.NewEvidenceBlobRepo(db)

	ctx := context.Background()

	// Set up: engagement -> scenario -> step -> execution.
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	_, exec := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")

	// Insert blob.
	err := blobRepo.InsertBlob(ctx, "sha256abc", "image/png", "sh/sha256abc", 512)
	if err != nil {
		t.Fatalf("InsertBlob: %v", err)
	}

	// Link evidence to execution.
	_, err = r.Evidence.Create(ctx, engagement.NewEvidence{
		BlobSHA256:  "sha256abc",
		Filename:    "test.png",
		Caption:     "",
		Side:        engagement.EvidenceSideRed,
		ExecutionID: exec.ID,
		UploadedBy:  "user-1",
		Size:        512,
		MIME:        "image/png",
	})
	if err != nil {
		t.Fatalf("Create evidence: %v", err)
	}

	// Quota query.
	used, err := blobRepo.EngagementBlobBytes(ctx, eng.ID)
	if err != nil {
		t.Fatalf("EngagementBlobBytes: %v", err)
	}
	if used != 512 {
		t.Errorf("quota = %d, want 512", used)
	}

	// Second blob, same engagement — quota counts unique blobs.
	err = blobRepo.InsertBlob(ctx, "sha256xyz", "image/jpeg", "sh/sha256xyz", 256)
	if err != nil {
		t.Fatalf("InsertBlob second: %v", err)
	}

	_, err = r.Evidence.Create(ctx, engagement.NewEvidence{
		BlobSHA256:  "sha256xyz",
		Filename:    "test2.jpg",
		Caption:     "",
		Side:        engagement.EvidenceSideBlue,
		ExecutionID: exec.ID,
		UploadedBy:  "user-1",
		Size:        256,
		MIME:        "image/jpeg",
	})
	if err != nil {
		t.Fatalf("Create evidence second: %v", err)
	}

	used, err = blobRepo.EngagementBlobBytes(ctx, eng.ID)
	if err != nil {
		t.Fatalf("EngagementBlobBytes: %v", err)
	}
	if used != 768 { // 512 + 256
		t.Errorf("quota = %d, want 768", used)
	}

	// Same blob linked twice — counts once for unique.
	_, err = r.Evidence.Create(ctx, engagement.NewEvidence{
		BlobSHA256:  "sha256abc",
		Filename:    "test3.png",
		Caption:     "",
		Side:        engagement.EvidenceSideRed,
		ExecutionID: exec.ID,
		UploadedBy:  "user-2",
		Size:        512,
		MIME:        "image/png",
	})
	if err != nil {
		t.Fatalf("Create evidence duplicate blob: %v", err)
	}

	used, err = blobRepo.EngagementBlobBytes(ctx, eng.ID)
	if err != nil {
		t.Fatalf("EngagementBlobBytes after duplicate: %v", err)
	}
	if used != 768 { // still 768 — unique blobs only
		t.Errorf("quota after duplicate blob = %d, want 768", used)
	}
}
