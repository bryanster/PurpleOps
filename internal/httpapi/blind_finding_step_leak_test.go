package httpapi

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
)

// TestBlindFindingStepIdsLeakUnrevealedSteps demonstrates a blind-mode
// information disclosure on the findings endpoints.
//
// In a blind engagement a blue-seat member must not learn which steps exist
// before they are revealed: getStep answers 404 (conceal) for an unrevealed
// step, and the archive export explicitly blind-filters finding step links
// (archivehandler.go). But finding.read carries no GuardBlindMode, and
// findingToWire returns every linked stepId with no blind filtering — so
// listFindings / getFinding hand a blue member the IDs of unrevealed steps,
// leaking exactly the step existence blind mode withholds.
func TestBlindFindingStepIdsLeakUnrevealedSteps(t *testing.T) {
	server := newAuthServer(t)

	red := createUser(t, server, "red@example.com", "RedUser")
	blue := createUser(t, server, "blue@example.com", "BlueUser")

	const (
		engID      = "01900000-b001-7000-8000-000000000001"
		scenarioID = "01900000-b001-7000-8000-000000000002"
		stepID     = "01900000-b001-7000-8000-000000000003"
		findingID  = "01900000-b001-7000-8000-000000000004"
	)

	seedBlindEngagementDB(t, server, engID, scenarioID, red, blue)
	seedUnrevealedStep(t, server, stepID, scenarioID)

	// Link a finding to the unrevealed step.
	if err := server.db.Write(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.finding (id, engagement_id, title, description, severity,
			 recommendation, "owner", status, created_from_execution, created_at, updated_at)
			 VALUES (?, ?, 'Hidden Finding', '', 'high', '', ?, 'open', NULL, NOW(), NOW())`,
			findingID, engID, red.ID); err != nil {
			return err
		}
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.finding_step (finding_id, step_id) VALUES (?, ?)`,
			findingID, stepID)
		return err
	}); err != nil {
		t.Fatalf("seeding finding: %v", err)
	}

	blueCookie := sessionCookie(t, server.login(blue.Email, testPassword))

	// Blind mode conceals the unrevealed step itself.
	rec := server.get(BasePath+"/engagements/"+engID+"/scenarios/"+scenarioID+"/steps/"+stepID, blueCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("blue fetching the unrevealed step = %d, want 404 (blind conceal)\nbody: %s",
			rec.Code, rec.Body)
	}

	// But listing findings leaks the unrevealed step's id in the stepIds array.
	rec = server.get(BasePath+"/engagements/"+engID+"/findings", blueCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("blue listing findings = %d, want 200\nbody: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), stepID) {
		t.Fatalf("blue's finding list does not expose the unrevealed step id — leak not reproduced\nbody: %s",
			rec.Body.String())
	}
}
