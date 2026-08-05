package attackpin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyOnUseContractDocExists(t *testing.T) {
	t.Parallel()
	// Walk up from this package to the repo root.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	for range 6 {
		candidate := filepath.Join(dir, "docs", "content-copy-on-use.md")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			doc = candidate
			break
		}
		dir = filepath.Dir(dir)
	}
	if doc == "" {
		t.Fatal("docs/content-copy-on-use.md missing — M2-007 copy-on-use contract")
	}
	b, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, needle := range []string{
		"Copy-on-use",
		"AssertPinned",
		"ResolveTechnique",
		"attack_version",
		"snapshot",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("contract doc missing %q", needle)
		}
	}
}
