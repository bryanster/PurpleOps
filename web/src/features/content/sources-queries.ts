import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

import { contentKeys } from './queries'

/**
 * Content sources control plane (M2-014).
 *
 * Mutations invalidate the source list and any open detail; job-producing
 * mutations also invalidate the jobs list so the progress panel and the global
 * "slot held" gate agree with the server without waiting on SSE.
 */

export type ContentSource = components['schemas']['ContentSource']
export type ContentSourceDetail = components['schemas']['ContentSourceDetail']
export type ContentSourceKind = components['schemas']['ContentSourceKind']
export type ContentSourceStatus = components['schemas']['ContentSourceStatus']
export type ContentSyncJob = components['schemas']['ContentSyncJob']
export type ContentSyncJobStatus = components['schemas']['ContentSyncJobStatus']
export type ContentSourceVersion = components['schemas']['ContentSourceVersion']
export type UpdateContentSourceRequest = components['schemas']['UpdateContentSourceRequest']

/** Builtin custom seed — cannot be deleted or network-synced. */
export const CUSTOM_SOURCE_ID = '01900000-0000-7000-8000-000000000005'

/**
 * The seeded MITRE ATT&CK source. Named because the first-run wizard installs
 * exactly this one and has no list to pick it out of — see
 * `features/setup/setup-page.tsx`. Both ids come from the migration that seeds
 * the registry (`0011_content.sql`).
 */
export const ATTACK_SOURCE_ID = '01900000-0000-7000-8000-000000000001'

const ACTIVE_JOB_STATUSES: ReadonlySet<ContentSyncJobStatus> = new Set([
  'queued',
  'running',
  'cancelling',
])

export function isActiveJobStatus(status: ContentSyncJobStatus): boolean {
  return ACTIVE_JOB_STATUSES.has(status)
}

export function isCustomSource(source: Pick<ContentSource, 'id' | 'kind'>): boolean {
  return source.kind === 'custom' || source.id === CUSTOM_SOURCE_ID
}

export const sourceKeys = {
  all: [...contentKeys.all, 'sources'] as const,
  list: () => [...sourceKeys.all, 'list'] as const,
  detail: (id: string) => [...sourceKeys.all, 'detail', id] as const,
  versions: (id: string) => [...sourceKeys.all, 'versions', id] as const,
  jobs: () => [...contentKeys.all, 'jobs'] as const,
  job: (id: string) => [...contentKeys.all, 'job', id] as const,
}

export function sourcesQueryOptions() {
  return queryOptions({
    queryKey: sourceKeys.list(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/content/sources', { signal })),
  })
}

export function useContentSources(): UseQueryResult<components['schemas']['ContentSourceList']> {
  return useQuery(sourcesQueryOptions())
}

export function sourceDetailQueryOptions(sourceId: string) {
  return queryOptions({
    queryKey: sourceKeys.detail(sourceId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/sources/{sourceId}', {
          params: { path: { sourceId } },
          signal,
        }),
      ),
    enabled: sourceId !== '',
  })
}

export function useContentSource(
  sourceId: string | undefined,
): UseQueryResult<ContentSourceDetail> {
  return useQuery({
    ...sourceDetailQueryOptions(sourceId ?? ''),
    enabled: sourceId !== undefined && sourceId !== '',
  })
}

export function sourceVersionsQueryOptions(sourceId: string) {
  return queryOptions({
    queryKey: sourceKeys.versions(sourceId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/sources/{sourceId}/versions', {
          params: { path: { sourceId } },
          signal,
        }),
      ),
    enabled: sourceId !== '',
  })
}

export function useContentSourceVersions(
  sourceId: string | undefined,
): UseQueryResult<components['schemas']['ContentSourceVersionList']> {
  return useQuery({
    ...sourceVersionsQueryOptions(sourceId ?? ''),
    enabled: sourceId !== undefined && sourceId !== '',
  })
}

export function jobsQueryOptions() {
  return queryOptions({
    queryKey: sourceKeys.jobs(),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/jobs', {
          params: { query: { limit: 50 } },
          signal,
        }),
      ),
    // Jobs move without user action; keep the slot gate honest when SSE is down.
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? []
      return items.some((job) => isActiveJobStatus(job.status)) ? 2_000 : false
    },
  })
}

export function useContentJobs(): UseQueryResult<components['schemas']['ContentSyncJobList']> {
  return useQuery(jobsQueryOptions())
}

export function jobQueryOptions(jobId: string) {
  return queryOptions({
    queryKey: sourceKeys.job(jobId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/jobs/{jobId}', {
          params: { path: { jobId } },
          signal,
        }),
      ),
    enabled: jobId !== '',
  })
}

export function useContentJob(jobId: string | undefined): UseQueryResult<ContentSyncJob> {
  return useQuery({
    ...jobQueryOptions(jobId ?? ''),
    enabled: jobId !== undefined && jobId !== '',
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status !== undefined && isActiveJobStatus(status) ? 2_000 : false
    },
  })
}

async function invalidateSources(queryClient: QueryClient, sourceId?: string): Promise<void> {
  const tasks: Promise<unknown>[] = [
    queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
    queryClient.invalidateQueries({ queryKey: sourceKeys.jobs() }),
    queryClient.invalidateQueries({ queryKey: contentKeys.attackVersions() }),
  ]
  if (sourceId !== undefined) {
    tasks.push(queryClient.invalidateQueries({ queryKey: sourceKeys.detail(sourceId) }))
    tasks.push(queryClient.invalidateQueries({ queryKey: sourceKeys.versions(sourceId) }))
  }
  await Promise.all(tasks)
}

