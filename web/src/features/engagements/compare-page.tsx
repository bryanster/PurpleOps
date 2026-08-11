import { type ReactNode, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { cn } from '@/lib/utils'

import { UNSCORED_LABEL, useAnalyticsCompare, type AnalyticsCompare } from './analytics-queries'
import { useEngagementContext } from './engagement-layout'
import { useEngagements } from './queries'

// ── Classification sort order ────────────────────────────────────────────────

const CLASSIFICATION_ORDER: Record<string, number> = {
  regressed: 0,
  improved: 1,
  newlyAttempted: 2,
  noLongerAttempted: 3,
  unchanged: 4,
  incomparable: 5,
}

const CLASSIFICATION_LABELS: Record<string, string> = {
  regressed: 'Regressed',
  improved: 'Improved',
  newlyAttempted: 'Newly attempted',
  noLongerAttempted: 'No longer attempted',
  unchanged: 'Unchanged',
  incomparable: 'Incomparable',
}

const CLASSIFICATION_COLOURS: Record<string, string> = {
  regressed: 'text-red-600',
  improved: 'text-green-600',
  newlyAttempted: 'text-blue-600',
  noLongerAttempted: 'text-gray-400',
  unchanged: 'text-gray-500',
  incomparable: 'text-amber-600',
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function ordinalDeltaLabel(row: { ordinalDelta?: number | null; classification: string }): string {
  if (row.ordinalDelta == null) return '—'
  if (row.ordinalDelta > 0) return `+${String(row.ordinalDelta)}`
  return String(row.ordinalDelta)
}

function describeClassification(row: {
  baselineCategory: string
  currentCategory: string
  classification: string
}): string {
  switch (row.classification) {
    case 'improved':
      return `Detection improved from ${row.baselineCategory || 'none'} to ${row.currentCategory || 'none'}`
    case 'regressed':
      return `Detection regressed from ${row.baselineCategory || 'none'} to ${row.currentCategory || 'none'}`
    case 'newlyAttempted':
      return `Newly attempted — ${row.currentCategory || 'unscored'}`
    case 'noLongerAttempted':
      return `No longer attempted — was ${row.baselineCategory || 'unscored'}`
    case 'incomparable':
      return 'Incomparable — one side is unscored'
    case 'unchanged':
    default:
      return `Unchanged — ${row.baselineCategory || 'unscored'}`
  }
}

// ── Summary row ──────────────────────────────────────────────────────────────

interface SummaryChip {
  key: string
  label: string
  count: number
}

function SummaryRow({
  data,
  activeFilter,
  onFilter,
}: {
  data: AnalyticsCompare
  activeFilter: string | null
  onFilter: (key: string | null) => void
}): ReactNode {
  const chips: SummaryChip[] = [
    { key: 'regressed', label: 'Regressed', count: data.regressed },
    { key: 'improved', label: 'Improved', count: data.improved },
    { key: 'newlyAttempted', label: 'New', count: data.newlyAttempted },
    { key: 'noLongerAttempted', label: 'Removed', count: data.noLongerAttempted },
    { key: 'unchanged', label: 'Unchanged', count: data.unchanged },
    { key: 'incomparable', label: 'Incomparable', count: data.incomparable },
  ]

  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Comparison summary">
      {chips.map((c) => (
        <button
          key={c.key}
          type="button"
          onClick={() => {
            onFilter(activeFilter === c.key ? null : c.key)
          }}
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium transition-colors',
            'focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
            activeFilter === c.key
              ? 'border-primary bg-primary/10 text-primary'
              : 'bg-muted text-muted-foreground hover:bg-muted/80 border-transparent',
          )}
        >
          <span className={CLASSIFICATION_COLOURS[c.key] ?? ''}>{c.label}</span>
          <span className="tabular-nums">{c.count}</span>
        </button>
      ))}
    </div>
  )
}

// ── Delta table ──────────────────────────────────────────────────────────────

