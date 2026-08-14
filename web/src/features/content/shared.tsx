import { type ReactNode, useId, useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { PageEmpty } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useSignedInUser } from '@/features/auth/current-user'
import { isAdmin } from '@/features/auth/queries'
import { cn } from '@/lib/utils'

import { CONTENT_CUSTOM_PATH, CONTENT_SOURCES_PATH } from './paths'
import type { ContentAttackVersion } from './queries'

/** Sentinel for "do not narrow by this filter". Not a real API value. */
export const ANY = 'any'

/**
 * Empty installation: no ATT&CK version is browsable yet.
 *
 * Copy differs by role (M2-EPIC): members are told to ask an admin; admins get
 * a link to the sources control plane. Neither path exposes enable/sync here —
 * that is M2-014's surface.
 */
export function EmptyLibrary(): ReactNode {
  const user = useSignedInUser()
  const admin = isAdmin(user)

  return (
    <PageEmpty
      title="No ATT&CK content installed"
      description={
        admin
          ? 'Install an ATT&CK release from the sources admin before anyone can browse techniques. Custom procedures and notes do not need ATT&CK — author or import them under Custom content.'
          : 'Ask an admin to install ATT&CK.'
      }
      action={
        admin ? (
          <div className="flex flex-wrap gap-2">
            <Button asChild variant="outline" size="sm">
              <Link to={CONTENT_SOURCES_PATH}>Open sources admin</Link>
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link to={CONTENT_CUSTOM_PATH}>Import v1 testcases</Link>
            </Button>
          </div>
        ) : undefined
      }
    />
  )
}

/**
 * Shared filter chrome: a search field plus optional trailing controls so every
 * tab feels like the same tool rather than five different ones.
 */
export function FilterChrome({
  search,
  onSearchChange,
  searchPlaceholder = 'Search by id or name',
  children,
}: {
  search: string
  onSearchChange: (value: string) => void
  searchPlaceholder?: string
  children?: ReactNode
}): ReactNode {
  const searchId = useId()

  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="flex min-w-56 flex-1 flex-col gap-2 sm:max-w-sm">
        <Label htmlFor={searchId}>Search</Label>
        <Input
          id={searchId}
          type="search"
          placeholder={searchPlaceholder}
          value={search}
          onChange={(event) => {
            onSearchChange(event.target.value)
          }}
        />
      </div>
      {children}
    </div>
  )
}

export function FilterSelect({
  id,
  label,
  value,
  onValueChange,
  anyLabel,
  options,
  className,
}: {
  id?: string
  label: string
  value: string
  onValueChange: (value: string) => void
  anyLabel: string
  options: readonly { value: string; label: string }[]
  className?: string
}): ReactNode {
  const generatedId = useId()
  const selectId = id ?? generatedId

  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={selectId}>{label}</Label>
      <Select value={value} onValueChange={onValueChange}>
        <SelectTrigger id={selectId} className={cn('w-44', className)}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY}>{anyLabel}</SelectItem>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

/**
 * ATT&CK version pin for the techniques tab.
 *
 * Healthy versions only. Changing the value must clear any open technique
 * detail so the previous version's identity cannot linger on screen.
 */
export function VersionSelect({
  versions,
  value,
  onValueChange,
}: {
  versions: readonly ContentAttackVersion[]
  value: string
  onValueChange: (value: string) => void
}): ReactNode {
  const id = useId()

  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>ATT&amp;CK version</Label>
      <Select value={value} onValueChange={onValueChange}>
        <SelectTrigger id={id} className="w-40">
          <SelectValue placeholder="Select version" />
        </SelectTrigger>
        <SelectContent>
          {versions.map((version) => (
            <SelectItem key={version.version} value={version.version}>
              {version.version}
              <span className="text-muted-foreground ml-2 text-xs">({version.itemCount})</span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

/**
 * Detail surface for library objects. A dialog rather than a route so the list
 * filters stay put while the reader peeks — and so closing returns focus to the
 * row that opened it.
 */
export function DetailDrawer({
  open,
  onOpenChange,
  title,
  description,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  children: ReactNode
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-6 py-4 text-left">
          <DialogTitle className="pr-8 text-base leading-snug">{title}</DialogTitle>
          {description !== undefined && (
            <DialogDescription className="text-left">{description}</DialogDescription>
          )}
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">{children}</div>
      </DialogContent>
    </Dialog>
  )
}

/** Read-only monospaced block with a copy control (detection rules, commands). */
export function CopyBlock({
  label,
  value,
  className,
}: {
  label: string
  value: string
  className?: string
}): ReactNode {
  const [copied, setCopied] = useState(false)

  async function copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      window.setTimeout(() => {
        setCopied(false)
      }, 1500)
    } catch {
      toast.error('Could not copy to the clipboard')
    }
  }

  return (
    <section className={cn('flex flex-col gap-2', className)}>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-medium">{label}</h3>
        <Button type="button" variant="outline" size="xs" onClick={() => void copy()}>
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
      <pre className="bg-muted max-h-80 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
        {value === '' ? '—' : value}
      </pre>
    </section>
  )
}

export function MetaRow({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <div className="grid gap-1 sm:grid-cols-[8rem_1fr] sm:gap-3">
      <dt className="text-muted-foreground text-sm">{label}</dt>
      <dd className="text-sm break-words">{children}</dd>
    </div>
  )
}

export function IdBadges({
  ids,
  empty = 'None',
}: {
  ids: readonly string[]
  empty?: string
}): ReactNode {
  if (ids.length === 0) {
    return <span className="text-muted-foreground">{empty}</span>
  }
  return (
    <ul className="flex flex-wrap gap-1.5">
      {ids.map((id) => (
        <li key={id}>
          <Badge variant="secondary" className="font-mono">
            {id}
          </Badge>
        </li>
      ))}
    </ul>
  )
}

/**
 * Placeholder for "use in scenario". Still disabled: importing library content
 * needs an engagement to import *into*, and the library has none in scope. The
 * flow itself shipped with M3 and lives on the other side of it — the
 * engagement's Workbook tab — so the tooltip names that route rather than a
 * milestone that has already landed.
 */
export function UseInScenarioPlaceholder(): ReactNode {
  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      disabled
      title="Import from an engagement: Workbook → Import CTID / From Template"
    >
      Use in scenario
    </Button>
  )
}
