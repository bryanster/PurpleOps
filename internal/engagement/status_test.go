package engagement_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storactivity "github.com/bryanster/blacklight/internal/store/activity"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The status state machine, from the service down to the row: draft → active →
// closed → archived, plus draft → closed, and nothing else.
//
// These go through a real database rather than a fake repository because the
// bug they exist for was in the schema — an index on app.engagement(status)
// turned every transition into a foreign key violation once the engagement had
// a member. A service test over a stub store would have stayed green through
// all of it.

// statusDeps is a service with an activity log attached, so the entry a
// transition writes can be read back.
type statusDeps struct {
	*testDeps
	entries *storactivity.Entries
}

func newStatusService(t *testing.T) *statusDeps {
	t.Helper()

	db := storetest.Migrated(t)
	engagements := storengagement.NewEngagements(db)
	memberships := identity.NewMemberships(db)
	users := identity.NewUsers(db)
	scenarios := storengagement.NewScenarios(db)
	entries := storactivity.New(db)

	svc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		Memberships: memberships,
		Scenarios:   scenarios,
		Users:       users,
		Activity:    events.New(entries),
	})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}

	return &statusDeps{
		testDeps: &testDeps{
			engagements: engagements,
			memberships: memberships,
			users:       users,
			svc:         svc,
		},
		entries: entries,
	}
}

// seatedEngagement is an engagement with its lead seated — the shape every
// engagement the API creates has, and the one the foreign key tripped over.
func (d *statusDeps) seatedEngagement(t *testing.T) (string, identity.User) {
	t.Helper()

	lead := d.createUser(t, "lead@test.com")
	engID := d.createEngagement(t)
	if _, err := d.memberships.Add(context.Background(), identity.NewMembership{
		EngagementID: engID, UserID: lead.ID, Role: authz.EngagementRoleLead, AddedBy: lead.ID,
	}); err != nil {
		t.Fatalf("seat the lead: %v", err)
	}
	return engID, lead
}

func TestSetStatusFollowsTheStateMachine(t *testing.T) {
	t.Parallel()

	// Every hop the machine allows, each from its own starting state.
	cases := []struct {
		name string
		from []storengagement.EngagementStatus // hops taken to reach the start
		to   storengagement.EngagementStatus
	}{
		{name: "draft to active", to: storengagement.EngagementStatusActive},
		{name: "draft to closed", to: storengagement.EngagementStatusClosed},
		{
			name: "active to closed",
			from: []storengagement.EngagementStatus{storengagement.EngagementStatusActive},
			to:   storengagement.EngagementStatusClosed,
		},
		{
			name: "closed to archived",
			from: []storengagement.EngagementStatus{
				storengagement.EngagementStatusActive,
				storengagement.EngagementStatusClosed,
			},
			to: storengagement.EngagementStatusArchived,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newStatusService(t)
			engID, lead := d.seatedEngagement(t)
			actor := d.actor(lead)

			for _, hop := range tc.from {
				if _, err := d.svc.SetStatus(context.Background(), actor, engID, hop); err != nil {
					t.Fatalf("reaching %q: %v", hop, err)
				}
			}

			got, err := d.svc.SetStatus(context.Background(), actor, engID, tc.to)
			if err != nil {
				t.Fatalf("SetStatus(%q): %v", tc.to, err)
			}
			if got.Status != tc.to {
				t.Errorf("returned status = %q, want %q", got.Status, tc.to)
			}

			stored, err := d.engagements.ByID(context.Background(), engID)
			if err != nil {
				t.Fatalf("ByID: %v", err)
			}
			if stored.Status != tc.to {
				t.Errorf("stored status = %q, want %q", stored.Status, tc.to)
			}
		})
	}
}

