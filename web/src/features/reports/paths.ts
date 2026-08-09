/**
 * Report routes (M6-013).
 *
 * Kept as constants so the route table, engagement nav, and report links
 * all spell the same path.
 */

export function engagementReportsPath(engagementId: string): string {
  return `/engagements/${engagementId}/reports`
}

export function engagementReportPath(engagementId: string, reportId: string): string {
  return `/engagements/${engagementId}/reports/${reportId}`
}
