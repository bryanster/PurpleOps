import {
  infiniteQueryOptions,
  queryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, unwrap, unwrapVoid } from '@/api/client'
import type { components } from '@/api/schema'

/**
 * Engagement, scenario, step, execution, finding, evidence, comment and member
 * reads and writes (M3-014).
 *
 * Every hook follows the same pattern as the content library: a `queryOptions()`
 * factory (importable by tests) and a `use*` hook that wraps it.
 */

// ── Types ────────────────────────────────────────────────────────────────────

export type Engagement = components['schemas']['Engagement']
export type EngagementStatus = components['schemas']['EngagementStatus']
export type EngagementMode = components['schemas']['EngagementMode']
export type CreateEngagement = components['schemas']['CreateEngagement']
export type PatchEngagement = components['schemas']['PatchEngagement']
export type SetEngagementStatus = components['schemas']['SetEngagementStatus']
export type EngagementMember = components['schemas']['EngagementMember']
/** Seat role stored on memberships — excludes platform-admin pseudo-role. */
export type MemberRole = components['schemas']['EngagementRole']
/** Caller capability in an engagement UI, including platform admin. */
export type EngagementRole = MemberRole | 'admin'
export type AddMember = components['schemas']['AddMember']
export type PatchMember = components['schemas']['PatchMember']
export type Scenario = components['schemas']['Scenario']
export type CreateScenario = components['schemas']['CreateScenario']
export type PatchScenario = components['schemas']['PatchScenario']
export type Step = components['schemas']['Step']
export type CreateStep = components['schemas']['CreateStep']
export type PatchStep = components['schemas']['PatchStep']
export type Execution = components['schemas']['Execution']
export type ExecutionStatus = components['schemas']['ExecutionStatus']
export type RedExecutionPatch = components['schemas']['RedExecutionPatch']
export type BlueDetectionPatch = components['schemas']['BlueDetectionPatch']
export type Finding = components['schemas']['Finding']
export type NewFinding = components['schemas']['NewFinding']
export type Evidence = components['schemas']['Evidence']
export type Comment = components['schemas']['Comment']
export type CreateComment = components['schemas']['CreateComment']
export type PatchComment = components['schemas']['PatchComment']
export type ImportPlanRequest = components['schemas']['ImportPlanRequest']
export type ImportPlanResponse = components['schemas']['ImportPlanResponse']
export type CreateStepFromTemplate = components['schemas']['CreateStepFromTemplate']

// ── Query keys ───────────────────────────────────────────────────────────────

export const engagementKeys = {
  all: ['engagements'] as const,
  list: (filters: { status?: string; limit?: number; cursor?: string }) =>
    [...engagementKeys.all, 'list', filters] as const,
  detail: (id: string) => [...engagementKeys.all, 'detail', id] as const,
  members: (engagementId: string) => [...engagementKeys.all, 'members', engagementId] as const,
  scenarios: (engagementId: string) => [...engagementKeys.all, 'scenarios', engagementId] as const,
  scenario: (engagementId: string, scenarioId: string) =>
    [...engagementKeys.all, 'scenario', engagementId, scenarioId] as const,
  steps: (engagementId: string, scenarioId: string) =>
    [...engagementKeys.all, 'steps', engagementId, scenarioId] as const,
  allSteps: (engagementId: string) => [...engagementKeys.all, 'allSteps', engagementId] as const,
  step: (engagementId: string, scenarioId: string, stepId: string) =>
    [...engagementKeys.all, 'step', engagementId, scenarioId, stepId] as const,
  executions: (engagementId: string) =>
    [...engagementKeys.all, 'executions', engagementId] as const,
  execution: (engagementId: string, executionId: string) =>
    [...engagementKeys.all, 'execution', engagementId, executionId] as const,
  comments: (engagementId: string, executionId: string) =>
    [...engagementKeys.all, 'comments', engagementId, executionId] as const,
  evidence: (engagementId: string, executionId: string) =>
    [...engagementKeys.all, 'evidence', engagementId, executionId] as const,
  findings: (engagementId: string) => [...engagementKeys.all, 'findings', engagementId] as const,
  activityPrefix: (engagementId: string) =>
    [...engagementKeys.all, 'activity', engagementId] as const,
  activity: (engagementId: string, filters: { verb?: string }) =>
    [...engagementKeys.all, 'activity', engagementId, filters] as const,
  analyticsCoverage: (engagementId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'coverage'] as const,
  analyticsDistribution: (engagementId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'distribution'] as const,
  analyticsMttd: (engagementId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'mttd'] as const,
  analyticsBurndown: (engagementId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'burndown'] as const,
  analyticsCompare: (engagementId: string, baselineId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'compare', baselineId] as const,
  analyticsComparePrefix: (engagementId: string) =>
    [...engagementKeys.all, 'analytics', engagementId, 'compare'] as const,
}

