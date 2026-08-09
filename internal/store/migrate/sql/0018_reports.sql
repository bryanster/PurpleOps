-- 0018_reports — report document model: drafts and ordered block instances (M6-002).
--
-- PLAN.md §8 and M6-EPIC: the report builder is a block registry over a durable
-- draft. One report per engagement (for now — the schema allows multiple, and
-- the application layer may later restrict to one without a migration).
--
-- # Draft only
--
-- report holds the mutable draft. Published immutable versions arrive in M6-011.
-- report_block holds ordered block instances with per-block params (JSON).
-- An empty draft (no blocks) is valid; the builder adds blocks.
--
-- # Foreign keys
--
-- DuckDB supports only ON DELETE RESTRICT (see 0002_identity.sql). The
-- application cascade for deleting a report removes report_block rows first.
--
-- # Branding overrides
--
-- client_name, logo_blob_ref and colours are NULLable overrides. NULL means
-- "fall back to install defaults" — the service layer applies the precedence,
-- not the database. Published versions snapshot the resolved values (M6-011).
--
-- # Params
--
-- params is a JSON object validated by the block registry's ValidateParams
-- (internal/report/params.go). No CHECK constraint on contents so the registry
-- validation vocabulary can grow without a migration.

-- ---------------------------------------------------------------------------
-- Report — the draft header
-- ---------------------------------------------------------------------------

CREATE TABLE app.report (
    id TEXT NOT NULL PRIMARY KEY,

    engagement_id TEXT NOT NULL
        REFERENCES app.engagement (id) ON DELETE RESTRICT,

    title TEXT NOT NULL DEFAULT '',

    -- Branding overrides. NULL = fall back to install defaults (M6-004).
    client_name TEXT,
    logo_blob_ref TEXT,

    -- JSON object: {"primary": "#rrggbb", "secondary": "#rrggbb"} or NULL.
    colours JSON,

    created_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,

    -- Optional — NULL until the first edit after create.
    updated_by TEXT,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX report_engagement_idx ON app.report (engagement_id);

-- ---------------------------------------------------------------------------
-- Report block — one ordered instance in a draft
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_block (
    id TEXT NOT NULL PRIMARY KEY,

    report_id TEXT NOT NULL
        REFERENCES app.report (id) ON DELETE RESTRICT,

    -- Dense, 0-based ordinal within one report.
    ordinal INTEGER NOT NULL,

    -- The block id from internal/report/blockids.go (e.g. "cover", "rich_text").
    block_id TEXT NOT NULL,

    -- Validated params JSON for this block instance.
    params JSON NOT NULL,

    CONSTRAINT report_block_unique_ordinal UNIQUE (report_id, ordinal)
);

CREATE INDEX report_block_report_ordinal_idx
    ON app.report_block (report_id, ordinal);
