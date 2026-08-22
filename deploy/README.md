# deploy/

Deployment artefacts. The container image is the supported way to run Blacklight (`PLAN.md` §8);
[`docs/deploy.md`](../docs/deploy.md) is the operator-facing guide and explains all of this in
context. This file is the map.

| File | What it is |
|---|---|
| `Dockerfile` | Three stages — SPA, Go binary, Debian runtime with Chromium. Built from the **repository root**: `docker build -f deploy/Dockerfile .`, or `make docker-build` |
| `entrypoint.sh` | Generates and persists a session secret when the operator did not supply one, then `exec`s the server. Nothing else |
| `healthcheck.sh` | The image's `HEALTHCHECK` command: `GET /api/v1/healthz` on the configured port |
| `smoke.sh` | Builds the image, runs it, and asserts every claim `docs/deploy.md` makes. `make docker-smoke`; CI runs the same script |
| `terraform/` | Azure Container Apps example — app, Azure Files data volume, Key Vault secrets. See `deploy/terraform/README.md` |
| `nixos-azure/` | NixOS VM on Azure — builds a Gen 1 VHD and runs the server natively (no container) as a systemd service behind Caddy, data on the VM's own disk. See `deploy/nixos-azure/README.md` |

[`compose.yml`](../compose.yml) is at the repository root rather than here, so that
`docker compose up` works in a clean clone with no `-f`.

Added in M0B-011.
