// Package report stores report drafts and their ordered block instances.
//
// It is storage and nothing else. Deciding whether a block_id is known
// (registry), validating params (registry.ValidateParams), and applying
// branding defaults all sit above it. A repository here writes what it is
// handed and reads it back; the only judgement it makes is about rows that
// are absent, which it reports as [apierr.NotFound] so that a handler does
// not have to translate sql.ErrNoRows for itself.
//
// # Draft only
//
// report and report_block hold the mutable draft. Published immutable
// versions arrive in M6-011. No version columns exist in this package.
//
// # No repository owns a database
//
// Every repository is constructed with a [DB] — the store's serialized
// writer and pooled reader, and nothing wider. None of them holds a *sql.DB,
// so none of them can open a transaction outside [store.DB.Write].
//
// # Branding
//
// client_name, logo_blob_ref and colours are nullable overrides. NULL means
// "fall back to install defaults" — the service layer applies the
// precedence, not this package.
package report
