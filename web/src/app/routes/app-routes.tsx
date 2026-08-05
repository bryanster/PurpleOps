import type { ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router'

import { AppShell } from '@/app/shell/app-shell'
import { AccountPage } from '@/features/account/account-page'
import { ActivityPage } from '@/features/admin/activity-page'
import { UsersPage } from '@/features/admin/users-page'
import { LibraryPage } from '@/features/content/library-page'
import { CONTENT_PATH } from '@/features/content/paths'
import { EnrolmentPage } from '@/features/auth/enrolment-page'
import { RequireAdmin, RequireAuth } from '@/features/auth/guards'
import { LoginPage } from '@/features/auth/login-page'
import { MfaChallengePage } from '@/features/auth/mfa-challenge-page'
import {
  ACCOUNT_PATH,
  ADMIN_ACTIVITY_PATH,
  ADMIN_USERS_PATH,
  ENROLMENT_PATH,
  LOGIN_PATH,
  MFA_CHALLENGE_PATH,
  TOKENS_PATH,
} from '@/features/auth/paths'
import { HealthPage } from '@/features/system/health-page'
import { VersionPage } from '@/features/system/version-page'
import { TokensPage } from '@/features/tokens/tokens-page'

import { NotFoundPage } from './not-found'

/**
 * Every route, in three tiers (M1-017).
 *
 * 1. **Public**, and outside the shell: the two sign-in screens. There is no
 *    session to draw a nav from, and a half-usable interface around a sign-in
 *    form is a phishing lesson nobody needs.
 * 2. **Signed in**, behind [RequireAuth]. Forced enrolment sits here rather
 *    than in the public tier — it *has* a session, one confined to enrolling —
 *    and deliberately outside the shell, because there is nothing else it may
 *    reach.
 * 3. **Signed in, inside the shell**, and within that, the administrator-only
 *    routes behind [RequireAdmin].
 *
 * The nesting is the enforcement, not a convenience: there is no route below
 * the guard that renders without it having run, so a screen cannot be reached
 * by adding it in the wrong place. That is what makes "the enrolment screen
 * cannot be escaped by editing the URL" a property of this table.
 *
 * The root still redirects to /system/version. M2–M6 give the product a real
 * landing screen and this becomes a redirect to that instead.
 */
export function AppRoutes(): ReactNode {
  return (
    <Routes>
      <Route path={LOGIN_PATH} element={<LoginPage />} />
      <Route path={MFA_CHALLENGE_PATH} element={<MfaChallengePage />} />

      <Route element={<RequireAuth />}>
        <Route path={ENROLMENT_PATH} element={<EnrolmentPage />} />

        <Route element={<AppShell />}>
          <Route index element={<Navigate to="/system/version" replace />} />

          <Route path={ACCOUNT_PATH} element={<AccountPage />} />
          <Route path={TOKENS_PATH} element={<TokensPage />} />
          <Route path={CONTENT_PATH} element={<LibraryPage />} />

          <Route element={<RequireAdmin />}>
            <Route path={ADMIN_USERS_PATH} element={<UsersPage />} />
            <Route path={ADMIN_ACTIVITY_PATH} element={<ActivityPage />} />
          </Route>

          <Route path="system/version" element={<VersionPage />} />
          <Route path="system/health" element={<HealthPage />} />

          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
