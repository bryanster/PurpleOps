// Package content stores the reference-content registry and its object tables
// (M2-001).
//
// It is storage and nothing else. Fetching upstream archives, parsing STIX /
// YAML, running jobs and answering "may they install this" all sit above it.
// A repository here writes what it is handed and reads it back; the only
// judgement it makes is about rows that are absent ([apierr.NotFound]) and
// about raw snapshot paths that would escape the configured content data root.
//
// # Two schemas, one database
//
// PLAN.md §2 keeps reference data in the content schema and engagement data in
// app. Nothing in this package references app tables. Engagement steps will
// later snapshot procedure/plan fields and may keep a weak template_id lineage
// pointer with no foreign key from app to content — that contract is deliberate
// so reinstalling ATT&CK cannot cascade into war-room history.
//
// # Rolling vs multi-version
//
// ATT&CK is multi-version: object rows for "14.1" and "15.1" coexist under the
// same source_id, distinguished by the version text column. Atomic, Sigma and
// CTID are rolling heads. They use the single version token [VersionCurrent]
// ("current"), and a re-sync replaces objects for that token inside one
// write transaction (stage-and-swap or delete-and-insert per family — adapters
// choose; repositories expose the primitives).
//
// # No repository owns a database
//
// Every repository is constructed with a [DB] — the store's serialized writer
// and pooled reader, and nothing wider. None of them holds a *sql.DB, so none
// of them can open a transaction outside [store.DB.Write]. There is no
// package-level handle to reach for (PLAN.md §6).
//
// # Delete is application-enforced
//
// DuckDB cannot hold foreign keys pointing at content_source or
// content_source_version without making those bookkeeping rows un-updatable
// (see migration 0011_content.sql and 0003_user_updatable.sql). Product delete
// (M2-002) must clear version and object rows in one write transaction before
// removing the source. The schema does not cascade within content and has no
// path into app.
package content
