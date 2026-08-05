// Package content is the domain layer over the reference-content registry.
//
// Storage lives in internal/store/content. This package owns the product rules
// that sit on top of those rows: who may enable a source, what delete refuses,
// and whether a source may accept a new reference from an engagement picker
// (M2-002). Adapters and the job runner (M2-003+) land here too.
//
// Nothing in this package decides authorization. api/openapi.yaml maps the
// HTTP surface to content.read / content.manage / content.sync, and the
// authorization middleware refuses everybody else before a handler is entered
// (M1-013). What lives here is what a change *means*.
package content
