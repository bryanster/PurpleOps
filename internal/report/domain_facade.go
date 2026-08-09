package report

import (
	"context"
	"io"

	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// DomainAdapter wraps engagement store repositories to implement
// [DomainFacade] for the report renderer.
type DomainAdapter struct {
	Scenarios  *storengagement.Scenarios
	Steps      *storengagement.Steps
	Executions *storengagement.Executions
	Findings   *storengagement.Findings
	Evidence   *storengagement.EvidenceRepo
}

var _ DomainFacade = (*DomainAdapter)(nil)

func (d *DomainAdapter) ListScenarios(ctx context.Context, engagementID string) ([]storengagement.Scenario, error) {
	return d.Scenarios.ListByEngagement(ctx, engagementID)
}

func (d *DomainAdapter) ListSteps(ctx context.Context, engagementID string, scope blind.Scope) ([]storengagement.Step, error) {
	return d.Steps.ListByEngagement(ctx, engagementID, scope)
}

func (d *DomainAdapter) ListExecutions(ctx context.Context, engagementID string) ([]storengagement.Execution, error) {
	return d.Executions.ListByEngagement(ctx, engagementID, nil, nil)
}

func (d *DomainAdapter) ListFindings(ctx context.Context, engagementID string) ([]storengagement.Finding, error) {
	return d.Findings.ListByEngagement(ctx, engagementID)
}

func (d *DomainAdapter) FindingSteps(ctx context.Context, findingID string) ([]storengagement.Step, error) {
	return d.Findings.Steps(ctx, findingID)
}

func (d *DomainAdapter) ListEvidence(ctx context.Context, executionID string) ([]storengagement.Evidence, error) {
	return d.Evidence.ListByExecution(ctx, executionID)
}

// EvidenceStorage implements [EvidenceAccess] by reading blobs from the
// content-addressed evidence store.
type EvidenceStorage struct {
	Store *evidence.Store
}

var _ EvidenceAccess = (*EvidenceStorage)(nil)

// OpenEvidence returns a reader for the blob identified by its SHA-256 hex digest.
func (e *EvidenceStorage) OpenEvidence(sha256hex string) (io.ReadCloser, error) {
	return e.Store.Open(sha256hex)
}