function DeltaTable({
  data,
  filter,
}: {
  data: AnalyticsCompare
  filter: string | null
}): ReactNode {
  const sorted = useMemo(() => {
    let rows = [...data.rows]
    if (filter) {
      rows = rows.filter((r) => r.classification === filter)
    }
    rows.sort((a, b) => {
      const aOrd = CLASSIFICATION_ORDER[a.classification] ?? 99
      const bOrd = CLASSIFICATION_ORDER[b.classification] ?? 99
      if (aOrd !== bOrd) return aOrd - bOrd
      // Within same classification, regressions with larger negative delta first
      if (a.ordinalDelta != null && b.ordinalDelta != null && a.ordinalDelta !== b.ordinalDelta) {
        return a.ordinalDelta - b.ordinalDelta
      }
      return a.techniqueId.localeCompare(b.techniqueId)
    })
    return rows
  }, [data.rows, filter])

  if (sorted.length === 0) {
    return (
      <div className="text-muted-foreground py-4 text-center text-sm">
        No techniques match this filter.
      </div>
    )
  }

  return (
    <div className="overflow-x-auto" role="table" aria-label="Technique comparison table">
      <div className="min-w-[800px]">
        {/* Header */}
        <div className="text-muted-foreground grid grid-cols-[1fr_2fr_1fr_1fr_1fr_1.5fr] gap-2 border-b px-2 pb-2 text-xs font-medium">
          <span>Technique</span>
          <span>Name</span>
          <span>Baseline</span>
          <span>Current</span>
          <span>Delta</span>
          <span>Classification</span>
        </div>
        {/* Rows */}
        {sorted.map((row) => (
          <div
            key={`${row.techniqueId}:${row.subtechniqueId || ''}`}
            className="hover:bg-muted/50 grid grid-cols-[1fr_2fr_1fr_1fr_1fr_1.5fr] gap-2 border-b px-2 py-2 text-sm"
            role="row"
          >
            <span className="font-mono text-xs" role="cell">
              {row.techniqueId}
              {row.subtechniqueId && (
                <span className="text-muted-foreground">
                  .{row.subtechniqueId.replace(/^T\d+\./, '')}
                </span>
              )}
            </span>
            <span className="truncate" role="cell" title={row.name}>
              {row.name}
            </span>
            <span className="text-muted-foreground text-xs" role="cell">
              {row.baselineCategory || UNSCORED_LABEL}
            </span>
            <span className="text-xs" role="cell">
              {row.currentCategory || UNSCORED_LABEL}
            </span>
            <span
              className={cn(
                'text-xs font-medium tabular-nums',
                (row.ordinalDelta ?? 0) > 0 && 'text-green-600',
                (row.ordinalDelta ?? 0) < 0 && 'text-red-600',
              )}
              role="cell"
            >
              {ordinalDeltaLabel(row)}
            </span>
            <span
              className={cn('text-xs', CLASSIFICATION_COLOURS[row.classification] ?? '')}
              role="cell"
              title={describeClassification(row)}
            >
              {CLASSIFICATION_LABELS[row.classification] ?? row.classification}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Pin mismatch warning ─────────────────────────────────────────────────────

function PinMismatchBanner({
  baseline,
  current,
}: {
  baseline: string
  current: string
}): ReactNode {
  return (
    <div
      role="alert"
      className="bg-muted text-muted-foreground rounded-lg border px-4 py-2 text-sm"
    >
      ATT&amp;CK version mismatch: baseline uses v{baseline}, current engagement uses v{current}.
      Techniques are compared by external ID; some may have changed across versions.
    </div>
  )
}

// ── Blind banners ────────────────────────────────────────────────────────────

function BaselineBlindBanner(): ReactNode {
  return (
    <div
      role="status"
      aria-label="Baseline blind engagement notice"
      className="bg-muted text-muted-foreground rounded-lg border px-4 py-2 text-sm"
    >
      Baseline shows revealed steps only against your seat in the baseline engagement.
    </div>
  )
}

function CurrentBlindBanner(): ReactNode {
  return (
    <div
      role="status"
      aria-label="Current blind engagement notice"
      className="bg-muted text-muted-foreground rounded-lg border px-4 py-2 text-sm"
    >
      This view covers revealed steps only. Totals differ per seat in a blind engagement.
    </div>
  )
}

// ── Baseline picker ──────────────────────────────────────────────────────────

function BaselinePicker({
  engagementId,
  selectedId,
  onChange,
}: {
  engagementId: string
  selectedId: string
  onChange: (id: string) => void
}): ReactNode {
  const engagements = useEngagements()

  const allEngagements = useMemo(() => {
    const list: { id: string; name: string; client: string; startsOn: string }[] = []
    for (const page of engagements.data?.pages ?? []) {
      for (const eng of page.items) {
        if (eng.id !== engagementId) {
          list.push({
            id: eng.id,
            name: eng.name,
            client: eng.client,
            startsOn: eng.startsOn,
          })
        }
      }
    }
    // Most recent first
    list.sort((a, b) => b.startsOn.localeCompare(a.startsOn))
    // Same client surfaced first
    return list
  }, [engagements.data, engagementId])

  return (
    <div className="flex items-center gap-2">
      <label htmlFor="baseline-picker" className="text-sm font-medium whitespace-nowrap">
        Compare with…
      </label>
      <select
        id="baseline-picker"
        value={selectedId}
        onChange={(e) => {
          onChange(e.target.value)
        }}
        className="border-input bg-background ring-offset-background focus-visible:ring-ring rounded-md border px-3 py-1.5 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
      >
        <option value="">Select an engagement…</option>
        {allEngagements.map((eng) => (
          <option key={eng.id} value={eng.id}>
            {eng.name}
            {eng.client ? ` — ${eng.client}` : ''} ({eng.startsOn})
          </option>
        ))}
      </select>
    </div>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function ComparePage(): ReactNode {
  const { engagementId } = useEngagementContext()
  const [searchParams, setSearchParams] = useSearchParams()
  const baselineId = searchParams.get('baseline') ?? ''
  const [activeFilter, setActiveFilter] = useState<string | null>(null)

  const compare = useAnalyticsCompare(engagementId, baselineId || undefined)

  const handleBaselineChange = (id: string) => {
    if (id) {
      setSearchParams({ baseline: id })
    } else {
      setSearchParams({})
    }
    setActiveFilter(null)
  }

  // No baseline selected
  if (!baselineId) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Compare</h2>
        </div>
        <div className="rounded-lg border p-8">
          <div className="mx-auto max-w-md space-y-4 text-center">
            <p className="text-muted-foreground text-sm">
              Compare this engagement&apos;s technique scores with a baseline engagement to see what
              improved, regressed, or stayed the same.
            </p>
            <BaselinePicker
              engagementId={engagementId}
              selectedId=""
              onChange={handleBaselineChange}
            />
          </div>
        </div>
      </div>
    )
  }

  if (compare.isPending) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Compare</h2>
          <BaselinePicker
            engagementId={engagementId}
            selectedId={baselineId}
            onChange={handleBaselineChange}
          />
        </div>
        <div className="rounded-lg border p-4">
          <PageLoading label="Loading comparison…" />
        </div>
      </div>
    )
  }

  if (compare.error) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Compare</h2>
          <BaselinePicker
            engagementId={engagementId}
            selectedId={baselineId}
            onChange={handleBaselineChange}
          />
        </div>
        <div className="rounded-lg border p-4">
          <PageError
            error={compare.error}
            onRetry={() => {
              void compare.refetch()
            }}
          />
        </div>
      </div>
    )
  }

  const data = compare.data

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">Compare</h2>
          <p className="text-muted-foreground text-sm">
            Baseline ← <span className="font-medium">Current</span>
          </p>
        </div>
        <BaselinePicker
          engagementId={engagementId}
          selectedId={baselineId}
          onChange={handleBaselineChange}
        />
      </div>

      {/* Blind banners */}
      {data.baselineBlindFiltered && <BaselineBlindBanner />}
      {data.currentBlindFiltered && <CurrentBlindBanner />}

      {/* Pin mismatch */}
      {data.pinMismatch && (
        <PinMismatchBanner
          baseline={data.pinMismatch.baseline}
          current={data.pinMismatch.current}
        />
      )}

      {/* Empty state */}
      {data.rows.length === 0 ? (
        <div className="rounded-lg border p-4">
          <PageEmpty
            title="No techniques to compare"
            description="The two engagements have no techniques in common. This is a real answer — not an error."
          />
        </div>
      ) : (
        <>
          {/* Summary row */}
          <SummaryRow data={data} activeFilter={activeFilter} onFilter={setActiveFilter} />

          {/* Delta table */}
          <div className="rounded-lg border">
            <div className="p-4">
              <DeltaTable data={data} filter={activeFilter} />
            </div>
          </div>
        </>
      )}
    </div>
  )
}
