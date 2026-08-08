package web

import (
	"io/fs"
	"strings"
	"testing"
)

// Dist has to hold up in both builds: the server serves it the same way either
// way, and the only thing it insists on is an entry point.
func TestDistHasAnIndexPage(t *testing.T) {
	t.Parallel()

	dist, isSPA := Dist()
	if dist == nil {
		t.Fatal("Dist returned no filesystem")
	}

	page, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	if !strings.Contains(string(page), "<html") {
		t.Errorf("index.html is not an HTML page:\n%s", page)
	}

	// The placeholder must say why it is there. Without the `spa` tag this is
	// what an operator sees, and "Blacklight has a blank page" is not a bug
	// report anyone can act on.
	if !isSPA && !strings.Contains(string(page), "build tag") {
		t.Errorf("the placeholder page does not explain itself:\n%s", page)
	}
}
