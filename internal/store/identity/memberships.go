package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

const membershipColumns = `engagement_id, user_id, role, added_by, added_at`

const selectMembership = `SELECT ` + membershipColumns + ` FROM app.engagement_member `

const insertMembership = `INSERT INTO app.engagement_member
	(engagement_id, user_id, role, added_by, added_at)
	VALUES (?, ?, ?, ?, ?)`

// Memberships reads and writes who is in an engagement, and as what.
// Construct it with [NewMemberships].
//
// This is the second of the two role levels (PLAN.md §4). A row here says
// nothing about what somebody may do to the installation, and [User.PlatformRole]
// says nothing about what they may do inside an engagement; the policy engine
// (M1-012) is the only thing that reads both.
type Memberships struct {
	db DB
}

// NewMemberships returns a repository over db.
func NewMemberships(db DB) *Memberships { return &Memberships{db: db} }

// Add puts a user in an engagement and returns the membership as stored.
//
// Somebody who is already a member is [apierr.Conflict] rather than a silent
// role change: "add them as blue" applied to an existing red member is an
// instruction whose author did not know the current state, and quietly
// switching sides mid-engagement is exactly the kind of change that should be
// deliberate. Use [Memberships.SetRole] to change one.
func (r *Memberships) Add(ctx context.Context, in NewMembership) (Membership, error) {
	var created Membership
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireUser(ctx, tx, in.UserID); err != nil {
			return err
		}
		if in.AddedBy != "" {
			// "Who gave them access" is the first question an incident review
			// asks, so it cannot point at nobody in particular.
			if err := requireUser(ctx, tx, in.AddedBy); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, insertMembership,
			in.EngagementID, in.UserID, in.Role, nullString(in.AddedBy), now()); err != nil {
			return err
		}
		var err error
		created, err = scanMembership(tx.QueryRowContext(ctx,
			selectMembership+`WHERE engagement_id = ? AND user_id = ?`, in.EngagementID, in.UserID))
		return err
	})
	switch {
	case store.IsUniqueViolation(err):
		return Membership{}, apierr.Conflict("that user is already a member of this engagement")
	case err != nil:
		return Membership{}, fmt.Errorf("identity: add user %q to engagement %q: %w",
			in.UserID, in.EngagementID, err)
	}
	return created, nil
}

// Get returns one membership, or [apierr.NotFound]. It is the lookup behind
// every engagement-scoped authorization decision.
func (r *Memberships) Get(ctx context.Context, engagementID, userID string) (Membership, error) {
	m, err := scanMembership(r.db.Read().QueryRowContext(ctx,
		selectMembership+`WHERE engagement_id = ? AND user_id = ?`, engagementID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Membership{}, apierr.NotFound("engagement member", engagementID+"/"+userID)
	}
	if err != nil {
		return Membership{}, fmt.Errorf("identity: read membership %q/%q: %w",
			engagementID, userID, err)
	}
	return m, nil
}

// SetRole changes an existing member's role and returns the membership as
// stored, or reports [apierr.NotFound] for somebody who is not a member.
//
// added_by and added_at are not touched: they record how this person got in,
// which a later role change does not alter.
func (r *Memberships) SetRole(ctx context.Context, engagementID, userID string, role authz.EngagementRole) (Membership, error) {
	var updated Membership
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.engagement_member SET role = ? WHERE engagement_id = ? AND user_id = ?`,
			role, engagementID, userID)
		if err != nil {
			return err
		}
		if err := requireOneRow(result, "engagement member", engagementID+"/"+userID); err != nil {
			return err
		}
		updated, err = scanMembership(tx.QueryRowContext(ctx,
			selectMembership+`WHERE engagement_id = ? AND user_id = ?`, engagementID, userID))
		return err
	})
	if err != nil {
		return Membership{}, fmt.Errorf("identity: set role for %q in engagement %q: %w",
			userID, engagementID, err)
	}
	return updated, nil
}

// Remove takes a user out of an engagement, or reports [apierr.NotFound].
func (r *Memberships) Remove(ctx context.Context, engagementID, userID string) error {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM app.engagement_member WHERE engagement_id = ? AND user_id = ?`,
			engagementID, userID)
		if err != nil {
			return err
		}
		return requireOneRow(result, "engagement member", engagementID+"/"+userID)
	})
	if err != nil {
		return fmt.Errorf("identity: remove %q from engagement %q: %w", userID, engagementID, err)
	}
	return nil
}

// ListByUser returns every engagement a user belongs to, oldest membership
// first. This is the query the user_id index exists for.
func (r *Memberships) ListByUser(ctx context.Context, userID string) ([]Membership, error) {
	return r.list(ctx, selectMembership+`WHERE user_id = ? ORDER BY added_at, engagement_id`,
		fmt.Sprintf("list memberships for user %q", userID), userID)
}

// ListByEngagement returns everybody in an engagement, oldest membership first.
func (r *Memberships) ListByEngagement(ctx context.Context, engagementID string) ([]Membership, error) {
	return r.list(ctx, selectMembership+`WHERE engagement_id = ? ORDER BY added_at, user_id`,
		fmt.Sprintf("list members of engagement %q", engagementID), engagementID)
}

// list runs one of the two listings. They differ only in their WHERE clause, so
// the row loop is written once; what they had in common was the part worth
// getting wrong twice.
func (r *Memberships) list(ctx context.Context, query, what string, arg string) ([]Membership, error) {
	rows, err := r.db.Read().QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("identity: %s: %w", what, err)
	}
	defer rows.Close()

	var members []Membership
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, fmt.Errorf("identity: %s: %w", what, err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: %s: %w", what, err)
	}
	return members, nil
}

func scanMembership(row interface{ Scan(...any) error }) (Membership, error) {
	var (
		m       Membership
		addedBy sql.NullString
	)
	if err := row.Scan(&m.EngagementID, &m.UserID, &m.Role, &addedBy, &m.AddedAt); err != nil {
		return Membership{}, err
	}
	m.AddedBy = addedBy.String
	m.AddedAt = m.AddedAt.UTC()
	return m, nil
}
