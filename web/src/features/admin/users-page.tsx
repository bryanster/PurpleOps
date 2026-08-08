import { type ReactNode, useId, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useSignedInUser } from '@/features/auth/current-user'
import { formatMoment } from '@/lib/time'

import { ConfirmDialog } from './confirm-dialog'
import { CreateUserDialog } from './create-user-dialog'
import { EditUserDialog } from './edit-user-dialog'
import {
  useDisableUser,
  useEnableUser,
  useRevokeUserSessions,
  useUsers,
  type PlatformRole,
  type User,
  type UserFilters,
  type UserStatus,
} from './queries'

/** The filter values that mean "do not narrow by this". Not a status or a role. */
const ANY = 'any'

/**
 * Administer accounts (M1-016, M1-017).
 *
 * Every destructive control here goes through [ConfirmDialog] with a sentence
 * saying what will happen — "signs them out of 3 sessions" rather than "are you
 * sure" — because the difference between disabling somebody and signing them
 * out is exactly the sort of thing that is obvious to whoever wrote the screen
 * and not to whoever is using it at the end of a long day.
 *
 * The screen is reachable only by an administrator, and that is enforced twice
 * over: `RequireAdmin` keeps a member off the route, and the server refuses
 * every request behind it (M1-013). The guard is for honesty; the server is the
 * access control.
 */
export function UsersPage(): ReactNode {
  const me = useSignedInUser()

  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<UserStatus | typeof ANY>(ANY)
  const [role, setRole] = useState<PlatformRole | typeof ANY>(ANY)

  const filters: UserFilters = {
    ...(search.trim() === '' ? {} : { q: search.trim() }),
    ...(status === ANY ? {} : { status }),
    ...(role === ANY ? {} : { role }),
  }
  const users = useUsers(filters)

  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<User | undefined>(undefined)

  const searchId = useId()
  const statusId = useId()
  const roleId = useId()

  const rows = users.data?.pages.flatMap((page) => page.items) ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold">Users</h1>
          <p className="text-muted-foreground max-w-prose text-sm">
            Everybody who can sign in to this installation. Role and status changes take effect on
            the person’s next request — they do not have to sign in again.
          </p>
        </div>
        <Button
          onClick={() => {
            setCreating(true)
          }}
        >
          New user
        </Button>
      </header>

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex min-w-56 flex-col gap-2">
          <Label htmlFor={searchId}>Search</Label>
          <Input
            id={searchId}
            type="search"
            placeholder="Name or email address"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value)
            }}
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={statusId}>Status</Label>
          <Select
            value={status}
            onValueChange={(value) => {
              setStatus(value as UserStatus | typeof ANY)
            }}
          >
            <SelectTrigger id={statusId} className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>Any status</SelectItem>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="invited">Invited</SelectItem>
              <SelectItem value="disabled">Disabled</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={roleId}>Role</Label>
          <Select
            value={role}
            onValueChange={(value) => {
              setRole(value as PlatformRole | typeof ANY)
            }}
          >
            <SelectTrigger id={roleId} className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>Any role</SelectItem>
              <SelectItem value="admin">Administrator</SelectItem>
              <SelectItem value="member">Member</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {users.isPending && <PageLoading label="Reading accounts…" />}

      {users.error && (
        <PageError
          error={users.error}
          onRetry={() => {
            void users.refetch()
          }}
        />
      )}

      {users.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title={filtered ? 'No accounts match those filters' : 'No accounts yet'}
            description={
              filtered
                ? 'Widen the search or clear the filters.'
                : 'Create one, or seed the first administrator with `blctl user create`.'
            }
          />
        ) : (
          <div className="flex flex-col gap-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Second factor</TableHead>
                  <TableHead>Last signed in</TableHead>
                  <TableHead className="w-0" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((user) => (
                  <UserRow
                    key={user.id}
                    user={user}
                    isSelf={user.id === me.id}
                    onEdit={() => {
                      setEditing(user)
                    }}
                  />
                ))}
              </TableBody>
            </Table>

            {users.hasNextPage && (
              <Button
                variant="outline"
                className="self-start"
                disabled={users.isFetchingNextPage}
                onClick={() => {
                  void users.fetchNextPage()
                }}
              >
                {users.isFetchingNextPage ? 'Loading…' : 'Load more'}
              </Button>
            )}
          </div>
        ))}

      <CreateUserDialog open={creating} onOpenChange={setCreating} />
      <EditUserDialog
        user={editing}
        onOpenChange={(open) => {
          if (!open) {
            setEditing(undefined)
          }
        }}
      />
    </div>
  )
}