export function useEnableContentSource(): UseMutationResult<
  ContentSource,
  Error,
  { sourceId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId }) =>
      unwrap(
        await api.POST('/content/sources/{sourceId}/enable', {
          params: { path: { sourceId } },
        }),
      ),
    onSuccess: async (_source, { sourceId }) => {
      await invalidateSources(queryClient, sourceId)
    },
  })
}

export function useDisableContentSource(): UseMutationResult<
  ContentSource,
  Error,
  { sourceId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId }) =>
      unwrap(
        await api.POST('/content/sources/{sourceId}/disable', {
          params: { path: { sourceId } },
        }),
      ),
    onSuccess: async (_source, { sourceId }) => {
      await invalidateSources(queryClient, sourceId)
    },
  })
}

export function useUpdateContentSource(): UseMutationResult<
  ContentSource,
  Error,
  { sourceId: string; patch: UpdateContentSourceRequest }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId, patch }) =>
      unwrap(
        await api.PATCH('/content/sources/{sourceId}', {
          params: { path: { sourceId } },
          body: patch,
        }),
      ),
    onSuccess: async (_source, { sourceId }) => {
      await invalidateSources(queryClient, sourceId)
    },
  })
}

export function useDeleteContentSource(): UseMutationResult<void, Error, { sourceId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId }) => {
      await api.DELETE('/content/sources/{sourceId}', {
        params: { path: { sourceId } },
      })
    },
    onSuccess: async (_void, { sourceId }) => {
      await invalidateSources(queryClient, sourceId)
    },
  })
}

export function useStartContentSourceSync(): UseMutationResult<
  ContentSyncJob,
  Error,
  { sourceId: string; version?: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId, version }) =>
      unwrap(
        await api.POST('/content/sources/{sourceId}/sync', {
          params: { path: { sourceId } },
          ...(version === undefined || version === ''
            ? {}
            : { body: { version } satisfies components['schemas']['StartContentSyncRequest'] }),
        }),
      ),
    onSuccess: async (job) => {
      queryClient.setQueryData(sourceKeys.job(job.id), job)
      await invalidateSources(queryClient, job.sourceId)
    },
  })
}

export function useReprocessContentSource(): UseMutationResult<
  ContentSyncJob,
  Error,
  { sourceId: string; version?: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId, version }) =>
      unwrap(
        await api.POST('/content/sources/{sourceId}/reprocess', {
          params: { path: { sourceId } },
          ...(version === undefined || version === ''
            ? {}
            : {
                body: {
                  version,
                } satisfies components['schemas']['ReprocessContentSourceRequest'],
              }),
        }),
      ),
    onSuccess: async (job) => {
      queryClient.setQueryData(sourceKeys.job(job.id), job)
      await invalidateSources(queryClient, job.sourceId)
    },
  })
}

/**
 * Offline bundle upload.
 *
 * The file stays in the FormData for this request only — never in query cache
 * or component state beyond the file input value (ticket M2-014).
 */
export function useUploadContentBundle(): UseMutationResult<
  ContentSyncJob,
  Error,
  { sourceId: string; file: File; version?: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sourceId, file, version }) => {
      const form = new FormData()
      form.append('file', file)
      if (version !== undefined && version !== '') {
        form.append('version', version)
      }
      // openapi-fetch serialises plain objects as JSON; multipart needs FormData
      // passed through untouched (defaultBodySerializer already handles that).
      return unwrap(
        await api.POST('/content/sources/{sourceId}/bundle', {
          params: { path: { sourceId } },
          body: form as unknown as components['schemas']['UploadContentBundleRequest'],
          bodySerializer: (body) => body as unknown as FormData,
        }),
      )
    },
    onSuccess: async (job) => {
      queryClient.setQueryData(sourceKeys.job(job.id), job)
      await invalidateSources(queryClient, job.sourceId)
    },
  })
}

export function useDeleteContentAttackVersion(): UseMutationResult<
  void,
  Error,
  { version: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ version }) => {
      await api.DELETE('/content/attack/versions/{version}', {
        params: { path: { version } },
      })
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
        queryClient.invalidateQueries({ queryKey: contentKeys.attackVersions() }),
      ])
    },
  })
}

/** The single active job holding the global slot, if any (newest first list). */
export function findActiveJob(
  jobs: readonly ContentSyncJob[] | undefined,
): ContentSyncJob | undefined {
  return jobs?.find((job) => isActiveJobStatus(job.status))
}

export function kindLabel(kind: ContentSourceKind): string {
  switch (kind) {
    case 'attack':
      return 'ATT&CK'
    case 'atomic':
      return 'Atomic'
    case 'sigma':
      return 'Sigma'
    case 'ctid':
      return 'CTID'
    case 'custom':
      return 'Custom'
  }
}

export function jobKindLabel(kind: ContentSyncJob['kind']): string {
  switch (kind) {
    case 'sync':
      return 'Sync'
    case 'bundle_import':
      return 'Bundle import'
    case 'reprocess':
      return 'Reprocess'
    case 'v1_import':
      return 'v1 import'
  }
}

export function statusLabel(status: ContentSourceStatus): string {
  switch (status) {
    case 'idle':
      return 'Idle'
    case 'syncing':
      return 'Syncing'
    case 'error':
      return 'Error'
  }
}

export interface JobSlotState {
  held: boolean
  activeJob: ContentSyncJob | undefined
  reason: string | undefined
}

/** True while the global content job slot is held (disables Sync / bundle / reprocess). */
export function useJobSlotHeld(): JobSlotState {
  const jobs = useContentJobs()
  const activeJob = findActiveJob(jobs.data?.items)
  if (activeJob === undefined) {
    return { held: false, activeJob: undefined, reason: undefined }
  }
  return {
    held: true,
    activeJob,
    reason: `Wait for job ${activeJob.id} (${activeJob.status}) to finish before starting another.`,
  }
}
