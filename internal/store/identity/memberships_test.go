package identity_test

import (
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestMembershipRoundTrips(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	lead := mustCreateUser(t, r, "lead@example.com")
	alice := mustCreateUser(t, r, "alice@example.com")

	created, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1",
		UserID:       alice.ID,
		Role:         identity.EngagementRoleRed,
		AddedBy:      lead.ID,
	})
	if err != nil {
		t.Fatalf("Add() = %v, want nil", err)
	}
	if created.Role != identity.EngagementRoleRed || created.AddedBy != lead.ID {
		t.Errorf("Add() returned %+v, want red, added by %q", created, lead.ID)
	}
	if created.AddedAt.IsZero() {
		t.Error("AddedAt is the zero time")
	}

	found, err := r.memberships.Get(t.Context(), "engagement-1", alice.ID)
	if err != nil {
		t.Fatalf("Get() = %v, want the membership", err)
	}
	if found != created {
		t.Errorf("Get() = %+v, want %+v", found, created)
	}
}

// TestAPlatformRoleIsNotAnEngagementRole is the distinction PLAN.md §4 exists
// to keep: a platform administrator has no engagement role until somebody adds
// them to one, and being a lead says nothing about the installation. Reading
// one as the other is the v1 defect this schema is shaped to prevent.
func TestAPlatformRoleIsNotAnEngagementRole(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	admin, err := r.users.Create(t.Context(), identity.NewUser{
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		PlatformRole: identity.PlatformRoleAdmin,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.memberships.Get(t.Context(), "engagement-1", admin.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Get() = %v for a platform admin nobody added, want not found", err)
	}

	// And an engagement lead is still an ordinary member of the installation.
	member := mustCreateUser(t, r, "lead@example.com")
	if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1", UserID: member.ID, Role: identity.EngagementRoleLead,
	}); err != nil {
		t.Fatal(err)
	}
	found, err := r.users.ByID(t.Context(), member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.PlatformRole != identity.PlatformRoleMember {
		t.Errorf("PlatformRole = %q after being made an engagement lead, want %q",
			found.PlatformRole, identity.PlatformRoleMember)
	}
}

func TestAddRefusesSomebodyWhoIsAlreadyAMember(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1", UserID: alice.ID, Role: identity.EngagementRoleRed,
	}); err != nil {
		t.Fatal(err)
	}

	// Adding them as blue must not quietly switch sides mid-engagement.
	_, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1", UserID: alice.ID, Role: identity.EngagementRoleBlue,
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("Add() = %v, want a conflict", err)
	}

	found, err := r.memberships.Get(t.Context(), "engagement-1", alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Role != identity.EngagementRoleRed {
		t.Errorf("Role = %q, want the original %q", found.Role, identity.EngagementRoleRed)
	}
}

func TestOnePersonCanBeInSeveralEngagements(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")

	for engagement, role := range map[string]identity.EngagementRole{
		"engagement-1": identity.EngagementRoleRed,
		"engagement-2": identity.EngagementRoleBlue,
	} {
		if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
			EngagementID: engagement, UserID: alice.ID, Role: role,
		}); err != nil {
			t.Fatalf("Add(%s) = %v, want nil", engagement, err)
		}
	}

	found, err := r.memberships.ListByUser(t.Context(), alice.ID)
	if err != nil {
		t.Fatalf("ListByUser() = %v, want nil", err)
	}
	if len(found) != 2 {
		t.Fatalf("ListByUser() returned %d, want 2", len(found))
	}
}

