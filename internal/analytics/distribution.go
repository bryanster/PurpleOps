package analytics

import (
	"context"
	"fmt"
)

// ---------------------------------------------------------------------------
// DistributionResult — shared envelope for single-distribution rollups
// ---------------------------------------------------------------------------

// DistributionBucket is one labelled bucket with its count.
// Every bucket in the vocabulary is present, including zeroes, so a
// consumer can render a complete chart without guessing the axis.
type DistributionBucket struct {
	Label string
	Count int
}

// DistributionResult bundles the bucket-by-bucket breakdown with the
// attempted count a consumer needs to state the denominator.
type DistributionResult struct {
	Attempted int
	Buckets   []DistributionBucket
}

// ---------------------------------------------------------------------------
// CategoryDistribution — detection category counts
// ---------------------------------------------------------------------------

// CategoryDistribution returns counts per detection category across
// attempted executions visible under scope, plus an unscored bucket
// (NULL category on an attempted execution) and a notAttempted count.
//
// The query is a single statement (PLAN.md §5).
func (q *Queries) CategoryDistribution(ctx context.Context, scope Scope) (*DistributionResult, error) {
	query := fmt.Sprintf(categoryDistQuery, scope.stepPredicate(), scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query, scope.EngagementID, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: category distribution: %w", err)
	}
	defer rows.Close()

	result := &DistributionResult{}
	catCounts := map[string]int{
		"none": 0, "telemetry": 0, "general": 0, "tactic": 0, "technique": 0, "unscored": 0,
	}
	var notAttempted int
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("analytics: scanning category distribution: %w", err)
		}
		switch label {
		case "notAttempted":
			notAttempted = count
		default:
			catCounts[label] = count
			result.Attempted += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating category distribution: %w", err)
	}

	result.Buckets = []DistributionBucket{
		{Label: "none", Count: catCounts["none"]},
		{Label: "telemetry", Count: catCounts["telemetry"]},
		{Label: "general", Count: catCounts["general"]},
		{Label: "tactic", Count: catCounts["tactic"]},
		{Label: "technique", Count: catCounts["technique"]},
		{Label: "unscored", Count: catCounts["unscored"]},
		{Label: "notAttempted", Count: notAttempted},
	}
	return result, nil
}

// categoryDistQuery returns one row per detection category plus
// notAttempted. The %s placeholder is the blind-step predicate, used
// twice — once for the attempted aggregate and once for notAttempted.
//
// DuckDB-specific: none. The CASE is ANSI-standard.
const categoryDistQuery = `
SELECT
    CASE WHEN execution.detection_category IS NULL THEN 'unscored'
         ELSE execution.detection_category
    END AS label,
    COUNT(*) AS count
FROM app.execution
JOIN app.step ON app.step.id = app.execution.step_id
WHERE app.step.scenario_id IN (
    SELECT id FROM app.scenario WHERE engagement_id = ?
)
AND (` + attemptedPredicate + `)
AND (%s)
GROUP BY label

UNION ALL

SELECT
    'notAttempted' AS label,
    COUNT(*) AS count
FROM app.execution
JOIN app.step ON app.step.id = app.execution.step_id
WHERE app.step.scenario_id IN (
    SELECT id FROM app.scenario WHERE engagement_id = ?
)
AND NOT (` + attemptedPredicate + `)
AND (%s)
`

// ---------------------------------------------------------------------------
// ProtectionRate — protection counts
// ---------------------------------------------------------------------------

// ProtectionRate returns counts per protection level across attempted
// executions visible under scope, plus an unscored bucket (NULL
// protection on an attempted execution).
//
// The query is a single statement (PLAN.md §5).
func (q *Queries) ProtectionRate(ctx context.Context, scope Scope) (*DistributionResult, error) {
	query := fmt.Sprintf(protectionRateQuery, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: protection rate: %w", err)
	}
	defer rows.Close()

	result := &DistributionResult{}
	protCounts := map[string]int{
		"blocked": 0, "partial": 0, "not_blocked": 0, "n/a": 0, "unscored": 0,
	}
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("analytics: scanning protection rate: %w", err)
		}
		protCounts[label] = count
		result.Attempted += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating protection rate: %w", err)
	}

	result.Buckets = []DistributionBucket{
		{Label: "blocked", Count: protCounts["blocked"]},
		{Label: "partial", Count: protCounts["partial"]},
		{Label: "not_blocked", Count: protCounts["not_blocked"]},
		{Label: "n/a", Count: protCounts["n/a"]},
		{Label: "unscored", Count: protCounts["unscored"]},
	}
	return result, nil
}

// protectionRateQuery returns one row per protection level for attempted
// executions. The %s placeholder is the blind-step predicate.
//
// DuckDB-specific: none.
const protectionRateQuery = `
SELECT
    COALESCE(execution.protection, 'unscored') AS label,
    COUNT(*) AS count
FROM app.execution
JOIN app.step ON app.step.id = app.execution.step_id
WHERE app.step.scenario_id IN (
    SELECT id FROM app.scenario WHERE engagement_id = ?
)
AND (` + attemptedPredicate + `)
AND (%s)
GROUP BY label
`

// ---------------------------------------------------------------------------
// OutcomeMix — derived outcome counts
// ---------------------------------------------------------------------------

