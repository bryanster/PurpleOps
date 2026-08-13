// Package engagement is the domain layer over the assessment workbook:
// engagements, their status lifecycle, pin management, and memberships.
//
// Storage lives in internal/store/engagement. This package owns the product
// rules that sit on top: pin assertion, status state machine, pin freeze,
// and creator-lead membership. Authorization is not decided here —
// api/openapi.yaml maps the HTTP surface to engagement.* actions and the
// middleware refuses before a handler is entered (M1-013).
package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Service is the engagement domain surface (M3-002). Construct with [New].
type Service struct {
	engagements *storengagement.Engagements
	attackpin   *attackpin.Service
	activity    *events.Log
	memberships MemberStore
	scenarios   *storengagement.Scenarios
	steps       *storengagement.Steps
	executions  *storengagement.Executions
	comments    *storengagement.Comments
	findings    *storengagement.Findings
	users       UserStore
}

type Deps struct {
	Engagements *storengagement.Engagements
	AttackPin   *attackpin.Service
	Activity    *events.Log // optional; nil skips durable activity rows
	Memberships MemberStore
	Scenarios   *storengagement.Scenarios
	Steps       *storengagement.Steps
	Executions  *storengagement.Executions
	Comments    *storengagement.Comments
	Findings    *storengagement.Findings
	Users       UserStore // optional; nil skips user validation on add
}

// New returns a Service over deps, or an error naming what is missing.
func New(deps Deps) (*Service, error) {
	if deps.Engagements == nil {
		return nil, errors.New("engagement: no engagements repository")
	}
	if deps.Memberships == nil {
		return nil, errors.New("engagement: no memberships repository")
	}
	// AttackPin is optional — only needed for create/update operations
	// that validate ATT&CK version pins.
	return &Service{
		engagements: deps.Engagements,
		attackpin:   deps.AttackPin,
		activity:    deps.Activity,
		memberships: deps.Memberships,
		users:       deps.Users,
		scenarios:   deps.Scenarios,
		steps:       deps.Steps,
		executions:  deps.Executions,
		comments:    deps.Comments,
		findings:    deps.Findings,
	}, nil
}

// CreateInput is the caller's half of creating an engagement.
type CreateInput struct {
	Name              string
	Client            string
	Description       string
	StartsOn          time.Time
	EndsOn            time.Time
	AttackVersion     string
	Mode              storengagement.EngagementMode
	AutoRevealOnStart bool
}

// validateCreate checks required fields and defaults.
func (in *CreateInput) validate() error {
	if in.Name == "" {
		return apierr.Validation(apierr.Field("name", "required"))
	}
	if in.Mode == "" {
		in.Mode = storengagement.EngagementModeStandard
	}
	if !in.Mode.Valid() {
		return apierr.Validation(apierr.Field("mode", "must be standard or blind"))
	}
	return nil
}

// Create writes a new engagement, adds the caller as lead member in the same
// transaction, and records activity. The attack_version must pass
// [attackpin.AssertPinned].
func (s *Service) Create(ctx context.Context, actor authn.Subject, in CreateInput) (storengagement.Engagement, error) {
	if err := in.validate(); err != nil {
		return storengagement.Engagement{}, err
	}
	if err := s.attackpin.AssertPinned(ctx, in.AttackVersion); err != nil {
		return storengagement.Engagement{}, err
	}

	newEng := storengagement.NewEngagement{
		Name:              in.Name,
		Client:            in.Client,
		Description:       in.Description,
		StartsOn:          in.StartsOn,
		EndsOn:            in.EndsOn,
		AttackVersion:     in.AttackVersion,
		Mode:              in.Mode,
		AutoRevealOnStart: in.AutoRevealOnStart,
		CreatedBy:         actor.UserID,
	}

	var engagementID string

	leadAfter := func(ctx context.Context, tx *sql.Tx) error {
		id := storengagement.AfterEntityID(ctx)
		engagementID = id
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_by, added_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, actor.UserID, string(authz.EngagementRoleLead), actor.UserID, time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("engagement: add lead member: %w", err)
		}
		return nil
	}

	var afterHooks []storengagement.After
	afterHooks = append(afterHooks, leadAfter)

	if s.activity != nil {
		activityAfter := func(ctx context.Context, tx *sql.Tx) error {
			return s.activity.Record(ctx, tx, events.Entry{
				EngagementID: engagementID,
				ActorID:      actor.UserID,
				Verb:         events.VerbEngagementCreated,
				ObjectType:   events.ObjectEngagement,
				ObjectID:     engagementID,
				Delta:        events.Delta(map[string]any{"name": in.Name}),
				At:           time.Now(),
			})
		}
		afterHooks = append(afterHooks, activityAfter)
	}

	return s.engagements.Create(ctx, newEng, afterHooks...)
}

