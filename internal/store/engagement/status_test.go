package engagement_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// Setting an engagement's status is an UPDATE of a column on a table that other
// tables point at with RESTRICT foreign keys. DuckDB rewrites an UPDATE that
// touches an *indexed* column into DELETE + INSERT, and checks the delete half
// against those foreign keys, so an index on app.engagement(status) made every
// transition fail with
//
//	Constraint Error: Violates foreign key constraint because key
//	"engagement_id: …" is still referenced by a foreign key in a different table
//
// on any engagement that had a member — which is all of them, since creating
// one seats its lead. 0016_app_domain no longer indexes status; these tests
// hold it there, one referencing table at a time, because the index would have
// to come back before any of them noticed.

// statusRepos is newRepos with the database handle, which these tests need in
// order to seed rows by direct SQL for tables that have no repository here.
func statusRepos(t *testing.T) (repos, *store.DB) {
	t.Helper()
	db := storetest.Migrated(t)
	return repos{
		Engagements: engagement.NewEngagements(db),
		Scenarios:   engagement.NewScenarios(db),
		Steps:       engagement.NewSteps(db),
		Executions:  engagement.NewExecutions(db),
		Findings:    engagement.NewFindings(db),
		Comments:    engagement.NewComments(db),
		Evidence:    engagement.NewEvidenceRepo(db),
	}, db
}

func TestSetStatusOnAnEngagementThatHasChildren(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		seed func(t *testing.T, r repos, db *store.DB, engID string)
	}{
		{
			name: "no children",
			seed: func(*testing.T, repos, *store.DB, string) {},
		},
		{
			name: "a member",
			seed: func(t *testing.T, _ repos, db *store.DB, engID string) {
				t.Helper()
				if err := writeSQL(t, db, `INSERT INTO app.engagement_member
					(engagement_id, user_id, role, added_at)
					VALUES (?, 'user-1', 'lead', NOW())`, engID); err != nil {
					t.Fatalf("seed member: %v", err)
				}
			},
		},
		{
			name: "a scenario",
			seed: func(t *testing.T, r repos, _ *store.DB, engID string) {
				t.Helper()
				mustCreateScenario(t, r, engID, 1, "Initial access")
			},
		},
		{
			name: "a finding",
			seed: func(t *testing.T, r repos, _ *store.DB, engID string) {
				t.Helper()
				if _, err := r.Findings.Create(context.Background(), engagement.NewFinding{
					EngagementID: engID,
					Title:        "Weak MFA policy",
					Description:  "MFA is not enforced for admins",
					Severity:     "High",
					Owner:        "user-1",
				}); err != nil {
					t.Fatalf("seed finding: %v", err)
				}
			},
		},
		{
			name: "a report",
			seed: func(t *testing.T, _ repos, db *store.DB, engID string) {
				t.Helper()
				if err := writeSQL(t, db, `INSERT INTO app.report
					(id, engagement_id, title, created_by, created_at, updated_at)
					VALUES ('report-1', ?, 'Final', 'user-1', NOW(), NOW())`, engID); err != nil {
					t.Fatalf("seed report: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, db := statusRepos(t)
			e := mustCreateEngagement(t, r)
			tc.seed(t, r, db, e.ID)

			updated, err := r.Engagements.SetStatus(context.Background(), e.ID, engagement.EngagementStatusActive)
			if err != nil {
				t.Fatalf("SetStatus: %v", err)
			}
			if updated.Status != engagement.EngagementStatusActive {
				t.Errorf("returned status = %q, want active", updated.Status)
			}

			// The returned row is a re-read, so confirm the write landed by
			// reading it again on its own.
			reread, err := r.Engagements.ByID(context.Background(), e.ID)
			if err != nil {
				t.Fatalf("ByID: %v", err)
			}
			if reread.Status != engagement.EngagementStatusActive {
				t.Errorf("stored status = %q, want active", reread.Status)
			}
			if !reread.UpdatedAt.After(e.UpdatedAt) {
				t.Errorf("updated_at = %v, want later than %v", reread.UpdatedAt, e.UpdatedAt)
			}
		})
	}
}

// The whole lifecycle in sequence, on an engagement that has a member, because
// the failure was per-UPDATE rather than per-value: every hop has to survive.
func TestSetStatusWalksTheWholeLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, db := statusRepos(t)
	e := mustCreateEngagement(t, r)
	if err := writeSQL(t, db, `INSERT INTO app.engagement_member
		(engagement_id, user_id, role, added_at)
		VALUES (?, 'user-1', 'lead', NOW())`, e.ID); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	mustCreateScenario(t, r, e.ID, 1, "Initial access")

	for _, want := range []engagement.EngagementStatus{
		engagement.EngagementStatusActive,
		engagement.EngagementStatusClosed,
		engagement.EngagementStatusArchived,
	} {
		got, err := r.Engagements.SetStatus(ctx, e.ID, want)
		if err != nil {
			t.Fatalf("SetStatus(%q): %v", want, err)
		}
		if got.Status != want {
			t.Fatalf("status = %q, want %q", got.Status, want)
		}
	}
}

// The after-hooks run in the same transaction as the UPDATE — the activity
// entry a status change writes is one — so a hook must still see the write.
func TestSetStatusRunsAfterHooksInTheSameTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r, db := statusRepos(t)
	e := mustCreateEngagement(t, r)
	if err := writeSQL(t, db, `INSERT INTO app.engagement_member
		(engagement_id, user_id, role, added_at)
		VALUES (?, 'user-1', 'lead', NOW())`, e.ID); err != nil {
		t.Fatalf("seed member: %v", err)
	}

	ran := false
	hook := func(ctx context.Context, tx *sql.Tx) error {
		ran = true
		var status string
		if err := tx.QueryRowContext(ctx,
			`SELECT status FROM app.engagement WHERE id = ?`, e.ID).Scan(&status); err != nil {
			return err
		}
		if status != string(engagement.EngagementStatusActive) {
			t.Errorf("status inside the hook = %q, want active", status)
		}
		return nil
	}

	if _, err := r.Engagements.SetStatus(ctx, e.ID, engagement.EngagementStatusActive, hook); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if !ran {
		t.Error("the after hook did not run")
	}
}
