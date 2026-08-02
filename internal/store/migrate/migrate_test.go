package migrate_test

import (
	"bytes"
	"io"
	"log/slog"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/migrate"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// These tests run against a real DuckDB file (see storetest) rather than a mock
// database, because most of what this package promises — that a failed
// migration leaves nothing behind, that DDL rolls back, that a second process
// cannot race — is a property of DuckDB, and a mock would agree with whatever
// the implementation happened to do.
//
// Test migrations append their version to one row of a "trail" table, so the
// order they ran in, and that each ran exactly once, is a single assertion.

// trail is a well-formed three-migration set. After all three, trail.path
// reads "123".
func trail() fstest.MapFS {
	return set(
		"0001_init.sql", "CREATE TABLE trail (path TEXT); INSERT INTO trail VALUES ('1');",
		"0002_second.sql", "UPDATE trail SET path = path || '2';",
		"0003_third.sql", "UPDATE trail SET path = path || '3';",
	)
}

func TestUpFromEmptyAppliesEveryMigrationInOrder(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	applied := mustApply(t, db, trail())

	assertVersions(t, applied, 1, 2, 3)
	assertTrail(t, db, "123")
	assertAppliedVersions(t, db, 1, 2, 3)
}

func TestUpRecordsWhatItApplied(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	before := time.Now().UTC()
	applied := mustApply(t, db, trail())

	m, err := migrate.New(trail(), quiet())
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Status(t.Context(), db)
	if err != nil {
		t.Fatalf("Status() = %v, want nil", err)
	}

	for i, state := range got {
		if !state.Applied {
			t.Fatalf("%s is not recorded as applied", state.Filename())
		}
		if state.Checksum != applied[i].Checksum {
			t.Errorf("%s recorded checksum %q, want %q", state.Filename(), state.Checksum, applied[i].Checksum)
		}
		// Stored UTC and served UTC (docs/tickets/README.md, "Time").
		if _, offset := state.AppliedAt.Zone(); offset != 0 {
			t.Errorf("%s applied_at is %v, want a UTC time", state.Filename(), state.AppliedAt)
		}
		if state.AppliedAt.Before(before) || state.AppliedAt.After(time.Now().UTC().Add(time.Minute)) {
			t.Errorf("%s applied_at is %v, want a time from this test run", state.Filename(), state.AppliedAt)
		}
	}
}

func TestUpOnAnUpToDateDatabaseIsAQuietNoOp(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	mustApply(t, db, trail())

	var alarming bytes.Buffer
	applied, err := applyAll(t, db, trail(), migrate.WithLogger(warnings(&alarming)))
	if err != nil {
		t.Fatalf("second Up() = %v, want nil", err)
	}
	if len(applied) != 0 {
		t.Errorf("second Up() applied %d migrations, want 0", len(applied))
	}
	if alarming.Len() != 0 {
		t.Errorf("an up-to-date database logged at warning or above:\n%s", alarming.String())
	}
	assertTrail(t, db, "123")
}

func TestUpAppliesOnlyTheMissingMigrations(t *testing.T) {
	t.Parallel()

	// Two of the three have already run; a later deployment adds a fourth.
	db := newDB(t)
	full := trail()
	mustApply(t, db, subset(full, "0001_init.sql", "0002_second.sql"))

	full["0004_fourth.sql"] = &fstest.MapFile{Data: []byte("UPDATE trail SET path = path || '4';")}
	applied := mustApply(t, db, full)

	assertVersions(t, applied, 3, 4)
	assertTrail(t, db, "1234")
	assertAppliedVersions(t, db, 1, 2, 3, 4)
}

func TestUpAbortsAtTheFirstFailingMigration(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	broken := trail()
	// A statement that works followed by one that does not: if the file were
	// not one transaction, "partial" would survive the failure.
	broken["0002_second.sql"] = &fstest.MapFile{
		Data: []byte("CREATE TABLE partial (id INTEGER);\nTHIS IS NOT SQL;\n"),
	}

	applied, err := applyAll(t, db, broken)
	if err == nil {
		t.Fatal("Up() = nil error, want a failure")
	}
	// The operator needs the file and the database's own complaint about it.
	assertErrorMentions(t, err, "0002_second.sql", "Parser Error", "THIS")

	if len(applied) != 1 {
		t.Errorf("Up() reported %d applied migrations, want the 1 that succeeded", len(applied))
	}
	assertAppliedVersions(t, db, 1)
	assertNoTable(t, db, "partial") // the failing file rolled back whole
	assertTrail(t, db, "1")         // and 0003, after it, never ran
}

func TestAMigrationAndItsRecordCommitTogether(t *testing.T) {
	t.Parallel()

	// A migration whose effects are committed without the row that records them
	// is re-applied on the next startup, against a schema that already has it —
	// so the process then fails to start over a migration that did work. The
	// two have to be one transaction, and the window this closes is too narrow
	// to hit by racing a crash.
	//
	// So the migration is made to break the insert that follows it instead: it
	// removes the table the record is about to go into. In one transaction that
	// rolls the whole file back, bookkeeping included.
	db := newDB(t)
	hostile := trail()
	hostile["0002_second.sql"] = &fstest.MapFile{
		Data: []byte("CREATE TABLE evidence (id INTEGER);\nDROP TABLE schema_migrations;\n"),
	}

	_, err := applyAll(t, db, hostile)
	if err == nil {
		t.Fatal("Up() = nil error, want a failure to record the migration")
	}
	assertErrorMentions(t, err, "0002_second.sql", "recording it in schema_migrations")

	assertNoTable(t, db, "evidence") // the migration's own effects went back
	assertAppliedVersions(t, db, 1)  // and the bookkeeping survived with it
}

func TestUpRejectsAnEditedMigration(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	mustApply(t, db, trail())

	edited := trail()
	edited["0002_second.sql"] = &fstest.MapFile{Data: []byte("UPDATE trail SET path = path || 'edited';")}

	_, err := applyAll(t, db, edited)
	if err == nil {
		t.Fatal("Up() = nil error, want a checksum failure")
	}
	assertErrorMentions(t, err, "0002_second.sql", "has changed since it was applied", "append-only")

	// Neither re-applied nor silently skipped: the database is exactly as it was.
	assertTrail(t, db, "123")
	assertAppliedVersions(t, db, 1, 2, 3)
}

func TestUpRejectsARenamedMigration(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	mustApply(t, db, trail())

	renamed := trail()
	renamed["0002_renamed.sql"] = renamed["0002_second.sql"]
	delete(renamed, "0002_second.sql")

	_, err := applyAll(t, db, renamed)
	if err == nil {
		t.Fatal("Up() = nil error, want a rename failure")
	}
	assertErrorMentions(t, err, "0002", "second", "renamed", "append-only")
	assertTrail(t, db, "123")
}

func TestUpRejectsADatabaseMigratedByANewerBinary(t *testing.T) {
	t.Parallel()

	// A rollback to an older build. Its queries were written against a schema
	// two migrations behind the one in front of it.
	db := newDB(t)
	mustApply(t, db, trail())

	older := subset(trail(), "0001_init.sql")
	_, err := applyAll(t, db, older)
	if err == nil {
		t.Fatal("Up() = nil error, want a failure")
	}
	assertErrorMentions(t, err, "0002", "second", "older than the database")
}

func TestABrokenSetIsRejectedBeforeAnySQLRuns(t *testing.T) {
	t.Parallel()

	// The gap is found while reading files, so the database is never touched —
	// not even to create the bookkeeping table.
	db := newDB(t)
	if _, err := applyAll(t, db, set("0001_init.sql", "SELECT 1", "0003_third.sql", "SELECT 3")); err == nil {
		t.Fatal("Up() = nil error, want a failure")
	}
	assertNoTable(t, db, "schema_migrations")
}

func TestStatusReportsAppliedAndPending(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	mustApply(t, db, subset(trail(), "0001_init.sql"))

	m, err := migrate.New(trail(), quiet())
	if err != nil {
		t.Fatal(err)
	}
	states, err := m.Status(t.Context(), db)
	if err != nil {
		t.Fatalf("Status() = %v, want nil", err)
	}

	if len(states) != 3 {
		t.Fatalf("Status() reported %d migrations, want 3", len(states))
	}
	for i, want := range []bool{true, false, false} {
		if states[i].Applied != want {
			t.Errorf("%s Applied = %t, want %t", states[i].Filename(), states[i].Applied, want)
		}
	}
	if !states[1].AppliedAt.IsZero() {
		t.Errorf("a pending migration reports AppliedAt = %v, want the zero time", states[1].AppliedAt)
	}
}

func TestStatusRefusesToReassureAboutADatabaseItCannotUse(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	mustApply(t, db, trail())

	edited := trail()
	edited["0001_init.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE trail (path TEXT);")}

	m, err := migrate.New(edited, quiet())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Status(t.Context(), db); err == nil {
		t.Fatal("Status() = nil error on a drifted database, want the same failure Up gives")
	}
}

