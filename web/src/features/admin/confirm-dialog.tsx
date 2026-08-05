import type { ReactNode } from 'react'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

/**
 * The one confirmation dialog, used by every destructive action in the
 * identity screens.
 *
 * It exists as a component rather than as a pattern for a specific reason: the
 * ticket's requirement is that a destructive action says *what will happen*
 * ("this signs them out of 3 sessions"), and a shared component with a required
 * `description` makes that the only way to use it. A confirm built ad hoc at
 * each call site is one where somebody writes "Are you sure?" at four in the
 * afternoon.
 *
 * An alert dialog rather than a plain one: it takes focus, it traps it, Escape
 * cancels, and it returns focus to whatever opened it — all of which Radix
 * gives for free and none of which a hand-rolled overlay does.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  destructive = true,
  pending = false,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  /** What will happen, in a sentence, including any count the user should know. */
  description: ReactNode
  confirmLabel: string
  cancelLabel?: string
  destructive?: boolean
  pending?: boolean
  onConfirm: () => void
}): ReactNode {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>{cancelLabel}</AlertDialogCancel>
          <AlertDialogAction
            disabled={pending}
            className={
              destructive ? 'bg-destructive hover:bg-destructive/90 text-white' : undefined
            }
            onClick={(event) => {
              // The dialog closes itself on action by default; prevented so the
              // caller decides — a request that fails should leave the dialog
              // open with the failure visible rather than vanishing.
              event.preventDefault()
              onConfirm()
            }}
          >
            {pending ? 'Working…' : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
