package httpapi

import (
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// DELETE /engagements/{id} is what the settings page's "Delete engagement"
// button calls. It answered 500 for every engagement — the schema could not
// delete an engagement row at all (0016_app_domain explains why) — so these
// tests drive the route end to end rather than the store alone.

func TestDeleteEngagementEmptiesItAndAnswers204(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123456111"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	// Enough of a workbook that the delete has to walk the graph rather than
	// drop a lone row: a scenario with a step, a finding, a report and a
	// report template.
	rec := s.post(BasePath+"/engagements/"+engID+"/scenarios", `{"name":"Initial access"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create scenario: %d\nbody: %s", rec.Code, rec.Body)
	}
	scenario := decodeJSON[gen.Scenario](t, rec)

	rec = s.post(BasePath+"/engagements/"+engID+"/scenarios/"+scenario.Id.String()+"/steps",
		`{"name":"Phish"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create step: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.post(BasePath+"/engagements/"+engID+"/findings",
		`{"title":"Weak EDR","description":"Endpoint agent missed the payload.","severity":"high"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create finding: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Final report"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.post(BasePath+"/engagements/"+engID+"/report-templates", `{"name":"House style"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create template: %d\nbody: %s", rec.Code, rec.Body)
	}

	// The button.
	rec = s.send(http.MethodDelete, BasePath+"/engagements/"+engID, "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete engagement: %d, want 204\nbody: %s", rec.Code, rec.Body)
	}

	if rec := s.get(BasePath+"/engagements/"+engID, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: %d, want 404\nbody: %s", rec.Code, rec.Body)
	}

	// And it is gone from the list the page navigates back to, not merely
	// hidden behind a 404 on the detail route.
	rec = s.get(BasePath+"/engagements", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list engagements: %d\nbody: %s", rec.Code, rec.Body)
	}
	for _, e := range decodeJSON[gen.EngagementPage](t, rec).Items {
		if e.Id.String() == engID {
			t.Error("the deleted engagement is still in the engagement list")
		}
	}
}

// A second delete of the same engagement is a 404, not a 500 — the double
// click, and the retry after a delete that failed partway.
func TestDeleteEngagementTwiceIs404(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123456222"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	if rec := s.send(http.MethodDelete, BasePath+"/engagements/"+engID, "", cookie); rec.Code != http.StatusNoContent {
		t.Fatalf("first delete: %d, want 204\nbody: %s", rec.Code, rec.Body)
	}
	if rec := s.send(http.MethodDelete, BasePath+"/engagements/"+engID, "", cookie); rec.Code != http.StatusNotFound {
		t.Errorf("second delete: %d, want 404\nbody: %s", rec.Code, rec.Body)
	}
}

// Deleting one engagement must not take another with it. The delete works
// through sub-selects keyed on the engagement id, and a missing predicate in
// any of them would empty the neighbours' rows too.
func TestDeleteEngagementLeavesTheOtherOneWorking(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	doomed := "019385a2-9000-7000-8cf0-ef0123456333"
	keep := "019385a2-9000-7000-8cf0-ef0123456444"
	seedEngagementPlumbing(t, s.db, doomed, user.ID, "lead")
	seedEngagementPlumbing(t, s.db, keep, user.ID, "lead")

	for _, id := range []string{doomed, keep} {
		rec := s.post(BasePath+"/engagements/"+id+"/scenarios", `{"name":"Initial access"}`, cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create scenario on %s: %d\nbody: %s", id, rec.Code, rec.Body)
		}
		rec = s.post(BasePath+"/engagements/"+id+"/reports", `{"title":"Final report"}`, cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create report on %s: %d\nbody: %s", id, rec.Code, rec.Body)
		}
	}

	if rec := s.send(http.MethodDelete, BasePath+"/engagements/"+doomed, "", cookie); rec.Code != http.StatusNoContent {
		t.Fatalf("delete engagement: %d, want 204\nbody: %s", rec.Code, rec.Body)
	}

	if rec := s.get(BasePath+"/engagements/"+keep, cookie); rec.Code != http.StatusOK {
		t.Fatalf("the surviving engagement reads %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	rec := s.get(BasePath+"/engagements/"+keep+"/scenarios", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("the surviving engagement's scenarios read %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	if items := decodeJSON[gen.ScenarioList](t, rec).Items; len(items) != 1 {
		t.Errorf("the surviving engagement has %d scenario(s), want 1", len(items))
	}
	rec = s.get(BasePath+"/engagements/"+keep+"/reports", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("the surviving engagement's reports read %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
}

// A red-team member is on the engagement but is not its lead, and may not
// delete it. Now that the delete works, the check that it is refused for the
// wrong seat matters more than it did when nobody could delete anything.
//
// The seat has to belong to a platform member: the default seeded user is a
// platform admin, who is allowed to delete any engagement whatever their seat
// on it.
func TestDeleteEngagementRefusedForNonLead(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	s.seedUser(t)
	red := s.seedUser(t, func(in *identity.NewUser) {
		in.Email = "red@example.com"
		in.PlatformRole = authz.PlatformRoleMember
	})
	rec := s.login(red.Email, testPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("red login: %d", rec.Code)
	}
	redCookie := sessionCookie(t, rec)

	engID := "019385a2-9000-7000-8cf0-ef0123456555"
	seedEngagementPlumbing(t, s.db, engID, red.ID, "red")

	if rec := s.send(http.MethodDelete, BasePath+"/engagements/"+engID, "", redCookie); rec.Code != http.StatusForbidden {
		t.Fatalf("delete as red: %d, want 403\nbody: %s", rec.Code, rec.Body)
	}
	if rec := s.get(BasePath+"/engagements/"+engID, redCookie); rec.Code != http.StatusOK {
		t.Errorf("engagement reads %d after a refused delete, want 200\nbody: %s", rec.Code, rec.Body)
	}
}
