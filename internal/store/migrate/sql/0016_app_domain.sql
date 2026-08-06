-- 0016_app_domain — the engagement workbook graph (M3-001).
--
-- PLAN.md §2 defines the product core: Engagement → Scenario → Step →
-- Execution, scored in ATT&CK Evaluations terms. Red and blue fill a shared
-- workbook; findings track remediation; evidence, comments and edit history
-- complete the graph. Everything lives in the app schema created by 0001_init.
--
-- # No rounds
--
-- Per M3-EPIC, retest rounds are out of v1. Execution is 1:1 with step
-- (UNIQUE(step_id)). Any operator who needs a post-remediation pass must create
-- a new engagement. There is no round table, no round_id column, no (step_id,
-- round_id) grain.
--
-- # Foreign keys
--
-- DuckDB supports only ON DELETE RESTRICT (see 0002_identity.sql), so every FK
-- here is RESTRICT. Application-enforced cascades for engagement delete belong
-- in M3-002; this migration only establishes referential integrity.
--
-- FK from app.engagement_member.engagement_id → app.engagement(id) is added
-- now that the parent exists (M1-001 left it out on purpose). DuckDB does not
-- support ALTER TABLE ADD CONSTRAINT for foreign keys, so the table is rebuilt
-- in the same pattern as 0003_user_updatable.
--
-- # No FK from app to content
--
-- Lineage columns (template_id, plan_id) are TEXT with no foreign key, per
-- copy-on-use rules (docs/content-copy-on-use.md). Content rows are
-- replaceable; app rows must not be dragged along.
--
-- # Evidence: exactly one parent
--
-- An evidence row links to either an execution or a comment — never both,
-- never neither. A CHECK constraint enforces this rather than relying on
-- application code alone.
--
-- # Detection modifiers as JSON
--
-- detection_modifiers is a JSON array of enum strings. DuckDB's JSON column
-- accepts any well-formed JSON; domain validation (M3-008) enforces the closed
-- vocabulary. The schema does not CHECK the array contents so that the
-- vocabulary can grow without a migration.
--
-- # Optimistic locking
--
-- execution.version (INT NOT NULL DEFAULT 1) is the optimistic-lock column.
-- Every red/blue PATCH must supply the caller's version; mismatch → 409.
-- Version is incremented atomically in the UPDATE, not in application code.
--
-- # Table ordering
--
-- Tables are ordered so that every FK reference resolves at CREATE TABLE time.
-- comment is created before evidence so the evidence.comment_id FK can be
-- declared in-line (DuckDB's ALTER TABLE ADD FOREIGN KEY support is limited).

-- ---------------------------------------------------------------------------
-- Engagement — the assessment header
-- ---------------------------------------------------------------------------

CREATE TABLE app.engagement (
    id TEXT NOT NULL PRIMARY KEY,

    name TEXT NOT NULL,
    client TEXT NOT NULL,
    description TEXT NOT NULL,

    -- Lifecycle: draft → active → closed → archived.
    status TEXT NOT NULL,

    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,

    -- ATT&CK version pin (e.g. "15.1"). Required on create, or set before the
    -- first step that resolves ATT&CK lineage. attackpin.References counts
    -- engagements that pin a version, so DeleteVersion can refuse when live
    -- engagements still reference it.
    attack_version TEXT NOT NULL,

    -- Standard: both sides see the workbook. Blind: red/lead decide what blue
    -- sees via per-step reveal (PLAN.md §4).
    mode TEXT NOT NULL,

    -- When true, the first red transition to running (or complete if skipping
    -- running) reveals the step automatically. Does not bulk-reveal on close.
    auto_reveal_on_start BOOLEAN NOT NULL,

    created_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT engagement_status_known
        CHECK (status IN ('draft', 'active', 'closed', 'archived')),
    CONSTRAINT engagement_mode_known
        CHECK (mode IN ('standard', 'blind'))
);

CREATE INDEX engagement_status_idx ON app.engagement (status);

-- ---------------------------------------------------------------------------
-- Scenario — ordered attack-chain section inside an engagement
-- ---------------------------------------------------------------------------

CREATE TABLE app.scenario (
    id TEXT NOT NULL PRIMARY KEY,
    engagement_id TEXT NOT NULL REFERENCES app.engagement (id) ON DELETE RESTRICT,

    -- 1-based order under the engagement. Unique per engagement so two
    -- scenarios cannot occupy the same position.
    ordinal INTEGER NOT NULL,

    name TEXT NOT NULL,
    narrative TEXT NOT NULL,

    -- Provenance: manual (built in UI), ctid (imported from emulation plan),
    -- imported (v1 or external format).
    source TEXT NOT NULL,

    threat_actor TEXT NOT NULL,
    source_ref TEXT NOT NULL,

    -- Weak lineage to content.emulation_plan. No FK: content is replaceable;
    -- app rows must survive a content re-sync.
    plan_id TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT scenario_ordinal_unique UNIQUE (engagement_id, ordinal),
    CONSTRAINT scenario_source_known
        CHECK (source IN ('manual', 'ctid', 'imported'))
);

CREATE INDEX scenario_engagement_ordinal_idx
    ON app.scenario (engagement_id, ordinal);

-- ---------------------------------------------------------------------------
-- Step — one technique/procedure row under a scenario
-- ---------------------------------------------------------------------------

CREATE TABLE app.step (
    id TEXT NOT NULL PRIMARY KEY,
    scenario_id TEXT NOT NULL REFERENCES app.scenario (id) ON DELETE RESTRICT,

    -- 1-based order under the scenario.
    ordinal INTEGER NOT NULL,

    name TEXT NOT NULL,
    objective TEXT NOT NULL,

    -- ATT&CK lineage as external-id text (e.g. "T1059", "T1059.001", "TA0002").
    -- Nullable when unknown at create time.
    technique_id TEXT,
    subtechnique_id TEXT,
    tactic_id TEXT,

    -- Structured procedure payload: platform, executor, command, cleanup,
    -- args, … M3-013 snapshots this from content.procedure_template.
    "procedure" JSON NOT NULL,

    -- Weak lineage. No FK: see header.
    template_id TEXT NOT NULL,

    target_asset TEXT NOT NULL,
    tools JSON NOT NULL,
    controls_in_scope JSON NOT NULL,

    -- Snapshot of engagement.attack_version at step-create time, so a
    -- step's ATT&CK resolution is stable even if the engagement pin changes.
    attack_version TEXT NOT NULL,

    -- NULL = hidden from blue when engagement is blind. Lead or red may set
    -- this to now(); the query-layer filter (internal/store/blind) reads it.
    revealed_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT step_ordinal_unique UNIQUE (scenario_id, ordinal)
);

CREATE INDEX step_scenario_ordinal_idx ON app.step (scenario_id, ordinal);

-- ---------------------------------------------------------------------------
-- Execution — red + blue fill-in for one step (1:1)
-- ---------------------------------------------------------------------------

CREATE TABLE app.execution (
    id TEXT NOT NULL PRIMARY KEY,
    step_id TEXT NOT NULL UNIQUE REFERENCES app.step (id) ON DELETE RESTRICT,

    -- Optimistic lock. Starts at 1; every red/blue PATCH increments.
    version INTEGER NOT NULL DEFAULT 1,

    -- Red side ----------------------------------------------------------------
    status TEXT NOT NULL,

    executed_by TEXT NOT NULL,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    command_run TEXT NOT NULL,
    source_host TEXT NOT NULL,
    target_host TEXT NOT NULL,
    red_notes TEXT NOT NULL,

    -- Blue side ---------------------------------------------------------------
    -- NULL until scored.
    detection_category TEXT,

    -- JSON array of enum strings (alert, correlated, delayed, config_change,
    -- residual_artifact). Empty array allowed. Domain validates vocabulary.
    detection_modifiers JSON NOT NULL,

    -- NULL until scored.
    protection TEXT,

    detected_at TIMESTAMP,
    detecting_source TEXT NOT NULL,
    detecting_rule_ref TEXT NOT NULL,
    alert_severity TEXT NOT NULL,
    blue_notes TEXT NOT NULL,

    scored_by TEXT NOT NULL,
    scored_at TIMESTAMP,

    -- Derived outcome is never stored (M3-008). ------------------------------------------------

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT execution_status_known
        CHECK (status IN ('pending', 'running', 'complete', 'blocked', 'skipped')),
    CONSTRAINT execution_detection_category_known
        CHECK (detection_category IS NULL OR detection_category IN (
            'none', 'telemetry', 'general', 'tactic', 'technique'
        )),
    CONSTRAINT execution_protection_known
        CHECK (protection IS NULL OR protection IN (
            'blocked', 'partial', 'not_blocked', 'n/a'
        ))
);

-- UNIQUE(step_id) above creates the index; no separate execution_step_id_idx
-- needed.

-- ---------------------------------------------------------------------------
-- Comment — on an execution (before evidence, so evidence FK resolves)
-- ---------------------------------------------------------------------------

CREATE TABLE app."comment" (
    id TEXT NOT NULL PRIMARY KEY,
    execution_id TEXT NOT NULL REFERENCES app.execution (id) ON DELETE RESTRICT,

    author_id TEXT NOT NULL,

    -- Current body. Edited via append to comment_revision; edited_at tracks
    -- the last edit (NULL when never edited).
    body TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL,
    edited_at TIMESTAMP
);

CREATE INDEX comment_execution_id_idx ON app."comment" (execution_id);

-- ---------------------------------------------------------------------------
-- Comment revision — append-only edit history
-- ---------------------------------------------------------------------------

CREATE TABLE app.comment_revision (
    id TEXT NOT NULL PRIMARY KEY,
    comment_id TEXT NOT NULL REFERENCES app."comment" (id) ON DELETE RESTRICT,

    -- Body at the time of this edit.
    body TEXT NOT NULL,

    edited_by TEXT NOT NULL,
    edited_at TIMESTAMP NOT NULL
);

CREATE INDEX comment_revision_comment_id_idx
    ON app.comment_revision (comment_id);

-- ---------------------------------------------------------------------------
-- Finding — remediation item on an engagement
-- ---------------------------------------------------------------------------

CREATE TABLE app.finding (
    id TEXT NOT NULL PRIMARY KEY,
    engagement_id TEXT NOT NULL REFERENCES app.engagement (id) ON DELETE RESTRICT,

    title TEXT NOT NULL,
    description TEXT NOT NULL,
    severity TEXT NOT NULL,
    recommendation TEXT NOT NULL,

    -- User who owns this finding. TEXT, no hard FK: consistent with the
    -- identity style where user_id is text without a constraint.
    "owner" TEXT NOT NULL,

    status TEXT NOT NULL,

    -- The execution this finding was created from, if any. Nullable: findings
    -- can be freeform items not tied to a specific step.
    created_from_execution TEXT,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT finding_status_known
        CHECK (status IN ('open', 'in_progress', 'resolved', 'accepted_risk'))
);

CREATE INDEX finding_engagement_status_idx
    ON app.finding (engagement_id, status);

-- ---------------------------------------------------------------------------
-- Finding ↔ Step join
-- ---------------------------------------------------------------------------

CREATE TABLE app.finding_step (
    finding_id TEXT NOT NULL REFERENCES app.finding (id) ON DELETE RESTRICT,
    step_id     TEXT NOT NULL REFERENCES app.step (id)     ON DELETE RESTRICT,

    PRIMARY KEY (finding_id, step_id)
);

-- ---------------------------------------------------------------------------
-- Evidence blob index — content-addressed, shared across references
-- ---------------------------------------------------------------------------

CREATE TABLE app.evidence_blob (
    sha256 TEXT NOT NULL PRIMARY KEY,

    size BIGINT NOT NULL,
    mime TEXT NOT NULL,

    -- Path relative to the configured evidence data root. Never absolute,
    -- never "..". Set by the blob upload handler (M3-009).
    storage_path TEXT NOT NULL,

    -- How many evidence rows point at this blob. GC drops the file when this
    -- reaches zero.
    ref_count INTEGER NOT NULL,

    created_at TIMESTAMP NOT NULL,

    CONSTRAINT evidence_blob_size_nonneg CHECK (size >= 0),
    CONSTRAINT evidence_blob_ref_count_nonneg CHECK (ref_count >= 0)
);

-- ---------------------------------------------------------------------------
-- Evidence — metadata row linking a blob to an execution or comment
-- ---------------------------------------------------------------------------

CREATE TABLE app.evidence (
    id TEXT NOT NULL PRIMARY KEY,
    blob_sha256 TEXT NOT NULL REFERENCES app.evidence_blob (sha256) ON DELETE RESTRICT,

    filename TEXT NOT NULL,
    caption TEXT NOT NULL,

    -- Which side uploaded it.
    side TEXT NOT NULL,

    -- Exactly one parent: either an execution or a comment, never both and
    -- never neither. The CHECK enforces this in the schema so no application
    -- bug can produce an orphan evidence row.
    execution_id TEXT REFERENCES app.execution (id) ON DELETE RESTRICT,
    comment_id   TEXT REFERENCES app."comment" (id) ON DELETE RESTRICT,

    uploaded_by TEXT NOT NULL,
    uploaded_at TIMESTAMP NOT NULL,

    -- Denormalised from blob for queries that filter without joining.
    size BIGINT NOT NULL,
    mime TEXT NOT NULL,

    CONSTRAINT evidence_parent_xor
        CHECK (
            (execution_id IS NOT NULL AND comment_id IS     NULL) OR
            (execution_id IS     NULL AND comment_id IS NOT NULL)
        ),
    CONSTRAINT evidence_side_known
        CHECK (side IN ('red', 'blue')),
    CONSTRAINT evidence_size_nonneg CHECK (size >= 0)
);

CREATE INDEX evidence_execution_id_idx ON app.evidence (execution_id);
CREATE INDEX evidence_comment_id_idx ON app.evidence (comment_id);
CREATE INDEX evidence_blob_sha256_idx ON app.evidence (blob_sha256);

-- ---------------------------------------------------------------------------
-- Add FK from engagement_member.engagement_id → engagement(id)
-- ---------------------------------------------------------------------------
--
-- M1-001 left this FK out because the parent table did not yet exist.
-- DuckDB does not support ALTER TABLE ADD CONSTRAINT for foreign keys, so the
-- table is rebuilt — the same pattern 0003_user_updatable used to drop FKs.
--
-- All existing columns, the PK and the CHECK are preserved. The only change is
-- the FK on engagement_id.
--
-- If any existing engagement_member row references a non-existent engagement,
-- the INSERT below fails — which is correct: the schema should not accept
-- orphaned memberships, and this surfaces the data problem at migration time
-- rather than letting it reach the application.

CREATE TABLE app.engagement_member_next (
    engagement_id TEXT NOT NULL REFERENCES app.engagement (id) ON DELETE RESTRICT,
    user_id       TEXT NOT NULL,
    role          TEXT NOT NULL,
    added_by      TEXT,
    added_at      TIMESTAMP NOT NULL,

    PRIMARY KEY (engagement_id, user_id),
    CONSTRAINT engagement_member_role_known
        CHECK (role IN ('lead', 'red', 'blue', 'observer'))
);

INSERT INTO app.engagement_member_next
    SELECT engagement_id, user_id, role, added_by, added_at
    FROM app.engagement_member;

DROP TABLE app.engagement_member;
ALTER TABLE app.engagement_member_next RENAME TO engagement_member;

CREATE INDEX engagement_member_user_id_idx
    ON app.engagement_member (user_id);
