package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/archive"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storeactivity "github.com/bryanster/blacklight/internal/store/activity"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// ---------------------------------------------------------------------------
// Archive streaming response
// ---------------------------------------------------------------------------

// archiveStreamingResponse implements gen.ExportEngagementArchiveResponseObject
// by streaming the archive ZIP through an io.Pipe.
type archiveStreamingResponse struct {
	pr *io.PipeReader
}

func (r archiveStreamingResponse) VisitExportEngagementArchiveResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="archive.zip"`)
	_, err := io.Copy(w, r.pr)
	r.pr.Close()
	return err
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// ExportEngagementArchive streams a complete engagement archive as a versioned ZIP.
func (h *handlers) ExportEngagementArchive(ctx context.Context,
	request gen.ExportEngagementArchiveRequestObject) (gen.ExportEngagementArchiveResponseObject, error) {

	engagementID := request.EngagementId.String()

	// 1. Get engagement header.
	eng, err := h.engagements.Get(ctx, engagementID)
	if err != nil {
		return nil, err
	}

	// 2. Build blind scope.
	scope, blindFiltered, err := h.analyticsScope(ctx, engagementID)
	if err != nil {
		return nil, err
	}

	// 3. Collect all workbook data.
	scenarioRows, err := h.scenarios.ListByEngagement(ctx, engagementID)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}

	// Collect steps — blind-filtered via scope.
	allSteps, err := h.steps.ListByEngagement(ctx, engagementID, scope.Blind)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	// Build a set of visible step IDs for filtering evidence, comments, activity.
	visibleStepIDs := make(map[string]bool)
	stepsByID := make(map[string]storengagement.Step)
	stepsByScenario := make(map[string][]storengagement.Step)
	for _, s := range allSteps {
		visibleStepIDs[s.ID] = true
		stepsByID[s.ID] = s
		stepsByScenario[s.ScenarioID] = append(stepsByScenario[s.ScenarioID], s)
	}

	// Collect executions — filter to visible steps.
	executions, err := h.executions.ListByEngagement(ctx, engagementID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	execByStep := make(map[string]storengagement.Execution)
	for _, e := range executions {
		if visibleStepIDs[e.StepID] {
			execByStep[e.StepID] = e
		}
	}

	// Collect comments for visible-step executions.
	var allComments []storengagement.Comment
	for _, e := range executions {
		if !visibleStepIDs[e.StepID] {
			continue
		}
		comments, err := h.comments.ListByExecution(ctx, e.ID)
		if err != nil {
			return nil, fmt.Errorf("list comments for execution %s: %w", e.ID, err)
		}
		allComments = append(allComments, comments...)
	}

	// Collect evidence for visible-step executions.
	var allEvidence []storengagement.Evidence
	for _, e := range executions {
		if !visibleStepIDs[e.StepID] {
			continue
		}
		ev, err := h.evidenceRepo.ListByExecution(ctx, e.ID)
		if err != nil {
			return nil, fmt.Errorf("list evidence for execution %s: %w", e.ID, err)
		}
		allEvidence = append(allEvidence, ev...)
	}

	// Collect findings — always all findings, but blind-filter linked step IDs.
	findings, err := h.findings.ListByEngagement(ctx, engagementID)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	type findingWithSteps struct {
		finding storengagement.Finding
		stepIDs []string
	}
	var findingsData []findingWithSteps
	for _, f := range findings {
		steps, err := h.findings.Steps(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("list finding steps for %s: %w", f.ID, err)
		}
		// Filter step IDs to visible ones.
		var visibleSteps []string
		for _, s := range steps {
			if visibleStepIDs[s.ID] {
				visibleSteps = append(visibleSteps, s.ID)
			}
		}
		findingsData = append(findingsData, findingWithSteps{finding: f, stepIDs: visibleSteps})
	}

	// 4. Collect unique user IDs and resolve display names.
	userIDs := collectUserIDs(eng, executions, allComments, findings, allEvidence)
	userNames := make(map[string]string)
	for _, id := range userIDs {
		u, err := h.users.ByID(ctx, id)
		if err != nil {
			// If a user is deleted or missing, use id as fallback.
			userNames[id] = id
			continue
		}
		userNames[id] = u.DisplayName
	}

	// 5. Build archive engagement data with resolved user refs.
	createdBy := archive.UserRef{ID: eng.CreatedBy, DisplayName: userNames[eng.CreatedBy]}

	engagementData := archive.EngagementArchive{
		Engagement: archive.EngagementToJSON(eng, createdBy),
	}

	for _, sc := range scenarioRows {
		scJSON := archive.ScenarioToJSON(sc)
		var stepsWithExecs []archive.StepWithExec
		for _, s := range stepsByScenario[sc.ID] {
			exec, hasExec := execByStep[s.ID]
			stepJSON := archive.StepToJSON(s)
			var execJSON archive.ExecutionJSON
			if hasExec {
				executedBy := archive.UserRef{ID: exec.ExecutedBy, DisplayName: userNames[exec.ExecutedBy]}
				scoredBy := archive.UserRef{ID: exec.ScoredBy, DisplayName: userNames[exec.ScoredBy]}
				execJSON = archive.ExecutionToJSON(exec, executedBy, scoredBy)
			}
			stepsWithExecs = append(stepsWithExecs, archive.StepWithExec{
				Step:      stepJSON,
				Execution: execJSON,
			})
		}
		engagementData.Scenarios = append(engagementData.Scenarios, archive.ScenarioWithSteps{
			Scenario: scJSON,
			Steps:    stepsWithExecs,
		})
	}

	for _, fwd := range findingsData {
		owner := archive.UserRef{ID: fwd.finding.Owner, DisplayName: userNames[fwd.finding.Owner]}
		engagementData.Findings = append(engagementData.Findings, archive.FindingToJSON(fwd.finding, owner, fwd.stepIDs))
	}

	for _, c := range allComments {
		author := archive.UserRef{ID: c.AuthorID, DisplayName: userNames[c.AuthorID]}
		engagementData.Comments = append(engagementData.Comments, archive.CommentToJSON(c, author))
	}

	// 6. Evidence entries.
	var evidenceEntries []archive.EvidenceEntry
	for _, e := range allEvidence {
		uploadedBy := archive.UserRef{ID: e.UploadedBy, DisplayName: userNames[e.UploadedBy]}
		evidenceEntries = append(evidenceEntries, archive.EvidenceToEntry(e, uploadedBy))
	}

	// 7. Compute analytics rollups — frozen as JSON.
	analyticsJSON, err := h.computeFrozenAnalytics(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("compute frozen analytics: %w", err)
	}

	// 8. Collect all activity rows with pagination.
	var allActivityRows []storeactivity.Row
	cursor := ""
	for {
		filter := storeactivity.ListFilter{
			ScopeEngagement: engagementID,
			Limit:           500,
			Cursor:          cursor,
		}
		rows, nextCursor, err := h.activityEntries.List(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("list activity: %w", err)
		}
		// Blind-filter activity rows.
		if blindFiltered {
			rows, err = h.filterBlindActivity(ctx, engagementID, rows)
			if err != nil {
				return nil, fmt.Errorf("filter activity: %w", err)
			}
		}
		allActivityRows = append(allActivityRows, rows...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	// Build activity JSONL rows from stored rows.
	var activityRows []json.RawMessage
	for _, row := range allActivityRows {
		raw, _ := json.Marshal(row) //nolint:errcheck
		activityRows = append(activityRows, raw)
	}
	// Build actor map for activity.jsonl enrichment.
	activityActors := make(map[string]string)
	for _, row := range allActivityRows {
		if _, ok := activityActors[row.ActorID]; !ok {
			activityActors[row.ActorID] = userNames[row.ActorID]
		}
	}

	// 9. Stream the archive through a pipe.
	pr, pw := io.Pipe()
	go func() {
		err := archive.WriteArchive(pw, archive.WriteOptions{
			EngagementID:   eng.ID,
			EngagementName: eng.Name,
			Client:         eng.Client,
			Mode:           string(eng.Mode),
			AttackVersion:  eng.AttackVersion,
			BlindFiltered:  blindFiltered,
			Engagement:     engagementData,
			Analytics:      analyticsJSON,
			Activity:       activityRows,
			Evidence:       evidenceEntries,
			EvidenceOpener: h.evidenceStore.Open,
			ActivityActors: activityActors,
		})
		pw.CloseWithError(err)
	}()

	return archiveStreamingResponse{pr: pr}, nil
}

// computeFrozenAnalytics runs all M5 rollups and returns them as a single
// JSON blob. If any query fails, the error is returned and no analytics
// appear in the archive — the archive is still valid without them.
func (h *handlers) computeFrozenAnalytics(ctx context.Context, scope analytics.Scope) (json.RawMessage, error) {
	type frozenAnalytics struct {
		Coverage     any `json:"coverage,omitempty"`
		Distribution any `json:"distribution,omitempty"`
		MTTD         any `json:"mttd,omitempty"`
		Burndown     any `json:"burndown,omitempty"`
	}

	var result frozenAnalytics

	// Coverage
	tc, err := h.analytics.TechniqueCoverage(ctx, scope)
	if err == nil {
		tacticCov, err := h.analytics.TacticCoverage(ctx, scope)
		if err == nil {
			result.Coverage = map[string]any{
				"technique": tc,
				"tactic":    tacticCov,
			}
		}
	}

	// Distribution
	catDist, err := h.analytics.CategoryDistribution(ctx, scope)
	if err == nil {
		protRate, _ := h.analytics.ProtectionRate(ctx, scope)      //nolint:errcheck
		outcome, _ := h.analytics.OutcomeMix(ctx, scope)           //nolint:errcheck
		modDist, _ := h.analytics.ModifierDistribution(ctx, scope) //nolint:errcheck
		result.Distribution = map[string]any{
			"categoryDistribution": catDist,
			"protectionRate":       protRate,
			"outcomeMix":           outcome,
			"modifierDistribution": modDist,
		}
	}

	// MTTD
	mttd, err := h.analytics.MTTD(ctx, scope)
	if err == nil {
		result.MTTD = mttd
	}

	// Burndown
	burndown, err := h.analytics.FindingsBurndown(ctx, scope, "")
	if err == nil {
		severity, _ := h.analytics.FindingsBySeverity(ctx, scope) //nolint:errcheck
		result.Burndown = map[string]any{
			"burndown":         burndown,
			"severitySnapshot": severity,
		}
	}

	return json.Marshal(result)
}

// collectUserIDs gathers every distinct user ID from the engagement graph.
func collectUserIDs(eng storengagement.Engagement,
	executions []storengagement.Execution,
	comments []storengagement.Comment,
	findings []storengagement.Finding,
	evidence []storengagement.Evidence) []string {

	seen := make(map[string]bool)
	seen[eng.CreatedBy] = true
	for _, e := range executions {
		if e.ExecutedBy != "" {
			seen[e.ExecutedBy] = true
		}
		if e.ScoredBy != "" {
			seen[e.ScoredBy] = true
		}
	}
	for _, c := range comments {
		if c.AuthorID != "" {
			seen[c.AuthorID] = true
		}
	}
	for _, f := range findings {
		if f.Owner != "" {
			seen[f.Owner] = true
		}
	}
	for _, ev := range evidence {
		if ev.UploadedBy != "" {
			seen[ev.UploadedBy] = true
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		if id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}
