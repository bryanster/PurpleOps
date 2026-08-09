import { type ReactNode, useMemo } from 'react'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { cn } from '@/lib/utils'

import { useEngagementContext } from './engagement-layout'
import {
  COLOUR_RAMP,
  LEGEND_LABELS,
  NOT_ATTEMPTED_COLOUR,
  NOT_ATTEMPTED_LABEL,
  UNSCORED_COLOUR,
  UNSCORED_LABEL,
  useAnalyticsCoverage,
  useAnalyticsDistribution,
  useAnalyticsMttd,
  useAnalyticsBurndown,
  type AnalyticsCoverage,
  type AnalyticsDistribution,
  type AnalyticsMttd,
  type AnalyticsBurndown,
  type TechniqueCoverageRow,
} from './analytics-queries'

// ── Constants ────────────────────────────────────────────────────────────────

const TACTIC_ORDER: Record<string, number> = {
  reconnaissance: 0,
  'resource-development': 1,
  'initial-access': 2,
  execution: 3,
  persistence: 4,
  'privilege-escalation': 5,
  'defense-evasion': 6,
  'credential-access': 7,
  discovery: 8,
  'lateral-movement': 9,
  collection: 10,
  'command-and-control': 11,
  exfiltration: 12,
  impact: 13,
}

// ── Panel wrapper ────────────────────────────────────────────────────────────

interface PanelProps {
  title: string
  loading: boolean
  error: Error | null
  onRetry?: () => void
  children: ReactNode
}


function PanelShell({ title, loading, error, onRetry, children }: PanelProps) {
  if (error) {
    return (
      <div className="rounded-lg border p-4">
        <h3 className="mb-2 text-sm font-medium">{title}</h3>
        <PageError error={error} onRetry={onRetry} />
      </div>
    )
  }

  if (loading) {
    return (
      <div className="rounded-lg border p-4">
        <h3 className="mb-2 text-sm font-medium">{title}</h3>
        <PageLoading label="Loading…" />
      </div>
    )
  }

  return (
    <div className="rounded-lg border p-4">
      <h3 className="text-sm font-medium">{title}</h3>
      <div className="mt-2">{children}</div>
    </div>
  )
}

// ── Blind banner ─────────────────────────────────────────────────────────────

function BlindBanner() {
  return (
    <div
      role="status"
      aria-label="Blind engagement notice"
      className="bg-muted text-muted-foreground rounded-lg border px-4 py-2 text-sm"
    >
      This view covers revealed steps only. Totals differ per seat in a blind engagement.
    </div>
  )
}

// ── Scorecards ───────────────────────────────────────────────────────────────

function CoverageCard({ data }: { data: AnalyticsCoverage }) {
  const { techniques } = data
  if (techniques.rows.length === 0) {
    return <PageEmpty title="Nothing scored yet" description="Add steps and score executions to see coverage." />
  }
  const pct = techniques.matrix > 0
    ? ((techniques.attempted / techniques.matrix) * 100).toFixed(1)
    : '0.0'
  return (
    <div className="space-y-1">
      <div className="text-2xl font-semibold tabular-nums">{pct}%</div>
      <p className="text-muted-foreground text-xs">
        {techniques.attempted} of {techniques.matrix} ATT&amp;CK techniques attempted
      </p>
      <p className="text-muted-foreground text-xs">
        {techniques.notAttempted} not attempted · {techniques.unmatched} unmatched
      </p>
    </div>
  )
}

