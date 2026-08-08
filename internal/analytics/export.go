package analytics

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// ExecutionsExport — one row per step in the engagement
// ---------------------------------------------------------------------------

// ExecutionsExport returns one row per step visible under scope, with
// scenario, technique, red status/times, blue scoring, MTTD and derived
// outcome. Rows are ordered by scenario ordinal then step ordinal.
//
// The caller MUST close the returned rows.
func (q *Queries) ExecutionsExport(ctx context.Context, scope Scope) (*sql.Rows, error) {
	query := fmt.Sprintf(executionsExportQuery, outcomeCase, scope.stepPredicate())
	return q.db.Read().QueryContext(ctx, query, scope.EngagementID)
}

// executionsExportQuery returns one row per step with full red/blue data.
// The first %s is outcomeCase; the second is the blind-step predicate.
const executionsExportQuery = `
SELECT
    sc.name                                       AS scenario_name,
    sc.ordinal                                    AS scenario_ordinal,
    st.ordinal                                    AS step_ordinal,
    st.name                                       AS step_name,
    st.technique_id,
    st.subtechnique_id,
    st.tactic_id,
    st.objective,
    st.target_asset,
    execution.status                              AS red_status,
    execution.executed_by,
    execution.started_at,
    execution.ended_at,
    execution.command_run,
    execution.source_host,
    execution.target_host,
    execution.red_notes,
    execution.detection_category,
    execution.detection_modifiers::VARCHAR        AS detection_modifiers,
    execution.protection,
    execution.detected_at,
    execution.detecting_source,
    execution.detecting_rule_ref,
    execution.alert_severity,
    execution.blue_notes,
    execution.scored_by,
    execution.scored_at,
    -- MTTD seconds: detected_at - started_at.
    CASE WHEN execution.detected_at IS NOT NULL AND execution.started_at IS NOT NULL
         THEN EXTRACT(EPOCH FROM (execution.detected_at - execution.started_at))
    END                                          AS mttd_seconds,
    -- Derived outcome from the shared CASE expression.
    %s                                            AS derived_outcome
FROM app.step st
JOIN app.scenario sc ON sc.id = st.scenario_id
JOIN app.execution execution ON execution.step_id = st.id
WHERE sc.engagement_id = ?
  AND %s
ORDER BY sc.ordinal, st.ordinal
`

// ---------------------------------------------------------------------------
// FindingsExport — one row per finding
// ---------------------------------------------------------------------------

// FindingsExport returns one row per finding visible under scope, with
// linked step IDs filtered to revealed steps only. Findings whose only
// linked steps are unrevealed are excluded entirely.
//
// The caller MUST close the returned rows.
func (q *Queries) FindingsExport(ctx context.Context, scope Scope) (*sql.Rows, error) {
	query := fmt.Sprintf(findingsExportQuery, scope.stepPredicate(), scope.stepPredicate())
	return q.db.Read().QueryContext(ctx, query, scope.EngagementID)
}

// findingsExportQuery returns one row per finding. Linked step IDs are
// aggregated and filtered to revealed steps only. Findings with no
// revealed linked steps (but at least one unrevealed linked step) are
// excluded. Both %s are the blind-step predicate.
//
// DuckDB-specific: string_agg.
const findingsExportQuery = `
SELECT
    f.title,
    f.description,
    f.severity,
    f.status,
    f."owner",
    f.recommendation,
    COALESCE(
        (SELECT string_agg(fs.step_id, ';' ORDER BY fs.step_id)
         FROM app.finding_step fs
         JOIN app.step st2 ON st2.id = fs.step_id
         WHERE fs.finding_id = f.id
           AND %s
        ),
        ''
    )                                             AS linked_step_ids,
    f.created_at,
    f.updated_at
FROM app.finding f
WHERE f.engagement_id = ?
  AND (
      NOT EXISTS (SELECT 1 FROM app.finding_step fs_empty WHERE fs_empty.finding_id = f.id)
      OR EXISTS (
          SELECT 1
          FROM app.finding_step fs_vis
          JOIN app.step st_vis ON st_vis.id = fs_vis.step_id
          WHERE fs_vis.finding_id = f.id
            AND %s
      )
  )
ORDER BY f.created_at
`