/**
 * The keys that go stale when the step tree of an engagement changes — a step
 * created, edited, deleted or reordered.
 *
 * The executions list belongs in here because a step is created together with
 * its execution row in one transaction, and the workbook joins the two: it
 * reads the executions list for a step's status badge and for the red and blue
 * editors in the drawer. Invalidating only the step lists left a freshly
 * created step with no execution in cache, so it opened an empty drawer until
 * the page was reloaded.
 *
 * Shared by the mutation hooks below and by the SSE path in
 * `event-invalidation.ts`, so that the person who made the change and everyone
 * watching it arrive refresh exactly the same views.
 */
export function stepTreeKeys(
  engagementId: string,
  scenarioId?: string,
): readonly (readonly unknown[])[] {
  const keys: readonly (readonly unknown[])[] = [
    engagementKeys.allSteps(engagementId),
    engagementKeys.executions(engagementId),
    engagementKeys.analyticsCoverage(engagementId),
  ]
  if (scenarioId) {
    return [...keys, engagementKeys.steps(engagementId, scenarioId)]
  }
  return keys
}

/** Invalidate every key in `keys` — the mutation-side counterpart of the SSE fan-out. */
function invalidateAll(qc: QueryClient, keys: readonly (readonly unknown[])[]): void {
  for (const key of keys) {
    void qc.invalidateQueries({ queryKey: key })
  }
}

// ── Engagements ──────────────────────────────────────────────────────────────

export function engagementsQueryOptions(filters: {
  status?: string
  limit?: number
  cursor?: string
}) {
  return infiniteQueryOptions({
    queryKey: engagementKeys.list(filters),
    queryFn: async ({ pageParam, signal }) => {
      const params: Record<string, unknown> = { limit: filters.limit ?? 50 }
      if (filters.status) {
        params.status = filters.status
      }
      if (pageParam) {
        params.cursor = pageParam
      }
      return unwrap(
        await api.GET('/engagements', {
          params: { query: params as never },
          signal,
        }),
      )
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
  })
}

export function useEngagements(filters: { status?: string } = {}): UseInfiniteQueryResult<{
  pages: components['schemas']['EngagementPage'][]
}> {
  return useInfiniteQuery(engagementsQueryOptions(filters))
}

export function useEngagement(engagementId: string | undefined): UseQueryResult<Engagement> {
  return useQuery(engagementQueryOptions(engagementId))
}

export function engagementQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementKeys.detail(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useCreateEngagement(): UseMutationResult<Engagement, Error, CreateEngagement> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body) =>
      unwrap(
        await api.POST('/engagements', {
          body,
          params: { path: undefined as never },
        }),
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: engagementKeys.all })
    },
  })
}

export function usePatchEngagement(): UseMutationResult<
  Engagement,
  Error,
  { engagementId: string; patch: PatchEngagement }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, patch }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}', {
          params: { path: { engagementId } },
          body: patch,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.detail(variables.engagementId),
      })
      void qc.invalidateQueries({ queryKey: engagementKeys.all })
    },
  })
}

