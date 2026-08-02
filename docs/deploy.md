# Deploying Blacklight

One process, one database file, one directory of evidence. There is no application server to
configure, no separate database to run, and nothing is fetched at first boot.

The **container image is the supported artefact** (`PLAN.md` §8). Running the binary directly works
and is documented below, but the image is the configuration that gets tested — `deploy/smoke.sh`
runs against it on every CI build.

> **Status:** v2 is under construction. Everything on this page is real and tested, but there is no
> login, no content and no engagement to put in it yet — the server serves the API and the app
> shell. `docs/tickets/` is the accurate description of what exists.

---

## Quick start

```sh
git clone https://github.com/bryanster/blacklight
cd blacklight
docker compose up --build
```

Then open <http://localhost:8080>. The first build takes a few minutes; the first *boot* takes
seconds and makes no network request.

That works with no configuration at all, which is fine for a laptop and not fine for anything else.
Before anyone else can reach it, read [Configuration](#configuration) — at minimum you must set
`BLACKLIGHT_BASE_URL`, `BLACKLIGHT_SESSION_SECRET` and `BLACKLIGHT_ENCRYPTION_KEY`.

| Command | What it does |
|---|---|
| `docker compose up -d --build` | Build and start in the background |
| `docker compose ps` | Show the container and its health |
| `docker compose logs -f` | Follow the log |
| `docker compose down` | Stop and remove the container. **Data is kept** |
| `docker compose down -v` | Stop, remove, **and delete the data volume** |

> ⚠️ **`docker compose down -v` is irreversible.** The `-v` deletes the named volume, and with it the
> database, every uploaded evidence file and any generated keys. There is no undo and no prompt.
> Take a [backup](#backup-and-restore) first, or use `docker compose down` — which keeps everything
> — unless deleting the deployment is exactly what you meant.

---

## Configuration

Every setting is an environment variable. [`.env.example`](../.env.example) documents all of them
with their defaults and is the authoritative list; copy it and edit:

```sh
cp .env.example .env
```

Compose reads `.env` from the repository root if it exists and ignores it if it does not. Values in
it override the image's defaults.

A value the server cannot use is a **startup error naming the variable**, never a silent fallback.
If the container exits immediately, `docker compose logs` has a sentence telling you which variable
and why.

### The three that matter

**`BLACKLIGHT_BASE_URL`** — the absolute URL your users type. OIDC and SAML redirect URIs and report
share links are built from it and cannot be derived from a request without trusting a proxy header,
so it is configured rather than guessed. It must be `https://` unless the host is loopback.

```sh
BLACKLIGHT_BASE_URL=https://blacklight.example.com
```

**`BLACKLIGHT_SESSION_SECRET`** — keys the hash session tokens are stored under. At least 32 bytes of
real entropy; placeholders and low-entropy values are rejected at startup rather than accepted and
quietly useless. The cookie carries a random token and the database keeps only its keyed hash, so a
copy of the database is not a set of live sessions — and rotating this signs everybody out, because
no stored hash can be reproduced from the cookies people are holding.

```sh
BLACKLIGHT_SESSION_SECRET=$(openssl rand -base64 32)
```

If you do not set it, the container's entrypoint generates one on first boot and stores it at
`/var/lib/blacklight/session.secret` on the data volume, announcing it in the log. That is what makes
`docker compose up` work on a clean clone. It is a convenience, not a design:

- the secret exists only in that volume, so losing the volume logs everybody out permanently;
- it cannot be shared, so a second instance cannot validate the first one's sessions;
- nothing outside the container can read it back for a disaster-recovery runbook.

Set the variable yourself for anything you would be upset to lose. Rotating it — either variable or
file — logs everybody out, which is also how you revoke every session at once.

**`BLACKLIGHT_ENCRYPTION_KEY`** — encrypts what the server holds on somebody else's behalf, which
today means the TOTP shared secrets people enrol (`docs/security.md`, *Multi-factor
authentication*). Same shape as the session secret: at least 32 bytes of real entropy, generated the
same way, and it must **not** be the same value — the server refuses to start if the two match.

```sh
BLACKLIGHT_ENCRYPTION_KEY=$(openssl rand -base64 32)
```

They are separate because their consequences are, and this is the part worth reading twice:

| | Rotating `BLACKLIGHT_SESSION_SECRET` | Rotating or losing `BLACKLIGHT_ENCRYPTION_KEY` |
|---|---|---|
| What happens | everybody is signed out | every enrolled authenticator stops working |
| Is it recoverable? | yes — people sign in again | **no** — everyone must re-enrol from scratch |
| Would you notice? | immediately, and you meant to | only when codes start being rejected |

Rotating the session secret is a lever you are meant to pull. If the same value were also the
encryption key, pulling it would silently destroy every second factor in the deployment — which is
why sharing one value between them is a startup error rather than a note in this file.

The entrypoint generates this one on first boot too, at `/var/lib/blacklight/encryption.key`, with
the same caveats and one more: **back it up with the database.** A restored database whose
encryption key is gone is a deployment where nobody with MFA can sign in.

### Sessions

**`BLACKLIGHT_SESSION_LIFETIME`** (default `12h`) is how long a session may live at most, counted
from when it was issued; nothing extends it. **`BLACKLIGHT_SESSION_IDLE_TIMEOUT`** (default `2h`) is
how long one may go unused before it ends, and is the one that protects an unattended browser. The
idle timeout must not exceed the lifetime, which would leave it with nothing to do — that is a
startup error naming both variables.

Shortening either takes effect immediately for every existing session: both are evaluated on each
request against the row, not baked into the cookie.

### Sign-in throttling

Failed sign-ins are rationed two ways, and both have to allow an attempt through. A refused attempt
is a `429` carrying `Retry-After`, and it is the same answer whether or not the account exists.

| Variable | Default | Counts |
|---|---|---|
| `BLACKLIGHT_LOGIN_ACCOUNT_FAILURES` | `5` | consecutive failures against one email address |
| `BLACKLIGHT_LOGIN_ACCOUNT_LOCKOUT` | `15m` | how long that address is then locked |
| `BLACKLIGHT_LOGIN_SOURCE_FAILURES` | `50` | failures from one client address, across all accounts |
| `BLACKLIGHT_LOGIN_SOURCE_LOCKOUT` | `15m` | how long that client address is then locked |

Each further lockout of the same key doubles the wait, three times — 15m, 30m, 1h, 2h — and then
stops growing. A successful sign-in clears the account's count and its place on that ladder, as does
going quiet for the length of the longest lockout; a success deliberately does not clear the
source's, because an attacker holding one valid account would otherwise refill their budget with it.
Hammering a locked key does not extend the lockout.

Two consequences worth expecting:

- **The right password during a lockout is refused too.** Somebody who locks themselves out waits,
  or an administrator waits with them; there is no override yet (`M1-016`). A lockout that the
  correct password ended would only be a delay on an attacker who had already guessed it.
- **Behind a reverse proxy, the source limit counts the proxy** — every user shares one address —
  unless `BLACKLIGHT_TRUSTED_PROXIES` names it. Set that (see below) or raise
  `BLACKLIGHT_LOGIN_SOURCE_FAILURES` to suit the size of the office behind it.

Lockouts are logged at `WARN` with the scope, the key and the client address. The state is held in
memory by the server process: it is per-instance, and restarting clears it.

### The first account

A new deployment has no users and no sign-up. Create the first administrator with the admin CLI —
see [`docs/cli.md`](cli.md):

```sh
docker compose run --rm blacklight blctl user create \
    --email you@example.com --name "Your Name" --admin
```

DuckDB gives the database file to one process at a time, so the server has to be stopped for this
(`docker compose stop` first), or the command has to run in a container that shares nothing with it.
The error says so if you forget.

### The one that is gone: `PASSWORD_SALT`

v1 read a single site-wide salt from the environment and mixed it into every password. There is no
equivalent here and nothing to set: passwords are hashed with Argon2id, each with its own random
salt, and the salt and cost parameters are stored inside the hash. So there is no secret to lose, no
secret to rotate, and two people who pick the same password no longer get the same hash.

Local passwords must be at least 12 characters, may be up to 128, and are refused if they appear in
the list of passwords attackers try first. There are no composition rules — a passphrase of a few
words is both stronger and easier to type than `Passw0rd!`.

### What the image sets for you

These have image defaults that describe the container's own filesystem. Override them only if you
are changing the layout.

| Variable | Image default |
|---|---|
| `BLACKLIGHT_ADDR` | `:8080` |
| `BLACKLIGHT_DB_PATH` | `/var/lib/blacklight/blacklight.duckdb` |
| `BLACKLIGHT_EVIDENCE_DIR` | `/var/lib/blacklight/evidence` |
| `BLACKLIGHT_CHROME_PATH` | `/usr/bin/chromium` |
| `BLACKLIGHT_BASE_URL` | `http://localhost:8080` — correct for a laptop, wrong for everything else |

Changing `BLACKLIGHT_ADDR` to a different port also changes what the image's health check probes: it
reads the port out of that variable. Port `0` — "ask the kernel for a free port" — cannot be
health-checked, and the check says so rather than reporting a healthy container nobody is probing.

### Behind a reverse proxy

Terminate TLS at the proxy, forward to the container's port, and tell Blacklight which proxy to
believe:

```sh
BLACKLIGHT_BASE_URL=https://blacklight.example.com
BLACKLIGHT_TRUSTED_PROXIES=10.0.0.0/8
```

Without `BLACKLIGHT_TRUSTED_PROXIES` the client address is always the address the connection came
from — which is your proxy, so every request appears to come from one IP and rate limiting and the
activity log lose their meaning. With it set too widely (never `0.0.0.0/0`) anyone can choose the
address that gets throttled and logged by sending a header. Set it to the proxy, and nothing else.

---

## Where the data lives

Everything that must survive the container is under one directory, `/var/lib/blacklight`, which
compose mounts as the named volume `blacklight-data`:

| Path | What it is |
|---|---|
| `blacklight.duckdb` | The database. Everything except evidence blobs |
| `blacklight.duckdb.wal` | DuckDB's write-ahead log. Present while the process runs; part of the database, not a temporary file |
| `evidence/` | Uploaded evidence, content-addressed |
| `session.secret` | The generated session secret, if you did not supply one |

The database and `evidence/` reference each other and are not much use apart. Treat them as one
thing: back them up together, restore them together.

The directory is owned by **uid 10001, gid 10001** inside the image, and the container runs as that
user — it is never root. Docker seeds a new *named* volume from the image, ownership included, so
the compose setup works with no chown. A **bind mount** does not work that way: the host directory
keeps its own ownership, so if you replace the named volume with `-v /srv/blacklight:/var/lib/blacklight`
you must `chown -R 10001:10001 /srv/blacklight` first, or startup fails with "is not writable by this
process".

---

## Administering it: `blctl`

The image carries a second binary, `blctl`, on `PATH`. It is how you migrate a database by hand,
look inside one, create users, and — as the milestones land — sync content and take backups. [`docs/cli.md`](cli.md) is the reference; the short version:

```sh
docker compose exec blacklight blctl version      # is the CLI the same build as the server?
docker compose stop                                # DuckDB admits one process at a time
docker compose run --rm blacklight blctl db info  # …so a command needs the server down
docker compose start
```

`blctl` reads the same environment and the same `BLACKLIGHT_DB_PATH` as the server, so inside the
image it already points at the right file. Anything that opens the database needs the server
stopped, `docker compose exec` included: DuckDB admits one process per file, and a command that
tries anyway fails with a message saying so rather than corrupting anything. `blctl version` is
the exception, because it opens nothing.

---

## Backup and restore

DuckDB writes through a WAL, so copying the files out from under a running process can capture a
torn state. **Stop the container first.** A backup that has never been restored is a hypothesis.

### Back up

```sh
docker compose stop
docker run --rm \
  -v blacklight_blacklight-data:/data:ro \
  -v "$PWD:/backup" \
  debian:trixie-slim \
  tar czf /backup/blacklight-$(date -u +%Y%m%d).tar.gz -C /data .
docker compose start
```

The volume is called `blacklight_blacklight-data`: compose prefixes the volume name with the project
name. `docker volume ls` confirms it.

Keep the archive somewhere the deployment is not. It contains the session secret, the encryption
key and every piece of evidence anyone has uploaded — treat it as classified as the engagements it
describes.

If you supplied `BLACKLIGHT_SESSION_SECRET` and `BLACKLIGHT_ENCRYPTION_KEY` yourself, they are in your
environment and not in this archive: back them up wherever you keep the rest of your secrets. A
restored database without its encryption key is one where nobody with an authenticator enrolled can
sign in, and there is no way back from that except an administrator resetting each of them.

### Restore

Into an empty volume, over a stopped deployment:

```sh
docker compose down                      # not -v: we are about to overwrite, not delete
docker run --rm \
  -v blacklight_blacklight-data:/data \
  -v "$PWD:/backup" \
  debian:trixie-slim \
  sh -c 'rm -rf /data/* /data/.[!.]* && tar xzf /backup/blacklight-20260801.tar.gz -C /data'
docker compose up -d
```

Restore into the same or a newer version of Blacklight, never an older one: migrations run forward on
startup and are append-only, so a newer database in an older binary is a schema the binary does not
understand.

### Upgrading

```sh
git pull
docker compose up -d --build
```

Migrations are applied at startup and logged one line each. Back up first; the reason the paragraph
above exists is that rolling *back* is the case that is not supported.

---

## Chromium and PDF rendering

The image carries Chromium so that M6's PDF reports work without a second container or a runtime
download. `BLACKLIGHT_CHROME_PATH` already points at it. Nothing renders PDFs yet — M6 builds that —
but the packaging is in place and tested now, on purpose, rather than discovered at the end.

**The sandbox is left on.** Chromium's renderer sandbox needs to create a user namespace, and
Docker's *default* seccomp profile blocks that, so on a stock Docker host the sandbox cannot start
and Chromium aborts. Three ways out, worst last:

1. **A host that allows unprivileged user namespaces through its seccomp policy.** Podman, and
   Docker with a profile that permits `clone`/`unshare` with `CLONE_NEWUSER`. Nothing to change
   here; this is why the image does not disable the sandbox for you.
2. **`security_opt: ["seccomp=unconfined"]` on the service.** Verified working. Note what it
   actually does: it turns off syscall filtering for the *whole container* to protect one process.
   That is a real trade, not a free one.
3. **`cap_add: ["SYS_ADMIN"]`.** Also works, also worse than it sounds — it is most of root.

If none of those are acceptable, M6 will need `--no-sandbox`, which gives up Chromium's own
isolation between the renderer and the rest of the container. The renderer only ever parses HTML
this deployment generated, so the exposure is bounded, but it is the option to reach for last.

`docker compose` does not set any of these. Two settings it does set, both for Chromium: `init: true`,
because Chromium leaves zombie children and PID 1 in a container reaps nothing by default; and
`shm_size: 512mb`, because the 64 MB default starves Chromium's renderers and they crash mid-render.

### Hardening that conflicts with it

`no-new-privileges`, `cap_drop: [ALL]` and a read-only root filesystem are all things you would
otherwise want, and all three break Chromium's sandbox in a container — `no-new-privileges` blocks
the setuid helper, dropping capabilities stops it doing anything once it runs, and Chromium needs a
writable `/tmp`. If this deployment will never render a PDF, they are safe to add and worth adding.
Otherwise, pick one side deliberately.

---

## Health checks

The image defines its own `HEALTHCHECK`, so `docker run`, compose and any orchestrator reading the
image config all get it — there is nothing to copy into your own compose file.

It is `GET /api/v1/healthz` from inside the container. The endpoint is public and unauthenticated by
design: a health check that needs a session reports "unhealthy" exactly when authentication breaks,
which is the one moment you most need the truth. It answers `200` when the database responds and
`503` when it does not, with the same body either way — so a monitor that only reads the status code
gets the right answer and one that reads the body finds out which dependency is down:

```json
{"status": "ok", "checks": {"db": "ok"}}
```

Failures during the first 30 seconds do not count against it — that window covers migrations on a
cold database.

To probe from outside the container, point your monitoring at `/api/v1/healthz` on the published
port. It is the same endpoint.

---

## Running without Docker

Supported, less tested. You need Go 1.25+, Node 24 (see `.prototools`), a C compiler — the DuckDB
driver is cgo — and, for M6, a Chromium binary.

```sh
make tools                              # once: pinned generators into ./bin
make build                              # SPA + binaries into ./bin
cp .env.example .env                    # then edit it
set -a && . ./.env && set +a
./bin/blacklight
```

Notes that bite people:

- **`make build`, not `go build`.** The frontend is embedded behind the `spa` build tag. A plain
  `go build` produces a working server that serves a placeholder page explaining that it was built
  wrong, and every other check still passes.
- **The parent directory of `BLACKLIGHT_DB_PATH` must already exist.** DuckDB creates the file;
  nothing creates the directory. `BLACKLIGHT_EVIDENCE_DIR` *is* created at startup.
- **Run it as a non-root service account** with a private data directory, under systemd or
  equivalent. `SIGTERM` starts a graceful shutdown and in-flight requests get
  `BLACKLIGHT_SHUTDOWN_TIMEOUT` to finish, so `Restart=on-failure` and a normal `systemctl restart`
  behave.
- **There is no static asset directory to deploy.** The binary is the whole application.
- **`make build` also produces `./bin/blctl`**, the admin CLI ([`docs/cli.md`](cli.md)). Install it
  beside the server and give it the same environment; it needs the service stopped, because DuckDB
  gives the database file to one process at a time.

---

## Building the image

```sh
make docker-build                       # blacklight:local, version stamped from git
make docker-smoke                       # build it, run it, and check every claim on this page
IMAGE=ghcr.io/bryanster/blacklight IMAGE_TAG=v2.0.0 make docker-build
```

The build context is the repository root and the Dockerfile is
[`deploy/Dockerfile`](../deploy/Dockerfile), so a bare `docker build` needs `-f`:

```sh
docker build -f deploy/Dockerfile -t blacklight:local .
```

### Two architectures

```sh
docker buildx build --platform linux/amd64,linux/arm64 -f deploy/Dockerfile -t blacklight:local .
```

`linux/amd64` and `linux/arm64` both work, and both build at roughly native speed. Neither build
stage runs under emulation: the frontend stage is pinned to the *build* platform because JavaScript
is the same bytes everywhere, and the Go stage is pinned there too and cross-compiles, installing a
`g++` cross toolchain when the architectures differ — cgo means the C compiler has to target the
other architecture as well. Emulating the Go stage instead is not merely slow; the toolchain
segfaults part-way through under QEMU.

You can smoke-test the other architecture on your own machine, at emulated speed:

```sh
PLATFORM=linux/amd64 deploy/smoke.sh
```

Expect exactly one failure there, `chromium --version`: `qemu-user` does not advertise SSE3 and
Chromium refuses to start without it. That is the emulator, not the image.

### Why Debian and not Alpine

The DuckDB driver is cgo, so there is no static binary and no `FROM scratch`. Alpine would mean
maintaining a musl build of DuckDB, which is not worth it for the size. Both build stages and the
runtime use the same Debian release, so the glibc the binary was linked against is the glibc it
finds at runtime.

### Size

Roughly **640 MB unpacked** on the host, **260 MB compressed** to pull (linux/arm64; amd64 is within
a few percent). Where it goes:

| | Unpacked |
|---|---|
| Chromium and its dependencies | ~345 MB |
| Debian trixie-slim base | ~109 MB |
| The `blacklight` binary | ~64 MB — mostly the statically linked DuckDB engine |
| Fonts, CA certificates, curl, tzdata | ~120 MB |

That is over the 500 MB the ticket aimed at, and the overshoot is Chromium: the Debian package is
~345 MB unpacked once trimmed, and there is no smaller glibc-compatible headless build to swap in.
The build already deletes ~230 MB of it — the Mesa hardware GL stack (LLVM, Gallium, Z3, the DRI
drivers), the Vulkan validation layer and the GTK icon themes — none of which a headless renderer in
a container without a GPU can use; Chromium falls back to the bundled SwiftShader software renderer,
and `--print-to-pdf` produces byte-identical output with them gone. Dropping Chromium entirely would
land the image at ~290 MB, and is the shape of a future `blacklight:slim` if the size ever matters
more than PDF reports working out of the box.

---

## Troubleshooting

**The container exits immediately.** `docker compose logs` — configuration errors are one plain
sentence naming the variable, printed before the log format is even applied.

**"must use https when BLACKLIGHT_ENV=production".** Session cookies are `Secure`, and browsers do
not send those over plain HTTP, so a production deployment on `http://` cannot log anyone in. Use
`https://`, or a loopback host, which browsers treat as a secure context.

**"is not writable by this process".** A bind mount that is not owned by uid 10001 — see
[Where the data lives](#where-the-data-lives).

**The UI is a page saying the interface was not built.** That binary was compiled without
`-tags spa`. The image always sets it; a hand-rolled `go build` does not.

**`docker compose up` says the port is in use.** Publish a different one and keep the base URL in
step:

```sh
BLACKLIGHT_HOST_PORT=9090
BLACKLIGHT_BASE_URL=http://localhost:9090
```

**Chromium aborts with "Failed to move to new namespace".** The sandbox — see
[Chromium and PDF rendering](#chromium-and-pdf-rendering).
