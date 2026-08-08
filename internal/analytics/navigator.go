package analytics

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/store/content"
)

// NavigatorSchemaVersion is the layer and navigator schema version pinned
// as a constant. When ATT&CK Navigator moves to a new schema version this is
// the single line that changes — it is not a query parameter, not
// per-engagement, and not per-deployment.
const NavigatorSchemaVersion = "4.5"

// NavigatorColourRamp maps detection-category ordinal 0-4 to hex colours.
// The same ramp is documented in docs/analytics.md and used by M5-013's
// heatmap — one ramp, two renderers.
var NavigatorColourRamp = [5]string{
	"#aeb3bf", // 0 — none: grey
	"#ffee58", // 1 — telemetry: yellow/amber
	"#fca128", // 2 — general: orange
	"#d13c3c", // 3 — tactic: red
	"#862121", // 4 — technique: dark red
}

// NavigatorLegendLabels are the human-readable labels for each detection
// category ordinal, matching the ramp positions above.
var NavigatorLegendLabels = [5]string{
	"None",
	"Telemetry",
	"General",
	"Tactic",
	"Technique",
}

// NavigatorLayerResult is the complete Navigator layer document. The handler
// maps this into the gen.NavigatorLayer shape for the wire.
type NavigatorLayerResult struct {
	Name           string
	Description    string
	AttackVersion  string
	Techniques     []NavigatorTechniqueEntry
	UnmatchedCount int
}

// NavigatorTechniqueEntry is one technique row in the Navigator layer.
type NavigatorTechniqueEntry struct {
	TechniqueID    string
	Score          int
	Color          string
	Comment        string
	Enabled        bool
	StepCount      int
	Protection     string
	IsSubtechnique bool
}

// NavigatorLegendItem is one entry in the layer legend.
type NavigatorLegendItem struct {
	Label string
	Color string
}

// navigatorLayerQuery returns one row per distinct technique external-id
// in the engagement visible under scope, with best-of scoring. The %s
// placeholder is the blind-step predicate from scope.stepPredicate().
//
// Unscored techniques (best_category IS NULL and not attempted) are excluded.
// Not-attempted techniques are included with enabled=false and a comment.
const navigatorLayerQuery = `
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
      ct.is_subtechnique
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
      m.external_id,
      b.best_cat_ordinal,
      b.best_prot_ordinal
  )
SELECT
  pt.external_id,
  pt.name,
  pt.is_subtechnique,
  pt.matched,
  pt.attempted,
  pt.best_category,
  pt.best_category_ordinal,
  pt.best_protection,
  pt.step_count,
  (SELECT COUNT(*) FROM per_technique WHERE NOT matched) AS unmatched_count
FROM per_technique pt
ORDER BY pt.external_id
`

// navigatorTechniqueRow is one technique row from the navigator query.
type navigatorTechniqueRow struct {
	ExternalID          string
	Name                string
	IsSubtechnique      bool
	Matched             bool
	Attempted           bool
	BestCategory        *string
	BestCategoryOrdinal *int
	BestProtection      *string
	StepCount           int
}

// NavigatorLayer builds an ATT&CK Navigator layer document for the engagement
// under the given scope.
func (q *Queries) NavigatorLayer(ctx context.Context, scope Scope) (*NavigatorLayerResult, error) {
	db := q.db.Read()

	var engName, engDesc, attackVersion string
	if err := db.QueryRowContext(ctx,
		`SELECT name, COALESCE(description, ''), attack_version FROM app.engagement WHERE id = ?`,
		scope.EngagementID,
	).Scan(&engName, &engDesc, &attackVersion); err != nil {
		return nil, fmt.Errorf("navigator: read engagement: %w", err)
	}

	pred := scope.stepPredicate()

	rows, err := db.QueryContext(ctx,
		fmt.Sprintf(navigatorLayerQuery, pred),
		scope.EngagementID,
		content.SourceIDAttack,
		scope.EngagementID,
	)
	if err != nil {
		return nil, fmt.Errorf("navigator: query techniques: %w", err)
	}
	defer rows.Close()

	var techniques []navigatorTechniqueRow
	var unmatched int
	for rows.Next() {
		var r navigatorTechniqueRow
		if err := rows.Scan(
			&r.ExternalID,
			&r.Name,
			&r.IsSubtechnique,
			&r.Matched,
			&r.Attempted,
			&r.BestCategory,
			&r.BestCategoryOrdinal,
			&r.BestProtection,
			&r.StepCount,
			&unmatched,
		); err != nil {
			return nil, fmt.Errorf("navigator: scan row: %w", err)
		}
		techniques = append(techniques, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("navigator: rows: %w", err)
	}

	// Build description with unmatched note when present.
	desc := engDesc
	if unmatched > 0 {
		if desc != "" {
			desc += "\n"
		}
		desc += fmt.Sprintf("Note: %d technique(s) referenced by steps are not present in ATT&CK v%s and are omitted.",
			unmatched, attackVersion)
	}

	entries := make([]NavigatorTechniqueEntry, 0, len(techniques))
	for _, t := range techniques {
		if entry, ok := buildEntry(t); ok {
			entries = append(entries, entry)
		}
	}

	return &NavigatorLayerResult{
		Name:           engName,
		Description:    desc,
		AttackVersion:  attackVersion,
		Techniques:     entries,
		UnmatchedCount: unmatched,
	}, nil
}

// buildEntry returns a Navigator technique entry, or false when the technique
// should be omitted:
//   - Unscored and not attempted (nobody looked — not evidence).
//   - Not present in the pinned ATT&CK version (unmatched — silently lost).
func buildEntry(t navigatorTechniqueRow) (NavigatorTechniqueEntry, bool) {
	// Unmatched: not in the pinned ATT&CK version — omit.
	if !t.Matched {
		return NavigatorTechniqueEntry{}, false
	}
	// Unscored and not attempted: omitted — nobody looked, not evidence.
	if !t.Attempted && t.BestCategory == nil {
		return NavigatorTechniqueEntry{}, false
	}

	entry := NavigatorTechniqueEntry{
		TechniqueID:    t.ExternalID,
		IsSubtechnique: t.IsSubtechnique,
	}

	if t.Attempted && t.BestCategoryOrdinal != nil {
		score := *t.BestCategoryOrdinal
		entry.Score = score
		entry.Color = NavigatorColourRamp[score]
	} else {
		entry.Score = 0
		entry.Color = ""
	}

	if t.StepCount > 0 {
		entry.Comment = fmt.Sprintf("%s — %d step(s)", t.Name, t.StepCount)
	} else {
		entry.Comment = t.Name
	}
	if t.Attempted && t.BestProtection != nil && *t.BestProtection != "" {
		entry.Comment += fmt.Sprintf(" | protection: %s", *t.BestProtection)
	}

	if t.Attempted {
		entry.Enabled = true
		entry.StepCount = t.StepCount
		if t.BestProtection != nil {
			entry.Protection = *t.BestProtection
		}
	} else {
		entry.Enabled = false
		entry.Comment = fmt.Sprintf("Not attempted — %d step(s) pending/skipped", t.StepCount)
		entry.StepCount = t.StepCount
	}

	return entry, true
}
