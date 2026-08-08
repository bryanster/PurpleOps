package analytics

import (
	"context"
	"database/sql"
	"fmt"
)

// CompareScope holds two independent scopes — one for each side of the
// comparison. The caller may hold different seats in the two engagements
// (e.g. lead in the retest, blue in the baseline), so each side applies
// its own blind filter. A single scope would quietly apply the wrong seat
// to one half.
type CompareScope struct {
	Baseline Scope
	Current  Scope
}

// PinMismatch is set when the two engagements pin different ATT&CK
// versions. Compare still produces results; the mismatch is advisory so
// the UI and report blocks can warn.
type PinMismatch struct {
	Baseline string
	Current  string
}

// CompareRow is one paired technique in the cross-engagement comparison.
// Techniques are matched on (technique_id, subtechnique_id).
type CompareRow struct {
	TechniqueID    string
	SubtechniqueID string
	Name           string // technique name from matrix, or "Unknown: Txxxx"

	BaselineCategory        string // detection category label; empty if unscored
	BaselineCategoryOrdinal *int   // numeric ordinal (0-4); nil if unscored
	BaselineProtection      string // protection label; empty if unscored

	CurrentCategory        string
	CurrentCategoryOrdinal *int
	CurrentProtection      string

	OrdinalDelta *int // current - baseline; nil if either side is unscored

	// Classification: one of improved, regressed, unchanged, newlyAttempted,
	// noLongerAttempted, incomparable.
	Classification string
}

// CompareResult bundles per-technique comparison rows with summary counts
// and an optional pin-mismatch advisory.
type CompareResult struct {
	Rows []CompareRow

	Improved          int
	Regressed         int
	Unchanged         int
	NewlyAttempted    int
	NoLongerAttempted int
	Incomparable      int

	PinMismatch *PinMismatch // nil when both engagements pin the same version
}

// Compare returns a cross-engagement technique-by-technique comparison
// between baseline and current scopes.
//
// The query is a single statement (PLAN.md §5). Classification is a SQL
// CASE expression, not a Go loop over rows.
//
// DuckDB-specific: FULL OUTER JOIN, subquery summary counts.
func (q *Queries) Compare(ctx context.Context, scope CompareScope) (*CompareResult, error) {
	query := fmt.Sprintf(compareQuery,
		scope.Baseline.stepPredicate(),
		scope.Current.stepPredicate(),
	)

	rows, err := q.db.Read().QueryContext(ctx, query,
		scope.Baseline.EngagementID,
		scope.Current.EngagementID,
		scope.Baseline.EngagementID,
		scope.Current.EngagementID,
		scope.Baseline.EngagementID,
		scope.Current.EngagementID,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics: compare: %w", err)
	}
	defer rows.Close()

	result := &CompareResult{}
	var basePinStr, curPinStr sql.NullString

	for rows.Next() {
		var row CompareRow
		var baseCat, baseProt, curCat, curProt sql.NullString
		var baseCatOrd, curCatOrd, ordDelta sql.NullInt64
		var basePin, curPin sql.NullString

		if err := rows.Scan(
			&row.TechniqueID, &row.SubtechniqueID, &row.Name,
			&baseCat, &baseCatOrd, &baseProt,
			&curCat, &curCatOrd, &curProt,
			&ordDelta,
			&row.Classification,
			&basePin, &curPin,
			&result.Improved, &result.Regressed, &result.Unchanged,
			&result.NewlyAttempted, &result.NoLongerAttempted, &result.Incomparable,
		); err != nil {
			return nil, fmt.Errorf("analytics: scanning compare row: %w", err)
		}

		if baseCat.Valid {
			row.BaselineCategory = baseCat.String
		}
		if baseCatOrd.Valid {
			v := int(baseCatOrd.Int64)
			row.BaselineCategoryOrdinal = &v
		}
		if baseProt.Valid {
			row.BaselineProtection = baseProt.String
		}
		if curCat.Valid {
			row.CurrentCategory = curCat.String
		}
		if curCatOrd.Valid {
			v := int(curCatOrd.Int64)
			row.CurrentCategoryOrdinal = &v
		}
		if curProt.Valid {
			row.CurrentProtection = curProt.String
		}
		if ordDelta.Valid {
			v := int(ordDelta.Int64)
			row.OrdinalDelta = &v
		}

		// Capture pin values from the first row (repeated on all rows via CROSS JOIN).
		if !basePinStr.Valid && basePin.Valid {
			basePinStr = basePin
		}
		if !curPinStr.Valid && curPin.Valid {
			curPinStr = curPin
		}

		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating compare rows: %w", err)
	}

	// PinMismatch: set when the two engagements pin different ATT&CK versions.
	if basePinStr.Valid && curPinStr.Valid && basePinStr.String != curPinStr.String {
		result.PinMismatch = &PinMismatch{
			Baseline: basePinStr.String,
			Current:  curPinStr.String,
		}
	}

	return result, nil
}

