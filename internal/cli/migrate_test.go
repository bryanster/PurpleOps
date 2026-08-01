package cli

import (
	"strings"
	"testing"
	"time"
)

// TestMigrateStatusBeforeAndAfterUp walks the sequence an operator performs on
// a new deployment, and asserts what they are shown at each step.
func TestMigrateStatusBeforeAndAfterUp(t *testing.T) {
	db := tempDB(t)

	before := runIn(t, db, "migrate", "status", "--json")
	if before.code != ExitOK {
		t.Fatalf("migrate status exited %d, want 0\nstderr: %s", before.code, before.stderr)
	}
	pending := decodeJSON(t, before.stdout)

	migrations := objects(t, pending, "migrations")
	if len(migrations) == 0 {
		t.Fatalf("migrate status reported no migrations at all: %v", pending)
	}
	if got := field[float64](t, pending, "schemaVersion"); got != 0 {
		t.Errorf("schemaVersion = %v on a fresh database, want 0", got)
	}
	if got, want := field[float64](t, pending, "pending"), float64(len(migrations)); got != want {
		t.Errorf("pending = %v, want %v: nothing is applied yet", got, want)
	}
	for _, migration := range migrations {
		file := field[string](t, migration, "file")
		if field[bool](t, migration, "applied") {
			t.Errorf("%s is reported as applied on a fresh database", file)
		}
		if _, dated := migration["appliedAt"]; dated {
			t.Errorf("%s has an appliedAt and has not been applied", file)
		}
	}

	if up := runIn(t, db, "migrate", "up"); up.code != ExitOK {
		t.Fatalf("migrate up exited %d, want 0\nstderr: %s", up.code, up.stderr)
	}

	after := runIn(t, db, "migrate", "status", "--json")
	if after.code != ExitOK {
		t.Fatalf("migrate status exited %d, want 0\nstderr: %s", after.code, after.stderr)
	}
	applied := decodeJSON(t, after.stdout)

	if got := field[float64](t, applied, "pending"); got != 0 {
		t.Errorf("pending = %v after migrate up, want 0", got)
	}
	if got, want := field[float64](t, applied, "schemaVersion"),
		field[float64](t, applied, "expectedSchemaVersion"); got != want {
		t.Errorf("schemaVersion = %v but this binary carries %v", got, want)
	}
	for _, migration := range objects(t, applied, "migrations") {
		file := field[string](t, migration, "file")
		if !field[bool](t, migration, "applied") {
			t.Errorf("%s is still pending after migrate up", file)
			continue
		}
		// The timestamp is the point of the field: "applied" without "when"
		// does not tell an operator whether it was this deployment or the one
		// before it.
		stamp := field[string](t, migration, "appliedAt")
		when, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			t.Errorf("%s was applied at %q, which is not RFC 3339: %v", file, stamp, err)
			continue
		}
		if time.Since(when) > time.Hour {
			t.Errorf("%s claims it was applied at %s, which is not just now", file, stamp)
		}
	}
}

// TestMigrateStatusIsReadableWithoutJSON checks the form a person sees: the
// table an operator reads over somebody's shoulder during a deployment.
func TestMigrateStatusIsReadableWithoutJSON(t *testing.T) {
	db := tempDB(t)
	runIn(t, db, "migrate", "up")

	got := runIn(t, db, "migrate", "status")
	if got.code != ExitOK {
		t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	for _, want := range []string{"VERSION", "APPLIED AT", "0001", "applied", "0 pending"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the table does not mention %q:\n%s", want, got.stdout)
		}
	}
}

// TestMigrateUpOnAnUpToDateDatabaseChangesNothing: running it twice is what a
// re-run of a deployment script does, and it must be boring.
func TestMigrateUpOnAnUpToDateDatabaseChangesNothing(t *testing.T) {
	db := tempDB(t)

	first := runIn(t, db, "migrate", "up", "--json")
	if first.code != ExitOK {
		t.Fatalf("the first migrate up exited %d, want 0\nstderr: %s", first.code, first.stderr)
	}
	if applied := field[[]any](t, decodeJSON(t, first.stdout), "appliedNow"); len(applied) == 0 {
		t.Fatalf("the first migrate up reported applying nothing: %s", first.stdout)
	}

	second := runIn(t, db, "migrate", "up", "--json")
	if second.code != ExitOK {
		t.Fatalf("the second migrate up exited %d, want 0\nstderr: %s", second.code, second.stderr)
	}
	if applied, reported := decodeJSON(t, second.stdout)["appliedNow"]; reported {
		t.Errorf("the second migrate up claims it applied %v", applied)
	}

	human := runIn(t, db, "migrate", "up")
	if !strings.Contains(human.stdout, "Already up to date") {
		t.Errorf("a no-op migrate up does not say so:\n%s", human.stdout)
	}
}
