-- 0020_report_versions — published immutable report versions (M6-011).
--
-- Publishing freezes a snapshot: blocks, resolved branding, rendered HTML,
-- and flags in effect at publish time. Once inserted, content columns are
-- never updated — immutability is enforced at the store layer.
--
-- # Immutable versions
--
-- report_version rows are append-only. No UPDATE of content columns
-- (blocks_json, branding_json, html, content_sha256) is permitted after
-- insert. pdf_sha256 is set once on first PDF generation.
--
-- # Ordinals
--
-- Each publish creates version N+1 for the parent report. Ordinals are
-- dense, 1-based, and unique per report.
--
-- # Evidence opt-in
--
-- include_evidence records the caller's choice at publish time. When false,
-- the rendered HTML omits evidence bytes. This flag is frozen too.
--
-- # Blind scope
--
-- Published versions always use the lead (full) scope. blind_scope is
-- recorded as 'lead_full' for honesty/auditability.
--
-- # Foreign keys
--
-- DuckDB supports only ON DELETE RESTRICT. Deleting a report requires
-- deleting its versions first (application-level cascade).

-- ---------------------------------------------------------------------------
-- Report version — one immutable published snapshot
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_version (
    id TEXT NOT NULL PRIMARY KEY,

    report_id TEXT NOT NULL
        REFERENCES app.report (id) ON DELETE RESTRICT,

    -- 1-based dense ordinal within one report.
    ordinal INTEGER NOT NULL,

    -- Title at publish time (copied from report.title for snapshot honesty).
    title TEXT NOT NULL,

    -- Who published this version.
    published_by TEXT NOT NULL,

    -- When this version was published.
    published_at TIMESTAMP NOT NULL,

    -- Evidence opt-in flag frozen at publish time.
    include_evidence BOOLEAN NOT NULL DEFAULT FALSE,

    -- Always 'lead_full' — published versions never use blue scope.
    blind_scope TEXT NOT NULL DEFAULT 'lead_full',

    -- Frozen copy of block_id + params for each block in ordinal order.
    blocks_json JSON NOT NULL,

    -- Resolved branding snapshot at publish time.
    branding_json JSON NOT NULL,

    -- Full rendered HTML document (self-contained).
    html TEXT NOT NULL,

    -- SHA-256 of the rendered HTML. Nullable until computed.
    content_sha256 TEXT,

    -- SHA-256 of the generated PDF. Nullable until first PDF generation.
    pdf_sha256 TEXT
);

-- Look up all versions of a report, newest first.
CREATE INDEX report_version_report_idx
    ON app.report_version (report_id, ordinal DESC);

-- Enforce dense unique ordinals per report.
CREATE UNIQUE INDEX report_version_ordinal_unique
    ON app.report_version (report_id, ordinal);
