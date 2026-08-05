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

import { DetailDrawer, FilterChrome, MetaRow } from './shared'
import { useNote, useNotes, type ContentNote, type NoteFilters } from './queries'

/**
 * Custom knowledge-base notes: list/search and a read-only detail. Creating and
 * editing belong to M2-015.
 */
export function NotesPanel(): ReactNode {
  const [search, setSearch] = useState('')
  const [technique, setTechnique] = useState('')
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)
  const techniqueId = useId()

  const filters: NoteFilters = {
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(technique.trim() === '' ? {} : { technique: technique.trim() }),
  }
  const list = useNotes(filters)
  const rows = list.data?.items ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-4">
      <FilterChrome search={search} onSearchChange={setSearch} searchPlaceholder="Title or body">
        <div className="flex flex-col gap-2">
          <Label htmlFor={techniqueId}>Technique</Label>
          <Input
            id={techniqueId}
            placeholder="T1003"
            value={technique}
            className="w-40 font-mono"
            onChange={(event) => {
              setTechnique(event.target.value)
            }}
          />
        </div>
      </FilterChrome>

      {list.isPending && <PageLoading label="Reading notes…" />}

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
            title={filtered ? 'No notes match those filters' : 'No notes yet'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : 'Custom notes appear here once an administrator adds them.'
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead className="w-32">Technique</TableHead>
                <TableHead>Tags</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <NoteRow
                  key={row.id}
                  note={row}
                  onOpen={() => {
                    setSelectedId(row.id)
                  }}
                />
              ))}
            </TableBody>
          </Table>
        ))}

      <NoteDetail
        noteId={selectedId}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedId(undefined)
          }
        }}
      />
    </div>
  )
}

function NoteRow({ note, onOpen }: { note: ContentNote; onOpen: () => void }): ReactNode {
  return (
    <TableRow>
      <TableCell>
        <button
          type="button"
          className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
          onClick={onOpen}
        >
          {note.title}
        </button>
      </TableCell>
      <TableCell className="font-mono text-xs">
        {note.techniqueExternalId === '' ? '—' : note.techniqueExternalId}
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap gap-1">
          {note.tags.length === 0
            ? '—'
            : note.tags.map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
        </div>
      </TableCell>
      <TableCell>
        <Button type="button" variant="ghost" size="sm" onClick={onOpen}>
          Open
        </Button>
      </TableCell>
    </TableRow>
  )
}

function NoteDetail({
  noteId,
  onOpenChange,
}: {
  noteId: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const detail = useNote(noteId)
  const open = noteId !== undefined

  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={detail.data?.title ?? 'Note'}
      description={
        detail.data !== undefined && detail.data.techniqueExternalId !== ''
          ? detail.data.techniqueExternalId
          : undefined
      }
    >
      {detail.isPending && <PageLoading label="Reading note…" />}
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
          <dl className="flex flex-col gap-3">
            {detail.data.techniqueExternalId !== '' && (
              <MetaRow label="Technique">
                <span className="font-mono">{detail.data.techniqueExternalId}</span>
              </MetaRow>
            )}
            {detail.data.tags.length > 0 && (
              <MetaRow label="Tags">
                <div className="flex flex-wrap gap-1">
                  {detail.data.tags.map((tag) => (
                    <Badge key={tag} variant="secondary">
                      {tag}
                    </Badge>
                  ))}
                </div>
              </MetaRow>
            )}
          </dl>

          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Body</h3>
            <pre className="bg-muted max-h-96 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
              {detail.data.bodyMarkdown === '' ? '—' : detail.data.bodyMarkdown}
            </pre>
          </section>
        </div>
      )}
    </DetailDrawer>
  )
}
