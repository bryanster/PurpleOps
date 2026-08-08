-- 0011_content — the reference-content registry and its object tables (M2-001).
--
-- PLAN.md §2 / §3: content is replaceable. Reinstalling ATT&CK v17 drops and
-- rebuilds content rows for that version; app engagement data must not be
-- dragged along. Every table here lives in the content schema created by 0001,
-- and nothing here references app.* .
--
-- # Foreign keys (and why there are almost none)
--
-- DuckDB implements UPDATE on an indexed table as delete+insert, and the delete
-- half runs ON DELETE RESTRICT against any child that points at the row
-- (see 0003_user_updatable.sql). content_source and content_source_version are
-- bookkeeping rows that are written on every sync — last_synced_at, status,
-- item_count, error — so a foreign key from an object table to either of them
-- would make every successful sync fail the moment the first object landed.
--
-- Referential integrity inside content is therefore application-enforced:
-- repositories require a real source before inserting a version or an object,
-- and product delete (M2-002) clears the content subtree in one write
-- transaction before removing the source. The schema does not cascade within
-- content and cannot cascade into app (there is no path to app from here).
--
-- # Versioning
--
-- ATT&CK is multi-version: rows for 14.1 and 15.1 coexist, keyed by the text
-- version column that matches content_source_version.version. Atomic, Sigma and
-- CTID are rolling heads: they use the single version token 'current', and a
-- re-sync replaces objects for that token inside one transaction (see the
-- package doc on internal/store/content).
--
-- # Object identity
--
-- Every content object has a surrogate UUIDv7 primary key and a natural key
-- UNIQUE (source_id, version, external_id). STIX / upstream IDs are never the
-- primary key (M2-EPIC).
--
-- # Raw snapshots
--
-- The last successful upstream bytes per (source, version) live on disk under
-- the configured content data directory. This schema stores only path + hash +
-- size. Paths are relative to that root; repositories reject anything that
-- would escape it.
--
-- # M3 copy-on-use
--
-- Engagement steps will snapshot procedure/plan fields and may keep a weak
-- template_id lineage pointer. There is deliberately no FK from app to content
-- required here; M3 must not assume one.

-- ---------------------------------------------------------------------------
-- Registry
-- ---------------------------------------------------------------------------

CREATE TABLE content.content_source (
	id TEXT NOT NULL PRIMARY KEY,

	-- Closed vocabulary. New kinds are a migration, not a string somebody
	-- passed to an API.
	kind TEXT NOT NULL,

	name TEXT NOT NULL,

	-- Default HTTPS archive / bundle base URL. Empty for the custom source,
	-- which is never fetched.
	url TEXT NOT NULL,

	-- Adapter-specific ref pattern or branch/tag hint (e.g. a release tag
	-- template). Empty when unused.
	ref TEXT NOT NULL,

	-- Soft switch: disabled sources stay on disk but browse/search/pickers omit
	-- them and APIs refuse new references (M2-EPIC disable semantics).
	enabled BOOLEAN NOT NULL,

	-- Operational state of the source as a whole. Independent of enabled:
	-- a disabled source can still be idle, and an enabled one can be in error.
	status TEXT NOT NULL,

	last_synced_at TIMESTAMP,
	item_count BIGINT NOT NULL,
	error TEXT NOT NULL,

	-- SPDX + human license/attribution shown in the UI and export headers.
	license_spdx TEXT NOT NULL,
	license_name TEXT NOT NULL,
	license_url TEXT NOT NULL,
	attribution TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_source_kind_known
		CHECK (kind IN ('attack', 'atomic', 'sigma', 'ctid', 'custom')),
	CONSTRAINT content_source_status_known
		CHECK (status IN ('idle', 'syncing', 'error')),
	CONSTRAINT content_source_item_count_nonneg
		CHECK (item_count >= 0)
);

