import type { QueryClient } from '@tanstack/react-query'

import { engagementKeys } from './queries'

/**
 * Payload shape inside a hub Event envelope for engagement-scoped activity.
 *
 * Every engagement event has at least `engagementId`.  Verb-specific parent
 * ids (`executionId`, `scenarioId`, `stepId`) are extracted from the payload
 * when present.
 */
export interface EngagementEventPayload {
  engagementId: string
  actorId?: string
  verb?: string
  objectType?: string
  objectId?: string
  executionId?: string
  scenarioId?: string
  stepId?: string
}

/**
 * Return the query keys that should be invalidated when `verb` fires on an
 * engagement.  The caller passes the engagement id and any parent ids from
 * the event payload.
 *
 * Unknown verbs invalidate nothing — the caller MAY log a dev warning.
 */
export function queryKeysForVerb(
  verb: string,
  engagementId: string,
  parents: { executionId?: string; scenarioId?: string; stepId?: string },
): ReadonlyArray<readonly unknown[]> {
  const activityKeys = [engagementKeys.activityPrefix(engagementId)]

  switch (verb) {
    // ── Engagement ──────────────────────────────────────────────────────
    case 'engagement.created':
    case 'engagement.deleted':
      return [engagementKeys.all]

    case 'engagement.updated':
    case 'engagement.status_changed':
      return [engagementKeys.detail(engagementId), engagementKeys.all, ...activityKeys]

    // ── Members ─────────────────────────────────────────────────────────
    case 'member.added':
    case 'member.role_changed':
    case 'member.removed':
      return [engagementKeys.members(engagementId), ...activityKeys]

    // ── Scenarios ───────────────────────────────────────────────────────
    case 'scenario.created':
    case 'scenario.updated':
    case 'scenario.deleted':
    case 'scenario.reordered':
      return [engagementKeys.scenarios(engagementId), ...activityKeys]

    case 'scenario.imported':
      return [
        engagementKeys.scenarios(engagementId),
        engagementKeys.allSteps(engagementId),
        ...activityKeys,
      ]

    // ── Steps ───────────────────────────────────────────────────────────
    case 'step.created':
    case 'step.updated':
    case 'step.deleted':
    case 'step.reordered': {
      const keys: ReadonlyArray<readonly unknown[]> = [
        engagementKeys.allSteps(engagementId),
        ...activityKeys,
      ]
      if (parents.scenarioId) {
        return [...keys, engagementKeys.steps(engagementId, parents.scenarioId)]
      }
      return keys
    }

    case 'step.revealed': {
      const base: ReadonlyArray<readonly unknown[]> = [
        engagementKeys.allSteps(engagementId),
        ...activityKeys,
      ]
      const withScenario =
        parents.scenarioId
          ? [
              ...base,
              engagementKeys.steps(engagementId, parents.scenarioId),
              engagementKeys.step(engagementId, parents.scenarioId, parents.stepId ?? ''),
            ]
          : base
      // A reveal may change the visible execution status for blue.
      if (parents.stepId) {
        return [...withScenario, engagementKeys.executions(engagementId)]
      }
      return withScenario
    }

    // ── Executions ──────────────────────────────────────────────────────
    case 'execution.red_updated':
    case 'execution.blue_updated': {
      const keys: ReadonlyArray<readonly unknown[]> = [
        engagementKeys.executions(engagementId),
        ...activityKeys,
      ]
      if (parents.executionId) {
        return [...keys, engagementKeys.execution(engagementId, parents.executionId)]
      }
      return keys
    }

    // ── Evidence ────────────────────────────────────────────────────────
    case 'evidence.uploaded':
    case 'evidence.deleted': {
      if (parents.executionId) {
        return [engagementKeys.evidence(engagementId, parents.executionId), ...activityKeys]
      }
      return [engagementKeys.executions(engagementId), ...activityKeys]
    }

    // ── Comments ────────────────────────────────────────────────────────
    case 'comment.created':
    case 'comment.edited': {
      if (parents.executionId) {
        return [engagementKeys.comments(engagementId, parents.executionId), ...activityKeys]
      }
      return [engagementKeys.executions(engagementId), ...activityKeys]
    }

    // ── Findings ────────────────────────────────────────────────────────
    case 'finding.created':
    case 'finding.updated':
    case 'finding.deleted':
    case 'finding.steps_changed':
      return [engagementKeys.findings(engagementId), ...activityKeys]

    default:
      // Unknown verb — no invalidation.  Caller may log a dev warning.
      return []
  }
}

/**
 * Apply query-key invalidations for one engagement event.
 *
 * Unknown verbs are silently ignored.  The caller is expected to log a
 * dev-mode warning before calling when appropriate.
 */
export function invalidateEngagementEvent(
  qc: QueryClient,
  verb: string,
  engagementId: string,
  parents: { executionId?: string; scenarioId?: string; stepId?: string },
): void {
  const keys = queryKeysForVerb(verb, engagementId, parents)
  for (const key of keys) {
    void qc.invalidateQueries({ queryKey: key })
  }
}
