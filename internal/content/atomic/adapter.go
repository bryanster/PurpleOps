package atomic

import (
	"context"
	"fmt"
	"strings"

	"github.com/bryanster/blacklight/internal/content"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Adapter is the Atomic Red Team content adapter.
//
// HTTP is optional; when nil, Fetch uses the client the runner injects on
// FetchRequest. Tests set FetchBytes to avoid network I/O.
type Adapter struct {
	// FetchBytes, when set, short-circuits Fetch with these archive (or YAML
	// directory zip) bytes. Version is always storecontent.VersionCurrent.
	FetchBytes []byte

	// FetchErr forces Fetch to fail (runner failure-path tests).
	FetchErr error
}

// New returns a production Atomic adapter.
func New() *Adapter { return &Adapter{} }

// Kind implements [content.Adapter].
func (a *Adapter) Kind() storecontent.Kind { return storecontent.KindAtomic }

// Fetch implements [content.Adapter].
func (a *Adapter) Fetch(ctx context.Context, req content.FetchRequest) (content.Bundle, error) {
	if err := ctx.Err(); err != nil {
		return content.Bundle{}, err
	}
	if a.FetchErr != nil {
		return content.Bundle{}, a.FetchErr
	}
	if len(a.FetchBytes) > 0 {
		return content.Bundle{
			Bytes:     append([]byte(nil), a.FetchBytes...),
			Size:      int64(len(a.FetchBytes)),
			Version:   storecontent.VersionCurrent,
			MediaType: "application/zip",
		}, nil
	}

	url := strings.TrimSpace(req.Source.URL)
	if url == "" {
		return content.Bundle{}, fmt.Errorf("atomic: fetch: source URL is empty")
	}
	raw, err := content.ReadAll(ctx, content.HTTPSource{
		URL:      url,
		MaxBytes: req.MaxBytes,
		Client:   req.HTTP,
	})
	if err != nil {
		return content.Bundle{}, fmt.Errorf("atomic: fetch %s: %w", url, err)
	}
	return content.Bundle{
		Bytes:     raw,
		Size:      int64(len(raw)),
		Version:   storecontent.VersionCurrent,
		MediaType: "application/zip",
	}, nil
}

// Parse implements [content.Adapter].
func (a *Adapter) Parse(ctx context.Context, bundle content.Bundle) (content.AST, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw := bundle.Bytes
	if len(raw) == 0 && bundle.Path != "" {
		var err error
		raw, err = content.ReadAll(ctx, content.FileSource{Path: bundle.Path})
		if err != nil {
			return nil, fmt.Errorf("atomic: parse: read path: %w", err)
		}
	}
	doc, err := parseAtomics(raw)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Normalize implements [content.Adapter].
func (a *Adapter) Normalize(ctx context.Context, ast content.AST) ([]content.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, ok := ast.(*atomicDoc)
	if !ok {
		return nil, fmt.Errorf("atomic: normalize: unexpected AST type %T", ast)
	}
	cat, err := normalize(doc)
	if err != nil {
		return nil, err
	}
	return []content.Object{cat}, nil
}

// Apply implements [content.Adapter].
func (a *Adapter) Apply(ctx context.Context, w content.Writer, objects []content.Object, prog content.Progress) error {
	if len(objects) != 1 {
		return fmt.Errorf("atomic: apply: expected 1 catalog object, got %d", len(objects))
	}
	cat, ok := objects[0].(*Catalog)
	if !ok {
		return fmt.Errorf("atomic: apply: unexpected object type %T", objects[0])
	}
	return applyCatalog(ctx, w, cat, prog)
}

// Ensure Adapter satisfies the interface at compile time.
var _ content.Adapter = (*Adapter)(nil)
