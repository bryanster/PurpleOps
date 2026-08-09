-- 0019_report_templates — engagement-scoped report templates (M6-003).
--
-- Templates capture arrangement + default params, not frozen HTML.
-- Engagement-scoped: no install-wide gallery in v1.
--
-- # Foreign keys
--
-- DuckDB supports only ON DELETE RESTRICT. The application cascade
-- for deleting a template removes template_block rows first.
--
-- # Soft limits
--
-- Cap templates per engagement at 20 (enforced in service layer).

-- ---------------------------------------------------------------------------
-- Report template — header owned by an engagement
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_template (
    id TEXT NOT NULL PRIMARY KEY,

    engagement_id TEXT NOT NULL
        REFERENCES app.engagement (id) ON DELETE RESTRICT,

    name TEXT NOT NULL,

    created_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,

    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX report_template_engagement_idx ON app.report_template (engagement_id);

-- ---------------------------------------------------------------------------
-- Report template block — one ordered block in a template
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_template_block (
    template_id TEXT NOT NULL
        REFERENCES app.report_template (id) ON DELETE RESTRICT,

    ordinal INTEGER NOT NULL,

    block_id TEXT NOT NULL,

    -- Validated params JSON for this block instance.
    params JSON NOT NULL,

    CONSTRAINT report_template_block_unique_ordinal UNIQUE (template_id, ordinal)
);

CREATE INDEX report_template_block_ordinal_idx
    ON app.report_template_block (template_id, ordinal);
