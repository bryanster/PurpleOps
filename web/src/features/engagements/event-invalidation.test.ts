import { describe, expect, it } from 'vitest'

import { engagementKeys } from './queries'
import { queryKeysForVerb } from './event-invalidation'

const ENG = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
const SCENARIO = 'scenario-1'
const EXEC = 'execution-1'
const STEP = 'step-1'

function keysEqual(
  a: ReadonlyArray<readonly unknown[]>,
  b: ReadonlyArray<readonly unknown[]>,
): boolean {
  if (a.length !== b.length) return false
  return a.every((key, i) => {
    const bk = b[i]
    if (key.length !== bk.length) return false
    return key.every((v, j) => v === bk[j])
  })
}

describe('queryKeysForVerb', () => {
  // ── Engagement ────────────────────────────────────────────────────────
  it('engagement.created invalidates all', () => {
    const keys = queryKeysForVerb('engagement.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.all])).toBe(true)
  })

  it('engagement.updated invalidates detail and all', () => {
    const keys = queryKeysForVerb('engagement.updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.detail(ENG), engagementKeys.all])).toBe(true)
  })

  it('engagement.status_changed invalidates detail and all', () => {
    const keys = queryKeysForVerb('engagement.status_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.detail(ENG), engagementKeys.all])).toBe(true)
  })

  it('engagement.deleted invalidates all', () => {
    const keys = queryKeysForVerb('engagement.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.all])).toBe(true)
  })

  // ── Members ───────────────────────────────────────────────────────────
  it('member.added invalidates members', () => {
    const keys = queryKeysForVerb('member.added', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG)])).toBe(true)
  })

  it('member.role_changed invalidates members', () => {
    const keys = queryKeysForVerb('member.role_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG)])).toBe(true)
  })

  it('member.removed invalidates members', () => {
    const keys = queryKeysForVerb('member.removed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.members(ENG)])).toBe(true)
  })

  // ── Scenarios ─────────────────────────────────────────────────────────
  it('scenario.created invalidates scenarios', () => {
    const keys = queryKeysForVerb('scenario.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.scenarios(ENG)])).toBe(true)
  })

  it('scenario.reordered invalidates scenarios', () => {
    const keys = queryKeysForVerb('scenario.reordered', ENG, {})
    expect(keysEqual(keys, [engagementKeys.scenarios(ENG)])).toBe(true)
  })

  it('scenario.imported invalidates scenarios and allSteps', () => {
    const keys = queryKeysForVerb('scenario.imported', ENG, {})
    expect(
      keysEqual(keys, [engagementKeys.scenarios(ENG), engagementKeys.allSteps(ENG)]),
    ).toBe(true)
  })

  // ── Steps ─────────────────────────────────────────────────────────────
  it('step.created invalidates allSteps and scenario steps', () => {
    const keys = queryKeysForVerb('step.created', ENG, { scenarioId: SCENARIO })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        engagementKeys.steps(ENG, SCENARIO),
      ]),
    ).toBe(true)
  })

  it('step.created without scenarioId invalidates allSteps only', () => {
    const keys = queryKeysForVerb('step.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.allSteps(ENG)])).toBe(true)
  })

  it('step.revealed invalidates allSteps, scenario steps, step, and executions', () => {
    const keys = queryKeysForVerb('step.revealed', ENG, {
      scenarioId: SCENARIO,
      stepId: STEP,
    })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        engagementKeys.steps(ENG, SCENARIO),
        engagementKeys.step(ENG, SCENARIO, STEP),
        engagementKeys.executions(ENG),
      ]),
    ).toBe(true)
  })

  it('step.reordered invalidates allSteps and scenario steps', () => {
    const keys = queryKeysForVerb('step.reordered', ENG, { scenarioId: SCENARIO })
    expect(
      keysEqual(keys, [
        engagementKeys.allSteps(ENG),
        engagementKeys.steps(ENG, SCENARIO),
      ]),
    ).toBe(true)
  })

  // ── Executions ────────────────────────────────────────────────────────
  it('execution.red_updated invalidates executions and detail', () => {
    const keys = queryKeysForVerb('execution.red_updated', ENG, { executionId: EXEC })
    expect(
      keysEqual(keys, [
        engagementKeys.executions(ENG),
        engagementKeys.execution(ENG, EXEC),
      ]),
    ).toBe(true)
  })

  it('execution.blue_updated invalidates executions and detail', () => {
    const keys = queryKeysForVerb('execution.blue_updated', ENG, { executionId: EXEC })
    expect(
      keysEqual(keys, [
        engagementKeys.executions(ENG),
        engagementKeys.execution(ENG, EXEC),
      ]),
    ).toBe(true)
  })

  it('execution.red_updated without executionId invalidates executions only', () => {
    const keys = queryKeysForVerb('execution.red_updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG)])).toBe(true)
  })

  // ── Evidence ──────────────────────────────────────────────────────────
  it('evidence.uploaded invalidates evidence for the execution', () => {
    const keys = queryKeysForVerb('evidence.uploaded', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.evidence(ENG, EXEC)])).toBe(true)
  })

  it('evidence.deleted without executionId invalidates executions', () => {
    const keys = queryKeysForVerb('evidence.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG)])).toBe(true)
  })

  // ── Comments ──────────────────────────────────────────────────────────
  it('comment.created invalidates comments for the execution', () => {
    const keys = queryKeysForVerb('comment.created', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.comments(ENG, EXEC)])).toBe(true)
  })

  it('comment.edited invalidates comments for the execution', () => {
    const keys = queryKeysForVerb('comment.edited', ENG, { executionId: EXEC })
    expect(keysEqual(keys, [engagementKeys.comments(ENG, EXEC)])).toBe(true)
  })

  it('comment.created without executionId invalidates executions', () => {
    const keys = queryKeysForVerb('comment.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.executions(ENG)])).toBe(true)
  })

  // ── Findings ──────────────────────────────────────────────────────────
  it('finding.created invalidates findings', () => {
    const keys = queryKeysForVerb('finding.created', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG)])).toBe(true)
  })

  it('finding.updated invalidates findings', () => {
    const keys = queryKeysForVerb('finding.updated', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG)])).toBe(true)
  })

  it('finding.deleted invalidates findings', () => {
    const keys = queryKeysForVerb('finding.deleted', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG)])).toBe(true)
  })

  it('finding.steps_changed invalidates findings', () => {
    const keys = queryKeysForVerb('finding.steps_changed', ENG, {})
    expect(keysEqual(keys, [engagementKeys.findings(ENG)])).toBe(true)
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
