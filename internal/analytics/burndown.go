package analytics

import (
	"context"
	"database/sql"
	"fmt"
)

// DefaultBurndownPointCap is the daily-point ceiling past which the burndown
// switches from daily to weekly buckets. Documented in docs/analytics.md.
const DefaultBurndownPointCap = 90

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// BurndownInterval selects the bucket granularity for the burndown series.
type BurndownInterval string

const (
	IntervalDaily  BurndownInterval = "daily"
	IntervalWeekly BurndownInterval = "weekly"
)

// BurndownPoint is one point in the burndown series.
type BurndownPoint struct {
	Date         string `json:"date"`
	Open         int    `json:"open"`
	InProgress   int    `json:"inProgress"`
	Resolved     int    `json:"resolved"`
	AcceptedRisk int    `json:"acceptedRisk"`
	TotalOpen    int    `json:"totalOpen"`
}

// BurndownResult holds the burndown series and metadata.
type BurndownResult struct {
	Interval BurndownInterval `json:"interval"`
	Points   []BurndownPoint  `json:"points"`
}

// SeverityBucket is one severity level in the findings-by-severity snapshot.
type SeverityBucket struct {
	Severity     string `json:"severity"`
	Open         int    `json:"open"`
	InProgress   int    `json:"inProgress"`
	Resolved     int    `json:"resolved"`
	AcceptedRisk int    `json:"acceptedRisk"`
	TotalOpen    int    `json:"totalOpen"`
}

// SeveritySnapshot is the current findings breakdown by severity × status.
type SeveritySnapshot struct {
	Buckets []SeverityBucket `json:"buckets"`
}

// ---------------------------------------------------------------------------
// FindingsBurndown — daily or weekly series from finding_status_history
// ---------------------------------------------------------------------------

