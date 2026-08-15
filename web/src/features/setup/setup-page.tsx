import { type ReactNode, useId, useState } from 'react'
import { useNavigate } from 'react-router'

import { PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CONTENT_SOURCES_PATH } from '@/features/content/paths'
import {
  ATTACK_SOURCE_ID,
  isActiveJobStatus,
  useContentJob,
  useContentSources,
  useEnableContentSource,
  useStartContentSourceSync,
} from '@/features/content/sources-queries'
import { formatMoment } from '@/lib/time'

import { SetupLayout } from './setup-layout'
import {
  useAttackReleases,
  useCompleteSetup,
  useSetupState,
  type ContentAttackRelease,
} from './queries'

/**
 * First-run setup: the screen the first administrator lands on, and the one
 * question it asks is which version of MITRE ATT&CK to install.
 *
 * # Why this screen exists
 *
 * The image ships with an empty content library on purpose — nothing is fetched
 * at first boot, so the container starts in seconds and starts at all on a
 * machine with no route out. The cost of that is an installation whose first
 * administrator signs in to a product that cannot map a step to a technique
 * yet. Everything works and nothing is useful. This closes that gap while the
 * person who can close it is looking at it.
 *
 * # Why a picker and not a button
 *
 * ATT&CK is versioned and engagements pin a version: a step recorded against
 * 15.1 means something different from the same step against 17.1, and
 * cross-engagement comparison is only honest between pins somebody chose. So
 * the first decision an installation makes is which release it works in, and
 * "latest" is offered as a default rather than assumed as a fact.
 *
 * # Why it can be skipped
 *
 * An air-gapped deployment cannot reach MITRE and installs from an offline
 * bundle instead (`docs/content-bundles.md`). Finishing without installing is
 * the correct answer for them, and a wizard that could not be finished would be
 * a screen they would learn to click past rather than read.
 */
export function SetupPage(): ReactNode {
  const navigate = useNavigate()
  const state = useSetupState()
  const complete = useCompleteSetup()

  const [jobId, setJobId] = useState<string | undefined>(undefined)

  // Every way out of this screen finishes setup, and that is deliberate: while
  // setup is unfinished the guard sends an administrator back here from every
  // in-app path, so a link that left without finishing would bounce. "Skip" and
  // "open Content sources" are therefore both endings, not escapes.
  function finish(to = '/'): void {
    complete.mutate(undefined, {
      onSuccess: () => {
        void navigate(to, { replace: true })
      },
    })
  }

  if (state.isPending) {
    return (
      <SetupLayout title="Setting up">
        <PageLoading label="Reading this installation's state…" />
      </SetupLayout>
    )
  }
  if (state.error) {
    return (
      <SetupLayout title="Setting up">
        <PageError
          error={state.error}
          onRetry={() => {
            void state.refetch()
          }}
        />
      </SetupLayout>
    )
  }

  if (jobId !== undefined) {
    return (
      <InstallStep
        jobId={jobId}
        onFinish={finish}
        finishing={complete.isPending}
        finishError={complete.error}
      />
    )
  }

  return (
    <ChooseStep
      onStarted={setJobId}
      onSkip={() => {
        finish()
      }}
      finishing={complete.isPending}
      finishError={complete.error}
    />
  )
}

