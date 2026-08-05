-- 0014_detection_library — detection rule list indexes (M2-009).
--
-- Supports library browse filters on rolling-head Sigma (and later custom)
-- detection rule refs. Substring search on name / description / external_id
-- stays a filtered scan under the version predicate — M2 does not ship FTS.
-- Technique membership filters cast the JSON column to text and match quoted
-- tokens; level is an exact case-insensitive column match.

CREATE INDEX content_detection_rule_ref_external_id_idx
	ON content.content_detection_rule_ref (source_id, version, external_id);

CREATE INDEX content_detection_rule_ref_level_idx
	ON content.content_detection_rule_ref (source_id, version, level);
