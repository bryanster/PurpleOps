import { type ReactNode, useId, useState } from 'react'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { DetailDrawer, FilterChrome, MetaRow, UseInScenarioPlaceholder } from './shared'
import { usePlan, usePlans, type ContentEmulationPlan, type PlanFilters } from './queries'

/**
 * CTID emulation-plan catalog: list + detail with steps in ordinal order.
 */
export function PlansPanel(): ReactNode {
  const [search, setSearch] = useState('')
  const [technique, setTechnique] = useState('')
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)
  const techniqueId = useId()

  const filters: PlanFilters = {
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(technique.trim() === '' ? {} : { technique: technique.trim() }),
  }
  const list = usePlans(filters)
  const rows = list.data?.items ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-4">
      <FilterChrome
        search={search}
        onSearchChange={setSearch}
        searchPlaceholder="Plan or adversary"
      >
        <div className="flex flex-col gap-2">
          <Label htmlFor={techniqueId}>Technique</Label>
          <Input
            id={techniqueId}
            placeholder="T1566.001"
            value={technique}
            className="w-40 font-mono"
            onChange={(event) => {
              setTechnique(event.target.value)
            }}
          />
        </div>
      </FilterChrome>

      {list.isPending && <PageLoading label="Reading emulation plans…" />}

      {list.error && (
        <PageError
          error={list.error}
          onRetry={() => {
            void list.refetch()
          }}
        />
      )}

      {list.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title={filtered ? 'No plans match those filters' : 'No emulation plans yet'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : 'Install a CTID catalog to populate this list.'
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-40">Adversary</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <PlanRow
                  key={row.id}
                  plan={row}
                  onOpen={() => {
                    setSelectedId(row.id)
                  }}
                />
              ))}
            </TableBody>
          </Table>
        ))}

      <PlanDetail
        planId={selectedId}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedId(undefined)
          }
        }}
      />
    </div>
  )
}

function PlanRow({ plan, onOpen }: { plan: ContentEmulationPlan; onOpen: () => void }): ReactNode {
  return (
    <TableRow>
      <TableCell>
        <button
          type="button"
          className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
          onClick={onOpen}
        >
          {plan.name}
        </button>
      </TableCell>
      <TableCell>
        <Badge variant="secondary">{plan.adversaryName || '—'}</Badge>
      </TableCell>
      <TableCell>
        <Button type="button" variant="ghost" size="sm" onClick={onOpen}>
          Open
        </Button>
      </TableCell>
    </TableRow>
  )
}

function PlanDetail({
  planId,
  onOpenChange,
}: {
  planId: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const detail = usePlan(planId)
  const open = planId !== undefined

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={detail.data?.name ?? 'Emulation plan'}
      description={
        detail.data !== undefined ? detail.data.adversaryName || 'No adversary label' : undefined
      }
    >
      {detail.isPending && <PageLoading label="Reading emulation plan…" />}
      {detail.error && (
        <PageError
          error={detail.error}
          onRetry={() => {
            void detail.refetch()
          }}
        />
      )}
      {detail.data && (
        <div className="flex flex-col gap-5">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <dl className="flex flex-col gap-3">
              <MetaRow label="Adversary">{detail.data.adversaryName || '—'}</MetaRow>
              <MetaRow label="Steps">{detail.data.steps.length}</MetaRow>
            </dl>
            <UseInScenarioPlaceholder />
          </div>

          {detail.data.description !== '' && (
            <section className="flex flex-col gap-2">
              <h3 className="text-sm font-medium">Description</h3>
              <p className="text-muted-foreground text-sm whitespace-pre-wrap">
                {detail.data.description}
              </p>
            </section>
          )}

          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Ordered steps</h3>
            {detail.data.steps.length === 0 ? (
              <p className="text-muted-foreground text-sm">This plan has no steps.</p>
            ) : (
              <ol className="flex flex-col gap-3">
                {detail.data.steps.map((step) => (
                  <li key={step.id} className="flex flex-col gap-1 rounded-md border p-3">
                    <div className="flex flex-wrap items-baseline gap-2">
                      <span className="text-muted-foreground font-mono text-xs">
                        {step.ordinal}.
                      </span>
                      <span className="font-medium">{step.name || 'Untitled step'}</span>
                      {step.techniqueExternalId !== '' && (
                        <Badge variant="outline" className="font-mono">
                          {step.techniqueExternalId}
                        </Badge>
                      )}
                    </div>
                    {step.description !== '' && (
                      <p className="text-muted-foreground text-sm whitespace-pre-wrap">
                        {step.description}
                      </p>
                    )}
                  </li>
                ))}
              </ol>
            )}
          </section>
        </div>
      )}
    </DetailDrawer>
  )
}
