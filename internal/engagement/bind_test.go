package engagement_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// bindFixture holds two engagements (A and B) with parallel workbook chains, so
// a cross-engagement child id can be driven through a domain method whose path
// engagement is the other one (M7-012).
type bindFixture struct {
	svc        *engagement.Service
	engA       storengagement.Engagement
	engB       storengagement.Engagement
	scA        storengagement.Scenario
	scB        storengagement.Scenario
	stepA      storengagement.Step
	stepB      storengagement.Step
	execA      storengagement.Execution
	execB      storengagement.Execution
	commentA   storengagement.Comment
	commentB   storengagement.Comment
	steps      *storengagement.Steps
	executions *storengagement.Executions
	scenarios  *storengagement.Scenarios
}

func newBindFixture(t *testing.T) *bindFixture {
	t.Helper()
	ctx := context.Background()

	db := storetest.Migrated(t)
	engagements := storengagement.NewEngagements(db)
	scenarios := storengagement.NewScenarios(db)
	steps := storengagement.NewSteps(db)
	executions := storengagement.NewExecutions(db)
	comments := storengagement.NewComments(db)
	findings := storengagement.NewFindings(db)
	memberships := identity.NewMemberships(db)

	svc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		Memberships: memberships,
		Scenarios:   scenarios,
		Steps:       steps,
		Executions:  executions,
		Comments:    comments,
		Findings:    findings,
	})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}

	newEngagement := func(name string) storengagement.Engagement {
		eng, err := engagements.Create(ctx, storengagement.NewEngagement{
			Name:          name,
			AttackVersion: "15.1",
			Mode:          storengagement.EngagementModeStandard,
			CreatedBy:     "0192f1a0-0000-7000-8000-000000000000",
		})
		if err != nil {
			t.Fatalf("create engagement %s: %v", name, err)
		}
		return eng
	}

	engA := newEngagement("A")
	engB := newEngagement("B")

	newScenario := func(engID, name string, ordinal int) storengagement.Scenario {
		sc, err := scenarios.Create(ctx, storengagement.NewScenario{
			EngagementID: engID,
			Ordinal:      ordinal,
			Name:         name,
			Source:       storengagement.ScenarioSourceManual,
		})
		if err != nil {
			t.Fatalf("create scenario %s: %v", name, err)
		}
		return sc
	}

	scA := newScenario(engA.ID, "scenario-A", 1)
	scB := newScenario(engB.ID, "scenario-B", 1)

	newStep := func(scID, name string, ordinal int) (storengagement.Step, storengagement.Execution) {
		step, exec, err := steps.CreateWithExecution(ctx, storengagement.NewStep{
			ScenarioID:    scID,
			Ordinal:       ordinal,
			Name:          name,
			TechniqueID:   "T1003",
			AttackVersion: "15.1",
		})
		if err != nil {
			t.Fatalf("create step %s: %v", name, err)
		}
		return step, exec
	}

	stepA, execA := newStep(scA.ID, "step-A", 1)
	stepB, execB := newStep(scB.ID, "step-B", 1)

	newComment := func(execID, name string) storengagement.Comment {
		c, err := comments.Create(ctx, storengagement.NewComment{
			ExecutionID: execID,
			AuthorID:    "0192f1a0-0000-7000-8000-000000000000",
			Body:        name,
		})
		if err != nil {
			t.Fatalf("create comment %s: %v", name, err)
		}
		return c
	}

	commentA := newComment(execA.ID, "comment-A")
	commentB := newComment(execB.ID, "comment-B")

	return &bindFixture{
		svc:        svc,
		engA:       engA,
		engB:       engB,
		scA:        scA,
		scB:        scB,
		stepA:      stepA,
		stepB:      stepB,
		execA:      execA,
		execB:      execB,
		commentA:   commentA,
		commentB:   commentB,
		steps:      steps,
		executions: executions,
		scenarios:  scenarios,
	}
}

