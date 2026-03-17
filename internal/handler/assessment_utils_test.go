package handler

import (
	"testing"

	"github.com/bryanster/purpleops/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRatingToScore(t *testing.T) {
	tests := []struct {
		rating string
		want   int
	}{
		{"Critical", 5},
		{"High", 4},
		{"Medium", 3},
		{"Low", 2},
		{"Informational", 1},
		{"Unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := ratingToScore(tt.rating)
		if got != tt.want {
			t.Errorf("ratingToScore(%q) = %d, want %d", tt.rating, got, tt.want)
		}
	}
}

func TestBuildStats(t *testing.T) {
	testcases := []models.TestCase{
		{Tactic: "Execution", Outcome: "Prevented", AlertSeverity: "Critical", PreventedRating: "High", Priority: "P1", PriorityUrgency: "Immediate", Controls: []string{"ctrl1"}},
		{Tactic: "Execution", Outcome: "Alerted", AlertSeverity: "Medium", DetectionRating: "Low"},
		{Tactic: "Persistence", Outcome: "Missed", AlertSeverity: "Low"},
		{Tactic: "Persistence", Outcome: "Logged"},
	}

	stats := buildStats(testcases)

	// Check "All" stats
	all := stats["All"]
	if all.Prevented != 1 {
		t.Errorf("All.Prevented = %d, want 1", all.Prevented)
	}
	if all.Alerted != 1 {
		t.Errorf("All.Alerted = %d, want 1", all.Alerted)
	}
	if all.Logged != 1 {
		t.Errorf("All.Logged = %d, want 1", all.Logged)
	}
	if all.Missed != 1 {
		t.Errorf("All.Missed = %d, want 1", all.Missed)
	}
	if all.Critical != 1 {
		t.Errorf("All.Critical = %d, want 1", all.Critical)
	}
	if all.Medium != 1 {
		t.Errorf("All.Medium = %d, want 1", all.Medium)
	}
	if all.Low != 1 {
		t.Errorf("All.Low = %d, want 1", all.Low)
	}

	// Check Execution stats
	exec := stats["Execution"]
	if exec.Prevented != 1 {
		t.Errorf("Execution.Prevented = %d, want 1", exec.Prevented)
	}
	if exec.Alerted != 1 {
		t.Errorf("Execution.Alerted = %d, want 1", exec.Alerted)
	}
	if len(exec.ScoresPrevent) != 1 {
		t.Errorf("Execution.ScoresPrevent length = %d, want 1", len(exec.ScoresPrevent))
	}
	if len(exec.ScoresDetect) != 1 {
		t.Errorf("Execution.ScoresDetect length = %d, want 1", len(exec.ScoresDetect))
	}
	if len(exec.Controls) != 1 {
		t.Errorf("Execution.Controls length = %d, want 1", len(exec.Controls))
	}

	// Check Persistence stats
	pers := stats["Persistence"]
	if pers.Missed != 1 {
		t.Errorf("Persistence.Missed = %d, want 1", pers.Missed)
	}
	if pers.Logged != 1 {
		t.Errorf("Persistence.Logged = %d, want 1", pers.Logged)
	}
}

func TestBuildStatsEmpty(t *testing.T) {
	stats := buildStats(nil)
	if stats["All"] == nil {
		t.Error("expected 'All' stats entry even with nil input")
	}
	all := stats["All"]
	if all.Prevented != 0 || all.Alerted != 0 || all.Logged != 0 || all.Missed != 0 {
		t.Error("expected all zeros for empty testcases")
	}
}

func TestGetFieldValue(t *testing.T) {
	id := bson.NewObjectID()
	a := &models.Assessment{
		Sources:           []models.Source{{ID: id, Name: "S1"}},
		Targets:           []models.Target{{ID: id, Name: "T1"}},
		Tools:             []models.Tool{{ID: id, Name: "Tool1"}},
		Controls:          []models.Control{{ID: id, Name: "C1"}},
		Tags:              []models.Tag{{ID: id, Name: "Tag1"}},
		Datasources:       []models.Datasource{{ID: id, Name: "DS1"}},
		Rules:             []models.DetectionRule{{ID: id, Name: "R1"}},
		DetectionSources:  []models.Datasource{{ID: id, Name: "DetSrc"}},
		PreventionSources: []models.Datasource{{ID: id, Name: "PrevSrc"}},
	}

	if v := getFieldValue(a, "sources"); v == nil {
		t.Error("expected non-nil for sources")
	}
	if v := getFieldValue(a, "targets"); v == nil {
		t.Error("expected non-nil for targets")
	}
	if v := getFieldValue(a, "tools"); v == nil {
		t.Error("expected non-nil for tools")
	}
	if v := getFieldValue(a, "controls"); v == nil {
		t.Error("expected non-nil for controls")
	}
	if v := getFieldValue(a, "tags"); v == nil {
		t.Error("expected non-nil for tags")
	}
	if v := getFieldValue(a, "datasources"); v == nil {
		t.Error("expected non-nil for datasources")
	}
	if v := getFieldValue(a, "rules"); v == nil {
		t.Error("expected non-nil for rules")
	}
	if v := getFieldValue(a, "detectionsources"); v == nil {
		t.Error("expected non-nil for detectionsources")
	}
	if v := getFieldValue(a, "preventionsources"); v == nil {
		t.Error("expected non-nil for preventionsources")
	}
	if v := getFieldValue(a, "unknown"); v != nil {
		t.Error("expected nil for unknown field")
	}
}

func TestUpdateSources(t *testing.T) {
	id1 := bson.NewObjectID()
	existing := []models.Source{
		{ID: id1, Name: "Old Source", Description: "Old Desc"},
	}

	data := []map[string]string{
		{"id": id1.Hex(), "name": "Updated Source", "description": "Updated Desc"},
		{"id": "tmp-1", "name": "New Source", "description": "New Desc"},
	}

	result := updateSources(existing, data)
	if len(result) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(result))
	}

	// First should be updated existing
	if result[0].Name != "Updated Source" {
		t.Errorf("expected 'Updated Source', got %q", result[0].Name)
	}
	if result[0].ID != id1 {
		t.Error("expected preserved ID for updated source")
	}

	// Second should be new
	if result[1].Name != "New Source" {
		t.Errorf("expected 'New Source', got %q", result[1].Name)
	}
	if result[1].ID == id1 {
		t.Error("expected new ID for new source")
	}
}

