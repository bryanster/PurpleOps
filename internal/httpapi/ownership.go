package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// Memberships is the part of the identity store [ownership] needs to answer
// Seat. [*identity.Memberships] satisfies it.
type Memberships interface {
	Get(ctx context.Context, engagementID, userID string) (identity.Membership, error)
}

// ownership resolves engagement-scoped authorization facts from the store
// (M7-011). It replaces the M1 stub that treated any path string as a real
// engagement: [Facts] walks the named resource to its owning engagement, loads
// that engagement's blind mode, and reports the reveal state of step-shaped
// resources. A missing row is [apierr.NotFound], indistinguishable from a
// concealed denial — which is the point.
type ownership struct {
	members     Memberships
	engagements *storengagement.Engagements
	scenarios   *storengagement.Scenarios
	steps       *storengagement.Steps
	executions  *storengagement.Executions
	evidence    *storengagement.EvidenceRepo
	findings    *storengagement.Findings
	comments    *storengagement.Comments
	reports     *storereport.Reports
	versions    *storereport.Versions
	templates   *storereport.Templates
	shares      *storereport.Shares
}

// NewOwnership returns an [Ownership] that loads facts from db. It is the
// production loader: [newServer] installs it when Deps.Ownership is nil.
func NewOwnership(db store.Store) Ownership {
	if db == nil {
		panic("httpapi: NewOwnership called with nil store")
	}
	return &ownership{
		members:     identity.NewMemberships(db),
		engagements: storengagement.NewEngagements(db),
		scenarios:   storengagement.NewScenarios(db),
		steps:       storengagement.NewSteps(db),
		executions:  storengagement.NewExecutions(db),
		evidence:    storengagement.NewEvidenceRepo(db),
		findings:    storengagement.NewFindings(db),
		comments:    storengagement.NewComments(db),
		reports:     storereport.NewReports(db),
		versions:    storereport.NewVersions(db),
		templates:   storereport.NewTemplates(db),
		shares:      storereport.NewShares(db),
	}
}

// Facts returns the ownership state of one resource. It walks the named row to
// its owning engagement rather than trusting the path label, and refuses when a
// path engagement is named and does not match the loaded owner — so a child id
// from another engagement reads as not-found, exactly like a missing row.
func (o *ownership) Facts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	facts, err := o.resolve(ctx, ref)
	if err != nil {
		// The store packages disagree about whether a missing row is
		// sql.ErrNoRows or apierr.ErrNotFound. Facts answers one way, so a
		// concealed denial and a missing id are the same 404.
		if errors.Is(err, sql.ErrNoRows) {
			return ResourceFacts{}, apierr.NotFound(string(ref.Type), ref.ID)
		}
		return ResourceFacts{}, err
	}
	if ref.EngagementID != "" && ref.EngagementID != facts.EngagementID {
		return ResourceFacts{}, apierr.NotFound(string(ref.Type), ref.ID)
	}
	return facts, nil
}

func (o *ownership) resolve(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	switch ref.Type {
	case authz.ResourceEngagement:
		return o.engagementFacts(ctx, ref.ID)
	case authz.ResourceMember:
		return o.engagementFacts(ctx, ref.EngagementID)
	case authz.ResourceScenario:
		return o.scenarioFacts(ctx, ref)
	case authz.ResourceExecution:
		return o.executionFacts(ctx, ref)
	case authz.ResourceComment:
		return o.commentFacts(ctx, ref)
	case authz.ResourceEvidence:
		return o.evidenceFacts(ctx, ref)
	case authz.ResourceFinding:
		return o.findingFacts(ctx, ref)
	case authz.ResourceReport:
		return o.reportFacts(ctx, ref)
	default:
		return ResourceFacts{}, apierr.Internal(fmt.Errorf("httpapi: no ownership walk for resource type %q", ref.Type))
	}
}

func (o *ownership) engagementFacts(ctx context.Context, id string) (ResourceFacts, error) {
	eng, err := o.engagements.ByID(ctx, id)
	if err != nil {
		return ResourceFacts{}, err
	}
	return ResourceFacts{
		EngagementID: eng.ID,
		Blind:        eng.Mode == storengagement.EngagementModeBlind,
		Revealed:     true,
	}, nil
}

