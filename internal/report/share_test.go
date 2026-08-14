package report_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// shareFixture is a ShareService over a migrated database with one published
// version available to share.
type shareFixture struct {
	db        *store.DB
	svc       *report.ShareService
	hasher    *report.ShareTokenHasher
	versionID string
}

func newShareFixture(t *testing.T) *shareFixture {
	t.Helper()
	ctx := context.Background()

	db := storetest.Migrated(t)
	hasher, err := report.NewShareTokenHasher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("share hasher: %v", err)
	}

	svc, err := report.NewShareService(report.ShareServiceDeps{
		Shares:      storereport.NewShares(db),
		Grants:      storereport.NewGrants(db),
		Versions:    storereport.NewVersions(db),
		TokenHasher: hasher,
		Activity:    events.New(activity.New(db)),
		BaseURL:     "http://example.com",
		SessionSec:  []byte("session-secret"),
	})
	if err != nil {
		t.Fatalf("NewShareService: %v", err)
	}

	eng, err := storengagement.NewEngagements(db).Create(ctx, storengagement.NewEngagement{
		Name:          "share-fixture",
		Client:        "test",
		StartsOn:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AttackVersion: "15.1",
		Mode:          storengagement.EngagementModeStandard,
		CreatedBy:     "seed",
	})
	if err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	rep, err := storereport.NewReports(db).Create(ctx, storereport.NewReport{
		EngagementID: eng.ID,
		Title:        "report",
		CreatedBy:    "seed",
	})
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}
	ver, err := storereport.NewVersions(db).Insert(ctx, storereport.NewVersion{
		ReportID:      rep.ID,
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

	return &shareFixture{db: db, svc: svc, hasher: hasher, versionID: ver.ID}
}

func strptr(s string) *string { return &s }

func TestCreateSharePasswordPolicy(t *testing.T) {
	t.Parallel()

	f := newShareFixture(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		password *string
		wantErr  bool
	}{
		{"short rejected", strptr("abc"), true},
		{"empty rejected", strptr(""), true},
		{"omitted allowed", nil, false},
		{"valid allowed", strptr("a memorable share passphrase"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.svc.CreateShare(ctx, report.CreateShareInput{
				VersionID: f.versionID,
				Password:  tt.password,
				CreatedBy: "seed",
			})
			if tt.wantErr && err == nil {
				t.Fatalf("CreateShare(password=%q) = nil error, want rejection", *tt.password)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CreateShare(password=%q) = %v, want success", *tt.password, err)
			}
		})
	}
}

func TestConcurrentClaimShareRespectsMaxGrants(t *testing.T) {
	t.Parallel()

	f := newShareFixture(t)
	ctx := context.Background()

	maxGrants := 1
	token, tokenHash, err := f.hasher.Generate()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	share, err := storereport.NewShares(f.db).Insert(ctx, storereport.NewShare{
		VersionID: f.versionID,
		TokenHash: tokenHash,
		CreatedBy: "seed",
		MaxGrants: &maxGrants,
	})
	if err != nil {
		t.Fatalf("seed share: %v", err)
	}

	results := make([]error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.ClaimShare(ctx, report.ClaimShareInput{
				Token:  token,
				UserID: "user-" + strconv.Itoa(i+1),
			})
			results[i] = err
		}(i)
	}
	wg.Wait()

	var ok, forbidden int
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, apierr.ErrForbidden):
			forbidden++
		default:
			t.Fatalf("unexpected claim error: %v", err)
		}
	}
	if ok != 1 || forbidden != 1 {
		t.Fatalf("claims = %d ok / %d forbidden, want exactly one of each (results: %v)", ok, forbidden, results)
	}

	grants, err := storereport.NewGrants(f.db).ListByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("list grants: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("grant count = %d, want 1", len(grants))
	}
}
