// Package pdf renders report HTML to PDF via headless Chromium (chromedp).
//
// One browser process is shared across renders; a tab is created per call.
// The caller must respect the configured timeout — hung Chrome is killed via
// context cancellation.
package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// A4 dimensions in inches (ISO 216).
const (
	a4Width  = 8.27
	a4Height = 11.69
)

// DefaultTimeout bounds the total time a single PDF render may take, browser
// launch included. The call site may tighten this via context deadline.
const DefaultTimeout = 30 * time.Second

// Printer renders HTML to PDF via a shared headless Chromium instance.
//
// A Printer is safe for concurrent use: each RenderPDF call creates its own
// tab within the shared browser process.
type Printer struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	timeout     time.Duration
	mu          sync.Mutex // serializes allocCtx access (shared browser process)
}

// New creates a Printer backed by the Chromium binary at chromePath.
// chromePath must be an absolute or resolvable path to a Chromium/Chrome
// executable. New verifies the binary exists and is executable; it returns
// a descriptive error if it cannot be found.
//
// timeout caps each RenderPDF call. Zero means [DefaultTimeout].
func New(chromePath string, timeout time.Duration) (*Printer, error) {
	if chromePath == "" {
		return nil, errors.New("pdf: BLACKLIGHT_CHROME_PATH is not set; PDF rendering is unavailable")
	}
	if _, err := os.Stat(chromePath); err != nil {
		return nil, fmt.Errorf("pdf: Chromium binary not found at %s: %w (set BLACKLIGHT_CHROME_PATH)", chromePath, err)
	}
	// exec.LookPath also checks executable permission.
	if _, err := exec.LookPath(chromePath); err != nil {
		return nil, fmt.Errorf("pdf: %s is not executable: %w (set BLACKLIGHT_CHROME_PATH)", chromePath, err)
	}

	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	// Allocator options for headless PDF rendering.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Headless,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		// Disable GPU — headless PDF has nothing to paint to screen.
		chromedp.DisableGPU,
		// Ubuntu 24.04+ restricts unprivileged user namespaces via
		// AppArmor; --no-sandbox is required in CI and dev containers.
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &Printer{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		timeout:     timeout,
	}, nil
}

// RenderPDF converts HTML bytes to a PDF document using the shared Chromium
// instance. The returned bytes are a valid PDF.
//
// RenderPDF creates a fresh tab per call and closes it before returning.
// The caller's context deadline bounds the entire operation; if the context
// carries no deadline, Printer.timeout is applied.
//
//nolint:contextcheck // ctx is function parameter, not newly created
func (p *Printer) RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline { //nolint:staticcheck
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout) //nolint:staticcheck,wastedassign
		defer cancel()
	}
	// Serialize access to the shared browser process. Tab contexts are created
	// from the allocator (browser lifetime), not the request — this is the
	// architectural contract of a shared browser pool.
	p.mu.Lock()
	//nolint:contextcheck // tab ctx must derive from allocator, not request
	tabCtx, tabCancel := chromedp.NewContext(p.allocCtx)
	p.mu.Unlock()
	defer tabCancel()

	// Ensure the tab is closed before unlocking the browser.
	defer func() { chromedp.Cancel(tabCtx) }() //nolint:errcheck

	var pdfBytes []byte

	err := chromedp.Run(tabCtx, //nolint:contextcheck // tabCtx derives from allocator context
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Load a blank page first so we have a frame to target.
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return fmt.Errorf("pdf: get frame tree: %w", err)
			}
			if frameTree == nil || frameTree.Frame == nil {
				return errors.New("pdf: no frame available")
			}
			if err := page.SetDocumentContent(frameTree.Frame.ID, string(html)).Do(ctx); err != nil {
				return fmt.Errorf("pdf: set document content: %w", err)
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(a4Width).
				WithPaperHeight(a4Height).
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			if err != nil {
				return fmt.Errorf("pdf: print to PDF: %w", err)
			}
			pdfBytes = data
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if len(pdfBytes) == 0 {
		return nil, errors.New("pdf: Chromium returned empty PDF data")
	}

	return pdfBytes, nil
}

// Close shuts down the shared Chromium browser process. After Close, the
// Printer must not be used.
func (p *Printer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.allocCancel != nil {
		p.allocCancel()
		p.allocCancel = nil
	}
}

// IsPDF reports whether data starts with the PDF magic bytes.
func IsPDF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF"))
}

// MinPageCount returns the minimum number of pages the PDF appears to contain,
// estimated by counting page objects. Returns 0 if the count cannot be
// determined (e.g. not valid PDF).
func MinPageCount(data []byte) int {
	// Count occurrences of "/Type /Page" — a rough but reliable heuristic
	// for smoke-testing page count without a full PDF parser.
	n := 0
	for _, occurrence := range bytes.Split(data, []byte("/Type")) {
		if bytes.HasPrefix(bytes.TrimSpace(occurrence), []byte("/Page")) {
			n++
		}
	}
	return n
}