export function useSetEngagementStatus(): UseMutationResult<
  Engagement,
  Error,
  { engagementId: string; status: SetEngagementStatus }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, status }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/status', {
          params: { path: { engagementId } },
          body: status,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.detail(variables.engagementId),
      })
      void qc.invalidateQueries({ queryKey: engagementKeys.all })
    },
  })
}

export function useDeleteEngagement(): UseMutationResult<void, Error, { engagementId: string }> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId }) => {
      unwrapVoid(
        await api.DELETE('/engagements/{engagementId}', {
          params: { path: { engagementId } },
        }),
      )
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: engagementKeys.all })
    },
  })
}

export function useEngagementMembers(
  engagementId: string | undefined,
): UseQueryResult<EngagementMember[]> {
  return useQuery({
    queryKey: engagementKeys.members(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/members', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useAddMember(): UseMutationResult<
  EngagementMember,
  Error,
  { engagementId: string; body: AddMember }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/members', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.members(variables.engagementId),
      })
    },
  })
}

export function usePatchMember(): UseMutationResult<
  EngagementMember,
  Error,
  { engagementId: string; userId: string; body: PatchMember }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, userId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/members/{userId}', {
          params: { path: { engagementId, userId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.members(variables.engagementId),
      })
    },
  })
}

export function useRemoveMember(): UseMutationResult<
  void,
  Error,
  { engagementId: string; userId: string }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, userId }) => {
      await api.DELETE('/engagements/{engagementId}/members/{userId}', {
        params: { path: { engagementId, userId } },
      })
    },
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.members(variables.engagementId),
      })
    },
  })
}

export function useScenarios(
  engagementId: string | undefined,
): UseQueryResult<components['schemas']['ScenarioList']> {
  return useQuery({
    queryKey: engagementKeys.scenarios(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/scenarios', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useCreateScenario(): UseMutationResult<
  Scenario,
  Error,
  { engagementId: string; body: CreateScenario }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/scenarios', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.scenarios(variables.engagementId),
      })
    },
  })
}

export function usePatchScenario(): UseMutationResult<
  Scenario,
  Error,
  { engagementId: string; scenarioId: string; body: PatchScenario }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/scenarios/{scenarioId}', {
          params: { path: { engagementId, scenarioId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.scenarios(variables.engagementId),
      })
    },
  })
}

export function useDeleteScenario(): UseMutationResult<
  void,
  Error,
  { engagementId: string; scenarioId: string }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId }) => {
      await api.DELETE('/engagements/{engagementId}/scenarios/{scenarioId}', {
        params: { path: { engagementId, scenarioId } },
      })
    },
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.scenarios(variables.engagementId),
      })
    },
  })
}

export function useReorderScenarios(): UseMutationResult<
  void,
  Error,
  { engagementId: string; ids: string[] }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, ids }) => {
      await api.PUT('/engagements/{engagementId}/scenarios/order', {
        params: { path: { engagementId } },
        body: { ids },
      })
    },
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.scenarios(variables.engagementId),
      })
    },
  })
}

export function useSteps(
  engagementId: string | undefined,
  scenarioId: string | undefined,
): UseQueryResult<components['schemas']['StepList']> {
  return useQuery({
    queryKey: engagementKeys.steps(engagementId ?? '', scenarioId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId || !scenarioId) throw new Error('engagementId and scenarioId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/scenarios/{scenarioId}/steps', {
          params: {
            path: { engagementId, scenarioId },
          },
          signal,
        }),
      )
    },
    enabled:
      engagementId !== undefined &&
      engagementId !== '' &&
      scenarioId !== undefined &&
      scenarioId !== '',
  })
}

