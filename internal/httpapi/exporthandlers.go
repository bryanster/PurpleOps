package httpapi

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// ---------------------------------------------------------------------------
// Streaming response types — implement gen.ExportEngagementResponseObject
// ---------------------------------------------------------------------------

// exportCSVResponse streams rows as CSV.
type exportCSVResponse struct {
	filename string
	write    func(w *csv.Writer) error
}

func (r exportCSVResponse) VisitExportEngagementResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, r.filename))
	w.WriteHeader(http.StatusOK)

	// UTF-8 BOM so Excel renders accented characters correctly.
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	if err := r.write(cw); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// exportJSONResponse streams rows as a JSON array.
type exportJSONResponse struct {
	filename string
	stream   func(w io.Writer) error
}

func (r exportJSONResponse) VisitExportEngagementResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, r.filename))
	w.WriteHeader(http.StatusOK)
	return r.stream(w)
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// ExportEngagement exports engagement data as flat JSON or CSV.
func (h *handlers) ExportEngagement(ctx context.Context,
	request gen.ExportEngagementRequestObject) (gen.ExportEngagementResponseObject, error) {

	engagementID := request.EngagementId.String()
	format := request.Params.Format
	dataset := request.Params.Dataset

	scope, _, err := h.analyticsScope(ctx, engagementID)
	if err != nil {
		return nil, err
	}

	name, err := h.engagementName(ctx, engagementID)
	if err != nil {
		return nil, err
	}
	safeName := sanitizeFilename(name)

	ext := string(format)
	filename := fmt.Sprintf("%s-%s.%s", safeName, string(dataset), ext)

	switch dataset {
	case gen.ExportEngagementParamsDatasetExecutions:
		return h.exportExecutions(ctx, scope, format, filename)
	case gen.ExportEngagementParamsDatasetFindings:
		return h.exportFindings(ctx, scope, format, filename)
	case gen.ExportEngagementParamsDatasetCoverage:
		return h.exportCoverage(ctx, scope, format, filename)
	default:
		return nil, fmt.Errorf("export: unknown dataset %q", dataset)
	}
}

// ---------------------------------------------------------------------------
// Dataset exporters
// ---------------------------------------------------------------------------

func (h *handlers) exportExecutions(ctx context.Context, scope analytics.Scope,
	format gen.ExportEngagementParamsFormat, filename string) (gen.ExportEngagementResponseObject, error) {
	rows, err := h.analytics.ExecutionsExport(ctx, scope) //nolint:rowserrcheck // checked in writer closures
	if err != nil {
		return nil, fmt.Errorf("export executions: %w", err)
	}

	switch format {
	case gen.ExportEngagementParamsFormatCsv:
		return exportCSVResponse{filename: filename, write: func(cw *csv.Writer) error {
			return writeExecutionsCSV(cw, rows)
		}}, nil
	case gen.ExportEngagementParamsFormatJson:
		return exportJSONResponse{filename: filename, stream: func(w io.Writer) error {
			return writeExecutionsJSON(w, rows)
		}}, nil
	default:
		rows.Close()
		return nil, fmt.Errorf("export: unknown format %q", format)
	}
}

func (h *handlers) exportFindings(ctx context.Context, scope analytics.Scope,
	format gen.ExportEngagementParamsFormat, filename string) (gen.ExportEngagementResponseObject, error) {
	rows, err := h.analytics.FindingsExport(ctx, scope) //nolint:rowserrcheck // checked in writer closures
	if err != nil {
		return nil, fmt.Errorf("export findings: %w", err)
	}
	// nolint:rowserrcheck

	switch format {
	case gen.ExportEngagementParamsFormatCsv:
		return exportCSVResponse{filename: filename, write: func(cw *csv.Writer) error {
			return writeFindingsCSV(cw, rows)
		}}, nil
	case gen.ExportEngagementParamsFormatJson:
		return exportJSONResponse{filename: filename, stream: func(w io.Writer) error {
			return writeFindingsJSON(w, rows)
		}}, nil
	default:
		rows.Close()
		return nil, fmt.Errorf("export: unknown format %q", format)
	}
}

