package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store/activity"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestPlatformActivityIsAdminOnly(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	admin := server.seedUser(t) // admin by default

	// A second account that is only a platform member.
	member, err := identity.NewUsers(server.db).Create(t.Context(), identity.NewUser{
		Email:        "member@example.com",
		DisplayName:  "Member",
		PasswordHash: testPasswordHash(),
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	log := events.New(activity.New(server.db))
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(context.Background(), tx, events.Entry{
			ActorID: admin.ID, Verb: events.VerbSessionLogin,
			ObjectType: events.ObjectSession, ObjectID: "s1",
			Delta: events.Delta(map[string]any{"ip": "1.2.3.4"}),
		})
	}); err != nil {
		t.Fatal(err)
	}

	// Admin can read.
	adminCookie := server.signIn(t)
	res := server.get(BasePath+"/activity", adminCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("admin status = %d body=%s", res.Code, res.Body)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	// Login itself also wrote session.login rows; at least the seeded one is present.
	if len(page.Items) < 1 {
		t.Fatalf("admin items = %d, want >= 1", len(page.Items))
	}

	// Member signs in and is refused.
	memberRes := server.login(member.Email, testPassword)
	if memberRes.Code != http.StatusOK {
		t.Fatalf("member login = %d body=%s", memberRes.Code, memberRes.Body)
	}
	memberCookie := sessionCookie(t, memberRes)
	res = server.get(BasePath+"/activity", memberCookie)
	if res.Code != http.StatusForbidden {
		t.Fatalf("member status = %d, want 403 body=%s", res.Code, res.Body)
	}
}

func TestEngagementActivityRequiresMembership(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	member := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "red@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	outsider, err := identity.NewUsers(server.db).Create(t.Context(), identity.NewUser{
		Email:        "out@example.com",
		DisplayName:  "Out",
		PasswordHash: testPasswordHash(),
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	engID := "01900000-0000-7000-8000-000000000001"
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement (id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			VALUES (?, 'test', 'test', '', 'draft', '2026-01-01', '2026-06-01', '15.1', 'standard', false, 'u1', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
			engID,
		)
		return err
	}); err != nil {
		t.Fatalf("creating engagement: %v", err)
	}
	if _, err := identity.NewMemberships(server.db).Add(t.Context(), identity.NewMembership{
		EngagementID: engID, UserID: member.ID, Role: authz.EngagementRoleRed,
	}); err != nil {
		t.Fatal(err)
	}

	log := events.New(activity.New(server.db))
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(context.Background(), tx, events.Entry{
			EngagementID: engID, ActorID: member.ID,
			Verb: "comment.created", ObjectType: "comment", ObjectID: "c1",
		})
	}); err != nil {
		t.Fatal(err)
	}

	// Member of the engagement can read.
	memberCookie := sessionCookie(t, server.login(member.Email, testPassword))
	res := server.get(BasePath+"/engagements/"+engID+"/activity", memberCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("member status = %d body=%s", res.Code, res.Body)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}

	// Non-member gets 404 (concealed), not 403.
	outCookie := sessionCookie(t, server.login(outsider.Email, testPassword))
	res = server.get(BasePath+"/engagements/"+engID+"/activity", outCookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("outsider status = %d, want 404 body=%s", res.Code, res.Body)
	}
}

