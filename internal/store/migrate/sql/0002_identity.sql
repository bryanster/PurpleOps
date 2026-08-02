-- 0002_identity — the people, how they prove who they are, and what they may do.
--
-- PLAN.md §4 defines two levels of role, and v1's failures came partly from
-- having one fuzzy level instead. They are two columns on two tables here:
-- app."user".platform_role decides what somebody may do to the installation,
-- app.engagement_member.role decides what they may do inside one engagement.
-- Neither column can be mistaken for the other, and no query can read one and
-- get the other's answer.
--
-- "user" is a reserved word in the SQL standard and in PostgreSQL, so it is
-- quoted every time it appears. DuckDB accepts it bare; the quotes are for the
-- escape hatch in PLAN.md §1, not for this engine.
--
-- Two DuckDB behaviours shape the foreign keys, both verified against the
-- driver rather than assumed:
--
--   * ON DELETE CASCADE does not parse — "FOREIGN KEY constraints cannot use
--     CASCADE, SET NULL or SET DEFAULT". RESTRICT is the only referential
--     action available, and it is written out rather than left implicit so
--     that a reader does not have to know that.
--   * RESTRICT is enforced: deleting a user who still owns an identity, a
--     session or a membership fails.
--
-- That is the policy we want regardless. A person is retired by setting
-- status = 'disabled', which keeps their name on the executions, comments and
-- findings they wrote — deleting the row would either orphan an engagement's
-- history or quietly rewrite it. Hard deletion stays possible for the one case
-- that needs it, an account created by mistake before it did anything, and the
-- database makes the caller clear the dependent rows out first rather than
-- discovering later that it lost them.

CREATE TABLE app."user" (
	id TEXT NOT NULL PRIMARY KEY,

	-- email is kept as it was typed, because that is how its owner writes it
	-- and how it should appear in a report. email_normalized is what every
	-- lookup and the uniqueness rule use.
	--
	-- DuckDB has no citext and no case-insensitive collation, so a second
	-- column is how "Alice@x.com and alice@x.com are one account" becomes a
	-- rule the schema enforces rather than a convention every caller has to
	-- remember. The CHECK is the half that matters: without it, a caller could
	-- evade uniqueness by writing a normalized value that is not the lowercased
	-- email. With it, the pair cannot disagree.
	--
	-- A generated column would express this with less to get wrong, and DuckDB
	-- supports one — but only as VIRTUAL, where PostgreSQL supports only
	-- STORED. Two columns and a CHECK are the portable spelling.
	email TEXT NOT NULL,
	email_normalized TEXT NOT NULL UNIQUE,

	display_name TEXT NOT NULL,

	-- NULL is used only where "none" is a real state rather than an unknown
	-- one: an SSO-only account has no password, and a new account has never
	-- logged in. Everything else on this table is NOT NULL.
	password_hash TEXT,

	platform_role TEXT NOT NULL,
	status TEXT NOT NULL,

	-- Whether an administrator requires MFA of this person specifically.
	-- M1-008 enforces it; the column exists here so that enforcement has
	-- somewhere to read from and is not inferred from whether they enrolled —
	-- which is the v1 hole PLAN.md §4 names.
	mfa_enforced BOOLEAN NOT NULL,

	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	last_login_at TIMESTAMP,

	-- The role and status vocabularies are constrained here rather than only in
	-- Go, so that a bug cannot write a role that no policy knows how to judge.
	-- Adding a value later means a migration, which is the point: a new role is
	-- a decision, not a string somebody passed.
	CONSTRAINT user_email_normalized_agrees CHECK (email_normalized = lower(trim(email))),
	CONSTRAINT user_platform_role_known CHECK (platform_role IN ('admin', 'member')),
	CONSTRAINT user_status_known CHECK (status IN ('active', 'disabled', 'invited'))
);

-- No separate index on email: UNIQUE (email_normalized) creates one, and
-- email_normalized is the column every lookup uses. An index on the display
-- column would serve no query.

-- identity is a login method pointing at a person, so that somebody who signs
-- in with a password today and through Entra tomorrow is one account rather
-- than two. subject is whatever the provider calls them — an email for local,
-- the "sub" claim for OIDC, the NameID for SAML.
CREATE TABLE app.identity (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES app."user" (id) ON DELETE RESTRICT,
	provider TEXT NOT NULL,
	subject TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,

	CONSTRAINT identity_provider_known CHECK (provider IN ('local', 'oidc', 'saml')),
	-- One subject belongs to one person. Without this, a second account could
	-- claim an existing OIDC subject and receive its logins.
	CONSTRAINT identity_provider_subject_unique UNIQUE (provider, subject)
);

-- Listing a person's login methods, and the check M1-016 makes before
-- disabling their last one.
CREATE INDEX identity_user_id_idx ON app.identity (user_id);

CREATE TABLE app.session (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES app."user" (id) ON DELETE RESTRICT,

	-- The cookie carries the token; only its hash is stored, so a copy of this
	-- database is not a set of working sessions. UNIQUE because it is the key
	-- every authenticated request looks up, and because two sessions sharing a
	-- hash is a collision worth failing loudly on.
	token_hash TEXT NOT NULL UNIQUE,

	created_at TIMESTAMP NOT NULL,
	last_seen_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,

	-- NULL until somebody or something ends it early. A session that simply
	-- ran out is expired, not revoked, and the two are worth telling apart when
	-- reading an audit trail.
	revoked_at TIMESTAMP,

	-- Empty rather than NULL when the request did not carry one. "We did not
	-- record it" and "it was absent" are the same fact to whoever reads this
	-- later, and making them different would buy a null check at every use.
	ip TEXT NOT NULL,
	user_agent TEXT NOT NULL,

	-- Whether MFA has been satisfied for this session, as opposed to for this
	-- user at some point in the past. M1-006 sets it; M1-008 refuses the
	-- request without it.
	mfa_satisfied BOOLEAN NOT NULL
);

-- Listing and revoking a person's sessions — on password change, on role
-- change, and on the account page. Expiry leads because the common query wants
-- the live ones.
CREATE INDEX session_user_id_expires_at_idx ON app.session (user_id, expires_at);

-- engagement_id deliberately has no foreign key. app.engagement does not exist
-- until M3, and a placeholder table here would leave M3 either living with a
-- schema it did not design or migrating away from one on its first day. M3's
-- migration creates the table and adds the reference; until then this column
-- is an identifier the application supplies and nothing enforces.
CREATE TABLE app.engagement_member (
	engagement_id TEXT NOT NULL,
	user_id TEXT NOT NULL REFERENCES app."user" (id) ON DELETE RESTRICT,
	role TEXT NOT NULL,

	-- NULL when nobody added them: a seeded or imported membership. When a
	-- person did, the reference is real, because "who gave them access" is the
	-- first question an incident review asks.
	added_by TEXT REFERENCES app."user" (id) ON DELETE RESTRICT,

	added_at TIMESTAMP NOT NULL,

	-- One row per person per engagement. Someone cannot be red and blue at
	-- once, which is what makes blind mode (PLAN.md §4) decidable.
	PRIMARY KEY (engagement_id, user_id),

	CONSTRAINT engagement_member_role_known CHECK (role IN ('lead', 'red', 'blue', 'observer'))
);

-- "Which engagements is this person in", asked on every request that resolves
-- an engagement role. The primary key answers the other direction already.
CREATE INDEX engagement_member_user_id_idx ON app.engagement_member (user_id);