func (h *handlers) exportCoverage(ctx context.Context, scope analytics.Scope,
	format gen.ExportEngagementParamsFormat, filename string) (gen.ExportEngagementResponseObject, error) {

	result, err := h.analytics.TechniqueCoverage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("export coverage: %w", err)
	}

	switch format {
	case gen.ExportEngagementParamsFormatCsv:
		return exportCSVResponse{filename: filename, write: func(cw *csv.Writer) error {
			return writeCoverageCSV(cw, result)
		}}, nil
	case gen.ExportEngagementParamsFormatJson:
		return exportJSONResponse{filename: filename, stream: func(w io.Writer) error {
			return writeCoverageJSON(w, result)
		}}, nil
	default:
		return nil, fmt.Errorf("export: unknown format %q", format)
	}
}

// ---------------------------------------------------------------------------
// engagementName returns the engagement's name for the export filename.
// ---------------------------------------------------------------------------

func (h *handlers) engagementName(ctx context.Context, id string) (string, error) {
	var name string
	err := h.store.Read().QueryRowContext(ctx,
		`SELECT name FROM app.engagement WHERE id = ?`, id).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("engagement %q not found", id)
		}
		return "", fmt.Errorf("export: engagement name: %w", err)
	}
	return name, nil
}

// ---------------------------------------------------------------------------
// Filename sanitation
// ---------------------------------------------------------------------------

// sanitizeFilename replaces characters unsafe in a Content-Disposition filename.
func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ':
			return '_'
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, s)
	// Collapse repeated underscores.
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	s = strings.Trim(s, "_.")
	if s == "" {
		return "export"
	}
	return s
}

// ---------------------------------------------------------------------------
// CSV injection escaping
// ---------------------------------------------------------------------------

// escapeCSV prefixes a field with a single quote when it starts with a
// character that Excel interprets as a formula trigger.
//
// The dangerous prefixes are =, +, -, @, tab (\t), and carriage return (\r).
// This is an acceptance criterion, not a judgement call (M5-011).
func escapeCSV(s string) string {
	if s == "" {
		return s
	}
	r, _ := utf8.DecodeRuneInString(s)
	switch r {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// csvRow applies injection escaping to every field in a string slice.
func csvRow(fields []string) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = escapeCSV(f)
	}
	return out
}

// ---------------------------------------------------------------------------
// CSV writers — one per dataset
// ---------------------------------------------------------------------------

var executionsCSVHeader = []string{
	"scenario_name", "scenario_ordinal", "step_ordinal", "step_name",
	"technique_id", "subtechnique_id", "tactic_id",
	"objective", "target_asset",
	"red_status", "executed_by", "started_at", "ended_at",
	"command_run", "source_host", "target_host", "red_notes",
	"detection_category", "detection_modifiers", "protection",
	"detected_at", "detecting_source", "detecting_rule_ref",
	"alert_severity", "blue_notes", "scored_by", "scored_at",
	"mttd_seconds", "derived_outcome",
}

