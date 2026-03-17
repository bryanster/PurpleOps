package handler

import (
	"testing"
)

func TestSanitizeFilenameSafe(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "_.._.._etc_passwd"},
		{"path/to/file.txt", "path_to_file.txt"},
		{".hidden", "hidden"},
		{"...dots", "dots"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
		{"file with spaces.txt", "file with spaces.txt"},
		{"back\\slash.txt", "back_slash.txt"},
	}

	for _, tt := range tests {
		got := sanitizeFilenameSafe(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilenameSafe(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
