import { ShieldAlertIcon, ShieldCheckIcon } from 'lucide-react'
import { type ReactNode, type SyntheticEvent, useEffect, useId, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSignedInUser } from '@/features/auth/current-user'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'
import { CODE_LENGTH, OtpInput } from '@/features/auth/otp-input'
import {
  useConfirmTotp,
  useDisableTotp,
  useEnrollTotp,
  useMarkEnrolmentComplete,
  useRegenerateRecoveryCodes,
  type RecoveryCodes,
} from '@/features/auth/queries'
import { RecoveryCodesPanel } from '@/features/auth/recovery-codes'

import { SettingsSection } from './section'

/**
 * Two-factor authentication, from the account screen.
 *
 * The panel reads `mfa` off `GET /auth/me` and says the three separate things
 * that state holds, because conflating them is the defect M1-008 exists to fix:
 * whether a factor is *required* of this person, whether they *have* one, and
 * whether this session *presented* one. An interface that showed only the
 * second would be repeating v1's mistake in a nicer font.
 *
 * Removing an authenticator is refused by the server while a factor is required,
 * and `mfa.required` says so in advance — so the button is disabled with the
 * reason beside it rather than offered and then refused.
 *
 * Each dialog below is a shell and a body. The body holds the state, so closing
 * the dialog unmounts it and the secret, the codes and the typed password stop
 * existing without anything having to remember to clear them.
 */
