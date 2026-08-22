# NixOS on Azure — native

A worked example that builds Blacklight **natively** (no container) and deploys it to an Azure VM
running it directly as a systemd service. One VM, one Go binary, the database on a local disk.

Contrast with [`deploy/azure-terraform`](../azure-terraform): that one is serverless (Azure Container
Apps) and runs the published container image with the database on an Azure Files share. This one
compiles the server and SPA with Nix, installs the binary on the machine, and gives DuckDB a local
filesystem — its preferred habitat — plus a machine you can SSH into.

**Quick start:** [`deploy.sh`](./deploy.sh) builds the VHD and drives every `az` step below in one
command — resource group, public IP, NSG, upload, image, VM — with each name overridable through
environment variables (see its header). The numbered sections are the manual walkthrough that
script automates.

## What the image contains

| Piece | How |
|---|---|
| Blacklight | The server + admin CLI, built by `blacklight.nix` (git-tracked source, SPA embedded) and installed natively |
| Data | `/var/lib/blacklight` on the VM's OS disk, owned by a fixed-uid `blacklight` user (10001) |
| Chromium | `pkgs.chromium`, pointed at by `BLACKLIGHT_CHROME_PATH` for PDF rendering (headless `--no-sandbox`, already set by the renderer) |
| TLS | Caddy reverse-proxies 80/443 to the server's loopback 8080, with an automatic Let's Encrypt certificate |
| Secrets | Generated on first boot by a systemd oneshot, persisted beside the database |
| Azure glue | waagent, cloud-init, OpenSSH — from nixpkgs' `azure` image profile, no extra config |
| Admin access | A declared `azureuser` account; Azure injects your SSH key at deploy time |

## Prerequisites

- [Nix](https://nixos.org/download) with flakes enabled, on an x86_64-linux machine (or one with a
  remote x86_64-linux builder — building the disk image runs a VM under QEMU).
- [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli), signed in: `az login`.
- A domain you control, to point at the VM for the TLS certificate.
- An SSH key pair.

## 0. Try the binary locally (optional, fast)

You can build and run the server on the machine in front of you, without Azure, to prove the native
build:

```sh
cd deploy/nixos-azure
nix build .#blacklight

# a one-liner smoke run against a throwaway database
tmp=$(mktemp -d) && \
  BLACKLIGHT_SESSION_SECRET=$(head -c 32 /dev/urandom | base64 -w 0) \
  BLACKLIGHT_ENCRYPTION_KEY=$(head -c 32 /dev/urandom | base64 -w 0) \
  BLACKLIGHT_BASE_URL=http://127.0.0.1:8080 \
  BLACKLIGHT_ADDR=127.0.0.1:8080 \
  BLACKLIGHT_DB_PATH=$tmp/blacklight.duckdb \
  BLACKLIGHT_EVIDENCE_DIR=$tmp/evidence \
  ./result/bin/blacklight
```

`curl http://127.0.0.1:8080/api/v1/healthz` should answer `{"checks":{"db":"ok"},"status":"ok"}`.
`./result/bin/` holds both `blacklight` and `blctl`. This works on ARM and x86_64 alike.

## 1. Set the domain

Edit `configuration.nix` and set `domain` to the public name users will type. It becomes both
`BLACKLIGHT_BASE_URL` and the Caddy virtual host, so it must be a name whose A record you can point
at the VM.

## 2. Build the VHD

```sh
cd deploy/nixos-azure
nix build .#azure-vhd
```

`./result` is a symlink to a fixed-size **Gen 1** VHD. (Equivalent:
`nixos-rebuild build-image --image-variant azure --flake .#blacklight`.)

The first build fetches and pins nixpkgs; commit the generated `flake.lock` for reproducibility.

## 3. Reserve an IP and point DNS

```sh
loc=eastus
rg=blacklight-nixos-rg

az group create --name $rg --location $loc

az network public-ip create --name blacklight-pip --resource-group $rg \
  --location $loc --sku Standard --allocation-method Static

az network public-ip show --name blacklight-pip --resource-group $rg \
  --query ipAddress --output tsv   # point your domain's A record here
```

Point the domain's A record at that IP now. DNS can take a while to propagate; the VM and the
certificate can wait for it.

## 4. Upload the VHD and make an image