// Get returns an engagement by id.
func (s *Service) Get(ctx context.Context, id string) (storengagement.Engagement, error) {
	return s.engagements.ByID(ctx, id)
}

// ListFilter narrows the engagement list from the HTTP layer.
type ListFilter struct {
	Status string
	After  string
	Limit  int
}

// List returns a page of engagements visible to the caller. Admins see every
// engagement; members see only the ones they belong to. The membership fence is
// applied here, from the caller's platform role and user id, so no handler can
// forget to filter.
func (s *Service) List(ctx context.Context, actor authn.Subject, filter ListFilter) ([]storengagement.Engagement, error) {
	storeFilter := storengagement.ListFilter{
		Status: filter.Status,
		After:  filter.After,
		Limit:  filter.Limit,
	}
	if actor.PlatformRole != authz.PlatformRoleAdmin {
		storeFilter.MemberID = actor.UserID
	}
	return s.engagements.List(ctx, storeFilter)
}

// UpdateInput is the caller's half of patching an engagement.
type UpdateInput struct {
	Name              *string
	Client            *string
	Description       *string
	StartsOn          *time.Time
	EndsOn            *time.Time
	AttackVersion     *string
	Mode              *storengagement.EngagementMode
	AutoRevealOnStart *bool
}

// Update patches an engagement after validating business rules:
// pin assertion on change, pin freeze check when steps exist.
func (s *Service) Update(ctx context.Context, actor authn.Subject, id string, in UpdateInput) (storengagement.Engagement, error) {
	current, err := s.engagements.ByID(ctx, id)
	if err != nil {
		return storengagement.Engagement{}, err
	}

	changes := storengagement.UpdateChanges{
		Name:              current.Name,
		Client:            current.Client,
		Description:       current.Description,
		StartsOn:          current.StartsOn,
		EndsOn:            current.EndsOn,
		AttackVersion:     current.AttackVersion,
		Mode:              current.Mode,
		AutoRevealOnStart: current.AutoRevealOnStart,
	}

	// Apply patches.
	if in.Name != nil {
		changes.Name = *in.Name
	}
	if in.Client != nil {
		changes.Client = *in.Client
	}
	if in.Description != nil {
		changes.Description = *in.Description
	}
	if in.StartsOn != nil {
		changes.StartsOn = *in.StartsOn
	}
	if in.EndsOn != nil {
		changes.EndsOn = *in.EndsOn
	}
	if in.Mode != nil {
		if !in.Mode.Valid() {
			return storengagement.Engagement{}, apierr.Validation(apierr.Field("mode", "must be standard or blind"))
		}
		changes.Mode = *in.Mode
	}
	if in.AutoRevealOnStart != nil {
		changes.AutoRevealOnStart = *in.AutoRevealOnStart
	}

	// Pin change: assert the new pin is valid, and check pin freeze.
	if in.AttackVersion != nil && *in.AttackVersion != current.AttackVersion {
		if err := s.attackpin.AssertPinned(ctx, *in.AttackVersion); err != nil {
			return storengagement.Engagement{}, err
		}
		count, err := s.engagements.CountSteps(ctx, id)
		if err != nil {
			return storengagement.Engagement{}, err
		}
		if count > 0 {
			return storengagement.Engagement{}, apierr.Conflict(
				"attack_version cannot be changed once steps exist on the engagement",
			)
		}
		changes.AttackVersion = *in.AttackVersion
	}

	var afterHooks []storengagement.After
	if s.activity != nil {
		activityAfter := func(ctx context.Context, tx *sql.Tx) error {
			return s.activity.Record(ctx, tx, events.Entry{
				EngagementID: id,
				ActorID:      actor.UserID,
				Verb:         events.VerbEngagementUpdated,
				ObjectType:   events.ObjectEngagement,
				ObjectID:     id,
				Delta:        patchDelta(in),
				At:           time.Now(),
			})
		}
		afterHooks = append(afterHooks, activityAfter)
	}

	return s.engagements.Update(ctx, id, changes, afterHooks...)
}

