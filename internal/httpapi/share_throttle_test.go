package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/report"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// seedPasswordShare inserts a published version and a password-gated share for
// it, returning the raw share token. The token hash is derived under the same
// encryption key the test server uses, so the server's ShareService resolves it.
func seedPasswordShare(t *testing.T, server *authServer, sharePassword string) string {
	t.Helper()
	ctx := t.Context()

	hasher, err := report.NewShareTokenHasher([]byte("test-encryption-key-also-not-real"))
	if err != nil {
		t.Fatalf("share hasher: %v", err)
	}
	token, tokenHash, err := hasher.Generate()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	hash, err := password.Hash(password.Plaintext(sharePassword))
	if err != nil {
		t.Fatalf("hash share password: %v", err)
	}

	eng, err := storengagement.NewEngagements(server.db).Create(ctx, storengagement.NewEngagement{
		Name:          "share-throttle-fixture",
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
	rep, err := storereport.NewReports(server.db).Create(ctx, storereport.NewReport{
		EngagementID: eng.ID,
		Title:        "report",
		CreatedBy:    "seed",
	})
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}
	ver, err := storereport.NewVersions(server.db).Insert(ctx, storereport.NewVersion{
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
	if _, err := storereport.NewShares(server.db).Insert(ctx, storereport.NewShare{
		VersionID:    ver.ID,
		TokenHash:    tokenHash,
		PasswordHash: &hash,
		CreatedBy:    "seed",
	}); err != nil {
		t.Fatalf("seed share: %v", err)
	}
	return token
}

func TestSharePasswordAttemptsAreThrottled(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	token := seedPasswordShare(t, server, "correct horse battery staple")

	target := BasePath + "/report-views/" + token + "/password"
	for i := 0; i < 3; i++ {
		rec := server.post(target, `{"password":"wrong password"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401\nbody: %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := server.post(target, `{"password":"wrong password"}`)
	throttled(t, rec, "the fourth share-password failure")
}