func TestActivityPaginationOverHTTP(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	admin := server.seedUser(t)

	log := events.New(activity.New(server.db))
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		for i := range 5 {
			if err := log.Record(context.Background(), tx, events.Entry{
				ActorID: admin.ID, Verb: events.VerbTokenCreated,
				ObjectType: events.ObjectToken, ObjectID: "t",
				At: base.Add(time.Duration(i) * time.Second),
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cookie := server.signIn(t)
	res := server.get(BasePath+"/activity?limit=2&verb=token.created", cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body)
	}
	var page1 struct {
		Items      []map[string]any `json:"items"`
		NextCursor *string          `json:"nextCursor"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("page1 items = %d", len(page1.Items))
	}
	if page1.NextCursor == nil || *page1.NextCursor == "" {
		t.Fatal("expected nextCursor")
	}

	res = server.get(BasePath+"/activity?limit=2&verb=token.created&cursor="+*page1.NextCursor, cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("page2 status = %d body=%s", res.Code, res.Body)
	}
	var page2 struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("page2 items = %d", len(page2.Items))
	}
	if page1.Items[0]["id"] == page2.Items[0]["id"] {
		t.Fatal("pages overlap")
	}
}

func TestLoginWritesActivityWithoutSecrets(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)

	// Failed login.
	fail := server.login(testEmail, "wrong password entirely")
	if fail.Code != http.StatusUnauthorized {
		t.Fatalf("failed login = %d", fail.Code)
	}

	// Successful login.
	_ = server.signIn(t)

	rows, _, err := activity.New(server.db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true, Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}

	var sawLogin, sawFail bool
	blob, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{testPassword, "wrong password", "$argon2"} {
		if strings.Contains(string(blob), banned) {
			t.Errorf("activity leaked %q", banned)
		}
	}
	for _, row := range rows {
		switch row.Verb {
		case string(events.VerbSessionLogin):
			sawLogin = true
		case string(events.VerbSessionLoginFailed):
			sawFail = true
			if !strings.Contains(string(row.Delta), testEmail) {
				t.Errorf("failed login delta missing email: %s", row.Delta)
			}
		}
	}
	if !sawLogin {
		t.Error("missing session.login")
	}
	if !sawFail {
		t.Error("missing session.login_failed")
	}
}

func TestBlindEngagementActivityFiltersUnrevealedSteps(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	red := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "red-lead@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	blue, err := identity.NewUsers(server.db).Create(t.Context(), identity.NewUser{
		Email:        "blue@example.com",
		DisplayName:  "Blue",
		PasswordHash: testPasswordHash(),
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	engID := "01900000-0000-7000-8000-000000000002"
	scenarioID := "01900000-0000-7000-8000-000000000003"
	stepID := "01900000-0000-7000-8000-000000000004"

	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement (id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			VALUES (?, 'blind-test', 'test', '', 'draft', '2026-01-01', '2026-06-01', '15.1', 'blind', false, 'u1', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
			engID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(t.Context(),
			`INSERT INTO app.scenario (id, engagement_id, ordinal, name, narrative, source, threat_actor, source_ref, plan_id, created_at, updated_at)
			VALUES (?, ?, 0, 'scenario-1', '', 'manual', '', '', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
			scenarioID, engID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(t.Context(),
			`INSERT INTO app.step (id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id, tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope, attack_version, revealed_at, created_at, updated_at)
			VALUES (?, ?, 0, 'step-1', '', 'T1003', '', '', '{}', '', '', '[]', '[]', '15.1', NULL, '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
			stepID, scenarioID,
		)
		return err
	}); err != nil {
		t.Fatalf("creating blind engagement: %v", err)
	}

	// Add memberships: red as lead, blue as blue.
	memberships := identity.NewMemberships(server.db)
	if _, err := memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: engID, UserID: red.ID, Role: authz.EngagementRoleLead,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: engID, UserID: blue.ID, Role: authz.EngagementRoleBlue,
	}); err != nil {
		t.Fatal(err)
	}

	// Record a step.created activity entry.
	log := events.New(activity.New(server.db))
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(context.Background(), tx, events.Entry{
			EngagementID: engID, ActorID: red.ID,
			Verb: "step.created", ObjectType: "step", ObjectID: stepID,
		})
	}); err != nil {
		t.Fatal(err)
	}

	// Blue sees NO activity — the step is unrevealed.
	blueCookie := sessionCookie(t, server.login(blue.Email, testPassword))
	res := server.get(BasePath+"/engagements/"+engID+"/activity", blueCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("blue status = %d body=%s", res.Code, res.Body)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("blue saw %d items, want 0 — unrevealed step.created must be withheld", len(page.Items))
	}

	// Lead (red) sees the activity.
	redCookie := sessionCookie(t, server.login(red.Email, testPassword))
	res = server.get(BasePath+"/engagements/"+engID+"/activity", redCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("lead status = %d body=%s", res.Code, res.Body)
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("lead saw %d items, want 1 — revealed step should be visible to lead", len(page.Items))
	}
}
