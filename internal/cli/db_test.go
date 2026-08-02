package cli

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
)

func TestDBInfoDescribesTheDatabase(t *testing.T) {
	db := tempDB(t)
	if up := runIn(t, db, "migrate", "up"); up.code != ExitOK {
		t.Fatalf("migrate up exited %d\nstderr: %s", up.code, up.stderr)
	}
	// Two rows in a table of its own, so the counts below cannot pass by
	// accident on an empty schema.
	createWidgets(t, db, 2)

	got := runIn(t, db, "db", "info", "--json")
	if got.code != ExitOK {
		t.Fatalf("db info exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	info := decodeJSON(t, got.stdout)

	if path := field[string](t, info, "path"); path != db {
		t.Errorf("path = %q, want %s", path, db)
	}
	if size := field[float64](t, info, "sizeBytes"); size <= 0 {
		t.Errorf("sizeBytes = %v, want the size of a database that exists", size)
	}
	if got, want := field[float64](t, info, "schemaVersion"),
		field[float64](t, info, "expectedSchemaVersion"); got != want {
		t.Errorf("schemaVersion = %v, want the %v this binary carries", got, want)
	}
	if pending := field[float64](t, info, "pendingMigrations"); pending != 0 {
		t.Errorf("pendingMigrations = %v after migrate up, want 0", pending)
	}

	rows := map[string]float64{}
	for _, table := range objects(t, info, "tables") {
		name := field[string](t, table, "schema") + "." + field[string](t, table, "name")
		rows[name] = field[float64](t, table, "rows")
	}
	// One from each schema: the migrator's bookkeeping in "main", and a table
	// in "app", where every domain table will live (migration 0001). A listing
	// of the current schema alone would miss the second.
	if got, want := rows["app.widgets"], float64(2); got != want {
		t.Errorf("app.widgets has %v rows, want %v (tables seen: %v)", got, want, rows)
	}
	if got, want := rows["main.schema_migrations"], float64(1); got < want {
		t.Errorf("main.schema_migrations has %v rows, want at least %v", got, want)
	}

	// Every table a migration created is listed, including the ones whose names
	// need quoting — app."user" is a reserved word, and a listing that silently
	// dropped it would be worse than one that failed (M1-001).
	for _, table := range []string{
		"app.user", "app.identity", "app.session", "app.engagement_member",
	} {
		if _, listed := rows[table]; !listed {
			t.Errorf("%s is missing from db info (tables seen: %v)", table, rows)
		}
	}
}

func TestDBInfoIsReadableWithoutJSON(t *testing.T) {
	db := tempDB(t)
	runIn(t, db, "migrate", "up")

	got := runIn(t, db, "db", "info")
	if got.code != ExitOK {
		t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	for _, want := range []string{db, "size", "write-ahead log", "schema version", "TABLE", "ROWS"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the report does not mention %q:\n%s", want, got.stdout)
		}
	}
}

// TestRefusesADatabaseAnotherProcessHolds is the failure an operator meets on
// their first day: running popsctl against a deployment whose server is up.
// DuckDB reports it as an IO error about locks, and this asserts we turn that
// into an instruction.
//
// It needs a second *process*, not a second [store.Open]: within one process
// DuckDB shares the instance and the second open succeeds, so an in-process
// version of this test would prove the opposite of what it claims. The commands
// themselves still run in this one.
func TestRefusesADatabaseAnotherProcessHolds(t *testing.T) {
	db := tempDB(t)
	// Migrated first, so the failure below is unambiguously about the lock
	// rather than about a database that does not exist yet.
	if up := runIn(t, db, "migrate", "up"); up.code != ExitOK {
		t.Fatalf("migrate up exited %d\nstderr: %s", up.code, up.stderr)
	}
	release := holdDatabase(t, db)

	for _, args := range [][]string{{"db", "info"}, {"migrate", "status"}, {"migrate", "up"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			got := runIn(t, db, args...)

			if got.code != ExitFailure {
				t.Fatalf("exited %d, want %d\nstdout: %s\nstderr: %s",
					got.code, ExitFailure, got.stdout, got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("a failed command wrote to stdout:\n%s", got.stdout)
			}
			// The three things the message has to carry: which file, that
			// somebody else has it, and what to do instead.
			for _, want := range []string{db, "one process at a time", "docker compose run"} {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("the error does not mention %q:\n%s", want, got.stderr)
				}
			}
		})
	}

	// The control: the same command succeeds the moment the holder lets go.
	// Without it the test would still pass if every command were broken.
	release()
	if got := runIn(t, db, "db", "info"); got.code != ExitOK {
		t.Errorf("db info exited %d once the database was free, want 0\nstderr: %s",
			got.code, got.stderr)
	}
}

// createWidgets puts a table with rows in it into the app schema, opening the
// database directly because no command can do this yet. It closes before
// returning: the CLI under test needs the file back.
func createWidgets(t *testing.T, path string, rows int) {
	t.Helper()

	ctx := context.Background()
	db, err := store.Open(ctx, config.Database{Path: path})
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("closing %s: %v", path, err)
		}
	}()

	err = db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE app.widgets (id INTEGER)`); err != nil {
			return err
		}
		for i := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO app.widgets VALUES (?)`, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("creating app.widgets: %v", err)
	}
}

// --- the second process ------------------------------------------------------

// heldMarker is what testdata/dbholder prints once it has the database. The
// test must not race ahead of it, or it passes for the wrong reason on a slow
// machine.
const heldMarker = "HELD"

// holdDatabase compiles and starts the holder, waits until it has the database,
// and returns the function that releases it. The holder is stopped when the
// test ends, whether or not that function was called.
func holdDatabase(t *testing.T, path string) (release func()) {
	t.Helper()

	holder := filepath.Join(t.TempDir(), "dbholder")
	// CGO is on by default and the DuckDB driver needs it; setting it here
	// would only mask a broken environment.
	if out, err := exec.CommandContext(t.Context(),
		"go", "build", "-o", holder, "./testdata/dbholder").CombinedOutput(); err != nil {
		t.Fatalf("building the database holder: %v\n%s", err, out)
	}

	child := exec.CommandContext(t.Context(), holder, path)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatalf("piping stdin to the holder: %v", err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatalf("piping stdout from the holder: %v", err)
	}
	if err := child.Start(); err != nil {
		t.Fatalf("starting the holder: %v", err)
	}

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		// Closing stdin is the polite stop: the holder closes the database and
		// exits 0. Killing it would leave DuckDB no chance to release the lock
		// before t.TempDir tries to remove the file.
		if err := stdin.Close(); err != nil {
			t.Errorf("closing the holder's stdin: %v", err)
		}
		if err := child.Wait(); err != nil {
			t.Errorf("the holder exited badly: %v", err)
		}
	}
	t.Cleanup(stop)

	waitFor(t, stdout, heldMarker)
	return stop
}

// waitFor reads lines until one contains marker, or the holder gives up.
func waitFor(t *testing.T, r io.Reader, marker string) {
	t.Helper()

	done := make(chan string, 1)
	go func() {
		var seen strings.Builder
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			seen.WriteString(scanner.Text() + "\n")
			if strings.Contains(scanner.Text(), marker) {
				done <- ""
				return
			}
		}
		done <- seen.String()
	}()

	select {
	case output := <-done:
		if output != "" {
			t.Fatalf("the holder never reported %q before it stopped:\n%s", marker, output)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("the holder never took the database")
	}
}
