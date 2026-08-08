// Package content is the domain layer over the reference-content registry and
// the global content job runner.
//
// Storage lives in internal/store/content. This package owns the product rules
// that sit on top of those rows: who may enable a source, what delete refuses,
// whether a source may accept a new reference from an engagement picker
// (M2-002), the Adapter pipeline, the single-slot job runner (M2-003), and
// custom content CRUD + export under the singleton custom source (M2-011).
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
// Offline bundle upload and reprocess-from-raw (M2-005) skip Fetch and feed the
// same Parse → Normalize → Apply path from a local file (spooled upload or the
// last raw snapshot). See docs/content-bundles.md for the operator contract.
//
// Rolling sources (Atomic, Sigma, CTID, custom) always write version token
// "current" and replace objects for that token. ATT&CK multi-version applies
// into the target version key only. On failure the prior successful catalog is
// left intact; on success a raw snapshot is stored under the configured content
// data root and the previous raw for that version is deleted.
//
// # Custom content
//
// [Custom] is the home for user-authored procedure templates, detection rule
// refs, and KB notes. All mutations attach to the seeded kind=custom source.
// ATT&CK does not need to be installed to author templates. Delete answers 409
// when referenced; the M2 ref counter is a stub that always reports zero until
// M3. Export produces YAML/JSON suitable for re-import (M2-012).
package content
