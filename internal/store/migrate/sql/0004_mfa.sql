-- 0004_mfa — the second factor: what somebody enrolled, and the short gap
-- between proving a password and proving they still hold it.
--
-- Two tables, because they hold two different things for two different lengths
-- of time. app.user_totp is a long-lived enrolment; app.mfa_challenge is a row
-- that lives for five minutes and is then useless. Keeping the pending state in
-- the database rather than in a signed cookie is what makes "used once" and
-- "expired" facts this server can enforce rather than claims a client makes.
--
-- Neither table has a foreign key to app."user", for the reason 0003 sets out
-- at length: DuckDB implements UPDATE as a delete and an insert, and the delete
-- half runs the RESTRICT check, so a referenced user row cannot be edited at
-- all. The rule is enforced in Go instead — identity.requireUser, inside the
-- same write transaction, which the serialized writer (PLAN.md §1) makes as
-- strong as the constraint was.

-- The enrolment. One per person: a second authenticator is a second enrolment
-- of the same account, which is what recovery codes (M1-007) are for instead.
CREATE TABLE app.user_totp (
	user_id TEXT NOT NULL PRIMARY KEY,

	-- The shared secret, AES-256-GCM under a key derived from
	-- BLACKLIGHT_ENCRYPTION_KEY, base64url. A copy of this file is not a set of
	-- working authenticators. The nonce is per record and travels in front of
	-- the ciphertext; internal/authn/secrets is the only thing that reads this
	-- column, and it never returns the plaintext to a caller that is not
	-- checking a code.
	secret_encrypted TEXT NOT NULL,

	-- NULL until the person has proved they can produce a code from it. An
	-- unconfirmed secret gates nothing: a half-finished enrolment — the browser
	-- closed between the QR code and the first code — must not be able to lock
	-- somebody out of their own account.
	confirmed_at TIMESTAMP,

	-- The last TOTP time step this enrolment accepted, or 0 before the first.
	-- It is the replay window, and it is here rather than in memory because a
	-- restart must not re-open a code that has already been spent: a code is
	-- accepted only when its step is *after* this one, so presenting the same
	-- six digits twice fails the second time even inside their thirty seconds.
	--
	-- The ticket's column list does not include it. Replay protection is on the
	-- same ticket and libraries do not implement it, so it needs somewhere to
	-- live; this is the smallest thing that works and survives a restart.
	last_used_step BIGINT NOT NULL,

	created_at TIMESTAMP NOT NULL
);

-- The pending state between a correct password and a presented second factor.
-- It is deliberately not a session: nothing resolves one of these into a
-- caller, and the only endpoint that will look at it is the verification one.
CREATE TABLE app.mfa_challenge (
	id TEXT NOT NULL PRIMARY KEY,
	user_id TEXT NOT NULL,

	-- Only the hash of the token in the cookie, for the same reason
	-- app.session stores only a hash: a stolen database is not a set of live
	-- challenges. UNIQUE because it is the key the verification endpoint looks
	-- up, and two challenges sharing a hash is worth failing loudly on.
	token_hash TEXT NOT NULL UNIQUE,

	created_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,

	-- NULL until a code was accepted against it. A challenge is spent by being
	-- used, not only by expiring, so one correct code buys exactly one session.
	consumed_at TIMESTAMP
);

-- "Clear out whatever this person had pending" — which is what starting a new
-- sign-in does, so that a challenge left behind by an abandoned attempt cannot
-- still be answered.
CREATE INDEX mfa_challenge_user_id_idx ON app.mfa_challenge (user_id);
