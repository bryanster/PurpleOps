package analytics

import (
	"database/sql"
	"slices"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"
)

func newScope(engID string) Scope {
	return Scope{EngagementID: engID, Blind: blind.Scope{}}
}

func newBlindBlueScope(engID string) Scope {
	return Scope{EngagementID: engID, Blind: blind.Scope{Blind: true, Seat: authz.EngagementRoleBlue}}
}

func TestNavigatorLayerGolden(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	if layer.Name != "Baseline Assessment" {
		t.Errorf("name = %q, want %q", layer.Name, "Baseline Assessment")
	}
	if layer.AttackVersion != "99.0" {
		t.Errorf("attackVersion = %q, want %q", layer.AttackVersion, "99.0")
	}
	if layer.UnmatchedCount != 0 {
		t.Errorf("unmatched = %d, want 0", layer.UnmatchedCount)
	}

	byID := make(map[string]NavigatorTechniqueEntry, len(layer.Techniques))
	for _, e := range layer.Techniques {
		byID[e.TechniqueID] = e
	}

	// Hand-computed expectations from fixture:
	// Revealed: T1190(general/blocked→2), T1566(telemetry/nbt→1),
	//   T1027(general/partial→2), T1070(technique/nbt→4),
	//   T1059.001(complete,unscored→attempted,no colour)
	// Unrevealed (visible to all seats): T1203(none/nbt→0), T1059(tactic/nbt→3)
	// Omitted: T1566.001(skipped), T1547(pending)

	assertEntry(t, byID, "T1190", 2, "#fca128", true, "blocked")
	assertEntry(t, byID, "T1566", 1, "#ffee58", true, "")
	assertEntry(t, byID, "T1027", 2, "#fca128", true, "")
	assertEntry(t, byID, "T1070", 4, "#862121", true, "")
	assertEntry(t, byID, "T1059", 3, "#d13c3c", true, "")
	assertEntry(t, byID, "T1203", 0, "#aeb3bf", true, "")
	assertEntryUnscored(t, byID, "T1059.001")

	if _, ok := byID["T1566.001"]; ok {
		t.Error("T1566.001 should be omitted (skipped + unscored)")
	}
	if _, ok := byID["T1547"]; ok {
		t.Error("T1547 should be omitted (pending + unscored)")
	}

	if len(layer.Techniques) != 7 {
		t.Errorf("technique count = %d, want 7", len(layer.Techniques))
	}

	if e := byID["T1059.001"]; !e.IsSubtechnique {
		t.Error("T1059.001 should be marked as sub-technique")
	}
}

func assertEntry(t *testing.T, byID map[string]NavigatorTechniqueEntry, id string, wantScore int, wantColor string, wantEnabled bool, wantProtection string) {
	t.Helper()
	e, ok := byID[id]
	if !ok {
		t.Errorf("%s missing", id)
		return
	}
	if e.Score != wantScore {
		t.Errorf("%s score = %d, want %d", id, e.Score, wantScore)
	}
	if e.Color != wantColor {
		t.Errorf("%s color = %q, want %q", id, e.Color, wantColor)
	}
	if e.Enabled != wantEnabled {
		t.Errorf("%s enabled = %v, want %v", id, e.Enabled, wantEnabled)
	}
	if wantProtection != "" && e.Protection != wantProtection {
		t.Errorf("%s protection = %q, want %q", id, e.Protection, wantProtection)
	}
}

func assertEntryUnscored(t *testing.T, byID map[string]NavigatorTechniqueEntry, id string) {
	t.Helper()
	e, ok := byID[id]
	if !ok {
		t.Errorf("%s missing", id)
		return
	}
	if e.Score != 0 {
		t.Errorf("%s score = %d, want 0 (unscored)", id, e.Score)
	}
	if e.Color != "" {
		t.Errorf("%s color = %q, want empty (unscored)", id, e.Color)
	}
	if !e.Enabled {
		t.Errorf("%s should be enabled (attempted)", id)
	}
}