-- At most one registry row per kind. Builtin seeds occupy the four upstream
-- kinds; the custom kind is the home for user-authored rows and cannot be
-- multiplied without a product decision.
CREATE UNIQUE INDEX content_source_kind_uidx ON content.content_source (kind);

CREATE TABLE content.content_source_version (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,

	-- Text label: ATT&CK release ("15.1") or the rolling token "current".
	version TEXT NOT NULL,

	status TEXT NOT NULL,
	item_count BIGINT NOT NULL,
	synced_at TIMESTAMP,
	error TEXT NOT NULL,

	-- Last successful raw snapshot. Empty until the first success.
	raw_sha256 TEXT NOT NULL,
	-- Relative to the configured content data root. Never absolute, never "..".
	raw_path TEXT NOT NULL,
	raw_bytes BIGINT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_source_version_natural_key
		UNIQUE (source_id, version),
	CONSTRAINT content_source_version_status_known
		CHECK (status IN ('pending', 'ready', 'error')),
	CONSTRAINT content_source_version_item_count_nonneg
		CHECK (item_count >= 0),
	CONSTRAINT content_source_version_raw_bytes_nonneg
		CHECK (raw_bytes >= 0)
);

CREATE INDEX content_source_version_source_id_idx
	ON content.content_source_version (source_id);

CREATE TABLE content.content_sync_job (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,

	-- Target version when known up front (ATT&CK pin, reprocess). NULL means
	-- "adapter discovers latest" for a sync of a multi-version source.
	version TEXT,

	kind TEXT NOT NULL,
	status TEXT NOT NULL,

	-- Free-text phase label the runner updates ("fetch", "parse", "apply").
	phase TEXT NOT NULL,
	progress_current BIGINT NOT NULL,
	progress_total BIGINT NOT NULL,
	message TEXT NOT NULL,
	error TEXT NOT NULL,

	-- Who started it. Empty for blctl / system. No FK: see header.
	created_by TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	started_at TIMESTAMP,
	finished_at TIMESTAMP,

	-- Opaque runner checkpoint (JSON object). Empty object when unused.
	checkpoint JSON NOT NULL,

	CONSTRAINT content_sync_job_kind_known
		CHECK (kind IN ('sync', 'reprocess', 'bundle_import', 'v1_import')),
	CONSTRAINT content_sync_job_status_known
		CHECK (status IN (
			'queued', 'running', 'cancelling', 'cancelled',
			'succeeded', 'failed', 'interrupted'
		)),
	CONSTRAINT content_sync_job_progress_nonneg
		CHECK (progress_current >= 0 AND progress_total >= 0)
);

CREATE INDEX content_sync_job_source_id_created_at_idx
	ON content.content_sync_job (source_id, created_at DESC);

CREATE INDEX content_sync_job_status_idx
	ON content.content_sync_job (status);

-- ---------------------------------------------------------------------------
-- ATT&CK object families (multi-version; rows duplicated per version)
-- ---------------------------------------------------------------------------

