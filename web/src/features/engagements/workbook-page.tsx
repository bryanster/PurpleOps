import {
  type SyntheticEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { PlayIcon, SquareIcon } from 'lucide-react'
import { toast } from 'sonner'
import { isApiError } from '@/api/errors'

import { formatMoment } from '@/lib/time'
import { useSignedInUser } from '@/features/auth/current-user'
import { api, API_BASE_URL } from '@/api/client'
import type { components } from '@/api/schema'
import { cn } from '@/lib/utils'
import { useFlashOnChange } from '@/lib/use-flash-on-change'
import { useQueryClient } from '@tanstack/react-query'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
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
import { Separator } from '@/components/ui/separator'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Textarea } from '@/components/ui/textarea'
import { useEngagementContext } from './engagement-layout'
import { markCommentRead, useCommentUnread } from './use-comment-unread'
import { usePlans, useProcedures } from '@/features/content/queries'
import {
  canSeeUnrevealed,
  canWriteBlue,
  canWriteComments,
  canWriteRed,
  canWriteWorkbook,
} from './roles'
import {
  engagementKeys,
  isStepRevealed,
  useAllEngagementSteps,
  useComments,
  useCreateComment,
  useCreateScenario,
  useCreateStep,
  useCreateStepFromTemplate,
  useEngagementExecutions,
  useEngagementMembers,
  useEvidenceList,
  useImportPlan,
  usePatchBlueDetection,
  usePatchComment,
  usePatchRedExecution,
  useRevealStep,
  useScenarios,
  type BlueDetectionPatch,
  type Comment,
  type CreateScenario,
  type CreateStep,
  type CreateStepFromTemplate,
  type EngagementRole,
  type Execution,
  type ImportPlanRequest,
  type RedExecutionPatch,
  type Scenario,
  type Step,
} from './queries'

// ── Labels ────────────────────────────────────────────────────────────────────

const STATUS_LABEL: Record<string, string> = {
  pending: 'Pending',
  running: 'Running',
  complete: 'Complete',
  blocked: 'Blocked',
  skipped: 'Skipped',
}

const STATUS_VARIANT: Record<string, 'secondary' | 'default' | 'outline' | 'destructive'> = {
  pending: 'secondary',
  running: 'default',
  complete: 'default',
  blocked: 'destructive',
  skipped: 'outline',
}

const DETECTION_LABEL: Record<string, string> = {
  none: 'None',
  telemetry: 'Telemetry',
  general: 'General',
  tactic: 'Tactic',
  technique: 'Technique',
}

const DETECTION_VARIANT: Record<string, 'outline' | 'secondary' | 'default' | 'destructive'> = {
  none: 'destructive',
  telemetry: 'secondary',
  general: 'secondary',
  tactic: 'default',
  technique: 'default',
}

const PROTECTION_LABEL: Record<string, string> = {
  blocked: 'Blocked',
  partial: 'Partial',
  not_blocked: 'Not Blocked',
  'n/a': 'N/A',
}

const OUTCOME_LABEL: Record<string, string> = {
  prevented: 'Prevented',
  detected: 'Detected',
  not_detected: 'Not Detected',
  not_applicable: 'N/A',
}

const SEVERITY_LABEL: Record<string, string> = {
  info: 'Info',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
}

// ── Scoring definitions (from docs/scoring.md) for tooltips ────────────────────

const CATEGORY_DEFINITIONS: Record<string, string> = {
  none: 'No detection capability.',
  telemetry: 'Minimal data collected — process creation, network flow.',
  general: 'Broad-spectrum alerting (AV, EDR default rule).',
  tactic: "Detection logic aligned to a tactic (e.g. 'credential access').",
  technique: 'Technique-specific detection (e.g. T1003.001).',
}

const MODIFIER_DEFINITIONS: Record<string, string> = {
  alert: 'Generated an alert.',
  correlated: 'Correlated with other events.',
  delayed: 'Detected after a lag (batch, analyst review).',
  config_change: 'Detected via a configuration change (new rule, tuning).',
  residual_artifact: 'Detected via post-execution artifacts (logs, forensic).',
}

const MODIFIER_LABEL: Record<string, string> = {
  alert: 'Alert',
  correlated: 'Correlated',
  delayed: 'Delayed',
  config_change: 'Config Change',
  residual_artifact: 'Residual Artifact',
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function WorkbookPage(): ReactNode {
  const { engagementId, role, closed } = useEngagementContext()

  const scenarios = useScenarios(engagementId)
  const steps = useAllEngagementSteps(engagementId)
  const executions = useEngagementExecutions(engagementId)

  const [expandedScenarios, setExpandedScenarios] = useState<Set<string>>(new Set())
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null)
  const [addScenarioOpen, setAddScenarioOpen] = useState(false)
  const [addStepOpen, setAddStepOpen] = useState(false)
  const [addStepScenarioId, setAddStepScenarioId] = useState<string>('')
  const [importCtidOpen, setImportCtidOpen] = useState(false)
  const [fromTemplateOpen, setFromTemplateOpen] = useState(false)
  const [fromTemplateScenarioId, setFromTemplateScenarioId] = useState<string>('')

  const stepsByScenario = useMemo(() => {
    const map = new Map<string, Step[]>()
    if (steps.data?.items) {
      for (const s of steps.data.items) {
        const arr = map.get(s.scenarioId)
        if (arr) arr.push(s)
        else map.set(s.scenarioId, [s])
      }
    }
    return map
  }, [steps.data])

  const executionByStepId = useMemo(() => {
    const map = new Map<string, Execution>()
    if (executions.data?.items) {
      for (const e of executions.data.items) {
        map.set(e.stepId, e)
      }
    }
    return map
  }, [executions.data])

  const toggleScenario = useCallback((id: string) => {
    setExpandedScenarios((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])
  const openExecution = useCallback(
    (step: Step) => {
      setSelectedStepId(step.id)
      const exec = executionByStepId.get(step.id)
      if (exec) {
        markCommentRead(engagementId, exec.id)
      }
    },
    [engagementId, executionByStepId],
  )

  const closeExecution = useCallback(() => {
    setSelectedStepId(null)
  }, [])

  // Derive current step and execution from live query data (M4-005).
  // This ensures the open drawer reflects remote updates without reload.
  const selectedStep = useMemo(() => {
    if (!selectedStepId || !steps.data?.items) return null
    return steps.data.items.find((s) => s.id === selectedStepId) ?? null
  }, [selectedStepId, steps.data])

  const selectedExecution = useMemo(() => {
    if (!selectedStepId) return null
    return executionByStepId.get(selectedStepId) ?? null
  }, [selectedStepId, executionByStepId])

  // Loading / error states
  if (scenarios.isLoading || steps.isLoading || executions.isLoading) {
    return <PageLoading label="Loading workbook..." />
  }
  if (scenarios.isError) {
    return (
      <PageError
        error={
          scenarios.error instanceof Error ? scenarios.error : new Error('Failed to load scenarios')
        }
      />
    )
  }
  if (steps.isError) {
    return (
      <PageError
        error={steps.error instanceof Error ? steps.error : new Error('Failed to load steps')}
      />
    )
  }
  if (executions.isError) {
    return (
      <PageError
        error={
          executions.error instanceof Error
            ? executions.error
            : new Error('Failed to load executions')
        }
      />
    )
  }

  const scenarioItems = scenarios.data?.items ?? []

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      {canWriteWorkbook(role) && !closed && (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            onClick={() => {
              setAddScenarioOpen(true)
            }}
          >
            Add Scenario
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              const first = scenarioItems[0]
              if (first) {
                setAddStepScenarioId(first.id)
              }
              setAddStepOpen(true)
            }}
          >
            Add Step
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setImportCtidOpen(true)
            }}
          >
            Import CTID
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              const first = scenarioItems[0]
              if (first) {
                setFromTemplateScenarioId(first.id)
              }
              setFromTemplateOpen(true)
            }}
          >
            From Template
          </Button>
        </div>
      )}

      {/* Scenarios */}
      {scenarioItems.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center text-sm">
          No scenarios yet. Add a scenario to start building the workbook.
        </p>
      ) : (
        <div className="space-y-4">
          {scenarioItems.map((scenario) => (
            <ScenarioSection
              key={scenario.id}
              scenario={scenario}
              steps={stepsByScenario.get(scenario.id) ?? []}
              executionByStepId={executionByStepId}
              expanded={expandedScenarios.has(scenario.id)}
              onToggle={() => {
                toggleScenario(scenario.id)
              }}
              onSelectStep={openExecution}
              role={role}
              engagementId={engagementId}
            />
          ))}
        </div>
      )}

      {/* Execution detail dialog */}
      {selectedStep && (
        <ExecutionDrawer
          step={selectedStep}
          execution={selectedExecution}
          engagementId={engagementId}
          role={role}
          closed={closed}
          onClose={closeExecution}
        />
      )}

      {/* Dialogs */}
      <AddScenarioDialog
        open={addScenarioOpen}
        onOpenChange={setAddScenarioOpen}
        engagementId={engagementId}
      />
      <AddStepDialog
        open={addStepOpen}
        onOpenChange={setAddStepOpen}
        engagementId={engagementId}
        scenarios={scenarioItems}
        defaultScenarioId={addStepScenarioId}
      />
      <ImportCtidDialog
        open={importCtidOpen}
        onOpenChange={setImportCtidOpen}
        engagementId={engagementId}
      />
      <StepFromTemplateDialog
        open={fromTemplateOpen}
        onOpenChange={setFromTemplateOpen}
        engagementId={engagementId}
        scenarios={scenarioItems}
        defaultScenarioId={fromTemplateScenarioId}
      />
    </div>
  )
}