func writeExecutionsCSV(cw *csv.Writer, rows *sql.Rows) error {
	defer rows.Close()

	if err := cw.Write(executionsCSVHeader); err != nil {
		return err
	}

	for rows.Next() {
		var (
			scenarioName, stepName                                      string
			scenarioOrdinal, stepOrdinal                                int
			techniqueID, subtechniqueID, tacticID                       sql.NullString
			objective, targetAsset                                      string
			redStatus, executedBy                                       string
			startedAt, endedAt                                          sql.NullTime
			commandRun, sourceHost, targetHost, redNotes                string
			detectionCategory, detectionModifiers, protection           sql.NullString
			detectedAt                                                  sql.NullTime
			detectingSource, detectingRuleRef, alertSeverity, blueNotes string
			scoredBy                                                    string
			scoredAt                                                    sql.NullTime
			mttdSeconds                                                 sql.NullFloat64
			derivedOutcome                                              string
		)

		if err := rows.Scan(
			&scenarioName, &scenarioOrdinal, &stepOrdinal, &stepName,
			&techniqueID, &subtechniqueID, &tacticID,
			&objective, &targetAsset,
			&redStatus, &executedBy, &startedAt, &endedAt,
			&commandRun, &sourceHost, &targetHost, &redNotes,
			&detectionCategory, &detectionModifiers, &protection,
			&detectedAt, &detectingSource, &detectingRuleRef,
			&alertSeverity, &blueNotes, &scoredBy, &scoredAt,
			&mttdSeconds, &derivedOutcome,
		); err != nil {
			return fmt.Errorf("export executions: scan: %w", err)
		}

		fields := csvRow([]string{
			scenarioName,
			fmt.Sprintf("%d", scenarioOrdinal),
			fmt.Sprintf("%d", stepOrdinal),
			stepName,
			nullStr(techniqueID),
			nullStr(subtechniqueID),
			nullStr(tacticID),
			objective,
			targetAsset,
			redStatus,
			executedBy,
			nullTime(startedAt),
			nullTime(endedAt),
			commandRun,
			sourceHost,
			targetHost,
			redNotes,
			nullStr(detectionCategory),
			nullStr(detectionModifiers),
			nullStr(protection),
			nullTime(detectedAt),
			detectingSource,
			detectingRuleRef,
			alertSeverity,
			blueNotes,
			scoredBy,
			nullTime(scoredAt),
			nullFloat(mttdSeconds),
			derivedOutcome,
		})

		if err := cw.Write(fields); err != nil {
			return err
		}
		cw.Flush() // stream each row
	}
	return rows.Err()
}

var findingsCSVHeader = []string{
	"title", "description", "severity", "status", "owner",
	"recommendation", "linked_step_ids", "created_at", "updated_at",
}

func writeFindingsCSV(cw *csv.Writer, rows *sql.Rows) error {
	defer rows.Close()

	if err := cw.Write(findingsCSVHeader); err != nil {
		return err
	}

	for rows.Next() {
		var (
			title, description, severity, status, owner string
			recommendation, linkedStepIDs               string
			createdAt, updatedAt                        time.Time
		)

		if err := rows.Scan(
			&title, &description, &severity, &status, &owner,
			&recommendation, &linkedStepIDs,
			&createdAt, &updatedAt,
		); err != nil {
			return fmt.Errorf("export findings: scan: %w", err)
		}

		fields := csvRow([]string{
			title, description, severity, status, owner,
			recommendation, linkedStepIDs,
			createdAt.UTC().Format(time.RFC3339),
			updatedAt.UTC().Format(time.RFC3339),
		})

		if err := cw.Write(fields); err != nil {
			return err
		}
		cw.Flush()
	}
	return rows.Err()
}

var coverageCSVHeader = []string{
	"technique_id", "name", "is_subtechnique", "parent_technique_id",
	"matched", "attempted", "best_category", "best_category_ordinal",
	"best_protection", "step_count",
}

func writeCoverageCSV(cw *csv.Writer, result *analytics.TechniqueCoverageResult) error {
	if err := cw.Write(coverageCSVHeader); err != nil {
		return err
	}
	for _, row := range result.Rows {
		ordinal := ""
		if row.BestCategoryOrdinal != nil {
			ordinal = fmt.Sprintf("%d", *row.BestCategoryOrdinal)
		}
		fields := csvRow([]string{
			row.TechniqueID,
			row.Name,
			fmt.Sprintf("%t", row.IsSubtechnique),
			row.ParentTechniqueID,
			fmt.Sprintf("%t", row.Matched),
			fmt.Sprintf("%t", row.Attempted),
			row.BestCategory,
			ordinal,
			row.BestProtection,
			fmt.Sprintf("%d", row.StepCount),
		})
		if err := cw.Write(fields); err != nil {
			return err
		}
		cw.Flush()
	}
	return nil
}

// ---------------------------------------------------------------------------
// JSON writers — one per dataset, streamed as arrays
// ---------------------------------------------------------------------------

