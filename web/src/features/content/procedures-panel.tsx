import { type ReactNode, useId, useState } from 'react'
import { Link } from 'react-router'

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

import { useSignedInUser } from '@/features/auth/current-user'
import { isAdmin } from '@/features/auth/queries'

import { CONTENT_CUSTOM_PATH } from './paths'
import {
  ANY,
  CopyBlock,
  DetailDrawer,
  FilterChrome,
  FilterSelect,
  IdBadges,
  MetaRow,
  UseInScenarioPlaceholder,
} from './shared'
import {
  useProcedure,
  useProcedures,
  type ContentProcedureTemplate,
  type ProcedureFilters,
} from './queries'

const PLATFORMS = [
  { value: 'windows', label: 'Windows' },
  { value: 'linux', label: 'Linux' },
  { value: 'macos', label: 'macOS' },
] as const

/**
 * Atomic (and custom) procedure templates: filters by technique/platform, and a
 * detail that keeps command, cleanup and args as separate sections — never one
 * flattened blob (PLAN.md §3).
 */
export function ProceduresPanel(): ReactNode {
  const user = useSignedInUser()
  const admin = isAdmin(user)
  const [search, setSearch] = useState('')
  const [technique, setTechnique] = useState('')
  const [platform, setPlatform] = useState<string>(ANY)
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)
  const techniqueId = useId()

  const filters: ProcedureFilters = {
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(technique.trim() === '' ? {} : { technique: technique.trim() }),
    ...(platform === ANY ? {} : { platform }),
  }
  const list = useProcedures(filters)
  const rows = list.data?.items ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-4">
      <FilterChrome search={search} onSearchChange={setSearch} searchPlaceholder="Name or id">
        <div className="flex flex-col gap-2">
          <Label htmlFor={techniqueId}>Technique</Label>
          <Input
            id={techniqueId}
            placeholder="T1059.001"
            value={technique}
            className="w-40 font-mono"
            onChange={(event) => {
              setTechnique(event.target.value)
            }}
          />
        </div>
        <FilterSelect
          label="Platform"
          value={platform}
          onValueChange={setPlatform}
          anyLabel="Any platform"
          options={PLATFORMS.map((p) => ({ value: p.value, label: p.label }))}
        />
      </FilterChrome>

      {list.isPending && <PageLoading label="Reading procedures…" />}

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
            title={filtered ? 'No procedures match those filters' : 'No procedures yet'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : admin
                  ? 'Install Atomic Red Team, or create/import a custom template.'
                  : 'Install Atomic Red Team (or ask an admin to add a custom template) to populate this list.'
            }
            action={
              !filtered && admin ? (
                <div className="flex flex-wrap gap-2">
                  <Button asChild variant="outline" size="sm">
                    <Link to={CONTENT_CUSTOM_PATH}>Create custom procedure</Link>
                  </Button>
                  <Button asChild variant="outline" size="sm">
                    <Link to={CONTENT_CUSTOM_PATH}>Import v1 testcases</Link>
                  </Button>
                </div>
              ) : undefined
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-40">Techniques</TableHead>
                <TableHead className="w-40">Platforms</TableHead>
                <TableHead className="w-28">Executor</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <ProcedureRow
                  key={row.id}
                  procedure={row}
                  onOpen={() => {
                    setSelectedId(row.id)
                  }}
                />
              ))}
            </TableBody>
          </Table>
        ))}

      <ProcedureDetail
        templateId={selectedId}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedId(undefined)
          }
        }}
      />
    </div>
  )
}

function ProcedureRow({
  procedure,
  onOpen,
}: {
  procedure: ContentProcedureTemplate
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
          {procedure.name}
        </button>
        {procedure.elevationRequired && (
          <Badge variant="outline" className="ml-2">
            Elevated
          </Badge>
        )}
      </TableCell>
      <TableCell>
        <span className="font-mono text-xs">
          {procedure.techniqueExternalIds.length === 0
            ? '—'
            : procedure.techniqueExternalIds.join(', ')}
        </span>
      </TableCell>
      <TableCell className="text-sm">
        {procedure.platforms.length === 0 ? '—' : procedure.platforms.join(', ')}
      </TableCell>
      <TableCell className="font-mono text-xs">{procedure.executor || '—'}</TableCell>
      <TableCell>
        <Button type="button" variant="ghost" size="sm" onClick={onOpen}>
          Open
        </Button>
      </TableCell>
    </TableRow>
  )
}

function ProcedureDetail({
  templateId,
  onOpenChange,
}: {
  templateId: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const detail = useProcedure(templateId)
  const open = templateId !== undefined

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={detail.data?.name ?? 'Procedure'}
      description={
        detail.data !== undefined
          ? detail.data.techniqueExternalIds.join(', ') || 'No technique mapping'
          : undefined
      }
    >
      {detail.isPending && <PageLoading label="Reading procedure…" />}
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
              <MetaRow label="Executor">
                <span className="font-mono">{detail.data.executor || '—'}</span>
              </MetaRow>
              <MetaRow label="Platforms">
                {detail.data.platforms.length === 0 ? '—' : detail.data.platforms.join(', ')}
              </MetaRow>
              <MetaRow label="Elevation">
                {detail.data.elevationRequired ? 'Required' : 'Not required'}
              </MetaRow>
              <MetaRow label="Techniques">
                <IdBadges ids={detail.data.techniqueExternalIds} />
              </MetaRow>
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

          <CopyBlock label="Command" value={detail.data.command} />

          {detail.data.cleanup !== '' && <CopyBlock label="Cleanup" value={detail.data.cleanup} />}

          {detail.data.inputArgs.length > 0 && (
            <section className="flex flex-col gap-2">
              <h3 className="text-sm font-medium">Input arguments</h3>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Default</TableHead>
                    <TableHead>Description</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detail.data.inputArgs.map((arg) => (
                    <TableRow key={arg.name}>
                      <TableCell className="font-mono text-xs">{arg.name}</TableCell>
                      <TableCell className="font-mono text-xs">{arg.type}</TableCell>
                      <TableCell className="font-mono text-xs">{arg.default || '—'}</TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {arg.description || '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </section>
          )}
        </div>
      )}
    </DetailDrawer>
  )
}
