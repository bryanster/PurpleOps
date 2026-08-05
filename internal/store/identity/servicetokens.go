package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// The two statements below are column lists, not credentials. gosec's G101
// matches on the identifier — anything named "…token…" holding a long string —
// and there is no spelling of "the columns of app.service_token" that avoids
// the word. The suppression is here rather than on a renamed constant because
// the name is right and the finding is not.
//
//nolint:gosec // G101: a SELECT column list, not a hardcoded credential.
const serviceTokenColumns = `id, name, prefix, token_hash, owner_user_id, created_by,
	scopes, engagement_id, created_at, expires_at, last_used_at, revoked_at, revoked_by`

const selectServiceToken = `SELECT ` + serviceTokenColumns + ` FROM app.service_token `

//nolint:gosec // G101: an INSERT statement, not a hardcoded credential.
const insertServiceToken = `INSERT INTO app.service_token
	(id, name, prefix, token_hash, owner_user_id, created_by, scopes,
	 engagement_id, created_at, expires_at, last_used_at, revoked_at, revoked_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL)`

// scopeSeparator joins the scope list into its column. It is a space because
// that is how OAuth 2.0 spells a scope list (RFC 6749 §3.3) and because no
// scope in internal/authz contains one — see 0008_service_token.sql for why the
// column is a list at all.
const scopeSeparator = " "

// ServiceTokens reads and writes the bearer credentials somebody automates with
// (M1-011). Construct it with [NewServiceTokens].
//
// It stores and returns; it does not decide. Whether a token is still usable
// depends on expiry, revocation and the live state of the account that owns it,
// and that judgement is one place — internal/authn/servicetoken — for the same
// reason [Sessions] keeps out of it.
type ServiceTokens struct {
	db DB
}

// NewServiceTokens returns a repository over db.
func NewServiceTokens(db DB) *ServiceTokens { return &ServiceTokens{db: db} }

// Create stores a new token and returns it as stored.
//
// Both accounts it names are checked inside the write transaction, which is
// where 0003_user_updatable moved the rule the foreign keys used to enforce.
//
// after runs inside the same transaction after the insert, so a side effect
// that fails (an activity row, today) rolls the token back with it (M1-015).
func (r *ServiceTokens) Create(ctx context.Context, in NewServiceToken, after ...After) (ServiceToken, error) {
	id, err := newID()
	if err != nil {
		return ServiceToken{}, err
	}

	var created ServiceToken
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, in.OwnerUserID); err != nil {
			return err
		}
		if in.CreatedBy != in.OwnerUserID {
			if err := requireUser(ctx, tx, in.CreatedBy); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, insertServiceToken,
			id, in.Name, in.Prefix, in.TokenHash, in.OwnerUserID, in.CreatedBy,
			joinScopes(in.Scopes), nullString(in.EngagementID), now(),
			toStorage(in.ExpiresAt)); err != nil {
			return err
		}
		var err error
		created, err = scanServiceToken(tx.QueryRowContext(ctx, selectServiceToken+`WHERE id = ?`, id))
		if err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, created.ID), tx, after)
	})
	switch {
	case store.IsUniqueViolation(err):
		// Two tokens cannot share a prefix. With a random one this never
		// happens, so it means the caller reused a value — worth failing on
		// rather than issuing a credential whose lookup is ambiguous.
		return ServiceToken{}, apierr.Conflict("that token prefix is already in use")
	case err != nil:
		return ServiceToken{}, fmt.Errorf("identity: create service token for user %q: %w", in.OwnerUserID, err)
	}
	return created, nil
}

// ByPrefix returns the token with this prefix, or [apierr.NotFound]. It is the
// lookup on every token-authenticated request, and it is a lookup rather than a
// scan because the prefix is unique and indexed — comparing a presented secret
// against every stored hash is the thing the prefix exists to avoid.
func (r *ServiceTokens) ByPrefix(ctx context.Context, prefix string) (ServiceToken, error) {
	t, err := scanServiceToken(r.db.Read().QueryRowContext(ctx,
		selectServiceToken+`WHERE prefix = ?`, prefix))
	if errors.Is(err, sql.ErrNoRows) {
		// The prefix is not echoed. It is the public half of a credential
		// somebody presented, and a not-found carrying it would put every
		// guessed prefix into the log.
		return ServiceToken{}, apierr.NotFound("service token", "(prefix)")
	}
	if err != nil {
		return ServiceToken{}, fmt.Errorf("identity: read service token by prefix: %w", err)
	}
	return t, nil
}