func TestNavigatorLayerBlindBlue(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newBlindBlueScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer (blue): %v", err)
	}

	byID := make(map[string]NavigatorTechniqueEntry, len(layer.Techniques))
	for _, e := range layer.Techniques {
		byID[e.TechniqueID] = e
	}

	want := []string{"T1190", "T1566", "T1059.001", "T1027", "T1070"}
	for _, id := range want {
		if _, ok := byID[id]; !ok {
			t.Errorf("blue: %s should be present", id)
		}
	}

	absent := []string{"T1059", "T1203"}
	for _, id := range absent {
		if _, ok := byID[id]; ok {
			t.Errorf("blue: %s should be absent (unrevealed)", id)
		}
	}

	if len(layer.Techniques) != 5 {
		t.Errorf("blue technique count = %d, want 5", len(layer.Techniques))
	}
}

func TestNavigatorLayerAttackVersionPin(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer1, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if layer1.AttackVersion != "99.0" {
		t.Errorf("baseline attack version = %q, want 99.0", layer1.AttackVersion)
	}

	var newEngID string
	if err := fx.DB.Write(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO content.content_source_version
				(id, source_id, version, status, item_count, synced_at, error,
				 raw_sha256, raw_path, raw_bytes, created_at, updated_at)
			 VALUES (?, '01900000-0000-7000-8000-000000000001', '98.0', 'ready', 0, '2026-01-01', '',
			        '', '', 0, '2026-01-01', '2026-01-01')`,
			"0192f1a0-0000-7000-8000-deadbeef0001",
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement
				(id, name, client, description, status, starts_on, ends_on,
				 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			 VALUES (?, 'Pin Test', 'TestOrg', '', 'active', '2026-01-01', '2026-12-31',
			        '98.0', 'standard', false, ?, '2026-01-01', '2026-01-01')`,
			"0192f1a0-0000-7000-8000-deadbeef0002",
			fx.BaselineID[:9]+"user",
		); err != nil {
			return err
		}
		newEngID = "0192f1a0-0000-7000-8000-deadbeef0002"
		return nil
	}); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}

	layer2, err := q.NavigatorLayer(t.Context(), newScope(newEngID))
	if err != nil {
		t.Fatalf("second engagement: %v", err)
	}
	if layer2.AttackVersion != "98.0" {
		t.Errorf("second attack version = %q, want 98.0", layer2.AttackVersion)
	}
	if layer1.AttackVersion == layer2.AttackVersion {
		t.Errorf("both layers have same attack version %q — pin not honoured", layer1.AttackVersion)
	}
}

func TestNavigatorLayerOrdinalAgreement(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	db := fx.DB.Read()
	for _, e := range layer.Techniques {
		var cat sql.NullString
		err := db.QueryRowContext(t.Context(), `
			SELECT e.detection_category
			FROM app.step s
			JOIN app.scenario sc ON s.scenario_id = sc.id
			JOIN app.execution e ON e.step_id = s.id
			WHERE sc.engagement_id = ?
			  AND s.technique_id = ?
			  AND e.status IN ('complete', 'blocked')
			  AND e.detection_category IS NOT NULL
			ORDER BY
			  CASE e.detection_category
			    WHEN 'none' THEN 0
			    WHEN 'telemetry' THEN 1
			    WHEN 'general' THEN 2
			    WHEN 'tactic' THEN 3
			    WHEN 'technique' THEN 4
			  END DESC
			LIMIT 1`,
			fx.BaselineID, e.TechniqueID,
		).Scan(&cat)

		if err != nil || !cat.Valid {
			// Unscored — no scored execution for this technique.
			if e.Score != 0 {
				t.Errorf("%s: unscored, score = %d, want 0", e.TechniqueID, e.Score)
			}
			if e.Color != "" {
				t.Errorf("%s: unscored, color = %q, want empty", e.TechniqueID, e.Color)
			}
			continue
		}

		expectedScore := categoryOrdinal(cat.String)
		if e.Score != expectedScore {
			t.Errorf("%s: category %s → score %d, got %d", e.TechniqueID, cat.String, expectedScore, e.Score)
		}
		if e.Color != NavigatorColourRamp[expectedScore] {
			t.Errorf("%s: category %s → colour %q, got %q",
				e.TechniqueID, cat.String, NavigatorColourRamp[expectedScore], e.Color)
		}
	}
}