// OutcomeMix returns counts per derived outcome (prevented, detected,
// not_detected, not_applicable) across attempted executions visible
// under scope, plus an unscored bucket.
//
// The query is a single statement (PLAN.md §5). It composes outcomeCase
// from [sqlfragments.go] rather than reimplementing the matrix.
func (q *Queries) OutcomeMix(ctx context.Context, scope Scope) (*DistributionResult, error) {
	query := fmt.Sprintf(outcomeMixQuery, outcomeCase, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: outcome mix: %w", err)
	}
	defer rows.Close()

	result := &DistributionResult{}
	outcomeCounts := map[string]int{
		"prevented": 0, "detected": 0, "not_detected": 0, "not_applicable": 0, "unscored": 0,
	}
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("analytics: scanning outcome mix: %w", err)
		}
		outcomeCounts[label] = count
		result.Attempted += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating outcome mix: %w", err)
	}

	result.Buckets = []DistributionBucket{
		{Label: "prevented", Count: outcomeCounts["prevented"]},
		{Label: "detected", Count: outcomeCounts["detected"]},
		{Label: "not_detected", Count: outcomeCounts["not_detected"]},
		{Label: "not_applicable", Count: outcomeCounts["not_applicable"]},
		{Label: "unscored", Count: outcomeCounts["unscored"]},
	}
	return result, nil
}

// outcomeMixQuery returns one row per derived outcome for attempted
// executions. The first %s is outcomeCase, the second is the blind-step
// predicate.
const outcomeMixQuery = `
SELECT
    %s AS label,
    COUNT(*) AS count
FROM app.execution
JOIN app.step ON app.step.id = app.execution.step_id
WHERE app.step.scenario_id IN (
    SELECT id FROM app.scenario WHERE engagement_id = ?
)
AND (` + attemptedPredicate + `)
AND (%s)
GROUP BY label
`

// ---------------------------------------------------------------------------
// ModifierDistribution — detection modifier counts
// ---------------------------------------------------------------------------

// ModifierDistribution returns counts per detection modifier across
// attempted, scored executions (detection_category IS NOT NULL) visible
// under scope.
//
// Counts are explicitly non-exclusive — one execution can carry several
// modifiers, so the sum of bucket counts does NOT equal the attempted
// count. The result carries [DistributionResult.Attempted] so consumers
// can note the denominator.
//
// The query is a single statement (PLAN.md §5).
// DuckDB-specific: unnest to explode the JSON array returned by
// json_extract_string with '$[*]'. A portable database would need a
// recursive CTE or application-side aggregation (doc.go § Porting costs).
func (q *Queries) ModifierDistribution(ctx context.Context, scope Scope) (*DistributionResult, error) {
	query := fmt.Sprintf(modifierDistQuery, scope.stepPredicate())

	rows, err := q.db.Read().QueryContext(ctx, query, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: modifier distribution: %w", err)
	}
	defer rows.Close()

	// Collect modifier counts; unknown modifiers land in "other".
	modCounts := map[string]int{}
	known := map[string]bool{
		"alert": true, "correlated": true, "delayed": true,
		"config_change": true, "residual_artifact": true,
	}
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("analytics: scanning modifier distribution: %w", err)
		}
		if !known[label] {
			label = "other"
		}
		modCounts[label] += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterating modifier distribution: %w", err)
	}

	// Compute attempted — the count of attempted, scored executions.
	var attempted int
	if err := q.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.execution
		 JOIN app.step ON app.step.id = app.execution.step_id
		 WHERE app.step.scenario_id IN (
		     SELECT id FROM app.scenario WHERE engagement_id = ?
		 )
		 AND (`+attemptedPredicate+`)
		 AND (`+scope.stepPredicate()+`)
		 AND execution.detection_category IS NOT NULL`,
		scope.EngagementID,
	).Scan(&attempted); err != nil {
		return nil, fmt.Errorf("analytics: counting scored for modifier distribution: %w", err)
	}

	// Build buckets: known modifiers in vocabulary order, "other" last.
	buckets := []DistributionBucket{
		{Label: "alert", Count: modCounts["alert"]},
		{Label: "correlated", Count: modCounts["correlated"]},
		{Label: "delayed", Count: modCounts["delayed"]},
		{Label: "config_change", Count: modCounts["config_change"]},
		{Label: "residual_artifact", Count: modCounts["residual_artifact"]},
		{Label: "other", Count: modCounts["other"]},
	}

	return &DistributionResult{
		Attempted: attempted,
		Buckets:   buckets,
	}, nil
}

// modifierDistQuery unnests detection_modifiers and counts each one.
// Only scored, attempted executions (detection_category IS NOT NULL) are
// included. The %s placeholder is the blind-step predicate.
//
// DuckDB-specific: LATERAL UNNEST to explode the JSON array returned by
// json_extract_string with '$[*]'. Empty JSON arrays produce an empty list,
// which LATERAL UNNEST expands to zero rows — they contribute zero to
// every bucket, which is correct: no modifiers means no modifier counts.
const modifierDistQuery = `
SELECT
    t.modifier AS label,
    COUNT(*) AS count
FROM app.execution
JOIN app.step ON app.step.id = app.execution.step_id,
     LATERAL (SELECT UNNEST(json_extract_string(app.execution.detection_modifiers, '$[*]'))) AS t(modifier)
WHERE app.step.scenario_id IN (
    SELECT id FROM app.scenario WHERE engagement_id = ?
)
AND (` + attemptedPredicate + `)
AND (%s)
AND app.execution.detection_category IS NOT NULL
GROUP BY t.modifier
`
