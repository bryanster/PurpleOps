import { type ReactNode, useState } from 'react'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { DetectionsPanel } from './detections-panel'
import { NotesPanel } from './notes-panel'
import { PlansPanel } from './plans-panel'
import { ProceduresPanel } from './procedures-panel'
import { EmptyLibrary } from './shared'
import { TechniquesPanel } from './techniques-panel'
import { isBrowsableAttackVersion, useAttackVersions } from './queries'

type LibraryTab = 'techniques' | 'procedures' | 'detections' | 'plans' | 'notes'

/**
 * Content library browser (M2-013).
 *
 * Member-accessible read surface for every installed content family. The empty
 * ATT&CK state is decided once at the page level so each tab does not invent
 * its own install CTA — and so enable/sync controls never appear here (those
 * are M2-014, admin-only).
 */
export function LibraryPage(): ReactNode {
  const versions = useAttackVersions()
  const [tab, setTab] = useState<LibraryTab>('techniques')

  if (versions.isPending) {
    return (
      <LibraryLayout>
        <PageLoading label="Reading the content library…" />
      </LibraryLayout>
    )
  }

  if (versions.error) {
    return (
      <LibraryLayout>
        <PageError
          error={versions.error}
          onRetry={() => {
            void versions.refetch()
          }}
        />
      </LibraryLayout>
    )
  }

  const hasAttack = versions.data.items.some(isBrowsableAttackVersion)

  return (
    <LibraryLayout>
      {!hasAttack && <EmptyLibrary />}

      <Tabs
        value={tab}
        onValueChange={(value) => {
          setTab(value as LibraryTab)
        }}
        className="gap-4"
      >
        <TabsList variant="line" className="w-full max-w-full flex-wrap justify-start">
          <TabsTrigger value="techniques">Techniques</TabsTrigger>
          <TabsTrigger value="procedures">Procedures</TabsTrigger>
          <TabsTrigger value="detections">Detection rules</TabsTrigger>
          <TabsTrigger value="plans">Emulation plans</TabsTrigger>
          <TabsTrigger value="notes">Notes</TabsTrigger>
        </TabsList>

        <TabsContent value="techniques" className="outline-none">
          {hasAttack ? <TechniquesPanel /> : null}
        </TabsContent>
        <TabsContent value="procedures" className="outline-none">
          <ProceduresPanel />
        </TabsContent>
        <TabsContent value="detections" className="outline-none">
          <DetectionsPanel />
        </TabsContent>
        <TabsContent value="plans" className="outline-none">
          <PlansPanel />
        </TabsContent>
        <TabsContent value="notes" className="outline-none">
          <NotesPanel />
        </TabsContent>
      </Tabs>
    </LibraryLayout>
  )
}

function LibraryLayout({ children }: { children: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">Content</h1>
        <p className="text-muted-foreground max-w-prose text-sm">
          Browse installed ATT&amp;CK techniques, Atomic procedures, detection rule references,
          emulation plans, and knowledge-base notes. Disabled sources do not appear here.
        </p>
      </header>
      {children}
    </div>
  )
}
