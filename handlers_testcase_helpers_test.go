package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"on", true},
		{"On", true},
		{"false", false},
		{"False", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"0", false},
		{"1", false},
	}

	for _, tt := range tests {
		got := parseBool(tt.input)
		if got != tt.want {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestComputeOutcome(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		tc   *TestCase
		want string
	}{
		{
			name: "prevented yes",
			tc:   &TestCase{Prevented: "Yes"},
			want: "Prevented",
		},
		{
			name: "prevented partial",
			tc:   &TestCase{Prevented: "Partial"},
			want: "Prevented",
		},
		{
			name: "alerted",
			tc:   &TestCase{Prevented: "No", Alerted: &trueVal},
			want: "Alerted",
		},
		{
			name: "logged",
			tc:   &TestCase{Prevented: "No", Alerted: &falseVal, Logged: &trueVal},
			want: "Logged",
		},
		{
			name: "missed",
			tc:   &TestCase{Prevented: "No", Alerted: &falseVal, Logged: &falseVal},
			want: "Missed",
		},
		{
			name: "empty - no data",
			tc:   &TestCase{},
			want: "",
		},
		{
			name: "only logged false, no prevented",
			tc:   &TestCase{Logged: &falseVal},
			want: "",
		},
		{
			name: "prevented overrides alerted",
			tc:   &TestCase{Prevented: "Yes", Alerted: &trueVal},
			want: "Prevented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeOutcome(tt.tc)
			if got != tt.want {
				t.Errorf("computeOutcome() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeUploadFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "passwd"},
		{"..\\..\\windows\\system32", "_.._windows_system32"},
		{".hidden", "hidden"},
		{"...dots", "dots"},
		{"path/to/file.txt", "file.txt"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
	}

	for _, tt := range tests {
		got := sanitizeUploadFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeUploadFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeIDs(t *testing.T) {
	validIDs := map[string]bool{
		"abc123": true,
		"def456": true,
		"ghi789": true,
	}

	tests := []struct {
		name  string
		ids   []string
		count int
	}{
		{"all valid", []string{"abc123", "def456"}, 2},
		{"some invalid", []string{"abc123", "invalid", "def456"}, 2},
		{"all invalid", []string{"x", "y", "z"}, 0},
		{"empty", []string{}, 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeIDs(tt.ids, validIDs)
			if len(got) != tt.count {
				t.Errorf("sanitizeIDs() returned %d items, want %d", len(got), tt.count)
			}
		})
	}
}

func TestExtractIDFunctions(t *testing.T) {
	id1 := bson.NewObjectID()
	id2 := bson.NewObjectID()

	t.Run("extractIDs (sources)", func(t *testing.T) {
		sources := []Source{{ID: id1}, {ID: id2}}
		m := extractIDs(sources)
		if len(m) != 2 {
			t.Errorf("expected 2 IDs, got %d", len(m))
		}
		if !m[id1.Hex()] {
			t.Error("missing id1")
		}
	})

	t.Run("extractTargetIDs", func(t *testing.T) {
		targets := []Target{{ID: id1}}
		m := extractTargetIDs(targets)
		if len(m) != 1 {
			t.Errorf("expected 1 ID, got %d", len(m))
		}
	})

	t.Run("extractToolIDs", func(t *testing.T) {
		tools := []Tool{{ID: id1}, {ID: id2}}
		m := extractToolIDs(tools)
		if len(m) != 2 {
			t.Errorf("expected 2 IDs, got %d", len(m))
		}
	})

	t.Run("extractControlIDs", func(t *testing.T) {
		controls := []Control{{ID: id1}}
		m := extractControlIDs(controls)
		if len(m) != 1 {
			t.Errorf("expected 1 ID, got %d", len(m))
		}
	})

	t.Run("extractTagIDs", func(t *testing.T) {
		tags := []Tag{{ID: id1}}
		m := extractTagIDs(tags)
		if len(m) != 1 {
			t.Errorf("expected 1 ID, got %d", len(m))
		}
	})

	t.Run("extractDatasourceIDs", func(t *testing.T) {
		ds := []Datasource{{ID: id1}}
		m := extractDatasourceIDs(ds)
		if len(m) != 1 {
			t.Errorf("expected 1 ID, got %d", len(m))
		}
	})

	t.Run("extractRuleIDs", func(t *testing.T) {
		rules := []DetectionRule{{ID: id1}, {ID: id2}}
		m := extractRuleIDs(rules)
		if len(m) != 2 {
			t.Errorf("expected 2 IDs, got %d", len(m))
		}
	})
}

func TestApplyFormField(t *testing.T) {
	form := url.Values{}
	form.Set("name", "Test Name")
	form.Set("empty", "")

	r := &http.Request{Form: form}
	r.Method = "POST"

	var name string
	applyFormField(r, "name", &name)
	if name != "Test Name" {
		t.Errorf("expected 'Test Name', got %q", name)
	}

	// Empty value with Has()
	var empty string = "original"
	applyFormField(r, "empty", &empty)
	if empty != "" {
		t.Errorf("expected empty string, got %q", empty)
	}

	// Missing field
	var missing string = "original"
	applyFormField(r, "missing", &missing)
	if missing != "original" {
		t.Errorf("expected 'original' for missing field, got %q", missing)
	}
}

func TestApplyFormBool(t *testing.T) {
	form := url.Values{}
	form.Set("alerted", "yes")
	form.Set("logged", "false")

	r := &http.Request{Form: form}
	r.Method = "POST"

	var alerted *bool
	applyFormBool(r, "alerted", &alerted)
	if alerted == nil || !*alerted {
		t.Error("expected alerted=true")
	}

	var logged *bool
	applyFormBool(r, "logged", &logged)
	if logged == nil || *logged {
		t.Error("expected logged=false")
	}

	var missing *bool
	applyFormBool(r, "missing", &missing)
	if missing != nil {
		t.Error("expected missing=nil")
	}
}

func TestApplyFormBoolVisible(t *testing.T) {
	form := url.Values{}
	form.Set("visible", "on")

	r := &http.Request{Form: form}
	r.Method = "POST"

	visible := false
	applyFormBoolVisible(r, "visible", &visible)
	if !visible {
		t.Error("expected visible=true from 'on'")
	}

	form.Set("visible", "False")
	visible = true
	applyFormBoolVisible(r, "visible", &visible)
	if visible {
		t.Error("expected visible=false from 'False'")
	}
}

func TestApplyFormTime(t *testing.T) {
	form := url.Values{}
	form.Set("starttime", "2024-03-15T10:30")
	form.Set("timezone", "0")

	r := &http.Request{Form: form}
	r.Method = "POST"
	r.Header = http.Header{}
	r.URL = &url.URL{}

	// Need to also set up r.ParseForm context
	r.Body = http.NoBody
	r.ContentLength = 0

	var startTime *time.Time
	applyFormTime(r, "starttime", &startTime)
	if startTime == nil {
		t.Fatal("expected startTime to be set")
	}
	if startTime.Year() != 2024 || startTime.Month() != 3 || startTime.Day() != 15 {
		t.Errorf("unexpected time: %v", startTime)
	}

	// Empty field
	var empty *time.Time
	applyFormTime(r, "empty", &empty)
	if empty != nil {
		t.Error("expected nil for empty time field")
	}

	// Invalid format
	form.Set("badtime", "not-a-time")
	var bad *time.Time
	applyFormTime(r, "badtime", &bad)
	if bad != nil {
		t.Error("expected nil for invalid time format")
	}
}