function UserRow({
  user,
  isSelf,
  onEdit,
}: {
  user: User
  isSelf: boolean
  onEdit: () => void
}): ReactNode {
  const disable = useDisableUser()
  const enable = useEnableUser()
  const revokeSessions = useRevokeUserSessions()

  const [confirmingDisable, setConfirmingDisable] = useState(false)
  const [confirmingRevoke, setConfirmingRevoke] = useState(false)

  const busy = disable.isPending || enable.isPending || revokeSessions.isPending

  return (
    <TableRow>
      <TableCell className="font-medium">
        {user.displayName}
        {isSelf && (
          <Badge variant="secondary" className="ml-2">
            You
          </Badge>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground">{user.email}</TableCell>
      <TableCell>
        <Badge variant={user.platformRole === 'admin' ? 'default' : 'outline'}>
          {user.platformRole === 'admin' ? 'Administrator' : 'Member'}
        </Badge>
      </TableCell>
      <TableCell>
        <Badge variant={user.status === 'active' ? 'secondary' : 'outline'}>{user.status}</Badge>
      </TableCell>
      <TableCell className="text-sm">{user.mfaEnforced ? 'Required' : 'Not required'}</TableCell>
      <TableCell className="text-sm whitespace-nowrap">
        {user.lastLoginAt === undefined ? 'Never' : formatMoment(user.lastLoginAt)}
      </TableCell>
      <TableCell>
        <div className="flex justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={onEdit} disabled={busy}>
            Edit
          </Button>
          <Button
            variant="ghost"
            size="sm"
            disabled={busy}
            onClick={() => {
              setConfirmingRevoke(true)
            }}
          >
            Sign out
          </Button>
          {user.status === 'disabled' ? (
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                enable.mutate(
                  { userId: user.id },
                  {
                    onSuccess: () => {
                      toast.success(`${user.displayName} can sign in again.`)
                    },
                  },
                )
              }}
            >
              Enable
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                setConfirmingDisable(true)
              }}
            >
              Disable
            </Button>
          )}
        </div>
      </TableCell>

      <ConfirmDialog
        open={confirmingRevoke}
        onOpenChange={setConfirmingRevoke}
        title={`Sign ${user.displayName} out everywhere?`}
        description={`Every browser they are signed in on stops working immediately. Their account stays active and they can sign in again straight away. Service tokens they own are not sessions and keep working — disable the account to stop those too.`}
        confirmLabel="Sign them out"
        pending={revokeSessions.isPending}
        onConfirm={() => {
          revokeSessions.mutate(
            { userId: user.id },
            {
              onSuccess: (result) => {
                setConfirmingRevoke(false)
                toast.success(
                  result.revoked === 0
                    ? `${user.displayName} had no live sessions.`
                    : `Ended ${String(result.revoked)} ${result.revoked === 1 ? 'session' : 'sessions'}.`,
                )
              },
              onError: () => {
                toast.error('Those sessions could not be ended.')
              },
            },
          )
        }}
      />

      <ConfirmDialog
        open={confirmingDisable}
        onOpenChange={setConfirmingDisable}
        title={`Disable ${user.displayName}?`}
        description={
          isSelf
            ? 'This is your own account. Disabling it signs you out of every browser, including this one, and you will not be able to sign back in — another administrator would have to enable it.'
            : 'They are signed out of every browser at once, and every service token they own stops working at its next request. Nothing they have written is removed, and you can enable the account again later.'
        }
        confirmLabel="Disable account"
        pending={disable.isPending}
        onConfirm={() => {
          disable.mutate(
            { userId: user.id },
            {
              onSuccess: () => {
                setConfirmingDisable(false)
                toast.success(`${user.displayName} was disabled.`)
              },
              onError: (error) => {
                // The last-admin guard is a 409 with a sentence in it, and that
                // sentence is more useful than anything this screen could
                // invent — it names the rule that refused.
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </TableRow>
  )
}
