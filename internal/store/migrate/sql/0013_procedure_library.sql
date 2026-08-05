-- 0013_procedure_library — procedure template list indexes (M2-008).
--
-- Supports library browse filters on rolling-head Atomic (and later custom)
-- procedure templates. Substring search on name / description / external_id
-- stays a filtered scan under the version predicate — M2 does not ship FTS.
-- Technique and platform membership filters cast the JSON columns to text and
-- match quoted tokens; that is portable ANSI and good enough for exact
-- external-id / platform labels.

CREATE INDEX content_procedure_template_external_id_idx
	ON content.content_procedure_template (source_id, version, external_id);
