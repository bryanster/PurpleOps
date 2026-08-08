import { type ReactNode, type SyntheticEvent, useId, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Textarea } from '@/components/ui/textarea'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'

import {
  parsePlatforms,
  parseTechniqueIds,
  useCreateCustomProcedure,
  useUpdateCustomProcedure,
  type ContentProcedureInputArg,
  type ContentProcedureTemplate,
} from './custom-queries'

interface ArgDraft {
  key: string
  name: string
  description: string
  type: string
  default: string
}

function emptyArg(): ArgDraft {
  return {
    key: crypto.randomUUID(),
    name: '',
    description: '',
    type: 'string',
    default: '',
  }
}

function argsFromTemplate(template: ContentProcedureTemplate | undefined): ArgDraft[] {
  if (template === undefined || template.inputArgs.length === 0) {
    return []
  }
  return template.inputArgs.map((arg) => ({
    key: crypto.randomUUID(),
    name: arg.name,
    description: arg.description,
    type: arg.type === '' ? 'string' : arg.type,
    default: arg.default,
  }))
}

function toInputArgs(args: readonly ArgDraft[]): ContentProcedureInputArg[] {
  return args
    .map((arg) => ({
      name: arg.name.trim(),
      description: arg.description.trim(),
      type: arg.type.trim() === '' ? 'string' : arg.type.trim(),
      default: arg.default,
    }))
    .filter((arg) => arg.name !== '')
}

/**
 * Create or edit a custom procedure template (M2-015).
 *
 * Input args are a field array — not a JSON textarea — so the happy path stays
 * readable. Technique ids are free text (comma/space separated MITRE ids).
 */
