package report

// Block IDs — stable identifiers for every v1 report block. The catalogue is
// locked by M6-EPIC; do not add, remove, or rename without an epic amendment.

const (
	IDCover                 ID = "cover"
	IDExecutiveSummary      ID = "executive_summary"
	IDScopeRoE              ID = "scope_roe"
	IDRichText              ID = "rich_text"
	IDPageBreak             ID = "page_break"
	IDCoverageHeatmap       ID = "coverage_heatmap"
	IDTacticScorecard       ID = "tactic_scorecard"
	IDDetectionDistribution ID = "detection_distribution"
	IDDetectionGaps         ID = "detection_gaps"
	IDMTTD                  ID = "mttd"
	IDEngagementCompare     ID = "engagement_compare"
	IDScenarioWalkthrough   ID = "scenario_walkthrough"
	IDFindingsBacklog       ID = "findings_backlog"
	IDEvidenceAppendix      ID = "evidence_appendix"
)

// AllBlockIDs returns every registered block ID in the stable catalogue order.
func AllBlockIDs() []ID {
	return []ID{
		IDCover,
		IDExecutiveSummary,
		IDScopeRoE,
		IDRichText,
		IDPageBreak,
		IDCoverageHeatmap,
		IDTacticScorecard,
		IDDetectionDistribution,
		IDDetectionGaps,
		IDMTTD,
		IDEngagementCompare,
		IDScenarioWalkthrough,
		IDFindingsBacklog,
		IDEvidenceAppendix,
	}
}
