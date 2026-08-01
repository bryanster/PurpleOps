import type { ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router'

import { AppShell } from '@/app/shell/app-shell'
import { HealthPage } from '@/features/system/health-page'
import { VersionPage } from '@/features/system/version-page'

import { LoginPlaceholderPage } from './login-placeholder'
import { NotFoundPage } from './not-found'

/**
 * Every route renders inside the shell, including the 404 — a wrong address
 * should still leave the user somewhere they can navigate out of.
 *
 * The root redirects to /system/version for now. M2–M6 give the product a real
 * landing screen and this becomes a redirect to that instead.
 */
export function AppRoutes(): ReactNode {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/system/version" replace />} />
        <Route path="login" element={<LoginPlaceholderPage />} />
        <Route path="system/version" element={<VersionPage />} />
        <Route path="system/health" element={<HealthPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  )
}
