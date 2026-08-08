import type { EngagementRole } from './queries'

/**
 * Engagement-role predicates (M3-014).
 *
 * These answer "what can this person do in this engagement" from the role
 * returned by `GET /auth/me` memberships. They are the client-side counterpart
 * of the server's authz policy (M1-013) — not access control, but honesty:
 * hiding a button the server would refuse is better UI than showing it and
 * letting the user hit a 403.
 */

/** May manage the engagement itself: settings, status, members. */
export function canManage(role: EngagementRole): boolean {
  return role === 'lead' || role === 'admin'
}

/** May write red-side: set execution status, command, notes. */
export function canWriteRed(role: EngagementRole): boolean {
  return role === 'lead' || role === 'red'
}

/** May write blue-side: detection scoring. */
export function canWriteBlue(role: EngagementRole): boolean {
  return role === 'lead' || role === 'blue'
}

/** May write comments (observers hold this one write). */
export function canWriteComments(_role: EngagementRole): boolean {
  return true // Every role, including observer, may comment.
}

/** May see unrevealed step details in blind mode. */
export function canSeeUnrevealed(role: EngagementRole): boolean {
  return role === 'lead' || role === 'red' || role === 'admin'
}

/** May manage the workbook structure (scenarios, steps, reorder). */
export function canWriteWorkbook(role: EngagementRole): boolean {
  return role === 'lead' || role === 'red'
}

/** May create findings. */
export function canWriteFindings(role: EngagementRole): boolean {
  return role === 'lead' || role === 'red' || role === 'blue'
}
