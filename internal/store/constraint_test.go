package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/storetest"
)

// TestIsUniqueViolationRecognisesADuplicateKey runs against a real database
// rather than a hand-made error: what this function has to keep up with is
// DuckDB's message, and only DuckDB can produce it.
func TestIsUniqueViolationRecognisesADuplicateKey(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	mustExec(t, db, `CREATE TABLE t (
		id TEXT NOT NULL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		role TEXT NOT NULL CHECK (role IN ('admin', 'member'))
	)`)
	mustExec(t, db, `CREATE TABLE child (
		id TEXT NOT NULL PRIMARY KEY,
		parent TEXT NOT NULL REFERENCES t (id)
	)`)
	mustExec(t, db, `INSERT INTO t VALUES ('1', 'a@x.com', 'admin')`)

	cases := []struct {
		name string
		stmt string
		want bool
	}{
		{"duplicate unique column", `INSERT INTO t VALUES ('2', 'a@x.com', 'member')`, true},
		{"duplicate primary key", `INSERT INTO t VALUES ('1', 'b@x.com', 'member')`, true},
		// The other constraint failures are refusals too, but they are not
		// "somebody already has that" and must not be reported as a conflict.
		{"check violation", `INSERT INTO t VALUES ('3', 'c@x.com', 'wizard')`, false},
		{"foreign key violation", `INSERT INTO child VALUES ('c1', 'nobody')`, false},
		{"not a constraint at all", `INSERT INTO t VALUES ('4')`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Write(t.Context(), func(tx *sql.Tx) error {
				_, err := tx.ExecContext(t.Context(), tc.stmt)
				return err
			})
			if err == nil {
				t.Fatal("the statement succeeded; it was written to fail")
			}
			if got := store.IsUniqueViolation(err); got != tc.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
}

// TestIsUniqueViolationIgnoresEverythingElse: a nil error, and an error from
// somewhere that is not the driver, are both "no".
func TestIsUniqueViolationIgnoresEverythingElse(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		nil,
		errors.New("violates unique constraint"), // The words, from the wrong source.
		store.ErrClosed,
		fmt.Errorf("wrapped: %w", sql.ErrNoRows),
	} {
		if store.IsUniqueViolation(err) {
			t.Errorf("IsUniqueViolation(%v) = true, want false", err)
		}
	}
}

func mustExec(t *testing.T, db *store.DB, stmt string) {
	t.Helper()

	err := db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), stmt)
		return err
	})
	if err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}
