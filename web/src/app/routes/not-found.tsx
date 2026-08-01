import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'

export function NotFoundPage(): ReactNode {
  return (
    <section className="flex max-w-prose flex-col items-start gap-4">
      <h1 className="text-xl font-semibold">Nothing here</h1>
      <p className="text-muted-foreground">
        That address does not match a screen in this application. If you followed a link from
        somewhere inside it, that is a bug worth reporting.
      </p>
      <Button asChild variant="outline">
        <Link to="/">Back to the start</Link>
      </Button>
    </section>
  )
}