export function useAllEngagementSteps(
  engagementId: string | undefined,
): UseQueryResult<components['schemas']['StepList']> {
  return useQuery({
    queryKey: engagementKeys.allSteps(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/steps', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useCreateStep(): UseMutationResult<
  Step,
  Error,
  { engagementId: string; scenarioId: string; body: CreateStep }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/scenarios/{scenarioId}/steps', {
          params: { path: { engagementId, scenarioId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      invalidateAll(qc, stepTreeKeys(variables.engagementId, variables.scenarioId))
    },
  })
}

export function usePatchStep(): UseMutationResult<
  Step,
  Error,
  { engagementId: string; scenarioId: string; stepId: string; body: PatchStep }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, stepId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/scenarios/{scenarioId}/steps/{stepId}', {
          params: { path: { engagementId, scenarioId, stepId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      invalidateAll(qc, stepTreeKeys(variables.engagementId, variables.scenarioId))
    },
  })
}

export function useDeleteStep(): UseMutationResult<
  void,
  Error,
  { engagementId: string; scenarioId: string; stepId: string }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, stepId }) => {
      await api.DELETE('/engagements/{engagementId}/scenarios/{scenarioId}/steps/{stepId}', {
        params: { path: { engagementId, scenarioId, stepId } },
      })
    },
    onSuccess: (_data, variables) => {
      invalidateAll(qc, stepTreeKeys(variables.engagementId, variables.scenarioId))
    },
  })
}

export function useRevealStep(): UseMutationResult<
  Step,
  Error,
  { engagementId: string; scenarioId: string; stepId: string }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, stepId }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/scenarios/{scenarioId}/steps/{stepId}/reveal', {
          params: { path: { engagementId, scenarioId, stepId } },
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.steps(variables.engagementId, variables.scenarioId),
      })
      void qc.invalidateQueries({
        queryKey: engagementKeys.allSteps(variables.engagementId),
      })
    },
  })
}

export function useReorderSteps(): UseMutationResult<
  void,
  Error,
  { engagementId: string; scenarioId: string; ids: string[] }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, ids }) => {
      await api.PUT('/engagements/{engagementId}/scenarios/{scenarioId}/steps/order', {
        params: { path: { engagementId, scenarioId } },
        body: { ids },
      })
    },
    onSuccess: (_data, variables) => {
      invalidateAll(qc, stepTreeKeys(variables.engagementId, variables.scenarioId))
    },
  })
}

export function useEngagementExecutions(
  engagementId: string | undefined,
): UseQueryResult<components['schemas']['ExecutionList']> {
  return useQuery({
    queryKey: engagementKeys.executions(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/executions', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useExecution(
  engagementId: string | undefined,
  executionId: string | undefined,
): UseQueryResult<Execution> {
  return useQuery({
    queryKey: engagementKeys.execution(engagementId ?? '', executionId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId || !executionId) throw new Error('engagementId and executionId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/executions/{executionId}', {
          params: {
            path: { engagementId, executionId },
          },
          signal,
        }),
      )
    },
    enabled:
      engagementId !== undefined &&
      engagementId !== '' &&
      executionId !== undefined &&
      executionId !== '',
  })
}

export function usePatchRedExecution(): UseMutationResult<
  Execution,
  Error,
  {
    engagementId: string
    executionId: string
    body: RedExecutionPatch
  }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, executionId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/executions/{executionId}/execution', {
          params: { path: { engagementId, executionId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.executions(variables.engagementId),
      })
      void qc.invalidateQueries({
        queryKey: engagementKeys.execution(variables.engagementId, variables.executionId),
      })
    },
  })
}

export function usePatchBlueDetection(): UseMutationResult<
  Execution,
  Error,
  {
    engagementId: string
    executionId: string
    body: BlueDetectionPatch
  }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, executionId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/executions/{executionId}/detection', {
          params: { path: { engagementId, executionId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.executions(variables.engagementId),
      })
      void qc.invalidateQueries({
        queryKey: engagementKeys.execution(variables.engagementId, variables.executionId),
      })
    },
  })
}

export function useComments(
  engagementId: string | undefined,
  executionId: string | undefined,
): UseQueryResult<Comment[]> {
  return useQuery({
    queryKey: engagementKeys.comments(engagementId ?? '', executionId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId || !executionId) throw new Error('engagementId and executionId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/executions/{executionId}/comments', {
          params: {
            path: { engagementId, executionId },
          },
          signal,
        }),
      )
    },
    enabled:
      engagementId !== undefined &&
      engagementId !== '' &&
      executionId !== undefined &&
      executionId !== '',
  })
}

