package report

import (
	"context"
	"database/sql"
	"time"

	"github.com/bryanster/blacklight/internal/store"
)

// ReportShare is one shareable link to a published report version (M6-012).
type ReportShare struct {
	ID           string
	VersionID    string
	TokenHash    string
	PasswordHash *string
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	Label        *string
	MaxGrants    *int
}

// ReportShareGrant is one user's access to a shared version (M6-012).
type ReportShareGrant struct {
	ID             string
	ShareID        string
	UserID         *string
	InviteCodeHash *string
	ClaimedAt      *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

// NewShare describes the caller's half of creating a share.
type NewShare struct {
	VersionID    string
	TokenHash    string
	PasswordHash *string
	ExpiresAt    *time.Time
	CreatedBy    string
	Label        *string
	MaxGrants    *int
}

// NewGrant describes the caller's half of creating a grant.
type NewGrant struct {
	ShareID string
	UserID  *string
}

// ---------------------------------------------------------------------------
// Shares repository
// ---------------------------------------------------------------------------

// Shares reads and writes report shares. Construct with [NewShares].
type Shares struct {
	db DB
}

// NewShares returns a repository over db.
func NewShares(db DB) *Shares { return &Shares{db: db} }

// Insert creates a new share.
func (s *Shares) Insert(ctx context.Context, in NewShare) (ReportShare, error) {
	id := newID()
	nowTime := now()
	share := ReportShare{
		ID:           id,
		VersionID:    in.VersionID,
		TokenHash:    in.TokenHash,
		PasswordHash: in.PasswordHash,
		ExpiresAt:    in.ExpiresAt,
		CreatedBy:    in.CreatedBy,
		CreatedAt:    nowTime,
		Label:        in.Label,
		MaxGrants:    in.MaxGrants,
	}
	err := s.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.report_share (id, version_id, token_hash, password_hash,
				expires_at, created_by, created_at, label, max_grants)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.VersionID, in.TokenHash, in.PasswordHash,
			in.ExpiresAt, in.CreatedBy, nowTime, in.Label, in.MaxGrants,
		)
		return err
	})
	if err != nil {
		return ReportShare{}, err
	}
	return share, nil
}

// ByTokenHash returns the share with a matching token hash, or nil.
func (s *Shares) ByTokenHash(ctx context.Context, tokenHash string) (*ReportShare, error) {
	rows, err := s.db.Read().QueryContext(ctx, `
		SELECT id, version_id, token_hash, password_hash,
			expires_at, revoked_at, created_by, created_at,
			label, max_grants
		FROM app.report_share
		WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	share, err := scanShare(rows)
	if err != nil {
		return nil, err
	}
	return share, rows.Err()
}

// ByID returns the share with this identifier, or nil.
func (s *Shares) ByID(ctx context.Context, id string) (*ReportShare, error) {
	rows, err := s.db.Read().QueryContext(ctx, `
		SELECT id, version_id, token_hash, password_hash,
			expires_at, revoked_at, created_by, created_at,
			label, max_grants
		FROM app.report_share
		WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	share, err := scanShare(rows)
	if err != nil {
		return nil, err
	}
	return share, rows.Err()
}

// ListByVersion returns every share for a version, newest first.
func (s *Shares) ListByVersion(ctx context.Context, versionID string) ([]ReportShare, error) {
	rows, err := s.db.Read().QueryContext(ctx, `
		SELECT id, version_id, token_hash, password_hash,
			expires_at, revoked_at, created_by, created_at,
			label, max_grants
		FROM app.report_share
		WHERE version_id = ?
		ORDER BY created_at DESC`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shares []ReportShare
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		shares = append(shares, *sh)
	}
	return shares, rows.Err()
}

// Revoke marks a share as revoked.
func (s *Shares) Revoke(ctx context.Context, id string) error {
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE app.report_share SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
			now().UTC(), id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "report_share", id)
	})
}

// DeleteByVersion removes all shares for a version (cascade). Runs in tx.
func (s *Shares) DeleteByVersion(tx *sql.Tx, versionID string) error {
	_, err := tx.ExecContext(context.TODO(), `DELETE FROM app.report_share WHERE version_id = ?`, versionID)
	return err
}

