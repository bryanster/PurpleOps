-- 0017_finding_status_history — append-only record of finding status changes (M5-003).
--
-- PLAN.md §2 and M5-EPIC: a burndown needs history, and there is none.
-- app.finding stores only the current status and updated_at, so "how many
-- findings were open on 12 June" is unanswerable from the workbook tables.
-- The activity log carries status deltas in finding.updated events, but
-- 0009_activity.sql commits to a blctl retention/prune command, and a burndown
-- that silently changes shape after retention is precisely the analytics error
-- the M5 epic's risk section exists to prevent.
--
-- This table is the source of truth for findings burndown (M5-007) and every
-- M6 report block that prints a burndown chart. It has a different retention
-- contract from app.activity, and the M5 epic explains why.
--
-- APPEND-ONLY. There is no update path and no delete path except the cascade
-- that follows a finding deletion — same discipline as app.activity. A history
-- row pointing at nothing is not evidence.
--
-- # No FK on finding_id or engagement_id
--
-- finding_id deliberately has no foreign key: DuckDB implements UPDATE as
-- DELETE+INSERT, and a RESTRICT child on finding_status_history would block
-- every finding patch. This is the same reason activity.actor_id has no FK
-- (0009_activity.sql). Application-enforced: DeleteFinding removes history
-- rows before the finding row, and Create/Update write both in one
-- transaction so the finding_id is always valid.
--
-- engagement_id also has no FK — it is denormalised for the burndown access
-- path and maintained by the repository in the same write transaction.
--
-- # Pre-M5 transitions
--
-- Findings created before this migration have no transition history. The
-- backfill below inserts one (NULL → current_status) row per existing finding
-- at the finding's created_at with changed_by = 'migration'. Pre-M5 transitions
-- are unrecoverable; the backfill row is the honest approximation, not a
-- reconstruction.

CREATE TABLE app.finding_status_history (
    id TEXT NOT NULL PRIMARY KEY,

    -- No FK: see header.
    finding_id TEXT NOT NULL,

    -- Denormalized so a burndown scans one engagement without joining finding.
    -- No FK: see header.
    engagement_id TEXT NOT NULL,

    -- NULL means creation — the finding did not previously exist in any status.
    from_status TEXT,

    to_status TEXT NOT NULL,

    -- User id, identity-style. No hard FK: see 0009_activity.sql for the
    -- reasoning (DuckDB's delete+insert UPDATE behavior and RESTRICT children).
    changed_by TEXT NOT NULL,

    -- UTC. "at" is reserved in DuckDB (M1-001 note).
    changed_at TIMESTAMP NOT NULL,

    CONSTRAINT fsh_to_status_known
        CHECK (to_status IN ('open', 'in_progress', 'resolved', 'accepted_risk'))
);

-- Burndown access path: one engagement, time-ordered.
CREATE INDEX fsh_engagement_changed_at_idx
    ON app.finding_status_history (engagement_id, changed_at);

-- ---------------------------------------------------------------------------
-- Backfill: one (NULL → current_status) row per existing finding
-- ---------------------------------------------------------------------------

INSERT INTO app.finding_status_history
    (id, finding_id, engagement_id, from_status, to_status, changed_by, changed_at)
SELECT
    -- UUIDv7 generated at migration time via uuid() — stable within a run and
    -- deterministic for tests that seed findings before applying this migration.
    uuid(),
    f.id,
    f.engagement_id,
    NULL,
    f.status,
    'migration',
    f.created_at
FROM app.finding f;
