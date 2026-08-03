-- 0007_saml_assertion — the assertions this deployment has already accepted,
-- so that none of them is accepted twice (M1-010).
--
-- SAML has no nonce. An assertion is a signed document naming who you are, and
-- it stays a valid signed document for its whole validity window — so anybody
-- who obtains a copy of one can present it again and be signed in as its
-- subject. A proxy log, a browser history on a shared machine, a referrer, a
-- crash report: the assertion travels through a browser, in a form field, and
-- there are more ways to end up holding one than there are ways to prevent it.
-- The only defence the profile offers is that a service provider must refuse an
-- assertion ID it has seen before, and no library does that for you, because no
-- library knows where you keep state.
--
-- This is that state, and it is a table rather than a map in the process on
-- purpose. A map is forgotten by a restart, and the window an assertion stays
-- replayable in — a few minutes — is comfortably longer than the time it takes
-- to restart this server. A replay cache that a deploy empties is a replay cache
-- with a scheduled hole in it.
--
-- Rows are swept by whatever writes the next one; see identity.SAMLAssertions.
-- The table is therefore self-bounding, with no background job and nothing to
-- forget to run.
CREATE TABLE app.saml_assertion (
	-- The assertion's own ID, exactly as the identity provider wrote it. This
	-- is the one identifier in the schema that Blacklight did not mint, which is
	-- why it is not called `id` and is not a UUIDv7: it is somebody else's
	-- value, and its only job is to be compared against itself.
	--
	-- PRIMARY KEY is the whole mechanism. The replay check is an INSERT that
	-- either succeeds or violates this constraint, decided by the database
	-- inside the write transaction — so two assertions arriving at once cannot
	-- both find the table empty and both be accepted, which is exactly what a
	-- SELECT-then-INSERT would allow.
	assertion_id TEXT NOT NULL PRIMARY KEY,

	-- When it was accepted here. Not used by the check; it is what makes the row
	-- answerable when somebody asks whether a particular assertion was ever
	-- consumed, and by then the question is being asked during an incident.
	consumed_at TIMESTAMP NOT NULL,

	-- The last moment this assertion could still be replayed: its own
	-- NotOnOrAfter, widened by the configured clock skew. After it, the row
	-- protects nothing and is swept.
	--
	-- Stored rather than derived because the skew is configuration and can
	-- change: a row written under one setting must not become sweepable early
	-- because somebody narrowed it afterwards.
	expires_at TIMESTAMP NOT NULL
);

-- The sweep is the only query that is not a primary-key lookup.
CREATE INDEX saml_assertion_expires_at_idx ON app.saml_assertion (expires_at);
