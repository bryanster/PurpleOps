package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// TestPublishedReportVersionIsNotReadableAcrossEngagements guards the
// cross-engagement IDOR on the published-report-version endpoints.
//
// getReportVersion / getReportVersionHtml / getReportVersionPdf declare
// `x-authz-resource: {type: report, engagement: engagementId, param: versionId,
// kind: version}`. The ownership walker resolves versionId to its owning
// report and engagement, and Facts refuses when that owner differs from the
// engagement named in the path — so a member of one engagement cannot read
// another engagement's published report by supplying its versionId under their
// own engagement's path.
func TestPublishedReportVersionIsNotReadableAcrossEngagements(t *testing.T) {
	s := newAuthServer(t)

	// The victim: an administrator who owns engagement B and publishes a report.
	admin := s.seedUser(t)
	adminCookie := s.signIn(t)

	// The attacker: an ordinary platform member who is a member of engagement A
	// only, and never of B.
	attacker := s.seedUser(t, func(in *identity.NewUser) {
		in.Email = "attacker@example.com"
		in.PlatformRole = authz.PlatformRoleMember
	})
	attackerCookie := sessionCookie(t, s.login(attacker.Email, testPassword))

	const engB = "019385a2-9100-7000-8cf0-ef0000000002" // victim's engagement
	const engA = "019385a2-9100-7000-8cf0-ef0000000001" // attacker's engagement
	seedEngagementPlumbing(t, s.db, engB, admin.ID, "lead")
	seedEngagementPlumbing(t, s.db, engA, attacker.ID, "red")

	// The victim publishes a report in B whose rendered HTML carries a marker.
	const marker = "VICTIM-CONFIDENTIAL-8F3A"
	rec := s.post(BasePath+"/engagements/"+engB+"/reports", `{"title":"Victim Report"}`, adminCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create victim report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)

	rec = s.send(http.MethodPut,
		BasePath+"/engagements/"+engB+"/reports/"+rep.Id.String()+"/blocks",
		`{"blocks":[{"blockId":"rich_text","params":{"html":"<p>`+marker+`</p>"}}]}`,
		adminCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("save victim blocks: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.post(BasePath+"/engagements/"+engB+"/reports/"+rep.Id.String()+"/publish", `{}`, adminCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish victim report: %d\nbody: %s", rec.Code, rec.Body)
	}
	version := decodeJSON[gen.ReportVersion](t, rec)

	// The victim can read their own published version.
	rec = s.get(BasePath+"/engagements/"+engB+"/reports/"+rep.Id.String()+"/versions/"+version.Id.String()+"/html", adminCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), marker) {
		t.Fatalf("victim's own version html = %d, marker=%v\nbody: %s",
			rec.Code, strings.Contains(rec.Body.String(), marker), truncate(rec.Body.String()))
	}

	// The attacker — a member of A and NOT of B — must not be able to read the
	// victim's published version by supplying its versionId under A's path.
	const fakeReportID = "019385a2-9100-7000-8cf0-ef1111111111"
	rec = s.get(BasePath+"/engagements/"+engA+"/reports/"+fakeReportID+"/versions/"+version.Id.String()+"/html", attackerCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-engagement version html = %d, want 404 (the version belongs to another engagement)\nbody: %s",
			rec.Code, rec.Body)
	}
}
