-- 0021_report_shares — share links, grants/guests, password gate (M6-012).
--
-- Client delivery without making engagement members of every reader.
-- Login-required access (no anonymous HTML) plus optional password and
-- revocable grants. Revocation returns 404 so link existence is not
-- confirmed after revoke.
--
-- # Threat model
--
-- The share token is 256 random bits, hex-encoded in the URL, and only its
-- HMAC-SHA256 is stored — the same property app.session has. A copy of the
-- database is not a set of working share URLs.
--
-- # Password gate
--
-- password_hash is optional Argon2id (same parameters as local passwords).
-- When set, the claim/view flow requires the password before granting access.
-- A short-lived cookie (bl_report_share) carries an HMAC of the share token
-- and password satisfaction — it is derived, not stored, like bl_csrf.
--
-- # Grants
--
-- report_share_grant rows are created when a signed-in user claims a share.
-- A grant binds the share to a specific user_id; without an active grant,
-- the share view routes return 404.
--
-- # Foreign keys
--
-- DuckDB supports only ON DELETE RESTRICT. Deleting a report_version
-- requires deleting its shares first (application-level cascade).

-- ---------------------------------------------------------------------------
-- Report share — one shareable link to a published version
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_share (
    id TEXT NOT NULL PRIMARY KEY,

    version_id TEXT NOT NULL
        REFERENCES app.report_version (id) ON DELETE RESTRICT,

    -- HMAC-SHA256 of the share token under the deployment encryption key.
    -- The token itself is never stored; it is shown once at creation in the
    -- claim URL and the recipient presents it to claim/view.
    token_hash TEXT NOT NULL,

    -- Optional Argon2id password hash. NULL means no password gate.
    password_hash TEXT,

    -- Optional expiry. NULL means the share never expires on its own
    -- (revocation is still possible).
    expires_at TIMESTAMP,

    -- NULL until somebody ends this share. Revoked shares return 404 on
    -- all view/claim routes.
    revoked_at TIMESTAMP,

    -- Who created this share.
    created_by TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL,

    -- Optional human-readable label for the share listing.
    label TEXT,

    -- Maximum number of grants that may be claimed. NULL means unlimited.
    max_grants INTEGER
);

-- Look up shares for a version (for listing by the lead).
CREATE INDEX report_share_version_idx
    ON app.report_share (version_id);

-- Look up a share by its token hash (for claim/view).
CREATE INDEX report_share_token_hash_idx
    ON app.report_share (token_hash);

-- ---------------------------------------------------------------------------
-- Report share grant — one user's access to a shared version
-- ---------------------------------------------------------------------------

CREATE TABLE app.report_share_grant (
    id TEXT NOT NULL PRIMARY KEY,

    share_id TEXT NOT NULL
        REFERENCES app.report_share (id) ON DELETE RESTRICT,

    -- The user who claimed this grant. NULL until claimed via invite code.
    user_id TEXT,

    -- Optional invite code hash for pre-created unbound grants.
    invite_code_hash TEXT,

    -- When this grant was claimed (user_id was set).
    claimed_at TIMESTAMP,

    -- NULL until somebody revokes this grant.
    revoked_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL
);

-- Look up grants for a share (for listing/revoking).
CREATE INDEX report_share_grant_share_idx
    ON app.report_share_grant (share_id);

-- Look up a grant by share + user (for checking access).
CREATE INDEX report_share_grant_user_idx
    ON app.report_share_grant (share_id, user_id);
