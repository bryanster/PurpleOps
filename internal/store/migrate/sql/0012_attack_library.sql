-- 0012_attack_library — ATT&CK library join + list indexes (M2-006).
--
-- Technique↔mitigation is stored as a natural-key join table (same shape as
-- content_technique_tactic) so library detail can answer "mitigations for
-- T1059.001 in v15.1" without re-parsing STIX, and bulk version replace can
-- delete by (source_id, version) without resolving UUIDs.
--
-- List indexes support the library filters: version scope, is_subtechnique,
-- and equality on external_id within a version. Substring search on name /
-- description stays a filtered scan under the version predicate — M2 does not
-- ship an FTS engine.

CREATE TABLE content.content_technique_mitigation (
	source_id TEXT NOT NULL,
	version TEXT NOT NULL,
	technique_external_id TEXT NOT NULL,
	mitigation_external_id TEXT NOT NULL,

	PRIMARY KEY (source_id, version, technique_external_id, mitigation_external_id)
);

CREATE INDEX content_technique_mitigation_mitigation_idx
	ON content.content_technique_mitigation (source_id, version, mitigation_external_id);

CREATE INDEX content_technique_is_subtechnique_idx
	ON content.content_technique (source_id, version, is_subtechnique);

CREATE INDEX content_technique_external_id_idx
	ON content.content_technique (source_id, version, external_id);

CREATE INDEX content_tactic_external_id_idx
	ON content.content_tactic (source_id, version, external_id);

CREATE INDEX content_mitigation_external_id_idx
	ON content.content_mitigation (source_id, version, external_id);

CREATE INDEX content_group_external_id_idx
	ON content.content_group (source_id, version, external_id);

CREATE INDEX content_software_external_id_idx
	ON content.content_software (source_id, version, external_id);
