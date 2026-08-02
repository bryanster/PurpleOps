package store

// An internal test, because what it checks — how a driver error is classified
// — is deliberately not exported. The behaviour it stands for is asserted end
// to end against two real processes in internal/cli
// (TestRefusesADatabaseAnotherProcessHolds); this is the fast, deterministic
// half, and the one that will notice if a DuckDB upgrade rewords the message.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/duckdb/duckdb-go/v2"
)

// errLockConflict is the error DuckDB v2.10505 returns when another process holds
// the file, copied from a real one.
var errLockConflict = fmt.Errorf("database/sql/driver: could not connect to database: %w",
	&duckdb.Error{
		Type: duckdb.ErrorTypeIO,
		Msg: `IO Error: Could not set lock on file "/var/lib/blacklight/blacklight.duckdb": ` +
			`Conflicting lock is held in /usr/local/bin/blacklight (PID 1) by user blacklight. ` +
			`See also https://duckdb.org/docs/stable/connect/concurrency`,
	})

func TestALockConflictIsTaggedAsSuch(t *testing.T) {
	t.Parallel()

	err := openError("/var/lib/blacklight/blacklight.duckdb", errLockConflict)

	if !errors.Is(err, ErrLocked) {
		t.Errorf("openError(%v) is not ErrLocked, so a caller cannot tell a busy database "+
			"from a broken one", err)
	}
	// The driver's own text survives: it names the process and the PID, which
	// is the part an operator acts on.
	for _, want := range []string{"/var/lib/blacklight/blacklight.duckdb", "PID 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

func TestOtherFailuresAreNotTaggedAsLocked(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"another IO error": fmt.Errorf("connect: %w", &duckdb.Error{
			Type: duckdb.ErrorTypeIO,
			Msg:  `IO Error: Cannot open file "/var/lib/blacklight/blacklight.duckdb": Permission denied`,
		}),
		"a message that mentions locks but is not from DuckDB": errors.New(
			"Conflicting lock is held in some other tool"),
		"nothing to do with the driver": errors.New("disk full"),
	}

	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := openError("/x.duckdb", err); errors.Is(got, ErrLocked) {
				t.Errorf("openError(%v) is tagged ErrLocked, which would send an operator "+
					"looking for a process that does not exist", err)
			}
		})
	}
}
