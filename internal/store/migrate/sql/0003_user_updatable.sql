-- 0003_user_updatable — remove the foreign keys pointing at app."user", so that
-- a user row can be updated at all.
--
-- 0002_identity gave app.identity, app.session and app.engagement_member a
-- REFERENCES app."user" (id) ON DELETE RESTRICT, on the reasoning that hard
-- deletion of somebody who still owns rows should be refused by the database.
-- That reasoning stands. The constraint cannot stay, because of what DuckDB
-- does with it.
--
-- DuckDB implements UPDATE on a table carrying an index as a delete followed by
-- an insert, and the delete half runs the RESTRICT check. The result is that
-- *any* update to a user row that is referenced by *any* child row fails:
--
--   Constraint Error: Violates foreign key constraint because key
--   "user_id: 019f…" is still referenced by a foreign key in a different table.
--
-- Not only updates to id — every update, including
-- `SET last_login_at = ?`. Verified against DuckDB v1.5.5, the version this
-- binary links. So recording a login, upgrading a password hash to today's cost
-- and changing a password were all impossible for every account that had ever
-- signed in or been given an identity row, which is all of them (M1-003).
--
-- What is lost is a hand-typed `DELETE FROM app."user"` being refused. That is
-- worth less than it sounds: the application has no path that deletes a user at
-- all — internal/store/identity/users.go has no Delete, on purpose, because an
-- account is retired with status = 'disabled' and its author rows keep their
-- author. The rule the foreign key was enforcing is now enforced by there being
-- no code that breaks it, and by this comment for whoever is holding a SQL
-- console.
--
-- Re-add the constraints if DuckDB gains an UPDATE that does not delete, or if
-- the escape hatch in PLAN.md §1 is ever taken to PostgreSQL, where none of
-- this applies.
--
-- DuckDB has no ALTER TABLE ... DROP CONSTRAINT ("No support for that ALTER
-- TABLE option yet!"), so each table is recreated, copied into, dropped and
-- renamed. Every other constraint, index and column is carried over unchanged —
-- compare against 0002_identity.sql, which stays as it was: migrations are
-- append-only.

-- identity ---------------------------------------------------------------

CREATE TABLE app.identity_next (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	subject TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,

	CONSTRAINT identity_provider_known CHECK (provider IN ('local', 'oidc', 'saml')),
	CONSTRAINT identity_provider_subject_unique UNIQUE (provider, subject)
);

INSERT INTO app.identity_next (id, user_id, provider, subject, created_at)
SELECT id, user_id, provider, subject, created_at FROM app.identity;

DROP TABLE app.identity;

ALTER TABLE app.identity_next RENAME TO identity;

CREATE INDEX identity_user_id_idx ON app.identity (user_id);

-- session ----------------------------------------------------------------

CREATE TABLE app.session_next (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	created_at TIMESTAMP NOT NULL,
	last_seen_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	revoked_at TIMESTAMP,
	ip TEXT NOT NULL,
	user_agent TEXT NOT NULL,
	mfa_satisfied BOOLEAN NOT NULL
);

INSERT INTO app.session_next
	(id, user_id, token_hash, created_at, last_seen_at, expires_at,
	 revoked_at, ip, user_agent, mfa_satisfied)
SELECT id, user_id, token_hash, created_at, last_seen_at, expires_at,
       revoked_at, ip, user_agent, mfa_satisfied FROM app.session;

DROP TABLE app.session;

ALTER TABLE app.session_next RENAME TO session;

CREATE INDEX session_user_id_expires_at_idx ON app.session (user_id, expires_at);

-- engagement_member ------------------------------------------------------

CREATE TABLE app.engagement_member_next (
	engagement_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL,

	-- added_by referenced app."user" too, and for the same reason it cannot:
	-- adding somebody to an engagement would otherwise be an update away from
	-- locking the person who did it.
	added_by TEXT,

	added_at TIMESTAMP NOT NULL,

	PRIMARY KEY (engagement_id, user_id),

	CONSTRAINT engagement_member_role_known CHECK (role IN ('lead', 'red', 'blue', 'observer'))
);

INSERT INTO app.engagement_member_next
	(engagement_id, user_id, role, added_by, added_at)
SELECT engagement_id, user_id, role, added_by, added_at FROM app.engagement_member;

DROP TABLE app.engagement_member;

ALTER TABLE app.engagement_member_next RENAME TO engagement_member;

CREATE INDEX engagement_member_user_id_idx ON app.engagement_member (user_id);
