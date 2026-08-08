import { type ChangeEvent, type ReactNode, useId, useRef, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
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
import { FormAlert } from '@/features/auth/form-alert'

import {
  summarizeImportReport,
  useImportCustomContent,
  type ContentImportIssue,
  type ContentImportReport,
  type ImportFormat,
} from './custom-queries'

type Step = 'pick' | 'preview' | 'done'

const FORMAT_OPTIONS: readonly { value: ImportFormat; label: string }[] = [
  { value: 'auto', label: 'Auto-detect' },
  { value: 'testcases_json', label: 'v1 testcases.json' },
  { value: 'testcases_yaml', label: 'v1 testcase YAML / zip' },
  { value: 'knowledgebase_yaml', label: 'v1 knowledgebase YAML / zip' },
]

/**
 * Multi-step v1/custom import wizard (M2-015).
 *
 * Steps stay explicit so warnings are read before confirm: pick file → dry-run
 * preview → confirm write. File stays on the input until submit (never in
 * query cache).
 */
export function ImportWizardDialog({
  open,
  onOpenChange,
  onImported,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onImported?: () => void
}): ReactNode {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
      }}
    >
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-xl flex-col gap-0 overflow-hidden p-0 sm:max-w-xl">
        {open && (
          <ImportWizardBody
            onClose={() => {
              onOpenChange(false)
            }}
            onImported={onImported}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function ImportWizardBody({
  onClose,
  onImported,
}: {
  onClose: () => void
  onImported?: () => void
}): ReactNode {
  const importMutation = useImportCustomContent()
  const fileId = useId()
  const formatId = useId()
  const inputRef = useRef<HTMLInputElement>(null)

  const [step, setStep] = useState<Step>('pick')
  const [format, setFormat] = useState<ImportFormat>('auto')
  const [chosenName, setChosenName] = useState<string | undefined>(undefined)
  const [preview, setPreview] = useState<ContentImportReport | undefined>(undefined)
  const [finalReport, setFinalReport] = useState<ContentImportReport | undefined>(undefined)
  const [jobId, setJobId] = useState<string | undefined>(undefined)

  const pending = importMutation.isPending

  function resetFileInput(): void {
    if (inputRef.current !== null) {
      inputRef.current.value = ''
    }
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>): void {
    const file = event.target.files?.[0]
    setChosenName(file?.name)
    setPreview(undefined)
    setFinalReport(undefined)
    setJobId(undefined)
    setStep('pick')
    importMutation.reset()
  }

  function requireFile(): File | undefined {
    const file = inputRef.current?.files?.[0]
    if (file === undefined) {
      toast.error('Choose a file first.')
      return undefined
    }
    return file
  }

  function runDryRun(): void {
    const file = requireFile()
    if (file === undefined) {
      return
    }
    importMutation.mutate(
      { file, format, dryRun: true },
      {
        onSuccess: (result) => {
          if (result.kind !== 'report') {
            toast.error('Dry-run returned a job unexpectedly.')
            return
          }
          setPreview(result.report)
          setStep('preview')
        },
      },
    )
  }

  function confirmImport(): void {
    const file = requireFile()
    if (file === undefined) {
      return
    }
    importMutation.mutate(
      { file, format, dryRun: false },
      {
        onSuccess: (result) => {
          if (result.kind === 'job') {
            setJobId(result.job.id)
            setFinalReport(undefined)
            setStep('done')
            toast.success(`Import queued (${result.job.id}).`)
            onImported?.()
            return
          }
          setFinalReport(result.report)
          setJobId(undefined)
          setStep('done')
          toast.success(summarizeImportReport(result.report))
          onImported?.()
        },
      },
    )
  }

  return (
    <>
      <DialogHeader className="border-b px-6 py-4 text-left">
        <DialogTitle>Import custom content</DialogTitle>
        <DialogDescription>
          Upload a v1 testcases file, knowledgebase YAML, custom export, or a zip of the above.
          Preview is a dry-run — nothing is written until you confirm.
        </DialogDescription>
      </DialogHeader>

      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
        <ol className="text-muted-foreground flex flex-wrap gap-2 text-xs">
          <StepChip active={step === 'pick'} done={step !== 'pick'} label="1. Choose file" />
          <StepChip active={step === 'preview'} done={step === 'done'} label="2. Review dry-run" />
          <StepChip active={step === 'done'} done={false} label="3. Confirm" />
        </ol>

        {importMutation.error !== null && step !== 'done' && (
          <FormAlert message={describeFailure(importMutation.error)} error={importMutation.error} />
        )}

        {(step === 'pick' || step === 'preview') && (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor={fileId}>File</Label>
              <Input
                id={fileId}
                ref={inputRef}
                type="file"
                accept=".json,.yaml,.yml,.zip,application/json,application/zip,text/yaml,application/x-yaml"
                disabled={pending}
                onChange={handleFileChange}
              />
              {chosenName !== undefined && (
                <p className="text-muted-foreground text-xs">{chosenName}</p>
              )}
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor={formatId}>Format</Label>
              <Select
                value={format}
                onValueChange={(value) => {
                  setFormat(value as ImportFormat)
                  setPreview(undefined)
                  if (step === 'preview') {
                    setStep('pick')
                  }
                }}
                disabled={pending}
              >
                <SelectTrigger id={formatId}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FORMAT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {step === 'preview' && preview !== undefined && <ImportReportView report={preview} />}

        {step === 'done' && finalReport !== undefined && (
          <div className="flex flex-col gap-3">
            <p className="text-sm font-medium">{summarizeImportReport(finalReport)}</p>
            <ImportReportView report={finalReport} />
          </div>
        )}

        {step === 'done' && jobId !== undefined && (
          <div className="flex flex-col gap-2">
            <p className="text-sm">
              Large import enqueued as job <code className="font-mono text-xs">{jobId}</code>. Watch
              progress under Content sources.
            </p>
          </div>
        )}
      </div>

      <DialogFooter className="border-t px-6 py-4">
        {step === 'pick' && (
          <>
            <Button type="button" variant="outline" disabled={pending} onClick={onClose}>
              Cancel
            </Button>
            <Button
              type="button"
              disabled={pending || chosenName === undefined}
              onClick={runDryRun}
            >
              {pending ? 'Previewing…' : 'Dry-run preview'}
            </Button>
          </>
        )}
        {step === 'preview' && (
          <>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => {
                setStep('pick')
                setPreview(undefined)
                importMutation.reset()
              }}
            >
              Back
            </Button>
            <Button
              type="button"
              disabled={pending || (preview !== undefined && preview.errors.length > 0)}
              title={
                preview !== undefined && preview.errors.length > 0
                  ? 'Fix import errors before confirming'
                  : undefined
              }
              onClick={confirmImport}
            >
              {pending ? 'Importing…' : 'Confirm import'}
            </Button>
          </>
        )}
        {step === 'done' && (
          <Button
            type="button"
            onClick={() => {
              resetFileInput()
              onClose()
            }}
          >
            Done
          </Button>
        )}
      </DialogFooter>
    </>
  )
}

function StepChip({
  label,
  active,
  done,
}: {
  label: string
  active: boolean
  done: boolean
}): ReactNode {
  return (
    <li>
      <Badge variant={active ? 'default' : done ? 'secondary' : 'outline'}>{label}</Badge>
    </li>
  )
}

function ImportReportView({ report }: { report: ContentImportReport }): ReactNode {
  return (
    <div className="flex flex-col gap-3 rounded-md border p-3 text-sm">
      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">format: {report.format}</Badge>
        {report.dryRun && <Badge variant="secondary">dry-run</Badge>}
      </div>
      <dl className="grid gap-1 sm:grid-cols-2">
        <Count label="Procedures created" value={report.proceduresCreated} />
        <Count label="Procedures updated" value={report.proceduresUpdated} />
        <Count label="Detections created" value={report.detectionsCreated} />
        <Count label="Detections updated" value={report.detectionsUpdated} />
        <Count label="Notes created" value={report.notesCreated} />
        <Count label="Notes updated" value={report.notesUpdated} />
      </dl>
      <IssueList title="Warnings" items={report.warnings} tone="warning" />
      <IssueList title="Errors" items={report.errors} tone="error" />
    </div>
  )
}

function Count({ label, value }: { label: string; value: number }): ReactNode {
  return (
    <div className="flex justify-between gap-2 sm:block">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-mono">{value}</dd>
    </div>
  )
}

function IssueList({
  title,
  items,
  tone,
}: {
  title: string
  items: readonly ContentImportIssue[]
  tone: 'warning' | 'error'
}): ReactNode {
  if (items.length === 0) {
    return null
  }
  return (
    <section className="flex flex-col gap-1">
      <h4
        className={
          tone === 'error' ? 'text-destructive text-sm font-medium' : 'text-sm font-medium'
        }
      >
        {title} ({items.length})
      </h4>
      <ul className="flex max-h-40 flex-col gap-1 overflow-y-auto text-xs">
        {items.map((item, index) => (
          <li key={`${item.path}-${String(index)}`} className="rounded border px-2 py-1">
            <span className="font-mono">{item.path}</span>
            <span className="text-muted-foreground"> — {item.message}</span>
          </li>
        ))}
      </ul>
    </section>
  )
}

function describeFailure(error: unknown): string {
  if (isApiError(error)) {
    return error.message
  }
  return 'That request failed.'
}