/** Step one: which release, or none. */
function ChooseStep({
  onStarted,
  onSkip,
  finishing,
  finishError,
}: {
  onStarted: (jobId: string) => void
  onSkip: () => void
  finishing: boolean
  finishError: Error | null
}): ReactNode {
  const releases = useAttackReleases()
  const sources = useContentSources()
  const enable = useEnableContentSource()
  const sync = useStartContentSourceSync()

  const manualId = useId()
  const [chosen, setChosen] = useState<string | undefined>(undefined)
  const [manual, setManual] = useState('')
  const [failure, setFailure] = useState<string | undefined>(undefined)

  const catalog = releases.data
  const items = catalog?.items ?? []
  // The default selection is upstream's newest, and an installed release when
  // upstream did not answer. Nothing is preselected until the catalog arrives,
  // so a fast click cannot install a version chosen by a loading state.
  const suggested = items.find((r) => r.latest)?.version ?? items[0]?.version
  const selected = chosen ?? suggested

  const attackSource = sources.data?.items.find((s) => s.id === ATTACK_SOURCE_ID)
  const busy = enable.isPending || sync.isPending

  function install(version: string): void {
    setFailure(undefined)
    const start = (): void => {
      sync.mutate(
        { sourceId: ATTACK_SOURCE_ID, version },
        {
          onSuccess: (job) => {
            onStarted(job.id)
          },
          onError: (error) => {
            setFailure(error.message)
          },
        },
      )
    }
    // The seeded ATT&CK source is disabled, because an installation that has
    // never been configured should not reach the internet on its own. Choosing
    // a version here is the moment that stops being true, so enabling it is
    // part of installing rather than a separate errand on another screen.
    if (attackSource !== undefined && !attackSource.enabled) {
      enable.mutate(
        { sourceId: ATTACK_SOURCE_ID },
        {
          onSuccess: start,
          onError: (error) => {
            setFailure(error.message)
          },
        },
      )
      return
    }
    start()
  }

  return (
    <SetupLayout
      title="Choose a MITRE ATT&CK version"
      description={
        <>
          Engagements pin the ATT&amp;CK release they were assessed against, so this is the version
          your first engagements will work in. You can install others later from{' '}
          <span className="font-medium">Administration → Content sources</span>.
        </>
      }
    >
      <div className="flex flex-col gap-6">
        {releases.isPending && <PageLoading label="Asking MITRE which releases are available…" />}

        {releases.error && (
          <PageError
            error={releases.error}
            onRetry={() => {
              void releases.refetch()
            }}
          />
        )}

        {catalog !== undefined && (
          <>
            {catalog.reachable ? (
              <ReleasePicker
                items={items}
                selected={selected}
                disabled={busy}
                onSelect={setChosen}
              />
            ) : (
              <OfflineNotice
                reason={catalog.unreachable ?? undefined}
                installed={items.filter((r) => r.installed)}
              />
            )}

            {!catalog.reachable && (
              <div className="flex flex-col gap-2">
                <Label htmlFor={manualId}>Install a release by label</Label>
                <Input
                  id={manualId}
                  value={manual}
                  placeholder="e.g. 17.1"
                  disabled={busy}
                  onChange={(event) => {
                    setManual(event.target.value)
                  }}
                />
                <p className="text-muted-foreground text-xs">
                  The release list could not be read, but a download of a named release may still
                  work — only the index was out of reach.
                </p>
              </div>
            )}

            {attackSource !== undefined && (
              <p className="text-muted-foreground text-xs">{attackSource.attribution}</p>
            )}
          </>
        )}

        {failure !== undefined && (
          <p className="text-destructive text-sm" role="alert">
            {failure}
          </p>
        )}
        {finishError !== null && (
          <p className="text-destructive text-sm" role="alert">
            {finishError.message}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-3">
          <Button
            type="button"
            disabled={
              busy || finishing || installTarget(selected, manual, catalog?.reachable) === undefined
            }
            onClick={() => {
              const target = installTarget(selected, manual, catalog?.reachable)
              if (target !== undefined) {
                install(target)
              }
            }}
          >
            {busy ? 'Starting…' : 'Install and continue'}
          </Button>
          <Button type="button" variant="outline" disabled={busy || finishing} onClick={onSkip}>
            {finishing ? 'Finishing…' : 'Skip for now'}
          </Button>
        </div>

        <p className="text-muted-foreground text-sm">
          Skipping leaves the library empty. Nothing else is affected, and you can install content
          at any time from <span className="font-medium">Administration → Content sources</span>,
          including from an offline bundle on a machine with no internet access.
        </p>
      </div>
    </SetupLayout>
  )
}

/**
 * What "Install" would install: the selected release, or a hand-typed label
 * when there was no list to select from. `undefined` disables the button —
 * there is nothing to install and pretending otherwise would start a job that
 * fails on an empty pin.
 */
function installTarget(
  selected: string | undefined,
  manual: string,
  reachable: boolean | undefined,
): string | undefined {
  if (reachable === false) {
    const typed = manual.trim()
    return typed === '' ? undefined : typed
  }
  return selected
}

function ReleasePicker({
  items,
  selected,
  disabled,
  onSelect,
}: {
  items: ContentAttackRelease[]
  selected: string | undefined
  disabled: boolean
  onSelect: (version: string) => void
}): ReactNode {
  if (items.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        MITRE answered with no Enterprise releases. That is upstream having changed shape rather
        than a state this installation can fix — skip for now and try again from Content sources.
      </p>
    )
  }

  return (
    <fieldset className="flex flex-col gap-2" disabled={disabled}>
      <legend className="sr-only">ATT&amp;CK release</legend>
      {items.map((release) => (
        <label
          key={release.version}
          className="hover:bg-accent/50 has-checked:border-primary flex cursor-pointer items-center gap-3 rounded-md border p-3 text-sm"
        >
          <input
            type="radio"
            name="attack-release"
            value={release.version}
            checked={selected === release.version}
            onChange={() => {
              onSelect(release.version)
            }}
          />
          <span className="font-medium">ATT&amp;CK {release.version}</span>
          {release.latest && <Badge variant="secondary">Latest</Badge>}
          {release.installed && <Badge variant="outline">Installed</Badge>}
          {release.released !== undefined && (
            <span className="text-muted-foreground ml-auto text-xs">
              {formatMoment(release.released)}
            </span>
          )}
        </label>
      ))}
    </fieldset>
  )
}

function OfflineNotice({
  reason,
  installed,
}: {
  reason: string | undefined
  installed: ContentAttackRelease[]
}): ReactNode {
  return (
    <div className="flex flex-col gap-2 rounded-md border p-4 text-sm" role="status">
      <p className="font-medium">MITRE could not be reached from this server.</p>
      <p className="text-muted-foreground">
        That is expected on an air-gapped installation. Content can be installed from an offline
        bundle instead — upload one from Administration → Content sources — and this screen does not
        need to be finished before that.
      </p>
      {reason !== undefined && (
        <p className="text-muted-foreground font-mono text-xs break-words">{reason}</p>
      )}
      {installed.length > 0 && (
        <p className="text-muted-foreground">
          Already installed here: {installed.map((r) => r.version).join(', ')}.
        </p>
      )}
    </div>
  )
}

/** Step two: the install running, and the way out of the wizard. */
function InstallStep({
  jobId,
  onFinish,
  finishing,
  finishError,
}: {
  jobId: string
  onFinish: (to?: string) => void
  finishing: boolean
  finishError: Error | null
}): ReactNode {
  const job = useContentJob(jobId)
  const status = job.data?.status
  const running = status === undefined || isActiveJobStatus(status)
  const failed = status === 'failed' || status === 'interrupted' || status === 'cancelled'

  return (
    <SetupLayout
      title={running ? 'Installing ATT&CK' : failed ? 'The install did not finish' : 'Ready'}
      description={
        running
          ? 'This takes a few minutes on a first install. It keeps running if you leave — the progress is on the Content sources screen too.'
          : failed
            ? 'Nothing was left half-installed: a failed sync leaves the previous catalog exactly as it was.'
            : 'ATT&CK is installed. Engagements can now pin this release.'
      }
    >
      <div className="flex flex-col gap-6">
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm" aria-live="polite">
          <dt className="text-muted-foreground">Version</dt>
          <dd>{job.data?.version ?? '—'}</dd>
          <dt className="text-muted-foreground">Phase</dt>
          <dd>{job.data?.phase === '' ? 'queued' : (job.data?.phase ?? 'queued')}</dd>
          <dt className="text-muted-foreground">Progress</dt>
          <dd>
            {job.data === undefined || job.data.progressTotal === 0
              ? '—'
              : `${String(job.data.progressCurrent)} / ${String(job.data.progressTotal)}`}
          </dd>
          <dt className="text-muted-foreground">Status</dt>
          <dd>{status ?? 'queued'}</dd>
        </dl>

        {job.data?.message !== undefined && job.data.message !== '' && (
          <p className="text-muted-foreground text-sm">{job.data.message}</p>
        )}
        {job.data?.error !== undefined && job.data.error !== '' && (
          <p className="text-destructive text-sm" role="alert">
            {job.data.error}
          </p>
        )}

        {finishError !== null && (
          <p className="text-destructive text-sm" role="alert">
            {finishError.message}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-3">
          <Button
            type="button"
            disabled={finishing}
            onClick={() => {
              onFinish()
            }}
          >
            {finishing ? 'Finishing…' : running ? 'Finish and continue' : 'Continue to Blacklight'}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={finishing}
            onClick={() => {
              onFinish(CONTENT_SOURCES_PATH)
            }}
          >
            Finish and open Content sources
          </Button>
        </div>
      </div>
    </SetupLayout>
  )
}
