package cli

import (
	"os"
	"strings"
	"testing"
)

// TestPendingCommandsSayWhenTheyArrive. Each of these is registered so that the
// shape of the tool is visible now, and each has to fail in a way that a script
// notices (exit 1) and a person can act on (a milestone, not "soon").
func TestPendingCommandsSayWhenTheyArrive(t *testing.T) {
	tests := []struct {
		args      []string
		milestone string
	}{
		{[]string{"content", "sync"}, "M2"},
		{[]string{"backup"}, "M7"},
		{[]string{"report", "render"}, "M6"},
	}

	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got := runIn(t, tempDB(t), tc.args...)

			if got.code != ExitFailure {
				t.Errorf("exited %d, want %d: a command that did nothing must not report success",
					got.code, ExitFailure)
			}
			if got.stdout != "" {
				t.Errorf("wrote to stdout, which is for results:\n%s", got.stdout)
			}
			if !strings.Contains(got.stderr, "not implemented") {
				t.Errorf("the error does not say it is not implemented:\n%s", got.stderr)
			}
			if !strings.Contains(got.stderr, tc.milestone) {
				t.Errorf("the error does not name %s, so nobody can tell when it arrives:\n%s",
					tc.milestone, got.stderr)
			}
		})
	}
}

// TestPendingCommandsTouchNothing: a command that cannot do its job must not
// have created a database on the way to saying so.
func TestPendingCommandsTouchNothing(t *testing.T) {
	db := tempDB(t)

	if got := runIn(t, db, "backup"); got.code != ExitFailure {
		t.Fatalf("backup exited %d, want %d", got.code, ExitFailure)
	}
	if _, err := os.Stat(db); err == nil {
		t.Errorf("%s was created by a command that is not implemented", db)
	}
}
