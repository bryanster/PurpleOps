import type { ReactNode } from 'react'

import { ThemeToggle } from '@/app/theme/theme-toggle'

/**
 * The frame the sign-in screens render in: centred, narrow, and deliberately
 * without the application shell.
 *
 * No nav and no top bar is the point rather than an aesthetic. Somebody on
 * these screens either has no session or has one confined to enrolling
 * (M1-008), so every link the shell would draw goes somewhere they cannot
 * reach — and the forced-enrolment screen in particular must not be surrounded
 * by an interface that looks half-usable.
 *
 * The theme toggle survives, because it is the one control here that works
 * without a session and somebody reading a QR code on a dark screen at 2am has
 * a real use for it.
 */
export function AuthLayout({
  title,
  description,
  children,
  footer,
}: {
  title: string
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
}): ReactNode {
  return (
    <div className="bg-background text-foreground flex min-h-dvh flex-col">
      <div className="flex justify-end p-4">
        <ThemeToggle />
      </div>

      <main className="flex flex-1 items-start justify-center px-4 pb-16">
        <section className="w-full max-w-md">
          <header className="mb-6 flex flex-col gap-2">
            <p className="text-muted-foreground text-sm font-medium tracking-tight">Blacklight</p>
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            {description !== undefined && (
              <div className="text-muted-foreground text-sm">{description}</div>
            )}
          </header>

          <div className="bg-card rounded-lg border p-6 shadow-sm">{children}</div>

          {footer !== undefined && <div className="mt-4 text-sm">{footer}</div>}
        </section>
      </main>
    </div>
  )
}
