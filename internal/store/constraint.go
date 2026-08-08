package store

import (
	"errors"
	"strings"

	"github.com/duckdb/duckdb-go/v2"
)

// Unique-violation messages, in full:
//
//	Constraint Error: Duplicate key "email_normalized: a@x.com" violates unique constraint.
//	Constraint Error: Duplicate key "engagement_id: e1, user_id: u2" violates primary key constraint.
const (
	uniqueMarker     = "violates unique constraint"
	primaryKeyMarker = "violates primary key constraint"
)

// IsUniqueViolation reports whether err is the database refusing a row because
// another one already holds the same key: an email that is taken, a session
// token that already exists, an engagement member added twice.
//
// It lives here rather than in a repository because it is a fact about DuckDB,
// and this package is the one place allowed to know those (see the package
// comment). A caller uses it to answer "that email address is already in use"
// instead of reporting the driver's error as a server fault.
//
// A primary key counts: to a caller inserting a row that is already there, the
// difference between the two constraints is not something they can act on.
//
// Matching on message text is fragile, and the fragility is bounded the same
// way it is for [isLockConflict]: the category ([duckdb.Error.Type]) narrows it
// to constraint failures, and getting the message wrong costs a clearer error
// message, never correctness — the write has already been refused either way.
func IsUniqueViolation(err error) bool {
	var dbErr *duckdb.Error
	if !errors.As(err, &dbErr) || dbErr.Type != duckdb.ErrorTypeConstraint {
		return false
	}
	return strings.Contains(dbErr.Msg, uniqueMarker) ||
		strings.Contains(dbErr.Msg, primaryKeyMarker)
}
