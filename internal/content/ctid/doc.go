// Package ctid implements the CTID adversary-emulation plan catalog adapter
// (M2-010).
//
// # Fetch
//
// The seeded source row carries:
//
//	url = https://github.com/center-for-threat-informed-defense/adversary_emulation_library/archive/refs/heads/master.zip
//	ref = master
//
// Fetch GETs the source URL (a GitHub archive zip of the repository). Offline
// bundle upload accepts the same zip bytes. Tests inject FetchBytes so CI never
// hits the network.
//
// # Parse
//
// Walks machine-readable plan YAML under:
//
//	{actor}/Emulation_Plan/yaml/*.yaml
//
// inside the archive file-by-file. One content_emulation_plan is produced per
// plan file that carries an emulation_plan_details header; each subsequent
// list entry becomes an ordered content_emulation_plan_step. Unknown layouts
// (no Emulation_Plan/yaml tree, missing details header, zero steps) fail the
// job rather than half-import. Micro-emulation plans and CALDERA planner YAML
// under planners/ are out of scope for this adapter.
//
// # External ids
//
// Plan: prefer upstream emulation_plan_details.id; else the actor directory
// slug (for example fin6). Step: prefer upstream step id; else
// {plan_external_id}/{1-based-position}. Documented in docs/content-ctid.md.
//
// # Ordinals
//
// Step position is 1-based document order within the plan YAML (dense, unique
// per plan). Upstream procedure_step labels (for example "2.1") are preserved
// inside the procedure JSON for display; they are not used as the ordinal.
//
// # Rolling head
//
// Version token is always "current". Apply stages under __staging__ then
// promotes in one store.Write transaction so a failed re-sync leaves the prior
// ready catalog intact.
//
// # Catalog only
//
// M2 stores plans for library browse. Turning a plan into an engagement
// Scenario is M3-013 — see docs/content-ctid.md § M3 import contract and
// docs/content-copy-on-use.md.
//
// # Operator docs
//
// See docs/content-ctid.md for the archive shape and offline bundle notes.
package ctid