func TestListStepsMismatchIsNotFound(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()

	if _, err := fx.svc.ListSteps(ctx, fx.engB.ID, fx.scA.ID, blind.Scope{}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ListSteps(B, A.scenario) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.ListSteps(ctx, fx.engA.ID, fx.scA.ID, blind.Scope{}); err != nil {
		t.Fatalf("ListSteps(A, A.scenario) error = %v, want nil", err)
	}
}
func actorForBind() authn.Subject {
	return authn.Subject{UserID: "0192f1a0-0000-7000-8000-000000000000", Email: "test@example.com", DisplayName: "Test"}
}

func TestScenarioBindingMismatchIsNotFound(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()

	if _, err := fx.svc.GetScenarioInEngagement(ctx, fx.engB.ID, fx.scA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("GetScenarioInEngagement(B, A.scenario) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.GetScenarioInEngagement(ctx, fx.engA.ID, fx.scA.ID); err != nil {
		t.Fatalf("GetScenarioInEngagement(A, A.scenario) error = %v, want nil", err)
	}
}

func TestScenarioWriteBindingMismatchDoesNotMutate(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()
	actor := actorForBind()

	name := "patched"
	if _, err := fx.svc.PatchScenario(ctx, actor, fx.engB.ID, fx.scA.ID, engagement.PatchScenarioInput{Name: &name}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("PatchScenario(B, A.scenario) error = %v, want NotFound", err)
	}
	if err := fx.svc.DeleteScenario(ctx, actor, fx.engB.ID, fx.scA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("DeleteScenario(B, A.scenario) error = %v, want NotFound", err)
	}
	// The scenario survives: it belonged to A, and B was the path engagement.
	sc, err := fx.scenarios.ByID(ctx, fx.scA.ID)
	if err != nil {
		t.Fatalf("scenario A should still exist: %v", err)
	}
	if sc.Name != fx.scA.Name {
		t.Errorf("scenario A name changed to %q, want %q", sc.Name, fx.scA.Name)
	}
}

func TestStepBindingMismatchIsNotFound(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()

	if _, err := fx.svc.GetStepInEngagement(ctx, fx.engB.ID, fx.stepA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("GetStepInEngagement(B, A.step) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.GetStepInEngagement(ctx, fx.engA.ID, fx.stepA.ID); err != nil {
		t.Fatalf("GetStepInEngagement(A, A.step) error = %v, want nil", err)
	}
}

func TestCreateStepMismatchDoesNotCreate(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()
	actor := actorForBind()

	before := stepCount(t, fx, fx.scA.ID)

	_, err := fx.svc.CreateStep(ctx, actor, engagement.CreateStepInput{
		EngagementID: fx.engB.ID, // wrong path engagement for scenario A
		ScenarioID:   fx.scA.ID,
		Name:         "intruder",
	})
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("CreateStep(B, A.scenario) error = %v, want NotFound", err)
	}

	after := stepCount(t, fx, fx.scA.ID)
	if after != before {
		t.Errorf("CreateStep mismatch created a step: %d -> %d", before, after)
	}
}

func TestStepWriteBindingMismatchDoesNotMutate(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()
	actor := actorForBind()

	name := "patched"
	if _, err := fx.svc.PatchStep(ctx, actor, fx.engB.ID, fx.stepA.ID, engagement.PatchStepInput{Name: &name}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("PatchStep(B, A.step) error = %v, want NotFound", err)
	}
	if err := fx.svc.DeleteStep(ctx, actor, fx.engB.ID, fx.stepA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("DeleteStep(B, A.step) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.RevealStep(ctx, actor, fx.engB.ID, fx.stepA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("RevealStep(B, A.step) error = %v, want NotFound", err)
	}
	step, err := fx.steps.ByID(ctx, fx.stepA.ID)
	if err != nil {
		t.Fatalf("step A should still exist: %v", err)
	}
	if step.Name != fx.stepA.Name {
		t.Errorf("step A name changed to %q, want %q", step.Name, fx.stepA.Name)
	}
}

func TestExecutionBindingMismatchIsNotFound(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()
	actor := actorForBind()

	if _, err := fx.svc.GetExecutionInEngagement(ctx, fx.engB.ID, fx.execA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("GetExecutionInEngagement(B, A.exec) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.GetExecutionInEngagement(ctx, fx.engA.ID, fx.execA.ID); err != nil {
		t.Fatalf("GetExecutionInEngagement(A, A.exec) error = %v, want nil", err)
	}

	status := storengagement.ExecutionStatusRunning
	if _, err := fx.svc.PatchRedExecution(ctx, actor, fx.engB.ID, fx.execA.ID, engagement.PatchRedExecutionInput{Version: fx.execA.Version, Status: &status}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("PatchRedExecution(B, A.exec) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.PatchBlueDetection(ctx, actor, fx.engB.ID, fx.execA.ID, engagement.PatchBlueDetectionInput{Version: fx.execA.Version}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("PatchBlueDetection(B, A.exec) error = %v, want NotFound", err)
	}
	exec, err := fx.executions.ByID(ctx, fx.execA.ID)
	if err != nil {
		t.Fatalf("execution A should still exist: %v", err)
	}
	if exec.Version != fx.execA.Version || exec.Status != fx.execA.Status {
		t.Errorf("execution A mutated: version %d status %s, want %d %s", exec.Version, exec.Status, fx.execA.Version, fx.execA.Status)
	}
}

func TestCommentBindingMismatchIsNotFound(t *testing.T) {
	fx := newBindFixture(t)
	ctx := context.Background()
	actor := actorForBind()

	if _, err := fx.svc.ListComments(ctx, fx.engB.ID, fx.execA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ListComments(B, A.exec) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.ListCommentRevisions(ctx, fx.engB.ID, fx.commentA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ListCommentRevisions(B, A.comment) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.CreateComment(ctx, actor, fx.engB.ID, fx.execA.ID, "intruder"); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("CreateComment(B, A.exec) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.EditComment(ctx, actor, engagement.EditCommentInput{
		CommentID:    fx.commentA.ID,
		EngagementID: fx.engB.ID,
		ExecutionID:  fx.commentA.ExecutionID,
		Body:         "intruder",
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("EditComment(B, A.comment) error = %v, want NotFound", err)
	}
}

func stepCount(t *testing.T, fx *bindFixture, scenarioID string) int {
	t.Helper()
	steps, err := fx.steps.ListByScenario(context.Background(), scenarioID, blind.Scope{})
	if err != nil {
		t.Fatalf("ListByScenario: %v", err)
	}
	return len(steps)
}
