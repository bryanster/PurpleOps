-- 0008_service_token — the bearer credentials somebody automates with (M1-011).
--
-- PLAN.md §4 on v1: "API keys authenticate nothing." A credential table whose
-- rows are never checked is worse than no table at all, because the interface
-- above it implies a protection that does not exist. Everything here is shaped
-- by the checking rather than by the listing.
--
-- What is stored, and what is not: the token reaches its owner once, at
-- creation, and this table holds a hash of the secret half and the clear prefix
-- in front of it. A copy of this database is therefore not a set of working
-- credentials — the same property app.session has, and for the same reason.
--
-- No foreign keys, for the reason 0003_user_updatable gives at length: DuckDB
-- implements UPDATE as a delete and an insert, and the delete half runs the
-- RESTRICT check, so a child row makes its parent uneditable. owner_user_id and
-- created_by are held to real accounts inside the write transaction instead —
-- see identity.ServiceTokens, which uses the same requireUser as every other
-- repository in that package.

CREATE TABLE app.service_token (
	id TEXT NOT NULL PRIMARY KEY,

	-- What its owner called it: "nightly coverage export", "CI". It is for a
	-- human deciding which row to revoke during an incident, so it is required
	-- and nothing generates a default.
	name TEXT NOT NULL,

	-- The public half of the credential, stored in clear because it is what
	-- every authenticated request looks the row up by. UNIQUE is not decoration
	-- there: the lookup must land on one row or none, and a prefix collision
	-- would otherwise turn into a scan comparing hashes — which is exactly the
	-- full-table comparison the prefix exists to avoid.
	prefix TEXT NOT NULL UNIQUE,

	-- HMAC-SHA256 of the secret half under a key derived from the deployment's
	-- encryption key. Not UNIQUE: two rows sharing this would be a collision in
	-- a 256-bit random value, and failing an unrelated insert years later is a
	-- worse answer than the constraint is worth. The prefix already guarantees
	-- one row per lookup.
	token_hash TEXT NOT NULL,

	-- Whose authority the token spends. Every request it makes is decided
	-- against this account's *live* platform role and status, not against
	-- anything recorded here — PLAN.md §9's "a service token cannot exceed its
	-- grants or its owner's live permissions". Demoting or disabling the owner
	-- constrains the token on its next request, with no change to this row.
	owner_user_id TEXT NOT NULL,

	-- Who created it. The same person as the owner today, because the only
	-- endpoints that mint one act on the caller's own account (M1-011); the
	-- column is separate because "who holds this" and "who issued this" are
	-- different questions during an incident review, and an administrative
	-- issue-on-behalf-of would make them differ.
	created_by TEXT NOT NULL,

	-- The scopes this token carries, space-separated, in the spelling
	-- internal/authz owns.
	--
	-- One column rather than a child table, and a space-separated list rather
	-- than an array. The array would be DuckDB's TEXT[], which the escape hatch
	-- in PLAN.md §1 forbids outside internal/store/duckdb/. The child table
	-- would need a foreign key back to this one to be worth having, and that is
	-- what 0003 established cannot exist here — without it the "child" is an
	-- unenforced pair of columns, which is a join to buy nothing.
	--
	-- Space-separated is OAuth 2.0's own spelling of a scope list (RFC 6749
	-- §3.3) and no scope in the vocabulary contains a space. There is no CHECK
	-- constraining the words: unlike a role, an unrecognised scope is harmless
	-- by construction — the policy grants on scopes it holds, never on scopes it
	-- fails to recognise, so a scope from a newer build reaching an older one
	-- grants nothing rather than everything.
	scopes TEXT NOT NULL,

	-- The one engagement this token may touch, or NULL for a token that may
	-- touch every engagement its owner can. It is a fence and never a grant: a
	-- bound token reaches nothing outside that engagement, and inside it reaches
	-- only what its owner's membership already allows.
	--
	-- No foreign key even in principle: app.engagement does not exist until M3,
	-- which is the same reason app.engagement_member's column has none.
	engagement_id TEXT,

	created_at TIMESTAMP NOT NULL,

	-- Required, and the row is refused without one. A credential with no expiry
	-- is a credential nobody ever revokes, because nothing ever reminds them to;
	-- the maximum is enforced in Go, where the policy can be argued with, and
	-- the NOT NULL here is what makes "forever" unrepresentable.
	expires_at TIMESTAMP NOT NULL,

	-- NULL until the token is first used. It is written back at most once per
	-- interval rather than on every request — see servicetoken.Manager — so it
	-- is accurate to within that interval and no more, which is all "is this
	-- token still in use?" needs.
	last_used_at TIMESTAMP,

	-- NULL until somebody ends it early. Expired and revoked are different
	-- facts: one is a token that ran its course and the other is a decision
	-- somebody made, and an incident review wants to tell them apart.
	revoked_at TIMESTAMP
);

-- "Which tokens does this person hold", which is the listing endpoint and the
-- query an administrator runs when an account is compromised. Newest first is
-- the id's job (UUIDv7), so this index carries only the key it filters on.
CREATE INDEX service_token_owner_user_id_idx ON app.service_token (owner_user_id);
