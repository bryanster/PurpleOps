import { type SyntheticEvent, type ReactNode, useId, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'
import { useChangePassword } from '@/features/auth/queries'

import { SettingsSection } from './section'

/**
 * Change your own password.
 *
 * Two things are worth knowing before you press the button, and both are said
 * on the screen rather than discovered afterwards: the current password is
 * required, and every *other* session you hold ends. The second is the reason
 * this panel does not need a "sign out everywhere" of its own.
 *
 * The password policy is not restated here. It lives on the server (M1-002),
 * which reports a violation as a field error on `newPassword` — so what the
 * user reads is the rule that was actually applied, rather than a second copy
 * of it that drifted.
 */
export function PasswordPanel(): ReactNode {
  const change = useChangePassword()

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [mismatch, setMismatch] = useState<string | undefined>(undefined)

  const currentId = useId()
  const newId = useId()
  const confirmId = useId()
  const currentErrorId = useId()
  const newErrorId = useId()
  const confirmErrorId = useId()

  const currentError = fieldErrorOf(change.error, 'currentPassword')
  const newError = fieldErrorOf(change.error, 'newPassword')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()

    // The one rule checked here rather than on the server, because the server
    // has no way to check it: it never sees the confirmation field.
    if (newPassword !== confirmation) {
      setMismatch('The two new passwords do not match.')
      return
    }
    setMismatch(undefined)

    change.mutate(
      { currentPassword, newPassword },
      {
        onSuccess: () => {
          setCurrentPassword('')
          setNewPassword('')
          setConfirmation('')
          toast.success('Your password was changed.', {
            description: 'Your other sessions have been signed out.',
          })
        },
      },
    )
  }

  return (
    <SettingsSection
      title="Password"
      description="Changing your password signs you out of every other browser you are signed in on. This one keeps working."
    >
      <form className="flex max-w-md flex-col gap-4" onSubmit={onSubmit} noValidate>
        {change.error !== null && currentError === undefined && newError === undefined && (
          <FormAlert message={describeFailure(change.error)} error={change.error} />
        )}

        <div className="flex flex-col gap-2">
          <Label htmlFor={currentId}>Current password</Label>
          <Input
            id={currentId}
            type="password"
            autoComplete="current-password"
            required
            value={currentPassword}
            onChange={(event) => {
              setCurrentPassword(event.target.value)
            }}
            aria-describedby={currentError === undefined ? undefined : currentErrorId}
            aria-invalid={currentError !== undefined}
          />
          <FieldError id={currentErrorId} message={currentError} />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={newId}>New password</Label>
          <Input
            id={newId}
            type="password"
            autoComplete="new-password"
            required
            value={newPassword}
            onChange={(event) => {
              setNewPassword(event.target.value)
            }}
            aria-describedby={newError === undefined ? undefined : newErrorId}
            aria-invalid={newError !== undefined}
          />
          <FieldError id={newErrorId} message={newError} />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={confirmId}>New password again</Label>
          <Input
            id={confirmId}
            type="password"
            autoComplete="new-password"
            required
            value={confirmation}
            onChange={(event) => {
              setConfirmation(event.target.value)
            }}
            aria-describedby={mismatch === undefined ? undefined : confirmErrorId}
            aria-invalid={mismatch !== undefined}
          />
          <FieldError id={confirmErrorId} message={mismatch} />
        </div>

        <Button
          type="submit"
          className="self-start"
          disabled={change.isPending || currentPassword === '' || newPassword === ''}
        >
          {change.isPending ? 'Changing…' : 'Change password'}
        </Button>
      </form>
    </SettingsSection>
  )
}

function describeFailure(error: unknown): string {
  if (isApiError(error, 'unauthenticated')) {
    return 'That is not your current password.'
  }
  return 'That password could not be changed.'
}
