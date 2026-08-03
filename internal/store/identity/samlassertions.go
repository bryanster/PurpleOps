package identity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// SAMLAssertions is the record of which SAML assertions have already been
// accepted here (M1-010). Construct it with [NewSAMLAssertions].
//
// It is the one repository in this package whose whole purpose is a constraint
// violation. Everything else stores a fact and reads it back; this stores a
// value so that storing it a second time fails, which is what makes an assertion
// single-use. 0007_saml_assertion.sql explains why that has to exist at all —
// SAML has no nonce, so a captured assertion is replayable for its whole
// validity window and nothing in the protocol stops it.
type SAMLAssertions struct {
	db DB
}

// NewSAMLAssertions returns a repository over db.
func NewSAMLAssertions(db DB) *SAMLAssertions { return &SAMLAssertions{db: db} }

// Consume records an assertion as used, and reports [apierr.Conflict] for one
// that has been used before.
//
// expiresAt is the last moment this assertion could still be replayed — its own
// NotOnOrAfter, already widened by whatever clock skew the caller allows. It is
// what lets the row be swept, and it is the caller's value rather than one
// computed here because the skew is that layer's configuration.
//
// The uniqueness is the database's, decided inside the write transaction. A
// read-then-write would let two copies of one assertion arriving together both
// find nothing and both be accepted, which is precisely the race a replay is.
//
// Expired rows are swept in the same transaction. That keeps the table bounded
// with no background job to schedule and none to forget: the table only grows
// when somebody signs in, so the sweep only has to run when somebody does.
func (r *SAMLAssertions) Consume(ctx context.Context, assertionID string, expiresAt time.Time) error {
	at := now()

	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		// Before the insert, and deliberately: an assertion whose own row has
		// expired is one this cache no longer has an opinion about, and it must
		// be possible to insert its ID again rather than for the stale row to
		// refuse it forever. Providers do reuse ID formats across long-lived
		// deployments.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM app.saml_assertion WHERE expires_at < ?`, at); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.saml_assertion (assertion_id, consumed_at, expires_at)
				VALUES (?, ?, ?)`,
			assertionID, at, toStorage(expiresAt))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		// Conflict rather than a bare error: it is a fact about the request, not
		// a fault, and the layer above turns it into the same refusal every other
		// bad assertion gets.
		return apierr.Conflict("that assertion has already been used")
	case err != nil:
		return fmt.Errorf("identity: consume SAML assertion %q: %w", assertionID, err)
	}
	return nil
}

// Count returns how many consumed assertions are on record. It exists for the
// tests that assert the sweep really removes rows; nothing in the server calls
// it.
func (r *SAMLAssertions) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT count(*) FROM app.saml_assertion`).Scan(&count); err != nil {
		return 0, fmt.Errorf("identity: count consumed SAML assertions: %w", err)
	}
	return count, nil
}
