import type { ReactNode } from 'react'

import { ThemeToggle } from '@/app/theme/theme-toggle'

/**
 * The frame the first-run wizard renders in: centred, shell-less, and wider
 * than the sign-in screens because it holds a list to choose from rather than
 * a form with two fields.
 *
 * No navigation, for the same reason the enrolment screen has none: while setup
 * is unfinished an administrator is redirected back here from every in-app
 * path, so a nav bar would be a set of links that bounce. The way out is a
 * button on the screen — install, or skip — and both of them finish setup.
 */
export function SetupLayout({
  title,
  description,
  children,
}: {
  title: string
  description?: ReactNode
  children: ReactNode
}): ReactNode {
  return (
    <div className="bg-background text-foreground flex min-h-dvh flex-col">
      <div className="flex justify-end p-4">
        <ThemeToggle />
      </div>

      <main className="flex flex-1 items-start justify-center px-4 pb-16">
        <section className="w-full max-w-2xl">
          <header className="mb-6 flex flex-col gap-2">
            <p className="text-muted-foreground text-sm font-medium tracking-tight">
              Blacklight · first-run setup
            </p>
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            {description !== undefined && (
              <div className="text-muted-foreground max-w-prose text-sm">{description}</div>
            )}
          </header>

          <div className="bg-card rounded-lg border p-6 shadow-sm">{children}</div>
        </section>
      </main>
    </div>
  )
}