// ── Scenario Section ──────────────────────────────────────────────────────────

function ScenarioSection({
  scenario,
  steps,
  executionByStepId,
  expanded,
  onToggle,
  onSelectStep,
  role,
  engagementId,
}: {
  scenario: Scenario
  steps: Step[]
  executionByStepId: Map<string, Execution>
  expanded: boolean
  onToggle: () => void
  onSelectStep: (step: Step) => void
  role: EngagementRole
  engagementId: string
}) {
  return (
    <div className="rounded-lg border">
      <button
        type="button"
        onClick={onToggle}
        className="hover:bg-muted/50 flex w-full items-center gap-3 rounded-t-lg px-4 py-3 text-left transition-colors"
      >
        <svg
          className={cn(
            'text-muted-foreground h-4 w-4 shrink-0 transition-transform',
            expanded && 'rotate-90',
          )}
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="m9 18 6-6-6-6" />
        </svg>
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-sm font-semibold">
            {scenario.ordinal}. {scenario.name}
          </h3>
          {scenario.threatActor && (
            <p className="text-muted-foreground truncate text-xs">{scenario.threatActor}</p>
          )}
        </div>
        <Badge variant="outline" className="shrink-0 text-xs">
          {steps.length} step{steps.length !== 1 ? 's' : ''}
        </Badge>
      </button>

      {expanded && (
        <div className="border-t">
          {scenario.narrative && (
            <p className="text-muted-foreground bg-muted/20 border-b px-4 py-2 text-xs">
              {scenario.narrative}
            </p>
          )}
          {steps.length === 0 ? (
            <p className="text-muted-foreground px-4 py-4 text-center text-xs">
              No steps in this scenario.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-12">#</TableHead>
                  <TableHead>Step</TableHead>
                  <TableHead className="w-28">Technique</TableHead>
                  <TableHead className="w-28">Status</TableHead>
                  <TableHead className="w-28">Detection</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {steps.map((step) => {
                  const exec = executionByStepId.get(step.id)
                  return (
                    <StepRow
                      key={step.id}
                      step={step}
                      execution={exec}
                      role={role}
                      engagementId={engagementId}
                      onClick={() => {
                        onSelectStep(step)
                      }}
                    />
                  )
                })}
              </TableBody>
            </Table>
          )}
        </div>
      )}
    </div>
  )
}

// ── Step Row ──────────────────────────────────────────────────────────────────

function StepRow({
  step,
  execution,
  role,
  engagementId,
  onClick,
}: {
  step: Step
  execution: Execution | undefined
  role: EngagementRole
  engagementId: string
  onClick: () => void
}) {
  const revealed = isStepRevealed(step)
  const canSee = canSeeUnrevealed(role)
  const visible = revealed || canSee

  const flash = useFlashOnChange(
    execution ? `${String(execution.version)}:${step.updatedAt}` : step.updatedAt,
  )

  return (
    <TableRow
      className={cn('hover:bg-muted/50 cursor-pointer', flash && 'animate-flash-update')}
      onClick={onClick}
    >
      <TableCell className="text-muted-foreground font-mono text-xs">{step.ordinal}</TableCell>
      <TableCell>
        <span className="flex items-center gap-2">
          {visible ? (
            <span className="text-sm font-medium">{step.name}</span>
          ) : (
            <span className="text-muted-foreground text-sm italic">[Unrevealed]</span>
          )}
          {execution && visible && (
            <UnreadCommentBadge engagementId={engagementId} executionId={execution.id} />
          )}
        </span>
      </TableCell>
      <TableCell>
        {step.techniqueId && (
          <Badge variant="outline" className="font-mono text-xs">
            {step.techniqueId}
          </Badge>
        )}
      </TableCell>
      <TableCell>
        {execution ? (
          <Badge variant={STATUS_VARIANT[execution.status] ?? 'secondary'}>
            {STATUS_LABEL[execution.status] ?? execution.status}
          </Badge>
        ) : (
          <Badge variant="secondary">Pending</Badge>
        )}
      </TableCell>
      <TableCell>
        {execution?.detectionCategory ? (
          <Badge variant={DETECTION_VARIANT[execution.detectionCategory] ?? 'outline'}>
            {DETECTION_LABEL[execution.detectionCategory] ?? execution.detectionCategory}
          </Badge>
        ) : (
          <span className="text-muted-foreground text-xs">&mdash;</span>
        )}
      </TableCell>
    </TableRow>
  )
}

// ── Unread Comment Badge ──────────────────────────────────────────────────────

