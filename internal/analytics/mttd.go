package analytics

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// MTTDResult — mean-time-to-detect with mandatory denominator
// ---------------------------------------------------------------------------

// MTTDResult holds the p50/p90/max MTTD percentiles over detected executions
// and the required count fields so no consumer can render MTTD without its
// denominator (M5-EPIC decision).
//
// Durations are integer seconds — never a formatted string (PLAN.md § Time).
// Percentiles are nil/absent when nothing was detected; zero means "detected
// instantly" and is a different statement.
type MTTDResult struct {
	// P50 is the 50th percentile MTTD in seconds (median). Nil when no
	// executions were detected.
	P50 *int `json:"p50,omitempty"`

	// P90 is the 90th percentile MTTD in seconds. Nil when no executions
	// were detected.
	P90 *int `json:"p90,omitempty"`

	// Max is the maximum MTTD in seconds. Nil when no executions were
	// detected.
	Max *int `json:"max,omitempty"`

	// DetectedCount is the number of attempted executions with both
	// started_at and detected_at set, and a detection category other
	// than 'none' — the denominator for the percentiles.
	DetectedCount int `json:"detectedCount"`

	// UndetectedCount is the number of attempted executions with
	// detection_category set and no detected_at, or category 'none'
	// regardless of detected_at.
	UndetectedCount int `json:"undetectedCount"`

	// UnscoredCount is the number of attempted executions blue has not
	// scored at all (detection_category IS NULL).
	UnscoredCount int `json:"unscoredCount"`

	// UnmeasurableCount is the number of detected executions where
	// started_at is NULL, so no duration exists.
	UnmeasurableCount int `json:"unmeasurableCount"`

	// AttemptedCount is the total number of attempted executions visible
	// under the scope. It equals DetectedCount + UndetectedCount +
	// UnscoredCount + UnmeasurableCount.
	AttemptedCount int `json:"attemptedCount"`
}

// ---------------------------------------------------------------------------
// MTTD — mean-time-to-detect percentiles
// ---------------------------------------------------------------------------

// MTTD returns p50/p90/max percentiles over detected executions, with the
// detected and undetected counts as required fields in the same payload.
//
// The query is a single statement (PLAN.md §5). DuckDB-specific:
// PERCENTILE_CONT — standard SQL:2003, but DuckDB provides a SIMD
// implementation. A portable database would need a window-function
// approximation or application-side sort (doc.go § Porting costs).
//
// Category 'none' counts as undetected even where a stray detected_at exists
// (M5-EPIC decision). detected_at before started_at is guarded — a negative
// duration is excluded from the percentiles and counted as unmeasurable.
func (q *Queries) MTTD(ctx context.Context, scope Scope) (*MTTDResult, error) {
	query := fmt.Sprintf(mttdQuery, scope.stepPredicate())

	var (
		detected, undetected, unscored, unmeasurable, attempted int
		p50, p90, maxMttd                                        sql.NullFloat64
	)
	if err := q.db.Read().QueryRowContext(ctx, query, scope.EngagementID).Scan(
		&attempted, &detected, &undetected, &unscored, &unmeasurable,
		&p50, &p90, &maxMttd,
	); err != nil {
		return nil, fmt.Errorf("analytics: MTTD: %w", err)
	}

	r := &MTTDResult{
		DetectedCount:    detected,
		UndetectedCount:  undetected,
		UnscoredCount:    unscored,
		UnmeasurableCount: unmeasurable,
		AttemptedCount:   attempted,
	}
	if p50.Valid {
		v := int(p50.Float64)
		r.P50 = &v
	}
	if p90.Valid {
		v := int(p90.Float64)
		r.P90 = &v
	}
	if maxMttd.Valid {
		v := int(maxMttd.Float64)
		r.Max = &v
	}
	return r, nil
}

// mttdQuery buckets every execution in the engagement into one of five
// categories, then aggregates into counts and percentiles.
//
// The %s placeholder is the blind-step predicate from scope.stepPredicate() —
// always a constant ("TRUE" or "revealed_at IS NOT NULL"), never caller input.
//
// DuckDB-specific: PERCENTILE_CONT and EPOCH. PERCENTILE_CONT is standard
// SQL:2003 but DuckDB uses its own SIMD implementation. EPOCH(timestamp)
// returns seconds as a double; the ANSI equivalent is
// EXTRACT(EPOCH FROM timestamp) which DuckDB also supports.
const mttdQuery = `
WITH attempted AS (
	SELECT e.status, e.detection_category, e.detected_at, e.started_at
	FROM app.step s
	JOIN app.execution e ON e.step_id = s.id
	WHERE s.scenario_id IN (
		SELECT sc.id FROM app.scenario sc WHERE sc.engagement_id = ?
	)
	AND %s
),
categorized AS (
	SELECT
		CASE
			WHEN status NOT IN ('complete', 'blocked') THEN 0
			WHEN detection_category IS NULL           THEN 1
			WHEN detection_category = 'none'          THEN 2
			WHEN detected_at IS NULL                  THEN 2
			WHEN started_at IS NULL                   THEN 3
			WHEN EPOCH(detected_at) - EPOCH(started_at) < 0 THEN 3
			ELSE 4
		END AS bucket,
		CASE
			WHEN status IN ('complete', 'blocked')
			 AND detection_category IS NOT NULL
			 AND detection_category != 'none'
			 AND detected_at IS NOT NULL
			 AND started_at IS NOT NULL
			 AND EPOCH(detected_at) - EPOCH(started_at) >= 0
			THEN EPOCH(detected_at) - EPOCH(started_at)
		END AS mttd_seconds
	FROM attempted
)
SELECT
	COALESCE(SUM(CASE WHEN bucket > 0 THEN 1 ELSE 0 END), 0) AS attempted_count,
	COALESCE(SUM(CASE WHEN bucket = 4 THEN 1 ELSE 0 END), 0) AS detected_count,
	COALESCE(SUM(CASE WHEN bucket = 2 THEN 1 ELSE 0 END), 0) AS undetected_count,
	COALESCE(SUM(CASE WHEN bucket = 1 THEN 1 ELSE 0 END), 0) AS unscored_count,
	COALESCE(SUM(CASE WHEN bucket = 3 THEN 1 ELSE 0 END), 0) AS unmeasurable_count,
	PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY mttd_seconds) AS p50,
	PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY mttd_seconds) AS p90,
	MAX(mttd_seconds) AS max_mttd
FROM categorized
`