// compareQuery is a single statement that computes the cross-engagement
// technique comparison with classification and summary counts.
//
// The %s placeholders are blind-step predicates — always constants
// ("TRUE" or "revealed_at IS NOT NULL"), never caller input.
//
// DuckDB-specific: FULL OUTER JOIN for symmetric matching, subqueries for
// per-row summary counts, and the CASE-based category/protection ordinal
// mapping (same as TechniqueCoverage).
const compareQuery = `
WITH
  -- Raw steps from each engagement, filtered by blind scope.
  baseline_steps AS (
    SELECT
      s.technique_id,
      COALESCE(NULLIF(s.subtechnique_id, ''), '') AS subtechnique_id,
      s.template_id,
      e.status,
      e.detection_category,
      e.protection
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    JOIN app.execution e ON e.step_id = s.id
    WHERE sc.engagement_id = ?
      AND %s
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),
  current_steps AS (
    SELECT
      s.technique_id,
      COALESCE(NULLIF(s.subtechnique_id, ''), '') AS subtechnique_id,
      s.template_id,
      e.status,
      e.detection_category,
      e.protection
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    JOIN app.execution e ON e.step_id = s.id
    WHERE sc.engagement_id = ?
      AND %s
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),

  -- Best-of per technique per engagement (same logic as TechniqueCoverage).
  baseline_best AS (
    SELECT
      bs.technique_id,
      bs.subtechnique_id,
      COALESCE(MAX(CASE WHEN bs.detection_category IS NOT NULL
        THEN CASE bs.detection_category
          WHEN 'none' THEN 0
          WHEN 'telemetry' THEN 1
          WHEN 'general' THEN 2
          WHEN 'tactic' THEN 3
          WHEN 'technique' THEN 4
        END
      END), NULL) AS best_cat_ordinal,
      COALESCE(MAX(CASE WHEN bs.protection IS NOT NULL
        THEN CASE bs.protection
          WHEN 'n/a' THEN 0
          WHEN 'not_blocked' THEN 1
          WHEN 'partial' THEN 2
          WHEN 'blocked' THEN 3
        END
      END), NULL) AS best_prot_ordinal
    FROM baseline_steps bs
    WHERE bs.status IN ('complete', 'blocked')
    GROUP BY bs.technique_id, bs.subtechnique_id
  ),
  current_best AS (
    SELECT
      cs.technique_id,
      cs.subtechnique_id,
      COALESCE(MAX(CASE WHEN cs.detection_category IS NOT NULL
        THEN CASE cs.detection_category
          WHEN 'none' THEN 0
          WHEN 'telemetry' THEN 1
          WHEN 'general' THEN 2
          WHEN 'tactic' THEN 3
          WHEN 'technique' THEN 4
        END
      END), NULL) AS best_cat_ordinal,
      COALESCE(MAX(CASE WHEN cs.protection IS NOT NULL
        THEN CASE cs.protection
          WHEN 'n/a' THEN 0
          WHEN 'not_blocked' THEN 1
          WHEN 'partial' THEN 2
          WHEN 'blocked' THEN 3
        END
      END), NULL) AS best_prot_ordinal
    FROM current_steps cs
    WHERE cs.status IN ('complete', 'blocked')
    GROUP BY cs.technique_id, cs.subtechnique_id
  ),

  -- All techniques that exist in each engagement regardless of blind
  -- filter. Used to suppress the "newlyAttempted" leak: a technique
  -- hidden from blue in a blind baseline must not appear as added in
  -- a comparison against a standard retest.
  baseline_all_techs AS (
    SELECT DISTINCT
      s.technique_id,
      COALESCE(NULLIF(s.subtechnique_id, ''), '') AS subtechnique_id
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    WHERE sc.engagement_id = ?
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),
  current_all_techs AS (
    SELECT DISTINCT
      s.technique_id,
      COALESCE(NULLIF(s.subtechnique_id, ''), '') AS subtechnique_id
    FROM app.step s
    JOIN app.scenario sc ON s.scenario_id = sc.id
    WHERE sc.engagement_id = ?
      AND s.technique_id IS NOT NULL
      AND s.technique_id != ''
  ),

  -- Technique names from the content matrix (best-effort).
  names AS (
    SELECT DISTINCT
      ct.external_id,
      ct.name
    FROM content.content_technique ct
    WHERE ct.source_id = 'attack'
  ),

  -- ATT&CK version pins from both engagements.
  pins AS (
    SELECT
      (SELECT attack_version FROM app.engagement WHERE id = ?) AS baseline_pin,
      (SELECT attack_version FROM app.engagement WHERE id = ?) AS current_pin
  ),

  -- Cross-engagement match on (technique_id, subtechnique_id) with
  -- classification and leak suppression.
  paired AS (
    SELECT
      COALESCE(b.technique_id, c.technique_id)       AS technique_id,
      COALESCE(b.subtechnique_id, c.subtechnique_id) AS subtechnique_id,
      COALESCE(n.name, 'Unknown: ' || COALESCE(b.technique_id, c.technique_id)) AS name,

      -- Baseline values.
      CASE b.best_cat_ordinal
        WHEN 0 THEN 'none'
        WHEN 1 THEN 'telemetry'
        WHEN 2 THEN 'general'
        WHEN 3 THEN 'tactic'
        WHEN 4 THEN 'technique'
      END AS baseline_category,
      b.best_cat_ordinal AS baseline_category_ordinal,
      CASE b.best_prot_ordinal
        WHEN 0 THEN 'n/a'
        WHEN 1 THEN 'not_blocked'
        WHEN 2 THEN 'partial'
        WHEN 3 THEN 'blocked'
      END AS baseline_protection,

      -- Current values.
      CASE c.best_cat_ordinal
        WHEN 0 THEN 'none'
        WHEN 1 THEN 'telemetry'
        WHEN 2 THEN 'general'
        WHEN 3 THEN 'tactic'
        WHEN 4 THEN 'technique'
      END AS current_category,
      c.best_cat_ordinal AS current_category_ordinal,
      CASE c.best_prot_ordinal
        WHEN 0 THEN 'n/a'
        WHEN 1 THEN 'not_blocked'
        WHEN 2 THEN 'partial'
        WHEN 3 THEN 'blocked'
      END AS current_protection,

      -- Ordinal delta: NULL if either side is unscored.
      CASE
        WHEN b.best_cat_ordinal IS NOT NULL AND c.best_cat_ordinal IS NOT NULL
        THEN c.best_cat_ordinal - b.best_cat_ordinal
      END AS ordinal_delta,

      -- Classification.
      --
      -- Leak suppression: if a technique is absent from the baseline-side
      -- result but actually EXISTS in the baseline at the storage level
      -- (blind filter is hiding it), suppress the row entirely by returning
      -- a sentinel classification of '' — filtered in the outer SELECT.
      -- Same for noLongerAttempted on the current side.
      CASE
        WHEN b.technique_id IS NULL AND bat.technique_id IS NOT NULL THEN ''
        WHEN c.technique_id IS NULL AND cat.technique_id IS NOT NULL THEN ''
        WHEN b.technique_id IS NULL THEN 'newlyAttempted'
        WHEN c.technique_id IS NULL THEN 'noLongerAttempted'
        WHEN b.best_cat_ordinal IS NULL OR c.best_cat_ordinal IS NULL THEN 'incomparable'
        WHEN c.best_cat_ordinal > b.best_cat_ordinal THEN 'improved'
        WHEN c.best_cat_ordinal < b.best_cat_ordinal THEN 'regressed'
        -- Same category ordinal: protection tiebreaker.
        WHEN c.best_prot_ordinal > b.best_prot_ordinal THEN 'improved'
        WHEN c.best_prot_ordinal < b.best_prot_ordinal THEN 'regressed'
        ELSE 'unchanged'
      END AS classification,

      pins.baseline_pin,
      pins.current_pin

    FROM baseline_best b
    FULL OUTER JOIN current_best c
      ON b.technique_id = c.technique_id
        AND b.subtechnique_id = c.subtechnique_id
    -- Leak-suppression: left-join the storage-level technique sets.
    LEFT JOIN baseline_all_techs bat
      ON c.technique_id = bat.technique_id
        AND c.subtechnique_id = bat.subtechnique_id
    LEFT JOIN current_all_techs cat
      ON b.technique_id = cat.technique_id
        AND b.subtechnique_id = cat.subtechnique_id
    LEFT JOIN names n
      ON COALESCE(b.technique_id, c.technique_id) = n.external_id
    CROSS JOIN pins
  ),

  -- Suppress rows whose classification was cleared by the leak filter.
  visible AS (
    SELECT * FROM paired WHERE classification != ''
  )

SELECT
  v.*,
  -- Summary counts as repeated columns on every row (same pattern as
  -- TechniqueCoverage).
  (SELECT COUNT(*) FROM visible WHERE classification = 'improved')          AS improved_count,
  (SELECT COUNT(*) FROM visible WHERE classification = 'regressed')         AS regressed_count,
  (SELECT COUNT(*) FROM visible WHERE classification = 'unchanged')         AS unchanged_count,
  (SELECT COUNT(*) FROM visible WHERE classification = 'newlyAttempted')    AS newly_attempted_count,
  (SELECT COUNT(*) FROM visible WHERE classification = 'noLongerAttempted') AS no_longer_attempted_count,
  (SELECT COUNT(*) FROM visible WHERE classification = 'incomparable')      AS incomparable_count
FROM visible v
ORDER BY v.technique_id, v.subtechnique_id
`
