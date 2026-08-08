import { CheckIcon, CopyIcon, TriangleAlertIcon } from 'lucide-react'
import { type ReactNode, type SyntheticEvent, useId, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'
import { fieldErrorOf } from '@/features/auth/field-errors'
import { FieldError, FormAlert } from '@/features/auth/form-alert'
import { formatMoment } from '@/lib/time'

import {
  TOKEN_SCOPES,
  useCreateServiceToken,
  useRevokeServiceToken,
  useServiceTokens,
  type CreatedServiceToken,
  type ServiceToken,
  type TokenScope,
} from './queries'

/**
 * Service tokens (M1-011, M1-017): credentials for programs rather than for
 * people.
 *
 * The screen is built around one fact — the secret exists in exactly one HTTP
 * response, ever — so the creation dialog does not close on success. It changes
 * into the only place that value will be shown, with the warning stated before
 * the token rather than under it.
 *
 * Revoked and expired tokens stay in the list. You cannot decide whether to
 * rotate something whose history you cannot see, and a list that hid them would
 * make "did I already revoke that?" unanswerable.
 */
export function TokensPage(): ReactNode {
  const tokens = useServiceTokens()
  const [creating, setCreating] = useState(false)
  const [revoking, setRevoking] = useState<ServiceToken | undefined>(undefined)
  const revoke = useRevokeServiceToken()

  const items = tokens.data?.items ?? []

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold">Service tokens</h1>
          <p className="text-muted-foreground max-w-prose text-sm">
            For scripts and pipelines. A token can never do more than you can — its permissions are
            read from your account on every request, so being demoted narrows every token you hold
            and being disabled stops them.
          </p>
        </div>
        <Button
          onClick={() => {
            setCreating(true)
          }}
        >
          New token
        </Button>
      </header>

      {tokens.isPending && <PageLoading label="Reading your tokens…" />}

      {tokens.error && (
        <PageError
          error={tokens.error}
          onRetry={() => {
            void tokens.refetch()
          }}
        />
      )}

      {tokens.data &&
        (items.length === 0 ? (
          <PageEmpty
            title="No tokens yet"
            description="Create one when a script needs to talk to this installation. It will be shown once and stored as a hash."
            action={
              // Named differently from the button in the header on purpose:
              // two controls with the same accessible name on one screen is a
              // screen reader announcing the same thing twice, and an
              // ambiguous target for anybody driving by keyboard or by script.
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setCreating(true)
                }}
              >
                Create your first token
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Prefix</TableHead>
                <TableHead>Scopes</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((token) => (
                <TableRow key={token.id}>
                  <TableCell className="font-medium">{token.name}</TableCell>
                  <TableCell className="font-mono text-xs">{token.prefix}</TableCell>
                  <TableCell className="max-w-64">
                    <div className="flex flex-wrap gap-1">
                      {token.scopes.map((scope) => (
                        <Badge key={scope} variant="secondary" className="font-mono text-xs">
                          {scope}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    {token.lastUsedAt === undefined ? 'Never' : formatMoment(token.lastUsedAt)}
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    {formatMoment(token.expiresAt)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={token.status === 'active' ? 'default' : 'outline'}>
                      {token.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {token.status === 'active' && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setRevoking(token)
                        }}
                      >
                        Revoke
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}

      <CreateTokenDialog open={creating} onOpenChange={setCreating} />

      <ConfirmDialog
        open={revoking !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            setRevoking(undefined)
          }
        }}
        title="Revoke this token?"
        description={
          revoking === undefined
            ? ''
            : `“${revoking.name}” (${revoking.prefix}) stops working at its next request. Anything using it will start failing, and this cannot be undone — mint a new token instead.`
        }
        confirmLabel="Revoke token"
        pending={revoke.isPending}
        onConfirm={() => {
          if (revoking === undefined) {
            return
          }
          revoke.mutate(
            { tokenId: revoking.id },
            {
              onSuccess: () => {
                setRevoking(undefined)
                toast.success('That token was revoked.')
              },
              onError: () => {
                toast.error('That token could not be revoked.')
              },
            },
          )
        }}
      />
    </div>
  )
}

/** How far out a new token's expiry defaults to. A year is the server's maximum. */
const DEFAULT_EXPIRY_DAYS = 90

function CreateTokenDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        {/* The body is its own component so that closing the dialog unmounts
            it. The token lives in that state and nowhere else — not in the
            query cache, not in storage — so this is the moment it stops
            existing on this machine. */}
        <CreateTokenDialogBody
          onDone={() => {
            onOpenChange(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

function CreateTokenDialogBody({ onDone }: { onDone: () => void }): ReactNode {
  const create = useCreateServiceToken()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<TokenScope[]>([])
  const [expiresOn, setExpiresOn] = useState(defaultExpiryDate())
  const [created, setCreated] = useState<CreatedServiceToken | undefined>(undefined)

  const nameId = useId()
  const expiryId = useId()
  const nameErrorId = useId()
  const expiryErrorId = useId()

  const nameError = fieldErrorOf(create.error, 'name')
  const expiryError = fieldErrorOf(create.error, 'expiresAt')
  const scopesError = fieldErrorOf(create.error, 'scopes')

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    create.mutate(
      {
        name: name.trim(),
        scopes,
        // A date input gives a calendar day; the API wants an instant. End of
        // that day in the browser's own zone, so "expires on the 3rd" means the
        // 3rd where the person choosing it lives.
        expiresAt: endOfDay(expiresOn),
      },
      { onSuccess: setCreated },
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{created ? 'Copy your token now' : 'New service token'}</DialogTitle>
        <DialogDescription>
          {created
            ? 'This is the only time it will be shown.'
            : 'It can do what you can do, narrowed by the scopes you pick.'}
        </DialogDescription>
      </DialogHeader>

      {created ? (
        <CreatedTokenPanel
          created={created}
          onDone={() => {
            onDone()
            toast.success('Token created.')
          }}
        />
      ) : (
        <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
          {create.error !== null &&
            nameError === undefined &&
            expiryError === undefined &&
            scopesError === undefined && (
              <FormAlert message="That token could not be created." error={create.error} />
            )}

          <div className="flex flex-col gap-2">
            <Label htmlFor={nameId}>Name</Label>
            <Input
              id={nameId}
              autoFocus
              required
              placeholder="nightly-report-export"
              value={name}
              maxLength={200}
              onChange={(event) => {
                setName(event.target.value)
              }}
              aria-describedby={nameError === undefined ? undefined : nameErrorId}
              aria-invalid={nameError !== undefined}
            />
            <FieldError id={nameErrorId} message={nameError} />
            <p className="text-muted-foreground text-xs">
              What this token is for. It is what you will read when deciding whether it is still
              needed.
            </p>
          </div>

          <fieldset className="flex flex-col gap-2">
            <legend className="mb-2 text-sm font-medium">Scopes</legend>
            <div className="flex flex-col gap-3">
              {TOKEN_SCOPES.map((entry) => (
                <ScopeCheckbox
                  key={entry.scope}
                  scope={entry.scope}
                  label={entry.label}
                  description={entry.description}
                  checked={scopes.includes(entry.scope)}
                  onToggle={(checked) => {
                    setScopes((current) =>
                      checked
                        ? [...current, entry.scope]
                        : current.filter((scope) => scope !== entry.scope),
                    )
                  }}
                />
              ))}
            </div>
            <FieldError id="token-scopes-error" message={scopesError} />
          </fieldset>

          <div className="flex flex-col gap-2">
            <Label htmlFor={expiryId}>Expires on</Label>
            <Input
              id={expiryId}
              type="date"
              required
              value={expiresOn}
              min={today()}
              onChange={(event) => {
                setExpiresOn(event.target.value)
              }}
              aria-describedby={expiryError === undefined ? undefined : expiryErrorId}
              aria-invalid={expiryError !== undefined}
            />
            <FieldError id={expiryErrorId} message={expiryError} />
            <p className="text-muted-foreground text-xs">
              Required, and at most a year out. A credential with no expiry is one nobody remembers
              to revoke.
            </p>
          </div>

          <Button
            type="submit"
            disabled={create.isPending || name.trim() === '' || scopes.length === 0}
          >
            {create.isPending ? 'Creating…' : 'Create token'}
          </Button>
        </form>
      )}
    </>
  )
}

function ScopeCheckbox({
  scope,
  label,
  description,
  checked,
  onToggle,
}: {
  scope: TokenScope
  label: string
  description: string
  checked: boolean
  onToggle: (checked: boolean) => void
}): ReactNode {
  const id = useId()
  const descriptionId = useId()

  return (
    <div className="flex items-start gap-2">
      <Checkbox
        id={id}
        checked={checked}
        aria-describedby={descriptionId}
        onCheckedChange={(value) => {
          onToggle(value === true)
        }}
      />
      <div className="flex flex-col gap-0.5">
        <Label htmlFor={id} className="font-normal">
          {label} <code className="text-muted-foreground font-mono text-xs">{scope}</code>
        </Label>
        <p id={descriptionId} className="text-muted-foreground text-xs">
          {description}
        </p>
      </div>
    </div>
  )
}

/**
 * The secret, once.
 *
 * Same treatment as the recovery codes and for the same reason: a warning that
 * cannot be missed, a copy button, and a deliberate acknowledgement before the
 * dialog will close. What differs is that there is nothing to download — a
 * token belongs in whatever secret store the script reads, not in a text file
 * in ~/Downloads.
 */
function CreatedTokenPanel({
  created,
  onDone,
}: {
  created: CreatedServiceToken
  onDone: () => void
}): ReactNode {
  const [copied, setCopied] = useState(false)
  const [acknowledged, setAcknowledged] = useState(false)
  const checkboxId = useId()

  return (
    <div className="flex flex-col gap-4">
      <div
        role="alert"
        className="border-destructive/40 bg-destructive/10 text-destructive flex gap-2 rounded-md border p-3 text-sm"
      >
        <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
        <p>
          Copy this now. It is stored as a hash, so nobody — not an administrator, not the server —
          can show it to you again. If you lose it, revoke this token and create another.
        </p>
      </div>

      <code className="bg-muted/50 rounded-md border p-3 font-mono text-sm break-all select-all">
        {created.token}
      </code>

      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            navigator.clipboard.writeText(created.token).then(
              () => {
                setCopied(true)
              },
              () => {
                setCopied(false)
              },
            )
          }}
        >
          {copied ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
          {copied ? 'Copied' : 'Copy token'}
        </Button>
      </div>

      <dl className="text-sm">
        <div className="flex gap-2">
          <dt className="text-muted-foreground w-24">Prefix</dt>
          <dd className="font-mono text-xs">{created.serviceToken.prefix}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="text-muted-foreground w-24">Expires</dt>
          <dd>{formatMoment(created.serviceToken.expiresAt)}</dd>
        </div>
      </dl>

      <div className="flex items-start gap-2">
        <Checkbox
          id={checkboxId}
          checked={acknowledged}
          onCheckedChange={(checked) => {
            setAcknowledged(checked === true)
          }}
        />
        <Label htmlFor={checkboxId} className="text-sm leading-snug font-normal">
          I have saved this token somewhere it can be read from again.
        </Label>
      </div>

      <Button type="button" onClick={onDone} disabled={!acknowledged}>
        Done
      </Button>
    </div>
  )
}

/** Today, as the `yyyy-mm-dd` a date input wants. */
function today(): string {
  return toDateInput(new Date())
}

function defaultExpiryDate(): string {
  const at = new Date()
  at.setDate(at.getDate() + DEFAULT_EXPIRY_DAYS)
  return toDateInput(at)
}

function toDateInput(at: Date): string {
  const month = String(at.getMonth() + 1).padStart(2, '0')
  const day = String(at.getDate()).padStart(2, '0')
  return `${String(at.getFullYear())}-${month}-${day}`
}

/** The last instant of a calendar day, in the browser's zone, as RFC 3339. */
function endOfDay(date: string): string {
  const at = new Date(`${date}T23:59:59`)
  return at.toISOString()
}
