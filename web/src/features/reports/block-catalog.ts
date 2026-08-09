/**
 * Client-side block catalogue (M6-013).
 *
 * Maps the 14 registered block ids from the server registry to the labels
 * and param shapes the builder UI needs. The ids match `internal/report/blockids.go`.
 * The server is authoritative (registry validation rejects unknown ids), so this
 * catalogue is documentation for the UI, not a security boundary.
 */

export interface BlockCatalogEntry {
  id: string
  title: string
  description: string
  /** Whether this block allows in templates (most do; page_break doesn't). */
  allowInTemplate: boolean
  /** Whether this block is omitted when publish has includeEvidence=false. */
  needsEvidenceOptIn: boolean
}

// Block id constants matching server-side blockids.go.
export const BLOCK_IDS = {
  COVER: 'cover',
  EXECUTIVE_SUMMARY: 'executive_summary',
  SCOPE_ROE: 'scope_roe',
  RICH_TEXT: 'rich_text',
  PAGE_BREAK: 'page_break',
  COVERAGE_HEATMAP: 'coverage_heatmap',
  TACTIC_SCORECARD: 'tactic_scorecard',
  DETECTION_DISTRIBUTION: 'detection_distribution',
  DETECTION_GAPS: 'detection_gaps',
  MTTD: 'mttd',
  ENGAGEMENT_COMPARE: 'engagement_compare',
  SCENARIO_WALKTHROUGH: 'scenario_walkthrough',
  FINDINGS_BACKLOG: 'findings_backlog',
  EVIDENCE_APPENDIX: 'evidence_appendix',
} as const

export type BlockId = (typeof BLOCK_IDS)[keyof typeof BLOCK_IDS]

/** Stable catalogue order matching server `AllBlockIDs()`. */
const CATALOGUE: BlockCatalogEntry[] = [
  {
    id: BLOCK_IDS.COVER,
    title: 'Cover',
    description: 'Title page with engagement name, client, dates, and branding.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.EXECUTIVE_SUMMARY,
    title: 'Executive summary',
    description: 'Free-text summary of the assessment, findings, and recommendations.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.SCOPE_ROE,
    title: 'Scope & rules of engagement',
    description: 'Defines what was tested, when, and under what constraints.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.RICH_TEXT,
    title: 'Rich text',
    description: 'Free-form section with headings, lists, links, and formatting.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.PAGE_BREAK,
    title: 'Page break',
    description: 'Forces a page break in PDF output.',
    allowInTemplate: false,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.COVERAGE_HEATMAP,
    title: 'Coverage heatmap',
    description: 'ATT&CK technique × tactic heatmap with detection coverage.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.TACTIC_SCORECARD,
    title: 'Per-tactic scorecard',
    description: 'One card per tactic: attempted, detected, prevented counts and percentages.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.DETECTION_DISTRIBUTION,
    title: 'Detection distribution',
    description: 'Breakdown of detection categories across all techniques evaluated.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.DETECTION_GAPS,
    title: 'Detection gaps',
    description: 'Techniques with no detection coverage, sorted by risk/relevance.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.MTTD,
    title: 'MTTD analysis',
    description: 'Mean-time-to-detect percentiles with detected/undetected counts.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.ENGAGEMENT_COMPARE,
    title: 'Engagement comparison',
    description: 'Compare coverage, detections, and outcomes against a baseline engagement.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.SCENARIO_WALKTHROUGH,
    title: 'Scenario walkthrough',
    description: 'Per-scenario narrative with step-by-step red/blue results.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.FINDINGS_BACKLOG,
    title: 'Findings backlog',
    description: 'All findings with severity, status, and linked techniques.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
  },
  {
    id: BLOCK_IDS.EVIDENCE_APPENDIX,
    title: 'Evidence appendix',
    description: 'Collected evidence files (screenshots, logs) — opt-in on publish.',
    allowInTemplate: true,
    needsEvidenceOptIn: true,
  },
]

export function getBlockEntry(id: string): BlockCatalogEntry | undefined {
  return CATALOGUE.find((b) => b.id === id)
}

export function getBlockTitle(id: string): string {
  return getBlockEntry(id)?.title ?? id
}

/** The full catalogue in display order. */
export function getCatalog(): BlockCatalogEntry[] {
  return CATALOGUE
}
