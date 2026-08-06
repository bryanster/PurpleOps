// Package engagement stores the workbook graph: engagements, their scenarios,
// steps, executions, findings, evidence and comments.
//
// It is storage and nothing else. Deciding whether a step may be edited
// (soft freeze), deriving a detection outcome (M3-008), and answering "may
// they" all sit above it. A repository here writes what it is handed and reads
// it back; the only judgement it makes is about rows that are absent, which it
// reports as [apierr.NotFound] so that a handler does not have to translate
// sql.ErrNoRows for itself.
//
// # No rounds
//
// Per M3-EPIC, retest rounds are out of v1. Execution is 1:1 with step
// (UNIQUE(step_id)). There is no round table, no round_id column, and no
// (step_id, round_id) grain anywhere in this package.
//
// # No repository owns a database
//
// Every repository is constructed with a [DB] — the store's serialized writer
// and pooled reader, and nothing wider. None of them holds a *sql.DB, so none
// of them can open a transaction outside [store.DB.Write], which is what keeps
// the single-writer rule from being a convention.
//
// # Optimistic locking
//
// Execution mutations require the caller's version. A mismatch returns
// [apierr.Conflict] with the current row so the caller can retry. Version is
// incremented atomically in the UPDATE, never in application code.
//
// # Copy-on-use
//
// Lineage columns (template_id, plan_id) are TEXT with no foreign key to
// content. Content rows are replaceable; app rows must survive a content
// re-sync. See docs/content-copy-on-use.md.
package engagement
