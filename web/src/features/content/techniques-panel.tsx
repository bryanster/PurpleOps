import { type ReactNode, useState } from 'react'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import {
  isBrowsableAttackVersion,
  useAttackVersions,
  useTactics,
  useTechnique,
  useTechniques,
  type ContentAttackVersion,
  type ContentTechnique,
  type TechniqueFilters,
} from './queries'
import {
  ANY,
  DetailDrawer,
  FilterChrome,
  FilterSelect,
  IdBadges,
  MetaRow,
  UseInScenarioPlaceholder,
  VersionSelect,
} from './shared'

/**
 * ATT&CK techniques for one installed version: search, tactic/subtechnique
 * filters, and a detail drawer with description + related external ids.
 */
export function TechniquesPanel(): ReactNode {
  const versionsQuery = useAttackVersions()
  const browsable: ContentAttackVersion[] =
    versionsQuery.data?.items.filter(isBrowsableAttackVersion) ?? []

  // Derived pin: honour an explicit pick when it is still installed, otherwise
  // fall back to the newest browsable label. No effect — the selector value is
  // a pure function of catalog + override.
  const [versionOverride, setVersionOverride] = useState<string | undefined>(undefined)
  const latestVersion = browsable.at(-1)?.version ?? ''
  const version =
    versionOverride !== undefined && browsable.some((v) => v.version === versionOverride)
      ? versionOverride
      : latestVersion

  const [search, setSearch] = useState('')
  const [tactic, setTactic] = useState<string>(ANY)
  const [subtechnique, setSubtechnique] = useState<string>(ANY)
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)

  const filters: TechniqueFilters = {
    ...(version === '' ? {} : { version }),
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(tactic === ANY ? {} : { tactic }),
    ...(subtechnique === ANY ? {} : { isSubtechnique: subtechnique === 'yes' }),
  }

  const tactics = useTactics({ version: version === '' ? undefined : version })
  const list = useTechniques(filters)
  const filtered = Object.keys(filters).some((key) => key !== 'version')

  if (versionsQuery.isPending) {
    return <PageLoading label="Reading installed ATT&CK versions…" />
  }
  if (versionsQuery.error) {
    return (
      <PageError
        error={versionsQuery.error}
        onRetry={() => {
          void versionsQuery.refetch()
        }}
      />
    )
  }
  if (browsable.length === 0) {
    // Parent already gates the whole page on empty library; this is defence if
    // the catalog has only non-ready versions.
    return (
      <PageEmpty
        title="No browsable ATT&CK version"
        description="A version must be ready and come from an enabled source before techniques appear here."
      />
    )
  }

  const rows = list.data?.items ?? []
  const tacticOptions =
    tactics.data?.items.map((t) => ({
      value: t.externalId,
      label: `${t.externalId} · ${t.name}`,
    })) ?? []

  return (
    <div className="flex flex-col gap-4">
      <FilterChrome
        search={search}
        onSearchChange={setSearch}
        searchPlaceholder="T1059, PowerShell, …"
      >
        <VersionSelect
          versions={browsable}
          value={version}
          onValueChange={(next) => {
            setVersionOverride(next)
            // Identity is (version, externalId). Drop the open detail so a
            // previous version's row cannot stay on screen under a new pin.
            setSelectedId(undefined)
          }}
        />
        <FilterSelect
          label="Tactic"
          value={tactic}
          onValueChange={setTactic}
          anyLabel="Any tactic"
          options={tacticOptions}
          className="w-56"
        />
        <FilterSelect
          label="Kind"
          value={subtechnique}
          onValueChange={setSubtechnique}
          anyLabel="Techniques and sub-techniques"
          options={[
            { value: 'no', label: 'Techniques only' },
            { value: 'yes', label: 'Sub-techniques only' },
          ]}
          className="w-56"
        />
      </FilterChrome>

      {list.isPending && <PageLoading label="Reading techniques…" />}

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
            title={filtered ? 'No techniques match those filters' : 'No techniques in this version'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : 'This ATT&CK version has no technique rows yet.'
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-32">ID</TableHead>
                <TableHead>Name</TableHead>
                <TableHead className="w-36">Kind</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TechniqueRow
                  key={row.id}
                  technique={row}
                  onOpen={() => {
                    setSelectedId(row.id)
                  }}
                />
              ))}
            </TableBody>
          </Table>
        ))}

      <TechniqueDetail
        techniqueId={selectedId}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedId(undefined)
          }
        }}
      />
    </div>
  )
}

function TechniqueRow({
  technique,
  onOpen,
}: {
  technique: ContentTechnique
  onOpen: () => void
}): ReactNode {
  return (
    <TableRow>
      <TableCell className="font-mono text-sm">{technique.externalId}</TableCell>
      <TableCell>
        <button
          type="button"
          className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
          onClick={onOpen}
        >
          {technique.name}
        </button>
      </TableCell>
      <TableCell>
        {technique.isSubtechnique ? (
          <Badge variant="outline">
            Sub-technique
            {technique.parentExternalId !== '' && (
              <span className="text-muted-foreground ml-1 font-mono">
                of {technique.parentExternalId}
              </span>
            )}
          </Badge>
        ) : (
          <Badge variant="secondary">Technique</Badge>
        )}
      </TableCell>
      <TableCell>
        <Button type="button" variant="ghost" size="sm" onClick={onOpen}>
          Open
        </Button>
      </TableCell>
    </TableRow>
  )
}

function TechniqueDetail({
  techniqueId,
  onOpenChange,
}: {
  techniqueId: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const detail = useTechnique(techniqueId)
  const open = techniqueId !== undefined

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={
        detail.data !== undefined ? `${detail.data.externalId} · ${detail.data.name}` : 'Technique'
      }
      description={
        detail.data !== undefined
          ? `ATT&CK ${detail.data.version}${detail.data.isSubtechnique ? ' · sub-technique' : ''}`
          : undefined
      }
    >
      {detail.isPending && <PageLoading label="Reading technique…" />}
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
          <div className="flex flex-wrap items-center justify-between gap-3">
            <dl className="flex flex-col gap-3">
              <MetaRow label="Version">{detail.data.version}</MetaRow>
              {detail.data.isSubtechnique && detail.data.parentExternalId !== '' && (
                <MetaRow label="Parent">
                  <span className="font-mono">{detail.data.parentExternalId}</span>
                </MetaRow>
              )}
            </dl>
            <UseInScenarioPlaceholder />
          </div>

          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Description</h3>
            <p className="text-muted-foreground text-sm whitespace-pre-wrap">
              {detail.data.description === '' ? 'No description.' : detail.data.description}
            </p>
          </section>

          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Tactics</h3>
            <IdBadges ids={detail.data.tactics} empty="No tactic mappings." />
          </section>

          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Mitigations</h3>
            <IdBadges ids={detail.data.mitigations} empty="No mitigations linked." />
          </section>
        </div>
      )}
    </DetailDrawer>
  )
}
