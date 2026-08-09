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


/**
 * Share claim and view routes (M6-014).
 *
 * These are public-ish routes — they live outside the app shell and are
 * served without engagement chrome.
 */

export const CLAIM_PATH = '/claim/:token'
export const VIEW_PATH = '/view/:token'

export function claimPath(token: string): string {
  return `/claim/${token}`
}

export function viewPath(token: string): string {
  return `/view/${token}`
}