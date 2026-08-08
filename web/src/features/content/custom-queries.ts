import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, API_BASE_URL, unwrap } from '@/api/client'
import { apiErrorFromResponse } from '@/api/errors'
import type { components } from '@/api/schema'

import { contentKeys } from './queries'
import { sourceKeys } from './sources-queries'

/**
 * Custom content authoring + v1 import/export (M2-015).
 *
 * Components import hooks from here and never touch `api` themselves. Mutations
 * invalidate both the custom lists and the shared library keys so a create on
 * this page is visible under Content immediately.
 */

export type ContentProcedureTemplate = components['schemas']['ContentProcedureTemplate']
export type ContentDetectionRule = components['schemas']['ContentDetectionRule']
export type ContentNote = components['schemas']['ContentNote']
export type ContentProcedureInputArg = components['schemas']['ContentProcedureInputArg']
export type CreateCustomProcedureTemplateRequest =
  components['schemas']['CreateCustomProcedureTemplateRequest']
export type UpdateCustomProcedureTemplateRequest =
  components['schemas']['UpdateCustomProcedureTemplateRequest']
export type CreateCustomDetectionRuleRequest =
  components['schemas']['CreateCustomDetectionRuleRequest']
export type UpdateCustomDetectionRuleRequest =
  components['schemas']['UpdateCustomDetectionRuleRequest']
export type CreateCustomNoteRequest = components['schemas']['CreateCustomNoteRequest']
export type UpdateCustomNoteRequest = components['schemas']['UpdateCustomNoteRequest']
export type ContentImportReport = components['schemas']['ContentImportReport']
export type ContentSyncJob = components['schemas']['ContentSyncJob']
export type ContentImportIssue = components['schemas']['ContentImportIssue']

export type ImportFormat = components['schemas']['ImportCustomContentRequest']['format']
export type ExportFormat = 'yaml' | 'json'
export type ExportType = 'procedure_templates' | 'detection_rules' | 'notes'

const PAGE_SIZE = 200

export interface CustomListFilters {
  q?: string
  technique?: string
  platform?: string
  level?: string
}

export const customKeys = {
  all: [...contentKeys.all, 'custom'] as const,
  procedures: (filters: CustomListFilters) => [...customKeys.all, 'procedures', filters] as const,
  procedure: (id: string) => [...customKeys.all, 'procedure', id] as const,
  detections: (filters: CustomListFilters) => [...customKeys.all, 'detections', filters] as const,
  detection: (id: string) => [...customKeys.all, 'detection', id] as const,
  notes: (filters: CustomListFilters) => [...customKeys.all, 'notes', filters] as const,
  note: (id: string) => [...customKeys.all, 'note', id] as const,
}

async function invalidateCustomContent(queryClient: QueryClient): Promise<void> {
  // One root covers library browse keys and the custom admin lists nested under it.
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: contentKeys.all }),
    queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
  ])
}

export function customProceduresQueryOptions(filters: CustomListFilters = {}) {
  return queryOptions({
    queryKey: customKeys.procedures(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/procedure-templates', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.technique === undefined || filters.technique === ''
                ? {}
                : { technique: filters.technique }),
              ...(filters.platform === undefined || filters.platform === ''
                ? {}
                : { platform: filters.platform }),
            },
          },
          signal,
        }),
      ),
  })
}

export function useCustomProcedures(
  filters: CustomListFilters = {},
): UseQueryResult<components['schemas']['ContentProcedureTemplateList']> {
  return useQuery(customProceduresQueryOptions(filters))
}

export function useCustomProcedure(
  templateId: string | undefined,
): UseQueryResult<ContentProcedureTemplate> {
  return useQuery({
    queryKey: customKeys.procedure(templateId ?? ''),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/procedure-templates/{templateId}', {
          params: { path: { templateId: templateId ?? '' } },
          signal,
        }),
      ),
    enabled: templateId !== undefined && templateId !== '',
  })
}

export function customDetectionsQueryOptions(filters: CustomListFilters = {}) {
  return queryOptions({
    queryKey: customKeys.detections(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/detection-rules', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.technique === undefined || filters.technique === ''
                ? {}
                : { technique: filters.technique }),
              ...(filters.level === undefined || filters.level === ''
                ? {}
                : { level: filters.level }),
            },
          },
          signal,
        }),
      ),
  })
}

export function useCustomDetections(
  filters: CustomListFilters = {},
): UseQueryResult<components['schemas']['ContentDetectionRuleList']> {
  return useQuery(customDetectionsQueryOptions(filters))
}

