package engagement_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

type testDeps struct {
	engagements *storengagement.Engagements
	memberships *identity.Memberships
	users       *identity.Users
	svc         *engagement.Service
}

func newTestService(t *testing.T) *testDeps {
	t.Helper()

	db := storetest.Migrated(t)
	engagements := storengagement.NewEngagements(db)
	memberships := identity.NewMemberships(db)
	users := identity.NewUsers(db)

	svc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		AttackPin:   nil,
		Memberships: memberships,
		Users:       users,
	})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}

	return &testDeps{
		engagements: engagements,
		memberships: memberships,
		users:       users,
		svc:         svc,
	}
}

func (d *testDeps) createUser(t *testing.T, email string) identity.User {
	t.Helper()
	u, err := d.users.Create(context.Background(), identity.NewUser{
		Email:        email,
		DisplayName:  email,
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func (d *testDeps) createEngagement(t *testing.T) string {
	t.Helper()

	e, err := d.engagements.Create(context.Background(), storengagement.NewEngagement{
		Name:          "Test",
		AttackVersion: "15.1",
		Mode:          storengagement.EngagementModeStandard,
		CreatedBy:     "0192f1a0-0000-7000-8000-000000000000",
	})
	if err != nil {
		t.Fatalf("create engagement: %v", err)
	}
	return e.ID
}

func (d *testDeps) actor(user identity.User) authn.Subject {
	return authn.Subject{UserID: user.ID, Email: user.Email, DisplayName: user.DisplayName}
}

func TestAddMember(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")
	alice := d.createUser(t, "alice@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	m, err := d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID,
		Role:   "red",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Role != authz.EngagementRoleRed {
		t.Errorf("role = %q, want red", m.Role)
	}
	if m.AddedBy != lead.ID {
		t.Errorf("added_by = %q, want %q", m.AddedBy, lead.ID)
	}

	// Duplicate add → 409.
	_, err = d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID,
		Role:   "blue",
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("duplicate add error = %v, want conflict", err)
	}
}

func TestListMembers(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")
	alice := d.createUser(t, "alice@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	if _, err := d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID, Role: "red",
	}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	members, err := d.svc.ListMembers(context.Background(), engID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}

	roles := map[string]authz.EngagementRole{}
	for _, m := range members {
		roles[m.UserID] = m.Role
	}
	if roles[lead.ID] != authz.EngagementRoleLead {
		t.Errorf("lead role = %q", roles[lead.ID])
	}
	if roles[alice.ID] != authz.EngagementRoleRed {
		t.Errorf("alice role = %q", roles[alice.ID])
	}
}

func TestPatchMemberRole(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")
	alice := d.createUser(t, "alice@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID, Role: "red",
	}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	m, err := d.svc.PatchMemberRole(context.Background(), d.actor(lead), engID, alice.ID, "blue")
	if err != nil {
		t.Fatalf("PatchMemberRole: %v", err)
	}
	if m.Role != authz.EngagementRoleBlue {
		t.Errorf("role = %q, want blue", m.Role)
	}
}

func TestRemoveMember(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")
	alice := d.createUser(t, "alice@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID, Role: "red",
	}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	err = d.svc.RemoveMember(context.Background(), d.actor(lead), engID, alice.ID)
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, err := d.svc.ListMembers(context.Background(), engID)
	if err != nil {
		t.Fatalf("ListMembers after remove: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("got %d members after remove, want 1", len(members))
	}
}

func TestLastLeadCannotBeDemoted(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	_, err = d.svc.PatchMemberRole(context.Background(), d.actor(lead), engID, lead.ID, "red")
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("last lead demotion error = %v, want conflict", err)
	}
}

func TestLastLeadCannotBeRemoved(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	err = d.svc.RemoveMember(context.Background(), d.actor(lead), engID, lead.ID)
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("last lead removal error = %v, want conflict", err)
	}
}

func TestLeadCanSelfDemoteWhenAnotherLeadExists(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead1 := d.createUser(t, "lead1@test.com")
	lead2 := d.createUser(t, "lead2@test.com")

	for _, l := range []identity.User{lead1, lead2} {
		_, err := d.memberships.Add(context.Background(), identity.NewMembership{
			EngagementID: engID, UserID: l.ID, Role: authz.EngagementRoleLead, AddedBy: l.ID,
		})
		if err != nil {
			t.Fatalf("seed lead: %v", err)
		}
	}

	m, err := d.svc.PatchMemberRole(context.Background(), d.actor(lead1), engID, lead1.ID, "observer")
	if err != nil {
		t.Fatalf("self demotion with another lead: %v", err)
	}
	if m.Role != authz.EngagementRoleObserver {
		t.Errorf("role = %q, want observer", m.Role)
	}
}

func TestNonLeadCanBeRemoved(t *testing.T) {
	t.Parallel()

	d := newTestService(t)
	engID := d.createEngagement(t)
	lead := d.createUser(t, "lead@test.com")
	alice := d.createUser(t, "alice@test.com")

	_, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := d.svc.AddMember(context.Background(), d.actor(lead), engID, engagement.AddMemberInput{
		UserID: alice.ID, Role: "red",
	}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	err = d.svc.RemoveMember(context.Background(), d.actor(lead), engID, alice.ID)
	if err != nil {
		t.Fatalf("RemoveMember non-lead: %v", err)
	}
}