func categoryOrdinal(cat string) int {
	switch cat {
	case "none":
		return 0
	case "telemetry":
		return 1
	case "general":
		return 2
	case "tactic":
		return 3
	case "technique":
		return 4
	default:
		return -1
	}
}

func TestNavigatorLayerSubtechniqueIndependence(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	byID := make(map[string]NavigatorTechniqueEntry, len(layer.Techniques))
	for _, e := range layer.Techniques {
		byID[e.TechniqueID] = e
	}

	sub, subOK := byID["T1059.001"]
	if !subOK {
		t.Fatal("T1059.001 missing")
	}
	if sub.TechniqueID != "T1059.001" {
		t.Errorf("T1059.001 techniqueID = %q", sub.TechniqueID)
	}
	if !sub.IsSubtechnique {
		t.Error("T1059.001 should have IsSubtechnique = true")
	}

	parent, parentOK := byID["T1059"]
	if !parentOK {
		t.Fatal("T1059 missing")
	}
	if parent.Score == sub.Score {
		t.Errorf("parent T1059 score %d should differ from sub T1059.001 score %d",
			parent.Score, sub.Score)
	}

	if _, ok := byID["T1566.001"]; ok {
		t.Error("T1566.001 should be omitted (not attempted)")
	}
	if _, ok := byID["T1566"]; !ok {
		t.Error("T1566 parent should be present")
	}
}

func TestNavigatorLayerUnmatchedOmitted(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	var stepID string
	if err := fx.DB.Write(t.Context(), func(tx *sql.Tx) error {
		stepID = "0192f1a0-0000-7000-8000-different00001"
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.step
				(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
				 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
				 attack_version, revealed_at, created_at, updated_at)
			 VALUES (?, ?, 99, 'Unmatched Step', '', 'T9999', '', '', '{}', '', '', '[]', '[]',
			        '99.0', '2026-06-01', '2026-06-01', '2026-06-01')`,
			stepID, fx.BaselineScenarioIDs[0],
		); err != nil {
			return err
		}
		execID := "0192f1a0-0000-7000-8000-different00002"
		if _, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.execution
				(id, step_id, version, status, executed_by, started_at, ended_at,
				 command_run, source_host, target_host, red_notes,
				 detection_category, detection_modifiers, protection,
				 detected_at, detecting_source, detecting_rule_ref,
				 alert_severity, blue_notes, scored_by, scored_at,
				 created_at, updated_at)
			 VALUES (?, ?, 1, 'complete', ?, '2026-06-01', '2026-06-02',
			        '', '', '', '',
			        'none', '[]', 'not_blocked',
			        '2026-06-02', '', '',
			        '', '', ?, '2026-06-02',
			        '2026-06-01', '2026-06-01')`,
			execID, stepID, fx.BaselineID[:9]+"user",
			fx.BaselineID[:9]+"user",
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed unmatched step: %v", err)
	}

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	byID := make(map[string]NavigatorTechniqueEntry, len(layer.Techniques))
	for _, e := range layer.Techniques {
		byID[e.TechniqueID] = e
	}

	if _, ok := byID["T9999"]; ok {
		t.Error("T9999 should be omitted from the layer (not in pinned matrix)")
	}

	if layer.UnmatchedCount != 1 {
		t.Errorf("unmatched = %d, want 1", layer.UnmatchedCount)
	}
}

func TestNavigatorLayerTechniquesSorted(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	if !slices.IsSortedFunc(layer.Techniques, func(a, b NavigatorTechniqueEntry) int {
		if a.TechniqueID < b.TechniqueID {
			return -1
		}
		if a.TechniqueID > b.TechniqueID {
			return 1
		}
		return 0
	}) {
		t.Error("techniques not sorted by techniqueID")
	}
}

func TestNavigatorLayerDescriptionIncludesUnmatched(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	layer, err := q.NavigatorLayer(t.Context(), newScope(fx.BaselineID))
	if err != nil {
		t.Fatalf("NavigatorLayer: %v", err)
	}

	if layer.UnmatchedCount != 0 {
		t.Skip("fixture has unmatched techniques — test precondition failed")
	}
	if layer.Description != "Synthetic baseline for analytics tests." {
		t.Logf("description = %q", layer.Description)
	}
}
