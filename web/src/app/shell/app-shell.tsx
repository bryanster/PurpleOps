import type { ReactNode } from 'react'
import { Outlet } from 'react-router'

import { ErrorBoundary } from '@/app/error/error-boundary'
import { Toaster } from '@/components/ui/sonner'

import { SideNav } from './side-nav'
import { TopBar } from './top-bar'

/**
 * The layout every screen renders inside: top bar, left nav, content outlet and
 * the toast host.
 *
 * The error boundary sits around the outlet rather than around the whole shell,
 * so a screen that throws leaves the user with working navigation to get
 * somewhere else. A second boundary in main.tsx catches the shell itself.
 *
 * Sized for 1280px and degrades to 768px; below that it is not a target — this
 * is a tool people use on a laptop beside a terminal.
 */
export function AppShell(): ReactNode {
  return (
    <div className="bg-background text-foreground flex h-dvh flex-col">
      <TopBar />
      <div className="flex min-h-0 flex-1">
        <SideNav />
        <main className="min-w-0 flex-1 overflow-y-auto p-6">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
      <Toaster />
    </div>
  )
}
