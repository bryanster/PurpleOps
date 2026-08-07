import { describe, expect, it } from 'vitest'

import { engagementKeys } from './queries'
import { queryKeysForVerb } from './event-invalidation'

const ENG = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
const SCENARIO = 'scenario-1'
const EXEC = 'execution-1'
const STEP = 'step-1'

/** Activity rail prefix — included by most engagement-scoped verbs (M4-008). */
const ACTIVITY = engagementKeys.activityPrefix(ENG)

function keysEqual(
  a: readonly (readonly unknown[])[],
  b: readonly (readonly unknown[])[],
): boolean {
  if (a.length !== b.length) return false
  return a.every((key, i) => {
    const bk = b[i]
    if (bk === undefined || key.length !== bk.length) return false
    return key.every((v, j) => v === bk[j])
  })
}

describe('queryKeysForVerb', () => {
  // ── Engagement ────────────────────────────────────────────────────────
  it('engagement.created invalidates all', () => {
    const keys = queryKeysForVerb('engagement.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.all])).toBe(true)
  })

  it('engagement.updated invalidates detail, all, and activity', () => {
    const keys = queryKeysForVerb('engagement.updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.detail(ENG), engagementKeys.all, ACTIVITY])).toBe(true)
  })

  it('engagement.status_changed invalidates detail, all, and activity', () => {
    const keys = queryKeysForVerb('engagement.status_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.detail(ENG), engagementKeys.all, ACTIVITY])).toBe(true)
  })

  it('engagement.deleted invalidates all', () => {
    const keys = queryKeysForVerb('engagement.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.all])).toBe(true)
  })

  // ── Members ───────────────────────────────────────────────────────────
  it('member.added invalidates members and activity', () => {
    const keys = queryKeysForVerb('member.added', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG), ACTIVITY])).toBe(true)
  })

  it('member.role_changed invalidates members and activity', () => {
    const keys = queryKeysForVerb('member.role_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG), ACTIVITY])).toBe(true)
  })

  it('member.removed invalidates members and activity', () => {
    const keys = queryKeysForVerb('member.removed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG), ACTIVITY])).toBe(true)
  })

  // ── Scenarios ─────────────────────────────────────────────────────────
  it('scenario.created invalidates scenarios and activity', () => {
    const keys = queryKeysForVerb('scenario.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.scenarios(ENG), ACTIVITY])).toBe(true)
  })

  it('scenario.reordered invalidates scenarios and activity', () => {
    const keys = queryKeysForVerb('scenario.reordered', ENG, {})
    expect(keysEqual(keys, [engagementKeys.scenarios(ENG), ACTIVITY])).toBe(true)
  })

  it('scenario.imported invalidates scenarios, allSteps, and activity', () => {
    const keys = queryKeysForVerb('scenario.imported', ENG, {})
    expect(
      keysEqual(keys, [engagementKeys.scenarios(ENG), engagementKeys.allSteps(ENG), ACTIVITY]),
    ).toBe(true)
  })

  // ── Steps ─────────────────────────────────────────────────────────────
  it('step.created invalidates allSteps, scenario steps, and activity', () => {
    const keys = queryKeysForVerb('step.created', ENG, { scenarioId: SCENARIO })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        ACTIVITY,
        engagementKeys.steps(ENG, SCENARIO),
      ]),
    ).toBe(true)
  })

  it('step.created without scenarioId invalidates allSteps and activity', () => {
    const keys = queryKeysForVerb('step.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.allSteps(ENG), ACTIVITY])).toBe(true)
  })

  it('step.revealed invalidates allSteps, scenario steps, step, executions, and activity', () => {
    const keys = queryKeysForVerb('step.revealed', ENG, {
      scenarioId: SCENARIO,
      stepId: STEP,
    })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        ACTIVITY,
        engagementKeys.steps(ENG, SCENARIO),
        engagementKeys.step(ENG, SCENARIO, STEP),
        engagementKeys.executions(ENG),
      ]),
    ).toBe(true)
  })

  it('step.reordered invalidates allSteps, scenario steps, and activity', () => {
    const keys = queryKeysForVerb('step.reordered', ENG, { scenarioId: SCENARIO })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        ACTIVITY,
        engagementKeys.steps(ENG, SCENARIO),
      ]),
    ).toBe(true)
  })

  // ── Executions ────────────────────────────────────────────────────────
  it('execution.red_updated invalidates executions, detail, and activity', () => {
    const keys = queryKeysForVerb('execution.red_updated', ENG, { executionId: EXEC })
    expect(
      keysEqual(keys, [
        engagementKeys.executions(ENG),
        ACTIVITY,
        engagementKeys.execution(ENG, EXEC),
      ]),
    ).toBe(true)
  })

  it('execution.blue_updated invalidates executions, detail, and activity', () => {
    const keys = queryKeysForVerb('execution.blue_updated', ENG, { executionId: EXEC })
    expect(
      keysEqual(keys, [
        engagementKeys.executions(ENG),
        ACTIVITY,
        engagementKeys.execution(ENG, EXEC),
      ]),
    ).toBe(true)
  })

  it('execution.red_updated without executionId invalidates executions and activity', () => {
    const keys = queryKeysForVerb('execution.red_updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG), ACTIVITY])).toBe(true)
  })

  // ── Evidence ──────────────────────────────────────────────────────────
  it('evidence.uploaded invalidates evidence for the execution and activity', () => {
    const keys = queryKeysForVerb('evidence.uploaded', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.evidence(ENG, EXEC), ACTIVITY])).toBe(true)
  })

  it('evidence.deleted without executionId invalidates executions and activity', () => {
    const keys = queryKeysForVerb('evidence.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG), ACTIVITY])).toBe(true)
  })

  // ── Comments ──────────────────────────────────────────────────────────
  it('comment.created invalidates comments for the execution and activity', () => {
    const keys = queryKeysForVerb('comment.created', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.comments(ENG, EXEC), ACTIVITY])).toBe(true)
  })

  it('comment.edited invalidates comments for the execution and activity', () => {
    const keys = queryKeysForVerb('comment.edited', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.comments(ENG, EXEC), ACTIVITY])).toBe(true)
  })

  it('comment.created without executionId invalidates executions and activity', () => {
    const keys = queryKeysForVerb('comment.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG), ACTIVITY])).toBe(true)
  })

  // ── Findings ──────────────────────────────────────────────────────────
  it('finding.created invalidates findings and activity', () => {
    const keys = queryKeysForVerb('finding.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG), ACTIVITY])).toBe(true)
  })

  it('finding.updated invalidates findings and activity', () => {
    const keys = queryKeysForVerb('finding.updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG), ACTIVITY])).toBe(true)
  })

  it('finding.deleted invalidates findings and activity', () => {
    const keys = queryKeysForVerb('finding.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG), ACTIVITY])).toBe(true)
  })

  it('finding.steps_changed invalidates findings and activity', () => {
    const keys = queryKeysForVerb('finding.steps_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG), ACTIVITY])).toBe(true)
  })

  // ── Unknown ───────────────────────────────────────────────────────────
  it('unknown verb returns empty array', () => {
    const keys = queryKeysForVerb('garbage.fire', ENG, {})
    expect(keys).toEqual([])
  })

  it('empty verb returns empty array', () => {
    const keys = queryKeysForVerb('', ENG, {})
    expect(keys).toEqual([])
  })
})
