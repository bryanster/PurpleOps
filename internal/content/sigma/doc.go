// Package sigma implements the SigmaHQ content adapter (M2-009).
//
// # Fetch
//
// The seeded source row carries:
//
//	url = https://github.com/SigmaHQ/sigma/archive/refs/heads/master.zip
//	ref = master
//
// Fetch GETs the source URL (a GitHub archive zip of the repository). Offline
// bundle upload accepts the same zip bytes. Tests inject FetchBytes so CI never
// hits the network.
//
// # Parse
//
// Walks rule YAML files under rules/ (and rules-*/ siblings) inside the archive
// file-by-file — bodies are not all buffered before parsing. One
// content_detection_rule_ref is produced per rule that already carries at least
// one ATT&CK technique tag. Unmapped rules are skipped (counted in the job
// message), never stored.
//
// # External ids
//
// Prefer upstream rule `id` when present. Otherwise use the archive-relative
// path (forward slashes, no leading repo-root prefix). Documented in
// docs/content-sigma.md.
//
// # Technique tags
//
// Only tags matching attack.t#### or attack.t####.### (case-insensitive) become
// technique external ids (T1059, T1059.001). Tactic-only tags like
// attack.execution are ignored. Wrong links are worse than skips.
//
// # Rolling head
//
// Version token is always "current". Apply stages under __staging__ then
// promotes in one store.Write transaction so a failed re-sync leaves the prior
// ready catalog intact.
//
// # Reference only
//
// Detection rules are never executed, deployed, or converted. This package has
// no execution API.
//
// # Operator docs
//
// See docs/content-sigma.md for the archive shape and offline bundle notes.
package sigma
