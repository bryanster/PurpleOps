import { AlertTriangleIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { ApiError } from '@/api/errors'
import { cn } from '@/lib/utils'

/**
 * What a form says when the server refused it.
 *
 * `role="alert"` so that a screen reader announces the refusal without the user
 * having to go looking for it, and the request ID so that "it says an error
 * occurred" becomes a line an operator can find in a log (M0B-007).
 *
 * `message` is passed in rather than taken from the error, because the screen
 * usually knows better: a 401 from the login form must read the same whether
 * the address was wrong or the password was, and the way to guarantee that is
 * to write one sentence here rather than to render whatever the server said.
 */
export function FormAlert({
  message,
  error,
  className,
}: {
  message: ReactNode
  error?: unknown
  className?: string
}): ReactNode {
  const requestId = error instanceof ApiError ? error.requestId : undefined

  return (
    <div
      role="alert"
      className={cn(
        'border-destructive/40 bg-destructive/10 text-destructive flex gap-2 rounded-md border p-3 text-sm',
        className,
      )}
    >
      <AlertTriangleIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <div className="flex flex-col gap-1">
        <div>{message}</div>
        {requestId !== undefined && (
          <div className="text-destructive/80 text-xs">
            Request <code className="font-mono">{requestId}</code>
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * A field's error, rendered where `aria-describedby` can point at it.
 *
 * Returning `null` rather than an empty element matters: an always-present
 * empty node referenced by `aria-describedby` is announced as nothing, which
 * reads to a screen-reader user as a field with a description they cannot hear.
 */
export function FieldError({ id, message }: { id: string; message?: string }): ReactNode {
  if (message === undefined) {
    return null
  }
  return (
    <p id={id} className="text-destructive text-sm">
      {message}
    </p>
  )
}