export function MfaPanel(): ReactNode {
  const user = useSignedInUser()
  const [enrolling, setEnrolling] = useState(false)
  const [regenerating, setRegenerating] = useState(false)
  const [disabling, setDisabling] = useState(false)

  const { enrolled, required, satisfied, recoveryCodesRemaining } = user.mfa

  return (
    <SettingsSection
      title="Two-factor authentication"
      description="A code from an authenticator app, on top of your password. Recovery codes stand in for it if you lose the device."
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          {enrolled ? (
            <Badge className="gap-1">
              <ShieldCheckIcon className="size-3" aria-hidden="true" />
              Authenticator set up
            </Badge>
          ) : (
            <Badge variant="secondary" className="gap-1">
              <ShieldAlertIcon className="size-3" aria-hidden="true" />
              No authenticator
            </Badge>
          )}
          {required && <Badge variant="outline">Required on your account</Badge>}
          {enrolled && !satisfied && (
            <Badge variant="outline">This session did not present a code</Badge>
          )}
        </div>

        {enrolled && (
          <p className="text-sm">
            {recoveryCodesRemaining} recovery {recoveryCodesRemaining === 1 ? 'code' : 'codes'}{' '}
            left.
            {recoveryCodesRemaining < 3 && (
              <span className="text-destructive">
                {' '}
                That is few enough to be worth replacing — a lost phone with no codes left means an
                administrator has to intervene.
              </span>
            )}
          </p>
        )}

        <div className="flex flex-wrap gap-2">
          {!enrolled && (
            <Button
              onClick={() => {
                setEnrolling(true)
              }}
            >
              Set up an authenticator
            </Button>
          )}

          {enrolled && (
            <>
              <Button
                variant="outline"
                onClick={() => {
                  setRegenerating(true)
                }}
              >
                Replace recovery codes
              </Button>
              <Button
                variant="outline"
                disabled={required}
                onClick={() => {
                  setDisabling(true)
                }}
              >
                Remove authenticator
              </Button>
            </>
          )}
        </div>

        {enrolled && required && (
          <p className="text-muted-foreground text-sm">
            An administrator requires a second factor on your account, so it cannot be removed —
            doing so would leave you unable to sign in at all. Ask them to lift the requirement
            first.
          </p>
        )}
      </div>

      <Dialog open={enrolling} onOpenChange={setEnrolling}>
        <DialogContent className="max-h-[85vh] overflow-y-auto">
          <EnrolmentDialogBody
            onDone={() => {
              setEnrolling(false)
              toast.success('Two-factor authentication is on for your account.')
            }}
          />
        </DialogContent>
      </Dialog>

      <Dialog open={regenerating} onOpenChange={setRegenerating}>
        <DialogContent className="max-h-[85vh] overflow-y-auto">
          <RegenerateDialogBody
            onDone={() => {
              setRegenerating(false)
              toast.success('Your recovery codes were replaced.')
            }}
          />
        </DialogContent>
      </Dialog>

      <Dialog open={disabling} onOpenChange={setDisabling}>
        <DialogContent>
          <DisableDialogBody
            onDone={() => {
              setDisabling(false)
              toast.success('Your authenticator was removed.')
            }}
          />
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

/**
 * Voluntary enrolment: the same two calls as the forced-enrolment screen, and
 * the same rule about the recovery codes — shown once, from component state,
 * behind a deliberate acknowledgement.
 */
function EnrolmentDialogBody({ onDone }: { onDone: () => void }): ReactNode {
  const enroll = useEnrollTotp()
  const confirm = useConfirmTotp()
  const complete = useMarkEnrolmentComplete()
  const [code, setCode] = useState('')
  const [codes, setCodes] = useState<RecoveryCodes | undefined>(undefined)
  const codeId = useId()

  // Once, on mount. An unconfirmed secret gates nothing, but calling this again
  // replaces it — which would invalidate whatever has just been scanned.
  const { mutate: startEnrolment } = enroll
  useEffect(() => {
    startEnrolment()
  }, [startEnrolment])

  function submit(value: string): void {
    if (confirm.isPending || value.length !== CODE_LENGTH) {
      return
    }
    confirm.mutate(
      { code: value },
      {
        onSuccess: setCodes,
        onError: () => {
          setCode('')
        },
      },
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{codes ? 'Save your recovery codes' : 'Set up an authenticator'}</DialogTitle>
        <DialogDescription>
          {codes
            ? 'This is the only time they are shown.'
            : 'Scan the code with an authenticator app, then enter what it shows.'}
        </DialogDescription>
      </DialogHeader>

      {codes ? (
        <RecoveryCodesPanel
          codes={codes}
          heading="Ten single-use codes"
          continueLabel="Done"
          onContinue={() => {
            // The refetch happens here rather than when the code was confirmed,
            // for the reason useConfirmTotp gives: until this moment the codes
            // on screen are the only copy in existence.
            void complete().then(onDone)
          }}
        />
      ) : (
        <div className="flex flex-col gap-4">
          {enroll.isPending && <PageLoading label="Generating a secret…" />}
          {enroll.error && (
            <PageError
              error={enroll.error}
              onRetry={() => {
                enroll.mutate()
              }}
            />
          )}
          {enroll.data && (
            <form
              className="flex flex-col gap-4"
              onSubmit={(event: SyntheticEvent) => {
                event.preventDefault()
                submit(code)
              }}
              noValidate
            >
              <img
                src={enroll.data.qrCode}
                alt="QR code containing your authenticator secret"
                className="bg-background size-40 self-start rounded-md border p-2"
              />
              <code className="bg-muted/50 rounded-md border p-2 font-mono text-xs break-all select-all">
                {enroll.data.secret}
              </code>

              <div className="flex flex-col gap-2">
                <Label htmlFor={codeId}>Code from the app</Label>
                <OtpInput
                  id={codeId}
                  value={code}
                  onChange={setCode}
                  onComplete={submit}
                  disabled={confirm.isPending}
                />
              </div>

              {confirm.error !== null && (
                <FormAlert
                  message="That code was not right. Codes change every 30 seconds — try the one showing now."
                  error={confirm.error}
                />
              )}

              <Button type="submit" disabled={confirm.isPending || code.length !== CODE_LENGTH}>
                {confirm.isPending ? 'Checking…' : 'Confirm'}
              </Button>
            </form>
          )}
        </div>
      )}
    </>
  )
}

/** Replace the recovery codes. Every previous one dies, including unused ones. */
function RegenerateDialogBody({ onDone }: { onDone: () => void }): ReactNode {
  const regenerate = useRegenerateRecoveryCodes()
  const [currentPassword, setCurrentPassword] = useState('')
  const [codes, setCodes] = useState<RecoveryCodes | undefined>(undefined)
  const passwordId = useId()
  const errorId = useId()

  const passwordError = fieldErrorOf(regenerate.error, 'currentPassword')

  return (
    <>
      <DialogHeader>
        <DialogTitle>Replace your recovery codes</DialogTitle>
        <DialogDescription>
          Ten new codes, and every code you hold now stops working — including the ones you have not
          used.
        </DialogDescription>
      </DialogHeader>

      {codes ? (
        <RecoveryCodesPanel
          codes={codes}
          heading="Your new codes"
          continueLabel="Done"
          onContinue={onDone}
        />
      ) : (
        <form
          className="flex flex-col gap-4"
          onSubmit={(event: SyntheticEvent) => {
            event.preventDefault()
            regenerate.mutate({ currentPassword }, { onSuccess: setCodes })
          }}
          noValidate
        >
          {regenerate.error !== null && passwordError === undefined && (
            <FormAlert
              message={describeRegenerateFailure(regenerate.error)}
              error={regenerate.error}
            />
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={passwordId}>Your password</Label>
            <Input
              id={passwordId}
              type="password"
              autoComplete="current-password"
              required
              value={currentPassword}
              onChange={(event) => {
                setCurrentPassword(event.target.value)
              }}
              aria-describedby={passwordError === undefined ? undefined : errorId}
              aria-invalid={passwordError !== undefined}
            />
            <FieldError id={errorId} message={passwordError} />
            <p className="text-muted-foreground text-xs">
              Asked for so that a session left open on a shared machine cannot mint credentials that
              walk past your second factor.
            </p>
          </div>

          <Button type="submit" disabled={regenerate.isPending || currentPassword === ''}>
            {regenerate.isPending ? 'Replacing…' : 'Replace codes'}
          </Button>
        </form>
      )}
    </>
  )
}

/** Remove the authenticator, and the recovery codes with it. */
function DisableDialogBody({ onDone }: { onDone: () => void }): ReactNode {
  const disable = useDisableTotp()
  const [currentPassword, setCurrentPassword] = useState('')
  const passwordId = useId()
  const errorId = useId()

  const passwordError = fieldErrorOf(disable.error, 'currentPassword')

  return (
    <>
      <DialogHeader>
        <DialogTitle>Remove your authenticator</DialogTitle>
        <DialogDescription>
          Your recovery codes go with it — they stand in for the authenticator, so leaving them
          behind would mean a removed second factor that could still be presented. Signing in will
          need only your password afterwards.
        </DialogDescription>
      </DialogHeader>

      <form
        className="flex flex-col gap-4"
        onSubmit={(event: SyntheticEvent) => {
          event.preventDefault()
          disable.mutate({ currentPassword }, { onSuccess: onDone })
        }}
        noValidate
      >
        {disable.error !== null && passwordError === undefined && (
          <FormAlert message={describeDisableFailure(disable.error)} error={disable.error} />
        )}

        <div className="flex flex-col gap-2">
          <Label htmlFor={passwordId}>Your password</Label>
          <Input
            id={passwordId}
            type="password"
            autoComplete="current-password"
            required
            value={currentPassword}
            onChange={(event) => {
              setCurrentPassword(event.target.value)
            }}
            aria-describedby={passwordError === undefined ? undefined : errorId}
            aria-invalid={passwordError !== undefined}
          />
          <FieldError id={errorId} message={passwordError} />
        </div>

        <Button
          type="submit"
          variant="destructive"
          disabled={disable.isPending || currentPassword === ''}
        >
          {disable.isPending ? 'Removing…' : 'Remove authenticator'}
        </Button>
      </form>
    </>
  )
}

function describeRegenerateFailure(error: unknown): string {
  if (isApiError(error, 'forbidden')) {
    return 'This session has not presented a second factor, so it cannot mint new codes. Sign in again with a code first.'
  }
  if (isApiError(error, 'conflict')) {
    return 'There is no authenticator to replace codes for.'
  }
  return 'Those codes could not be replaced.'
}

function describeDisableFailure(error: unknown): string {
  if (isApiError(error, 'forbidden')) {
    return 'A second factor is required of your account, so it cannot be removed.'
  }
  return 'The authenticator could not be removed.'
}