export function useCustomDetection(
  ruleId: string | undefined,
): UseQueryResult<ContentDetectionRule> {
  return useQuery({
    queryKey: customKeys.detection(ruleId ?? ''),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/detection-rules/{ruleId}', {
          params: { path: { ruleId: ruleId ?? '' } },
          signal,
        }),
      ),
    enabled: ruleId !== undefined && ruleId !== '',
  })
}

export function customNotesQueryOptions(filters: CustomListFilters = {}) {
  return queryOptions({
    queryKey: customKeys.notes(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/notes', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.technique === undefined || filters.technique === ''
                ? {}
                : { technique: filters.technique }),
            },
          },
          signal,
        }),
      ),
  })
}

export function useCustomNotes(
  filters: CustomListFilters = {},
): UseQueryResult<components['schemas']['ContentNoteList']> {
  return useQuery(customNotesQueryOptions(filters))
}

export function useCustomNote(noteId: string | undefined): UseQueryResult<ContentNote> {
  return useQuery({
    queryKey: customKeys.note(noteId ?? ''),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/notes/{noteId}', {
          params: { path: { noteId: noteId ?? '' } },
          signal,
        }),
      ),
    enabled: noteId !== undefined && noteId !== '',
  })
}

export function useCreateCustomProcedure(): UseMutationResult<
  ContentProcedureTemplate,
  Error,
  CreateCustomProcedureTemplateRequest
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) =>
      unwrap(await api.POST('/content/custom/procedure-templates', { body })),
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useUpdateCustomProcedure(): UseMutationResult<
  ContentProcedureTemplate,
  Error,
  { templateId: string; patch: UpdateCustomProcedureTemplateRequest }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ templateId, patch }) =>
      unwrap(
        await api.PATCH('/content/custom/procedure-templates/{templateId}', {
          params: { path: { templateId } },
          body: patch,
        }),
      ),
    onSuccess: async (row) => {
      queryClient.setQueryData(customKeys.procedure(row.id), row)
      queryClient.setQueryData(contentKeys.procedure(row.id), row)
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useDeleteCustomProcedure(): UseMutationResult<void, Error, { templateId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ templateId }) => {
      await api.DELETE('/content/custom/procedure-templates/{templateId}', {
        params: { path: { templateId } },
      })
    },
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useCreateCustomDetection(): UseMutationResult<
  ContentDetectionRule,
  Error,
  CreateCustomDetectionRuleRequest
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/content/custom/detection-rules', { body })),
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useUpdateCustomDetection(): UseMutationResult<
  ContentDetectionRule,
  Error,
  { ruleId: string; patch: UpdateCustomDetectionRuleRequest }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ ruleId, patch }) =>
      unwrap(
        await api.PATCH('/content/custom/detection-rules/{ruleId}', {
          params: { path: { ruleId } },
          body: patch,
        }),
      ),
    onSuccess: async (row) => {
      queryClient.setQueryData(customKeys.detection(row.id), row)
      queryClient.setQueryData(contentKeys.detection(row.id), row)
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useDeleteCustomDetection(): UseMutationResult<void, Error, { ruleId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ ruleId }) => {
      await api.DELETE('/content/custom/detection-rules/{ruleId}', {
        params: { path: { ruleId } },
      })
    },
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useCreateCustomNote(): UseMutationResult<
  ContentNote,
  Error,
  CreateCustomNoteRequest
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/content/custom/notes', { body })),
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useUpdateCustomNote(): UseMutationResult<
  ContentNote,
  Error,
  { noteId: string; patch: UpdateCustomNoteRequest }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ noteId, patch }) =>
      unwrap(
        await api.PATCH('/content/custom/notes/{noteId}', {
          params: { path: { noteId } },
          body: patch,
        }),
      ),
    onSuccess: async (row) => {
      queryClient.setQueryData(customKeys.note(row.id), row)
      queryClient.setQueryData(contentKeys.note(row.id), row)
      await invalidateCustomContent(queryClient)
    },
  })
}

export function useDeleteCustomNote(): UseMutationResult<void, Error, { noteId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ noteId }) => {
      await api.DELETE('/content/custom/notes/{noteId}', {
        params: { path: { noteId } },
      })
    },
    onSuccess: async () => {
      await invalidateCustomContent(queryClient)
    },
  })
}

export type ImportCustomResult =
  { kind: 'report'; report: ContentImportReport } | { kind: 'job'; job: ContentSyncJob }

export interface ImportCustomInput {
  file: File
  format: ImportFormat
  dryRun: boolean
  failFast?: boolean
}

