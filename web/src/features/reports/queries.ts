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
export type ReportVersion = components['schemas']['ReportVersion']
export type PublishReport = components['schemas']['PublishReport']
export type ReportShare = components['schemas']['ReportShare']
export type ReportShareGrant = components['schemas']['ReportShareGrant']
export type CreateReportShareBody = components['schemas']['CreateReportShare']
export type CreateReportShareResult = components['schemas']['CreateReportShareResult']
export type ReportShareInfo = components['schemas']['ReportShareInfo']
export type ClaimReportShareBody = components['schemas']['ClaimReportShare']
export type ClaimReportShareResult = components['schemas']['ClaimReportShareResult']

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
  versions: (engagementId: string, reportId: string) =>
    [...reportKeys.list(engagementId), reportId, 'versions'] as const,
  versionDetail: (engagementId: string, reportId: string, versionId: string) =>
    [...reportKeys.versions(engagementId, reportId), versionId] as const,
  shares: (versionId: string) => ['report-versions', versionId, 'shares'] as const,
  shareInfo: (token: string) => ['report-views', token, 'info'] as const,
  shareHtml: (token: string) => ['report-views', token, 'html'] as const,
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
    mutationFn: async ({ engagementId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/reports', {
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
      const result = await api.POST('/engagements/{engagementId}/reports/{reportId}/preview', {
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

// ── Publish (M6-011) ─────────────────────────────────────────────────────────

export function usePublishReport(): UseMutationResult<
  ReportVersion,
  Error,
  { engagementId: string; reportId: string; body: PublishReport }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ engagementId, reportId, body }) =>
      unwrap(
        await api.POST('/engagements/{engagementId}/reports/{reportId}/publish', {
          params: { path: { engagementId, reportId } },
          body,
        }),
      ),
    onSuccess: (_data, { engagementId, reportId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.versions(engagementId, reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.detail(engagementId, reportId),
      })
    },
  })
}

// ── Versions (M6-011) ────────────────────────────────────────────────────────

export function versionsQueryOptions(
  engagementId: string | undefined,
  reportId: string | undefined,
) {
  return queryOptions({
    queryKey:
      engagementId && reportId ? reportKeys.versions(engagementId, reportId) : reportKeys.all(''),
    enabled: Boolean(engagementId && reportId),
    queryFn: async () => {
      if (!engagementId || !reportId) throw new Error('engagementId and reportId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/reports/{reportId}/versions', {
          params: { path: { engagementId, reportId } },
        }),
      )
    },
  })
}

export function useVersions(
  engagementId: string | undefined,
  reportId: string | undefined,
): UseQueryResult<ReportVersion[]> {
  return useQuery(versionsQueryOptions(engagementId, reportId))
}

export function versionQueryOptions(
  engagementId: string | undefined,
  reportId: string | undefined,
  versionId: string | undefined,
) {
  return queryOptions({
    queryKey:
      engagementId && reportId && versionId
        ? reportKeys.versionDetail(engagementId, reportId, versionId)
        : reportKeys.all(''),
    enabled: Boolean(engagementId && reportId && versionId),
    queryFn: async () => {
      if (!engagementId || !reportId || !versionId)
        throw new Error('engagementId, reportId, and versionId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/reports/{reportId}/versions/{versionId}', {
          params: { path: { engagementId, reportId, versionId } },
        }),
      )
    },
  })
}

export function useVersion(
  engagementId: string | undefined,
  reportId: string | undefined,
  versionId: string | undefined,
): UseQueryResult<ReportVersion> {
  return useQuery(versionQueryOptions(engagementId, reportId, versionId))
}

// ── Shares (M6-012) ──────────────────────────────────────────────────────────

export function sharesQueryOptions(versionId: string | undefined) {
  return queryOptions({
    queryKey: versionId ? reportKeys.shares(versionId) : ['report-versions', ''],
    enabled: Boolean(versionId),
    queryFn: async () => {
      if (!versionId) throw new Error('versionId required')
      return unwrap(
        await api.GET('/report-versions/{versionId}/shares', {
          params: { path: { versionId } },
        }),
      )
    },
  })
}

export function useShares(versionId: string | undefined): UseQueryResult<ReportShare[]> {
  return useQuery(sharesQueryOptions(versionId))
}

export function useCreateShare(): UseMutationResult<
  CreateReportShareResult,
  Error,
  { versionId: string; body: CreateReportShareBody }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ versionId, body }) =>
      unwrap(
        await api.POST('/report-versions/{versionId}/shares', {
          params: { path: { versionId } },
          body,
        }),
      ),
    onSuccess: (_data, { versionId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.shares(versionId),
      })
    },
  })
}

export function useRevokeShare(): UseMutationResult<
  void,
  Error,
  { shareId: string; versionId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ shareId }) => {
      await api.DELETE('/report-shares/{shareId}', {
        params: { path: { shareId } },
      })
    },
    onSuccess: (_data, { versionId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.shares(versionId),
      })
    },
  })
}

export function useRevokeGrant(): UseMutationResult<
  void,
  Error,
  { shareId: string; grantId: string; versionId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ shareId, grantId }) => {
      await api.DELETE('/report-shares/{shareId}/grants/{grantId}', {
        params: { path: { shareId, grantId } },
      })
    },
    onSuccess: (_data, { versionId }) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.shares(versionId),
      })
    },
  })
}

// ── Share Views (public-ish) ─────────────────────────────────────────────────

export function shareInfoQueryOptions(token: string | undefined) {
  return queryOptions({
    queryKey: token ? reportKeys.shareInfo(token) : ['report-views', '', 'info'],
    enabled: Boolean(token),
    queryFn: async () => {
      if (!token) throw new Error('token required')
      return unwrap(
        await api.GET('/report-views/{token}', {
          params: { path: { token } },
        }),
      )
    },
  })
}

export function useShareInfo(token: string | undefined): UseQueryResult<ReportShareInfo> {
  return useQuery(shareInfoQueryOptions(token))
}

export function useClaimShare(): UseMutationResult<
  ClaimReportShareResult,
  Error,
  { token: string; body?: ClaimReportShareBody }
> {
  return useMutation({
    mutationFn: async ({ token, body }) =>
      unwrap(
        await api.POST('/report-views/{token}/claim', {
          params: { path: { token } },
          body: body ?? {},
        }),
      ),
  })
}

export function useVerifySharePassword(): UseMutationResult<
  void,
  Error,
  { token: string; password: string }
> {
  return useMutation({
    mutationFn: async ({ token, password }) => {
      unwrap(
        await api.POST('/report-views/{token}/password', {
          params: { path: { token } },
          body: { password },
        }),
      )
    },
  })
}

export function shareHtmlQueryOptions(token: string | undefined) {
  return queryOptions({
    queryKey: token ? reportKeys.shareHtml(token) : ['report-views', '', 'html'],
    enabled: Boolean(token),
    queryFn: async () => {
      if (!token) throw new Error('token required')
      return unwrap(
        await api.GET('/report-views/{token}/html', {
          params: { path: { token } },
          parseAs: 'text',
        }),
      )
    },
  })
}

export function useShareHtml(token: string | undefined): UseQueryResult<string> {
  return useQuery(shareHtmlQueryOptions(token))
}
