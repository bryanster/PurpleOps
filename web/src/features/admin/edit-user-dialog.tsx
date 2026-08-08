import { type ReactNode, type SyntheticEvent, useId, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useSignedInUser } from '@/features/auth/current-user'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'

import {
  useUpdateUser,
  type PlatformRole,
  type UpdateUserRequest,
  type User,
  type UserStatus,
} from './queries'

/**
 * Edit one account.
 *
 * A patch, not a replacement: only the fields that were actually changed are
 * sent, so two administrators editing different things at the same time do not
 * overwrite each other (M1-016). That is why this builds its body by comparing
 * against the account it opened with rather than sending the whole form.
 *
 * The consequences that are not obvious from the controls are written next to
 * them — that setting the status to disabled ends every session, and that
 * demoting yourself is possible and immediate.
 */
export function EditUserDialog({
  user,
  onOpenChange,
}: {
  user: User | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  if (user === undefined) {
    return null
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        {/* Keyed by the account, and holding the form state itself: opening a
            different row mounts a fresh body seeded from that row, so there is
            no effect re-seeding fields and no window in which the form shows
            one person's name over another's identifier. */}
        <EditUserDialogBody
          key={user.id}
          account={user}
          onDone={() => {
            onOpenChange(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

function EditUserDialogBody({ account, onDone }: { account: User; onDone: () => void }): ReactNode {
  const me = useSignedInUser()
  const update = useUpdateUser()

  // Seeded from the account this body was mounted for. No effect keeps them in
  // step, because a remount is what a different account produces.
  const [displayName, setDisplayName] = useState(account.displayName)
  const [platformRole, setPlatformRole] = useState<PlatformRole>(account.platformRole)
  const [status, setStatus] = useState<UserStatus>(account.status)
  const [mfaEnforced, setMfaEnforced] = useState(account.mfaEnforced)

  const nameId = useId()
  const roleId = useId()
  const statusId = useId()
  const mfaId = useId()
  const nameErrorId = useId()

  const patch: UpdateUserRequest = {
    ...(displayName.trim() === account.displayName ? {} : { displayName: displayName.trim() }),
    ...(platformRole === account.platformRole ? {} : { platformRole }),
    ...(status === account.status ? {} : { status }),
    ...(mfaEnforced === account.mfaEnforced ? {} : { mfaEnforced }),
  }
  const changed = Object.keys(patch).length > 0
  const nameError = fieldErrorOf(update.error, 'displayName')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    if (!changed) {
      return
    }
    update.mutate(
      { userId: account.id, patch },
      {
        onSuccess: (saved) => {
          onDone()
          toast.success(`${saved.displayName} was updated.`)
        },
      },
    )
  }

  const editingSelf = account.id === me.id

  return (
    <>
      <DialogHeader>
        <DialogTitle>Edit {account.displayName}</DialogTitle>
        <DialogDescription>
          {account.email} — the address is not editable, because it is what a single sign-on
          identity is matched by.
        </DialogDescription>
      </DialogHeader>

      <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {update.error !== null && nameError === undefined && (
          <FormAlert message={describeFailure(update.error)} error={update.error} />
        )}

        <div className="flex flex-col gap-2">
          <Label htmlFor={nameId}>Display name</Label>
          <Input
            id={nameId}
            maxLength={200}
            value={displayName}
            onChange={(event) => {
              setDisplayName(event.target.value)
            }}
            aria-describedby={nameError === undefined ? undefined : nameErrorId}
            aria-invalid={nameError !== undefined}
          />
          <FieldError id={nameErrorId} message={nameError} />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={roleId}>Platform role</Label>
          <Select
            value={platformRole}
            onValueChange={(value) => {
              setPlatformRole(value as PlatformRole)
            }}
          >
            <SelectTrigger id={roleId}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="member">Member</SelectItem>
              <SelectItem value="admin">Administrator</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-muted-foreground text-xs">
            {editingSelf && platformRole === 'member' && account.platformRole === 'admin'
              ? 'This is your own account: demoting it takes effect on your next request, and you will lose these screens immediately.'
              : 'Takes effect at their next request. They do not need to sign in again.'}
          </p>
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={statusId}>Status</Label>
          <Select
            value={status}
            onValueChange={(value) => {
              setStatus(value as UserStatus)
            }}
          >
            <SelectTrigger id={statusId}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="invited">Invited</SelectItem>
              <SelectItem value="disabled">Disabled</SelectItem>
            </SelectContent>
          </Select>
          {status === 'disabled' && account.status !== 'disabled' && (
            <p className="text-muted-foreground text-xs">
              Disabling signs them out of every browser and stops every service token they own at
              its next request.
            </p>
          )}
        </div>

        <div className="flex items-start gap-2">
          <Checkbox
            id={mfaId}
            checked={mfaEnforced}
            onCheckedChange={(checked) => {
              setMfaEnforced(checked === true)
            }}
          />
          <div className="flex flex-col gap-0.5">
            <Label htmlFor={mfaId} className="font-normal">
              Require a second factor
            </Label>
            <p className="text-muted-foreground text-xs">
              Turning this on confines them to the enrolment screen until they set up an
              authenticator — including on sessions they already have open.
            </p>
          </div>
        </div>

        <Button type="submit" disabled={update.isPending || !changed}>
          {update.isPending ? 'Saving…' : 'Save changes'}
        </Button>
      </form>
    </>
  )
}

function describeFailure(error: unknown): string {
  if (isApiError(error, 'conflict')) {
    // The server's sentence names the rule — the last active administrator
    // cannot be demoted or disabled — and is better than anything generic.
    return error instanceof Error ? error.message : 'That change was refused.'
  }
  return 'Those changes could not be saved.'
}
