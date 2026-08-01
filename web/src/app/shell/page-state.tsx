import { Loader2Icon } from 'lucide-react'
import type { ReactNode } from 'react'

import { Button } from '@/components/ui/button'

/**
 * The two states every data-backed screen has besides "loaded". They live in
 * the shell so that a slow request and a failed one look the same everywhere —
 * the alternative is each screen inventing its own spinner.
 */
export function PageLoading({ label = 'Loading…' }: { label?: string }): ReactNode {
  return (
    <div className="text-muted-foreground flex items-center gap-2" role="status">
      <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
      {label}
    </div>
  )
}

export function PageError({ error, onRetry }: { error: Error; onRetry?: () => void }): ReactNode {
  return (
    <div role="alert" className="flex max-w-prose flex-col items-start gap-3">
      <p className="font-medium">That request failed.</p>
      <p className="text-muted-foreground text-sm">{error.message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  )
}