func TestSetRoleChangesOnlyTheRole(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	lead := mustCreateUser(t, r, "lead@example.com")
	alice := mustCreateUser(t, r, "alice@example.com")
	created, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1", UserID: alice.ID,
		Role: identity.EngagementRoleObserver, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := r.memberships.SetRole(t.Context(), "engagement-1", alice.ID, identity.EngagementRoleBlue)
	if err != nil {
		t.Fatalf("SetRole() = %v, want nil", err)
	}
	if updated.Role != identity.EngagementRoleBlue {
		t.Errorf("Role = %q, want %q", updated.Role, identity.EngagementRoleBlue)
	}
	// How they got in is not rewritten by a later change to what they may do.
	if updated.AddedBy != created.AddedBy || !updated.AddedAt.Equal(created.AddedAt) {
		t.Errorf("SetRole() rewrote the provenance: added by %q at %s, was %q at %s",
			updated.AddedBy, updated.AddedAt, created.AddedBy, created.AddedAt)
	}

	if _, err := r.memberships.SetRole(t.Context(), "engagement-1", "no-such-user",
		identity.EngagementRoleRed); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("SetRole() for a non-member = %v, want not found", err)
	}
}

func TestRemoveTakesSomebodyOutOfOneEngagementOnly(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	for _, engagement := range []string{"engagement-1", "engagement-2"} {
		if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
			EngagementID: engagement, UserID: alice.ID, Role: identity.EngagementRoleRed,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.memberships.Remove(t.Context(), "engagement-1", alice.ID); err != nil {
		t.Fatalf("Remove() = %v, want nil", err)
	}
	if _, err := r.memberships.Get(t.Context(), "engagement-1", alice.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Get() = %v after Remove(), want not found", err)
	}
	if _, err := r.memberships.Get(t.Context(), "engagement-2", alice.ID); err != nil {
		t.Errorf("Get(engagement-2) = %v; removing one membership removed another", err)
	}

	if err := r.memberships.Remove(t.Context(), "engagement-1", alice.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("second Remove() = %v, want not found", err)
	}
}

func TestListByEngagementReturnsTheWholeTeam(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	roles := map[string]identity.EngagementRole{
		"lead@example.com":     identity.EngagementRoleLead,
		"red@example.com":      identity.EngagementRoleRed,
		"blue@example.com":     identity.EngagementRoleBlue,
		"observer@example.com": identity.EngagementRoleObserver,
	}
	byID := map[string]identity.EngagementRole{}
	for email, role := range roles {
		user := mustCreateUser(t, r, email)
		byID[user.ID] = role
		if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
			EngagementID: "engagement-1", UserID: user.ID, Role: role,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Somebody in a different engagement, who must not appear.
	outsider := mustCreateUser(t, r, "outsider@example.com")
	if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-2", UserID: outsider.ID, Role: identity.EngagementRoleRed,
	}); err != nil {
		t.Fatal(err)
	}

	found, err := r.memberships.ListByEngagement(t.Context(), "engagement-1")
	if err != nil {
		t.Fatalf("ListByEngagement() = %v, want nil", err)
	}
	if len(found) != len(roles) {
		t.Fatalf("ListByEngagement() returned %d members, want %d", len(found), len(roles))
	}
	for _, m := range found {
		if want, ok := byID[m.UserID]; !ok {
			t.Errorf("%q is not a member of this engagement", m.UserID)
		} else if m.Role != want {
			t.Errorf("%q has role %q, want %q", m.UserID, m.Role, want)
		}
	}
}

func TestAMembershipMustBelongToARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	_, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "engagement-1", UserID: "no-such-user", Role: identity.EngagementRoleRed,
	})
	if err == nil {
		t.Fatal("a membership was created for a user who does not exist")
	}
}

// TestAnUnknownEngagementIsAccepted documents the one thing this schema does
// not enforce, so that a reader meeting it does not think it is an oversight:
// app.engagement arrives with M3, and until it does nothing checks the
// identifier. See the comment in 0002_identity.sql.
func TestAnUnknownEngagementIsAccepted(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.memberships.Add(t.Context(), identity.NewMembership{
		EngagementID: "an-engagement-that-does-not-exist", UserID: alice.ID,
		Role: identity.EngagementRoleRed,
	}); err != nil {
		t.Errorf("Add() = %v; until M3 there is nothing to check the engagement against", err)
	}
}
