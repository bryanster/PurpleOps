import type { ReactNode } from 'react'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import { useAsync } from '@/lib/use-async'

import { fetchVersion } from './api'

/**
 * Build identity of the server this SPA was served by. Exists to prove the
 * plumbing end to end: the Vite proxy in dev, the same-origin embed in
 * production (M0B-010), and the generated route behind both.
 */
export function VersionPage(): ReactNode {
  const { state, reload } = useAsync(fetchVersion)

  return (
    <section className="flex flex-col gap-4">
      <h1 className="text-xl font-semibold">Version</h1>

      {state.status === 'loading' && <PageLoading label="Reading the server version…" />}
      {state.status === 'error' && <PageError error={state.error} onRetry={reload} />}
      {state.status === 'ready' && (
        <Table className="max-w-xl">
          <TableBody>
            <TableRow>
              <TableCell className="text-muted-foreground">Version</TableCell>
              <TableCell className="font-mono">{state.data.version}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell className="text-muted-foreground">Commit</TableCell>
              <TableCell className="font-mono">{state.data.commit}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell className="text-muted-foreground">Built</TableCell>
              <TableCell className="font-mono">{state.data.buildDate}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      )}
    </section>
  )
}
