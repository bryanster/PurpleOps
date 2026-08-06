package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

const engagementColumns = `id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at`

const selectEngagement = `SELECT ` + engagementColumns + ` FROM app.engagement `

const insertEngagement = `INSERT INTO app.engagement
	(id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Engagements reads and writes assessments. Construct it with [NewEngagements].
type Engagements struct {
	db DB
}

// NewEngagements returns a repository over db.
func NewEngagements(db DB) *Engagements { return &Engagements{db: db} }

// Create writes a new engagement and returns it as stored.
// ID, status, created_at and updated_at are assigned by the store.
func (r *Engagements) Create(ctx context.Context, in NewEngagement, after ...After) (Engagement, error) {
	var result Engagement
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("engagement: generate id: %w", err)
		}
		ts := now()
		result = Engagement{
			ID:                id,
			Name:              in.Name,
			Client:            in.Client,
			Description:       in.Description,
			Status:            EngagementStatusDraft,
			StartsOn:          toStorage(in.StartsOn),
			EndsOn:            toStorage(in.EndsOn),
			AttackVersion:     in.AttackVersion,
			Mode:              in.Mode,
			AutoRevealOnStart: in.AutoRevealOnStart,
			CreatedBy:         in.CreatedBy,
			CreatedAt:         ts,
			UpdatedAt:         ts,
		}
		_, err = tx.ExecContext(ctx, insertEngagement,
			result.ID,
			result.Name,
			result.Client,
			result.Description,
			string(result.Status),
			result.StartsOn,
			result.EndsOn,
			result.AttackVersion,
			string(result.Mode),
			result.AutoRevealOnStart,
			result.CreatedBy,
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("engagement: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Engagement{}, err
	}
	return result, nil
}

// ByID returns the engagement with this identifier, or [apierr.NotFound].
func (r *Engagements) ByID(ctx context.Context, id string) (Engagement, error) {
	e, err := scanEngagement(r.db.Read().QueryRowContext(ctx, selectEngagement+`WHERE id = ?`, id))
	if err != nil {
		return Engagement{}, fmt.Errorf("engagement: read %q: %w", id, err)
	}
	return e, nil
}

// CountByAttackVersion returns how many engagements pin the given ATT&CK version.
// This is the attackpin.References implementation backing DeleteVersion refusal.
func (r *Engagements) CountByAttackVersion(ctx context.Context, version string) (int64, error) {
	var count int64
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.engagement WHERE attack_version = ?`, version,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("engagement: count by attack version: %w", err)
	}
	return count, nil
}

// scanEngagement reads one row of engagementColumns. It takes the interface both
// *sql.Row and *sql.Rows satisfy.
func scanEngagement(row interface{ Scan(...any) error }) (Engagement, error) {
	var e Engagement
	if err := row.Scan(
		&e.ID, &e.Name, &e.Client, &e.Description,
		&e.Status, &e.StartsOn, &e.EndsOn, &e.AttackVersion,
		&e.Mode, &e.AutoRevealOnStart, &e.CreatedBy,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Engagement{}, err
		}
		return Engagement{}, err
	}
	e.CreatedAt = e.CreatedAt.UTC()
	e.UpdatedAt = e.UpdatedAt.UTC()
	return e, nil
}
