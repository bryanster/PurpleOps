import { type ReactNode, useEffect, useRef } from 'react'

import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/** How many digits an authenticator code has. The server's `TOTPCodeRequest`. */
export const CODE_LENGTH = 6

/**
 * The six-digit field, as one input rather than six boxes.
 *
 * Six separate inputs are the fashionable shape and the wrong one here. They
 * break paste in several browsers, they announce as six unlabelled fields to a
 * screen reader, and they make "select all and retype" — what somebody does
 * when their first attempt was of a code that has since rolled over — a
 * fiddle. One input with `autoComplete="one-time-code"` gets the iOS and
 * Android keyboard suggestion for free, and paste is just paste.
 *
 * Everything that is not a digit is dropped on the way in, so a code pasted as
 * "492 817" from an app that puts a space in the middle works, and the field
 * cannot hold a value the server would reject as malformed.
 *
 * `onComplete` fires the moment six digits are present — from typing, from
 * pasting, or from the platform's own autofill. That is the auto-submit, and it
 * is why the caller must make submitting idempotent while a request is in
 * flight (both callers disable the form).
 */
export function OtpInput({
  id,
  value,
  onChange,
  onComplete,
  disabled,
  describedBy,
  label = 'Six-digit code',
  autoFocus = true,
}: {
  id: string
  value: string
  onChange: (value: string) => void
  onComplete?: (value: string) => void
  disabled?: boolean
  describedBy?: string
  label?: string
  autoFocus?: boolean
}): ReactNode {
  const ref = useRef<HTMLInputElement>(null)

  // Focused on mount rather than with the `autoFocus` attribute: React's
  // autoFocus is applied at hydration, and this field is often rendered after
  // a navigation, where nothing hydrates. The ref runs either way.
  useEffect(() => {
    if (autoFocus) {
      ref.current?.focus()
    }
  }, [autoFocus])

  return (
    <Input
      ref={ref}
      id={id}
      name="code"
      value={value}
      disabled={disabled}
      // Not type="number": a spinner on a code is meaningless, and Firefox
      // silently drops a leading zero from a numeric field — which is one code
      // in ten arriving as five digits.
      type="text"
      inputMode="numeric"
      autoComplete="one-time-code"
      pattern="[0-9]*"
      // Deliberately no maxLength. The browser applies it to the raw value
      // *before* this component strips the non-digits, so pasting "492 817" —
      // which is how several authenticators render a code — would be truncated
      // to "492 81" and arrive here as five digits. The slice below is the
      // limit, and it counts the right characters.
      aria-label={label}
      aria-describedby={describedBy}
      className={cn('text-center font-mono text-2xl tracking-[0.4em]')}
      onChange={(event) => {
        const digits = event.target.value.replace(/\D/g, '').slice(0, CODE_LENGTH)
        onChange(digits)
        if (digits.length === CODE_LENGTH) {
          onComplete?.(digits)
        }
      }}
    />
  )
}