func TestTheEmbeddedMigrationsCreateTheAppAndContentSchemas(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	applied, err := migrate.Up(t.Context(), db, quiet())
	if err != nil {
		t.Fatalf("Up() = %v, want nil", err)
	}
	if len(applied) == 0 {
		t.Fatal("Up() applied nothing to an empty database")
	}
	if applied[0].Filename() != "0001_init.sql" {
		t.Errorf("the first embedded migration is %q, want %q", applied[0].Filename(), "0001_init.sql")
	}

	for _, schema := range []string{"app", "content"} {
		var n int
		if err := db.Read().QueryRowContext(t.Context(),
			"SELECT count(*) FROM information_schema.schemata WHERE schema_name = ?", schema).Scan(&n); err != nil {
			t.Fatalf("looking for the %s schema: %v", schema, err)
		}
		if n != 1 {
			t.Errorf("the %s schema does not exist after migrating", schema)
		}
	}

	// The shipped set is also the one a running server re-checks on every
	// restart, so it must be a no-op the second time.
	again, err := migrate.Up(t.Context(), db, quiet())
	if err != nil {
		t.Fatalf("second Up() = %v, want nil", err)
	}
	if len(again) != 0 {
		t.Errorf("second Up() applied %d migrations, want 0", len(again))
	}
}