func TestUpdateSourcesNonMatchingID(t *testing.T) {
	id1 := bson.NewObjectID()
	existing := []models.Source{
		{ID: id1, Name: "Source1"},
	}

	// Reference a non-existing ID
	data := []map[string]string{
		{"id": bson.NewObjectID().Hex(), "name": "Ghost", "description": ""},
	}

	result := updateSources(existing, data)
	if len(result) != 0 {
		t.Errorf("expected 0 sources for non-matching ID, got %d", len(result))
	}
}

func TestUpdateTargets(t *testing.T) {
	existing := []models.Target{}
	data := []map[string]string{
		{"id": "tmp-1", "name": "New Target", "description": "Desc"},
	}

	result := updateTargets(existing, data)
	if len(result) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result))
	}
	if result[0].Name != "New Target" {
		t.Errorf("expected 'New Target', got %q", result[0].Name)
	}
}

func TestUpdateTools(t *testing.T) {
	id1 := bson.NewObjectID()
	existing := []models.Tool{{ID: id1, Name: "Tool1"}}
	data := []map[string]string{
		{"id": id1.Hex(), "name": "Tool1 Updated", "description": "Desc"},
	}

	result := updateTools(existing, data)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Name != "Tool1 Updated" {
		t.Errorf("expected 'Tool1 Updated', got %q", result[0].Name)
	}
}

func TestUpdateControls(t *testing.T) {
	data := []map[string]string{
		{"id": "tmp-1", "name": "Firewall", "description": "Network firewall"},
	}

	result := updateControls(nil, data)
	if len(result) != 1 {
		t.Fatalf("expected 1 control, got %d", len(result))
	}
	if result[0].Name != "Firewall" {
		t.Errorf("expected 'Firewall', got %q", result[0].Name)
	}
}

func TestUpdateTags(t *testing.T) {
	id1 := bson.NewObjectID()
	existing := []models.Tag{{ID: id1, Name: "Tag1", Colour: "#ff0000"}}
	data := []map[string]string{
		{"id": id1.Hex(), "name": "Tag1", "colour": "#00ff00"},
		{"id": "tmp-1", "name": "NewTag", "colour": "#0000ff"},
	}

	result := updateTags(existing, data)
	if len(result) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(result))
	}
	if result[0].Colour != "#00ff00" {
		t.Errorf("expected colour '#00ff00', got %q", result[0].Colour)
	}
	if result[1].Name != "NewTag" {
		t.Errorf("expected 'NewTag', got %q", result[1].Name)
	}
}

func TestUpdateDatasources(t *testing.T) {
	data := []map[string]string{
		{"id": "tmp-1", "name": "DS1", "description": "Data source 1"},
	}

	result := updateDatasources(nil, data)
	if len(result) != 1 {
		t.Fatalf("expected 1 datasource, got %d", len(result))
	}
}

func TestUpdateDetectionRules(t *testing.T) {
	data := []map[string]string{
		{"id": "tmp-1", "name": "Rule1", "description": "Detection rule 1"},
		{"id": "tmp-2", "name": "Rule2", "description": "Detection rule 2"},
	}

	result := updateDetectionRules(nil, data)
	if len(result) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(result))
	}
}