func (q *Queries) FindingsBurndown(ctx context.Context, scope Scope, interval BurndownInterval) (*BurndownResult, error) {
	if interval == "" {
		interval = chooseInterval(ctx, q.db.Read(), scope.EngagementID)
	}

	var query string
	switch interval {
	case IntervalWeekly:
		query = burndownWeeklyQuery
	default:
		query = burndownDailyQuery
	}

	rows, err := q.db.Read().QueryContext(ctx, query, scope.EngagementID, scope.EngagementID, scope.EngagementID, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: FindingsBurndown: %w", err)
	}
	defer rows.Close()

	var points []BurndownPoint
	for rows.Next() {
		var dateLabel string
		var op int32
		var o, ip, r, ar, to int32
		if err := rows.Scan(&dateLabel, &o, &ip, &r, &ar, &to); err != nil {
			return nil, fmt.Errorf("analytics: FindingsBurndown: scan: %w", err)
		}
		_ = op
		points = append(points, BurndownPoint{
			Date:         dateLabel,
			Open:         int(o),
			InProgress:   int(ip),
			Resolved:     int(r),
			AcceptedRisk: int(ar),
			TotalOpen:    int(to),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: FindingsBurndown: rows: %w", err)
	}

	return &BurndownResult{Interval: interval, Points: points}, nil
}

// chooseInterval reads the engagement date range and picks daily or weekly.
func chooseInterval(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, engagementID string) BurndownInterval {
	var days int32
	if err := db.QueryRowContext(ctx,
		`SELECT LEAST((SELECT ends_on FROM app.engagement WHERE id = ?), CURRENT_DATE) - (SELECT starts_on FROM app.engagement WHERE id = ?)`,
		engagementID, engagementID,
	).Scan(&days); err != nil {
		return IntervalDaily
	}
	if days > DefaultBurndownPointCap {
		return IntervalWeekly
	}
	return IntervalDaily
}

// ---------------------------------------------------------------------------
// burndownQuery — shared CTE building the spine-and-status structure.
//
// DuckDB-specific: GENERATE_SERIES, QUALIFY, DATE_TRUNC for weekly grouping.
// A portable database would need a recursive CTE for the spine and
// FIRST_VALUE/LAST_VALUE instead of QUALIFY (doc.go § Porting costs).
// ---------------------------------------------------------------------------

// burndownDailyQuery returns one point per day.
const burndownDailyQuery = `
WITH
bounds AS (
    SELECT
        (SELECT starts_on FROM app.engagement WHERE id = ?)::DATE AS start_dt,
        LEAST((SELECT ends_on FROM app.engagement WHERE id = ?)::DATE, CURRENT_DATE) AS end_dt
),
spine AS (
    SELECT UNNEST(GENERATE_SERIES(
        (SELECT start_dt FROM bounds),
        (SELECT end_dt FROM bounds),
        INTERVAL '1' DAY
    )) AS d
),
-- For each finding, get its status history as (from_date, to_date] ranges.
-- from_date is the day the status became effective; to_date is the day
-- the NEXT status became effective (or infinity for the current status).
history_ranges AS (
    SELECT
        h.finding_id,
        h.to_status,
        CAST(h.changed_at AS DATE) AS from_date,
        COALESCE(
            LEAD(CAST(h.changed_at AS DATE)) OVER (
                PARTITION BY h.finding_id ORDER BY h.changed_at
            ),
            DATE '9999-12-31'
        ) AS to_date
    FROM app.finding_status_history h
    WHERE h.engagement_id = ?
),
-- Each finding's first change date, for clamping.
finding_first AS (
    SELECT
        finding_id,
        MIN(from_date) AS first_date,
        -- The creation status (to_status of the first history row).
        FIRST(to_status ORDER BY from_date ASC) AS creation_status
    FROM history_ranges
    GROUP BY finding_id
),
-- Cross spine × findings, join to history ranges.
-- For a finding whose first history is before the spine start: it appears
-- naturally (the range covers the start date).
-- For a finding whose first history is after the spine end: we clamp it
-- at the end date with a UNION ALL below.
-- For a finding with no history at all (shouldn't happen in practice
-- because finding creation writes a row): we drop it.
spine_status AS (
    SELECT
        s.d,
        hr.finding_id,
        hr.to_status
    FROM spine s
    CROSS JOIN finding_first ff
    JOIN history_ranges hr ON hr.finding_id = ff.finding_id
        AND hr.from_date <= s.d AND s.d < hr.to_date
    WHERE ff.first_date <= (SELECT end_dt FROM bounds)

    UNION ALL

    -- Clamping: findings whose first history is after end_dt appear at end_dt
    -- with their creation status.
    SELECT
        b.end_dt,
        ff.finding_id,
        ff.creation_status
    FROM bounds b
    CROSS JOIN finding_first ff
    WHERE ff.first_date > b.end_dt
),
daily_counts AS (
    SELECT
        s.d,
        COALESCE(COUNT(CASE WHEN s.to_status = 'open' THEN 1 END), 0) AS open_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'in_progress' THEN 1 END), 0) AS in_progress_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'resolved' THEN 1 END), 0) AS resolved_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'accepted_risk' THEN 1 END), 0) AS accepted_risk_count,
        COALESCE(COUNT(CASE WHEN s.to_status IN ('open', 'in_progress') THEN 1 END), 0) AS total_open_count
    FROM spine_status s
    GROUP BY s.d
),
-- Ensure the spine always has rows: LEFT JOIN spine → daily_counts so an
-- engagement with no findings still produces zeroes.
all_dates AS (
    SELECT
        spine.d,
        COALESCE(dc.open_count, 0) AS open_count,
        COALESCE(dc.in_progress_count, 0) AS in_progress_count,
        COALESCE(dc.resolved_count, 0) AS resolved_count,
        COALESCE(dc.accepted_risk_count, 0) AS accepted_risk_count,
        COALESCE(dc.total_open_count, 0) AS total_open_count
    FROM spine
    LEFT JOIN daily_counts dc ON dc.d = spine.d
)
SELECT
    strftime(d, '%Y-%m-%d') AS date_label,
    open_count,
    in_progress_count,
    resolved_count,
    accepted_risk_count,
    total_open_count
FROM all_dates
ORDER BY d
`

// burndownWeeklyQuery returns one point per ISO week.
const burndownWeeklyQuery = `
WITH
bounds AS (
    SELECT
        (SELECT starts_on FROM app.engagement WHERE id = ?)::DATE AS start_dt,
        LEAST((SELECT ends_on FROM app.engagement WHERE id = ?)::DATE, CURRENT_DATE) AS end_dt
),
spine AS (
    SELECT UNNEST(GENERATE_SERIES(
        (SELECT start_dt FROM bounds),
        (SELECT end_dt FROM bounds),
        INTERVAL '1' DAY
    )) AS d
),
history_ranges AS (
    SELECT
        h.finding_id,
        h.to_status,
        CAST(h.changed_at AS DATE) AS from_date,
        COALESCE(
            LEAD(CAST(h.changed_at AS DATE)) OVER (
                PARTITION BY h.finding_id ORDER BY h.changed_at
            ),
            DATE '9999-12-31'
        ) AS to_date
    FROM app.finding_status_history h
    WHERE h.engagement_id = ?
),
finding_first AS (
    SELECT
        finding_id,
        MIN(from_date) AS first_date,
        FIRST(to_status ORDER BY from_date ASC) AS creation_status
    FROM history_ranges
    GROUP BY finding_id
),
spine_status AS (
    SELECT
        s.d,
        hr.finding_id,
        hr.to_status
    FROM spine s
    CROSS JOIN finding_first ff
    JOIN history_ranges hr ON hr.finding_id = ff.finding_id
        AND hr.from_date <= s.d AND s.d < hr.to_date
    WHERE ff.first_date <= (SELECT end_dt FROM bounds)

    UNION ALL

    SELECT
        b.end_dt,
        ff.finding_id,
        ff.creation_status
    FROM bounds b
    CROSS JOIN finding_first ff
    WHERE ff.first_date > b.end_dt
),
daily_counts AS (
    SELECT
        s.d,
        COALESCE(COUNT(CASE WHEN s.to_status = 'open' THEN 1 END), 0) AS open_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'in_progress' THEN 1 END), 0) AS in_progress_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'resolved' THEN 1 END), 0) AS resolved_count,
        COALESCE(COUNT(CASE WHEN s.to_status = 'accepted_risk' THEN 1 END), 0) AS accepted_risk_count,
        COALESCE(COUNT(CASE WHEN s.to_status IN ('open', 'in_progress') THEN 1 END), 0) AS total_open_count
    FROM spine_status s
    GROUP BY s.d
),
all_dates AS (
    SELECT
        spine.d,
        COALESCE(dc.open_count, 0) AS open_count,
        COALESCE(dc.in_progress_count, 0) AS in_progress_count,
        COALESCE(dc.resolved_count, 0) AS resolved_count,
        COALESCE(dc.accepted_risk_count, 0) AS accepted_risk_count,
        COALESCE(dc.total_open_count, 0) AS total_open_count
    FROM spine
    LEFT JOIN daily_counts dc ON dc.d = spine.d
),
-- Group daily results by week using DATE_TRUNC('week').
-- The date_label is the Monday of each week.
weekly AS (
    SELECT
        DATE_TRUNC('week', d) AS week_start,
        open_count,
        in_progress_count,
        resolved_count,
        accepted_risk_count,
        total_open_count
    FROM all_dates
)
SELECT
    strftime(week_start, '%Y-%m-%d') AS date_label,
    SUM(open_count) AS open_count,
    SUM(in_progress_count) AS in_progress_count,
    SUM(resolved_count) AS resolved_count,
    SUM(accepted_risk_count) AS accepted_risk_count,
    SUM(total_open_count) AS total_open_count
FROM weekly
GROUP BY week_start
ORDER BY week_start
`

// ---------------------------------------------------------------------------
// FindingsBySeverity — current snapshot by severity × status
// ---------------------------------------------------------------------------

func (q *Queries) FindingsBySeverity(ctx context.Context, scope Scope) (*SeveritySnapshot, error) {
	rows, err := q.db.Read().QueryContext(ctx, severitySnapshotQuery, scope.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("analytics: FindingsBySeverity: %w", err)
	}
	defer rows.Close()

	var buckets []SeverityBucket
	for rows.Next() {
		var b SeverityBucket
		if err := rows.Scan(&b.Severity, &b.Open, &b.InProgress, &b.Resolved, &b.AcceptedRisk, &b.TotalOpen); err != nil {
			return nil, fmt.Errorf("analytics: FindingsBySeverity: scan: %w", err)
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: FindingsBySeverity: rows: %w", err)
	}

	return &SeveritySnapshot{Buckets: buckets}, nil
}

const severitySnapshotQuery = `
SELECT
    f.severity,
    COALESCE(COUNT(CASE WHEN f.status = 'open' THEN 1 END), 0) AS open_count,
    COALESCE(COUNT(CASE WHEN f.status = 'in_progress' THEN 1 END), 0) AS in_progress_count,
    COALESCE(COUNT(CASE WHEN f.status = 'resolved' THEN 1 END), 0) AS resolved_count,
    COALESCE(COUNT(CASE WHEN f.status = 'accepted_risk' THEN 1 END), 0) AS accepted_risk_count,
    COALESCE(COUNT(CASE WHEN f.status IN ('open', 'in_progress') THEN 1 END), 0) AS total_open_count
FROM app.finding f
WHERE f.engagement_id = ?
GROUP BY f.severity
ORDER BY f.severity
`