// scanShare scans a share row.
func scanShare(scanner interface{ Scan(dest ...any) error }) (*ReportShare, error) {
	var sh ReportShare
	var passwordHash, label sql.NullString
	var expiresAt, revokedAt sql.NullTime
	var maxGrants sql.NullInt64
	if err := scanner.Scan(&sh.ID, &sh.VersionID, &sh.TokenHash,
		&passwordHash, &expiresAt, &revokedAt,
		&sh.CreatedBy, &sh.CreatedAt,
		&label, &maxGrants); err != nil {
		return nil, err
	}
	sh.PasswordHash = nullStrPtr(passwordHash)
	sh.ExpiresAt = nullTimePtr(expiresAt)
	sh.RevokedAt = nullTimePtr(revokedAt)
	sh.Label = nullStrPtr(label)
	if maxGrants.Valid {
		v := int(maxGrants.Int64)
		sh.MaxGrants = &v
	}
	return &sh, nil
}

// ---------------------------------------------------------------------------
// Grants repository
// ---------------------------------------------------------------------------

// Grants reads and writes share grants. Construct with [NewGrants].
type Grants struct {
	db DB
}

// NewGrants returns a repository over db.
func NewGrants(db DB) *Grants { return &Grants{db: db} }

// Insert creates a new grant.
func (g *Grants) Insert(ctx context.Context, in NewGrant) (ReportShareGrant, error) {
	id := newID()
	nowTime := now()
	grant := ReportShareGrant{
		ID:        id,
		ShareID:   in.ShareID,
		UserID:    in.UserID,
		CreatedAt: nowTime,
	}
	if in.UserID != nil {
		grant.ClaimedAt = &nowTime
	}
	err := g.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.report_share_grant (id, share_id, user_id, claimed_at, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			id, in.ShareID, in.UserID, grant.ClaimedAt, nowTime,
		)
		return err
	})
	if err != nil {
		return ReportShareGrant{}, err
	}
	return grant, nil
}

// ByShareAndUser returns a non-revoked grant for the given share+user, or nil.
func (g *Grants) ByShareAndUser(ctx context.Context, shareID, userID string) (*ReportShareGrant, error) {
	rows, err := g.db.Read().QueryContext(ctx, `
		SELECT id, share_id, user_id, invite_code_hash,
			claimed_at, revoked_at, created_at
		FROM app.report_share_grant
		WHERE share_id = ? AND user_id = ? AND revoked_at IS NULL`, shareID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	grant, err := scanGrant(rows)
	if err != nil {
		return nil, err
	}
	return grant, rows.Err()
}

// ListByShare returns every grant for a share.
func (g *Grants) ListByShare(ctx context.Context, shareID string) ([]ReportShareGrant, error) {
	rows, err := g.db.Read().QueryContext(ctx, `
		SELECT id, share_id, user_id, invite_code_hash,
			claimed_at, revoked_at, created_at
		FROM app.report_share_grant
		WHERE share_id = ?
		ORDER BY created_at`, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []ReportShareGrant
	for rows.Next() {
		gr, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, *gr)
	}
	return grants, rows.Err()
}

// Revoke marks a grant as revoked.
func (g *Grants) Revoke(ctx context.Context, id string) error {
	return g.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE app.report_share_grant SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
			now().UTC(), id)
		if err != nil {
			return err
		}
		return requireOneRow(result, "report_share_grant", id)
	})
}

// DeleteByShare removes all grants for a share (for cascade within tx).
func (g *Grants) DeleteByShare(tx *sql.Tx, shareID string) error {
	_, err := tx.ExecContext(context.TODO(), `DELETE FROM app.report_share_grant WHERE share_id = ?`, shareID)
	return err
}

// GrantCount returns the number of non-revoked grants for a share.
func (g *Grants) GrantCount(ctx context.Context, shareID string) (int, error) {
	var count int
	if err := g.db.Read().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM app.report_share_grant
		WHERE share_id = ? AND revoked_at IS NULL`, shareID,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// scanGrant scans a grant row.
func scanGrant(scanner interface{ Scan(dest ...any) error }) (*ReportShareGrant, error) {
	var gr ReportShareGrant
	var userIDStr, inviteCodeHash sql.NullString
	var claimedAt, revokedAt sql.NullTime
	if err := scanner.Scan(&gr.ID, &gr.ShareID, &userIDStr,
		&inviteCodeHash, &claimedAt, &revokedAt, &gr.CreatedAt); err != nil {
		return nil, err
	}
	gr.UserID = nullStrPtr(userIDStr)
	gr.InviteCodeHash = nullStrPtr(inviteCodeHash)
	gr.ClaimedAt = nullTimePtr(claimedAt)
	gr.RevokedAt = nullTimePtr(revokedAt)
	return &gr, nil
}

// nullStrPtr returns a *string from an sql.NullString.
func nullStrPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// nullTimePtr returns a *time.Time from an sql.NullTime.
func nullTimePtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

// Ensure unused import is referenced.
var _ store.Store
