import type { ReactNode } from 'react'

/**
 * Where a 401 sends the user (`api/query-provider.tsx`).
 *
 * A placeholder, not a screen: there are no sessions until M1-003 and no login
 * form until M1-017, so nothing can reach this yet. It exists so the global 401
 * handler navigates somewhere honest rather than into the 404 page, and it is
 * replaced wholesale by the real one.
 */
export function LoginPlaceholderPage(): ReactNode {
  return (
    <section className="flex max-w-prose flex-col gap-3">
      <h1 className="text-xl font-semibold">Sign in</h1>
      <p className="text-muted-foreground">
        Authentication is not built yet. Until then this deployment has no accounts and no session
        to sign in to.
      </p>
    </section>
  )
}
