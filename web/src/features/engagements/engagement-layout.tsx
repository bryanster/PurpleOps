import { createContext, type ReactNode, use } from 'react'
import { Navigate, NavLink, Outlet, useParams } from 'react-router'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { useSignedInUser } from '@/features/auth/current-user'
import { cn } from '@/lib/utils'

import {
  engagementFindingsPath,
  engagementPath,
  engagementSettingsPath,
  engagementWorkbookPath,
} from './paths'
import {
  isEngagementClosed,
  isPlatformAdmin,
  roleInEngagement,
  useEngagement,
  type EngagementRole,
} from './queries'
import { useEngagementEvents } from './use-engagement-events'

interface TabDef {
  label: string
  to: string
}

/**
 * Shared chrome for every engagement detail route.
 *
 * Fetches the engagement, resolves the caller's role from `GET /auth/me`, and
 * renders a header with sub-navigation. Each tab is its own route so that deep
 * linking to the workbook or findings works on refresh.
 */
export function EngagementLayout(): ReactNode {
  const { engagementId } = useParams<{ engagementId: string }>()
  const engagement = useEngagement(engagementId)
  const user = useSignedInUser()
  useEngagementEvents(engagementId)

  if (engagement.isPending) {
    return <PageLoading label="Loading engagement…" />
  }

  if (engagement.error) {
    return (
      <PageError
        error={engagement.error}
        onRetry={() => {
          void engagement.refetch()
        }}
      />
    )
  }

  if (!engagement.data) {
    return <Navigate to="/engagements" replace />
  }

  const eng = engagement.data
  const role = isPlatformAdmin(user)
    ? ('admin' as EngagementRole)
    : roleInEngagement(eng.id, user)

  if (role === undefined) {
    return <Navigate to="/engagements" replace />
  }

  const closed = isEngagementClosed(eng.status)

  const tabs: TabDef[] = [
    { label: 'Overview', to: engagementPath(eng.id) },
    { label: 'Workbook', to: engagementWorkbookPath(eng.id) },
    { label: 'Findings', to: engagementFindingsPath(eng.id) },
    { label: 'Settings', to: engagementSettingsPath(eng.id) },
  ]

  return (
    <EngagementContextProvider engagementId={eng.id} role={role} closed={closed}>
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold">{eng.name}</h1>
              <StatusBadge status={eng.status} />
              <Badge variant="outline">
                {eng.mode === 'blind' ? 'Blind' : 'Standard'}
              </Badge>
            </div>
            <p className="text-muted-foreground text-sm">
              {eng.client && <span>{eng.client} · </span>}
              ATT&amp;CK {eng.attackVersion}
            </p>
          </div>
          <Badge variant="secondary">{roleLabel(role)}</Badge>
        </div>

        <nav className="border-b" aria-label="Engagement sections">
          <div className="flex gap-1 -mb-px overflow-x-auto">
            {tabs.map((tab) => (
              <NavLink
                key={tab.to}
                to={tab.to}
                end={tab.to === engagementPath(eng.id)}
                className={({ isActive }) =>
                  cn(
                    'border-b-2 px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap',
                    'focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
                    isActive
                      ? 'border-primary text-foreground'
                      : 'border-transparent text-muted-foreground hover:text-foreground hover:border-muted-foreground/30',
                  )
                }
              >
                {tab.label}
              </NavLink>
            ))}
          </div>
        </nav>

        <Outlet />
      </div>
    </EngagementContextProvider>
  )
}

// ── Engagement Context ────────────────────────────────────────────────────────

export interface EngagementContextValue {
  engagementId: string
  role: EngagementRole
  closed: boolean
}

export const EngagementCtx = createContext<EngagementContextValue | undefined>(
  undefined,
)

function EngagementContextProvider({
  engagementId,
  role,
  closed,
  children,
}: EngagementContextValue & { children: ReactNode }): ReactNode {
  return (
    <EngagementCtx value={{ engagementId, role, closed }}>
      {children}
    </EngagementCtx>
  )
}

/** The engagement identity + caller role for any child component. */
export function useEngagementContext(): EngagementContextValue {
  const ctx = use(EngagementCtx)
  if (ctx === undefined) {
    throw new Error(
      'useEngagementContext must be used inside EngagementLayout',
    )
  }
  return ctx
}

// ── Shared helpers ────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }): ReactNode {
  const variant =
    status === 'active'
      ? 'default'
      : status === 'draft'
        ? 'secondary'
        : status === 'closed'
          ? 'outline'
          : 'secondary'
  return (
    <Badge variant={variant as never}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </Badge>
  )
}

function roleLabel(role: EngagementRole): string {
  switch (role) {
    case 'lead':
      return 'Lead'
    case 'red':
      return 'Red'
    case 'blue':
      return 'Blue'
    case 'observer':
      return 'Observer'
    case 'admin':
      return 'Admin'
  }
}
