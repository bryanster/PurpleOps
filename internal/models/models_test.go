package models

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// --- APIKey model ---

func TestAPIKeyDefaults(t *testing.T) {
	userID := bson.NewObjectID()
	now := time.Now()

	k := APIKey{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Name:      "CI pipeline",
		KeyHash:   "abc123hash",
		Prefix:    "pops_abc1",
		CreatedAt: now,
		Active:    true,
	}

	if k.Name != "CI pipeline" {
		t.Errorf("unexpected Name: %q", k.Name)
	}
	if k.UserID != userID {
		t.Error("UserID mismatch")
	}
	if !k.Active {
		t.Error("expected Active=true")
	}
	if k.LastUsedAt != nil {
		t.Error("expected LastUsedAt=nil by default")
	}
	if len(k.Roles) != 0 {
		t.Errorf("expected empty Roles, got %d", len(k.Roles))
	}
	if len(k.Assessments) != 0 {
		t.Errorf("expected empty Assessments, got %d", len(k.Assessments))
	}
}

func TestAPIKeyWithRolesAndAssessments(t *testing.T) {
	r1 := bson.NewObjectID()
	r2 := bson.NewObjectID()
	a1 := bson.NewObjectID()

	k := APIKey{
		ID:          bson.NewObjectID(),
		UserID:      bson.NewObjectID(),
		Roles:       []bson.ObjectID{r1, r2},
		Assessments: []bson.ObjectID{a1},
		Active:      true,
	}

	if len(k.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(k.Roles))
	}
	if k.Roles[0] != r1 || k.Roles[1] != r2 {
		t.Error("role IDs not preserved")
	}
	if len(k.Assessments) != 1 {
		t.Errorf("expected 1 assessment, got %d", len(k.Assessments))
	}
	if k.Assessments[0] != a1 {
		t.Error("assessment ID not preserved")
	}
}

func TestAPIKeyLastUsedAt(t *testing.T) {
	t1 := time.Now()
	k := APIKey{
		LastUsedAt: &t1,
	}
	if k.LastUsedAt == nil {
		t.Fatal("expected LastUsedAt to be set")
	}
	if !k.LastUsedAt.Equal(t1) {
		t.Error("LastUsedAt value mismatch")
	}
}

func TestAPIKeyInactiveFlag(t *testing.T) {
	k := APIKey{Active: false}
	if k.Active {
		t.Error("expected Active=false")
	}
}

func TestEsc(t *testing.T) {
	tests := []struct {
		input string
		raw   bool
		want  string
	}{
		{"hello", false, "hello"},
		{"hello", true, "hello"},
		{"<script>alert('xss')</script>", false, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"<script>alert('xss')</script>", true, "<script>alert('xss')</script>"},
		{"a & b", false, "a &amp; b"},
		{"a & b", true, "a & b"},
		{`"quoted"`, false, "&#34;quoted&#34;"},
		{`"quoted"`, true, `"quoted"`},
		{"", false, ""},
		{"", true, ""},
	}

	for _, tt := range tests {
		got := Esc(tt.input, tt.raw)
		if got != tt.want {
			t.Errorf("Esc(%q, %v) = %q, want %q", tt.input, tt.raw, got, tt.want)
		}
	}
}

func TestTimeStr(t *testing.T) {
	tm := time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC)

	if got := TimeStr(&tm); got != "2024-03-15 10:30:45" {
		t.Errorf("TimeStr(&tm) = %q, want %q", got, "2024-03-15 10:30:45")
	}

	if got := TimeStr(nil); got != "None" {
		t.Errorf("TimeStr(nil) = %q, want %q", got, "None")
	}
}

func TestTimeStrLocal(t *testing.T) {
	tm := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	if got := TimeStrLocal(&tm); got != "2024-03-15T10:30" {
		t.Errorf("TimeStrLocal(&tm) = %q, want %q", got, "2024-03-15T10:30")
	}

	if got := TimeStrLocal(nil); got != "" {
		t.Errorf("TimeStrLocal(nil) = %q, want %q", got, "")
	}
}

