-- 0009_activity — the append-only record of what happened (M1-015).
--
-- PLAN.md §2: this table "drives the SSE feed AND the report timeline". It is
-- not a bolt-on audit log; M4's live collaboration and M6's engagement narrative
-- both read from it. The shape here is therefore fixed for those readers: a
-- verb, an object, an optional engagement, a redacted delta, and a time.
--
-- APPEND-ONLY. There is no update path and no delete path in the application.
-- Retention and pruning are a blctl command, not an API endpoint, and neither
-- is this migration's job. A future migration that added either would be a
-- deliberate decision about the audit trail, not a convenience.
--
-- engagement_id is nullable: platform events (login, token create, role change)
-- have none. It deliberately has no foreign key, for the same reason
-- 0002_identity.sql left engagement_member.engagement_id without one — app.engagement
-- does not exist until M3. M3 adds the reference; until then the column is an
-- identifier the application supplies.
--
-- actor_id is nullable for the same reason session.user_id is not: a failed
-- login may not resolve to an account. When it does, the value is a real user
-- id; when it does not, NULL is the fact "we do not know who", and the attempted
-- email lives in delta. No foreign key, for the reason 0003_user_updatable
-- gives: DuckDB implements UPDATE as delete+insert and RESTRICT children block
-- the delete half.
--
-- delta is JSON. Secrets never belong in it — password hashes, token secrets,
-- TOTP shared secrets, session tokens, recovery-code plaintext. The application
-- is responsible for redacting before insert; the column type cannot enforce
-- that, only a test can.

CREATE TABLE app.activity (
	id TEXT NOT NULL PRIMARY KEY,

	engagement_id TEXT,
	actor_id TEXT,

	-- object.past_tense_verb — "user.created", "session.login_failed". The
	-- vocabulary grows with M3–M6; the naming pattern is fixed here.
	verb TEXT NOT NULL,

	object_type TEXT NOT NULL,
	object_id TEXT NOT NULL,

	delta JSON,

	-- UTC. Ordering within a millisecond is by id (UUIDv7), which is why that
	-- identifier was chosen. "at" is quoted: AT is a reserved word.
	"at" TIMESTAMP NOT NULL
);

-- Engagement timeline and the SSE feed (M4): newest first inside one
-- engagement. Platform rows (engagement_id IS NULL) sit together under the
-- same index.
CREATE INDEX activity_engagement_id_at_idx
	ON app.activity (engagement_id, "at" DESC);

-- "What did this person do", asked of the platform feed and of incident review.
CREATE INDEX activity_actor_id_at_idx
	ON app.activity (actor_id, "at" DESC);
