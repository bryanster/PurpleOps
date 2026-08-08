import type { ReactNode } from 'react'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'

import { type HealthState, useHealth } from './queries'

export function HealthPage(): ReactNode {
  const { data, error, isPending, isFetching, refetch } = useHealth()

  const reload = (): void => {
    void refetch()
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <h1 className="text-xl font-semibold">Health</h1>
        <Button variant="outline" size="sm" onClick={reload} disabled={isFetching}>
          Refresh
        </Button>
      </div>

      {isPending && <PageLoading label="Checking the server…" />}
      {error && <PageError error={error} onRetry={reload} />}
      {data && (
        <Table className="max-w-xl">
          <TableBody>
            <TableRow>
              <TableCell className="text-muted-foreground">Overall</TableCell>
              <TableCell>
                <HealthBadge state={data.status} />
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell className="text-muted-foreground">Database</TableCell>
              <TableCell>
                <HealthBadge state={data.checks.db} />
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      )}
    </section>
  )
}

/** The state is spelled out as well as coloured: colour alone is not a status. */
function HealthBadge({ state }: { state: HealthState }): ReactNode {
  return (
    <Badge variant={state === 'ok' ? 'secondary' : 'destructive'}>
      {state === 'ok' ? 'ok' : 'error'}
    </Badge>
  )
}
