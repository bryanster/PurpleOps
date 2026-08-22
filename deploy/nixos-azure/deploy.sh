#!/usr/bin/env bash
#
# Deploy the NixOS (native, no-container) example to Azure with the Azure CLI.
#
#     cd deploy/nixos-azure && ./deploy.sh
#
# What it does, in order:
#   1. builds the Gen 1 VHD (`nix build .#azure-vhd`, unless SKIP_BUILD is set)
#   2. creates a resource group, a static public IP and an NSG (SSH + 80/443)
#   3. uploads the VHD to a page blob and turns it into a managed image
#   4. creates the VM from that image, with the static IP and the NSG
#   5. prints the public IP and the next steps
#
# Prerequisites: `az login`, and Nix with flakes enabled. The `domain` baked
# into configuration.nix must have an A record pointing at the IP this prints —
# Caddy needs it for the Let's Encrypt certificate.
#
# For a fresh deployment. Re-running fails on the resources that already exist.
#
# Environment:
#   LOCATION     Azure region                        (default eastus)
#   RG           resource group name                 (default blacklight-nixos-rg)
#   VM_NAME      VM name                             (default blacklight)
#   IMAGE_NAME   managed image name                  (default blacklight-nixos-image)
#   PIP_NAME     public IP name                      (default blacklight-pip)
#   NSG_NAME     network security group name         (default blacklight-nsg)
#   ADMIN_USER   admin account, must match config    (default azureuser)
#   SSH_KEY      path to the public key to install   (default ~/.ssh/id_ed25519.pub)
#   VM_SIZE      VM size                             (default Standard_B2s)
#   OS_DISK_GB   OS disk size                        (default 32)
#   SA_NAME      storage account name                (default: generated)
#   SKIP_BUILD   non-empty to use an existing ./result VHD

set -euo pipefail

HERE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

LOCATION="${LOCATION:-eastus}"
RG="${RG:-blacklight-nixos-rg}"
VM_NAME="${VM_NAME:-blacklight}"
IMAGE_NAME="${IMAGE_NAME:-blacklight-nixos-image}"
PIP_NAME="${PIP_NAME:-blacklight-pip}"
NSG_NAME="${NSG_NAME:-blacklight-nsg}"
ADMIN_USER="${ADMIN_USER:-azureuser}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519.pub}"
VM_SIZE="${VM_SIZE:-Standard_B2s}"
OS_DISK_GB="${OS_DISK_GB:-32}"
SA_NAME="${SA_NAME:-blacklightvhd$RANDOM$RANDOM}"
SKIP_BUILD="${SKIP_BUILD:-}"

# `~` at the front of a variable is not expanded by the shell or by az.
SSH_KEY="${SSH_KEY/#\~/$HOME}"

say() { printf '\n==> %s\n' "$*"; }

# ── Prerequisites ────────────────────────────────────────────────────────────
command -v az >/dev/null || { echo "az CLI not found — install it and run 'az login'" >&2; exit 1; }
az account show >/dev/null || { echo "not logged in — run 'az login'" >&2; exit 1; }
[ -f "$SSH_KEY" ] || { echo "ssh public key not found at $SSH_KEY" >&2; exit 1; }

# ── 1. Build the VHD ────────────────────────────────────────────────────────
if [ -n "$SKIP_BUILD" ]; then
	say "skipping build (SKIP_BUILD set), using $HERE/result"
else
	say "building the VHD (nix build .#azure-vhd)"
	( cd "$HERE" && nix build .#azure-vhd )
fi
VHD="$HERE/result"
[ -e "$VHD" ] || { echo "VHD not found at $VHD — build it with 'nix build .#azure-vhd' first" >&2; exit 1; }

# ── 2. Resource group, public IP, NSG ───────────────────────────────────────
say "creating resource group $RG in $LOCATION"
az group create --name "$RG" --location "$LOCATION" --output none

say "creating static public IP $PIP_NAME"
az network public-ip create --name "$PIP_NAME" --resource-group "$RG" \
	--location "$LOCATION" --sku Standard --allocation-method Static --output none

say "creating NSG $NSG_NAME (SSH, 80, 443)"
az network nsg create --name "$NSG_NAME" --resource-group "$RG" --location "$LOCATION" --output none
az network nsg rule create --resource-group "$RG" --nsg-name "$NSG_NAME" \
	--name ssh --priority 100 --direction Inbound --access Allow \
	--protocol Tcp --destination-port-ranges 22 --output none
az network nsg rule create --resource-group "$RG" --nsg-name "$NSG_NAME" \
	--name web --priority 110 --direction Inbound --access Allow \
	--protocol Tcp --destination-port-ranges 80 443 --output none

# ── 3. Upload the VHD and make an image ─────────────────────────────────────
say "creating storage account $SA_NAME"
az storage account create --name "$SA_NAME" --resource-group "$RG" --location "$LOCATION" \
	--sku Standard_LRS --kind StorageV2 --output none

say "uploading the VHD as a page blob"
az storage container create --name vhds --account-name "$SA_NAME" --output none
az storage blob upload --account-name "$SA_NAME" --container-name vhds \
	--name blacklight.vhd --type page --file "$VHD" --no-progress

say "creating image $IMAGE_NAME"
az image create --name "$IMAGE_NAME" --resource-group "$RG" \
	--source "https://$SA_NAME.blob.core.windows.net/vhds/blacklight.vhd" \
	--os-type Linux --hyper-v-generation V1 --output none

# ── 4. Create the VM ────────────────────────────────────────────────────────
say "creating VM $VM_NAME (this takes a few minutes)"
az vm create --name "$VM_NAME" --resource-group "$RG" --location "$LOCATION" \
	--image "$IMAGE_NAME" --size "$VM_SIZE" --os-disk-size-gb "$OS_DISK_GB" \
	--admin-username "$ADMIN_USER" --ssh-key-values "$SSH_KEY" \
	--nsg "$NSG_NAME" --public-ip-address "$PIP_NAME" \
	--security-type Standard --output none

# ── 5. Summary ──────────────────────────────────────────────────────────────
IP="$(az network public-ip show --name "$PIP_NAME" --resource-group "$RG" --query ipAddress --output tsv)"

say "deployed"
printf 'public IP:   %s\n' "$IP"
printf 'ssh:         ssh %s@%s\n' "$ADMIN_USER" "$IP"
printf 'domain:      point the A record for the `domain` in configuration.nix at %s\n' "$IP"
printf '             then open https://<domain>\n'
printf 'first admin: see README.md §7 — stop the service, run blctl, start it again\n'