func (o *ownership) scenarioFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.ID == "" {
		return o.engagementFacts(ctx, ref.EngagementID)
	}
	sc, err := o.scenarios.ByID(ctx, ref.ID)
	if err != nil {
		return ResourceFacts{}, err
	}
	return o.engagementFacts(ctx, sc.EngagementID)
}

func (o *ownership) executionFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.ID == "" {
		return o.engagementFacts(ctx, ref.EngagementID)
	}
	return o.executionByIDFacts(ctx, ref.ID)
}

func (o *ownership) commentFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.ID == "" {
		return o.engagementFacts(ctx, ref.EngagementID)
	}
	c, err := o.comments.ByID(ctx, ref.ID)
	if err != nil {
		return ResourceFacts{}, err
	}
	return o.executionByIDFacts(ctx, c.ExecutionID)
}

func (o *ownership) evidenceFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.Kind == "execution" {
		return o.executionByIDFacts(ctx, ref.ID)
	}
	ev, err := o.evidence.ByID(ctx, ref.ID)
	if err != nil {
		return ResourceFacts{}, err
	}
	if ev.ExecutionID != "" {
		return o.executionByIDFacts(ctx, ev.ExecutionID)
	}
	c, err := o.comments.ByID(ctx, ev.CommentID)
	if err != nil {
		return ResourceFacts{}, err
	}
	return o.executionByIDFacts(ctx, c.ExecutionID)
}

func (o *ownership) findingFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.ID == "" {
		return o.engagementFacts(ctx, ref.EngagementID)
	}
	f, err := o.findings.ByID(ctx, ref.ID)
	if err != nil {
		return ResourceFacts{}, err
	}
	return o.engagementFacts(ctx, f.EngagementID)
}

func (o *ownership) reportFacts(ctx context.Context, ref ResourceRef) (ResourceFacts, error) {
	switch ref.Kind {
	case "", "report":
		if ref.ID == "" {
			return o.engagementFacts(ctx, ref.EngagementID)
		}
		r, err := o.reports.ByID(ctx, ref.ID)
		if err != nil {
			return ResourceFacts{}, err
		}
		return o.engagementFacts(ctx, r.EngagementID)
	case "template":
		t, err := o.templates.ByID(ctx, ref.ID)
		if err != nil {
			return ResourceFacts{}, err
		}
		return o.engagementFacts(ctx, t.EngagementID)
	case "version":
		v, err := o.versions.ByID(ctx, ref.ID)
		if err != nil {
			return ResourceFacts{}, err
		}
		r, err := o.reports.ByID(ctx, v.ReportID)
		if err != nil {
			return ResourceFacts{}, err
		}
		return o.engagementFacts(ctx, r.EngagementID)
	case "share":
		s, err := o.shares.ByID(ctx, ref.ID)
		if err != nil {
			return ResourceFacts{}, err
		}
		if s == nil {
			return ResourceFacts{}, apierr.NotFound("share", ref.ID)
		}
		v, err := o.versions.ByID(ctx, s.VersionID)
		if err != nil {
			return ResourceFacts{}, err
		}
		r, err := o.reports.ByID(ctx, v.ReportID)
		if err != nil {
			return ResourceFacts{}, err
		}
		return o.engagementFacts(ctx, r.EngagementID)
	default:
		return ResourceFacts{}, apierr.Internal(fmt.Errorf("httpapi: unknown report kind %q", ref.Kind))
	}
}

func (o *ownership) executionByIDFacts(ctx context.Context, executionID string) (ResourceFacts, error) {
	exec, err := o.executions.ByID(ctx, executionID)
	if err != nil {
		return ResourceFacts{}, err
	}
	return o.stepFacts(ctx, exec.StepID)
}

