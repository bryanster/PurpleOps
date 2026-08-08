import { queryOptions, useQuery, type UseQueryResult } from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

/**
 * Content library reads (M2-013).
 *
 * Every list key carries its filters — version especially — so switching the
 * ATT&CK pin cannot serve a previous version's rows from cache. Components
 * import hooks; nothing here is called with a hand-written URL.
 */

export type ContentTechnique = components['schemas']['ContentTechnique']
export type ContentTechniqueDetail = components['schemas']['ContentTechniqueDetail']
export type ContentTactic = components['schemas']['ContentTactic']
export type ContentProcedureTemplate = components['schemas']['ContentProcedureTemplate']
export type ContentDetectionRule = components['schemas']['ContentDetectionRule']
export type ContentEmulationPlan = components['schemas']['ContentEmulationPlan']
export type ContentEmulationPlanDetail = components['schemas']['ContentEmulationPlanDetail']
export type ContentNote = components['schemas']['ContentNote']
export type ContentAttackVersion = components['schemas']['ContentAttackVersion']

/** Server default is 500; keep one page within that for interactive browsing. */
const PAGE_SIZE = 200

export interface TechniqueFilters {
  version?: string
  q?: string
  tactic?: string
  /** `'any'` is not sent; only true/false reach the wire. */
  isSubtechnique?: boolean
}

export interface ProcedureFilters {
  q?: string
  technique?: string
  platform?: string
  sourceId?: string
}

export interface DetectionFilters {
  q?: string
  technique?: string
  level?: string
  sourceId?: string
}

export interface PlanFilters {
  q?: string
  technique?: string
  sourceId?: string
}

export interface NoteFilters {
  q?: string
  technique?: string
}

export const contentKeys = {
  all: ['content'] as const,
  attackVersions: () => [...contentKeys.all, 'attack-versions'] as const,
  tactics: (filters: { version?: string; q?: string }) =>
    [...contentKeys.all, 'tactics', filters] as const,
  techniques: (filters: TechniqueFilters) => [...contentKeys.all, 'techniques', filters] as const,
  technique: (id: string) => [...contentKeys.all, 'technique', id] as const,
  procedures: (filters: ProcedureFilters) => [...contentKeys.all, 'procedures', filters] as const,
  procedure: (id: string) => [...contentKeys.all, 'procedure', id] as const,
  detections: (filters: DetectionFilters) => [...contentKeys.all, 'detections', filters] as const,
  detection: (id: string) => [...contentKeys.all, 'detection', id] as const,
  plans: (filters: PlanFilters) => [...contentKeys.all, 'plans', filters] as const,
  plan: (id: string) => [...contentKeys.all, 'plan', id] as const,
  notes: (filters: NoteFilters) => [...contentKeys.all, 'notes', filters] as const,
  note: (id: string) => [...contentKeys.all, 'note', id] as const,
}

/**
 * An installed ATT&CK version that is safe to browse: source enabled, parse
 * finished, and at least one object. Empty or disabled installs stay out of the
 * version selector so picking one cannot land on a blank list.
 */
export function isBrowsableAttackVersion(v: ContentAttackVersion): boolean {
  return v.sourceEnabled && v.status === 'ready' && v.itemCount > 0
}

export function attackVersionsQueryOptions() {
  return queryOptions({
    queryKey: contentKeys.attackVersions(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/content/attack/versions', { signal })),
  })
}

export function useAttackVersions(): UseQueryResult<
  components['schemas']['ContentAttackVersionList']
> {
  return useQuery(attackVersionsQueryOptions())
}

export function tacticsQueryOptions(filters: { version?: string; q?: string }) {
  return queryOptions({
    queryKey: contentKeys.tactics(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/tactics', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.version === undefined || filters.version === ''
                ? {}
                : { version: filters.version }),
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
            },
          },
          signal,
        }),
      ),
    enabled: filters.version !== undefined && filters.version !== '',
  })
}

export function useTactics(filters: {
  version?: string
  q?: string
}): UseQueryResult<components['schemas']['ContentTacticList']> {
  return useQuery(tacticsQueryOptions(filters))
}

export function techniquesQueryOptions(filters: TechniqueFilters) {
  return queryOptions({
    queryKey: contentKeys.techniques(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/techniques', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.version === undefined || filters.version === ''
                ? {}
                : { version: filters.version }),
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.tactic === undefined || filters.tactic === ''
                ? {}
                : { tactic: filters.tactic }),
              ...(filters.isSubtechnique === undefined
                ? {}
                : { isSubtechnique: filters.isSubtechnique }),
            },
          },
          signal,
        }),
      ),
    enabled: filters.version !== undefined && filters.version !== '',
  })
}

