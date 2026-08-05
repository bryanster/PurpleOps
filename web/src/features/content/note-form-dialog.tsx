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
  parseTags,
  useCreateCustomNote,
  useUpdateCustomNote,
  type ContentNote,
} from './custom-queries'

/**
 * Create or edit a custom knowledge-base note (M2-015).
 *
 * Textarea + plain preview is enough — rich markdown WYSIWYG is out of scope.
 */
export function NoteFormDialog({
  open,
  onOpenChange,
  initial,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: ContentNote
  onSaved?: (row: ContentNote) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        {open && (
          <NoteFormBody
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

function NoteFormBody({
  initial,
  onCancel,
  onSaved,
}: {
  initial?: ContentNote
  onCancel: () => void
  onSaved: (row: ContentNote) => void
}): ReactNode {
  const create = useCreateCustomNote()
  const update = useUpdateCustomNote()
  const pending = create.isPending || update.isPending
  const error = create.error ?? update.error
  const editing = initial !== undefined

  const [title, setTitle] = useState(initial?.title ?? '')
  const [body, setBody] = useState(initial?.bodyMarkdown ?? '')
  const [tags, setTags] = useState(initial?.tags.join(', ') ?? '')
  const [technique, setTechnique] = useState(initial?.techniqueExternalId ?? '')
  const [showPreview, setShowPreview] = useState(false)

  const titleId = useId()
  const bodyId = useId()
  const tagsId = useId()
  const techniqueId = useId()
  const titleErrorId = useId()
  const bodyErrorId = useId()
  const techniqueErrorId = useId()

  const titleError = fieldErrorOf(error, 'title')
  const bodyError = fieldErrorOf(error, 'bodyMarkdown') ?? fieldErrorOf(error, 'body_markdown')
  const techniqueError =
    fieldErrorOf(error, 'techniqueExternalId') ?? fieldErrorOf(error, 'technique_external_id')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    const payload = {
      title: title.trim(),
      bodyMarkdown: body,
      tags: parseTags(tags),
      techniqueExternalId: technique.trim(),
    }

    if (editing) {
      update.mutate(
        { noteId: initial.id, patch: payload },
        {
          onSuccess: (row) => {
            toast.success(`Saved “${row.title}”.`)
            onSaved(row)
          },
        },
      )
      return
    }

    create.mutate(payload, {
      onSuccess: (row) => {
        toast.success(`Created “${row.title}”.`)
        onSaved(row)
      },
    })
  }

  return (
    <>
      <DialogHeader className="border-b px-6 py-4 text-left">
        <DialogTitle>{editing ? 'Edit note' : 'New note'}</DialogTitle>
        <DialogDescription>
          Freeform markdown knowledge-base entry under the custom source. Body size is capped by
          server config.
        </DialogDescription>
      </DialogHeader>

      <form className="flex min-h-0 flex-1 flex-col" onSubmit={onSubmit} noValidate>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
          {error !== null &&
            titleError === undefined &&
            bodyError === undefined &&
            techniqueError === undefined && (
              <FormAlert message={describeFailure(error)} error={error} />
            )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={titleId}>Title</Label>
            <Input
              id={titleId}
              required
              autoFocus
              value={title}
              disabled={pending}
              aria-invalid={titleError !== undefined}
              aria-describedby={titleError === undefined ? undefined : titleErrorId}
              onChange={(event) => {
                setTitle(event.target.value)
              }}
            />
            <FieldError id={titleErrorId} message={titleError} />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor={techniqueId}>Technique (optional)</Label>
              <Input
                id={techniqueId}
                className="font-mono"
                placeholder="T1003"
                value={technique}
                disabled={pending}
                aria-invalid={techniqueError !== undefined}
                aria-describedby={techniqueError === undefined ? undefined : techniqueErrorId}
                onChange={(event) => {
                  setTechnique(event.target.value)
                }}
              />
              <FieldError id={techniqueErrorId} message={techniqueError} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={tagsId}>Tags</Label>
              <Input
                id={tagsId}
                placeholder="conti, credential-access"
                value={tags}
                disabled={pending}
                onChange={(event) => {
                  setTags(event.target.value)
                }}
              />
              <p className="text-muted-foreground text-xs">Comma-separated.</p>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-2">
              <Label htmlFor={bodyId}>Markdown body</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={pending}
                onClick={() => {
                  setShowPreview((value) => !value)
                }}
              >
                {showPreview ? 'Edit' : 'Preview'}
              </Button>
            </div>
            {showPreview ? (
              <pre className="bg-muted max-h-80 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
                {body === '' ? '—' : body}
              </pre>
            ) : (
              <Textarea
                id={bodyId}
                required
                rows={12}
                value={body}
                disabled={pending}
                aria-invalid={bodyError !== undefined}
                aria-describedby={bodyError === undefined ? undefined : bodyErrorId}
                onChange={(event) => {
                  setBody(event.target.value)
                }}
              />
            )}
            <FieldError id={bodyErrorId} message={bodyError} />
          </div>
        </div>

        <DialogFooter className="border-t px-6 py-4">
          <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending || title.trim() === '' || body.trim() === ''}>
            {pending ? 'Saving…' : editing ? 'Save changes' : 'Create note'}
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
