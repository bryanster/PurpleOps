package content

import (
	"context"
	"database/sql"
	"io"
	"net/http"

	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Phase names written to content_sync_job.phase and carried on progress events.
// M2-004 maps these onto SSE event types.
const (
	PhaseFetch     = "fetch"
	PhaseParse     = "parse"
	PhaseNormalize = "normalize"
	PhaseApply     = "apply"
	PhaseFinalize  = "finalize"
)

// Adapter turns one upstream kind into content rows.
//
// Pipeline: Fetch → Parse → Normalize → Apply. Fetch is skipped when the job
// already holds a local bundle (reprocess / offline upload — M2-005). Parse and
// Normalize are pure-ish (no store writes). Apply does every write, in batches
// via [Writer], and must not perform network I/O inside a Write callback.
//
// Concrete adapters land in M2-006…M2-010. Tests register a fixture adapter.
type Adapter interface {
	Kind() storecontent.Kind

	// Fetch retrieves upstream bytes (typically HTTPS). req.Version empty means
	// "latest discoverable" for the kind. The returned Bundle may refine the
	// version label (e.g. ATT&CK resolving latest → "15.1").
	Fetch(ctx context.Context, req FetchRequest) (Bundle, error)

	// Parse turns raw bundle bytes into an adapter-private AST.
	Parse(ctx context.Context, bundle Bundle) (AST, error)

	// Normalize flattens the AST into opaque objects ready for Apply.
	Normalize(ctx context.Context, ast AST) ([]Object, error)

	// Apply upserts objects through w. Report progress via prog at batch
	// boundaries. Observe ctx.Done() between batches so cancel is prompt.
	Apply(ctx context.Context, w Writer, objects []Object, prog Progress) error
}

// FetchRequest is what the runner hands an adapter's Fetch.
type FetchRequest struct {
	// Source is the registry row being synced.
	Source SourceInfo

	// Version is the caller-requested pin (e.g. ATT&CK "15.1"). Empty means
	// latest discoverable per the adapter.
	Version string

	// MaxBytes is the download ceiling from config. Fetch must enforce it.
	MaxBytes int64

	// HTTP is the client to use for network I/O. Nil means the runner's
	// policy client (M7-014). Tests inject a short-timeout client.
	HTTP HTTPDoer

	// Policy fences the URLs Fetch may touch (M7-014). Adapters pass it to
	// HTTPSource so a fetch is validated even when HTTP is injected.
	Policy URLPolicy
}

// SourceInfo is the registry subset adapters need. Kept small so a test can
// build one without a database row.
type SourceInfo struct {
	ID   string
	Kind storecontent.Kind
	Name string
	URL  string
	Ref  string
}

// Bundle is fetched (or pre-seated) raw content ready to parse.
type Bundle struct {
	// Bytes is the payload. Prefer this for modest payloads; large adapters may
	// later stream from Path, but Apply's raw-snapshot step always sees Bytes
	// today so the sha256 is of what was parsed.
	Bytes []byte

	// Path is a local filesystem path when the payload was not held in memory
	// (bundle upload / reprocess). Empty when Bytes is set.
	Path string

	// SHA256 is the hex digest of the payload. Empty means the runner computes
	// it from Bytes before persisting the raw snapshot.
	SHA256 string

	// Size is len(Bytes) or the on-disk size. Informational; the runner
	// re-derives from Bytes when present.
	Size int64

	// Version is the resolved version token (ATT&CK release label, or
	// "current" for rolling sources). Required before Apply.
	Version string

	// MediaType is optional advisory content type from the fetch.
	MediaType string
}

// AST is an adapter-private parse tree. The runner never inspects it.
type AST any

// Object is one normalized library row. Opaque to the runner; the same adapter
// that produced it consumes it in Apply.
type Object any

// Writer is how Apply reaches the database. Every call to Write is one
// store.Write transaction — never hold it across network I/O, and keep each
// batch short enough that interactive requests stay healthy (M2-016).
type Writer interface {
	// Write runs fn inside one serialized write transaction. ctx cancel while
	// waiting for the lock aborts without running fn (M0B-003).
	Write(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error

	// SourceID is the registry row the job is writing into.
	SourceID() string

	// Version is the version token objects must carry (never another version's
	// rows — ATT&CK multi-version isolation).
	Version() string

	// BatchSize is the configured WriteBatch hint. Adapters may use it to slice
	// objects; the runner does not slice for them.
	BatchSize() int
}

// Progress reports job phase/counters. The runner persists them on the job row
// and fans them out to in-process subscribers (M2-004 wires SSE on top).
type Progress interface {
	// Report updates phase and counters. message may be empty.
	// Safe for concurrent use from a single job's adapter goroutine only.
	Report(ctx context.Context, phase string, current, total int64, message string)
}

// HTTPDoer is the fragment of *http.Client Fetch needs. *http.Client satisfies
// it; tests inject fakes.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ByteSource yields bytes for Fetch / reprocess / upload without the adapter
// caring which. Implementations: [HTTPSource], [FileSource].
type ByteSource interface {
	// Open returns a reader and the known size (-1 if unknown). Caller closes.
	Open(ctx context.Context) (rc io.ReadCloser, size int64, err error)
}