// ListByOwner returns every token row a user holds, newest first, including
// expired and revoked ones — an owner deciding whether to rotate needs to see
// what ended and when, the same as they do for sessions.
//
// It is one query for two endpoints: the owner's own listing passes the caller,
// and M1-018's administrative listing passes the account named in its path.
// Nothing here knows which, deliberately — an administrator who could be shown a
// different set from the owner is an administrator working from a second
// account of what exists.
func (r *ServiceTokens) ListByOwner(ctx context.Context, ownerUserID string) ([]ServiceToken, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectServiceToken+`WHERE owner_user_id = ? ORDER BY id DESC`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("identity: list service tokens for user %q: %w", ownerUserID, err)
	}
	defer rows.Close()

	var tokens []ServiceToken
	for rows.Next() {
		t, err := scanServiceToken(rows)
		if err != nil {
			return nil, fmt.Errorf("identity: list service tokens for user %q: %w", ownerUserID, err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list service tokens for user %q: %w", ownerUserID, err)
	}
	return tokens, nil
}

// Revoke ends one token belonging to ownerUserID, and records revokedBy as who
// ended it.
//
// The owner is part of the statement rather than checked beforehand, which is
// what makes a token that is not theirs indistinguishable from one that does not
// exist: both match no row, and both are [apierr.NotFound]. A check that read the
// row first and compared would answer the two differently, and the difference is
// a way to find out which identifiers are real. That property is what M1-018's
// administrative endpoint reuses rather than reimplements — it passes the account
// named in its path as the owner, so a token identifier belonging to a *different*
// account is a 404 there too.
//
// revokedBy is the owner on the owner's own revocation and an administrator on
// M1-018's. It is written in the same statement as revoked_at so the two cannot
// disagree.
//
// Revoking a token that has already been revoked keeps the original timestamp and
// the original revoker — the first revocation is when access actually stopped, and
// whoever arrived second did not stop anything — and is not an error, because the
// caller's intent is satisfied either way.
func (r *ServiceTokens) Revoke(ctx context.Context, id, ownerUserID, revokedBy string, at time.Time,
	after ...After) (ServiceToken, error) {
	var revoked ServiceToken
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, revokedBy); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx,
			`UPDATE app.service_token SET revoked_at = ?, revoked_by = ?
			 WHERE id = ? AND owner_user_id = ? AND revoked_at IS NULL`,
			toStorage(at), revokedBy, id, ownerUserID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("counting the affected rows: %w", err)
		}
		revoked, err = scanServiceToken(tx.QueryRowContext(ctx,
			selectServiceToken+`WHERE id = ? AND owner_user_id = ?`, id, ownerUserID))
		if errors.Is(err, sql.ErrNoRows) {
			return apierr.NotFound("service token", id)
		}
		if err != nil {
			return err
		}
		if affected == 1 {
			return runAfter(WithAfterEntity(ctx, revoked.ID), tx, after)
		}
		return nil
	})
	if err != nil {
		return ServiceToken{}, fmt.Errorf("identity: revoke service token %q: %w", id, err)
	}
	return revoked, nil
}

// SetLastUsedAt records that a token was used.
//
// Not keyed on revoked_at IS NULL: this runs after the token has already been
// judged usable, and a token revoked in the microseconds between the two is one
// whose last use is still worth recording.
func (r *ServiceTokens) SetLastUsedAt(ctx context.Context, id string, at time.Time) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.service_token SET last_used_at = ? WHERE id = ?`, toStorage(at), id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "service token", id)
	})
	if err != nil {
		return fmt.Errorf("identity: touch service token %q: %w", id, err)
	}
	return nil
}

// DeleteExpired removes tokens that expired before the given time, and reports
// how many. Revoked tokens are removed on the same terms rather than
// immediately, so that "this token was revoked" stays answerable for as long as
// an expired one does — the same retention rule [Sessions.DeleteExpired] keeps.
func (r *ServiceTokens) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	var deleted int64
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM app.service_token WHERE expires_at < ?`, toStorage(before))
		if err != nil {
			return err
		}
		deleted, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("identity: delete service tokens expired before %s: %w",
			before.UTC().Format(time.RFC3339), err)
	}
	return deleted, nil
}

// joinScopes renders the scope list into its column. A nil or empty list is the
// empty string, which reads back as no scopes — a token that may do nothing,
// which is a token the creation endpoint refuses rather than one this package
// has an opinion about.
func joinScopes(scopes []authz.TokenScope) string {
	words := make([]string, len(scopes))
	for i, scope := range scopes {
		words[i] = string(scope)
	}
	return strings.Join(words, scopeSeparator)
}

// splitScopes reads the column back. strings.Fields rather than Split so that
// a value with repeated or surrounding spaces — hand-edited in a SQL console,
// most likely — produces the scopes it names rather than empty entries between
// them.
func splitScopes(column string) []authz.TokenScope {
	words := strings.Fields(column)
	if len(words) == 0 {
		return nil
	}
	scopes := make([]authz.TokenScope, len(words))
	for i, word := range words {
		scopes[i] = authz.TokenScope(word)
	}
	return scopes
}

func scanServiceToken(row interface{ Scan(...any) error }) (ServiceToken, error) {
	var (
		t          ServiceToken
		scopes     string
		engagement sql.NullString
		lastUsed   sql.NullTime
		revoked    sql.NullTime
		revokedBy  sql.NullString
	)
	if err := row.Scan(&t.ID, &t.Name, &t.Prefix, &t.TokenHash, &t.OwnerUserID, &t.CreatedBy,
		&scopes, &engagement, &t.CreatedAt, &t.ExpiresAt, &lastUsed, &revoked, &revokedBy); err != nil {
		return ServiceToken{}, err
	}
	t.Scopes = splitScopes(scopes)
	t.EngagementID = engagement.String
	t.CreatedAt = t.CreatedAt.UTC()
	t.ExpiresAt = t.ExpiresAt.UTC()
	t.LastUsedAt = fromNullTime(lastUsed)
	t.RevokedAt = fromNullTime(revoked)
	t.RevokedBy = revokedBy.String
	return t, nil
}
