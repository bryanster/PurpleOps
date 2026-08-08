//go:build spa

package web

import (
	"io/fs"
	"strings"
	"testing"
)

// Only meaningful in the build that carries the real frontend: `make test-spa`,
// after `make build`. It is the check that the tag actually captured a Vite
// build rather than a directory with an index.html in it — an embed that
// matched a stale or half-written dist compiles perfectly.
func TestTheEmbeddedDistIsAViteBuild(t *testing.T) {
	t.Parallel()

	dist, isSPA := Dist()
	if !isSPA {
		t.Fatal("built with -tags spa but Dist reports a placeholder")
	}

	page, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	if !strings.Contains(string(page), "/assets/") {
		t.Errorf("index.html loads nothing from /assets/; this is not a built app:\n%s", page)
	}

	assets, err := fs.ReadDir(dist, "assets")
	if err != nil {
		t.Fatalf("reading assets/: %v", err)
	}
	var scripts int
	for _, asset := range assets {
		if strings.HasSuffix(asset.Name(), ".js") {
			scripts++
		}
	}
	if scripts == 0 {
		t.Errorf("assets/ holds no scripts (%d entries)", len(assets))
	}
}