export function useCreateComment(): UseMutationResult<
  Comment,
  Error,
  { engagementId: string; executionId: string; body: CreateComment }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, executionId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/executions/{executionId}/comments', {
          params: { path: { engagementId, executionId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.comments(variables.engagementId, variables.executionId),
      })
    },
  })
}

export function usePatchComment(): UseMutationResult<
  Comment,
  Error,
  { engagementId: string; commentId: string; body: PatchComment }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, commentId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/comments/{commentId}', {
          params: { path: { engagementId, commentId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      // Invalidate all comment lists — the comment's executionId is
      // not known to the mutation, so we invalidate executions which
      // cascades to comments via the SSE-driven invalidations.
      void qc.invalidateQueries({
        queryKey: engagementKeys.executions(variables.engagementId),
      })
    },
  })
}

export function useEvidenceList(
  engagementId: string | undefined,
  executionId: string | undefined,
): UseQueryResult<Evidence[]> {
  return useQuery({
    queryKey: engagementKeys.evidence(engagementId ?? '', executionId ?? ''),
    queryFn: async ({ signal }) => {
      if (!executionId) throw new Error('executionId required')
      return unwrap(
        await api.GET('/executions/{executionId}/evidence', {
          params: {
            path: { executionId },
          },
          signal,
        }),
      )
    },
    enabled:
      engagementId !== undefined &&
      engagementId !== '' &&
      executionId !== undefined &&
      executionId !== '',
  })
}

export function useFindings(engagementId: string | undefined): UseQueryResult<Finding[]> {
  return useQuery({
    queryKey: engagementKeys.findings(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/findings', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function useCreateFinding(): UseMutationResult<
  Finding,
  Error,
  { engagementId: string; body: NewFinding }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/findings', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({
        queryKey: engagementKeys.findings(variables.engagementId),
      })
    },
  })
}

// ── Imports ──────────────────────────────────────────────────────────────────

export function useImportPlan(): UseMutationResult<
  ImportPlanResponse,
  Error,
  { engagementId: string; body: ImportPlanRequest }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/import-plan', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      // An import lands scenarios and their steps at once.
      void qc.invalidateQueries({
        queryKey: engagementKeys.scenarios(variables.engagementId),
      })
      invalidateAll(qc, stepTreeKeys(variables.engagementId))
    },
  })
}

export function useCreateStepFromTemplate(): UseMutationResult<
  Step,
  Error,
  {
    engagementId: string
    scenarioId: string
    body: CreateStepFromTemplate
  }
> {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, scenarioId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/scenarios/{scenarioId}/steps/from-template', {
          params: { path: { engagementId, scenarioId } },
          body,
        }),
      ),
    onSuccess: (_data, variables) => {
      invalidateAll(qc, stepTreeKeys(variables.engagementId, variables.scenarioId))
    },
  })
}

// ── Helpers ──────────────────────────────────────────────────────────────────

/** Find the caller's role in an engagement from `GET /auth/me` memberships. */
export function roleInEngagement(
  engagementId: string,
  user: components['schemas']['CurrentUser'],
): EngagementRole | undefined {
  return user.memberships.find((m) => m.engagementId === engagementId)?.role
}

/** The caller is an admin and thus has implicit access to every engagement. */
export function isPlatformAdmin(user: components['schemas']['CurrentUser']): boolean {
  return user.platformRole === 'admin'
}

/** Whether this engagement is closed (disables structure edits). */
export function isEngagementClosed(status: EngagementStatus): boolean {
  return status === 'closed' || status === 'archived'
}

/** A step is revealed (visible to blue) when revealedAt is set. */
export function isStepRevealed(step: Step): boolean {
  return step.revealedAt !== undefined
}