func TestSetStatusRejectsIllegalTransitions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from []storengagement.EngagementStatus
		to   storengagement.EngagementStatus
	}{
		{name: "draft skips straight to archived", to: storengagement.EngagementStatusArchived},
		{
			name: "active cannot go back to draft",
			from: []storengagement.EngagementStatus{storengagement.EngagementStatusActive},
			to:   storengagement.EngagementStatusDraft,
		},
		{
			name: "active cannot skip closed",
			from: []storengagement.EngagementStatus{storengagement.EngagementStatusActive},
			to:   storengagement.EngagementStatusArchived,
		},
		{
			name: "closed cannot reopen",
			from: []storengagement.EngagementStatus{storengagement.EngagementStatusClosed},
			to:   storengagement.EngagementStatusActive,
		},
		{
			name: "archived is terminal",
			from: []storengagement.EngagementStatus{
				storengagement.EngagementStatusClosed,
				storengagement.EngagementStatusArchived,
			},
			to: storengagement.EngagementStatusActive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newStatusService(t)
			engID, lead := d.seatedEngagement(t)
			actor := d.actor(lead)

			for _, hop := range tc.from {
				if _, err := d.svc.SetStatus(context.Background(), actor, engID, hop); err != nil {
					t.Fatalf("reaching %q: %v", hop, err)
				}
			}
			before, err := d.engagements.ByID(context.Background(), engID)
			if err != nil {
				t.Fatalf("ByID: %v", err)
			}

			if _, err := d.svc.SetStatus(context.Background(), actor, engID, tc.to); !errors.Is(err, apierr.ErrConflict) {
				t.Fatalf("SetStatus(%q) error = %v, want conflict", tc.to, err)
			}

			after, err := d.engagements.ByID(context.Background(), engID)
			if err != nil {
				t.Fatalf("ByID: %v", err)
			}
			if after.Status != before.Status {
				t.Errorf("status moved to %q on a refused transition, want %q", after.Status, before.Status)
			}
		})
	}
}

func TestSetStatusRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	d := newStatusService(t)
	engID, lead := d.seatedEngagement(t)

	_, err := d.svc.SetStatus(context.Background(), d.actor(lead), engID, "finished")
	if !errors.Is(err, apierr.ErrValidation) {
		t.Fatalf("error = %v, want validation", err)
	}
}

// Setting the status it already has is a no-op rather than a conflict, so a
// double-clicked button does not answer 409.
func TestSetStatusToTheCurrentStatusIsANoOp(t *testing.T) {
	t.Parallel()

	d := newStatusService(t)
	engID, lead := d.seatedEngagement(t)
	actor := d.actor(lead)

	if _, err := d.svc.SetStatus(context.Background(), actor, engID, storengagement.EngagementStatusActive); err != nil {
		t.Fatalf("first SetStatus: %v", err)
	}
	got, err := d.svc.SetStatus(context.Background(), actor, engID, storengagement.EngagementStatusActive)
	if err != nil {
		t.Fatalf("repeat SetStatus: %v", err)
	}
	if got.Status != storengagement.EngagementStatusActive {
		t.Errorf("status = %q, want active", got.Status)
	}
}

func TestSetStatusRecordsActivity(t *testing.T) {
	t.Parallel()

	d := newStatusService(t)
	engID, lead := d.seatedEngagement(t)

	if _, err := d.svc.SetStatus(context.Background(), d.actor(lead), engID,
		storengagement.EngagementStatusActive); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	rows, _, err := d.entries.List(context.Background(), storactivity.ListFilter{
		ScopeEngagement: engID,
		Verb:            string(events.VerbEngagementStatusChanged),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d status-changed entries, want 1", len(rows))
	}
	if rows[0].ActorID != lead.ID {
		t.Errorf("actor = %q, want %q", rows[0].ActorID, lead.ID)
	}
	if got := string(rows[0].Delta); got != `{"from":"draft","to":"active"}` {
		t.Errorf("delta = %s, want the from/to pair", got)
	}
}

// A refused transition must not leave an activity row behind.
func TestRefusedTransitionRecordsNoActivity(t *testing.T) {
	t.Parallel()

	d := newStatusService(t)
	engID, lead := d.seatedEngagement(t)

	if _, err := d.svc.SetStatus(context.Background(), d.actor(lead), engID,
		storengagement.EngagementStatusArchived); !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("error = %v, want conflict", err)
	}

	rows, _, err := d.entries.List(context.Background(), storactivity.ListFilter{
		ScopeEngagement: engID,
		Verb:            string(events.VerbEngagementStatusChanged),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d status-changed entries, want none", len(rows))
	}
}
