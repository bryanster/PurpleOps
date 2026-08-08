package blind_test

// The store-layer half of M1-013's blind-mode criterion: "a blue member's
// repository read returns no unrevealed steps, verified at the store layer
// independently of any HTTP test."
//
// Independently is the point, so these tests build no server and speak no HTTP.
// They run the predicate against a real DuckDB table, because a filter that is
// only ever asserted against its own source code is a filter nobody has checked
// is valid SQL.
//
// The table is a fixture rather than app.step: steps arrive with M3, and a
// mechanism that could not be tested until then is a mechanism that would arrive
// untested. Its shape is the shape M3's will have as far as this package is
// concerned — an engagement, and a boolean saying whether the row has been shown
// to the blue side.

import (
	"database/sql"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/blind"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The fixture rows. Two revealed, two not, in one engagement — so a filter that
// returned nothing and a filter that returned everything are both visibly wrong.
var steps = []struct {
	id       string
	revealed bool
}{
	{"step-1", true},
	{"step-2", false},
	{"step-3", true},
	{"step-4", false},
}

// TestABlueReaderOfABlindEngagementSeesOnlyRevealedSteps is the criterion
// itself, at the layer it has to hold at.
func TestABlueReaderOfABlindEngagementSeesOnlyRevealedSteps(t *testing.T) {
	t.Parallel()

	db := probeTable(t)
	scope := blind.Scope{Blind: true, Seat: authz.EngagementRoleBlue}

	got := read(t, db, scope)

	want := []string{"step-1", "step-3"}
	if !equal(got, want) {
		t.Errorf("a blue member of a blind engagement read %v, want %v — the unrevealed steps are the ones "+
			"blind mode exists to withhold", got, want)
	}
}

// TestEveryOtherReaderSeesEveryStep is the other half. Without it the test above
// would pass against a filter that hides everything from everybody, which would
// be a different bug and not an improvement.
func TestEveryOtherReaderSeesEveryStep(t *testing.T) {
	t.Parallel()

	db := probeTable(t)
	all := []string{"step-1", "step-2", "step-3", "step-4"}

	cases := map[string]blind.Scope{
		"the red seat of a blind engagement":               {Blind: true, Seat: authz.EngagementRoleRed},
		"the lead of a blind engagement":                   {Blind: true, Seat: authz.EngagementRoleLead},
		"an observer of a blind engagement":                {Blind: true, Seat: authz.EngagementRoleObserver},
		"an administrator with no seat in it":              {Blind: true},
		"the blue seat of an engagement that is not blind": {Seat: authz.EngagementRoleBlue},
		"nobody in particular, nothing blind":              {},
	}

	for name, scope := range cases {
		t.Run(name, func(t *testing.T) {
			if got := read(t, db, scope); !equal(got, all) {
				t.Errorf("%s read %v, want every step %v", name, got, all)
			}
		})
	}
}

// TestWhereAndPermitsAgree holds the two ways of applying the filter to the same
// answer. They are used in different places — a query builds a WHERE clause, an
// event about to be pushed to a subscriber has a row already — and the moment
// they disagree, one of those places leaks.
func TestWhereAndPermitsAgree(t *testing.T) {
	t.Parallel()

	db := probeTable(t)

	for _, scope := range everyScope() {
		visible := map[string]bool{}
		for _, id := range read(t, db, scope) {
			visible[id] = true
		}
		for _, step := range steps {
			if got, want := scope.Permits(step.revealed), visible[step.id]; got != want {
				t.Errorf("%+v: Permits(revealed=%v) = %v, but the query %s the row",
					scope, step.revealed, got, included(want))
			}
		}
	}
}

// TestTheFilterAgreesWithThePolicyAboutWhoBlueIs is the belt-and-braces claim
// made checkable.
//
// The filter exists because a rule might be missed, so it must not itself be a
// second, subtly different opinion about who blind mode applies to. For every
// seat and every blind flag, a reader the policy withholds an unrevealed step
// from is a reader this filter hides it from — and, just as importantly, the
// reverse: a filter that hid rows the policy would have shown would be a feature
// nobody asked for, arriving as an empty page.
func TestTheFilterAgreesWithThePolicyAboutWhoBlueIs(t *testing.T) {
	t.Parallel()

	for _, scope := range everyScope() {
		subject := authz.Subject{
			UserID: "reader",
			Method: authz.MethodCookie,
			// A platform role that holds nothing, so that the grant comes from
			// the seat and the guard is reached rather than short-circuited.
			PlatformRole: authz.PlatformRoleMember,
			Memberships:  map[string]authz.EngagementRole{"engagement-1": scope.Seat},
		}
		unrevealed := authz.Resource{
			Type:            authz.ResourceExecution,
			ID:              "step-2",
			EngagementID:    "engagement-1",
			EngagementBlind: scope.Blind,
		}

		decision := authz.Can(t.Context(), subject, authz.ActionExecutionRead, unrevealed)
		policyWithholds := !decision.Allowed

		if scope.Seat == "" {
			// Not a member at all: the policy refuses for a reason that has
			// nothing to do with blind mode, so there is nothing to compare.
			continue
		}
		if got := scope.Withholds(); got != policyWithholds {
			t.Errorf("%+v: the filter withholds = %v and the policy withholds = %v (%s). "+
				"The two fences must agree about who blue is",
				scope, got, policyWithholds, decision.Reason)
		}
	}
}

// everyScope is the whole input space: each engagement role, plus no seat at
// all, against blind and not.
func everyScope() []blind.Scope {
	seats := append(authz.EngagementRoles(), "")

	scopes := make([]blind.Scope, 0, len(seats)*2)
	for _, seat := range seats {
		for _, isBlind := range []bool{true, false} {
			scopes = append(scopes, blind.Scope{Blind: isBlind, Seat: seat})
		}
	}
	return scopes
}

// probeTable builds a database holding the fixture rows above.
func probeTable(t *testing.T) *store.DB {
	t.Helper()

	db := storetest.New(t)
	err := db.Write(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(),
			`CREATE TABLE step_probe (id TEXT PRIMARY KEY, engagement_id TEXT NOT NULL, revealed BOOLEAN NOT NULL)`,
		); err != nil {
			return err
		}
		for _, step := range steps {
			if _, err := tx.ExecContext(t.Context(),
				`INSERT INTO step_probe (id, engagement_id, revealed) VALUES (?, ?, ?)`,
				step.id, "engagement-1", step.revealed); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("building the fixture table: %v", err)
	}
	return db
}

// read runs the query a repository would run, with the scope's predicate in it.
// The rest of the WHERE clause is there on purpose: the predicate has to
// compose, and one that only works as the whole clause would fail on the first
// real query.
func read(t *testing.T, db *store.DB, scope blind.Scope) []string {
	t.Helper()

	query := `SELECT id FROM step_probe WHERE engagement_id = ? AND ` + scope.Where("revealed") + ` ORDER BY id`
	rows, err := db.Read().QueryContext(t.Context(), query, "engagement-1")
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading: %v", err)
	}
	return ids
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// included renders whether a query returned a row, for a failure message that
// reads as a sentence.
func included(visible bool) string {
	if visible {
		return "returned"
	}
	return "did not return"
}
