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

export function engagementFindingsPath(engagementId: string): string {
  return `/engagements/${engagementId}/findings`
}

export function engagementSettingsPath(engagementId: string): string {
  return `/engagements/${engagementId}/settings`
}
export function engagementReportsPath(engagementId: string): string {
  return `/engagements/${engagementId}/reports`
}

export function engagementReportPath(
  engagementId: string,
  reportId: string,
): string {
  return `/engagements/${engagementId}/reports/${reportId}`
}