export function useTechniques(
  filters: TechniqueFilters,
): UseQueryResult<components['schemas']['ContentTechniqueList']> {
  return useQuery(techniquesQueryOptions(filters))
}

export function techniqueQueryOptions(techniqueId: string) {
  return queryOptions({
    queryKey: contentKeys.technique(techniqueId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/techniques/{techniqueId}', {
          params: { path: { techniqueId } },
          signal,
        }),
      ),
  })
}

export function useTechnique(
  techniqueId: string | undefined,
): UseQueryResult<ContentTechniqueDetail> {
  return useQuery({
    ...techniqueQueryOptions(techniqueId ?? ''),
    enabled: techniqueId !== undefined && techniqueId !== '',
  })
}

export function proceduresQueryOptions(filters: ProcedureFilters) {
  return queryOptions({
    queryKey: contentKeys.procedures(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/procedure-templates', {
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
              ...(filters.sourceId === undefined || filters.sourceId === ''
                ? {}
                : { sourceId: filters.sourceId }),
            },
          },
          signal,
        }),
      ),
  })
}

export function useProcedures(
  filters: ProcedureFilters,
): UseQueryResult<components['schemas']['ContentProcedureTemplateList']> {
  return useQuery(proceduresQueryOptions(filters))
}

export function procedureQueryOptions(templateId: string) {
  return queryOptions({
    queryKey: contentKeys.procedure(templateId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/procedure-templates/{templateId}', {
          params: { path: { templateId } },
          signal,
        }),
      ),
  })
}

export function useProcedure(
  templateId: string | undefined,
): UseQueryResult<ContentProcedureTemplate> {
  return useQuery({
    ...procedureQueryOptions(templateId ?? ''),
    enabled: templateId !== undefined && templateId !== '',
  })
}

export function detectionsQueryOptions(filters: DetectionFilters) {
  return queryOptions({
    queryKey: contentKeys.detections(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/detection-rules', {
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
              ...(filters.sourceId === undefined || filters.sourceId === ''
                ? {}
                : { sourceId: filters.sourceId }),
            },
          },
          signal,
        }),
      ),
  })
}

export function useDetections(
  filters: DetectionFilters,
): UseQueryResult<components['schemas']['ContentDetectionRuleList']> {
  return useQuery(detectionsQueryOptions(filters))
}

export function detectionQueryOptions(ruleId: string) {
  return queryOptions({
    queryKey: contentKeys.detection(ruleId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/detection-rules/{ruleId}', {
          params: { path: { ruleId } },
          signal,
        }),
      ),
  })
}

export function useDetection(ruleId: string | undefined): UseQueryResult<ContentDetectionRule> {
  return useQuery({
    ...detectionQueryOptions(ruleId ?? ''),
    enabled: ruleId !== undefined && ruleId !== '',
  })
}

export function plansQueryOptions(filters: PlanFilters) {
  return queryOptions({
    queryKey: contentKeys.plans(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/emulation-plans', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.technique === undefined || filters.technique === ''
                ? {}
                : { technique: filters.technique }),
              ...(filters.sourceId === undefined || filters.sourceId === ''
                ? {}
                : { sourceId: filters.sourceId }),
            },
          },
          signal,
        }),
      ),
  })
}

export function usePlans(
  filters: PlanFilters,
): UseQueryResult<components['schemas']['ContentEmulationPlanList']> {
  return useQuery(plansQueryOptions(filters))
}

export function planQueryOptions(planId: string) {
  return queryOptions({
    queryKey: contentKeys.plan(planId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/emulation-plans/{planId}', {
          params: { path: { planId } },
          signal,
        }),
      ),
  })
}

export function usePlan(planId: string | undefined): UseQueryResult<ContentEmulationPlanDetail> {
  return useQuery({
    ...planQueryOptions(planId ?? ''),
    enabled: planId !== undefined && planId !== '',
  })
}

export function notesQueryOptions(filters: NoteFilters) {
  return queryOptions({
    queryKey: contentKeys.notes(filters),
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

export function useNotes(
  filters: NoteFilters,
): UseQueryResult<components['schemas']['ContentNoteList']> {
  return useQuery(notesQueryOptions(filters))
}

export function noteQueryOptions(noteId: string) {
  return queryOptions({
    queryKey: contentKeys.note(noteId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/content/custom/notes/{noteId}', {
          params: { path: { noteId } },
          signal,
        }),
      ),
  })
}

export function useNote(noteId: string | undefined): UseQueryResult<ContentNote> {
  return useQuery({
    ...noteQueryOptions(noteId ?? ''),
    enabled: noteId !== undefined && noteId !== '',
  })
}
