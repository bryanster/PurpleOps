import type { ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router'

import { AppShell } from '@/app/shell/app-shell'
import { AccountPage } from '@/features/account/account-page'
import { ActivityPage } from '@/features/admin/activity-page'
import { UsersPage } from '@/features/admin/users-page'
import { CustomContentPage } from '@/features/content/custom-page'
import { LibraryPage } from '@/features/content/library-page'
import { CONTENT_CUSTOM_PATH, CONTENT_PATH, CONTENT_SOURCES_PATH } from '@/features/content/paths'
import { SourcesPage } from '@/features/content/sources-page'
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
import { EngagementLayout } from '@/features/engagements/engagement-layout'
import { EngagementsPage } from '@/features/engagements/engagements-page'
import { FindingsPage } from '@/features/engagements/findings-page'
import { OverviewPage } from '@/features/engagements/overview-page'
import { SettingsPage } from '@/features/engagements/settings-page'
import { WorkbookPage } from '@/features/engagements/workbook-page'
import {
  ENGAGEMENTS_PATH,
  engagementFindingsPath,
  engagementPath,
  engagementSettingsPath,
  engagementWorkbookPath,
} from '@/features/engagements/paths'
import { HealthPage } from '@/features/system/health-page'
import { VersionPage } from '@/features/system/version-page'
import { TokensPage } from '@/features/tokens/tokens-page'

import { NotFoundPage } from './not-found'

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

          <Route path={ENGAGEMENTS_PATH} element={<EngagementsPage />} />

          <Route
            path={`${ENGAGEMENTS_PATH}/:engagementId`}
            element={<EngagementLayout />}
          >
            <Route index element={<OverviewPage />} />
            <Route path="workbook" element={<WorkbookPage />} />
            <Route path="findings" element={<FindingsPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>

          <Route element={<RequireAdmin />}>
            <Route path={ADMIN_USERS_PATH} element={<UsersPage />} />
            <Route path={ADMIN_ACTIVITY_PATH} element={<ActivityPage />} />
            <Route path={CONTENT_SOURCES_PATH} element={<SourcesPage />} />
            <Route path={CONTENT_CUSTOM_PATH} element={<CustomContentPage />} />
          </Route>

          <Route path="system/version" element={<VersionPage />} />
          <Route path="system/health" element={<HealthPage />} />

          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
