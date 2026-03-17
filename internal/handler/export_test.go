package handler

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFlattenValue(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"*bool true", &trueVal, "true"},
		{"*bool false", &falseVal, "false"},
		{"*bool nil", (*bool)(nil), ""},
		{"string slice", []string{"a", "b", "c"}, "a, b, c"},
		{"empty string slice", []string{}, ""},
		{"interface slice", []interface{}{"x", 1, true}, "x, 1, true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenValue(tt.input)
			if got != tt.want {
				t.Errorf("flattenValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteCSVExport(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "test.csv")

	records := []map[string]interface{}{
		{"name": "TC1", "mitreid": "T1059", "outcome": "Prevented"},
		{"name": "TC2", "mitreid": "T1053", "outcome": "Missed"},
	}

	if err := writeCSVExport(outPath, records); err != nil {
		t.Fatalf("writeCSVExport error: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("failed to open CSV: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	// 1 header + 2 data rows
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}

	// Check headers contain expected fields
	headers := rows[0]
	hasName := false
	for _, h := range headers {
		if h == "name" {
			hasName = true
		}
	}
	if !hasName {
		t.Error("CSV headers should contain 'name'")
	}
}

func TestWriteCSVExportEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "empty.csv")

	if err := writeCSVExport(outPath, nil); err != nil {
		t.Fatalf("writeCSVExport error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestCreateZip(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("world"), 0644)

	zipPath := filepath.Join(t.TempDir(), "test.zip")

	if err := createZip(zipPath, srcDir); err != nil {
		t.Fatalf("createZip error: %v", err)
	}

	// Verify ZIP contents
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Errorf("expected 2 files in ZIP, got %d", len(r.File))
	}

	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	if !names["file1.txt"] {
		t.Error("ZIP should contain file1.txt")
	}
	if !names[filepath.Join("subdir", "file2.txt")] {
		t.Error("ZIP should contain subdir/file2.txt")
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("alpha"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("beta"), 0644)

	dstDir := filepath.Join(t.TempDir(), "copy")
	os.MkdirAll(dstDir, 0755)

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	// Check copied files
	data, err := os.ReadFile(filepath.Join(dstDir, "a.txt"))
	if err != nil {
		t.Fatalf("failed to read copied a.txt: %v", err)
	}
	if string(data) != "alpha" {
		t.Errorf("a.txt content = %q, want %q", string(data), "alpha")
	}

	data, err = os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("failed to read copied sub/b.txt: %v", err)
	}
	if string(data) != "beta" {
		t.Errorf("sub/b.txt content = %q, want %q", string(data), "beta")
	}
}

func TestCreateZipEmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "empty.zip")

	if err := createZip(zipPath, srcDir); err != nil {
		t.Fatalf("createZip error: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("expected 0 files in ZIP, got %d", len(r.File))
	}
}

func TestWriteCSVExportWithSliceValues(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "slices.csv")

	records := []map[string]interface{}{
		{
			"name":  "TC1",
			"tools": []string{"nmap", "burp"},
			"tags":  []interface{}{"red", "internal"},
		},
	}

	if err := writeCSVExport(outPath, records); err != nil {
		t.Fatalf("writeCSVExport error: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("failed to open CSV: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("expected 2 rows (header + 1 data), got %d", len(rows))
	}

	// Find the tools column and verify it's comma-separated
	headers := rows[0]
	for i, h := range headers {
		if h == "tools" {
			if rows[1][i] != "nmap, burp" {
				t.Errorf("tools column = %q, want %q", rows[1][i], "nmap, burp")
			}
		}
	}
}

func TestFlattenValueFormats(t *testing.T) {
	// Test various fmt.Sprintf fallback cases
	type custom struct{ X int }
	got := flattenValue(custom{42})
	want := fmt.Sprintf("%v", custom{42})
	if got != want {
		t.Errorf("flattenValue(custom) = %q, want %q", got, want)
	}
}