CREATE TABLE content.content_tactic (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_tactic_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_tactic_source_version_idx
	ON content.content_tactic (source_id, version);

CREATE TABLE content.content_technique (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,

	-- Sub-techniques carry the parent's MITRE id (e.g. T1059 for T1059.001).
	-- Empty string when this row is a top-level technique.
	is_subtechnique BOOLEAN NOT NULL,
	parent_external_id TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_technique_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_technique_source_version_idx
	ON content.content_technique (source_id, version);

CREATE INDEX content_technique_parent_idx
	ON content.content_technique (source_id, version, parent_external_id);

-- Technique ↔ tactic membership as a join table so M5 heatmaps can SQL over it
-- without parsing JSON. Natural keys rather than surrogate FKs: bulk
-- version-scoped replace deletes by (source_id, version) without resolving
-- UUIDs, and there is no FK update hazard (see header).
CREATE TABLE content.content_technique_tactic (
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	technique_external_id TEXT NOT NULL,
	tactic_external_id TEXT NOT NULL,

	PRIMARY KEY (source_id, version, technique_external_id, tactic_external_id)
);

CREATE INDEX content_technique_tactic_tactic_idx
	ON content.content_technique_tactic (source_id, version, tactic_external_id);

CREATE TABLE content.content_mitigation (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_mitigation_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_mitigation_source_version_idx
	ON content.content_mitigation (source_id, version);

CREATE TABLE content.content_group (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_group_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_group_source_version_idx
	ON content.content_group (source_id, version);

CREATE TABLE content.content_software (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,

	-- Upstream distinguishes malware from tool. Constrained so a typo cannot
	-- invent a third kind an analytics query never looks for.
	software_type TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_software_natural_key
		UNIQUE (source_id, version, external_id),
	CONSTRAINT content_software_type_known
		CHECK (software_type IN ('malware', 'tool'))
);

CREATE INDEX content_software_source_version_idx
	ON content.content_software (source_id, version);

CREATE TABLE content.content_data_source (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_data_source_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_data_source_source_version_idx
	ON content.content_data_source (source_id, version);

-- ---------------------------------------------------------------------------
-- Atomic + custom structured procedures
-- ---------------------------------------------------------------------------

-- Structure is preserved deliberately. Flattening to one "actions" string was
-- a v1 defect (PLAN.md §3); platforms, executor, command, cleanup and input
-- args stay distinct columns/JSON so the UI and M3 copy-on-use can round-trip
-- them. Engagement steps will snapshot these fields and may store template_id
-- as weak lineage with no FK from app → content.
CREATE TABLE content.content_procedure_template (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,

	platforms JSON NOT NULL,
	executor TEXT NOT NULL,
	elevation_required BOOLEAN NOT NULL,
	command TEXT NOT NULL,
	cleanup TEXT NOT NULL,
	input_args JSON NOT NULL,
	technique_external_ids JSON NOT NULL,
	dependency_executor_name TEXT NOT NULL,
	dependencies TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_procedure_template_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_procedure_template_source_version_idx
	ON content.content_procedure_template (source_id, version);

-- ---------------------------------------------------------------------------
-- Sigma + custom detection references
-- ---------------------------------------------------------------------------

CREATE TABLE content.content_detection_rule_ref (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,

	technique_external_ids JSON NOT NULL,
	level TEXT NOT NULL,
	rule_status TEXT NOT NULL,
	logsource JSON NOT NULL,
	rule_yaml TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_detection_rule_ref_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_detection_rule_ref_source_version_idx
	ON content.content_detection_rule_ref (source_id, version);

-- ---------------------------------------------------------------------------
-- CTID emulation-plan catalog (catalog only in M2; scenario import is M3)
-- ---------------------------------------------------------------------------

CREATE TABLE content.content_emulation_plan (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_emulation_plan_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_emulation_plan_source_version_idx
	ON content.content_emulation_plan (source_id, version);

CREATE TABLE content.content_emulation_plan_step (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	plan_id TEXT NOT NULL,

	-- 1-based order under the plan. Unique per plan so a catalog cannot hold
	-- two steps at the same position.
	position INTEGER NOT NULL,

	external_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	technique_external_id TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_emulation_plan_step_natural_key
		UNIQUE (source_id, version, external_id),
	CONSTRAINT content_emulation_plan_step_position_unique
		UNIQUE (plan_id, position),
	CONSTRAINT content_emulation_plan_step_position_positive
		CHECK (position >= 1)
);

CREATE INDEX content_emulation_plan_step_plan_id_idx
	ON content.content_emulation_plan_step (plan_id, position);

-- ---------------------------------------------------------------------------
-- Freeform KB notes (custom / imported)
-- ---------------------------------------------------------------------------

CREATE TABLE content.content_note (
	id TEXT NOT NULL PRIMARY KEY,
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	external_id TEXT NOT NULL,

	title TEXT NOT NULL,
	body_markdown TEXT NOT NULL,
	tags JSON NOT NULL,
	technique_external_id TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	CONSTRAINT content_note_natural_key
		UNIQUE (source_id, version, external_id)
);

CREATE INDEX content_note_source_version_idx
	ON content.content_note (source_id, version);

-- ---------------------------------------------------------------------------
-- Builtin seeds
-- ---------------------------------------------------------------------------
--
-- Four upstream sources, disabled, with default HTTPS archive base URLs and
-- filled-in license/attribution. One enabled custom source — always present,
-- never "synced" from upstream. Stable UUIDs so docs, blctl and tests can name
-- them without a lookup-by-kind round trip.
--
-- Timestamps are the migration epoch, not "now": a seed is not an event that
-- happened at apply time, and two fresh databases should compare equal.

INSERT INTO content.content_source (
	id, kind, name, url, ref, enabled, status,
	last_synced_at, item_count, error,
	license_spdx, license_name, license_url, attribution,
	created_at, updated_at
) VALUES
(
	'01900000-0000-7000-8000-000000000001',
	'attack',
	'MITRE ATT&CK Enterprise',
	'https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master',
	'enterprise-attack/enterprise-attack-{version}.json',
	false,
	'idle',
	NULL,
	0,
	'',
	'Apache-2.0',
	'Apache License 2.0',
	'https://www.apache.org/licenses/LICENSE-2.0',
	'ATT&CK content is © MITRE Corporation and used under the Apache License 2.0. ATT&CK is a registered trademark of The MITRE Corporation.',
	TIMESTAMP '2026-01-01 00:00:00',
	TIMESTAMP '2026-01-01 00:00:00'
),
(
	'01900000-0000-7000-8000-000000000002',
	'atomic',
	'Atomic Red Team',
	'https://github.com/redcanaryco/atomic-red-team/archive/refs/heads/master.zip',
	'master',
	false,
	'idle',
	NULL,
	0,
	'',
	'MIT',
	'MIT License',
	'https://opensource.org/licenses/MIT',
	'Atomic Red Team is © Red Canary and contributors, used under the MIT License.',
	TIMESTAMP '2026-01-01 00:00:00',
	TIMESTAMP '2026-01-01 00:00:00'
),
(
	'01900000-0000-7000-8000-000000000003',
	'sigma',
	'SigmaHQ Core',
	'https://github.com/SigmaHQ/sigma/archive/refs/heads/master.zip',
	'master',
	false,
	'idle',
	NULL,
	0,
	'',
	'LGPL-2.1-or-later',
	'GNU Lesser General Public License v2.1 or later',
	'https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html',
	'Sigma rules are © SigmaHQ contributors. Detection rule content is licensed under LGPL-2.1-or-later unless a rule file says otherwise.',
	TIMESTAMP '2026-01-01 00:00:00',
	TIMESTAMP '2026-01-01 00:00:00'
),
(
	'01900000-0000-7000-8000-000000000004',
	'ctid',
	'CTID Adversary Emulation Library',
	'https://github.com/center-for-threat-informed-defense/adversary_emulation_library/archive/refs/heads/master.zip',
	'master',
	false,
	'idle',
	NULL,
	0,
	'',
	'Apache-2.0',
	'Apache License 2.0',
	'https://www.apache.org/licenses/LICENSE-2.0',
	'Adversary emulation library © Center for Threat-Informed Defense and contributors, used under the Apache License 2.0.',
	TIMESTAMP '2026-01-01 00:00:00',
	TIMESTAMP '2026-01-01 00:00:00'
),
(
	'01900000-0000-7000-8000-000000000005',
	'custom',
	'Custom content',
	'',
	'',
	true,
	'idle',
	NULL,
	0,
	'',
	'',
	'',
	'',
	'User-authored content for this installation.',
	TIMESTAMP '2026-01-01 00:00:00',
	TIMESTAMP '2026-01-01 00:00:00'
);
