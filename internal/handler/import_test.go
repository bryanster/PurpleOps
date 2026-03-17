package handler

import (
	"testing"

	"github.com/bryanster/purpleops/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"credential-access", "Credential Access"},
		{"command-and-control", "Command And Control"},
		{"execution", "Execution"},
		{"lateral-movement", "Lateral Movement"},
		{"", ""},
		{"single", "Single"},
		{"already-Capitalized", "Already Capitalized"},
	}

	for _, tt := range tests {
		got := toTitleCase(tt.input)
		if got != tt.want {
			t.Errorf("toTitleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "_/_/_/etc/passwd"},
		{"path/to/file.txt", "path/to/file.txt"},
		{"./file.txt", "file.txt"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeFilenameDoubleDots(t *testing.T) {
	// Should remove all ".." occurrences
	got := sanitizeFilename("a/../b/../../c")
	if got == "" {
		t.Error("sanitizeFilename should not return empty string")
	}
	// The exact output depends on filepath.Clean behavior, but should not contain ".."
	for i := 0; i < len(got)-1; i++ {
		if got[i] == '.' && got[i+1] == '.' {
			t.Errorf("sanitizeFilename result %q still contains '..'", got)
			break
		}
	}
}

func TestFindExistingMultiEntry(t *testing.T) {
	id1 := bson.NewObjectID()
	id2 := bson.NewObjectID()

	assessment := &models.Assessment{
		Sources:     []models.Source{{ID: id1, Name: "Source1"}},
		Targets:     []models.Target{{ID: id1, Name: "Target1"}},
		Tools:       []models.Tool{{ID: id1, Name: "Tool1"}},
		Controls:    []models.Control{{ID: id1, Name: "Control1"}},
		Tags:        []models.Tag{{ID: id1, Name: "Tag1"}},
		Datasources: []models.Datasource{{ID: id2, Name: "DS1"}},
		Rules:       []models.DetectionRule{{ID: id2, Name: "Rule1"}},
	}

	tests := []struct {
		field string
		name  string
		want  string
	}{
		{"sources", "Source1", id1.Hex()},
		{"sources", "Nonexistent", ""},
		{"targets", "Target1", id1.Hex()},
		{"targets", "Nonexistent", ""},
		{"tools", "Tool1", id1.Hex()},
		{"tools", "Nonexistent", ""},
		{"controls", "Control1", id1.Hex()},
		{"controls", "Nonexistent", ""},
		{"tags", "Tag1", id1.Hex()},
		{"tags", "Nonexistent", ""},
		{"datasources", "DS1", id2.Hex()},
		{"datasources", "Nonexistent", ""},
		{"rules", "Rule1", id2.Hex()},
		{"rules", "Nonexistent", ""},
		{"unknown", "anything", ""},
	}

	for _, tt := range tests {
		t.Run(tt.field+"_"+tt.name, func(t *testing.T) {
			got := findExistingMultiEntry(assessment, tt.field, tt.name)
			if got != tt.want {
				t.Errorf("findExistingMultiEntry(%q, %q) = %q, want %q", tt.field, tt.name, got, tt.want)
			}
		})
	}
}
