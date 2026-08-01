import type { ReactNode } from 'react'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'

import { useVersion } from './queries'

/**
 * Build identity of the server this SPA was served by. Exists to prove the
 * plumbing end to end: the Vite proxy in dev, the same-origin embed in
 * production (M0B-010), and the generated client and route behind both.
 */
export function VersionPage(): ReactNode {
  const { data, error, isPending, refetch } = useVersion()

  if (isPending) {
    return (
      <VersionLayout>
        <PageLoading label="Reading the server version…" />
      </VersionLayout>
    )
  }
  if (error) {
    return (
      <VersionLayout>
        <PageError
          error={error}
          onRetry={() => {
            void refetch()
          }}
        />
      </VersionLayout>
    )
  }

  return (
    <VersionLayout>
      <Table className="max-w-xl">
        <TableBody>
          <TableRow>
            <TableCell className="text-muted-foreground">Version</TableCell>
            <TableCell className="font-mono">{data.version}</TableCell>
          </TableRow>
          <TableRow>
            <TableCell className="text-muted-foreground">Commit</TableCell>
            <TableCell className="font-mono">{data.commit}</TableCell>
          </TableRow>
          <TableRow>
            <TableCell className="text-muted-foreground">Built</TableCell>
            <TableCell className="font-mono">{data.buildDate}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </VersionLayout>
  )
}

function VersionLayout({ children }: { children: ReactNode }): ReactNode {
  return (
    <section className="flex flex-col gap-4">
      <h1 className="text-xl font-semibold">Version</h1>
      {children}
    </section>
  )
}
