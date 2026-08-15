package cli

import (
	"strings"
	"testing"
)

// `blctl setup`, which is how a provisioning run and the end-to-end harness get
// past a wizard they have already done the work of.

func TestSetupStartsUnfinished(t *testing.T) {
	db := migratedDB(t)

	got := run(t, nil, "setup", "status", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if result := decodeJSON(t, got.stdout); result["completed"] != false {
		t.Errorf("completed = %v on a fresh database, want false", result["completed"])
	}
}

func TestSetupCompleteIsReadBackAndDoesNotMoveOnARepeat(t *testing.T) {
	db := migratedDB(t)

	first := run(t, nil, "setup", "complete", "--db", db, "--json")
	if first.code != ExitOK {
		t.Fatalf("complete exited %d: %s", first.code, first.stderr)
	}
	done := decodeJSON(t, first.stdout)
	if done["completed"] != true {
		t.Fatalf("completed = %v after complete, want true", done["completed"])
	}
	if done["completedAt"] == nil || done["completedAt"] == "" {
		t.Error("completedAt is empty; when an installation was set up is worth recording")
	}
	// Nobody signed in to run this, and the row says so rather than naming a
	// user who did not do it.
	if by, ok := done["completedBy"]; ok && by != "" {
		t.Errorf("completedBy = %v, want it absent for a command-line write", by)
	}

	status := run(t, nil, "setup", "status", "--db", db, "--json")
	if result := decodeJSON(t, status.stdout); result["completedAt"] != done["completedAt"] {
		t.Errorf("status reads %v, complete wrote %v", result["completedAt"], done["completedAt"])
	}

	again := run(t, nil, "setup", "complete", "--db", db, "--json")
	if result := decodeJSON(t, again.stdout); result["completedAt"] != done["completedAt"] {
		t.Errorf("a second complete moved completedAt to %v from %v", result["completedAt"], done["completedAt"])
	}
}

func TestSetupResetBringsTheWizardBack(t *testing.T) {
	db := migratedDB(t)

	if got := run(t, nil, "setup", "complete", "--db", db); got.code != ExitOK {
		t.Fatalf("complete exited %d: %s", got.code, got.stderr)
	}

	got := run(t, nil, "setup", "reset", "--db", db, "--json")
	if got.code != ExitOK {
		t.Fatalf("reset exited %d: %s", got.code, got.stderr)
	}
	if result := decodeJSON(t, got.stdout); result["completed"] != false {
		t.Errorf("completed = %v after reset, want false", result["completed"])
	}

	// Resetting something that was never set is not a failure: the caller asked
	// for first-run state and that is what they have.
	if second := run(t, nil, "setup", "reset", "--db", db); second.code != ExitOK {
		t.Errorf("a second reset exited %d, want %d: %s", second.code, ExitOK, second.stderr)
	}
}

func TestSetupStatusSaysWhatHappensNext(t *testing.T) {
	db := migratedDB(t)

	got := run(t, nil, "setup", "status", "--db", db)
	if got.code != ExitOK {
		t.Fatalf("exited %d: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "setup wizard") {
		t.Errorf("the human-readable output does not say what an unfinished setup means:\n%s", got.stdout)
	}
}

func TestSetupNeedsAMigratedDatabase(t *testing.T) {
	got := run(t, nil, "setup", "status", "--db", tempDB(t))
	if got.code == ExitOK {
		t.Fatalf("status succeeded against a database with no schema:\n%s", got.stdout)
	}
}
