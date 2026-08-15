// Package settings stores the decisions an administrator makes on behalf of the
// whole installation: today whether a second factor is required (M1-008), later
// whatever M2–M6 needs a checkbox for.
//
// It is storage and nothing else, as internal/store/identity is. What a setting
// means, what its default is and who may change it all live above this package —
// here a setting is a key, a string, and who last wrote it.
//
// # Keys are owned by their reader
//
// This package knows no key names. The code that acts on a setting declares its
// key and encodes its own value, so that adding one is a change in one place
// rather than in a table here and a caller there. A key this deployment does not
// recognise is returned like any other and ignored by everything — which is what
// lets a binary run against a database written by a newer one.
//
// # Absence is the default
//
// [Store.All] returns what has been written and does not invent rows. A setting
// nobody has touched is absent, and its reader supplies the default — for
// M1-008, "not required". A deployment that has never been configured is
// therefore configured correctly.
package settings

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/store"
)

// DB is the part of the store this package needs: pooled reads, and writes
// serialized into one transaction at a time. [store.Store] satisfies it.
//
// Declared here, in the package that consumes it, for the reason
// internal/store/identity gives: the dependency is the two methods called rather
// than everything a database happens to offer, and a test can substitute one.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// Setting is one stored decision.
type Setting struct {
	Key   string
	Value string

	UpdatedAt time.Time

	// UpdatedBy is the user who last wrote it, or "" when nothing did — a value
	// set from the command line, or by a migration. "Who turned this off" is
	// the first question asked once it turns out to be off.
	UpdatedBy string
}

// Store reads and writes platform settings. Construct it with [New].
type Store struct {
	db DB
}

// New returns a store over db.
func New(db DB) *Store { return &Store{db: db} }

// All returns every stored setting, keyed by key.
//
// Every setting, rather than one lookup per key: the table holds a handful of
// rows and is read on paths that already do two queries, so one query that
// returns all of it beats three that each return a row. A caller wanting one
// value indexes the map — and a missing key is the zero [Setting], which is the
// same answer as "never set".
func (s *Store) All(ctx context.Context) (map[string]Setting, error) {
	rows, err := s.db.Read().QueryContext(ctx,
		`SELECT key, value, updated_at, updated_by FROM app.platform_setting`)
	if err != nil {
		return nil, fmt.Errorf("settings: read the platform settings: %w", err)
	}
	defer rows.Close()

	all := map[string]Setting{}
	for rows.Next() {
		var (
			setting   Setting
			updatedBy sql.NullString
		)
		if err := rows.Scan(&setting.Key, &setting.Value,
			&setting.UpdatedAt, &updatedBy); err != nil {
			return nil, fmt.Errorf("settings: read the platform settings: %w", err)
		}
		setting.UpdatedAt = setting.UpdatedAt.UTC()
		setting.UpdatedBy = updatedBy.String
		all[setting.Key] = setting
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settings: read the platform settings: %w", err)
	}
	return all, nil
}

// Put writes values, creating or replacing each, and records who did it. An
// empty updatedBy stores NULL: nobody did, which is what the command line and a
// migration are.
//
// Every key in one write transaction, which is the reason this takes a map
// rather than a key and a value. A policy made of two booleans has to change as
// one thing — otherwise a request that half-succeeds leaves a state neither the
// caller nor the administrator asked for, and the difference between the two
// halves is exactly what somebody would then be locked out by.
//
// An empty map writes nothing and is not an error: a caller that computed no
// changes has nothing to say, and refusing it would push that check into every
// caller.
func (s *Store) Put(ctx context.Context, values map[string]string, updatedBy string) error {
	if len(values) == 0 {
		return nil
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	by := any(updatedBy)
	if updatedBy == "" {
		by = nil
	}

	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		for key, value := range values {
			// Delete-then-insert rather than an UPSERT: DuckDB's ON CONFLICT
			// syntax is its own, and PLAN.md §1 keeps SQL outside
			// internal/store/duckdb portable. Both statements run inside the
			// one serialized writer, so there is no window in which the row is
			// missing to anybody.
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM app.platform_setting WHERE key = ?`, key); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.platform_setting (key, value, updated_at, updated_by)
					VALUES (?, ?, ?, ?)`, key, value, at, by); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("settings: write %d platform settings: %w", len(values), err)
	}
	return nil
}

// Delete removes keys, and does not mind which of them were there.
//
// Deleting rather than writing a zero is for the one case where "never set" is
// the state a caller means to restore: `blctl setup reset` puts an installation
// back to the one the image ships, and the first-run wizard's whole question is
// whether anybody has written that row. Every other setting in this table turns
// off by storing false, for the reason 0006_platform_setting sets out — "never
// set" and "set and then turned off" should stay distinguishable — so reach for
// [Store.Put] unless absence is genuinely the value.
//
// Missing keys are not an error: the caller asked for them gone, and they are.
func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, key := range keys {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM app.platform_setting WHERE key = ?`, key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("settings: delete %d platform settings: %w", len(keys), err)
	}
	return nil
}
