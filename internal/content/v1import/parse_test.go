package v1import_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bryanster/blacklight/internal/content/v1import"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "v1import")
}

func TestParseTestcasesJSON(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "testcases.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := v1import.ParseBytes(raw, "testcases.json", v1import.FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if b.Format != v1import.FormatTestcasesJSON {
		t.Fatalf("format: %s", b.Format)
	}
	if len(b.Testcases) != 2 {
		t.Fatalf("got %d testcases, want 2", len(b.Testcases))
	}
	if b.Testcases[0].Name == "" || b.Testcases[0].Command == "" {
		t.Fatalf("first testcase incomplete: %+v", b.Testcases[0])
	}
	if b.Testcases[0].ExternalID == "" {
		t.Fatal("missing external id")
	}
	// Flat actions → warning about absent cleanup/args.
	if len(b.Warnings) == 0 {
		t.Fatal("expected import warnings for flat actions")
	}
}

func TestParseTestcasesYAMLDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(fixtureDir(t), "testcases")
	b, err := v1import.ParsePath(dir, v1import.FormatTestcasesYAML)
	if err != nil {
		t.Fatal(err)
	}
	// broken.yaml is an error; the two valid files should parse.
	if len(b.Testcases) != 2 {
		t.Fatalf("testcases=%d errors=%v", len(b.Testcases), b.Errors)
	}
	if len(b.Errors) == 0 {
		t.Fatal("expected broken.yaml error")
	}
	// lsass-procexp.yaml has no name — derived from objective.
	var found bool
	for _, tc := range b.Testcases {
		if tc.Name == "Dump LSASS ProcExp" || tc.Command == "procexp.exe" {
			found = true
			if len(tc.TechniqueExternalIDs) != 1 || tc.TechniqueExternalIDs[0] != "T1003.001" {
				t.Fatalf("technique: %+v", tc.TechniqueExternalIDs)
			}
		}
	}
	if !found {
		t.Fatalf("lsass testcase missing: %+v", b.Testcases)
	}
}

func TestParseKnowledgebaseYAML(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(fixtureDir(t), "knowledgebase")
	b, err := v1import.ParsePath(dir, v1import.FormatKnowledgebaseYAML)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Notes) != 2 {
		t.Fatalf("notes=%d errors=%v", len(b.Notes), b.Errors)
	}
	for _, n := range b.Notes {
		if n.BodyMarkdown == "" {
			t.Fatalf("empty body: %+v", n)
		}
		if n.Title == "" {
			t.Fatalf("empty title: %+v", n)
		}
	}
}

func TestParseHybridZipAuto(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "hybrid.zip"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := v1import.ParseBytes(raw, "hybrid.zip", v1import.FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Testcases) < 3 {
		t.Fatalf("testcases=%d", len(b.Testcases))
	}
	if len(b.Notes) != 2 {
		t.Fatalf("notes=%d", len(b.Notes))
	}
}

func TestExternalIDStable(t *testing.T) {
	t.Parallel()
	a := v1import.ExternalIDForTestcase("", "Service Execution via sc.exe", "x")
	b := v1import.ExternalIDForTestcase("", "Service Execution via sc.exe", "y")
	if a != b {
		t.Fatalf("name-derived ids differ: %s vs %s", a, b)
	}
	c := v1import.ExternalIDForNote("T1003", "T1003.yaml")
	d := v1import.ExternalIDForNote("t1003", "other.yaml")
	if c != d {
		t.Fatalf("note ids differ: %s vs %s", c, d)
	}
}