function UnreadCommentBadge({
  engagementId,
  executionId,
}: {
  engagementId: string
  executionId: string
}) {
  const comments = useComments(engagementId, executionId)
  const lastComment =
    comments.data && comments.data.length > 0 ? comments.data[comments.data.length - 1] : undefined
  const newestAt = lastComment?.createdAt ?? null
  const { hasUnread } = useCommentUnread(engagementId, executionId, newestAt)

  if (!hasUnread) return null

  return (
    <span className="bg-primary text-primary-foreground flex h-5 w-5 items-center justify-center rounded-full text-[10px] leading-none font-bold">
      {comments.data ? Math.min(comments.data.length, 99) : '!'}
    </span>
  )
}

// ── Execution Drawer (Dialog) ─────────────────────────────────────────────────

function ExecutionDrawer({
  step,
  execution,
  engagementId,
  role,
  closed,
  onClose,
}: {
  step: Step
  execution: Execution | null
  engagementId: string
  role: EngagementRole
  closed: boolean
  onClose: () => void
}) {
  const revealed = isStepRevealed(step)
  const canSee = canSeeUnrevealed(role)
  const visible = revealed || canSee

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
    >
      <DialogContent className="max-h-[92vh] max-w-7xl overflow-y-auto sm:max-w-7xl">
        <DialogHeader>
          <DialogTitle>{visible ? step.name : '[Unrevealed]'}</DialogTitle>
          <DialogDescription>{step.objective && <span>{step.objective}</span>}</DialogDescription>
        </DialogHeader>

        {/* Step info bar */}
        <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
          {step.techniqueId && (
            <Badge variant="outline" className="font-mono">
              {step.techniqueId}
            </Badge>
          )}
          {step.tacticId && (
            <Badge variant="secondary" className="font-mono">
              {step.tacticId}
            </Badge>
          )}
          {step.targetAsset && <span>Target: {step.targetAsset}</span>}
          {execution && (
            <Badge variant={STATUS_VARIANT[execution.status] ?? 'secondary'}>
              {STATUS_LABEL[execution.status] ?? execution.status}
            </Badge>
          )}
          {/* Outcome (read-only, server-derived) */}
          {execution && (
            <span className="flex items-center gap-1.5">
              <span>Outcome:</span>
              {execution.outcome ? (
                <Badge variant="outline">
                  {OUTCOME_LABEL[execution.outcome] ?? execution.outcome}
                </Badge>
              ) : (
                <span>—</span>
              )}
            </span>
          )}
          {/* MTTD — the diff between the red start and the blue detection.
              It sits in the shared bar because neither team owns it. */}
          {execution && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="flex items-center gap-1.5">
                  <span>MTTD:</span>
                  {execution.mttdSeconds != null ? (
                    <Badge variant="secondary" className="tabular-nums">
                      {formatDuration(execution.mttdSeconds)}
                    </Badge>
                  ) : (
                    <span>
                      {execution.startedAt == null ? 'not started' : 'awaiting detection'}
                    </span>
                  )}
                </span>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="max-w-[260px] text-xs">
                Time to detect: the blue detection time minus the red start time. Both must be set.
              </TooltipContent>
            </Tooltip>
          )}
          {/* Reveal button */}
          {canSee && !revealed && !closed && (
            <RevealButton
              engagementId={engagementId}
              scenarioId={step.scenarioId}
              stepId={step.id}
            />
          )}
        </div>

        {/* Red team on the left, blue team on the right */}
        {execution && (
          <div className="grid gap-4 md:grid-cols-2">
            <RedExecutionEditor
              engagementId={engagementId}
              execution={execution}
              closed={closed}
              readOnly={!canWriteRed(role)}
            />
            <BlueDetectionEditor
              engagementId={engagementId}
              execution={execution}
              closed={closed}
              readOnly={!canWriteBlue(role)}
            />
          </div>
        )}

        <Separator />

        {/* Comments */}
        {canWriteComments(role) && (
          <CommentsSection engagementId={engagementId} executionId={execution?.id} role={role} />
        )}

        {/* Evidence */}
        <EvidenceSection engagementId={engagementId} executionId={execution?.id} role={role} />
      </DialogContent>
    </Dialog>
  )
}

// ── Reveal Button ─────────────────────────────────────────────────────────────

function RevealButton({
  engagementId,
  scenarioId,
  stepId,
}: {
  engagementId: string
  scenarioId: string
  stepId: string
}) {
  const reveal = useRevealStep()
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={reveal.isPending}
      onClick={() => {
        reveal.mutate(
          { engagementId, scenarioId, stepId },
          {
            onSuccess: () => toast.success('Step revealed to blue team'),
            onError: (err) => toast.error(err.message),
          },
        )
      }}
    >
      Reveal to Blue
    </Button>
  )
}

// ── Team panels ───────────────────────────────────────────────────────────────

/** Coloured panel wrapping one team's half of the step view (red left, blue right). */
function TeamPanel({
  team,
  title,
  children,
}: {
  team: 'red' | 'blue'
  title: string
  children: ReactNode
}) {
  const red = team === 'red'
  return (
    <section
      className={cn(
        'flex h-full flex-col overflow-hidden rounded-lg border',
        red
          ? 'border-team-red-border bg-team-red-surface'
          : 'border-team-blue-border bg-team-blue-surface',
      )}
    >
      <h4
        className={cn(
          'px-3 py-2 text-sm font-semibold',
          red ? 'bg-team-red text-team-red-foreground' : 'bg-team-blue text-team-blue-foreground',
        )}
      >
        {title}
      </h4>
      {/* `min-h-0` keeps a notes field that grows from pushing past the panel. */}
      <div className="flex min-h-0 flex-1 flex-col gap-3 p-3">{children}</div>
    </section>
  )
}