func TestBoolPtr(t *testing.T) {
	truePtr := BoolPtr(true)
	if truePtr == nil || *truePtr != true {
		t.Error("BoolPtr(true) should return pointer to true")
	}

	falsePtr := BoolPtr(false)
	if falsePtr == nil || *falsePtr != false {
		t.Error("BoolPtr(false) should return pointer to false")
	}
}

func TestNowPtr(t *testing.T) {
	before := time.Now().UTC()
	result := NowPtr()
	after := time.Now().UTC()

	if result == nil {
		t.Fatal("NowPtr() returned nil")
	}
	if result.Before(before) || result.After(after) {
		t.Error("NowPtr() should return current UTC time")
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0.00"},
		{100, "100.00"},
		{33.333333, "33.33"},
		{50.5, "50.50"},
		{99.999, "100.00"},
		{0.1, "0.10"},
	}

	for _, tt := range tests {
		got := FormatFloat(tt.input)
		if got != tt.want {
			t.Errorf("FormatFloat(%f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAssessmentMultiToJSON(t *testing.T) {
	id1 := bson.NewObjectID()
	id2 := bson.NewObjectID()

	assessment := Assessment{
		Sources: []Source{
			{ID: id1, Name: "Source1", Description: "Desc1"},
			{ID: id2, Name: "Source2", Description: "Desc2"},
		},
		Tags: []Tag{
			{ID: id1, Name: "Tag1", Colour: "#ff0000"},
		},
		Tools: []Tool{
			{ID: id1, Name: "Tool1", Description: "ToolDesc"},
		},
		Controls: []Control{
			{ID: id1, Name: "Ctrl1", Description: "CtrlDesc"},
		},
		Targets: []Target{
			{ID: id1, Name: "Target1", Description: "TargetDesc"},
		},
		Datasources: []Datasource{
			{ID: id1, Name: "DS1", Description: "DSDesc"},
		},
		Rules: []DetectionRule{
			{ID: id1, Name: "Rule1", Description: "RuleDesc"},
		},
		DetectionSources: []Datasource{
			{ID: id1, Name: "DetSrc1", Description: "DetDesc"},
		},
		PreventionSources: []Datasource{
			{ID: id1, Name: "PrevSrc1", Description: "PrevDesc"},
		},
	}

	// Test sources with escaping
	result := assessment.MultiToJSON("sources", false)
	if len(result) != 2 {
		t.Errorf("expected 2 sources, got %d", len(result))
	}
	if result[0]["name"] != "Source1" {
		t.Errorf("expected Source1, got %v", result[0]["name"])
	}

	// Test sources raw
	result = assessment.MultiToJSON("sources", true)
	if len(result) != 2 {
		t.Errorf("expected 2 sources raw, got %d", len(result))
	}

	// Test tags
	result = assessment.MultiToJSON("tags", false)
	if len(result) != 1 {
		t.Errorf("expected 1 tag, got %d", len(result))
	}
	if result[0]["colour"] != "#ff0000" {
		t.Errorf("expected #ff0000, got %v", result[0]["colour"])
	}

	// Test tools
	result = assessment.MultiToJSON("tools", false)
	if len(result) != 1 {
		t.Errorf("expected 1 tool, got %d", len(result))
	}

	// Test controls
	result = assessment.MultiToJSON("controls", false)
	if len(result) != 1 {
		t.Errorf("expected 1 control, got %d", len(result))
	}

	// Test targets
	result = assessment.MultiToJSON("targets", false)
	if len(result) != 1 {
		t.Errorf("expected 1 target, got %d", len(result))
	}

	// Test datasources
	result = assessment.MultiToJSON("datasources", false)
	if len(result) != 1 {
		t.Errorf("expected 1 datasource, got %d", len(result))
	}

	// Test rules
	result = assessment.MultiToJSON("rules", false)
	if len(result) != 1 {
		t.Errorf("expected 1 rule, got %d", len(result))
	}

	// Test detectionsources
	result = assessment.MultiToJSON("detectionsources", false)
	if len(result) != 1 {
		t.Errorf("expected 1 detection source, got %d", len(result))
	}

	// Test preventionsources
	result = assessment.MultiToJSON("preventionsources", false)
	if len(result) != 1 {
		t.Errorf("expected 1 prevention source, got %d", len(result))
	}

	// Test unknown field
	result = assessment.MultiToJSON("unknown", false)
	if result != nil {
		t.Errorf("expected nil for unknown field, got %v", result)
	}
}

func TestAssessmentMultiToJSON_Escaping(t *testing.T) {
	id := bson.NewObjectID()

	assessment := Assessment{
		Sources: []Source{
			{ID: id, Name: "<b>XSS</b>", Description: "a & b"},
		},
	}

	// With escaping
	result := assessment.MultiToJSON("sources", false)
	if result[0]["name"] != "&lt;b&gt;XSS&lt;/b&gt;" {
		t.Errorf("expected escaped name, got %v", result[0]["name"])
	}
	if result[0]["description"] != "a &amp; b" {
		t.Errorf("expected escaped description, got %v", result[0]["description"])
	}

	// Raw (no escaping)
	result = assessment.MultiToJSON("sources", true)
	if result[0]["name"] != "<b>XSS</b>" {
		t.Errorf("expected raw name, got %v", result[0]["name"])
	}
}

func TestTestCaseToJSON(t *testing.T) {
	alerted := true
	logged := false
	startTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tc := TestCase{
		ID:              bson.NewObjectID(),
		AssessmentID:    "invalid-not-hex", // Will cause ToJSONMulti to return nil
		Name:            "Test Case 1",
		Objective:       "Test <objective>",
		MitreID:         "T1059",
		Tactic:          "Execution",
		State:           "Complete",
		Prevented:       "Yes",
		PreventedRating: "High",
		Alerted:         &alerted,
		AlertSeverity:   "Critical",
		Logged:          &logged,
		DetectionRating: "Medium",
		Visible:         true,
		Outcome:         "Prevented",
		StartTime:       &startTime,
		RedFiles: []FileDoc{
			{Name: "evidence.png", Path: "files/abc/def/evidence.png", Caption: "Screenshot"},
		},
		BlueFiles: []FileDoc{},
	}

	j := tc.ToJSON(false)

	if j["name"] != "Test Case 1" {
		t.Errorf("expected 'Test Case 1', got %v", j["name"])
	}
	if j["objective"] != "Test &lt;objective&gt;" {
		t.Errorf("expected escaped objective, got %v", j["objective"])
	}
	if j["alerted"] != &alerted {
		t.Errorf("expected alerted pointer")
	}
	if j["visible"] != true {
		t.Errorf("expected visible=true")
	}
	if j["starttime"] != "2024-01-01 12:00:00" {
		t.Errorf("expected formatted start time, got %v", j["starttime"])
	}

	// Red files
	redFiles := j["redfiles"].([]string)
	if len(redFiles) != 1 {
		t.Errorf("expected 1 red file, got %d", len(redFiles))
	}
	if redFiles[0] != "files/abc/def/evidence.png|Screenshot" {
		t.Errorf("unexpected red file format: %s", redFiles[0])
	}

	// Blue files
	blueFiles := j["bluefiles"].([]string)
	if len(blueFiles) != 0 {
		t.Errorf("expected 0 blue files, got %d", len(blueFiles))
	}

	// Raw mode
	jRaw := tc.ToJSON(true)
	if jRaw["objective"] != "Test <objective>" {
		t.Errorf("expected raw objective, got %v", jRaw["objective"])
	}
}
