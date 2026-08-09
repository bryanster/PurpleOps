import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

/**
 * Report, block, template and branding queries (M6-002, M6-003, M6-004).
 */

// ── Types ────────────────────────────────────────────────────────────────────

export type Report = components['schemas']['Report']
export type ReportBlock = components['schemas']['ReportBlock']
export type ReportBlockInput = components['schemas']['ReportBlockInput']
export type CreateReport = components['schemas']['CreateReport']
export type PatchReport = components['schemas']['PatchReport']
export type PutReportBlocks = components['schemas']['PutReportBlocks']
export type ReportTemplate = components['schemas']['ReportTemplate']
export type ReportTemplateBlock = components['schemas']['ReportTemplateBlock']
export type CreateReportTemplate = components['schemas']['CreateReportTemplate']
export type PatchReportTemplate = components['schemas']['PatchReportTemplate']
export type ApplyTemplate = components['schemas']['ApplyTemplate']
export type CreateTemplateFromReport = components['schemas']['CreateTemplateFromReport']
export type ReportBranding = components['schemas']['ReportBranding']

// ── Query keys ───────────────────────────────────────────────────────────────

export const reportKeys = {
  all: (engagementId: string) => ['engagements', engagementId, 'reports'] as const,
  list: (engagementId: string) => [...reportKeys.all(engagementId), 'list'] as const,
  detail: (engagementId: string, reportId: string) =>
    [...reportKeys.all(engagementId), 'detail', reportId] as const,
  templates: (engagementId: string) => [...reportKeys.all(engagementId), 'templates'] as const,
  templateDetail: (engagementId: string, templateId: string) =>
    [...reportKeys.templates(engagementId), 'detail', templateId] as const,
  branding: () => ['settings', 'report-branding'] as const,
  preview: (engagementId: string, reportId: string) =>
    [...reportKeys.detail(engagementId, reportId), 'preview'] as const,
}

// ── Reports ──────────────────────────────────────────────────────────────────

export function reportsQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementId ? reportKeys.list(engagementId) : reportKeys.all(''),
    enabled: Boolean(engagementId),
    queryFn: async () => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/reports', {
          params: { path: { engagementId } },
        }),
      )
    },
  })
}

export function useReports(engagementId: string | undefined): UseQueryResult<Report[]> {
  return useQuery(reportsQueryOptions(engagementId))
}

export function useCreateReport(): UseMutationResult<
  Report,
  Error,
  { engagementId: string; body: CreateReport }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ engagementId, body }) =>
      unwrap(
        api.POST('/engagements/{engagementId}/reports', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.list(engagementId),
      })
    },
  })
}

function reportQueryOptions(engagementId: string | undefined, reportId: string | undefined) {
  return queryOptions({
    queryKey:
      engagementId && reportId ? reportKeys.detail(engagementId, reportId) : reportKeys.all(''),
    enabled: Boolean(engagementId && reportId),
    queryFn: async () => {
      if (!engagementId || !reportId) throw new Error('engagementId and reportId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/reports/{reportId}', {
          params: { path: { engagementId, reportId } },
        }),
      )
    },
  })
}

export function useReport(
  engagementId: string | undefined,
  reportId: string | undefined,
): UseQueryResult<Report> {
  return useQuery(reportQueryOptions(engagementId, reportId))
}

export function usePatchReport(): UseMutationResult<
  Report,
  Error,
  { engagementId: string; reportId: string; body: PatchReport }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, reportId, body }) =>
      unwrap(
        await api.PATCH('/engagements/{engagementId}/reports/{reportId}', {
          params: { path: { engagementId, reportId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId, reportId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.detail(engagementId, reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.list(engagementId),
      })
    },
  })
}

export function useDeleteReport(): UseMutationResult<
  void,
  Error,
  { engagementId: string; reportId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, reportId }) => {
      unwrap(
        await api.DELETE('/engagements/{engagementId}/reports/{reportId}', {
          params: { path: { engagementId, reportId } },
        }),
      )
    },
    onSuccess: (_data, { engagementId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.list(engagementId),
      })
    },
  })
}

export function usePutReportBlocks(): UseMutationResult<
  Report,
  Error,
  {
    engagementId: string
    reportId: string
    body: PutReportBlocks
  }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, reportId, body }) =>
      unwrap(
        await api.PUT('/engagements/{engagementId}/reports/{reportId}/blocks', {
          params: { path: { engagementId, reportId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId, reportId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.detail(engagementId, reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.preview(engagementId, reportId),
      })
    },
  })
}

// ── Preview ──────────────────────────────────────────────────────────────────

export function previewQueryOptions(
  engagementId: string | undefined,
  reportId: string | undefined,
  includeEvidence?: boolean,
) {
  return queryOptions({
    queryKey:
      engagementId && reportId
        ? [...reportKeys.preview(engagementId, reportId), { includeEvidence }]
        : reportKeys.all(''),
    enabled: Boolean(engagementId && reportId),
    queryFn: async ({ signal }) => {
      if (!engagementId || !reportId) throw new Error('engagementId and reportId required')
      const result = await api.GET('/engagements/{engagementId}/reports/{reportId}/preview', {
        params: {
          path: { engagementId, reportId },
          query: includeEvidence ? { includeEvidence } : undefined,
        },
        parseAs: 'text',
        signal,
      })
      if (result.error) throw new Error(JSON.stringify(result.error))
      return result.data
    },
    staleTime: 30_000,
  })
}

export function usePreviewHtml(
  engagementId: string | undefined,
  reportId: string | undefined,
  includeEvidence?: boolean,
): UseQueryResult<string> {
  return useQuery(previewQueryOptions(engagementId, reportId, includeEvidence))
}

// ── Templates ────────────────────────────────────────────────────────────────

export function templatesQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementId ? reportKeys.templates(engagementId) : reportKeys.all(''),
    enabled: Boolean(engagementId),
    queryFn: async () => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/report-templates', {
          params: { path: { engagementId } },
        }),
      )
    },
  })
}

export function useTemplates(engagementId: string | undefined): UseQueryResult<ReportTemplate[]> {
  return useQuery(templatesQueryOptions(engagementId))
}

export function useCreateTemplate(): UseMutationResult<
  ReportTemplate,
  Error,
  { engagementId: string; body: CreateReportTemplate }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/report-templates', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.templates(engagementId),
      })
    },
  })
}

export function useApplyTemplate(): UseMutationResult<
  Report,
  Error,
  { engagementId: string; reportId: string; body: ApplyTemplate }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, reportId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/reports/{reportId}/apply-template', {
          params: { path: { engagementId, reportId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId, reportId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.detail(engagementId, reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.preview(engagementId, reportId),
      })
    },
  })
}

export function useCreateTemplateFromReport(): UseMutationResult<
  ReportTemplate,
  Error,
  { engagementId: string; body: CreateTemplateFromReport }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/report-templates/from-report', {
          params: { path: { engagementId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.templates(engagementId),
      })
    },
  })
}

// ── Branding ─────────────────────────────────────────────────────────────────

export function brandingQueryOptions() {
  return queryOptions({
    queryKey: reportKeys.branding(),
    queryFn: async () =>
      unwrap(
        await api.GET('/settings/report-branding', {
          params: {},
        }),
      ),
    staleTime: 5 * 60_000,
  })
}

export function useReportBranding(): UseQueryResult<ReportBranding> {
  return useQuery(brandingQueryOptions())
}
