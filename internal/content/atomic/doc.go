// Package atomic implements the Atomic Red Team content adapter (M2-008).
//
// # Fetch
//
// The seeded source row carries:
//
//	url = https://github.com/redcanaryco/atomic-red-team/archive/refs/heads/master.zip
//	ref = master
//
// Fetch GETs the source URL (a GitHub archive zip of the repository). Offline
// bundle upload accepts the same zip bytes (or a directory of atomics YAML).
// Tests inject FetchBytes so CI never hits the network.
//
// # Parse
//
// Walks atomics/Txxxx/Txxxx.yaml files inside the archive (or a bare directory
// of those YAML files). One content_procedure_template is produced per
// atomic_tests entry — not one row per technique file.
//
// # External ids
//
// Prefer upstream auto_generated_guid when present. Otherwise derive
// "{attack_technique}/{zero-based-index}" so re-sync upserts rather than
// duplicating. See docs/content-atomic.md.
//
// # Rolling head
//
// Version token is always "current". Apply stages under __staging__ then
// promotes in one store.Write transaction so a failed re-sync leaves the prior
// ready catalog intact.
//
// # Operator docs
//
// See docs/content-atomic.md for the archive shape and offline bundle notes.
package atomic