```sh
sa=blacklightvhd$RANDOM
az storage account create --name $sa --resource-group $rg --location $loc \
  --sku Standard_LRS --kind StorageV2
az storage container create --name vhds --account-name $sa

# A VHD must land as a page blob, not a block blob.
az storage blob upload --account-name $sa --container-name vhds \
  --name blacklight.vhd --type page --file ./result

az image create --name blacklight-nixos-image --resource-group $rg \
  --source "https://$sa.blob.core.windows.net/vhds/blacklight.vhd" \
  --os-type Linux --hyper-v-generation V1
```

For a large upload, `azcopy copy ./result "<blob-url>?<SAS>" --blob-type PageBlob` is faster; either
way the blob must be a page blob.

## 5. Create the VM

```sh
# NSG with SSH plus the two ports Caddy needs.
az network nsg create --name blacklight-nsg --resource-group $rg --location $loc
az network nsg rule create --resource-group $rg --nsg-name blacklight-nsg \
  --name ssh --priority 100 --direction Inbound --access Allow \
  --protocol Tcp --destination-port-ranges 22
az network nsg rule create --resource-group $rg --nsg-name blacklight-nsg \
  --name web --priority 110 --direction Inbound --access Allow \
  --protocol Tcp --destination-port-ranges 80 443

az vm create --name blacklight --resource-group $rg --location $loc \
  --image blacklight-nixos-image --size Standard_B2s --os-disk-size-gb 32 \
  --admin-username azureuser --ssh-key-values ~/.ssh/id_ed25519.pub \
  --nsg blacklight-nsg --public-ip-address blacklight-pip \
  --security-type Standard
```

Two things are load-bearing:

- `--admin-username azureuser` must match the user declared in `configuration.nix` — Azure injects
  the SSH key into that account at first boot, and it must already exist in the image's
  `/etc/passwd`.
- Keep `--security-type Standard`. The image boots with BIOS/GRUB (Gen 1), so Trusted Launch /
  Secure Boot would stop it from booting. `--size` is your choice; `Standard_B2s` is a small
  burstable, SCSI-attached SKU that needs no disk-controller fuss.

## 6. Open it

Once the A record resolves, open `https://<domain>`. Caddy requests the Let's Encrypt certificate on
first request; give it a few seconds. If it fails:

```sh
ssh azureuser@<ip> sudo journalctl -u caddy -e
```

## 7. Create the first administrator

A fresh deployment has no users and no sign-up. You have a shell on this machine, so `blctl` is the
cleaner route ([`docs/cli.md`](../../docs/cli.md)):

```sh
IP=$(az network public-ip show --name blacklight-pip --resource-group $rg \
  --query ipAddress --output tsv)

# DuckDB gives the database to one process at a time, so stop the server first.
ssh azureuser@$IP sudo systemctl stop blacklight

# blctl reads the password from stdin once when stdin is not a terminal
# (see docs/cli.md); run it interactively and it prompts instead.
printf '%s' "$PASSWORD" | ssh azureuser@$IP sudo -u blacklight env \
  BLACKLIGHT_DB_PATH=/var/lib/blacklight/blacklight.duckdb \
  BLACKLIGHT_EVIDENCE_DIR=/var/lib/blacklight/evidence \
  blctl user create --email you@example.com --name "Your Name" --admin

ssh azureuser@$IP sudo systemctl start blacklight
```

Then sign in and change the password.

(Where you would rather not shell into the machine, the server can bootstrap the first admin from its
own configuration — `BLACKLIGHT_BOOTSTRAP_ADMIN_*`, see `docs/deploy.md` "The first account".)

## Operations

- **Upgrade.** Edit `configuration.nix` (or pull the repo) and run `nixos-rebuild switch --flake
  .#blacklight` over SSH. Stop the server and back up first. To change the pinned nixpkgs or the
  image tag, update `flake.nix` and `nix flake update`.
- **Backup.** `systemctl stop blacklight`, then run `blctl backup` against the same data directory —
  plus the two generated key files in `/var/lib/blacklight` (`session.secret`, `encryption.key`). See
  `docs/deploy.md`.
- **Logs.** `journalctl -u blacklight` (the app) and `journalctl -u caddy` (TLS).
- **Rebuild the image.** `nix build .#azure-vhd` produces a fresh VHD to swap the VM's disk with.

## Teardown

```sh
az group delete --name blacklight-nixos-rg --yes
```

Deletes the VM, disk, image, storage account and the public IP. The data goes with the disk.