type executionRow struct {
	ScenarioName       string   `json:"scenario_name"`
	ScenarioOrdinal    int      `json:"scenario_ordinal"`
	StepOrdinal        int      `json:"step_ordinal"`
	StepName           string   `json:"step_name"`
	TechniqueID        *string  `json:"technique_id"`
	SubtechniqueID     *string  `json:"subtechnique_id"`
	TacticID           *string  `json:"tactic_id"`
	Objective          string   `json:"objective"`
	TargetAsset        string   `json:"target_asset"`
	RedStatus          string   `json:"red_status"`
	ExecutedBy         string   `json:"executed_by"`
	StartedAt          *string  `json:"started_at"`
	EndedAt            *string  `json:"ended_at"`
	CommandRun         string   `json:"command_run"`
	SourceHost         string   `json:"source_host"`
	TargetHost         string   `json:"target_host"`
	RedNotes           string   `json:"red_notes"`
	DetectionCategory  *string  `json:"detection_category"`
	DetectionModifiers *string  `json:"detection_modifiers"`
	Protection         *string  `json:"protection"`
	DetectedAt         *string  `json:"detected_at"`
	DetectingSource    string   `json:"detecting_source"`
	DetectingRuleRef   string   `json:"detecting_rule_ref"`
	AlertSeverity      string   `json:"alert_severity"`
	BlueNotes          string   `json:"blue_notes"`
	ScoredBy           string   `json:"scored_by"`
	ScoredAt           *string  `json:"scored_at"`
	MttdSeconds        *float64 `json:"mttd_seconds"`
	DerivedOutcome     string   `json:"derived_outcome"`
}

