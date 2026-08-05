import { type SyntheticEvent, type ReactNode, useId, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSignedInUser } from '@/features/auth/current-user'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'
import { useUpdateDisplayName } from '@/features/auth/queries'

import { SettingsSection } from './section'

/**
 * Your name and your address.
 *
 * The address is shown and not editable, and the sentence next to it says why:
 * it is what a federated sign-in links an account by, so changing it here could
 * move somebody else's single sign-on onto this account (`PATCH /users/me` has
 * no field for it, which is the stronger half of the same statement).
 *
 * The platform role is shown for the same reason — it is the answer to "why can
 * I not see the admin screens" — and is likewise not something this form could
 * ask to change.
 */
export function ProfilePanel(): ReactNode {
  const user = useSignedInUser()
  const rename = useUpdateDisplayName()
  const [displayName, setDisplayName] = useState(user.displayName)

  const nameId = useId()
  const nameErrorId = useId()
  const nameError = fieldErrorOf(rename.error, 'displayName')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    rename.mutate(
      { displayName: displayName.trim() },
      {
        onSuccess: () => {
          toast.success('Your display name was changed.')
        },
      },
    )
  }

  const unchanged = displayName.trim() === user.displayName || displayName.trim() === ''

  return (
    <SettingsSection
      title="Profile"
      description="How you appear to everybody else on this installation."
    >
      <form className="flex max-w-md flex-col gap-4" onSubmit={onSubmit} noValidate>
        {rename.error !== null && nameError === undefined && (
          <FormAlert message="That change could not be saved." error={rename.error} />
        )}

        <div className="flex flex-col gap-2">
          <Label htmlFor={nameId}>Display name</Label>
          <Input
            id={nameId}
            value={displayName}
            maxLength={200}
            onChange={(event) => {
              setDisplayName(event.target.value)
            }}
            aria-describedby={nameError === undefined ? undefined : nameErrorId}
            aria-invalid={nameError !== undefined}
          />
          <FieldError id={nameErrorId} message={nameError} />
        </div>

        <dl className="text-sm">
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-32">Email address</dt>
            <dd>{user.email}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-32">Platform role</dt>
            <dd>{user.platformRole === 'admin' ? 'Administrator' : 'Member'}</dd>
          </div>
        </dl>

        <p className="text-muted-foreground text-xs">
          Neither of those can be changed here. The address is what a single sign-on identity is
          matched by, and the role is an administrator’s decision.
        </p>

        <Button type="submit" className="self-start" disabled={rename.isPending || unchanged}>
          {rename.isPending ? 'Saving…' : 'Save name'}
        </Button>
      </form>
    </SettingsSection>
  )
}