func (o *ownership) stepFacts(ctx context.Context, stepID string) (ResourceFacts, error) {
	step, err := o.steps.ByID(ctx, stepID)
	if err != nil {
		return ResourceFacts{}, err
	}
	sc, err := o.scenarios.ByID(ctx, step.ScenarioID)
	if err != nil {
		return ResourceFacts{}, err
	}
	facts, err := o.engagementFacts(ctx, sc.EngagementID)
	if err != nil {
		return ResourceFacts{}, err
	}
	facts.Revealed = step.RevealedAt != nil
	return facts, nil
}

// Seat returns the caller's role in the engagement, or false when they are
// not a member.
func (o *ownership) Seat(ctx context.Context, engagementID, userID string) (authz.EngagementRole, bool, error) {
	m, err := o.members.Get(ctx, engagementID, userID)
	if errors.Is(err, apierr.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Role, true, nil
}

// enforceEvidenceSide checks that the caller's seat permits uploading on the
// given side. Admin without a seat may write either. Red seat -> red only,
// blue seat -> blue only, lead -> either.
func enforceEvidenceSide(ctx context.Context, o Ownership, platformRole authz.PlatformRole, userID, engagementID string, side storengagement.EvidenceSide) error {
	// Platform admin without a seat may write either side for support.
	if platformRole == authz.PlatformRoleAdmin {
		seat, member, err := o.Seat(ctx, engagementID, userID)
		if err != nil {
			return err
		}
		if !member {
			return nil // admin without seat: allowed either side
		}
		switch seat {
		case authz.EngagementRoleLead:
			return nil
		case authz.EngagementRoleRed:
			if side != storengagement.EvidenceSideRed {
				return apierr.Forbidden("red seat may only upload side=red evidence")
			}
		case authz.EngagementRoleBlue:
			if side != storengagement.EvidenceSideBlue {
				return apierr.Forbidden("blue seat may only upload side=blue evidence")
			}
		}
		return nil
	}

	// Non-admin: enforce seat.
	seat, member, err := o.Seat(ctx, engagementID, userID)
	if err != nil {
		return err
	}
	if !member {
		return apierr.Forbidden("not a member of this engagement")
	}
	switch seat {
	case authz.EngagementRoleLead:
		return nil
	case authz.EngagementRoleRed:
		if side != storengagement.EvidenceSideRed {
			return apierr.Forbidden("red seat may only upload side=red evidence")
		}
	case authz.EngagementRoleBlue:
		if side != storengagement.EvidenceSideBlue {
			return apierr.Forbidden("blue seat may only upload side=blue evidence")
		}
	default:
		return apierr.Forbidden("observer cannot upload evidence")
	}
	return nil
}

// canDeleteEvidence returns nil when the caller is the uploader, the engagement
// lead, or a platform admin. It uses the authz constants for role literals.
func canDeleteEvidence(ctx context.Context, o Ownership, platformRole authz.PlatformRole, userID, engagementID, uploadedBy string) error {
	if userID == uploadedBy {
		return nil
	}
	if platformRole == authz.PlatformRoleAdmin {
		return nil
	}
	if engagementID == "" {
		return apierr.Forbidden("only the uploader, lead, or admin may delete evidence")
	}
	seat, member, err := o.Seat(ctx, engagementID, userID)
	if err != nil {
		return err
	}
	if member && seat == authz.EngagementRoleLead {
		return nil
	}
	return apierr.Forbidden("only the uploader, lead, or admin may delete evidence")
}

// canEditComment checks the finer-grained edit permission beyond what the
// authz middleware enforces: only the author, engagement lead, or platform
// admin may edit. The middleware already verified comment.write, which all
// members hold — this narrows it further.
func canEditComment(ctx context.Context, engagementID string) error {
	authzCtx, ok := authorizationFrom(ctx)
	if !ok {
		return apierr.Forbidden("cannot verify authorization")
	}
	subject := authzCtx.Subject
	if subject.PlatformRole == authz.PlatformRoleAdmin {
		return nil
	}
	seat, _ := subject.MembershipIn(engagementID)
	if seat == authz.EngagementRoleLead {
		return nil
	}
	return apierr.Forbidden("only the author, lead, or admin may edit a comment")
}
