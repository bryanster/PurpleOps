import { type SyntheticEvent, type ReactNode, useEffect, useId, useState } from 'react'
import { useNavigate } from 'react-router'

import { ApiError, isApiError } from '@/api/errors'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'

import { AuthLayout } from './auth-layout'
import { FormAlert } from './form-alert'
import { CODE_LENGTH, OtpInput } from './otp-input'
import {
  useConfirmTotp,
  useEnrollTotp,
  useLogout,
  useMarkEnrolmentComplete,
  type RecoveryCodes,
} from './queries'
import { RecoveryCodesPanel } from './recovery-codes'

/**
 * Forced enrolment (M1-008): the screen somebody sees when a second factor is
 * required of them and they have none.
 *
 * It is a blocking screen in the literal sense. There is no application shell
 * around it, no navigation out of it, and no route that renders anything else
 * while this state holds — `RequireAuth` redirects every in-app path back here,
 * and the server refuses every endpoint but the two enrolment ones with a 403.
 * A screen that could be dismissed would recreate exactly the hole M1-008
 * closes: v1 asked "have you enrolled?" and let anybody who skipped the form in
 * with a password alone.
 *
 * What it is *not* is punitive. It explains why it is there, offers the secret
 * three ways so that a camera which will not focus is not a dead end, and ends
 * by putting the recovery codes in front of the person once.
 *
 * The one way out is signing out, which leaves the account exactly as it was.
 */
export function EnrolmentPage(): ReactNode {
  const navigate = useNavigate()
  const enroll = useEnrollTotp()
  const confirm = useConfirmTotp()
  const complete = useMarkEnrolmentComplete()
  const logout = useLogout()

  const [code, setCode] = useState('')
  const [codes, setCodes] = useState<RecoveryCodes | undefined>(undefined)

  const codeId = useId()
  const errorId = useId()

  // Started once, on mount. An unconfirmed secret gates nothing and calling the
  // endpoint again replaces it, so a re-render must not mint a new one — that
  // would invalidate whatever the person has just scanned.
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
        onSuccess: (minted) => {
          // Held in component state and nowhere else: not the query cache, not
          // storage. When this component unmounts they are gone, which is the
          // same promise the server makes.
          setCodes(minted)
        },
        onError: () => {
          setCode('')
        },
      },
    )
  }

  if (codes !== undefined) {
    return (
      <AuthLayout
        title="Almost done"
        description="Your authenticator is set up. One thing left, and it only happens once."
      >
        <RecoveryCodesPanel
          codes={codes}
          continueLabel="Continue to Blacklight"
          onContinue={() => {
            // Only now: refetching earlier would have moved the guard out from
            // under this screen while it was still the only place the codes
            // existed (see useConfirmTotp). By this point they have been saved.
            void complete().then(() => navigate('/', { replace: true }))
          }}
        />
      </AuthLayout>
    )
  }

  return (
    <AuthLayout
      title="Set up two-factor authentication"
      description="An administrator requires a second factor on your account. Until one is set up, this is the only screen available to you."
      footer={
        <button
          type="button"
          className="text-muted-foreground hover:text-foreground underline"
          onClick={() => {
            logout.mutate(undefined, {
              onSuccess: () => {
                void navigate('/login', { replace: true })
              },
            })
          }}
        >
          Sign out instead
        </button>
      }
    >
      <div className="flex flex-col gap-6">
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
          <>
            <ol className="flex flex-col gap-4 text-sm">
              <li className="flex flex-col gap-2">
                <span className="font-medium">1. Scan this with an authenticator app</span>
                <img
                  // A data: URI the server rendered (M1-006), not markup it
                  // supplied — the content security policy allows img-src data:
                  // and allows no inline SVG.
                  src={enroll.data.qrCode}
                  alt="QR code containing your authenticator secret"
                  className="bg-background size-44 self-start rounded-md border p-2"
                />
              </li>

              <li className="flex flex-col gap-2">
                <span className="font-medium">…or type this key in by hand</span>
                <code className="bg-muted/50 rounded-md border p-3 font-mono text-sm break-all select-all">
                  {enroll.data.secret}
                </code>
                <span className="text-muted-foreground text-xs">
                  Anyone holding this key can produce your codes. Do not paste it into a chat.
                </span>
              </li>
            </ol>

            <form
              className="flex flex-col gap-4"
              onSubmit={(event: SyntheticEvent) => {
                event.preventDefault()
                submit(code)
              }}
              noValidate
            >
              <div className="flex flex-col gap-2">
                <Label htmlFor={codeId}>2. Enter the code it shows</Label>
                <OtpInput
                  id={codeId}
                  value={code}
                  onChange={setCode}
                  onComplete={submit}
                  disabled={confirm.isPending}
                  describedBy={confirm.error === null ? undefined : errorId}
                  label="Six-digit code from your authenticator"
                />
              </div>

              {confirm.error !== null && (
                <FormAlert message={describeFailure(confirm.error)} error={confirm.error} />
              )}
              <span id={errorId} className="sr-only">
                {describeFailure(confirm.error)}
              </span>

              <Button type="submit" disabled={confirm.isPending || code.length !== CODE_LENGTH}>
                {confirm.isPending ? 'Checking…' : 'Confirm'}
              </Button>
            </form>
          </>
        )}
      </div>
    </AuthLayout>
  )
}

/**
 * What to say about a refused confirmation.
 *
 * A wrong code here is a 400 naming the `code` field rather than a 401: the
 * caller is signed in, and this is a form to correct rather than a session to
 * re-establish (M1-006). The most common cause is a code that rolled over
 * between reading and typing, so that is what the wording addresses.
 */
function describeFailure(error: unknown): string | undefined {
  if (error === null || error === undefined) {
    return undefined
  }
  if (isApiError(error, 'validation_failed')) {
    return 'That code was not right. Codes change every 30 seconds — try the one showing now, and check your phone’s clock if it keeps failing.'
  }
  if (isApiError(error, 'conflict')) {
    return 'This account already has a confirmed authenticator. Reload the page.'
  }
  if (error instanceof ApiError && error.status >= 500) {
    return 'The server could not answer that. Try again in a moment.'
  }
  return 'That code could not be confirmed.'
}
