import { type SyntheticEvent, type ReactNode, useId, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import { ApiError, isApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { AuthLayout } from './auth-layout'
import { FormAlert } from './form-alert'
import { CODE_LENGTH, OtpInput } from './otp-input'
import { LOGIN_PATH } from './paths'
import { useVerifyRecoveryCode, useVerifyTotp } from './queries'
import { DEFAULT_LANDING, safeReturnTo } from './return-to'

/**
 * The second half of a sign-in (M1-006, M1-007).
 *
 * There is no session here — the credential is a short-lived cookie the login
 * response set, which authorizes these two endpoints and nothing else. That is
 * why this screen has no shell around it and why every failure it can show ends
 * in "start again": there is nothing to be half signed in to.
 *
 * The recovery-code path is a peer of the code path, not a hidden fallback. The
 * person using it has lost their phone, which is a bad enough day without
 * hunting for the link.
 */
export function MfaChallengePage(): ReactNode {
  const navigate = useNavigate()
  const location = useLocation()
  const returnTo = safeReturnTo(readReturnTo(location.state))

  const [usingRecoveryCode, setUsingRecoveryCode] = useState(false)
  const [code, setCode] = useState('')
  const [recoveryCode, setRecoveryCode] = useState('')

  const verifyTotp = useVerifyTotp()
  const verifyRecovery = useVerifyRecoveryCode()

  const codeId = useId()
  const recoveryId = useId()
  const errorId = useId()

  const active = usingRecoveryCode ? verifyRecovery : verifyTotp
  const failure = describeFailure(active.error, usingRecoveryCode)

  function submitTotp(value: string): void {
    if (verifyTotp.isPending || value.length !== CODE_LENGTH) {
      return
    }
    verifyTotp.mutate(
      { code: value },
      {
        onSuccess: () => {
          void navigate(returnTo, { replace: true })
        },
        onError: () => {
          // Cleared so the next attempt starts from an empty field: a code that
          // was refused is a code that has usually rolled over, and making
          // somebody select-all before retyping is a small cruelty.
          setCode('')
        },
      },
    )
  }

  function submitRecovery(event: SyntheticEvent): void {
    event.preventDefault()
    verifyRecovery.mutate(
      { code: recoveryCode.trim() },
      {
        onSuccess: () => {
          void navigate(returnTo, { replace: true })
        },
      },
    )
  }

  return (
    <AuthLayout
      title="One more step"
      description={
        usingRecoveryCode
          ? 'Enter one of the recovery codes you saved when you set up your authenticator. Each one works once.'
          : 'Enter the six-digit code from your authenticator app. It changes every 30 seconds.'
      }
      footer={
        <Link to={LOGIN_PATH} className="text-muted-foreground hover:text-foreground underline">
          Start again
        </Link>
      }
    >
      <div className="flex flex-col gap-6">
        {failure !== undefined && <FormAlert message={failure} error={active.error} />}

        {usingRecoveryCode ? (
          <form className="flex flex-col gap-4" onSubmit={submitRecovery} noValidate>
            <div className="flex flex-col gap-2">
              <Label htmlFor={recoveryId}>Recovery code</Label>
              <Input
                id={recoveryId}
                name="recoveryCode"
                autoFocus
                autoComplete="off"
                spellCheck={false}
                placeholder="3K9M-2PTV-XA47-QRJH-58WY"
                className="font-mono"
                value={recoveryCode}
                onChange={(event) => {
                  setRecoveryCode(event.target.value)
                }}
                aria-describedby={failure === undefined ? undefined : errorId}
              />
              <p className="text-muted-foreground text-xs">
                Upper or lower case, with or without the hyphens — the server folds the characters
                that look alike.
              </p>
            </div>

            <Button type="submit" disabled={verifyRecovery.isPending || recoveryCode.trim() === ''}>
              {verifyRecovery.isPending ? 'Checking…' : 'Use recovery code'}
            </Button>
          </form>
        ) : (
          <form
            className="flex flex-col gap-4"
            onSubmit={(event) => {
              event.preventDefault()
              submitTotp(code)
            }}
            noValidate
          >
            <div className="flex flex-col gap-2">
              <Label htmlFor={codeId}>Authenticator code</Label>
              <OtpInput
                id={codeId}
                value={code}
                onChange={setCode}
                onComplete={submitTotp}
                disabled={verifyTotp.isPending}
                describedBy={failure === undefined ? undefined : errorId}
              />
            </div>

            <Button type="submit" disabled={verifyTotp.isPending || code.length !== CODE_LENGTH}>
              {verifyTotp.isPending ? 'Checking…' : 'Verify'}
            </Button>
          </form>
        )}

        <span id={errorId} className="sr-only">
          {failure}
        </span>

        <Button
          type="button"
          variant="link"
          className="self-start px-0"
          onClick={() => {
            setUsingRecoveryCode((using) => !using)
          }}
        >
          {usingRecoveryCode ? 'Use my authenticator instead' : 'Use a recovery code instead'}
        </Button>
      </div>
    </AuthLayout>
  )
}

/**
 * What to tell somebody whose second factor was refused.
 *
 * The 401 case covers five different server-side reasons — wrong code, reused
 * code, expired pending state, no pending state, a code that was never theirs —
 * and the server answers all five identically on purpose. The wording says the
 * two things a person can act on: try the current code, or start again if this
 * has been sitting open.
 */
function describeFailure(error: unknown, usingRecoveryCode: boolean): string | undefined {
  if (error === null || error === undefined) {
    return undefined
  }
  if (isApiError(error, 'rate_limited')) {
    return 'Too many attempts. Wait a few minutes and start the sign-in again.'
  }
  if (isApiError(error, 'unauthenticated')) {
    return usingRecoveryCode
      ? 'That recovery code was not accepted. Each code works once, and the sign-in expires after a few minutes — start again if this screen has been open a while.'
      : 'That code was not accepted. Codes expire after 30 seconds, so try the one showing now — and start again if this screen has been open a while.'
  }
  if (isApiError(error, 'validation_failed')) {
    return usingRecoveryCode ? 'That does not look like a recovery code.' : 'A code is six digits.'
  }
  if (error instanceof ApiError && error.status >= 500) {
    return 'The server could not answer that. Try again in a moment.'
  }
  return 'That could not be checked.'
}

/**
 * The destination the login screen handed over in router state.
 *
 * Router state is arbitrary — anybody can push a history entry — so it is read
 * defensively and handed to `safeReturnTo` like every other source. That it
 * came from our own navigation is not something this screen can verify.
 */
function readReturnTo(state: unknown): string {
  if (typeof state === 'object' && state !== null && 'returnTo' in state) {
    const { returnTo } = state
    if (typeof returnTo === 'string') {
      return returnTo
    }
  }
  return DEFAULT_LANDING
}
