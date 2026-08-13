package httpapi

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// engagementIDs returns the set of engagement ids present in a page.
func engagementIDs(page gen.EngagementPage) map[string]bool {
	ids := make(map[string]bool, len(page.Items))
	for _, e := range page.Items {
		ids[e.Id.String()] = true
	}
	return ids
}

// engagementOrder returns the ids in page order, newest first as the endpoint
// promises.
func engagementOrder(page gen.EngagementPage) []string {
	out := make([]string, 0, len(page.Items))
	for _, e := range page.Items {
		out = append(out, e.Id.String())
	}
	return out
}

// TestEngagementListIsMembershipScoped (BL-001) proves a platform member sees
// only their own engagements and an admin sees everything, and that a non-member
// still gets 404 on a concealed id.
func TestEngagementListIsMembershipScoped(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)

	server.seedUser(t) // alice, a platform administrator
	memberA := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member-a@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	memberB := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member-b@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	memberC := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member-c@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})

	const eaID = "019385a2-aaaa-7000-8cf0-ef0123456789"
	const ebID = "019385a2-bbbb-7000-8cf0-ef0123456789"
	const ecID = "019385a2-cccc-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, server.db, eaID, memberA.ID, "lead")
	seedEngagementPlumbing(t, server.db, ebID, memberB.ID, "lead")
	// A second seat shape: A is an observer of EC, proving the membership fence
	// is role-agnostic.
	seedEngagementPlumbing(t, server.db, ecID, memberA.ID, "observer")

	adminCookie := server.signIn(t)
	aCookie := sessionCookie(t, server.login(memberA.Email, testPassword))
	bCookie := sessionCookie(t, server.login(memberB.Email, testPassword))
	cCookie := sessionCookie(t, server.login(memberC.Email, testPassword))

	adminPage := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements", adminCookie))
	adminIDs := engagementIDs(adminPage)
	if !adminIDs[eaID] || !adminIDs[ebID] || !adminIDs[ecID] {
		t.Fatalf("admin list = %v, want %s, %s and %s", adminIDs, eaID, ebID, ecID)
	}

	aPage := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements", aCookie))
	if ids := engagementIDs(aPage); !ids[eaID] || !ids[ecID] || ids[ebID] {
		t.Fatalf("member A list = %v, want %s and %s but not %s", ids, eaID, ecID, ebID)
	}

	bPage := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements", bCookie))
	if ids := engagementIDs(bPage); !ids[ebID] || ids[eaID] {
		t.Fatalf("member B list = %v, want %s but not %s", ids, ebID, eaID)
	}

	// A member with no seats sees an empty list, not an error — same
	// concealment as 404 for a specific engagement they do not belong to.
	cPage := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements", cCookie))
	if len(cPage.Items) != 0 {
		t.Fatalf("member C with no seats sees %v, want an empty list", engagementIDs(cPage))
	}

	// Concealment survives: B asking for A's engagement directly is still 404,
	// identical to a missing id.
	if res := server.get(BasePath+"/engagements/"+eaID, bCookie); res.Code != http.StatusNotFound {
		t.Fatalf("member B GET /engagements/{EA} = %d, want 404\nbody: %s", res.Code, res.Body)
	}
}

// TestEngagementListPaginationAcrossMembershipFence proves cursor pagination
// neither skips nor leaks rows when invisible engagements sit between visible
// ones in creation order.
func TestEngagementListPaginationAcrossMembershipFence(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)

	memberA := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member-a@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	memberB := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member-b@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})

	const (
		e1 = "019385a2-0001-7000-8cf0-ef0123456789" // A, oldest
		e2 = "019385a2-0002-7000-8cf0-ef0123456789" // A
		e3 = "019385a2-0003-7000-8cf0-ef0123456789" // A, newest
		i1 = "019385a2-0101-7000-8cf0-ef0123456789" // B, newer than every A row
		i2 = "019385a2-0102-7000-8cf0-ef0123456789" // B, between e3 and e2
		i3 = "019385a2-0103-7000-8cf0-ef0123456789" // B, between e2 and e1
	)

	// created_at values are distinct so the cursor comparison is decided by
	// time alone, and invisible rows fall between A's visible ones.
	rows := []struct{ id, owner, createdAt string }{
		{i1, "b", "2026-01-01 10:04:00"},
		{e3, "a", "2026-01-01 10:03:00"},
		{i2, "b", "2026-01-01 10:02:00"},
		{e2, "a", "2026-01-01 10:01:00"},
		{i3, "b", "2026-01-01 10:00:00"},
		{e1, "a", "2026-01-01 09:59:00"},
	}
	for _, r := range rows {
		ownerID := memberA.ID
		if r.owner == "b" {
			ownerID = memberB.ID
		}
		seedEngagementAt(t, server.db, r.id, r.id, ownerID, r.createdAt)
		seedMembershipAt(t, server.db, r.id, ownerID, "lead")
	}

	aCookie := sessionCookie(t, server.login(memberA.Email, testPassword))

	page1 := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements?limit=2", aCookie))
	if got, want := engagementOrder(page1), []string{e3, e2}; !equalStrings(got, want) {
		t.Fatalf("page 1 = %v, want %v", got, want)
	}
	next, err := page1.NextCursor.Get()
	if err != nil || next != e2 {
		t.Fatalf("page 1 cursor = %q (err %v), want %s", next, err, e2)
	}

	page2 := decodeJSON[gen.EngagementPage](t, server.get(BasePath+"/engagements?limit=2&cursor="+next, aCookie))
	if got, want := engagementOrder(page2), []string{e1}; !equalStrings(got, want) {
		t.Fatalf("page 2 = %v, want %v", got, want)
	}
	if _, err := page2.NextCursor.Get(); err == nil {
		t.Fatalf("page 2 has a next cursor, want the last page")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// seedEngagementAt inserts an engagement with an explicit created_at so a test
// can control list ordering without relying on wall-clock microsecond ties.
func seedEngagementAt(t *testing.T, db store.Store, id, name, createdBy, createdAt string) {
	t.Helper()
	if err := db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement (id, name, client, description, status,
			 starts_on, ends_on, attack_version, mode, auto_reveal_on_start,
			 created_by, created_at, updated_at)
			 VALUES ($1, $2, 'Client Co', '', 'active',
			 '2025-01-01', '2025-12-31', '16.1', 'standard', false,
			 $3, $4, $4)`,
			id, name, createdBy, createdAt)
		return err
	}); err != nil {
		t.Fatalf("seed engagement %s: %v", id, err)
	}
}

// seedMembershipAt seats userID in engagementID.
func seedMembershipAt(t *testing.T, db store.Store, engagementID, userID, role string) {
	t.Helper()
	if err := db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_at)
			 VALUES ($1, $2, $3, NOW())`,
			engagementID, userID, role)
		return err
	}); err != nil {
		t.Fatalf("seed membership %s/%s: %v", engagementID, userID, err)
	}
}