function DetectionCard({ data }: { data: AnalyticsDistribution }) {
  const { category } = data
  if (category.attempted === 0) {
    return <PageEmpty title="Nothing scored yet" />
  }
  const labelToColour: Record<string, string> = {}
  for (let i = 0; i < LEGEND_LABELS.length; i++) {
    labelToColour[LEGEND_LABELS[i] as string] = COLOUR_RAMP[i] as string
  }
  const total = category.buckets.reduce((s, b) => s + b.count, 0)
  return (
    <div className="space-y-2">
      <p className="text-muted-foreground text-xs">
        {category.attempted} scored executions
      </p>
      <div className="flex h-2.5 w-full overflow-hidden rounded-full">
        {category.buckets.map((b) => (
          <div
            key={b.label}
            className="h-full transition-all"
            style={{
              width: `${String(total > 0 ? (b.count / total) * 100 : 0)}%`,
              backgroundColor: labelToColour[b.label] ?? UNSCORED_COLOUR,
            }}
            title={`${b.label}: ${String(b.count)}`}
          />
        ))}
      </div>
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        {category.buckets.map((b) => {
          const colour = labelToColour[b.label] ?? UNSCORED_COLOUR
          return (
            <span key={b.label} className="text-muted-foreground inline-flex items-center gap-1 text-xs">
              <span className="inline-block size-2.5 rounded-sm flex-shrink-0" style={{ backgroundColor: colour }} />
              {b.label}: {b.count}
            </span>
          )
        })}
      </div>
    </div>
  )
}

function ProtectionRateCard({ data }: { data: AnalyticsDistribution }) {
  const { protection } = data
  if (protection.attempted === 0) {
    return <PageEmpty title="Nothing scored yet" />
  }
  const blocked = protection.buckets.find((b) => b.label === 'blocked')?.count ?? 0
  const partial = protection.buckets.find((b) => b.label === 'partial')?.count ?? 0
  const prevented = blocked + partial
  const pct = protection.attempted > 0
    ? ((prevented / protection.attempted) * 100).toFixed(1)
    : '0.0'
  return (
    <div className="space-y-1">
      <div className="text-2xl font-semibold tabular-nums">{pct}%</div>
      <p className="text-muted-foreground text-xs">
        {prevented} of {protection.attempted} scored executions blocked or partially blocked
      </p>
      <p className="text-muted-foreground text-xs">
        {blocked} fully blocked · {partial} partially blocked
      </p>
    </div>
  )
}

function MttdCard({ data }: { data: AnalyticsMttd }) {
  if (data.attemptedCount === 0) {
    return <PageEmpty title="Nothing scored yet" />
  }
  const fmt = (seconds: number) => {
    if (seconds < 60) return `${String(seconds)}s`
    if (seconds < 3600) return `${String(Math.floor(seconds / 60))}m ${String(seconds % 60)}s`
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    return `${String(h)}h ${String(m)}m`
  }
  return (
    <div className="space-y-1">
      <div className="flex items-baseline gap-2">
        <span className="text-muted-foreground text-xs">p50</span>
        <span className="text-lg font-semibold tabular-nums">
          {data.p50 != null ? fmt(data.p50) : '—'}
        </span>
      </div>
      <p className="text-muted-foreground text-xs">
        p90: {data.p90 != null ? fmt(data.p90) : '—'} · max: {data.max != null ? fmt(data.max) : '—'}
      </p>
      <p className="text-muted-foreground text-xs">
        {data.detectedCount} detected · {data.undetectedCount} undetected
        {data.unscoredCount > 0 && <> · {data.unscoredCount} unscored</>}
      </p>
    </div>
  )
}

function FindingsCard({ data }: { data: AnalyticsBurndown }) {
  const { severity } = data
  let totalOpen = 0
  let totalClosed = 0
  for (const b of severity.buckets) {
    totalOpen += b.totalOpen
    totalClosed += b.resolved + b.acceptedRisk
  }
  if (totalOpen === 0 && totalClosed === 0) {
    return <PageEmpty title="No findings" description="Create findings from the Findings tab." />
  }
  return (
    <div className="space-y-1">
      <div className="text-2xl font-semibold tabular-nums">{totalOpen}</div>
      <p className="text-muted-foreground text-xs">
        open findings · {totalClosed} closed
      </p>
      {severity.buckets.filter((b) => b.totalOpen > 0).length > 0 && (
        <div className="flex flex-wrap gap-x-3 gap-y-1">
          {severity.buckets
            .filter((b) => b.totalOpen > 0)
            .map((b) => (
              <span key={b.severity} className="text-muted-foreground text-xs">
                {b.severity}: {b.totalOpen}
              </span>
            ))}
        </div>
      )}
    </div>
  )
}

