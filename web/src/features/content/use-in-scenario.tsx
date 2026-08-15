import { type ReactNode, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  ENGAGEMENTS_PATH,
  engagementWorkbookUsePath,
  type WorkbookUseKind,
} from '@/features/engagements/paths'
import { useEngagements } from '@/features/engagements/queries'

/**
 * Copy per content kind. The library says what will happen on the other side so
 * "use in scenario" does not read as the same action for a whole plan and for a
 * single technique.
 */
const COPY: Record<WorkbookUseKind, { title: string; description: string }> = {
  plan: {
    title: 'Use plan in an engagement',
    description:
      'The plan is imported as a new scenario, one step per plan step, each with a pending execution.',
  },
  procedure: {
    title: 'Use procedure in an engagement',
    description: 'The procedure template is added as a new step in a scenario you pick.',
  },
  technique: {
    title: 'Use technique in an engagement',
    description: 'A new step is created with this technique id filled in.',
  },
}

/** Engagements that still accept new workbook content. */
const OPEN_STATUSES = new Set(['draft', 'active'])

/**
 * "Use in scenario" for a library object.
 *
 * The library has no engagement in scope, so the button asks for one and then
 * hands off to the engagement's Workbook, which owns every import path already
 * (`ImportPlan`, `CreateStepFromTemplate`, `CreateStep`). Nothing is imported
 * from here — the workbook dialog opens pre-filled and the operator confirms
 * there, where the engagement's ATT&CK pin and workbook role actually apply.
 */
export function UseInScenarioButton({
  kind,
  id,
  disabled = false,
}: {
  kind: WorkbookUseKind
  /** Plan/template UUID, or the technique's ATT&CK external id. */
  id: string
  disabled?: boolean
}): ReactNode {
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        disabled={disabled || id === ''}
        onClick={() => {
          setOpen(true)
        }}
      >
        Use in scenario
      </Button>
      <UseInScenarioDialog kind={kind} id={id} open={open} onOpenChange={setOpen} />
    </>
  )
}

function UseInScenarioDialog({
  kind,
  id,
  open,
  onOpenChange,
}: {
  kind: WorkbookUseKind
  id: string
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const navigate = useNavigate()
  const engagements = useEngagements()
  const [engagementId, setEngagementId] = useState('')

  // Closed and archived engagements refuse new scenarios and steps server-side
  // (`ImportPlan` 409s), so they are not offered here.
  const options = useMemo(() => {
    const pages = engagements.data?.pages ?? []
    return pages.flatMap((page) => page.items).filter((item) => OPEN_STATUSES.has(item.status))
  }, [engagements.data])

  const copy = COPY[kind]

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{copy.title}</DialogTitle>
          <DialogDescription>{copy.description}</DialogDescription>
        </DialogHeader>

        {engagements.isPending ? (
          <p className="text-muted-foreground text-sm">Loading engagements…</p>
        ) : engagements.error ? (
          <p className="text-destructive text-sm">Could not load engagements.</p>
        ) : options.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            No open engagements. Create one from{' '}
            <Link to={ENGAGEMENTS_PATH} className="underline underline-offset-4">
              Engagements
            </Link>
            , then come back.
          </p>
        ) : (
          <div className="space-y-2">
            <Label>Engagement</Label>
            <Select value={engagementId} onValueChange={setEngagementId}>
              <SelectTrigger>
                <SelectValue placeholder="Select an engagement..." />
              </SelectTrigger>
              <SelectContent>
                {options.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name}
                    {item.client !== '' && (
                      <span className="text-muted-foreground ml-2 text-xs">{item.client}</span>
                    )}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              onOpenChange(false)
            }}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={engagementId === ''}
            onClick={() => {
              onOpenChange(false)
              void navigate(engagementWorkbookUsePath(engagementId, kind, id))
            }}
          >
            Continue
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
