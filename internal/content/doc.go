// Package content is the domain layer over the reference-content registry and
// the global content job runner.
//
// Storage lives in internal/store/content. This package owns the product rules
// that sit on top of those rows: who may enable a source, what delete refuses,
// whether a source may accept a new reference from an engagement picker
// (M2-002), the Adapter pipeline, and the single-slot job runner (M2-003).
//
// Nothing in this package decides authorization. api/openapi.yaml maps the
// HTTP surface to content.read / content.manage / content.sync, and the
// authorization middleware refuses everybody else before a handler is entered
// (M1-013). What lives here is what a change *means*.
//
// # Job runner
//
// At most one content job is queued or running in the installation. StartSync
// enforces the gate with 409; the worker drains the queue. Adapters implement
// Fetch → Parse → Normalize → Apply. Apply writes only through Writer batches
// and never holds the store write lock across network I/O. Progress is
// persisted on the job row and fanned out to in-process subscribers so M2-004
// can wire SSE without rewriting the runner.
//
// Rolling sources (Atomic, Sigma, CTID) always write version token "current"
// and replace objects for that token. ATT&CK multi-version applies into the
// target version key only. On failure the prior successful catalog is left
// intact; on success a raw snapshot is stored under the configured content
// data root and the previous raw for that version is deleted.
package content
