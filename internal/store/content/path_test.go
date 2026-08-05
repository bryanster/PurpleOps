package content_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/store/content"
)

func TestRawRelAndAbsRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := content.NewPaths(root)

	rel, err := p.RawRel(content.SourceIDAttack, "15.1", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if want := "raw/" + content.SourceIDAttack + "/15.1/abc123"; rel != want {
		t.Fatalf("RawRel = %q, want %q", rel, want)
	}

	abs, err := p.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(abs, root) {
		t.Fatalf("Abs = %q, not under root %q", abs, root)
	}
	if filepath.Base(abs) != "abc123" {
		t.Fatalf("Abs base = %q", filepath.Base(abs))
	}
}

func TestCleanRelRejectsEscapes(t *testing.T) {
	t.Parallel()
	p := content.NewPaths(t.TempDir())

	cases := []string{
		"",
		"/etc/passwd",
		`C:\windows\system32`,
		"../secret",
		"raw/../../etc/passwd",
		"raw/foo/../../../etc/passwd",
		"raw/./../../x",
	}
	for _, rel := range cases {
		if _, err := p.CleanRel(rel); err == nil {
			t.Errorf("CleanRel(%q) accepted", rel)
		}
		if _, err := p.Abs(rel); err == nil {
			t.Errorf("Abs(%q) accepted", rel)
		}
	}
}

func TestRawRelRejectsUnsafeSegments(t *testing.T) {
	t.Parallel()
	p := content.NewPaths(t.TempDir())

	if _, err := p.RawRel("../x", "15.1", "abc"); err == nil {
		t.Error("accepted source id with ..")
	}
	if _, err := p.RawRel("src", "a/b", "abc"); err == nil {
		t.Error("accepted version with slash")
	}
	if _, err := p.RawRel("src", "15.1", ""); err == nil {
		t.Error("accepted empty sha")
	}
}
