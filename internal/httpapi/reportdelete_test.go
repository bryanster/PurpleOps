package httpapi

import (
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// The two things the reports list page does that had no handler-level test:
// delete a row, and show how many blocks each row holds. Both were broken in
// the product — DELETE returned a 500 for any report with a block, and the
// list carried no count for the page to read.

// TestDeleteReportWithBlocks is the Delete button in the reports list. The
// store deletes the block rows and the report row in separate committed
// transactions because DuckDB will not accept both in one; this asserts the
// endpoint the button calls actually answers 204 and the report goes away.
func TestDeleteReportWithBlocks(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9100-7000-8cf0-ef0123456001"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")
	reportsPath := BasePath + "/engagements/" + engID + "/reports"

	rec := s.post(reportsPath, `{"title":"Doomed"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)
	base := reportsPath + "/" + rep.Id.String()

	rec = s.send(http.MethodPut, base+"/blocks",
		`{"blocks":[{"blockId":"cover","params":{}},{"blockId":"rich_text","params":{"html":"<p>x</p>"}}]}`,
		cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("put blocks: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.del(base, "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete report: %d, want 204\nbody: %s", rec.Code, rec.Body)
	}

	if rec := s.get(base, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: %d, want 404", rec.Code)
	}
	rec = s.get(reportsPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list reports: %d\nbody: %s", rec.Code, rec.Body)
	}
	if items := decodeJSON[[]gen.Report](t, rec); len(items) != 0 {
		t.Errorf("list has %d reports after delete, want 0", len(items))
	}
}

// TestDeleteReportNotFound keeps the answer for an id that never existed a 404
// rather than the 204 a blind `DELETE … WHERE` would give.
func TestDeleteReportNotFound(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9100-7000-8cf0-ef0123456002"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	rec := s.del(BasePath+"/engagements/"+engID+
		"/reports/019385a2-9100-7000-8cf0-ef012345dead", "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown report: %d, want 404\nbody: %s", rec.Code, rec.Body)
	}
}

// TestListReportsCarriesBlockCount is the "always 0 blocks" bug. The list
// response omits `blocks` by design, so the count has to travel as its own
// field or the page has nothing to render.
func TestListReportsCarriesBlockCount(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9100-7000-8cf0-ef0123456003"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")
	reportsPath := BasePath + "/engagements/" + engID + "/reports"

	rec := s.post(reportsPath, `{"title":"Three blocks"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	filled := decodeJSON[gen.Report](t, rec)
	if filled.BlockCount != 0 {
		t.Errorf("new report blockCount = %d, want 0", filled.BlockCount)
	}

	rec = s.post(reportsPath, `{"title":"Still empty"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second report: %d\nbody: %s", rec.Code, rec.Body)
	}
	emptyReport := decodeJSON[gen.Report](t, rec)

	base := reportsPath + "/" + filled.Id.String()
	rec = s.send(http.MethodPut, base+"/blocks",
		`{"blocks":[
			{"blockId":"cover","params":{}},
			{"blockId":"rich_text","params":{"html":"<p>x</p>"}},
			{"blockId":"findings_backlog","params":{}}
		]}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("put blocks: %d\nbody: %s", rec.Code, rec.Body)
	}
	if saved := decodeJSON[gen.Report](t, rec); saved.BlockCount != 3 {
		t.Errorf("blockCount after save = %d, want 3", saved.BlockCount)
	}

	rec = s.get(reportsPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list reports: %d\nbody: %s", rec.Code, rec.Body)
	}
	counts := map[string]int{}
	for _, item := range decodeJSON[[]gen.Report](t, rec) {
		counts[item.Id.String()] = item.BlockCount
	}
	if got := counts[filled.Id.String()]; got != 3 {
		t.Errorf("list blockCount for the three-block report = %d, want 3", got)
	}
	if got := counts[emptyReport.Id.String()]; got != 0 {
		t.Errorf("list blockCount for the empty report = %d, want 0", got)
	}

	// A patch that does not touch the blocks must not zero the count either —
	// the page re-reads the list after a rename.
	rec = s.send(http.MethodPatch, base, `{"title":"Renamed"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch report: %d\nbody: %s", rec.Code, rec.Body)
	}
	if patched := decodeJSON[gen.Report](t, rec); patched.BlockCount != 3 {
		t.Errorf("blockCount after a title patch = %d, want 3", patched.BlockCount)
	}
}