/**
 * Multipart v1/custom import. Dry-run always returns a report (never a job).
 * Large non-dry uploads may answer 202 with a job.
 */
export function useImportCustomContent(): UseMutationResult<
  ImportCustomResult,
  Error,
  ImportCustomInput
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ file, format, dryRun, failFast }) => {
      const form = new FormData()
      form.append('file', file)
      form.append('format', format)
      const result = await api.POST('/content/custom/import', {
        params: {
          query: {
            dryRun,
            ...(failFast === undefined ? {} : { failFast }),
          },
        },
        body: form as unknown as components['schemas']['ImportCustomContentRequest'],
        bodySerializer: (body) => body as unknown as FormData,
      })
      if (result.error !== undefined) {
        throw result.error instanceof Error ? result.error : new Error('API request failed')
      }
      const data = unwrap(result)
      if (result.response.status === 202) {
        return { kind: 'job' as const, job: data as ContentSyncJob }
      }
      return { kind: 'report' as const, report: data as ContentImportReport }
    },
    onSuccess: async (result, variables) => {
      if (variables.dryRun) {
        return
      }
      if (result.kind === 'job') {
        queryClient.setQueryData(sourceKeys.job(result.job.id), result.job)
      }
      await invalidateCustomContent(queryClient)
    },
  })
}

/**
 * Download a custom content export as a file.
 *
 * Uses plain fetch rather than openapi-fetch so the response body stays a Blob
 * regardless of `application/yaml` vs `application/json` content negotiation.
 * openapi-fetch consumes the response body during parsing, preventing
 * subsequent blob extraction needed for client-side file downloads.
 */
export async function downloadCustomExport(options: {
  format?: 'yaml' | 'json'
  type?: 'procedure_templates' | 'detection_rules' | 'notes'
}): Promise<void> {
  const format = options.format ?? 'yaml'
  const params = new URLSearchParams({ format })
  if (options.type !== undefined) {
    params.set('type', options.type)
  }
  /* eslint-disable-next-line no-restricted-globals --
   * Fetch is required for blob download; the generated openapi-fetch client
   * consumes the response body during parsing, preventing client-side blob extraction.
   */
  const response = await fetch(`${API_BASE_URL}/content/custom/export?${params.toString()}`, {
    method: 'GET',
    credentials: 'same-origin',
    headers: { Accept: format === 'json' ? 'application/json' : 'application/yaml' },
  })
  if (!response.ok) {
    throw await apiErrorFromResponse(response)
  }
  const blob = await response.blob()
  if (blob.size === 0) {
    throw new Error('Export was empty')
  }
  const ext = format === 'json' ? 'json' : 'yaml'
  const filename = `blacklight-custom-export.${ext}`
  const url = URL.createObjectURL(blob)
  try {
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    anchor.rel = 'noopener'
    document.body.append(anchor)
    anchor.click()
    anchor.remove()
  } finally {
    URL.revokeObjectURL(url)
  }
}

export function useExportCustomContent(): UseMutationResult<
  void,
  Error,
  { format?: 'yaml' | 'json'; type?: 'procedure_templates' | 'detection_rules' | 'notes' }
> {
  return useMutation({
    mutationFn: downloadCustomExport,
  })
}

/** Split a comma/whitespace technique list into external ids. */
export function parseTechniqueIds(raw: string): string[] {
  return raw
    .split(/[\s,]+/)
    .map((part) => part.trim())
    .filter((part) => part !== '')
}

/** Split a comma-separated tag list. */
export function parseTags(raw: string): string[] {
  return raw
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part !== '')
}

/** Platforms from a free-text multi field (comma or whitespace). */
export function parsePlatforms(raw: string): string[] {
  return parseTechniqueIds(raw).map((p) => p.toLowerCase())
}

export function summarizeImportReport(report: ContentImportReport): string {
  const bits = [
    report.proceduresCreated + report.proceduresUpdated > 0
      ? `${String(report.proceduresCreated + report.proceduresUpdated)} procedure(s)`
      : undefined,
    report.detectionsCreated + report.detectionsUpdated > 0
      ? `${String(report.detectionsCreated + report.detectionsUpdated)} detection(s)`
      : undefined,
    report.notesCreated + report.notesUpdated > 0
      ? `${String(report.notesCreated + report.notesUpdated)} note(s)`
      : undefined,
  ].filter((b): b is string => b !== undefined)
  if (bits.length === 0) {
    return report.dryRun ? 'Dry-run found nothing to import.' : 'Nothing was imported.'
  }
  return report.dryRun ? `Dry-run would touch ${bits.join(', ')}.` : `Imported ${bits.join(', ')}.`
}
