package analytics

import (
	"context"
	"database/sql"
	"fmt"

	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// ---------------------------------------------------------------------------
// TechniqueCoverage — one row per technique / sub-technique in the engagement
// ---------------------------------------------------------------------------

// TechniqueCoverageRow is one technique cell in the coverage rollup.
// Sub-techniques are their own cells; a parent technique counts as covered
// only if a step names it directly.
type TechniqueCoverageRow struct {
	TechniqueID         string // ATT&CK external id (e.g. "T1059", "T1059.001")
	Name                string // technique name from matrix, or "Unknown: Txxxx"
	IsSubtechnique      bool
	ParentTechniqueID   string // empty for top-level techniques
	Matched             bool   // true if technique exists in pinned ATT&CK version
	Attempted           bool   // true if any execution is complete or blocked
	BestCategory        string // highest-ordinal detection_category across attempted executions; empty if unscored
	BestCategoryOrdinal *int   // numeric ordinal (0-4) matching BestCategory; nil if unscored
	BestProtection      string // best-achieved protection across attempted executions; empty if unscored
	StepCount           int    // number of distinct steps referencing this technique
}

// TechniqueCoverageResult bundles the per-technique rows with the summary
// counts that a consumer needs to print both denominators.
type TechniqueCoverageResult struct {
	Rows                   []TechniqueCoverageRow
	AttemptedTechniques    int // distinct techniques with an attempted execution
	NotAttemptedTechniques int // distinct techniques with NO attempted execution
	MatrixTechniques       int // distinct techniques in the pinned ATT&CK version
	UnmatchedTechniques    int // workbook techniques absent from the pinned version
}

// TechniqueCoverage returns one row per distinct technique external-id
// visible under scope, with best-of scoring and both denominator counts.
//
// The query is a single statement (PLAN.md §5).
func (q *Queries) TechniqueCoverage(ctx context.Context, scope Scope) (*TechniqueCoverageResult, error) {
	query := fmt.Sprintf(techniqueCoverageQuery, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query,
		scope.EngagementID,
		storecontent.SourceIDAttack,
		scope.EngagementID,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics: technique coverage: %w", err)
	}
	defer rows.Close()

	result := &TechniqueCoverageResult{}
	for rows.Next() {
		var row TechniqueCoverageRow
		var attempted, matched bool
		var bestCat, bestProt sql.NullString
		var bestCatOrdinal sql.NullInt64

		if err := rows.Scan(
			&row.TechniqueID, &row.Name, &row.IsSubtechnique,
			&row.ParentTechniqueID, &matched, &attempted,
			&bestCat, &bestCatOrdinal, &bestProt,
			&row.StepCount,
			&result.AttemptedTechniques,
			&result.NotAttemptedTechniques,
			&result.MatrixTechniques,
			&result.UnmatchedTechniques,
		); err != nil {
			return nil, fmt.Errorf("analytics: scanning technique coverage row: %w", err)
		}

		row.Attempted = attempted
		row.Matched = matched
		if bestCat.Valid {
			row.BestCategory = bestCat.String
		}
		if bestCatOrdinal.Valid {
			v := int(bestCatOrdinal.Int64)
			row.BestCategoryOrdinal = &v
		}
		if bestProt.Valid {
			row.BestProtection = bestProt.String
		}

		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating technique coverage rows: %w", err)
	}

	return result, nil
}

// techniqueCoverageQuery is a single statement that returns one row per
// distinct technique external-id in the engagement, plus summary counts
// as repeated columns on every row.
//
// The %s placeholder is the blind-step predicate from scope.stepPredicate() —
// always a constant ("TRUE" or "revealed_at IS NOT NULL"), never caller input.
//
// DuckDB-specific: CASE expressions mapping category/protection strings
// to ordinals for MAX aggregation. A portable database would need a lookup
// table join.
const techniqueCoverageQuery = `
WITH
  workbook_techniques AS (
    SELECT DISTINCT
      s.technique_id AS external_id,
      s.id AS step_id
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    WHERE sc.engagement_id = ?
      AND %s
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),
  technique_execs AS (
    SELECT
      wt.external_id,
      e.status,
      e.detection_category,
      e.protection
    FROM workbook_techniques wt
    JOIN app.execution e ON e.step_id = wt.step_id
  ),
  matrix AS (
    SELECT
      ct.external_id,
      ct.name,
      ct.is_subtechnique,
      ct.parent_external_id
    FROM content.content_technique ct
    WHERE ct.source_id = ?
      AND ct.version = (SELECT attack_version FROM app.engagement WHERE id = ?)
  ),
  best AS (
    SELECT
      wt.external_id,
      MAX(CASE WHEN te.detection_category IS NOT NULL
        THEN CASE te.detection_category
          WHEN 'none' THEN 0
          WHEN 'telemetry' THEN 1
          WHEN 'general' THEN 2
          WHEN 'tactic' THEN 3
          WHEN 'technique' THEN 4
        END
      END) AS best_cat_ordinal,
      MAX(CASE WHEN te.protection IS NOT NULL
        THEN CASE te.protection
          WHEN 'n/a' THEN 0
          WHEN 'not_blocked' THEN 1
          WHEN 'partial' THEN 2
          WHEN 'blocked' THEN 3
        END
      END) AS best_prot_ordinal
    FROM workbook_techniques wt
    JOIN technique_execs te ON wt.external_id = te.external_id
    WHERE te.status IN ('complete', 'blocked')
    GROUP BY wt.external_id
  ),
  per_technique AS (
    SELECT
      wt.external_id,
      COALESCE(
        NULLIF(m.name, ''),
        'Unknown: ' || wt.external_id
      ) AS name,
      COALESCE(m.is_subtechnique, false) AS is_subtechnique,
      COALESCE(NULLIF(m.parent_external_id, ''), '') AS parent_technique_id,
      m.external_id IS NOT NULL AS matched,
      COALESCE(BOOL_OR(te.status IN ('complete', 'blocked')), false) AS attempted,
      CASE b.best_cat_ordinal
        WHEN 0 THEN 'none'
        WHEN 1 THEN 'telemetry'
        WHEN 2 THEN 'general'
        WHEN 3 THEN 'tactic'
        WHEN 4 THEN 'technique'
      END AS best_category,
      b.best_cat_ordinal AS best_category_ordinal,
      CASE b.best_prot_ordinal
        WHEN 0 THEN 'n/a'
        WHEN 1 THEN 'not_blocked'
        WHEN 2 THEN 'partial'
        WHEN 3 THEN 'blocked'
      END AS best_protection,
      COUNT(DISTINCT wt.step_id) AS step_count
    FROM workbook_techniques wt
    LEFT JOIN matrix m ON wt.external_id = m.external_id
    LEFT JOIN technique_execs te ON wt.external_id = te.external_id
    LEFT JOIN best b ON wt.external_id = b.external_id
    GROUP BY
      wt.external_id,
      m.name,
      m.is_subtechnique,
      m.parent_external_id,
      m.external_id,
      b.best_cat_ordinal,
      b.best_prot_ordinal
  )
SELECT
  pt.*,
  (SELECT COUNT(*) FROM per_technique WHERE attempted)       AS attempted_techniques,
  (SELECT COUNT(*) FROM per_technique WHERE NOT attempted)   AS not_attempted_techniques,
  (SELECT COUNT(*) FROM matrix)                               AS matrix_techniques,
  (SELECT COUNT(*) FROM per_technique WHERE NOT matched)      AS unmatched_techniques
FROM per_technique pt
ORDER BY pt.external_id
`

// ---------------------------------------------------------------------------
// TacticCoverage — one row per tactic in the pinned ATT&CK version
// ---------------------------------------------------------------------------

// TacticCoverageRow is one tactic in the coverage rollup. A technique in
// two tactics counts in both; the sum of tactic-covered counts can exceed
// the technique-covered count — this is documented, not deduplicated.
type TacticCoverageRow struct {
	TacticID             string         // ATT&CK external id (e.g. "TA0001")
	TacticName           string         // tactic name from matrix
	TechniquesAttempted  int            // distinct attempted techniques in this tactic
	TechniquesInMatrix   int            // distinct matrix techniques in this tactic
	CategoryDistribution map[string]int // detection category → count of attempted techniques
}

// TacticCoverageResult bundles the per-tactic rows.
type TacticCoverageResult struct {
	Rows []TacticCoverageRow
}

// TacticCoverage returns one row per tactic in the engagement's pinned
// ATT&CK version, with attempted-technique counts and category distribution.
//
// The query is a single statement (PLAN.md §5). Category distribution is
// fetched in a second query because combining it with the tactic row
// requires JSON aggregation — and the tactic list is bounded (~14).
func (q *Queries) TacticCoverage(ctx context.Context, scope Scope) (*TacticCoverageResult, error) {
	query := fmt.Sprintf(tacticCoverageQuery, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query,
		scope.EngagementID,          // sc.engagement_id
		storecontent.SourceIDAttack, // ct.source_id
		scope.EngagementID,          // ct.version subquery
		storecontent.SourceIDAttack, // mtt.source_id
		scope.EngagementID,          // mtt.version subquery
	)
	if err != nil {
		return nil, fmt.Errorf("analytics: tactic coverage: %w", err)
	}
	defer rows.Close()

	result := &TacticCoverageResult{}
	for rows.Next() {
		var row TacticCoverageRow
		if err := rows.Scan(
			&row.TacticID,
			&row.TacticName,
			&row.TechniquesAttempted,
			&row.TechniquesInMatrix,
		); err != nil {
			return nil, fmt.Errorf("analytics: scanning tactic coverage row: %w", err)
		}
		row.CategoryDistribution = make(map[string]int)
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating tactic coverage rows: %w", err)
	}

	if len(result.Rows) > 0 {
		if err := q.fillCategoryDistribution(ctx, scope, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// fillCategoryDistribution runs a single query that returns tactic × category
// counts for every tactic in the engagement, then populates the rows.
func (q *Queries) fillCategoryDistribution(ctx context.Context, scope Scope, result *TacticCoverageResult) error {
	query := fmt.Sprintf(tacticCategoryDistQuery, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query,
		scope.EngagementID,
		storecontent.SourceIDAttack,
		scope.EngagementID,
		storecontent.SourceIDAttack,
		scope.EngagementID,
	)
	if err != nil {
		return fmt.Errorf("analytics: tactic category distribution: %w", err)
	}
	defer rows.Close()

	dist := make(map[string]map[string]int)
	for rows.Next() {
		var tacticID, category sql.NullString
		var count int
		if err := rows.Scan(&tacticID, &category, &count); err != nil {
			return fmt.Errorf("analytics: scanning category distribution: %w", err)
		}
		if !tacticID.Valid {
			continue
		}
		m, ok := dist[tacticID.String]
		if !ok {
			m = make(map[string]int)
			dist[tacticID.String] = m
		}
		cat := "unscored"
		if category.Valid && category.String != "" {
			cat = category.String
		}
		m[cat] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("analytics: iterating category distribution: %w", err)
	}

	for i := range result.Rows {
		if m, ok := dist[result.Rows[i].TacticID]; ok {
			result.Rows[i].CategoryDistribution = m
		}
	}

	return nil
}

// tacticCoverageQuery returns one row per tactic with attempted and matrix
// technique counts. The %s placeholder is the blind-step predicate.
const tacticCoverageQuery = `
WITH
  workbook_techniques AS (
    SELECT DISTINCT
      s.technique_id AS external_id,
      s.id AS step_id
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    WHERE sc.engagement_id = ?
      AND %s
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),
  technique_execs AS (
    SELECT
      wt.external_id,
      e.status
    FROM workbook_techniques wt
    JOIN app.execution e ON e.step_id = wt.step_id
  ),
  per_technique AS (
    SELECT
      wt.external_id,
      COALESCE(BOOL_OR(te.status IN ('complete', 'blocked')), false) AS attempted
    FROM workbook_techniques wt
    LEFT JOIN technique_execs te ON wt.external_id = te.external_id
    GROUP BY wt.external_id
  ),
  tactics AS (
    SELECT
      ct.external_id,
      ct.name
    FROM content.content_tactic ct
    WHERE ct.source_id = ?
      AND ct.version = (SELECT attack_version FROM app.engagement WHERE id = ?)
  )
SELECT
  t.external_id AS tactic_id,
  t.name        AS tactic_name,
  COUNT(DISTINCT CASE WHEN pt.attempted
    THEN mtt.technique_external_id
  END) AS techniques_attempted,
  COUNT(DISTINCT mtt.technique_external_id) AS techniques_in_matrix
FROM tactics t
JOIN content.content_technique_tactic mtt
  ON mtt.tactic_external_id = t.external_id
 AND mtt.source_id = ?
 AND mtt.version = (SELECT attack_version FROM app.engagement WHERE id = ?)
LEFT JOIN per_technique pt
  ON mtt.technique_external_id = pt.external_id
GROUP BY t.external_id, t.name
ORDER BY t.external_id
`

// tacticCategoryDistQuery returns (tactic_id, detection_category, count) rows
// for attempted techniques in each tactic. Category is the technique's best
// category (same as TechniqueCoverage).
const tacticCategoryDistQuery = `
WITH
  workbook_techniques AS (
    SELECT DISTINCT
      s.technique_id AS external_id,
      s.id AS step_id
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    WHERE sc.engagement_id = ?
      AND %s
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),
  technique_execs AS (
    SELECT
      wt.external_id,
      e.status,
      e.detection_category
    FROM workbook_techniques wt
    JOIN app.execution e ON e.step_id = wt.step_id
  ),
  best AS (
    SELECT
      wt.external_id,
      MAX(CASE WHEN te.detection_category IS NOT NULL
        THEN CASE te.detection_category
          WHEN 'none' THEN 0
          WHEN 'telemetry' THEN 1
          WHEN 'general' THEN 2
          WHEN 'tactic' THEN 3
          WHEN 'technique' THEN 4
        END
      END) AS best_cat_ordinal
    FROM workbook_techniques wt
    JOIN technique_execs te ON wt.external_id = te.external_id
    WHERE te.status IN ('complete', 'blocked')
    GROUP BY wt.external_id
  ),
  per_technique AS (
    SELECT
      wt.external_id,
      COALESCE(BOOL_OR(te.status IN ('complete', 'blocked')), false) AS attempted,
      CASE b.best_cat_ordinal
        WHEN 0 THEN 'none'
        WHEN 1 THEN 'telemetry'
        WHEN 2 THEN 'general'
        WHEN 3 THEN 'tactic'
        WHEN 4 THEN 'technique'
      END AS best_category
    FROM workbook_techniques wt
    LEFT JOIN technique_execs te ON wt.external_id = te.external_id
    LEFT JOIN best b ON wt.external_id = b.external_id
    GROUP BY wt.external_id, b.best_cat_ordinal
  )
SELECT
  mtt.tactic_external_id AS tactic_id,
  pt.best_category        AS category,
  COUNT(*)                AS cnt
FROM content.content_technique_tactic mtt
JOIN per_technique pt
  ON mtt.technique_external_id = pt.external_id
 AND mtt.source_id = ?
 AND mtt.version = (SELECT attack_version FROM app.engagement WHERE id = ?)
WHERE pt.attempted
GROUP BY mtt.tactic_external_id, pt.best_category
ORDER BY mtt.tactic_external_id, pt.best_category
`
