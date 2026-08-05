import { CheckIcon, CopyIcon } from 'lucide-react'
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
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'

import { useCreateUser, type CreatedUser, type PlatformRole } from './queries'

/**
 * Create an account (M1-016).
 *
 * The password field is what decides which of two accounts this is, and the
 * form says so rather than leaving an administrator to find out: with one, a
 * local account that is active immediately; without one, an account that exists
 * for single sign-on to claim on its first successful sign-in.
 *
 * There is no mail transport in this deployment, so nothing is sent. The
 * response carries the link to pass on, and this dialog stays open showing it —
 * an administrator who closed it before copying would have to go and find the
 * sign-in URL themselves.
 */
export function CreateUserDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        {/* A component of its own, so closing the dialog unmounts every field —
            including the password, which should not survive in memory because
            somebody changed their mind. */}
        <CreateUserDialogBody
          onDone={() => {
            onOpenChange(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

function CreateUserDialogBody({ onDone }: { onDone: () => void }): ReactNode {
  const create = useCreateUser()

  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [platformRole, setPlatformRole] = useState<PlatformRole>('member')
  const [withPassword, setWithPassword] = useState(true)
  const [password, setPassword] = useState('')
  const [mfaEnforced, setMfaEnforced] = useState(false)
  const [created, setCreated] = useState<CreatedUser | undefined>(undefined)

  const emailId = useId()
  const nameId = useId()
  const roleId = useId()
  const passwordId = useId()
  const withPasswordId = useId()
  const mfaId = useId()
  const emailErrorId = useId()
  const passwordErrorId = useId()

  const emailError = fieldErrorOf(create.error, 'email')
  const passwordError = fieldErrorOf(create.error, 'password')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    create.mutate(
      {
        email: email.trim(),
        displayName: displayName.trim(),
        platformRole,
        ...(withPassword ? { password } : {}),
        ...(mfaEnforced ? { mfaEnforced } : {}),
      },
      { onSuccess: setCreated },
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{created ? 'Account created' : 'New user'}</DialogTitle>
        <DialogDescription>
          {created
            ? 'Nothing was emailed — this deployment has no mail transport. Pass these on yourself.'
            : 'An account on this installation. You can change everything except the address afterwards.'}
        </DialogDescription>
      </DialogHeader>

      {created ? (
        <CreatedUserPanel
          created={created}
          onDone={() => {
            onDone()
            toast.success(`${created.user.displayName} can now be signed in as.`)
          }}
        />
      ) : (
        <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
          {create.error !== null && emailError === undefined && passwordError === undefined && (
            <FormAlert message={describeFailure(create.error)} error={create.error} />
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={emailId}>Email address</Label>
            <Input
              id={emailId}
              type="email"
              autoFocus
              required
              value={email}
              onChange={(event) => {
                setEmail(event.target.value)
              }}
              aria-describedby={emailError === undefined ? undefined : emailErrorId}
              aria-invalid={emailError !== undefined}
            />
            <FieldError id={emailErrorId} message={emailError} />
            <p className="text-muted-foreground text-xs">
              Not editable afterwards: it is what a single sign-on identity is matched by.
            </p>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor={nameId}>Display name</Label>
            <Input
              id={nameId}
              required
              maxLength={200}
              value={displayName}
              onChange={(event) => {
                setDisplayName(event.target.value)
              }}
            />
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
              An administrator can read and change every account, every setting and the activity
              log.
            </p>
          </div>

          <div className="flex items-start gap-2">
            <Checkbox
              id={withPasswordId}
              checked={withPassword}
              onCheckedChange={(checked) => {
                setWithPassword(checked === true)
              }}
            />
            <div className="flex flex-col gap-0.5">
              <Label htmlFor={withPasswordId} className="font-normal">
                Set a password now
              </Label>
              <p className="text-muted-foreground text-xs">
                Ticked: a local account, active immediately, and you tell them the password.
                Unticked: an invited account that the first successful single sign-on claims.
              </p>
            </div>
          </div>

          {withPassword && (
            <div className="flex flex-col gap-2">
              <Label htmlFor={passwordId}>Password</Label>
              <Input
                id={passwordId}
                type="password"
                autoComplete="new-password"
                required
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value)
                }}
                aria-describedby={passwordError === undefined ? undefined : passwordErrorId}
                aria-invalid={passwordError !== undefined}
              />
              <FieldError id={passwordErrorId} message={passwordError} />
              <p className="text-muted-foreground text-xs">
                They should change it once they are in. The policy is the server’s, and it will say
                here if this one does not meet it.
              </p>
            </div>
          )}

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
                They will be made to set up an authenticator before they can do anything else.
              </p>
            </div>
          </div>

          <Button
            type="submit"
            disabled={
              create.isPending ||
              email.trim() === '' ||
              displayName.trim() === '' ||
              (withPassword && password === '')
            }
          >
            {create.isPending ? 'Creating…' : 'Create account'}
          </Button>
        </form>
      )}
    </>
  )
}

function CreatedUserPanel({
  created,
  onDone,
}: {
  created: CreatedUser
  onDone: () => void
}): ReactNode {
  const [copied, setCopied] = useState(false)

  return (
    <div className="flex flex-col gap-4">
      <dl className="text-sm">
        <div className="flex gap-2">
          <dt className="text-muted-foreground w-28">Name</dt>
          <dd>{created.user.displayName}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="text-muted-foreground w-28">Email</dt>
          <dd>{created.user.email}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="text-muted-foreground w-28">Status</dt>
          <dd>{created.user.status}</dd>
        </div>
      </dl>

      <div className="flex flex-col gap-2">
        <Label>Where they sign in</Label>
        <code className="bg-muted/50 rounded-md border p-3 font-mono text-xs break-all select-all">
          {created.inviteUrl}
        </code>
        <p className="text-muted-foreground text-xs">
          The link carries no credential and grants nothing — it is simply this installation’s
          sign-in page. Send the password separately, by whatever your organisation uses for
          secrets.
        </p>
      </div>

      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() => {
          navigator.clipboard.writeText(created.inviteUrl).then(
            () => {
              setCopied(true)
            },
            () => {
              setCopied(false)
            },
          )
        }}
      >
        {copied ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
        {copied ? 'Copied' : 'Copy link'}
      </Button>

      <Button type="button" onClick={onDone}>
        Done
      </Button>
    </div>
  )
}

function describeFailure(error: unknown): string {
  if (isApiError(error, 'conflict')) {
    return 'An account with that address already exists — in any casing. Search for it rather than creating a second one.'
  }
  return 'That account could not be created.'
}
