-- 0001_init — the two schemas every later migration builds on.
--
-- PLAN.md §2: reference data and engagement data share one database but not one
-- schema. "content" holds ATT&CK, Atomic Red Team, Sigma and CTID material,
-- which is replaceable — reinstalling ATT&CK v17 drops and rebuilds it. "app"
-- holds engagements, scoring and evidence, which is not replaceable by
-- anything. Keeping them apart is what makes the reinstall a safe operation
-- rather than one that has to be careful.
--
-- Nothing else belongs in this migration. Domain tables arrive with the
-- milestone that needs them, each in a migration of its own.

CREATE SCHEMA app;

CREATE SCHEMA content;
