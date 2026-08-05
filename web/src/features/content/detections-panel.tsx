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

import {
  ANY,
  CopyBlock,
  DetailDrawer,
  FilterChrome,
  FilterSelect,
  IdBadges,
  MetaRow,
} from './shared'
import {
  useDetection,
  useDetections,
  type ContentDetectionRule,
  type DetectionFilters,
} from './queries'

const LEVELS = [
  { value: 'informational', label: 'Informational' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
] as const

/** Product copy required on every detection detail (PLAN.md §3 / M2-009). */
export const REFERENCE_ONLY_MESSAGE = 'Reference only — not deployed by Blacklight'

/**
 * Detection rule references (Sigma + custom). Always labelled reference-only;
 * the body is a read-only YAML viewer with copy.
 */
export function DetectionsPanel(): ReactNode {
  const [search, setSearch] = useState('')
  const [technique, setTechnique] = useState('')
  const [level, setLevel] = useState<string>(ANY)
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)
  const techniqueId = useId()

  const filters: DetectionFilters = {
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(technique.trim() === '' ? {} : { technique: technique.trim() }),
    ...(level === ANY ? {} : { level }),
  }
  const list = useDetections(filters)
  const rows = list.data?.items ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-4">
      <FilterChrome search={search} onSearchChange={setSearch} searchPlaceholder="Title or id">
        <div className="flex flex-col gap-2">
          <Label htmlFor={techniqueId}>Technique</Label>
          <Input
            id={techniqueId}
            placeholder="T1059"
            value={technique}
            className="w-40 font-mono"
            onChange={(event) => {
              setTechnique(event.target.value)
            }}
          />
        </div>
        <FilterSelect
          label="Level"
          value={level}
          onValueChange={setLevel}
          anyLabel="Any level"
          options={LEVELS.map((l) => ({ value: l.value, label: l.label }))}
        />
      </FilterChrome>

      {list.isPending && <PageLoading label="Reading detection rules…" />}

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
            title={filtered ? 'No rules match those filters' : 'No detection rules yet'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : 'Install Sigma (or add a custom rule) to populate this list.'
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead className="w-40">Techniques</TableHead>
                <TableHead className="w-28">Level</TableHead>
                <TableHead className="w-28">Status</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <DetectionRow
                  key={row.id}
                  rule={row}
                  onOpen={() => {
                    setSelectedId(row.id)
                  }}
                />
              ))}
            </TableBody>
          </Table>
        ))}

      <DetectionDetail
        ruleId={selectedId}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedId(undefined)
          }
        }}
      />
    </div>
  )
}

function DetectionRow({
  rule,
  onOpen,
}: {
  rule: ContentDetectionRule
  onOpen: () => void
}): ReactNode {
  return (
    <TableRow>
      <TableCell>
        <button
          type="button"
          className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
          onClick={onOpen}
        >
          {rule.name}
        </button>
      </TableCell>
      <TableCell>
        <span className="font-mono text-xs">
          {rule.techniqueExternalIds.length === 0 ? '—' : rule.techniqueExternalIds.join(', ')}
        </span>
      </TableCell>
      <TableCell>
        <Badge variant="outline">{rule.level || '—'}</Badge>
      </TableCell>
      <TableCell className="text-muted-foreground text-sm">{rule.status || '—'}</TableCell>
      <TableCell>
        <Button type="button" variant="ghost" size="sm" onClick={onOpen}>
          Open
        </Button>
      </TableCell>
    </TableRow>
  )
}

function DetectionDetail({
  ruleId,
  onOpenChange,
}: {
  ruleId: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const detail = useDetection(ruleId)
  const open = ruleId !== undefined

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={detail.data?.name ?? 'Detection rule'}
      description={REFERENCE_ONLY_MESSAGE}
    >
      {detail.isPending && <PageLoading label="Reading detection rule…" />}
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
          <p
            role="note"
            className="bg-muted text-muted-foreground rounded-md border border-dashed px-3 py-2 text-sm"
          >
            {REFERENCE_ONLY_MESSAGE}
          </p>

          <dl className="flex flex-col gap-3">
            <MetaRow label="Level">
              <Badge variant="outline">{detail.data.level || '—'}</Badge>
            </MetaRow>
            <MetaRow label="Status">{detail.data.status || '—'}</MetaRow>
            <MetaRow label="Techniques">
              <IdBadges ids={detail.data.techniqueExternalIds} />
            </MetaRow>
          </dl>

          {detail.data.description !== '' && (
            <section className="flex flex-col gap-2">
              <h3 className="text-sm font-medium">Description</h3>
              <p className="text-muted-foreground text-sm whitespace-pre-wrap">
                {detail.data.description}
              </p>
            </section>
          )}

          <CopyBlock label="Rule body" value={detail.data.ruleYaml} />
        </div>
      )}
    </DetailDrawer>
  )
}