// patchDelta builds a redacted delta from the non-nil fields of an UpdateInput.
func patchDelta(in UpdateInput) map[string]any {
	d := map[string]any{}
	if in.Name != nil {
		d["name"] = *in.Name
	}
	if in.Client != nil {
		d["client"] = *in.Client
	}
	if in.Description != nil {
		d["description"] = *in.Description
	}
	if in.StartsOn != nil {
		d["starts_on"] = in.StartsOn.Format(time.RFC3339)
	}
	if in.EndsOn != nil {
		d["ends_on"] = in.EndsOn.Format(time.RFC3339)
	}
	if in.AttackVersion != nil {
		d["attack_version"] = *in.AttackVersion
	}
	if in.Mode != nil {
		d["mode"] = string(*in.Mode)
	}
	if in.AutoRevealOnStart != nil {
		d["auto_reveal_on_start"] = *in.AutoRevealOnStart
	}
	return events.Delta(d)
}

// validStatusTransitions is the engagement status state machine.
// draft → active → closed → archived, plus draft → closed.
var validStatusTransitions = map[storengagement.EngagementStatus][]storengagement.EngagementStatus{
	storengagement.EngagementStatusDraft: {
		storengagement.EngagementStatusActive,
		storengagement.EngagementStatusClosed,
	},
	storengagement.EngagementStatusActive: {
		storengagement.EngagementStatusClosed,
	},
	storengagement.EngagementStatusClosed: {
		storengagement.EngagementStatusArchived,
	},
}

// SetStatus validates the transition and applies it.
// Illegal transitions are 409 with a problem detail naming the current and
// requested states.
func (s *Service) SetStatus(ctx context.Context, actor authn.Subject, id string, newStatus storengagement.EngagementStatus) (storengagement.Engagement, error) {
	if !newStatus.Valid() {
		return storengagement.Engagement{}, apierr.Validation(apierr.Field("status", fmt.Sprintf("unknown status: %q", newStatus)))
	}

	current, err := s.engagements.ByID(ctx, id)
	if err != nil {
		return storengagement.Engagement{}, err
	}

	if current.Status == newStatus {
		// No-op: return the engagement as-is.
		return current, nil
	}

	allowed, ok := validStatusTransitions[current.Status]
	if !ok {
		return storengagement.Engagement{}, apierr.Conflict(
			fmt.Sprintf("cannot transition from %q to %q: %q is a terminal state with no outgoing transitions", current.Status, newStatus, current.Status),
		)
	}
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return storengagement.Engagement{}, apierr.Conflict(
			fmt.Sprintf("cannot transition from %q to %q", current.Status, newStatus),
		)
	}

	var afterHooks []storengagement.After
	if s.activity != nil {
		activityAfter := func(ctx context.Context, tx *sql.Tx) error {
			return s.activity.Record(ctx, tx, events.Entry{
				EngagementID: id,
				ActorID:      actor.UserID,
				Verb:         events.VerbEngagementStatusChanged,
				ObjectType:   events.ObjectEngagement,
				ObjectID:     id,
				Delta: events.Delta(map[string]any{
					"from": string(current.Status),
					"to":   string(newStatus),
				}),
				At: time.Now(),
			})
		}
		afterHooks = append(afterHooks, activityAfter)
	}

	return s.engagements.SetStatus(ctx, id, newStatus, afterHooks...)
}

// Delete removes an engagement and its entire workbook graph.
func (s *Service) Delete(ctx context.Context, actor authn.Subject, id string) error {
	// Read first to confirm existence and capture name for activity.
	e, err := s.engagements.ByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.engagements.Delete(ctx, id); err != nil {
		return err
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: id,
			ActorID:      actor.UserID,
			Verb:         events.VerbEngagementDeleted,
			ObjectType:   events.ObjectEngagement,
			ObjectID:     id,
			Delta:        events.Delta(map[string]any{"name": e.Name}),
			At:           time.Now(),
		}); err != nil {
			return fmt.Errorf("engagement: delete %q succeeded but activity log failed: %w", id, err)
		}
	}

	return nil
}