// ── Heatmap ──────────────────────────────────────────────────────────────────

function cellColour(row: TechniqueCoverageRow): { colour: string; label: string } {
  if (!row.attempted) {
    return { colour: NOT_ATTEMPTED_COLOUR, label: NOT_ATTEMPTED_LABEL }
  }
  if (row.bestCategoryOrdinal == null) {
    return { colour: UNSCORED_COLOUR, label: UNSCORED_LABEL }
  }
  if (row.bestCategoryOrdinal >= 0 && row.bestCategoryOrdinal < COLOUR_RAMP.length) {
    return { colour: COLOUR_RAMP[row.bestCategoryOrdinal] as string, label: LEGEND_LABELS[row.bestCategoryOrdinal] as string }
  }
  return { colour: UNSCORED_COLOUR, label: UNSCORED_LABEL }
}

function HeatmapLegend() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs" aria-label="Heatmap legend">
      {LEGEND_LABELS.map((label, i) => (
        <span key={label} className="inline-flex items-center gap-1">
          <span className="inline-block size-3 rounded-sm flex-shrink-0" style={{ backgroundColor: COLOUR_RAMP[i] }} />
          {label}
        </span>
      ))}
      <span className="inline-flex items-center gap-1">
        <span className="inline-block size-3 rounded-sm flex-shrink-0 border" style={{ backgroundColor: NOT_ATTEMPTED_COLOUR }} />
        {NOT_ATTEMPTED_LABEL}
      </span>
      <span className="inline-flex items-center gap-1">
        <span className="inline-block size-3 rounded-sm flex-shrink-0 border" style={{ backgroundColor: UNSCORED_COLOUR }} />
        {UNSCORED_LABEL}
      </span>
    </div>
  )
}

function ThermalCell({ row }: { row: TechniqueCoverageRow }) {
  const { colour, label } = cellColour(row)
  return (
    <span
      className={cn(
        'inline-block size-4 rounded-sm flex-shrink-0',
        !row.attempted && 'border border-dashed',
        row.attempted && row.bestCategoryOrdinal == null && 'border',
      )}
      style={{ backgroundColor: colour }}
      title={`${row.techniqueId}: ${row.name} — ${label}`}
      aria-label={`${row.name}: ${label}`}
    />
  )
}

