/**
 * Engagement routes (M3-014).
 *
 * Kept as constants so the route table, the nav, and the engagement list links
 * all spell the same path.
 */
export const ENGAGEMENTS_PATH = '/engagements'

export function engagementPath(engagementId: string): string {
  return `/engagements/${engagementId}`
}
export function engagementAnalyticsPath(engagementId: string): string {
  return `/engagements/${engagementId}/analytics`
}

export function engagementComparePath(engagementId: string, baselineId: string): string {
  return `/engagements/${engagementId}/analytics/compare?baseline=${encodeURIComponent(baselineId)}`
}

export function engagementWorkbookPath(engagementId: string): string {
  return `/engagements/${engagementId}/workbook`
}

/**
 * Library hand-off contract.
 *
 * The content library can name a plan/template/technique but has no engagement
 * in scope, so "use in scenario" sends the operator here and the workbook opens
 * the matching dialog pre-filled. The params are consumed and stripped on
 * arrival — a reload must not re-open the dialog.
 */
export type WorkbookUseKind = 'plan' | 'procedure' | 'technique'

export const WORKBOOK_USE_PARAM = 'use'
export const WORKBOOK_USE_ID_PARAM = 'useId'

export function engagementWorkbookUsePath(
  engagementId: string,
  kind: WorkbookUseKind,
  id: string,
): string {
  const params = new URLSearchParams({
    [WORKBOOK_USE_PARAM]: kind,
    [WORKBOOK_USE_ID_PARAM]: id,
  })
  return `${engagementWorkbookPath(engagementId)}?${params.toString()}`
}

export function parseWorkbookUseKind(value: string | null): WorkbookUseKind | null {
  return value === 'plan' || value === 'procedure' || value === 'technique' ? value : null
}

export function engagementFindingsPath(engagementId: string): string {
  return `/engagements/${engagementId}/findings`
}

export function engagementSettingsPath(engagementId: string): string {
  return `/engagements/${engagementId}/settings`
}
export function engagementReportsPath(engagementId: string): string {
  return `/engagements/${engagementId}/reports`
}

export function engagementReportPath(engagementId: string, reportId: string): string {
  return `/engagements/${engagementId}/reports/${reportId}`
}