function RedExecutionEditor({
  engagementId,
  execution,
  closed,
  readOnly,
}: {
  engagementId: string
  execution: Execution
  closed: boolean
  readOnly: boolean
}) {
  const qc = useQueryClient()
  const patchRed = usePatchRedExecution()

  const [status, setStatus] = useState(execution.status)
  const [commandRun, setCommandRun] = useState(execution.commandRun)
  const [sourceHost, setSourceHost] = useState(execution.sourceHost)
  const [targetHost, setTargetHost] = useState(execution.targetHost)
  const [redNotes, setRedNotes] = useState(execution.redNotes)
  const [startedAt, setStartedAt] = useState(toLocalDatetime(execution.startedAt))
  const [endedAt, setEndedAt] = useState(toLocalDatetime(execution.endedAt))

  // Reset local state when the execution version changes (remote update or
  // 409 recovery refetch).  This ensures the editor always reflects the
  // committed server state (M4-005).
  const [version, setVersion] = useState(execution.version)
  /* eslint-disable react-hooks/set-state-in-effect -- syncing local edit state with external data on version change (M4-005) */
  useEffect(() => {
    if (execution.version !== version) {
      setVersion(execution.version)
      setStatus(execution.status)
      setCommandRun(execution.commandRun)
      setSourceHost(execution.sourceHost)
      setTargetHost(execution.targetHost)
      setRedNotes(execution.redNotes)
      setStartedAt(toLocalDatetime(execution.startedAt))
      setEndedAt(toLocalDatetime(execution.endedAt))
    }
  }, [execution.version]) // eslint-disable-line react-hooks/exhaustive-deps
  /* eslint-enable react-hooks/set-state-in-effect */

  const disabled = closed || readOnly

  /**
   * The PATCH body for everything the editor currently holds, with `overrides`
   * applied last. The timer buttons go through this too: a stopwatch press is
   * also a save, so pressing Start mid-sentence does not discard the notes the
   * operator is typing when the version bump resyncs this editor.
   */
  const redBody = (overrides?: Partial<RedExecutionPatch>): RedExecutionPatch => ({
    version: execution.version,
    status,
    commandRun,
    sourceHost,
    targetHost,
    redNotes,
    startedAt: editedDatetime(execution.startedAt, startedAt),
    endedAt: editedDatetime(execution.endedAt, endedAt),
    ...overrides,
  })

  const submit = (body: RedExecutionPatch, success: string) => {
    patchRed.mutate(
      { engagementId, executionId: execution.id, body },
      {
        onSuccess: () => toast.success(success),
        onError: (err) => {
          if (isApiError(err, 'conflict')) {
            toast.error('This execution was modified by someone else. Reloading current state.')
            void qc.invalidateQueries({
              queryKey: engagementKeys.executions(engagementId),
            })
            void qc.invalidateQueries({
              queryKey: engagementKeys.execution(engagementId, execution.id),
            })
          } else {
            toast.error(err.message)
          }
        },
      },
    )
  }

  const handleSave = () => {
    if (isInverted(startedAt, endedAt)) {
      toast.error('Ended at cannot precede started at.')
      return
    }
    submit(redBody(), 'Red execution saved')
  }

  // The timer writes on the press rather than waiting for Save: the whole point
  // of a stopwatch is that the moment it records is the moment the button went
  // down, not the moment somebody remembered to save the form. MTTD is measured
  // off this timestamp, so a save-later timer would measure the save.
  const handleStart = () => {
    submit(
      redBody({
        startedAt: new Date().toISOString(),
        status: status === 'pending' ? 'running' : status,
      }),
      'Execution started',
    )
  }

  const handleStop = () => {
    submit(
      redBody({
        endedAt: new Date().toISOString(),
        status: status === 'running' ? 'complete' : status,
      }),
      'Execution ended',
    )
  }

  return (
    <TeamPanel team="red" title="Red Execution">
      <div className="space-y-1">
        <Label className="text-xs">Status</Label>
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v as Execution['status'])
          }}
          disabled={disabled}
        >
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(STATUS_LABEL).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <ExecutionTimer
        execution={execution}
        startedAt={startedAt}
        endedAt={endedAt}
        onStartedAtChange={setStartedAt}
        onEndedAtChange={setEndedAt}
        onStart={handleStart}
        onStop={handleStop}
        disabled={disabled}
        pending={patchRed.isPending}
      />

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label className="text-xs">Source Host</Label>
          <Input
            value={sourceHost}
            onChange={(e) => {
              setSourceHost(e.target.value)
            }}
            disabled={disabled}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Target Host</Label>
          <Input
            value={targetHost}
            onChange={(e) => {
              setTargetHost(e.target.value)
            }}
            disabled={disabled}
          />
        </div>
      </div>
      <div className="space-y-1">
        <Label className="text-xs">Command Run</Label>
        <Input
          className="font-mono"
          value={commandRun}
          onChange={(e) => {
            setCommandRun(e.target.value)
          }}
          disabled={disabled}
        />
      </div>
      <div className="flex flex-1 flex-col space-y-1">
        <Label className="text-xs">Red Notes</Label>
        <Textarea
          rows={5}
          className="min-h-24 flex-1"
          value={redNotes}
          onChange={(e) => {
            setRedNotes(e.target.value)
          }}
          disabled={disabled}
        />
      </div>
      {!disabled && (
        <Button size="sm" className="self-start" onClick={handleSave} disabled={patchRed.isPending}>
          {patchRed.isPending ? 'Saving...' : 'Save Red'}
        </Button>
      )}
    </TeamPanel>
  )
}

// ── Execution timer ───────────────────────────────────────────────────────────

/**
 * Seconds between two instants, ticking once a second while the second one is
 * still open. The clock is external state, so it is held in state and pushed by
 * an interval rather than read from `Date.now()` during render.
 */
function useElapsedSeconds(from?: string, to?: string): number | null {
  const [now, setNow] = useState(() => Date.now())
  const live = from != null && to == null

  /* eslint-disable react-hooks/set-state-in-effect -- a wall clock is external state; the first reading has to land when the timer starts, not a second after it */
  useEffect(() => {
    if (!live) return undefined
    setNow(Date.now())
    const id = setInterval(() => {
      setNow(Date.now())
    }, 1000)
    return () => {
      clearInterval(id)
    }
  }, [live, from])
  /* eslint-enable react-hooks/set-state-in-effect */

  if (from == null) return null
  const start = new Date(from).getTime()
  const end = to != null ? new Date(to).getTime() : now
  if (Number.isNaN(start) || Number.isNaN(end)) return null
  return Math.max(0, Math.round((end - start) / 1000))
}

/**
 * The red operator's stopwatch, and the manual entry beside it.
 *
 * Start and Stop write immediately (their handlers own the mutation); the two
 * datetime fields are ordinary editor state saved by Save Red, for the exercise
 * that was run before anyone opened the workbook — a timer nobody can override
 * is a timer that loses the run it was meant to record.
 *
 * `started_at` is one half of MTTD (`detected_at − started_at`, docs/scoring.md),
 * which is why the fields carry seconds: minute-truncated times would round the
 * measurement, and a start rounded up past blue's detection is a `400`.
 */
