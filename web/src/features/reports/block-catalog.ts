/**
 * Client-side block catalogue (M6-013).
 *
 * Maps the 14 registered block ids from the server registry to the labels
 * and param shapes the builder UI needs. The ids match `internal/report/blockids.go`.
 * The server is authoritative (registry validation rejects unknown ids), so this
 * catalogue is documentation for the UI, not a security boundary.
 */

/** A block's parameter values, as stored and as sent back on save. */
export type BlockParams = Record<string, unknown>

/**
 * One editable parameter, mirroring the server's `ParamProperty` for the same
 * key. `kind` is the editor to render; the server's JSON type follows from it
 * (html/text/textarea → string, toggle → boolean, integer → integer).
 *
 * The `name` must match the server schema exactly: `ValidateParams` rejects
 * unknown keys, so a field the builder spells differently is a 400 on save and
 * not a silently ignored value.
 */
export interface BlockParamField {
  name: string
  label: string
  kind: 'html' | 'text' | 'textarea' | 'toggle' | 'integer' | 'select' | 'engagement'
  /** Helper text under the field. */
  help?: string
  placeholder?: string
  /** For `select`: the closed set the server's Enum declares. */
  options?: { value: string; label: string }[]
}

export interface BlockCatalogEntry {
  id: string
  title: string
  description: string
  /** Whether this block allows in templates (most do; page_break doesn't). */
  allowInTemplate: boolean
  /** Whether this block is omitted when publish has includeEvidence=false. */
  needsEvidenceOptIn: boolean
  /** Editable parameters, in the order the builder shows them. */
  params?: BlockParamField[]
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
    params: [
      {
        name: 'title',
        label: 'Title',
        kind: 'text',
        help: 'Defaults to the engagement name.',
        placeholder: 'Engagement name',
      },
      { name: 'subtitle', label: 'Subtitle', kind: 'text' },
      { name: 'showDate', label: 'Show the engagement dates', kind: 'toggle' },
      { name: 'showLogo', label: 'Show the branding logo', kind: 'toggle' },
    ],
  },
  {
    id: BLOCK_IDS.EXECUTIVE_SUMMARY,
    title: 'Executive summary',
    description: 'Free-text summary of the assessment, findings, and recommendations.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
    params: [{ name: 'body', label: 'Summary', kind: 'html' }],
  },
  {
    id: BLOCK_IDS.SCOPE_ROE,
    title: 'Scope & rules of engagement',
    description: 'Defines what was tested, when, and under what constraints.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
    params: [
      { name: 'body', label: 'Scope narrative', kind: 'html' },
      {
        name: 'systems',
        label: 'In-scope systems',
        kind: 'textarea',
        help: 'One per line.',
        placeholder: 'dc01.corp.example.com',
      },
    ],
  },
  {
    id: BLOCK_IDS.RICH_TEXT,
    title: 'Rich text',
    description: 'Free-form section with headings, lists, links, and formatting.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
    params: [{ name: 'html', label: 'Content', kind: 'html' }],
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
    params: [
      {
        name: 'verbosity',
        label: 'Detail level',
        kind: 'select',
        options: [
          { value: 'summary', label: 'Summary — tactic level' },
          { value: 'full', label: 'Full — individual techniques' },
        ],
      },
    ],
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
    params: [
      {
        name: 'maxRows',
        label: 'Maximum rows per section',
        kind: 'integer',
        help: 'Defaults to 50.',
      },
    ],
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
    params: [
      {
        name: 'baselineEngagementId',
        label: 'Baseline engagement',
        kind: 'engagement',
        help: 'Required — the engagement this report is compared against.',
      },
    ],
  },
  {
    id: BLOCK_IDS.SCENARIO_WALKTHROUGH,
    title: 'Scenario walkthrough',
    description: 'Per-scenario narrative with step-by-step red/blue results.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
    params: [
      {
        name: 'verbosity',
        label: 'Detail level',
        kind: 'select',
        options: [
          { value: 'summary', label: 'Summary' },
          { value: 'full', label: 'Full — every step' },
        ],
      },
    ],
  },
  {
    id: BLOCK_IDS.FINDINGS_BACKLOG,
    title: 'Findings backlog',
    description: 'All findings with severity, status, and linked techniques.',
    allowInTemplate: true,
    needsEvidenceOptIn: false,
    params: [
      {
        name: 'includeResolved',
        label: 'Include resolved and accepted-risk findings',
        kind: 'toggle',
      },
    ],
  },
  {
    id: BLOCK_IDS.EVIDENCE_APPENDIX,
    title: 'Evidence appendix',
    description: 'Collected evidence files (screenshots, logs) — opt-in on publish.',
    allowInTemplate: true,
    needsEvidenceOptIn: true,
    params: [
      { name: 'limit', label: 'Maximum items', kind: 'integer', help: 'Defaults to 50.' },
      { name: 'imagesOnly', label: 'Images only', kind: 'toggle' },
    ],
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
