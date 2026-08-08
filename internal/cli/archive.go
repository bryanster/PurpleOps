package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/archive"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/store"
	storeactivity "github.com/bryanster/blacklight/internal/store/activity"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	identity "github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/blind"
)

// newArchiveCommand builds `blctl engagement archive`.
func newArchiveCommand(a *app) *cobra.Command {
	var (
		outputFile  string
		evidenceDir string
	)

	cmd := &cobra.Command{
		Use:   "archive <engagement-id>",
		Short: "Export an engagement archive as a versioned ZIP.",
		Long: `Exports a complete, self-contained engagement archive: structure,
scores, findings, comments, activity and evidence files, in one
versioned ZIP. The format is documented in docs/archive.md.

The archive is written to stdout (--output -) or a file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engagementID := args[0]
			cfg, err := a.settings()
			if err != nil {
				return fmt.Errorf("settings: %w", err)
			}
			a.logger(cfg)

			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				return runArchive(ctx, db, engagementID, outputFile, evidenceDir)
			})
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "-", "Output file path (- for stdout)")
	cmd.Flags().StringVar(&evidenceDir, "evidence-dir", "./evidence", "Evidence blob store directory")

	return cmd
}

// runArchive builds and writes the engagement archive.
func runArchive(ctx context.Context, db *store.DB, engagementID, outputFile, evidenceDir string) error {
	engagements := storengagement.NewEngagements(db)
	scenarios := storengagement.NewScenarios(db)
	steps := storengagement.NewSteps(db)
	executions := storengagement.NewExecutions(db)
	comments := storengagement.NewComments(db)
	findings := storengagement.NewFindings(db)
	evidenceRepo := storengagement.NewEvidenceRepo(db)
	blobRepo := storengagement.NewEvidenceBlobRepo(db)
	activityEntries := storeactivity.New(db)
	users := identity.NewUsers(db)
	queries := analytics.NewQueries(db)

	evidenceCfg := config.Evidence{
		MaxUploadBytes:     256 << 20,
		MaxEngagementBytes: 2 << 30,
	}
	evidenceStore := evidence.NewStore(evidenceDir, evidenceCfg, blobRepo)

	// 1. Get engagement.
	eng, err := engagements.ByID(ctx, engagementID)
	if err != nil {
		return fmt.Errorf("get engagement: %w", err)
	}

	// 2. Build blind scope — blctl exports the full (unfiltered) record.
	scope := analytics.Scope{
		EngagementID: engagementID,
		Blind:        blind.Scope{},
	}

	// 3. Collect workbook data.
	scenarioRows, err := scenarios.ListByEngagement(ctx, engagementID)
	if err != nil {
		return fmt.Errorf("list scenarios: %w", err)
	}

	allSteps, err := steps.ListByEngagement(ctx, engagementID, scope.Blind)
	if err != nil {
		return fmt.Errorf("list steps: %w", err)
	}

	visibleStepIDs := make(map[string]bool)
	stepsByScenario := make(map[string][]storengagement.Step)
	for _, s := range allSteps {
		visibleStepIDs[s.ID] = true
		stepsByScenario[s.ScenarioID] = append(stepsByScenario[s.ScenarioID], s)
	}

	allExecs, err := executions.ListByEngagement(ctx, engagementID, nil, nil)
	if err != nil {
		return fmt.Errorf("list executions: %w", err)
	}
	execByStep := make(map[string]storengagement.Execution)
	for _, e := range allExecs {
		if visibleStepIDs[e.StepID] {
			execByStep[e.StepID] = e
		}
	}

	var allComments []storengagement.Comment
	for _, e := range allExecs {
		if !visibleStepIDs[e.StepID] {
			continue
		}
		cmts, err := comments.ListByExecution(ctx, e.ID)
		if err != nil {
			return fmt.Errorf("list comments: %w", err)
		}
		allComments = append(allComments, cmts...)
	}

	var allEvidence []storengagement.Evidence
	for _, e := range allExecs {
		if !visibleStepIDs[e.StepID] {
			continue
		}
		ev, err := evidenceRepo.ListByExecution(ctx, e.ID)
		if err != nil {
			return fmt.Errorf("list evidence: %w", err)
		}
		allEvidence = append(allEvidence, ev...)
	}

	allFindings, err := findings.ListByEngagement(ctx, engagementID)
	if err != nil {
		return fmt.Errorf("list findings: %w", err)
	}
	type findingWithSteps struct {
		finding storengagement.Finding
		stepIDs []string
	}
	var findingsData []findingWithSteps
	for _, f := range allFindings {
		fSteps, err := findings.Steps(ctx, f.ID)
		if err != nil {
			return fmt.Errorf("list finding steps: %w", err)
		}
		var visibleSteps []string
		for _, s := range fSteps {
			if visibleStepIDs[s.ID] {
				visibleSteps = append(visibleSteps, s.ID)
			}
		}
		findingsData = append(findingsData, findingWithSteps{finding: f, stepIDs: visibleSteps})
	}

	// 4. Resolve user display names.
	userIDs := cliCollectUserIDs(eng, allExecs, allComments, allFindings, allEvidence)
	userNames := make(map[string]string)
	for _, id := range userIDs {
		u, err := users.ByID(ctx, id)
		if err != nil {
			userNames[id] = id
			continue
		}
		userNames[id] = u.DisplayName
	}

	// 5. Build archive data.
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
	var evidenceEntries []archive.EvidenceEntry
	for _, e := range allEvidence {
		uploadedBy := archive.UserRef{ID: e.UploadedBy, DisplayName: userNames[e.UploadedBy]}
		evidenceEntries = append(evidenceEntries, archive.EvidenceToEntry(e, uploadedBy))
	}

	// 6. Compute analytics.
	analyticsJSON, _ := cliComputeFrozenAnalytics(ctx, queries, scope)

	// 7. Collect activity rows.
	var allActivityRows []storeactivity.Row
	cursor := ""
	for {
		filter := storeactivity.ListFilter{
			ScopeEngagement: engagementID,
			Limit:           500,
			Cursor:          cursor,
		}
		rows, nextCursor, err := activityEntries.List(ctx, filter)
		if err != nil {
			return fmt.Errorf("list activity: %w", err)
		}
		allActivityRows = append(allActivityRows, rows...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	var activityRows []json.RawMessage
	for _, row := range allActivityRows {
		raw, _ := json.Marshal(row)
		activityRows = append(activityRows, raw)
	}
	activityActors := make(map[string]string)
	for _, row := range allActivityRows {
		if _, ok := activityActors[row.ActorID]; !ok {
			activityActors[row.ActorID] = userNames[row.ActorID]
		}
	}

	// 8. Open output.
	var out io.Writer
	if outputFile == "" || outputFile == "-" {
		out = os.Stdout
	} else {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	// 9. Write archive.
	return archive.WriteArchive(out, archive.WriteOptions{
		EngagementID:   eng.ID,
		EngagementName: eng.Name,
		Client:         eng.Client,
		Mode:           string(eng.Mode),
		AttackVersion:  eng.AttackVersion,
		BlindFiltered:  false,
		Engagement:     engagementData,
		Analytics:      analyticsJSON,
		Activity:       activityRows,
		Evidence:       evidenceEntries,
		EvidenceOpener: evidenceStore.Open,
		ActivityActors: activityActors,
	})
}

func cliComputeFrozenAnalytics(ctx context.Context, q *analytics.Queries, scope analytics.Scope) (json.RawMessage, error) {
	type frozen struct {
		Coverage     any `json:"coverage,omitempty"`
		Distribution any `json:"distribution,omitempty"`
		MTTD         any `json:"mttd,omitempty"`
		Burndown     any `json:"burndown,omitempty"`
	}
	var result frozen
	if tc, err := q.TechniqueCoverage(ctx, scope); err == nil {
		if tactic, err := q.TacticCoverage(ctx, scope); err == nil {
			result.Coverage = map[string]any{"technique": tc, "tactic": tactic}
		}
	}
	if cat, err := q.CategoryDistribution(ctx, scope); err == nil {
		protRate, _ := q.ProtectionRate(ctx, scope)
		outcome, _ := q.OutcomeMix(ctx, scope)
		modDist, _ := q.ModifierDistribution(ctx, scope)
		result.Distribution = map[string]any{
			"categoryDistribution": cat,
			"protectionRate":       protRate,
			"outcomeMix":           outcome,
			"modifierDistribution": modDist,
		}
	}
	if mttd, err := q.MTTD(ctx, scope); err == nil {
		result.MTTD = mttd
	}
	if burndown, err := q.FindingsBurndown(ctx, scope, ""); err == nil {
		severity, _ := q.FindingsBySeverity(ctx, scope)
		result.Burndown = map[string]any{"burndown": burndown, "severitySnapshot": severity}
	}
	return json.Marshal(result)
}

func cliCollectUserIDs(eng storengagement.Engagement,
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