function ExecutionTimer({
  execution,
  startedAt,
  endedAt,
  onStartedAtChange,
  onEndedAtChange,
  onStart,
  onStop,
  disabled,
  pending,
}: {
  execution: Execution
  startedAt: string
  endedAt: string
  onStartedAtChange: (v: string) => void
  onEndedAtChange: (v: string) => void
  onStart: () => void
  onStop: () => void
  disabled: boolean
  pending: boolean
}) {
  // The clock reads committed server state, not the editor's unsaved fields:
  // it shows the run, not the draft.
  const elapsed = useElapsedSeconds(execution.startedAt, execution.endedAt)
  const running = execution.startedAt != null && execution.endedAt == null
  const inverted = isInverted(startedAt, endedAt)

  let caption = 'Not started'
  if (running) caption = 'Running'
  else if (elapsed != null) caption = 'Duration'

  return (
    <div className="space-y-2">
      <Label className="text-xs">Timing</Label>
      <div className="bg-background/60 flex items-center gap-3 rounded-md border px-3 py-2">
        <span
          className={cn(
            'font-mono text-2xl tabular-nums',
            running ? 'text-foreground' : 'text-muted-foreground',
          )}
          aria-label="Elapsed time"
        >
          {elapsed == null ? '--:--' : formatClock(elapsed)}
        </span>
        <span className="text-muted-foreground text-xs">{caption}</span>
        {!disabled && execution.startedAt == null && (
          <Button
            size="sm"
            variant="outline"
            className="ml-auto"
            onClick={onStart}
            disabled={pending}
          >
            <PlayIcon className="size-3.5" />
            Start
          </Button>
        )}
        {!disabled && running && (
          <Button
            size="sm"
            variant="outline"
            className="ml-auto"
            onClick={onStop}
            disabled={pending}
          >
            <SquareIcon className="size-3.5" />
            Stop
          </Button>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label htmlFor="red-started-at" className="text-xs">
            Started At
          </Label>
          <Input
            id="red-started-at"
            type="datetime-local"
            step={1}
            value={startedAt}
            onChange={(e) => {
              onStartedAtChange(e.target.value)
            }}
            disabled={disabled}
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor="red-ended-at" className="text-xs">
            Ended At
          </Label>
          <Input
            id="red-ended-at"
            type="datetime-local"
            step={1}
            value={endedAt}
            onChange={(e) => {
              onEndedAtChange(e.target.value)
            }}
            disabled={disabled}
          />
        </div>
      </div>
      {inverted && <p className="text-destructive text-xs">Ended at cannot precede started at.</p>}
    </div>
  )
}

function BlueDetectionEditor({
  engagementId,
  execution,
  closed,
  readOnly,
}: {
  engagementId: string
  execution: Execution
  closed: boolean
  readOnly: boolean
}) {
  const qc = useQueryClient()
  const patchBlue = usePatchBlueDetection()

  const [detectionCategory, setDetectionCategory] = useState(execution.detectionCategory ?? '')
  const [selectedModifiers, setSelectedModifiers] = useState<string[]>(execution.detectionModifiers)
  const [protection, setProtection] = useState(execution.protection ?? '')
  const [detectedAt, setDetectedAt] = useState(toLocalDatetime(execution.detectedAt))
  const [detectingSource, setDetectingSource] = useState(execution.detectingSource)
  const [detectingRuleRef, setDetectingRuleRef] = useState(execution.detectingRuleRef)
  const [alertSeverity, setAlertSeverity] = useState(execution.alertSeverity)
  const [blueNotes, setBlueNotes] = useState(execution.blueNotes)
  const [advancedOpen, setAdvancedOpen] = useState(execution.detectionModifiers.length > 0)

  // Reset local state when the execution version changes (M4-005).
  const [blueVersion, setBlueVersion] = useState(execution.version)
  /* eslint-disable react-hooks/set-state-in-effect -- syncing local edit state with external data on version change (M4-005) */
  useEffect(() => {
    if (execution.version !== blueVersion) {
      setBlueVersion(execution.version)
      setDetectionCategory(execution.detectionCategory ?? '')
      setSelectedModifiers(execution.detectionModifiers)
      setProtection(execution.protection ?? '')
      setDetectedAt(toLocalDatetime(execution.detectedAt))
      setDetectingSource(execution.detectingSource)
      setDetectingRuleRef(execution.detectingRuleRef)
      setAlertSeverity(execution.alertSeverity)
      setBlueNotes(execution.blueNotes)
      setAdvancedOpen(execution.detectionModifiers.length > 0)
    }
  }, [execution.version]) // eslint-disable-line react-hooks/exhaustive-deps
  /* eslint-enable react-hooks/set-state-in-effect */

  const disabled = closed || readOnly

  // MTTD is `detected_at − started_at`, so the server rejects a detection that
  // precedes the red start. Say so at the field rather than as a 400 on save.
  const beforeStart = isInverted(toLocalDatetime(execution.startedAt), detectedAt)

  const toggleModifier = (mod: string) => {
    setSelectedModifiers((prev) =>
      prev.includes(mod) ? prev.filter((m) => m !== mod) : [...prev, mod],
    )
  }

  const handleSave = () => {
    patchBlue.mutate(
      {
        engagementId,
        executionId: execution.id,
        body: {
          version: execution.version,
          detectionCategory:
            detectionCategory === ''
              ? undefined
              : (detectionCategory as BlueDetectionPatch['detectionCategory']),
          detectionModifiers:
            selectedModifiers.length > 0
              ? (selectedModifiers as BlueDetectionPatch['detectionModifiers'])
              : undefined,
          protection:
            protection === '' ? undefined : (protection as BlueDetectionPatch['protection']),
          detectedAt: editedDatetime(execution.detectedAt, detectedAt),
          detectingSource: detectingSource || undefined,
          detectingRuleRef: detectingRuleRef || undefined,
          alertSeverity:
            alertSeverity === ''
              ? undefined
              : (alertSeverity as BlueDetectionPatch['alertSeverity']),
          blueNotes: blueNotes || undefined,
        },
      },
      {
        onSuccess: () => toast.success('Blue detection saved'),
        onError: (err) => {
          if (isApiError(err, 'conflict')) {
            toast.error('This execution was modified by someone else. Reloading current state.')
            void qc.invalidateQueries({
              queryKey: engagementKeys.executions(engagementId),
            })
            void qc.invalidateQueries({
              queryKey: engagementKeys.execution(engagementId, execution.id),
            })
          } else {
            toast.error(err.message)
          }
        },
      },
    )
  }

  const modifierOptions = Object.entries(MODIFIER_LABEL)
  const protectionOptions = Object.entries(PROTECTION_LABEL)
  const severityOptions = Object.entries(SEVERITY_LABEL)

  const categories = [
    { key: 'none' as const, label: 'None', def: CATEGORY_DEFINITIONS.none },
    { key: 'telemetry' as const, label: 'Telemetry', def: CATEGORY_DEFINITIONS.telemetry },
    { key: 'general' as const, label: 'General', def: CATEGORY_DEFINITIONS.general },
    { key: 'tactic' as const, label: 'Tactic', def: CATEGORY_DEFINITIONS.tactic },
    { key: 'technique' as const, label: 'Technique', def: CATEGORY_DEFINITIONS.technique },
  ]

  return (
    <TeamPanel team="blue" title="Blue Detection">
      {/* Detection category — 5-button scale */}
      <div className="space-y-1.5">
        <Label className="text-xs">Detection Category</Label>
        <div className="grid grid-cols-5 gap-1">
          {categories.map((cat) => {
            const selected = detectionCategory === cat.key
            return (
              <Tooltip key={cat.key}>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setDetectionCategory(cat.key)
                    }}
                    className={cn(
                      'rounded-md border px-1.5 py-1.5 text-[11px] font-medium transition-colors',
                      'focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none',
                      disabled && 'cursor-not-allowed opacity-60',
                      selected
                        ? 'border-primary bg-primary text-primary-foreground'
                        : 'border-input bg-background hover:bg-accent hover:text-accent-foreground',
                    )}
                  >
                    {cat.label}
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-[240px] text-xs">
                  {cat.def}
                </TooltipContent>
              </Tooltip>
            )
          })}
        </div>
      </div>

      {/* Protection */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label className="text-xs">Protection</Label>
          <Select value={protection} onValueChange={setProtection} disabled={disabled}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">—</SelectItem>
              {protectionOptions.map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Detected At + MTTD */}
        <div className="space-y-1">
          <Label htmlFor="blue-detected-at" className="text-xs">
            Detected At
          </Label>
          <Input
            id="blue-detected-at"
            type="datetime-local"
            step={1}
            value={detectedAt}
            onChange={(e) => {
              setDetectedAt(e.target.value)
            }}
            disabled={disabled}
          />
          {execution.mttdSeconds != null && (
            <p className="text-muted-foreground text-xs">
              MTTD: {formatDuration(execution.mttdSeconds)}
            </p>
          )}
          {beforeStart && (
            <p className="text-destructive text-xs">
              Detected at cannot precede the red start time.
            </p>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label className="text-xs">Detecting Source</Label>
          <Input
            value={detectingSource}
            onChange={(e) => {
              setDetectingSource(e.target.value)
            }}
            disabled={disabled}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Alert Severity</Label>
          <Select value={alertSeverity} onValueChange={setAlertSeverity} disabled={disabled}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">—</SelectItem>
              {severityOptions.map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-1">
        <Label className="text-xs">Detecting Rule Ref</Label>
        <Input
          value={detectingRuleRef}
          onChange={(e) => {
            setDetectingRuleRef(e.target.value)
          }}
          disabled={disabled}
        />
      </div>

      {/* Advanced — modifiers */}
      <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
        <CollapsibleTrigger className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs transition-colors">
          <span>Advanced</span>
          <span className={cn('transition-transform', advancedOpen && 'rotate-90')}>▸</span>
        </CollapsibleTrigger>
        <CollapsibleContent className="space-y-1 pt-2">
          <Label className="text-xs">Detection Modifiers</Label>
          <div className="flex flex-wrap gap-2">
            {modifierOptions.map(([value, label]) => {
              const active = selectedModifiers.includes(value)
              return (
                <Tooltip key={value}>
                  <TooltipTrigger asChild>
                    <Badge
                      variant={active ? 'default' : 'outline'}
                      className={cn(
                        'select-none',
                        disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
                      )}
                      onClick={() => {
                        if (!disabled) toggleModifier(value)
                      }}
                    >
                      {label}
                    </Badge>
                  </TooltipTrigger>
                  <TooltipContent side="bottom" className="max-w-[240px] text-xs">
                    {MODIFIER_DEFINITIONS[value] ?? value}
                  </TooltipContent>
                </Tooltip>
              )
            })}
          </div>
        </CollapsibleContent>
      </Collapsible>

      <div className="flex flex-1 flex-col space-y-1">
        <Label className="text-xs">Blue Notes</Label>
        <Textarea
          rows={5}
          className="min-h-24 flex-1"
          value={blueNotes}
          onChange={(e) => {
            setBlueNotes(e.target.value)
          }}
          disabled={disabled}
        />
      </div>

      {!disabled && (
        <Button
          size="sm"
          className="self-start"
          onClick={handleSave}
          disabled={patchBlue.isPending}
        >
          {patchBlue.isPending ? 'Saving...' : 'Save Blue'}
        </Button>
      )}
    </TeamPanel>
  )
}

// ── Comments Section ──────────────────────────────────────────────────────────

function CommentsSection({
  engagementId,
  executionId,
  role,
}: {
  engagementId: string
  executionId: string | undefined
  role: EngagementRole
}) {
  const comments = useComments(executionId ? engagementId : undefined, executionId)
  const members = useEngagementMembers(engagementId)
  const user = useSignedInUser()
  const createComment = useCreateComment()
  const patchComment = usePatchComment()
  const [body, setBody] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editBody, setEditBody] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)
  const wasAtBottom = useRef(true)

  // Build author lookup map: userId → displayName
  const authorName = useMemo(() => {
    const map: Record<string, string> = {}
    if (members.data) {
      for (const m of members.data) {
        map[m.id] = m.displayName
      }
    }
    return map
  }, [members.data])

  // Check if user may edit a comment (author, lead, or admin)
  const canEdit = useCallback(
    (commentAuthorId: string) =>
      commentAuthorId === user.id || role === 'lead' || user.platformRole === 'admin',
    [user.id, user.platformRole, role],
  )

  // Track whether scroll is at bottom before re-renders
  const checkScrollBottom = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    wasAtBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24
  }, [])

  // Auto-scroll to bottom after new comments load, only if already at bottom
  useEffect(() => {
    const el = scrollRef.current
    if (el && wasAtBottom.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [comments.data])

  const handleSubmit = (e: SyntheticEvent) => {
    e.preventDefault()
    if (!body.trim() || !executionId) return
    createComment.mutate(
      { engagementId, executionId, body: { body: body.trim() } },
      {
        onSuccess: () => {
          setBody('')
          wasAtBottom.current = true
          toast.success('Comment added')
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  const startEdit = (comment: Comment) => {
    setEditingId(comment.id)
    setEditBody(comment.body)
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditBody('')
  }

  const saveEdit = (commentId: string) => {
    if (!editBody.trim()) return
    patchComment.mutate(
      {
        engagementId,
        commentId,
        body: { body: editBody.trim() },
      },
      {
        onSuccess: () => {
          setEditingId(null)
          setEditBody('')
          toast.success('Comment updated')
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  if (!executionId) {
    return (
      <p className="text-muted-foreground text-xs">
        Execution not yet available. Run the step first.
      </p>
    )
  }

  return (
    <div className="space-y-3">
      <h4 className="text-sm font-semibold">Comments</h4>

      {comments.data && comments.data.length > 0 ? (
        <div
          ref={scrollRef}
          className="max-h-48 space-y-2 overflow-y-auto"
          onScroll={checkScrollBottom}
        >
          {comments.data.map((c) => (
            <div key={c.id} className="rounded-md border p-2 text-sm">
              <div className="text-muted-foreground mb-1 flex items-center justify-between gap-2 text-xs">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{authorName[c.authorId] ?? c.authorId}</span>
                  <span>{formatMoment(c.createdAt)}</span>
                  {c.editedAt && <span className="italic">(edited)</span>}
                </div>
                {canEdit(c.authorId) && editingId !== c.id && (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => {
                      startEdit(c)
                    }}
                    aria-label="Edit comment"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="12"
                      height="12"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
                    </svg>
                  </Button>
                )}
              </div>

              {editingId === c.id ? (
                <div className="space-y-2">
                  <Textarea
                    rows={2}
                    value={editBody}
                    onChange={(e) => {
                      setEditBody(e.target.value)
                    }}
                  />
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      onClick={() => {
                        saveEdit(c.id)
                      }}
                      disabled={patchComment.isPending || !editBody.trim()}
                    >
                      {patchComment.isPending ? 'Saving...' : 'Save'}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={cancelEdit}>
                      Cancel
                    </Button>
                  </div>
                </div>
              ) : (
                <p className="whitespace-pre-wrap">{c.body}</p>
              )}
            </div>
          ))}
        </div>
      ) : (
        <p className="text-muted-foreground text-xs">No comments yet.</p>
      )}

      <form onSubmit={handleSubmit} className="space-y-2">
        <Textarea
          rows={2}
          placeholder="Add a comment..."
          value={body}
          onChange={(e) => {
            setBody(e.target.value)
          }}
        />
        <Button type="submit" size="sm" disabled={createComment.isPending || !body.trim()}>
          {createComment.isPending ? 'Posting...' : 'Post Comment'}
        </Button>
      </form>
    </div>
  )
}

// ── Evidence Section ──────────────────────────────────────────────────────────
function EvidenceSection({
  engagementId,
  executionId,
  role,
}: {
  engagementId: string
  executionId: string | undefined
  role: EngagementRole
}) {
  const qc = useQueryClient()
  const evidence = useEvidenceList(executionId ? engagementId : undefined, executionId)
  const fileRef = useRef<HTMLInputElement>(null)
  const [caption, setCaption] = useState('')
  const [uploading, setUploading] = useState(false)

  const handleUpload = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file || !executionId) return

    setUploading(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      if (caption.trim()) formData.append('caption', caption.trim())
      const side = canWriteRed(role) ? 'red' : 'blue'
      formData.append('side', side)

      const result = await api.POST('/executions/{executionId}/evidence', {
        params: { path: { executionId } },
        body: formData as unknown as components['schemas']['NewEvidenceRequest'],
        bodySerializer: (body) => body as unknown as FormData,
      })

      if (result.error !== undefined) {
        throw result.error instanceof Error ? result.error : new Error('Upload failed')
      }

      toast.success('Evidence uploaded')
      setCaption('')
      if (fileRef.current) fileRef.current.value = ''
      void qc.invalidateQueries({
        queryKey: engagementKeys.evidence(engagementId, executionId),
      })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  if (!executionId) {
    return <p className="text-muted-foreground text-xs">Execution not yet available.</p>
  }

  return (
    <div className="space-y-3">
      <h4 className="text-sm font-semibold">Evidence</h4>

      {evidence.data && evidence.data.length > 0 ? (
        <div className="space-y-2">
          {evidence.data.map((ev) => (
            <div key={ev.id} className="flex items-center gap-3 rounded-md border p-2 text-sm">
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{ev.filename}</p>
                <p className="text-muted-foreground text-xs">
                  {formatBytes(ev.size)} &middot; {ev.mime} &middot;{' '}
                  {ev.caption && <span>{ev.caption}</span>}
                </p>
              </div>
              <a
                href={`${API_BASE_URL}/engagements/${engagementId}/executions/${executionId}/evidence/${ev.id}/download`}
                className="text-primary shrink-0 text-xs hover:underline"
                download
              >
                Download
              </a>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-muted-foreground text-xs">No evidence uploaded.</p>
      )}

      <div className="space-y-2">
        <Input type="file" ref={fileRef} className="text-xs" />
        <Input
          placeholder="Caption (optional)"
          value={caption}
          onChange={(e) => {
            setCaption(e.target.value)
          }}
          className="text-xs"
        />
        <Button
          size="sm"
          onClick={() => {
            void handleUpload()
          }}
          disabled={uploading}
        >
          {uploading ? 'Uploading...' : 'Upload Evidence'}
        </Button>
      </div>
    </div>
  )
}

// ── Add Scenario Dialog ───────────────────────────────────────────────────────

function AddScenarioDialog({
  open,
  onOpenChange,
  engagementId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  engagementId: string
}) {
  const createScenario = useCreateScenario()
  const [name, setName] = useState('')
  const [narrative, setNarrative] = useState('')
  const [threatActor, setThreatActor] = useState('')

  const handleSubmit = (e: SyntheticEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    createScenario.mutate(
      {
        engagementId,
        body: {
          name: name.trim(),
          narrative: narrative.trim(),
          threatActor: threatActor.trim(),
          source: 'manual',
          sourceRef: '',
        } satisfies CreateScenario,
      },
      {
        onSuccess: () => {
          toast.success('Scenario created')
          setName('')
          setNarrative('')
          setThreatActor('')
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Scenario</DialogTitle>
          <DialogDescription>Create a new manual scenario for this engagement.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value)
              }}
              placeholder="Scenario name"
              required
            />
          </div>
          <div className="space-y-2">
            <Label>Threat Actor</Label>
            <Input
              value={threatActor}
              onChange={(e) => {
                setThreatActor(e.target.value)
              }}
              placeholder="e.g. APT29"
            />
          </div>
          <div className="space-y-2">
            <Label>Narrative</Label>
            <Textarea
              rows={3}
              value={narrative}
              onChange={(e) => {
                setNarrative(e.target.value)
              }}
              placeholder="Brief description of the scenario..."
            />
          </div>
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
            <Button type="submit" disabled={createScenario.isPending || !name.trim()}>
              {createScenario.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Add Step Dialog ───────────────────────────────────────────────────────────

function AddStepDialog({
  open,
  onOpenChange,
  engagementId,
  scenarios,
  defaultScenarioId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  engagementId: string
  scenarios: Scenario[]
  defaultScenarioId: string
}) {
  const createStep = useCreateStep()
  const [scenarioId, setScenarioId] = useState(defaultScenarioId)
  const [name, setName] = useState('')
  const [objective, setObjective] = useState('')
  const [techniqueId, setTechniqueId] = useState('')
  const [targetAsset, setTargetAsset] = useState('')

  // Sync defaultScenarioId when dialog opens
  const handleOpenChange = (open: boolean) => {
    if (open && defaultScenarioId) setScenarioId(defaultScenarioId)
    onOpenChange(open)
  }

  const handleSubmit = (e: SyntheticEvent) => {
    e.preventDefault()
    if (!name.trim() || !scenarioId) return
    createStep.mutate(
      {
        engagementId,
        scenarioId,
        body: {
          name: name.trim(),
          objective: objective.trim(),
          techniqueId: techniqueId.trim(),
          subtechniqueId: '',
          tacticId: '',
          templateId: '',
          targetAsset: targetAsset.trim(),
          tools: [],
          controlsInScope: [],
        } satisfies CreateStep,
      },
      {
        onSuccess: () => {
          toast.success('Step created')
          setName('')
          setObjective('')
          setTechniqueId('')
          setTargetAsset('')
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Step</DialogTitle>
          <DialogDescription>Create a new step in a scenario.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label>Scenario</Label>
            <Select value={scenarioId} onValueChange={setScenarioId}>
              <SelectTrigger>
                <SelectValue placeholder="Select scenario..." />
              </SelectTrigger>
              <SelectContent>
                {scenarios.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.ordinal}. {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value)
              }}
              placeholder="Step name"
              required
            />
          </div>
          <div className="space-y-2">
            <Label>Objective</Label>
            <Input
              value={objective}
              onChange={(e) => {
                setObjective(e.target.value)
              }}
              placeholder="What this step accomplishes"
            />
          </div>
          <div className="space-y-2">
            <Label>Technique ID</Label>
            <Input
              value={techniqueId}
              onChange={(e) => {
                setTechniqueId(e.target.value)
              }}
              placeholder="e.g. T1059"
            />
          </div>
          <div className="space-y-2">
            <Label>Target Asset</Label>
            <Input
              value={targetAsset}
              onChange={(e) => {
                setTargetAsset(e.target.value)
              }}
              placeholder="e.g. DC01"
            />
          </div>
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
            <Button type="submit" disabled={createStep.isPending || !name.trim() || !scenarioId}>
              {createStep.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Import CTID Dialog ────────────────────────────────────────────────────────

function ImportCtidDialog({
  open,
  onOpenChange,
  engagementId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  engagementId: string
}) {
  const plans = usePlans({})
  const importPlan = useImportPlan()

  const [planId, setPlanId] = useState('')
  const [name, setName] = useState('')
  const [startingOrdinal, setStartingOrdinal] = useState('1')

  const handleSubmit = (e: SyntheticEvent) => {
    e.preventDefault()
    if (!planId) return
    const body: ImportPlanRequest = { planId }
    if (name.trim()) body.name = name.trim()
    const ordinal = parseInt(startingOrdinal, 10)
    if (!isNaN(ordinal) && ordinal > 0) body.startingOrdinal = ordinal

    importPlan.mutate(
      { engagementId, body },
      {
        onSuccess: (data) => {
          toast.success(
            `Imported ${String(data.stepCount)} step${data.stepCount !== 1 ? 's' : ''} to scenario "${data.scenario.name}"`,
          )
          if (data.warnings.length > 0) {
            toast.warning(`${String(data.warnings.length)} technique(s) could not be resolved`)
          }
          setPlanId('')
          setName('')
          setStartingOrdinal('1')
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Import CTID Plan</DialogTitle>
          <DialogDescription>
            Import an emulation plan from the content library as a new scenario.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label>Plan</Label>
            <Select value={planId} onValueChange={setPlanId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a plan..." />
              </SelectTrigger>
              <SelectContent>
                {plans.data?.items.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Override Name (optional)</Label>
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value)
              }}
              placeholder="Leave blank to use plan name"
            />
          </div>
          <div className="space-y-2">
            <Label>Starting Ordinal</Label>
            <Input
              type="number"
              min={1}
              value={startingOrdinal}
              onChange={(e) => {
                setStartingOrdinal(e.target.value)
              }}
            />
          </div>
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
            <Button type="submit" disabled={importPlan.isPending || !planId}>
              {importPlan.isPending ? 'Importing...' : 'Import'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Step From Template Dialog ─────────────────────────────────────────────────

function StepFromTemplateDialog({
  open,
  onOpenChange,
  engagementId,
  scenarios,
  defaultScenarioId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  engagementId: string
  scenarios: Scenario[]
  defaultScenarioId: string
}) {
  const procedures = useProcedures({})
  const createFromTemplate = useCreateStepFromTemplate()

  const [scenarioId, setScenarioId] = useState(defaultScenarioId)
  const [templateId, setTemplateId] = useState('')
  const [name, setName] = useState('')
  const [objective, setObjective] = useState('')
  const [targetAsset, setTargetAsset] = useState('')
  const [argValuesText, setArgValuesText] = useState('')

  const handleOpenChange = (open: boolean) => {
    if (open && defaultScenarioId) setScenarioId(defaultScenarioId)
    onOpenChange(open)
  }

  const handleSubmit = (e: SyntheticEvent) => {
    e.preventDefault()
    if (!templateId || !scenarioId) return

    let argValues: Record<string, string> | undefined
    if (argValuesText.trim()) {
      argValues = {}
      for (const line of argValuesText.split('\n')) {
        const eq = line.indexOf('=')
        if (eq > 0) {
          argValues[line.slice(0, eq).trim()] = line.slice(eq + 1).trim()
        }
      }
    }

    const body: CreateStepFromTemplate = {
      templateId,
      name: name.trim() || undefined,
      objective: objective.trim() || undefined,
      targetAsset: targetAsset.trim() || undefined,
      argValues,
    }

    createFromTemplate.mutate(
      { engagementId, scenarioId, body },
      {
        onSuccess: () => {
          toast.success('Step created from template')
          setTemplateId('')
          setName('')
          setObjective('')
          setTargetAsset('')
          setArgValuesText('')
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Step From Template</DialogTitle>
          <DialogDescription>
            Create a step from a content library procedure template.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label>Scenario</Label>
            <Select value={scenarioId} onValueChange={setScenarioId}>
              <SelectTrigger>
                <SelectValue placeholder="Select scenario..." />
              </SelectTrigger>
              <SelectContent>
                {scenarios.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.ordinal}. {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Template</Label>
            <Select value={templateId} onValueChange={setTemplateId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a template..." />
              </SelectTrigger>
              <SelectContent>
                {procedures.data?.items.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Override Name (optional)</Label>
            <Input
              value={name}
              onChange={(e) => {
                setName(e.target.value)
              }}
              placeholder="Leave blank to use template name"
            />
          </div>
          <div className="space-y-2">
            <Label>Override Objective (optional)</Label>
            <Input
              value={objective}
              onChange={(e) => {
                setObjective(e.target.value)
              }}
              placeholder="Leave blank to use template description"
            />
          </div>
          <div className="space-y-2">
            <Label>Target Asset</Label>
            <Input
              value={targetAsset}
              onChange={(e) => {
                setTargetAsset(e.target.value)
              }}
              placeholder="e.g. DC01"
            />
          </div>
          <div className="space-y-2">
            <Label>Argument Values</Label>
            <Textarea
              rows={4}
              value={argValuesText}
              onChange={(e) => {
                setArgValuesText(e.target.value)
              }}
              placeholder={'KEY1=value1\nKEY2=value2'}
            />
            <p className="text-muted-foreground text-xs">
              One <code>KEY=VALUE</code> pair per line. Used for template variable substitution.
            </p>
          </div>
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
              type="submit"
              disabled={createFromTemplate.isPending || !templateId || !scenarioId}
            >
              {createFromTemplate.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${String(bytes)} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/**
 * Convert an ISO 8601 UTC string to a datetime-local value (no timezone
 * suffix). Seconds are kept: they are the resolution MTTD is measured at.
 */
function toLocalDatetime(iso: string | undefined): string {
  if (iso == null || iso === '') return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${String(d.getFullYear())}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/**
 * Convert a datetime-local value back to ISO 8601 UTC, or `undefined` when the
 * field is empty or half-typed — the API has no way to say "clear this", so an
 * absent value means "leave it alone".
 */
function fromLocalDatetime(local: string): string | undefined {
  if (local === '') return undefined
  const d = new Date(local)
  if (isNaN(d.getTime())) return undefined
  return d.toISOString()
}

/**
 * What to PATCH for a timestamp field: the edited value, or `undefined` when
 * the field still holds what the server sent.
 *
 * An untouched field is left out rather than echoed back, because the round
 * trip through `datetime-local` drops milliseconds — a timer press recorded at
 * `…:32.324Z` would come back as `…:32.000Z` on the next unrelated save, moving
 * a timestamp MTTD is measured from.
 */
function editedDatetime(current: string | undefined, local: string): string | undefined {
  if (local === toLocalDatetime(current)) return undefined
  return fromLocalDatetime(local)
}

/** True when both datetime-local fields are set and the second precedes the first. */
function isInverted(from: string, to: string): boolean {
  if (from === '' || to === '') return false
  const a = new Date(from).getTime()
  const b = new Date(to).getTime()
  if (isNaN(a) || isNaN(b)) return false
  return b < a
}

/** A stopwatch reading: `mm:ss`, growing to `h:mm:ss` past the hour. */
function formatClock(seconds: number): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return h > 0 ? `${String(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}

/** A duration in prose, matching the analytics MTTD card. */
function formatDuration(seconds: number): string {
  if (seconds < 60) return `${String(seconds)}s`
  if (seconds < 3600) return `${String(Math.floor(seconds / 60))}m ${String(seconds % 60)}s`
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${String(h)}h ${String(m)}m`
}
