import { describe, expect, test } from 'vitest'

import {
  canManage,
  canSeeUnrevealed,
  canWriteBlue,
  canWriteComments,
  canWriteFindings,
  canWriteRed,
  canWriteWorkbook,
} from './roles'

/**
 * The predicates in roles.ts are the client-side reading of
 * internal/authz/policy.go. When the two disagree the UI either shows a button
 * the server refuses (a 403 in the user's face) or hides one it would have
 * accepted (a feature that looks missing) — the second is how the workbook
 * toolbar went absent for platform administrators.
 *
 * Every engagement-scoped rule in the server table lists `Platform: admins`,
 * so 'admin' holds every write below.
 */
describe('engagement role predicates', () => {
  test.each([
    ['lead', true],
    ['red', false],
    ['blue', false],
    ['observer', false],
    ['admin', true],
  ] as const)('canManage(%s) === %s', (role, expected) => {
    expect(canManage(role)).toBe(expected)
  })

  test.each([
    ['lead', true],
    ['red', true],
    ['blue', false],
    ['observer', false],
    ['admin', true],
  ] as const)('canWriteRed(%s) === %s', (role, expected) => {
    expect(canWriteRed(role)).toBe(expected)
  })

  test.each([
    ['lead', true],
    ['red', false],
    ['blue', true],
    ['observer', false],
    ['admin', true],
  ] as const)('canWriteBlue(%s) === %s', (role, expected) => {
    expect(canWriteBlue(role)).toBe(expected)
  })

  // workbook.write — Engagement: leadAndRed, Platform: admins.
  test.each([
    ['lead', true],
    ['red', true],
    ['blue', false],
    ['observer', false],
    ['admin', true],
  ] as const)('canWriteWorkbook(%s) === %s', (role, expected) => {
    expect(canWriteWorkbook(role)).toBe(expected)
  })

  // finding.write — Engagement: writers (not observer), Platform: admins.
  test.each([
    ['lead', true],
    ['red', true],
    ['blue', true],
    ['observer', false],
    ['admin', true],
  ] as const)('canWriteFindings(%s) === %s', (role, expected) => {
    expect(canWriteFindings(role)).toBe(expected)
  })

  test.each([
    ['lead', true],
    ['red', true],
    ['blue', false],
    ['observer', false],
    ['admin', true],
  ] as const)('canSeeUnrevealed(%s) === %s', (role, expected) => {
    expect(canSeeUnrevealed(role)).toBe(expected)
  })

  // comment.write — Engagement: allMembers. The observer's one write.
  test.each(['lead', 'red', 'blue', 'observer', 'admin'] as const)(
    'canWriteComments(%s) is true',
    (role) => {
      expect(canWriteComments(role)).toBe(true)
    },
  )
})
