import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { TOKENS_PATH } from '@/features/auth/paths'

import { MfaPanel } from './mfa-panel'
import { PasswordPanel } from './password-panel'
import { ProfilePanel } from './profile-panel'
import { SessionsPanel } from './sessions-panel'

/**
 * Everything somebody can do about their own account (M1-017).
 *
 * Four panels, in the order somebody deals with them: who you are, the password
 * you sign in with, the second factor on top of it, and the browsers that are
 * signed in right now. Service tokens are a screen of their own — they are
 * credentials for programs rather than for people, and mixing them in here
 * invites revoking the wrong thing.
 */
export function AccountPage(): ReactNode {
  return (
    <div className="flex max-w-4xl flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">Your account</h1>
        <p className="text-muted-foreground max-w-prose text-sm">
          Changes here take effect immediately, including on your other sessions.
        </p>
      </header>

      <ProfilePanel />
      <PasswordPanel />
      <MfaPanel />
      <SessionsPanel />

      <p className="text-muted-foreground text-sm">
        Looking for API credentials?{' '}
        <Button asChild variant="link" className="h-auto p-0">
          <Link to={TOKENS_PATH}>Service tokens</Link>
        </Button>{' '}
        are managed separately.
      </p>
    </div>
  )
}