func TestASecondProcessCannotMigrateTheSameFile(t *testing.T) {
	t.Parallel()

	// Two servers started against one database file. DuckDB allows one
	// read-write process, so the second must fail before it can migrate
	// anything — and it must say so in terms an operator can act on.
	//
	// This needs a real second process: DuckDB's instance cache hands two
	// handles inside one process the same instance, so the lock is invisible
	// from there.
	dir := t.TempDir()
	path := filepath.Join(dir, "blacklight.duckdb")
	probe := buildLockProbe(t, dir)

	db := openAt(t, path)
	if _, err := migrate.Up(t.Context(), db, quiet()); err != nil {
		t.Fatalf("the first process failed to migrate: %v", err)
	}

	out, err := exec.CommandContext(t.Context(), probe, path).CombinedOutput()
	if err == nil {
		t.Fatalf("the second process migrated the same open database:\n%s", out)
	}
	if !strings.Contains(string(out), "lock") {
		t.Errorf("the second process failed, but not with a lock error:\n%s", out)
	}

	// The control: once the first process lets go, the same probe succeeds and
	// finds nothing to do. Without this the test would still pass if the probe
	// were simply broken.
	if err := db.Close(); err != nil {
		t.Fatalf("closing the first handle: %v", err)
	}
	out, err = exec.CommandContext(t.Context(), probe, path).CombinedOutput()
	if err != nil {
		t.Fatalf("the second process could not migrate a closed database: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "MIGRATED") {
		t.Errorf("the second process reported %q, want MIGRATED", strings.TrimSpace(string(out)))
	}
}

// buildLockProbe compiles testdata/lockprobe into dir and returns its path.
func buildLockProbe(t *testing.T, dir string) string {
	t.Helper()

	probe := filepath.Join(dir, "lockprobe")
	// CGO is on by default and the DuckDB driver needs it; an explicit setting
	// here would only mask a broken environment.
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", probe, "./testdata/lockprobe").CombinedOutput()
	if err != nil {
		t.Fatalf("building the lock probe: %v\n%s", err, out)
	}
	return probe
}

// --- helpers ---------------------------------------------------------------

// newDB returns an empty database with no schema at all.
func newDB(t *testing.T) *store.DB {
	t.Helper()
	return storetest.New(t)
}

// openAt is storetest.New for the one test that has to choose the path and
// close the handle itself part-way through. store.DB.Close is idempotent, so
// the cleanup registered here is still correct after that.
func openAt(t *testing.T, path string) *store.DB {
	t.Helper()

	db, err := store.Open(t.Context(), config.Database{Path: path})
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing %s: %v", path, err)
		}
	})
	return db
}

