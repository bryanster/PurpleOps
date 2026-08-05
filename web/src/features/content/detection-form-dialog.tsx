import { type ReactNode, type SyntheticEvent, useId, useState } from 'react'
import { toast } from 'sonner'

import { isApiError } from '@/api/errors'
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
import { Textarea } from '@/components/ui/textarea'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'

import {
  parseTechniqueIds,
  useCreateCustomDetection,
  useUpdateCustomDetection,
  type ContentDetectionRule,
} from './custom-queries'

/**
 * Create or edit a custom detection rule reference (M2-015).
 *
 * Body is a plain textarea (Monaco is optional and out of M2 scope). Reference
 * only — Blacklight never deploys rules.
 */
export function DetectionFormDialog({
  open,
  onOpenChange,
  initial,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: ContentDetectionRule
  onSaved?: (row: ContentDetectionRule) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        {open && (
          <DetectionFormBody
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

function DetectionFormBody({
  initial,
  onCancel,
  onSaved,
}: {
  initial?: ContentDetectionRule
  onCancel: () => void
  onSaved: (row: ContentDetectionRule) => void
}): ReactNode {
  const create = useCreateCustomDetection()
  const update = useUpdateCustomDetection()
  const pending = create.isPending || update.isPending
  const error = create.error ?? update.error
  const editing = initial !== undefined

  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [techniques, setTechniques] = useState(initial?.techniqueExternalIds.join(', ') ?? '')
  const [level, setLevel] = useState(initial?.level ?? '')
  const [body, setBody] = useState(initial?.ruleYaml ?? '')

  const nameId = useId()
  const descriptionId = useId()
  const techniquesId = useId()
  const levelId = useId()
  const bodyId = useId()
  const nameErrorId = useId()
  const bodyErrorId = useId()
  const techniquesErrorId = useId()

  const nameError = fieldErrorOf(error, 'name')
  const bodyError = fieldErrorOf(error, 'ruleYaml') ?? fieldErrorOf(error, 'rule_yaml')
  const techniquesError =
    fieldErrorOf(error, 'techniqueExternalIds') ?? fieldErrorOf(error, 'technique_external_ids')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    const payload = {
      name: name.trim(),
      description: description.trim(),
      techniqueExternalIds: parseTechniqueIds(techniques),
      level: level.trim(),
      ruleYaml: body,
    }

    if (editing) {
      update.mutate(
        { ruleId: initial.id, patch: payload },
        {
          onSuccess: (row) => {
            toast.success(`Saved “${row.name}”.`)
            onSaved(row)
          },
        },
      )
      return
    }

    create.mutate(payload, {
      onSuccess: (row) => {
        toast.success(`Created “${row.name}”.`)
        onSaved(row)
      },
    })
  }

  return (
    <>
      <DialogHeader className="border-b px-6 py-4 text-left">
        <DialogTitle>{editing ? 'Edit detection rule' : 'New detection rule'}</DialogTitle>
        <DialogDescription>
          Reference only — not deployed by Blacklight. Store the rule body so operators can copy it
          into their own tooling.
        </DialogDescription>
      </DialogHeader>

      <form className="flex min-h-0 flex-1 flex-col" onSubmit={onSubmit} noValidate>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
          {error !== null &&
            nameError === undefined &&
            bodyError === undefined &&
            techniquesError === undefined && (
              <FormAlert message={describeFailure(error)} error={error} />
            )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={nameId}>Title</Label>
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
              rows={2}
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
                placeholder="T1059.001"
                value={techniques}
                disabled={pending}
                aria-invalid={techniquesError !== undefined}
                aria-describedby={techniquesError === undefined ? undefined : techniquesErrorId}
                onChange={(event) => {
                  setTechniques(event.target.value)
                }}
              />
              <FieldError id={techniquesErrorId} message={techniquesError} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={levelId}>Level</Label>
              <Input
                id={levelId}
                placeholder="high"
                value={level}
                disabled={pending}
                onChange={(event) => {
                  setLevel(event.target.value)
                }}
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor={bodyId}>Rule body</Label>
            <Textarea
              id={bodyId}
              required
              rows={12}
              className="font-mono text-xs"
              value={body}
              disabled={pending}
              aria-invalid={bodyError !== undefined}
              aria-describedby={bodyError === undefined ? undefined : bodyErrorId}
              onChange={(event) => {
                setBody(event.target.value)
              }}
            />
            <FieldError id={bodyErrorId} message={bodyError} />
          </div>
        </div>

        <DialogFooter className="border-t px-6 py-4">
          <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending || name.trim() === '' || body.trim() === ''}>
            {pending ? 'Saving…' : editing ? 'Save changes' : 'Create detection'}
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