func writeExecutionsJSON(w io.Writer, rows *sql.Rows) error {
	defer rows.Close()
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if _, err := w.Write([]byte("[\n")); err != nil {
		return err
	}
	first := true
	for rows.Next() {
		var (
			scenarioName, stepName                                      string
			scenarioOrdinal, stepOrdinal                                int
			techniqueID, subtechniqueID, tacticID                       sql.NullString
			objective, targetAsset                                      string
			redStatus, executedBy                                       string
			startedAt, endedAt                                          sql.NullTime
			commandRun, sourceHost, targetHost, redNotes                string
			detectionCategory, detectionModifiers, protection           sql.NullString
			detectedAt                                                  sql.NullTime
			detectingSource, detectingRuleRef, alertSeverity, blueNotes string
			scoredBy                                                    string
			scoredAt                                                    sql.NullTime
			mttdSeconds                                                 sql.NullFloat64
			derivedOutcome                                              string
		)

		if err := rows.Scan(
			&scenarioName, &scenarioOrdinal, &stepOrdinal, &stepName,
			&techniqueID, &subtechniqueID, &tacticID,
			&objective, &targetAsset,
			&redStatus, &executedBy, &startedAt, &endedAt,
			&commandRun, &sourceHost, &targetHost, &redNotes,
			&detectionCategory, &detectionModifiers, &protection,
			&detectedAt, &detectingSource, &detectingRuleRef,
			&alertSeverity, &blueNotes, &scoredBy, &scoredAt,
			&mttdSeconds, &derivedOutcome,
		); err != nil {
			return fmt.Errorf("export executions json: scan: %w", err)
		}

		if !first {
			if _, err := w.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		first = false

		row := executionRow{
			ScenarioName:       scenarioName,
			ScenarioOrdinal:    scenarioOrdinal,
			StepOrdinal:        stepOrdinal,
			StepName:           stepName,
			TechniqueID:        nullStrPtr(techniqueID),
			SubtechniqueID:     nullStrPtr(subtechniqueID),
			TacticID:           nullStrPtr(tacticID),
			Objective:          objective,
			TargetAsset:        targetAsset,
			RedStatus:          redStatus,
			ExecutedBy:         executedBy,
			StartedAt:          nullTimePtr(startedAt),
			EndedAt:            nullTimePtr(endedAt),
			CommandRun:         commandRun,
			SourceHost:         sourceHost,
			TargetHost:         targetHost,
			RedNotes:           redNotes,
			DetectionCategory:  nullStrPtr(detectionCategory),
			DetectionModifiers: nullStrPtr(detectionModifiers),
			Protection:         nullStrPtr(protection),
			DetectedAt:         nullTimePtr(detectedAt),
			DetectingSource:    detectingSource,
			DetectingRuleRef:   detectingRuleRef,
			AlertSeverity:      alertSeverity,
			BlueNotes:          blueNotes,
			ScoredBy:           scoredBy,
			ScoredAt:           nullTimePtr(scoredAt),
			MttdSeconds:        nullFloatPtr(mttdSeconds),
			DerivedOutcome:     derivedOutcome,
		}

		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err := w.Write([]byte("]\n"))
	return err
}

type findingRow struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Severity       string `json:"severity"`
	Status         string `json:"status"`
	Owner          string `json:"owner"`
	Recommendation string `json:"recommendation"`
	LinkedStepIDs  string `json:"linked_step_ids"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func writeFindingsJSON(w io.Writer, rows *sql.Rows) error {
	defer rows.Close()
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if _, err := w.Write([]byte("[\n")); err != nil {
		return err
	}
	first := true
	for rows.Next() {
		var (
			title, description, severity, status, owner string
			recommendation, linkedStepIDs               string
			createdAt, updatedAt                        time.Time
		)
		if err := rows.Scan(
			&title, &description, &severity, &status, &owner,
			&recommendation, &linkedStepIDs,
			&createdAt, &updatedAt,
		); err != nil {
			return fmt.Errorf("export findings json: scan: %w", err)
		}

		if !first {
			if _, err := w.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		first = false

		if err := enc.Encode(findingRow{
			Title:          title,
			Description:    description,
			Severity:       severity,
			Status:         status,
			Owner:          owner,
			Recommendation: recommendation,
			LinkedStepIDs:  linkedStepIDs,
			CreatedAt:      createdAt.UTC().Format(time.RFC3339),
			UpdatedAt:      updatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err := w.Write([]byte("]\n"))
	return err
}

type coverageRow struct {
	TechniqueID         string `json:"technique_id"`
	Name                string `json:"name"`
	IsSubtechnique      bool   `json:"is_subtechnique"`
	ParentTechniqueID   string `json:"parent_technique_id"`
	Matched             bool   `json:"matched"`
	Attempted           bool   `json:"attempted"`
	BestCategory        string `json:"best_category"`
	BestCategoryOrdinal *int   `json:"best_category_ordinal"`
	BestProtection      string `json:"best_protection"`
	StepCount           int    `json:"step_count"`
}

func writeCoverageJSON(w io.Writer, result *analytics.TechniqueCoverageResult) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if _, err := w.Write([]byte("[\n")); err != nil {
		return err
	}
	first := true
	for _, row := range result.Rows {
		if !first {
			if _, err := w.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		first = false

		if err := enc.Encode(coverageRow{
			TechniqueID:         row.TechniqueID,
			Name:                row.Name,
			IsSubtechnique:      row.IsSubtechnique,
			ParentTechniqueID:   row.ParentTechniqueID,
			Matched:             row.Matched,
			Attempted:           row.Attempted,
			BestCategory:        row.BestCategory,
			BestCategoryOrdinal: row.BestCategoryOrdinal,
			BestProtection:      row.BestProtection,
			StepCount:           row.StepCount,
		}); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("]\n"))
	return err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullStrPtr(ns sql.NullString) *string {
	if ns.Valid {
		s := ns.String
		return &s
	}
	return nil
}

func nullTime(t sql.NullTime) string {
	if t.Valid {
		return t.Time.UTC().Format(time.RFC3339)
	}
	return ""
}

func nullTimePtr(t sql.NullTime) *string {
	if t.Valid {
		s := t.Time.UTC().Format(time.RFC3339)
		return &s
	}
	return nil
}

func nullFloat(f sql.NullFloat64) string {
	if f.Valid {
		return fmt.Sprintf("%.0f", f.Float64)
	}
	return ""
}

func nullFloatPtr(f sql.NullFloat64) *float64 {
	if f.Valid {
		return &f.Float64
	}
	return nil
}
