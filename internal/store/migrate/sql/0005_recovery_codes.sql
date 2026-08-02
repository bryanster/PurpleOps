-- 0005_recovery_codes — the way back in when the phone is gone (M1-007).
--
-- A single-tenant tool that is self-hosted has no help desk. Without this
-- table, a lost authenticator means an administrator editing the database by
-- hand, and a lost authenticator belonging to the *only* administrator means
-- reinstalling. Ten codes, printed once, are what stand between those two
-- outcomes and an ordinary Tuesday.
--
-- No foreign key to app."user", for the reason 0003_user_updatable sets out and
-- 0004_mfa repeats: DuckDB implements UPDATE as a delete and an insert, and the
-- delete half runs the RESTRICT check, so a referenced user row could not be
-- edited at all. identity.requireUser enforces it inside the same write
-- transaction instead, which the serialized writer (PLAN.md §1) makes as strong
-- as the constraint was.
CREATE TABLE app.user_recovery_code (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL,

	-- HMAC-SHA256 of the normalized code under a key derived from
	-- BLACKLIGHT_ENCRYPTION_KEY, base64url. Only the hash, for the same reason
	-- app.session and app.mfa_challenge store only hashes: a copy of this file
	-- is not a set of working codes.
	--
	-- Hashed with an HMAC rather than with Argon2id, which is the choice
	-- M1-007 asks to be justified. A code carries ~100 bits from crypto/rand,
	-- so there is no dictionary for a slow KDF to slow down — and verification
	-- has to compare against every unused code a person holds, which under
	-- Argon2id would be ten sequential derivations on a pre-authentication
	-- endpoint. That is a denial-of-service lever pointed at the login path in
	-- exchange for hardening a secret that does not need it. The key is the
	-- encryption key rather than the session secret because rotating the
	-- session secret is the documented way to sign everybody out, and it must
	-- not also silently destroy every recovery code in the deployment.
	--
	-- UNIQUE because a collision between two codes is worth failing loudly on
	-- rather than resolving arbitrarily, exactly as mfa_challenge.token_hash is.
	code_hash TEXT NOT NULL UNIQUE,

	-- NULL until the code is spent. Kept rather than deleted so that "you have
	-- used seven of your ten codes" is answerable, and so that a code presented
	-- twice is refused by a row that exists rather than by one that is missing
	-- — the second is indistinguishable from a code that was never issued.
	used_at TIMESTAMP,

	created_at TIMESTAMP NOT NULL
);

-- Every read is "the codes belonging to this person": counting what is left,
-- checking a presented one, and deleting the set when it is replaced.
CREATE INDEX user_recovery_code_user_id_idx ON app.user_recovery_code (user_id);
