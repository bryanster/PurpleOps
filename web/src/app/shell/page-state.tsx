import { Loader2Icon } from 'lucide-react'
import type { ReactNode } from 'react'

import { ApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'

/**
 * The three states every data-backed screen has besides "loaded". They live in
 * the shell so that a slow request, a failed one and an empty answer look the
 * same everywhere — the alternative is each screen inventing its own spinner
 * and its own blank table.
 */
export function PageLoading({ label = 'Loading…' }: { label?: string }): ReactNode {
  return (
    <div className="text-muted-foreground flex items-center gap-2" role="status">
      <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
      {label}
    </div>
  )
}

/**
 * A failed request, with the identifier that makes it findable.
 *
 * The request ID is not decoration: it is the one string a user can quote and
 * an operator can grep for (M0B-007), and a screen that swallows it turns a
 * five-second log search into a conversation. It is rendered only when the
 * failure carried one — a network error that never reached the server has none,
 * and inventing a placeholder would be worse than saying nothing.
 */
export function PageError({ error, onRetry }: { error: Error; onRetry?: () => void }): ReactNode {
  const requestId = error instanceof ApiError ? error.requestId : undefined

  return (
    <div role="alert" className="flex max-w-prose flex-col items-start gap-3">
      <p className="font-medium">That request failed.</p>
      <p className="text-muted-foreground text-sm">{error.message}</p>
      {requestId !== undefined && (
        <p className="text-muted-foreground text-xs">
          Quote request <code className="font-mono">{requestId}</code> if you report this.
        </p>
      )}
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  )
}

/**
 * A screen, table or panel with nothing in it yet.
 *
 * "Nothing here" and "we could not load that" are different facts, and a blank
 * area says neither. `action` is where a screen puts the control that would
 * change the situation — create the first one, or widen a filter.
 */
export function PageEmpty({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: ReactNode
}): ReactNode {
  return (
    <div className="flex flex-col items-start gap-2 rounded-lg border border-dashed p-6">
      <p className="font-medium">{title}</p>
      {description !== undefined && (
        <p className="text-muted-foreground max-w-prose text-sm">{description}</p>
      )}
      {action}
    </div>
  )
}
