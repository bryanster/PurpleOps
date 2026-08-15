# First-run setup

A fresh installation boots with an empty content library. That is deliberate —
nothing is fetched at first boot, so the container starts in seconds and starts
at all on a machine with no route out — and the cost of it is that the first
administrator signs in to a product which cannot yet map a step to an ATT&CK
technique. Every screen works and none of them is useful.

The setup wizard closes that gap while the person who can close it is looking at
it. It asks one question: **which version of MITRE ATT&CK to install.**

## What the administrator sees

The first administrator to sign in on an installation nobody has set up is sent
to `/setup` from wherever they were going, and stays there until they finish it.
Members are not: an empty library is not theirs to fill, and the endpoints the
wizard uses are administrators-only anyway, so they see the product as it is.

The screen lists every Enterprise release MITRE currently publishes, newest
first, with the newest preselected and anything already installed marked as
such. Choosing one and pressing **Install and continue** does two things in
order:

1. **Enables the ATT&CK source.** It is seeded disabled, because an installation
   nobody has configured should not reach the internet on its own. Choosing a
   version is the moment that stops being true.
2. **Starts a sync** pinned to the chosen release — the same job the sources
   admin screen starts, on the same single global slot, streaming the same
   progress.

The install keeps running if the administrator leaves. **Skip for now** finishes
setup without installing anything.

## Why a picker and not a button

ATT&CK is versioned and engagements pin a version. A step assessed against 15.1
means something different from the same step against 17.1, and cross-engagement
comparison is only honest between pins somebody chose. So the first decision an
installation makes is which release it works in, and "latest" is offered as a
default rather than assumed as a fact.

Version labels are not sorted here or anywhere else: `4.0`, `10.0` and `17.1`
order correctly under neither string comparison nor semver. The list is in
MITRE's own order, which is newest first. See
[`content-attack.md`](content-attack.md#version-discovery) for the endpoint the
picker reads.

## Air-gapped installations

MITRE's release index is read while the request is open, and on an installation
with no route out it cannot be. That is a supported deployment rather than a
fault, so the endpoint answers `200` with `reachable: false` and the screen says
so, carries the transport error, and points at the offline bundle path
([`content-bundles.md`](content-bundles.md)) instead of showing an empty list.

Two ways through from there:

- **Install a release by label.** Only the index was out of reach; a named
  release is fetched directly, so a partial outage does not block an install.
- **Skip for now**, and import a bundle from Administration → Content sources.

## What "completed" means

It records a **decision, not an outcome**: that somebody reached the end of the
wizard. It does not mean content is installed. Tying it to installed content
would give an air-gapped deployment a screen it could never dismiss, and a
screen you cannot get rid of is one people learn to click through.

Completing is idempotent and one-way from the API. A second call keeps the first
timestamp and actor, so a client that lost the response can simply send it
again.

| | |
|---|---|
| Stored as | `setup.completed_at` in `app.platform_setting` (RFC 3339, absent until finished) |
| Read by | `GET /setup` (`settings.read`) |
| Written by | `POST /setup/complete` (`settings.manage`), `blctl setup complete` |
| Cleared by | `blctl setup reset` — and nothing else |
| Activity verb | `setup.completed` |
| Go package | `internal/setup` |
| Screen | `web/src/features/setup` |

## Provisioning and test harnesses

A provisioning run that creates the first administrator with `blctl user create`
and installs content with `blctl content sync` has already done everything the
wizard asks. `blctl setup complete` stops the browser opening on a screen with
nothing left to decide. The end-to-end suite seeds exactly that, in
`e2e/harness/auth.ts`, so a spec about some other screen does not start on the
wizard — see [`cli.md`](cli.md#blctl-setup-status--complete--reset).

There is no endpoint for `reset`, on purpose. Putting an installation back into
first-run state is an operator's decision made at the machine.
