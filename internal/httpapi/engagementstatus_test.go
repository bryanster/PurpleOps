package httpapi

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// POST /engagements/{id}/status is what the "Activate" / "Close" / "Archive"
// buttons on the overview and settings pages call. It answered 500 for every
// engagement that had so much as one member — which is every engagement, since
// creating one seats its lead — because app.engagement(status) was indexed and
// DuckDB rewrites an UPDATE of an indexed column into DELETE + INSERT, which
// the RESTRICT foreign keys pointing at the engagement then refuse. So these
// tests drive the route end to end, on an engagement with children, rather
// than the state machine alone.

// seedEngagementAtStatus seeds one engagement in the given status with userID
// seated in role, plus a scenario, so the row has both kinds of referencing
// child: a membership and a workbook row.
func seedEngagementAtStatus(t *testing.T, db *store.DB, engID, userID, role, status string) {
	t.Helper()

	if err := db.Write(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement (id, name, client, description, status,
			 starts_on, ends_on, attack_version, mode, auto_reveal_on_start,
			 created_by, created_at, updated_at)
			 VALUES ($1, 'Test Eng', 'Client Co', '', $3,
			 '2025-01-01', '2025-12-31', '16.1', 'standard', false,
			 $2, NOW(), NOW())`, engID, userID, status); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_at)
			 VALUES ($1, $2, $3, NOW())`, engID, userID, role); err != nil {
			return err
		}
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.scenario (id, engagement_id, ordinal, name, narrative,
			 source, threat_actor, source_ref, plan_id, created_at, updated_at)
			 VALUES ($1 || '-scn', $1, 1, 'Initial access', '',
			 'manual', '', '', '', NOW(), NOW())`, engID)
		return err
	}); err != nil {
		t.Fatalf("seed engagement at %q: %v", status, err)
	}
}

func TestEngagementStatusWalksTheLifecycleOverTheAPI(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123450001"
	seedEngagementAtStatus(t, s.db, engID, user.ID, "lead", "draft")

	for _, want := range []string{"active", "closed", "archived"} {
		rec := s.post(BasePath+"/engagements/"+engID+"/status", `{"status":"`+want+`"}`, cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("transition to %q: %d\nbody: %s", want, rec.Code, rec.Body)
		}
		if got := string(decodeJSON[gen.Engagement](t, rec).Status); got != want {
			t.Fatalf("response status = %q, want %q", got, want)
		}

		// And it is the stored row that moved, not just the response.
		read := s.get(BasePath+"/engagements/"+engID, cookie)
		if read.Code != http.StatusOK {
			t.Fatalf("read back after %q: %d\nbody: %s", want, read.Code, read.Body)
		}
		if got := string(decodeJSON[gen.Engagement](t, read).Status); got != want {
			t.Fatalf("stored status = %q, want %q", got, want)
		}
	}
}

// draft → closed is the one hop that skips a state, and the list page's "Close"
// button on a draft engagement is what takes it.
func TestEngagementStatusClosesADraft(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123450002"
	seedEngagementAtStatus(t, s.db, engID, user.ID, "lead", "draft")

	rec := s.post(BasePath+"/engagements/"+engID+"/status", `{"status":"closed"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("draft to closed: %d\nbody: %s", rec.Code, rec.Body)
	}
}

func TestEngagementStatusRefusesAnIllegalTransition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from string
		to   string
	}{
		{name: "draft skips straight to archived", from: "draft", to: "archived"},
		{name: "active cannot go back to draft", from: "active", to: "draft"},
		{name: "active cannot skip closed", from: "active", to: "archived"},
		{name: "closed cannot reopen", from: "closed", to: "active"},
		{name: "archived is terminal", from: "archived", to: "active"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newAuthServer(t)
			user := s.seedUser(t)
			cookie := s.signIn(t)

			engID := "019385a2-9000-7000-8cf0-ef012345001" + string(rune('0'+i))
			seedEngagementAtStatus(t, s.db, engID, user.ID, "lead", tc.from)

			rec := s.post(BasePath+"/engagements/"+engID+"/status", `{"status":"`+tc.to+`"}`, cookie)
			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body)
			}
			// The detail names both states, so the SPA can say what happened
			// rather than "conflict".
			detail := ""
			if d := decodeProblem(t, rec).Detail; d != nil {
				detail = *d
			}
			if !strings.Contains(detail, tc.from) || !strings.Contains(detail, tc.to) {
				t.Errorf("detail = %q, want it to name %q and %q", detail, tc.from, tc.to)
			}

			read := decodeJSON[gen.Engagement](t, s.get(BasePath+"/engagements/"+engID, cookie))
			if string(read.Status) != tc.from {
				t.Errorf("stored status = %q, want it left at %q", read.Status, tc.from)
			}
		})
	}
}

func TestEngagementStatusRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123450003"
	seedEngagementAtStatus(t, s.db, engID, user.ID, "lead", "draft")

	rec := s.post(BasePath+"/engagements/"+engID+"/status", `{"status":"finished"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body)
	}
}

// engagement.manage is lead-only, so the other seats cannot move the status
// even though they can read the engagement.
func TestEngagementStatusIsLeadOnly(t *testing.T) {
	t.Parallel()

	for i, role := range []string{"red", "blue", "observer"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()

			s := newAuthServer(t)
			user := s.seedUser(t, func(u *identity.NewUser) { u.PlatformRole = authz.PlatformRoleMember })
			cookie := s.signIn(t)

			engID := "019385a2-9000-7000-8cf0-ef012345002" + string(rune('0'+i))
			seedEngagementAtStatus(t, s.db, engID, user.ID, role, "draft")

			rec := s.post(BasePath+"/engagements/"+engID+"/status", `{"status":"active"}`, cookie)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403\nbody: %s", rec.Code, rec.Body)
			}
		})
	}
}
