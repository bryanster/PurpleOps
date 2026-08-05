-- 0015_emulation_library — CTID plan catalog columns + list indexes (M2-010).
--
-- M2-001 left content_emulation_plan / content_emulation_plan_step with the
-- shared object columns only. The CTID adapter owns the fields that carry
-- adversary identity, source metadata, and the structured procedure payload
-- M3-013 will snapshot onto scenario steps (ticket M2-010; PLAN.md §3).
--
-- DuckDB rejects ADD COLUMN with NOT NULL / DEFAULT constraints ("Adding
-- columns with constraints not yet supported"), so these land nullable. The
-- repository and adapter always write non-null values (empty string / `{}`)
-- and coerce NULLs on read the same way service_token.revoked_by is handled.
--
-- procedure is a JSON object: platforms, executors/commands, input_arguments,
-- tactic, procedure_group/step labels, cti_source, dependencies. Empty object
-- when upstream provides none. technique_external_id stays a single TEXT column
-- (CTID steps carry at most one attack_id); missing technique is empty string.

ALTER TABLE content.content_emulation_plan
	ADD COLUMN adversary_name TEXT;

ALTER TABLE content.content_emulation_plan
	ADD COLUMN metadata JSON;

ALTER TABLE content.content_emulation_plan_step
	ADD COLUMN procedure JSON;

CREATE INDEX content_emulation_plan_external_id_idx
	ON content.content_emulation_plan (source_id, version, external_id);

CREATE INDEX content_emulation_plan_step_external_id_idx
	ON content.content_emulation_plan_step (source_id, version, external_id);
