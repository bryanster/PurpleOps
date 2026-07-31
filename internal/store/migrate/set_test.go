package migrate_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bryanster/purpleops/internal/store/migrate"
)

// The set is validated by New, before a database is involved at all, so these
// tests need no store. TestABrokenSetIsRejectedBeforeAnySQLRuns, in
// migrate_test.go, is what proves that ordering holds.

func TestNewRejectsAMalformedSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files fstest.MapFS
		// want are substrings the error must contain: what is wrong, and enough
		// to find it with.
		want []string
	}{
		{
			name:  "empty",
			files: fstest.MapFS{},
			want:  []string{"empty"},
		},
		{
			name: "duplicate versions",
			files: set(
				"0001_init.sql", "SELECT 1",
				"0002_users.sql", "SELECT 1",
				"0002_sessions.sql", "SELECT 1",
			),
			want: []string{"0002_sessions.sql", "0002_users.sql", "cannot share a version"},
		},
		{
			name: "a gap in the sequence",
			files: set(
				"0001_init.sql", "SELECT 1",
				"0003_users.sql", "SELECT 1",
			),
			want: []string{"0003_users.sql", "is version 3 where 0002 was expected", "no gaps"},
		},
		{
			name:  "not starting at one",
			files: set("0002_init.sql", "SELECT 1"),
			want:  []string{"0002_init.sql", "0001 was expected"},
		},
		{
			name:  "version zero",
			files: set("0000_init.sql", "SELECT 1"),
			want:  []string{"0000_init.sql", "0001 was expected"},
		},
		{
			name:  "an unpadded version",
			files: set("1_init.sql", "SELECT 1"),
			want:  []string{"1_init.sql", "padded to four digits"},
		},
		{
			name:  "an uppercase name",
			files: set("0001_Init.sql", "SELECT 1"),
			want:  []string{"0001_Init.sql", "lower_snake_case"},
		},
		{
			name:  "a name with no version",
			files: set("init.sql", "SELECT 1"),
			want:  []string{"init.sql", "not a migration filename"},
		},
		{
			name: "a stray non-SQL file",
			files: set(
				"0001_init.sql", "SELECT 1",
				"0002_users.sql.bak", "SELECT 1",
			),
			want: []string{"0002_users.sql.bak", "not a migration filename"},
		},
		{
			name: "a subdirectory",
			files: fstest.MapFS{
				"0001_init.sql":         {Data: []byte("SELECT 1")},
				"archive/0000_init.sql": {Data: []byte("SELECT 1")},
			},
			want: []string{"archive", "is a directory"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, err := migrate.New(tc.files)
			if err == nil {
				t.Fatalf("New(%v) = %v, nil; want an error", tc.files, m)
			}
			assertErrorMentions(t, err, tc.want...)
		})
	}
}

func TestNewAcceptsAWellFormedSet(t *testing.T) {
	t.Parallel()

	// Deliberately not in filename order in the map, and with a two-word name:
	// the set is sorted by version, not by however the directory is read.
	m, err := migrate.New(set(
		"0003_add_indexes.sql", "SELECT 3",
		"0001_init.sql", "SELECT 1",
		"0002_users2.sql", "SELECT 2",
	))
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	states, err := m.Status(t.Context(), newDB(t))
	if err != nil {
		t.Fatalf("Status() = %v, want nil", err)
	}

	var got []string
	for _, state := range states {
		got = append(got, state.Filename())
	}
	want := []string{"0001_init.sql", "0002_users2.sql", "0003_add_indexes.sql"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("set is\n\t%v\nwant\n\t%v", got, want)
	}
}

func TestChecksumIgnoresLineEndings(t *testing.T) {
	t.Parallel()

	// A checkout with core.autocrlf=true delivers the same committed migration
	// as different bytes. That must not read as an edited migration.
	unix := set("0001_init.sql", "CREATE SCHEMA app;\nCREATE SCHEMA content;\n")
	windows := set("0001_init.sql", "CREATE SCHEMA app;\r\nCREATE SCHEMA content;\r\n")

	db := newDB(t)
	if _, err := applyAll(t, db, unix); err != nil {
		t.Fatalf("applying the LF copy: %v", err)
	}
	if _, err := applyAll(t, db, windows); err != nil {
		t.Fatalf("the CRLF copy of the same migration was rejected: %v", err)
	}
}

func TestChecksumDistinguishesRealEdits(t *testing.T) {
	t.Parallel()

	// The counterpart to the test above: normalising line endings must not have
	// made the checksum blind to a change that matters.
	original := set("0001_init.sql", "CREATE SCHEMA app;\n")
	edited := set("0001_init.sql", "CREATE SCHEMA app;\nCREATE SCHEMA content;\n")

	db := newDB(t)
	if _, err := applyAll(t, db, original); err != nil {
		t.Fatalf("applying the original: %v", err)
	}
	if _, err := applyAll(t, db, edited); err == nil {
		t.Fatal("an edited migration was accepted; want a checksum error")
	}
}
