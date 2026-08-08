-- 0006_platform_setting — the decisions an administrator makes on behalf of the
-- whole installation, starting with whether a second factor is required
-- (M1-008).
--
-- One key/value table rather than one column per decision. The alternative — a
-- single-row `app.settings` with a typed column for each — reads better in SQL
-- and costs a migration every time a setting is added, which over M2–M6 is a
-- migration for every checkbox. What is lost is the database checking the shape
-- of a value; what replaces it is that nothing writes this table except a typed
-- encoder in Go (internal/store/settings), and every read goes through the
-- matching decoder, which reports a value it cannot read rather than guessing.
--
-- Absence is the default, deliberately. A deployment that has never been
-- configured has no rows here, and every setting reads as its zero value — for
-- M1-008 that is "not required", so a fresh installation does not lock its first
-- administrator into enrolling before they have seen the product. Turning a
-- policy off writes `false` rather than deleting the row, so "never set" and
-- "set and then turned off" stay distinguishable to whoever reads updated_by.
--
-- No foreign key on updated_by, for the reason 0003_user_updatable sets out at
-- length: DuckDB implements UPDATE as a delete and an insert, and the delete
-- half runs the RESTRICT check, so a referenced user row could not be edited at
-- all.
CREATE TABLE app.platform_setting (
	-- Dotted and lower_snake_case — "mfa.required_for_all". The Go side holds
	-- the only list of keys that mean anything; a row with any other key is
	-- ignored on read rather than being an error, so a downgrade after a
	-- setting is added does not fail to start.
	--
	-- `key` and `value` are non-reserved in both DuckDB and PostgreSQL, so
	-- neither needs the quoting app."user" does.
	key TEXT NOT NULL PRIMARY KEY,

	-- The value, encoded by whoever owns the key: "true"/"false" for the two
	-- booleans M1-008 stores. TEXT rather than a union of typed columns
	-- because a setting's type is the Go type that reads it, and a second
	-- declaration of it here would be one more thing to keep in step.
	value TEXT NOT NULL,

	updated_at TIMESTAMP NOT NULL,

	-- Who changed it, or NULL when nothing did — a value written by a
	-- migration or by the command line rather than by a person. "Who turned
	-- MFA off" is the first question asked after it turns out to be off.
	updated_by TEXT
);