function Heatmap({ data }: { data: AnalyticsCoverage }) {
  const { techniques, tactics } = data

  // Group techniques by parent: parent techniques first, sub-techniques nested
  const parentGroups = useMemo(() => {
    const groups: Record<string, TechniqueCoverageRow[]> = {}
    for (const row of techniques.rows) {
      const key = row.isSubtechnique ? row.parentTechniqueId : row.techniqueId
      groups[key] ??= []
      groups[key].push(row)
    }
    return groups
  }, [techniques])

  // Flatten: parent first, then subs
  const flatRows = useMemo(() => {
    const result: TechniqueCoverageRow[] = []
    for (const [, rows] of Object.entries(parentGroups)) {
      const parents = rows.filter((r) => !r.isSubtechnique)
      const subs = rows.filter((r) => r.isSubtechnique)
      result.push(...parents, ...subs)
    }
    return result
  }, [parentGroups])

  // Order tactics by ATT&CK order
  const orderedTactics = useMemo(() => {
    return [...tactics.rows].sort((a, b) => {
      const ao = TACTIC_ORDER[a.tacticId] ?? 99
      const bo = TACTIC_ORDER[b.tacticId] ?? 99
      return ao - bo
    })
  }, [tactics])

  if (techniques.rows.length === 0) {
    return <PageEmpty title="Nothing scored yet" description="Add steps and score executions to see the heatmap." />
  }

  return (
    <div className="space-y-4">
      <HeatmapLegend />

      {/* Tactic-level summary bars */}
      {orderedTactics.length > 0 && (
        <div className="space-y-1">
          <p className="text-muted-foreground text-xs font-medium">Tactic coverage</p>
          <div className="flex flex-wrap gap-2">
            {orderedTactics.map((tactic) => {
              const total = tactic.categories.reduce((s, c) => s + c.count, 0)
              if (total === 0) {
                return (
                  <div key={tactic.tacticId} className="rounded border px-2 py-1 text-xs" style={{ backgroundColor: NOT_ATTEMPTED_COLOUR }}>
                    <span className="text-muted-foreground">{tactic.tacticName}</span>
                    <span className="ml-1 text-muted-foreground">—</span>
                  </div>
                )
              }
              return (
                <div
                  key={tactic.tacticId}
                  className="rounded border px-2 py-1 text-xs"
                  title={`${tactic.tacticName}: ${String(tactic.attemptedTechniques)} attempted of ${String(tactic.matrixTechniques)} matrix`}
                >
                  <span className="font-medium">{tactic.tacticName}</span>
                  <div className="mt-0.5 flex h-1.5 w-16 overflow-hidden rounded-full">
                    {tactic.categories
                      .filter((c) => c.count > 0)
                      .map((c) => {
                        const idx = LEGEND_LABELS.indexOf(c.category as typeof LEGEND_LABELS[number])
                        const colour = idx >= 0 ? COLOUR_RAMP[idx] : UNSCORED_COLOUR
                        return (
                          <div
                            key={c.category}
                            className="h-full"
                            style={{ width: `${String((c.count / total) * 100)}%`, backgroundColor: colour }}
                          />
                        )
                      })}
                  </div>
                  <span className="text-muted-foreground ml-1">{String(tactic.attemptedTechniques)}/{String(tactic.matrixTechniques)}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Technique grid */}
      <div className="overflow-x-auto" role="grid" aria-label="ATT&CK technique heatmap">
        <div className="flex flex-wrap gap-1">
          {flatRows.map((row) => (
            <div
              key={row.techniqueId}
              className={cn(
                'flex items-center gap-1.5 rounded border px-1.5 py-0.5',
                row.isSubtechnique && 'ml-3',
              )}
              role="gridcell"
              title={`${row.techniqueId}: ${row.name} — ${cellColour(row).label}${row.stepCount > 0 ? ` (${String(row.stepCount)} steps)` : ''}`}
            >
              <ThermalCell row={row} />
              <span className={cn('text-xs', row.isSubtechnique && 'text-muted-foreground')}>
                {row.isSubtechnique ? row.name : (
                  <>
                    <span className="font-medium">{row.techniqueId}</span>
                    <span className="text-muted-foreground"> {row.name}</span>
                  </>
                )}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// ── Burndown chart ───────────────────────────────────────────────────────────

interface BurndownChartProps {
  data: AnalyticsBurndown
}

function BurndownChart({ data }: BurndownChartProps) {
  const { points, interval } = data

  if (points.length === 0) {
    return <PageEmpty title="No findings history" description="Burndown data appears once findings start changing status." />
  }

  const maxTotal = Math.max(...points.map((p) => p.totalOpen + p.resolved + p.acceptedRisk), 1)

  const chartHeight = 160
  const pad = 10
  const chartWidth = points.length > 1 ? (points.length - 1) * 40 + pad * 2 : 200

  // SVG polyline for totalOpen
  const linePoints = points
    .map((p, i) => {
      const x = points.length > 1 ? (i / (points.length - 1)) * (chartWidth - pad * 2) + pad : chartWidth / 2
      const y = chartHeight - pad - (p.totalOpen / maxTotal) * (chartHeight - pad * 2)
      return `${String(x)},${String(y)}`
    })
    .join(' ')

  return (
    <div className="space-y-2">
      <p className="text-muted-foreground text-xs">
        {interval === 'daily' ? 'Daily' : 'Weekly'} burndown
      </p>
      <div className="overflow-x-auto">
        <svg
          viewBox={`0 0 ${String(chartWidth)} ${String(chartHeight)}`}
          className="w-full"
          style={{ maxHeight: chartHeight }}
          role="img"
          aria-label={`Findings burndown chart — ${interval}`}
        >
          {/* Grid lines */}
          {[0, 0.25, 0.5, 0.75].map((frac) => {
            const y = chartHeight - pad - frac * (chartHeight - pad * 2)
            return (
              <line
                key={frac}
                x1={pad}
                y1={y}
                x2={chartWidth - pad}
                y2={y}
                stroke="var(--color-border)"
                strokeWidth={0.5}
                strokeDasharray="3 3"
              />
            )
          })}
          {/* Baseline */}
          <line
            x1={pad}
            y1={chartHeight - pad}
            x2={chartWidth - pad}
            y2={chartHeight - pad}
            stroke="var(--color-border)"
            strokeWidth={1}
          />
          {/* TotalOpen line */}
          <polyline
            fill="none"
            stroke={COLOUR_RAMP[4]}
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            points={linePoints}
          />
          {/* Dots */}
          {points.map((p, i) => {
            const x = points.length > 1 ? (i / (points.length - 1)) * (chartWidth - pad * 2) + pad : chartWidth / 2
            const y = chartHeight - pad - (p.totalOpen / maxTotal) * (chartHeight - pad * 2)
            return (
              <circle
                key={i}
                cx={x}
                cy={y}
                r={3}
                fill={COLOUR_RAMP[4]}
              >
                <title>{`${p.date}: ${String(p.totalOpen)} open (${String(p.open)} new + ${String(p.inProgress)} in progress), ${String(p.resolved)} resolved, ${String(p.acceptedRisk)} accepted risk`}</title>
              </circle>
            )
          })}
        </svg>
      </div>
      {points.length > 1 && (
        <div className="flex justify-between text-[10px] text-muted-foreground">
          <span>{points[0]?.date}</span>
          <span>{points[points.length - 1]?.date}</span>
        </div>
      )}
    </div>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function AnalyticsPage(): ReactNode {
  const { engagementId } = useEngagementContext()

  const coverage = useAnalyticsCoverage(engagementId)
  const distribution = useAnalyticsDistribution(engagementId)
  const mttd = useAnalyticsMttd(engagementId)
  const burndown = useAnalyticsBurndown(engagementId)

  const blindFiltered =
    (coverage.data?.blindFiltered ??
      distribution.data?.blindFiltered ??
      mttd.data?.blindFiltered ??
      burndown.data?.blindFiltered) ??
    false

  return (
    <div className="space-y-6 p-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Analytics</h2>
      </div>

      {blindFiltered && <BlindBanner />}

      {/* Scorecards row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <PanelShell
          title="Coverage"
          loading={coverage.isPending}
          error={coverage.error}
          onRetry={() => { void coverage.refetch() }}
        >
          {coverage.data && <CoverageCard data={coverage.data} />}
        </PanelShell>

        <PanelShell
          title="Detection"
          loading={distribution.isPending}
          error={distribution.error}
          onRetry={() => { void distribution.refetch() }}
        >
          {distribution.data && <DetectionCard data={distribution.data} />}
        </PanelShell>

        <PanelShell
          title="Protection rate"
          loading={distribution.isPending}
          error={distribution.error}
          onRetry={() => { void distribution.refetch() }}
        >
          {distribution.data && <ProtectionRateCard data={distribution.data} />}
        </PanelShell>

        <PanelShell
          title="MTTD"
          loading={mttd.isPending}
          error={mttd.error}
          onRetry={() => { void mttd.refetch() }}
        >
          {mttd.data && <MttdCard data={mttd.data} />}
        </PanelShell>

        <PanelShell
          title="Findings"
          loading={burndown.isPending}
          error={burndown.error}
          onRetry={() => { void burndown.refetch() }}
        >
          {burndown.data && <FindingsCard data={burndown.data} />}
        </PanelShell>
      </div>

      {/* Heatmap */}
      <PanelShell
        title="ATT&amp;CK Heatmap"
        loading={coverage.isPending}
        error={coverage.error}
        onRetry={() => { void coverage.refetch() }}
      >
        {coverage.data && <Heatmap data={coverage.data} />}
      </PanelShell>

      {/* Burndown */}
      <PanelShell
        title="Findings Burndown"
        loading={burndown.isPending}
        error={burndown.error}
        onRetry={() => { void burndown.refetch() }}
      >
        {burndown.data && <BurndownChart data={burndown.data} />}
      </PanelShell>
    </div>
  )
}