export function ProcedureFormDialog({
  open,
  onOpenChange,
  initial,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: ContentProcedureTemplate
  onSaved?: (row: ContentProcedureTemplate) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        {open && (
          <ProcedureFormBody
            key={initial?.id ?? 'new'}
            initial={initial}
            onCancel={() => {
              onOpenChange(false)
            }}
            onSaved={(row) => {
              onSaved?.(row)
              onOpenChange(false)
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function ProcedureFormBody({
  initial,
  onCancel,
  onSaved,
}: {
  initial?: ContentProcedureTemplate
  onCancel: () => void
  onSaved: (row: ContentProcedureTemplate) => void
}): ReactNode {
  const create = useCreateCustomProcedure()
  const update = useUpdateCustomProcedure()
  const pending = create.isPending || update.isPending
  const error = create.error ?? update.error
  const editing = initial !== undefined

  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [techniques, setTechniques] = useState(initial?.techniqueExternalIds.join(', ') ?? '')
  const [platforms, setPlatforms] = useState(initial?.platforms.join(', ') ?? '')
  const [executor, setExecutor] = useState(initial?.executor ?? '')
  const [elevationRequired, setElevationRequired] = useState(initial?.elevationRequired ?? false)
  const [command, setCommand] = useState(initial?.command ?? '')
  const [cleanup, setCleanup] = useState(initial?.cleanup ?? '')
  const [args, setArgs] = useState<ArgDraft[]>(() => argsFromTemplate(initial))

  const nameId = useId()
  const descriptionId = useId()
  const techniquesId = useId()
  const platformsId = useId()
  const executorId = useId()
  const elevationId = useId()
  const commandId = useId()
  const cleanupId = useId()
  const nameErrorId = useId()
  const techniquesErrorId = useId()
  const commandErrorId = useId()

  const nameError = fieldErrorOf(error, 'name')
  const techniquesError =
    fieldErrorOf(error, 'techniqueExternalIds') ?? fieldErrorOf(error, 'technique_external_ids')
  const commandError = fieldErrorOf(error, 'command')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    const body = {
      name: name.trim(),
      description: description.trim(),
      platforms: parsePlatforms(platforms),
      executor: executor.trim(),
      elevationRequired,
      command,
      cleanup,
      inputArgs: toInputArgs(args),
      techniqueExternalIds: parseTechniqueIds(techniques),
    }

    if (editing) {
      update.mutate(
        { templateId: initial.id, patch: body },
        {
          onSuccess: (row) => {
            toast.success(`Saved “${row.name}”.`)
            onSaved(row)
          },
        },
      )
      return
    }

    create.mutate(body, {
      onSuccess: (row) => {
        toast.success(`Created “${row.name}”.`)
        onSaved(row)
      },
    })
  }

  return (
    <>
      <DialogHeader className="border-b px-6 py-4 text-left">
        <DialogTitle>{editing ? 'Edit procedure' : 'New procedure template'}</DialogTitle>
        <DialogDescription>
          Custom templates keep command, cleanup, and input arguments as separate fields — never a
          single flattened actions string.
        </DialogDescription>
      </DialogHeader>

      <form className="flex min-h-0 flex-1 flex-col" onSubmit={onSubmit} noValidate>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
          {error !== null &&
            nameError === undefined &&
            techniquesError === undefined &&
            commandError === undefined && (
              <FormAlert message={describeFailure(error)} error={error} />
            )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={nameId}>Name</Label>
            <Input
              id={nameId}
              required
              autoFocus
              value={name}
              disabled={pending}
              aria-invalid={nameError !== undefined}
              aria-describedby={nameError === undefined ? undefined : nameErrorId}
              onChange={(event) => {
                setName(event.target.value)
              }}
            />
            <FieldError id={nameErrorId} message={nameError} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor={descriptionId}>Description</Label>
            <Textarea
              id={descriptionId}
              rows={3}
              value={description}
              disabled={pending}
              onChange={(event) => {
                setDescription(event.target.value)
              }}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor={techniquesId}>Technique ids</Label>
              <Input
                id={techniquesId}
                className="font-mono"
                placeholder="T1059.001, T1059"
                value={techniques}
                disabled={pending}
                aria-invalid={techniquesError !== undefined}
                aria-describedby={techniquesError === undefined ? undefined : techniquesErrorId}
                onChange={(event) => {
                  setTechniques(event.target.value)
                }}
              />
              <FieldError id={techniquesErrorId} message={techniquesError} />
              <p className="text-muted-foreground text-xs">Optional. Comma or space separated.</p>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={platformsId}>Platforms</Label>
              <Input
                id={platformsId}
                placeholder="windows, linux"
                value={platforms}
                disabled={pending}
                onChange={(event) => {
                  setPlatforms(event.target.value)
                }}
              />
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor={executorId}>Executor</Label>
              <Input
                id={executorId}
                className="font-mono"
                placeholder="powershell, sh, bash…"
                value={executor}
                disabled={pending}
                onChange={(event) => {
                  setExecutor(event.target.value)
                }}
              />
            </div>
            <div className="flex items-center gap-2 pt-6">
              <Checkbox
                id={elevationId}
                checked={elevationRequired}
                disabled={pending}
                onCheckedChange={(value) => {
                  setElevationRequired(value === true)
                }}
              />
              <Label htmlFor={elevationId} className="font-normal">
                Elevation required
              </Label>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor={commandId}>Command</Label>
            <Textarea
              id={commandId}
              rows={5}
              className="font-mono text-xs"
              value={command}
              disabled={pending}
              aria-invalid={commandError !== undefined}
              aria-describedby={commandError === undefined ? undefined : commandErrorId}
              onChange={(event) => {
                setCommand(event.target.value)
              }}
            />
            <FieldError id={commandErrorId} message={commandError} />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor={cleanupId}>Cleanup</Label>
            <Textarea
              id={cleanupId}
              rows={3}
              className="font-mono text-xs"
              value={cleanup}
              disabled={pending}
              onChange={(event) => {
                setCleanup(event.target.value)
              }}
            />
          </div>

          <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-2">
              <h3 className="text-sm font-medium">Input arguments</h3>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={pending}
                onClick={() => {
                  setArgs((current) => [...current, emptyArg()])
                }}
              >
                Add argument
              </Button>
            </div>
            {args.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                No input arguments. Add one when the command references {'#{name}'} placeholders.
              </p>
            ) : (
              <ul className="flex flex-col gap-3">
                {args.map((arg, index) => (
                  <li key={arg.key} className="grid gap-2 rounded-md border p-3 sm:grid-cols-2">
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={`arg-name-${arg.key}`}>Name</Label>
                      <Input
                        id={`arg-name-${arg.key}`}
                        className="font-mono"
                        value={arg.name}
                        disabled={pending}
                        onChange={(event) => {
                          const value = event.target.value
                          setArgs((current) =>
                            current.map((row, i) => (i === index ? { ...row, name: value } : row)),
                          )
                        }}
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={`arg-type-${arg.key}`}>Type</Label>
                      <Input
                        id={`arg-type-${arg.key}`}
                        className="font-mono"
                        value={arg.type}
                        disabled={pending}
                        onChange={(event) => {
                          const value = event.target.value
                          setArgs((current) =>
                            current.map((row, i) => (i === index ? { ...row, type: value } : row)),
                          )
                        }}
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={`arg-default-${arg.key}`}>Default</Label>
                      <Input
                        id={`arg-default-${arg.key}`}
                        className="font-mono"
                        value={arg.default}
                        disabled={pending}
                        onChange={(event) => {
                          const value = event.target.value
                          setArgs((current) =>
                            current.map((row, i) =>
                              i === index ? { ...row, default: value } : row,
                            ),
                          )
                        }}
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={`arg-desc-${arg.key}`}>Description</Label>
                      <Input
                        id={`arg-desc-${arg.key}`}
                        value={arg.description}
                        disabled={pending}
                        onChange={(event) => {
                          const value = event.target.value
                          setArgs((current) =>
                            current.map((row, i) =>
                              i === index ? { ...row, description: value } : row,
                            ),
                          )
                        }}
                      />
                    </div>
                    <div className="sm:col-span-2">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        disabled={pending}
                        onClick={() => {
                          setArgs((current) => current.filter((_, i) => i !== index))
                        }}
                      >
                        Remove argument
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>

        <DialogFooter className="border-t px-6 py-4">
          <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending || name.trim() === ''}>
            {pending ? 'Saving…' : editing ? 'Save changes' : 'Create procedure'}
          </Button>
        </DialogFooter>
      </form>
    </>
  )
}

function describeFailure(error: unknown): string {
  if (isApiError(error)) {
    return error.message
  }
  return 'That request failed.'
}
