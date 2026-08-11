import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { useCurrentUser } from '@/features/auth/queries'
import { LOGIN_PATH } from '@/features/auth/paths'
import { viewPath } from '@/features/reports/paths'
import { LinkIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Navigate, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'

import { useShareInfo, useClaimShare } from '../queries'

/**
 * Guest claim page — handles the claim flow for shared report invites (M6-014).
 *
 * Flow:
 *   1. Check share info (exists, password required, already claimed).
 *   2. If logged out → show login/guest-register gate.
 *   3. If password required → prompt for password.
 *   4. Claim → redirect to HTML view.
 */
export function ClaimPage(): ReactNode {
  const { token } = useParams<{ token: string }>()
  const navigate = useNavigate()
  const currentUser = useCurrentUser()
  const info = useShareInfo(token)
  const claim = useClaimShare()

  const [password, setPassword] = useState('')

  if (!token) return <Navigate to={LOGIN_PATH} replace />

  // Wait for user check before evaluating share info
  if (currentUser.isPending) return <PageLoading label="Loading…" />

  if (info.isPending) return <PageLoading label="Checking share…" />
  if (info.error) {
    return (
      <PageError
        error={info.error}
        onRetry={() => {
          void info.refetch()
        }}
      />
    )
  }

  const data = info.data
  if (!data.exists) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="mx-auto max-w-sm text-center">
          <h1 className="text-lg font-semibold">Report not found</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            This share link is invalid or has been revoked.
          </p>
        </div>
      </div>
    )
  }

  const user = currentUser.data

  // Not signed in — show login gate
  if (!user) {
    return <GuestGate token={token} shareLabel={data.label} />
  }

  // Already claimed — redirect to view
  if (data.alreadyClaimed) {
    return <Navigate to={viewPath(token)} replace />
  }

  // Password required — prompt
  if (data.passwordRequired) {
    return (
      <PasswordGate
        shareLabel={data.label}
        password={password}
        onPasswordChange={setPassword}
        onClaim={() => {
          if (password) {
            claim.mutate(
              { token, body: { password } },
              {
                onSuccess: () => {
                  void navigate(viewPath(token), { replace: true })
                },
                onError: () => {
                  toast.error(
                    'Failed to claim access. The share may be expired or the password incorrect.',
                  )
                },
              },
            )
          }
        }}
        isPending={claim.isPending}
      />
    )
  }

  // Direct claim — no password needed
  return (
    <ClaimGate
      shareLabel={data.label}
      onClaim={() => {
        claim.mutate(
          { token },
          {
            onSuccess: () => {
              void navigate(viewPath(token), { replace: true })
            },
            onError: () => {
              toast.error(
                'Failed to claim access. The share may be expired or the password incorrect.',
              )
            },
          },
        )
      }}
      isPending={claim.isPending}
    />
  )
}

function GuestGate({ token, shareLabel }: { token: string; shareLabel?: string }): ReactNode {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="mx-auto w-full max-w-sm space-y-6 text-center">
        <div className="space-y-1">
          <LinkIcon className="text-muted-foreground mx-auto size-8" />
          <h1 className="text-lg font-semibold">
            {shareLabel ? `Shared report: ${shareLabel}` : 'Shared report'}
          </h1>
          <p className="text-muted-foreground text-sm">
            Sign in or create a guest account to view this report.
          </p>
        </div>
        <div className="flex flex-col gap-2">
          <Button asChild>
            <a href={`${LOGIN_PATH}?redirect=${encodeURIComponent(`/claim/${token}`)}`}>Sign in</a>
          </Button>
          <p className="text-muted-foreground text-xs">
            Send this link out of band. No email integration in this release.
          </p>
        </div>
      </div>
    </div>
  )
}

function PasswordGate({
  shareLabel: _shareLabel,
  password,
  onPasswordChange,
  onClaim,
  isPending,
}: {
  shareLabel?: string
  password: string
  onPasswordChange: (v: string) => void
  onClaim: () => void
  isPending: boolean
}): ReactNode {
  const canSubmit = password.length > 0

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="mx-auto w-full max-w-sm space-y-6">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold">Password required</h1>
          <p className="text-muted-foreground text-sm">
            This share is password-protected. Enter the password to continue.
          </p>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="claim-password">Share password</Label>
          <Input
            id="claim-password"
            type="password"
            placeholder="Enter password"
            value={password}
            onChange={(e) => {
              onPasswordChange(e.target.value)
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && canSubmit) onClaim()
            }}
          />
        </div>
        <Button className="w-full" onClick={onClaim} disabled={!canSubmit || isPending}>
          {isPending ? 'Verifying…' : 'Continue'}
        </Button>
      </div>
    </div>
  )
}

function ClaimGate({
  shareLabel: _shareLabel,
  onClaim,
  isPending,
}: {
  shareLabel?: string
  onClaim: () => void
  isPending: boolean
}): ReactNode {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="mx-auto w-full max-w-sm space-y-6 text-center">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold">Claim access</h1>
          <p className="text-muted-foreground text-sm">
            You have been invited to view this report. Claim your access to continue.
          </p>
        </div>
        <Button className="w-full" onClick={onClaim} disabled={isPending}>
          {isPending ? 'Claiming…' : 'Claim access'}
        </Button>
      </div>
    </div>
  )
}