// set builds a migration directory from filename/contents pairs.
func set(pairs ...string) fstest.MapFS {
	if len(pairs)%2 != 0 {
		panic("set: want filename/contents pairs")
	}
	files := make(fstest.MapFS, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		files[pairs[i]] = &fstest.MapFile{Data: []byte(pairs[i+1])}
	}
	return files
}

// subset returns the named files of a set, for the tests that migrate part of
// the way and then continue with the rest.
func subset(files fstest.MapFS, names ...string) fstest.MapFS {
	out := make(fstest.MapFS, len(names))
	for _, name := range names {
		out[name] = files[name]
	}
	return out
}

// applyAll is New followed by Up, since almost every test does both and cares
// about whichever fails first.
func applyAll(t *testing.T, db migrate.DB, files fstest.MapFS, opts ...migrate.Option) ([]migrate.Migration, error) {
	t.Helper()

	m, err := migrate.New(files, append([]migrate.Option{quiet()}, opts...)...)
	if err != nil {
		return nil, err
	}
	return m.Up(t.Context(), db)
}

func mustApply(t *testing.T, db migrate.DB, files fstest.MapFS) []migrate.Migration {
	t.Helper()

	applied, err := applyAll(t, db, files)
	if err != nil {
		t.Fatalf("applying %v: %v", filenames(files), err)
	}
	return applied
}

// quiet discards the migrator's progress. Tests that care about the log install
// their own handler instead.
func quiet() migrate.Option {
	return migrate.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// warnings captures only what an operator would read as a problem, for the test
// that asserts an up-to-date database is a quiet no-op.
func warnings(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func filenames(files fstest.MapFS) []string {
	return slices.Sorted(maps.Keys(files))
}

func assertVersions(t *testing.T, applied []migrate.Migration, want ...int) {
	t.Helper()

	got := make([]int, len(applied))
	for i, migration := range applied {
		got[i] = migration.Version
	}
	if !slices.Equal(got, want) {
		t.Errorf("Up() applied versions %v, want %v", got, want)
	}
}

// assertAppliedVersions reads schema_migrations directly: what the migrator
// reports and what the table records are two different claims.
func assertAppliedVersions(t *testing.T, db *store.DB, want ...int) {
	t.Helper()

	rows, err := db.Read().QueryContext(t.Context(), "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	defer rows.Close()

	var got []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("reading schema_migrations: %v", err)
		}
		got = append(got, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("schema_migrations holds versions %v, want %v", got, want)
	}
}

func assertTrail(t *testing.T, db *store.DB, want string) {
	t.Helper()

	var got string
	if err := db.Read().QueryRowContext(t.Context(), "SELECT path FROM trail").Scan(&got); err != nil {
		t.Fatalf("reading the migration trail: %v", err)
	}
	if got != want {
		t.Errorf("migrations ran as %q, want %q", got, want)
	}
}

func assertNoTable(t *testing.T, db *store.DB, name string) {
	t.Helper()

	var n int
	if err := db.Read().QueryRowContext(t.Context(),
		"SELECT count(*) FROM information_schema.tables WHERE table_name = ?", name).Scan(&n); err != nil {
		t.Fatalf("looking for table %q: %v", name, err)
	}
	if n != 0 {
		t.Errorf("table %q exists, want it not to", name)
	}
}

func assertErrorMentions(t *testing.T, err error, substrings ...string) {
	t.Helper()

	for _, want := range substrings {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
