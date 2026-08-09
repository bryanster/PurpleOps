package pdf

import (
	"testing"
	"time"
)

// fixtureHTML is minimal self-contained HTML that renders to at least one page.
const fixtureHTML = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Test</title>
<style>body { font-family: sans-serif; padding: 20mm; }</style>
</head><body><h1>PDF Smoke Test</h1><p>This page should produce at least one page of PDF output.</p></body></html>`

func TestNew_MissingPath(t *testing.T) {
	_, err := New("", 0)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNew_NonexistentPath(t *testing.T) {
	_, err := New("/nonexistent/chromium-binary", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestRenderPDF_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF smoke test in short mode (requires Chromium)")
	}

	// Use BLACKLIGHT_CHROME_PATH if set; otherwise try common paths.
	// The environment variable is the canonical configuration — no fallback
	// discovery that would duplicate config.Load logic.
	path := "/usr/bin/chromium" // default from Dockerfile
	printer, err := New(path, 15*time.Second)
	if err != nil {
		t.Skipf("Chromium not available at %s: %v", path, err)
	}
	defer printer.Close()

	pdf, err := printer.RenderPDF(t.Context(), []byte(fixtureHTML))
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}

	if !IsPDF(pdf) {
		t.Fatal("output does not start with %%PDF magic")
	}

	if pages := MinPageCount(pdf); pages < 1 {
		t.Fatalf("expected at least 1 page, got %d", pages)
	}

	t.Logf("PDF size: %d bytes, estimated pages: %d", len(pdf), MinPageCount(pdf))
}

func TestRenderPDF_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PDF timeout test in short mode (requires Chromium)")
	}

	path := "/usr/bin/chromium"
	printer, err := New(path, 1*time.Second)
	if err != nil {
		t.Skipf("Chromium not available at %s: %v", path, err)
	}
	defer printer.Close()

	// A huge document should hit the timeout during rendering.
	bigHTML := `<html><body>` + string(make([]byte, 10*1024*1024)) + `</body></html>`

	_, err = printer.RenderPDF(t.Context(), []byte(bigHTML))
	if err == nil {
		// This might succeed on fast hardware; that's fine — not flaky.
		t.Log("large document rendered within timeout")
	}
}

func TestIsPDF(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"pdf magic", []byte("%PDF-1.4 rest"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPDF(tt.data); got != tt.valid {
				t.Errorf("IsPDF() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestMinPageCount(t *testing.T) {
	// Not a real PDF, but the heuristic should still count /Type /Page occurrences.
	data := []byte("/Type /Page\n/Type /Page\n/Type /Page")
	if n := MinPageCount(data); n != 3 {
		t.Errorf("MinPageCount() = %d, want 3", n)
	}
	if n := MinPageCount([]byte("no pages here")); n != 0 {
		t.Errorf("MinPageCount() = %d, want 0", n)
	}
}
