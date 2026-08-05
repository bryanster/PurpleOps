import type { ReactNode } from 'react'

/**
 * One bordered block of the settings screens, with a heading and a sentence
 * saying what changing it does.
 *
 * The description is not decoration. Every control on these screens has a
 * consequence somewhere else — a password change signs out your other browsers,
 * removing an authenticator deletes your recovery codes — and a settings screen
 * that lists switches without saying what they do is how people find out
 * afterwards.
 */
export function SettingsSection({
  title,
  description,
  children,
  headingLevel = 'h2',
}: {
  title: string
  description?: ReactNode
  children: ReactNode
  headingLevel?: 'h2' | 'h3'
}): ReactNode {
  const Heading = headingLevel

  return (
    <section className="flex flex-col gap-4 rounded-lg border p-6">
      <header className="flex flex-col gap-1">
        <Heading className="font-semibold">{title}</Heading>
        {description !== undefined && (
          <div className="text-muted-foreground max-w-prose text-sm">{description}</div>
        )}
      </header>
      {children}
    </section>
  )
}
