package attack

import (
	"context"
	"fmt"
	"strings"

	"github.com/bryanster/blacklight/internal/content"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// DefaultIndexPath is appended to the source base URL to discover releases.
const DefaultIndexPath = "index.json"

// Adapter is the ATT&CK Enterprise content adapter.
//
// HTTP is optional; when nil, Fetch uses the client the runner injects on
// FetchRequest. Tests set FetchBytes / IndexBytes to avoid network I/O.
type Adapter struct {
	// FetchBytes, when set, short-circuits Fetch with these STIX (or archive)
	// bytes. Version comes from req.Version or the bundle's collection label.
	FetchBytes []byte

	// IndexBytes, when set, is the index.json body used for latest discovery
	// instead of an HTTP GET.
	IndexBytes []byte

	// FetchErr forces Fetch to fail (runner failure-path tests).
	FetchErr error
}

// New returns a production ATT&CK adapter.
func New() *Adapter { return &Adapter{} }

// Kind implements [content.Adapter].
func (a *Adapter) Kind() storecontent.Kind { return storecontent.KindAttack }

// Fetch implements [content.Adapter].
func (a *Adapter) Fetch(ctx context.Context, req content.FetchRequest) (content.Bundle, error) {
	if err := ctx.Err(); err != nil {
		return content.Bundle{}, err
	}
	if a.FetchErr != nil {
		return content.Bundle{}, a.FetchErr
	}
	if len(a.FetchBytes) > 0 {
		version := strings.TrimSpace(req.Version)
		if version == "" {
			// Prefer collection label inside the fixture when the caller did
			// not pin a version.
			if v, err := peekBundleVersion(a.FetchBytes); err == nil && v != "" {
				version = v
			}
		}
		if version == "" {
			return content.Bundle{}, fmt.Errorf("attack: fetch: fixture bytes need a version pin or collection label")
		}
		return content.Bundle{
			Bytes:   append([]byte(nil), a.FetchBytes...),
			Size:    int64(len(a.FetchBytes)),
			Version: version,
		}, nil
	}

	base := strings.TrimRight(strings.TrimSpace(req.Source.URL), "/")
	if base == "" {
		return content.Bundle{}, fmt.Errorf("attack: fetch: source URL is empty")
	}
	ref := strings.TrimSpace(req.Source.Ref)
	if ref == "" {
		ref = "enterprise-attack/enterprise-attack-{version}.json"
	}

	version := strings.TrimSpace(req.Version)
	if version == "" {
		var err error
		version, err = a.discoverLatest(ctx, req, base)
		if err != nil {
			return content.Bundle{}, err
		}
	}

	bundleURL := buildBundleURL(base, ref, version)
	raw, err := content.ReadAll(ctx, content.HTTPSource{
		URL:      bundleURL,
		MaxBytes: req.MaxBytes,
		Client:   req.HTTP,
		Policy:   req.Policy,
	})
	if err != nil {
		return content.Bundle{}, fmt.Errorf("attack: fetch %s: %w", bundleURL, err)
	}
	return content.Bundle{
		Bytes:     raw,
		Size:      int64(len(raw)),
		Version:   version,
		MediaType: "application/json",
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
			return nil, fmt.Errorf("attack: parse: read path: %w", err)
		}
	}
	doc, err := parseSTIX(raw)
	if err != nil {
		return nil, err
	}
	if bundle.Version != "" && doc.Version != "" && bundle.Version != doc.Version {
		return nil, fmt.Errorf(
			"attack: parse: version label mismatch: job/fetch wants %q, bundle collection is %q",
			bundle.Version, doc.Version,
		)
	}
	if doc.Version == "" {
		doc.Version = bundle.Version
	}
	if doc.Version == "" {
		return nil, fmt.Errorf("attack: parse: bundle has no ATT&CK version label")
	}
	return doc, nil
}

// Normalize implements [content.Adapter].
func (a *Adapter) Normalize(ctx context.Context, ast content.AST) ([]content.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, ok := ast.(*stixDoc)
	if !ok {
		return nil, fmt.Errorf("attack: normalize: unexpected AST type %T", ast)
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
		return fmt.Errorf("attack: apply: expected 1 catalog object, got %d", len(objects))
	}
	cat, ok := objects[0].(*Catalog)
	if !ok {
		return fmt.Errorf("attack: apply: unexpected object type %T", objects[0])
	}
	if cat.Version == "" {
		return fmt.Errorf("attack: apply: catalog version is empty")
	}
	if w.Version() != "" && w.Version() != cat.Version {
		return fmt.Errorf("attack: apply: writer version %q != catalog %q", w.Version(), cat.Version)
	}
	return applyCatalog(ctx, w, cat, prog)
}

func (a *Adapter) discoverLatest(ctx context.Context, req content.FetchRequest, base string) (string, error) {
	var raw []byte
	if len(a.IndexBytes) > 0 {
		raw = a.IndexBytes
	} else {
		indexURL := base + "/" + DefaultIndexPath
		var err error
		raw, err = content.ReadAll(ctx, content.HTTPSource{
			URL:      indexURL,
			MaxBytes: req.MaxBytes,
			Client:   req.HTTP,
			Policy:   req.Policy,
		})
		if err != nil {
			return "", fmt.Errorf("attack: discover latest via %s: %w", indexURL, err)
		}
	}
	version, err := latestEnterpriseVersion(raw)
	if err != nil {
		return "", err
	}
	return version, nil
}

func buildBundleURL(base, ref, version string) string {
	path := strings.ReplaceAll(ref, "{version}", version)
	path = strings.TrimLeft(path, "/")
	return base + "/" + path
}

// Ensure Adapter satisfies the interface at compile time.
var _ content.Adapter = (*Adapter)(nil)
