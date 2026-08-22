# Blacklight, running natively on a single NixOS VM (Azure) — no container.
#
# The `azure` image variant (flake.nix) pulls in nixpkgs' azure-common profile,
# which already sets up the Azure Linux agent (services.waagent), cloud-init for
# first-boot networking, an OpenSSH daemon, the Hyper-V kernel modules and a
# serial console on ttyS0. What remains to declare here is the Blacklight
# systemd service, the TLS reverse proxy in front of it, and the account you
# SSH in as. Edit `domain` before building; everything else is optional.
#
# The server and CLI are built by blacklight.nix (git-tracked source, SPA
# embedded) and installed straight onto the machine — the binary is the
# service, no podman, no image pull at boot.

{ config, pkgs, lib, src, ... }:

let
  # The public name users type. Must be a domain you control, with an A record
  # pointing at the VM's public IP — Caddy requests a Let's Encrypt certificate
  # for it, and BLACKLIGHT_BASE_URL is built from it.
  domain = "blacklight.example.com";

  # bin/blacklight and bin/blctl, built for this machine's architecture.
  blacklight = pkgs.callPackage ./blacklight.nix { inherit src; };
in
{
  # The admin account. Azure injects the SSH key you pass to
  # `az vm create --ssh-key-values` into this account's ~/.ssh/authorized_keys
  # at first boot, so the name here must match `--admin-username`. Declaring
  # the user (rather than letting waagent `useradd` it) is required: NixOS
  # manages /etc/passwd declaratively, and an imperative useradd on first boot
  # is exactly what fails silently on a NixOS image. There is deliberately no
  # openssh.authorizedKeys.keys here — the credential comes from Azure, not
  # from the image.
  users.users.azureuser = {
    isNormalUser = true;
    extraGroups = [ "wheel" ];
  };

  # The admin account has no password (key-only login), so passwordless sudo
  # is how it runs admin commands like `systemctl` and `blctl`.
  security.sudo.extraRules = [
    {
      users = [ "azureuser" ];
      commands = [ { command = "ALL"; options = [ "NOPASSWD" ]; } ];
    }
  ];

  # The service account the server runs as, with a fixed uid/gid matching the
  # container image (deploy/Dockerfile): a stable number makes backups and
  # bind-mounted data behave the same across the two deployment paths.
  users.users.blacklight = {
    isSystemUser = true;
    uid = 10001;
    group = "blacklight";
    # No createHome: the data directory is created and owned by the tmpfiles
    # rules below, with the 0750 mode the container image uses.
    home = "/var/lib/blacklight";
  };
  users.groups.blacklight = { gid = 10001; };

  # The data directory — the DuckDB database, its WAL, the generated session
  # secret and encryption key, and uploaded evidence — on the VM's own disk,
  # which is where DuckDB wants to be.
  systemd.tmpfiles.rules = [
    "d /var/lib/blacklight 0750 blacklight blacklight - -"
    "d /var/lib/blacklight/evidence 0750 blacklight blacklight - -"
  ];

  # ── Secrets ───────────────────────────────────────────────────────────────
  #
  # No entrypoint here, so this oneshot does the entrypoint's one job
  # (deploy/entrypoint.sh): generate the two required keys on first boot and
  # persist them beside the database, reusing them on every later start. The
  # server reads them from the environment; the files below are written as
  # EnvironmentFile entries (KEY=value) and loaded by the service.
  systemd.services.blacklight-init-secrets = {
    description = "Generate Blacklight session secret and encryption key";
    wantedBy = [ "multi-user.target" ];
    before = [ "blacklight.service" ];
    path = [ pkgs.coreutils ];
    serviceConfig = {
      Type = "oneshot";
      User = "blacklight";
      Group = "blacklight";
      RemainAfterExit = true;
    };
    script = ''
      umask 077
      if [ ! -s /var/lib/blacklight/session.secret ]; then
        printf 'BLACKLIGHT_SESSION_SECRET=%s\n' \
          "$(head -c 32 /dev/urandom | base64 -w 0)" \
          > /var/lib/blacklight/session.secret
      fi
      if [ ! -s /var/lib/blacklight/encryption.key ]; then
        printf 'BLACKLIGHT_ENCRYPTION_KEY=%s\n' \
          "$(head -c 32 /dev/urandom | base64 -w 0)" \
          > /var/lib/blacklight/encryption.key
      fi
    '';
  };

  # ── The server ────────────────────────────────────────────────────────────
  systemd.services.blacklight = {
    description = "Blacklight server";
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" "blacklight-init-secrets.service" ];
    requires = [ "blacklight-init-secrets.service" ];
    environment = {
      # Bound to loopback: the reverse proxy is the only way in.
      BLACKLIGHT_ADDR = "127.0.0.1:8080";
      BLACKLIGHT_BASE_URL = "https://${domain}";
      BLACKLIGHT_DB_PATH = "/var/lib/blacklight/blacklight.duckdb";
      BLACKLIGHT_EVIDENCE_DIR = "/var/lib/blacklight/evidence";
      # The PDF renderer launches Chromium headless with --no-sandbox
      # (internal/report/pdf), so no SUID sandbox setup is required.
      BLACKLIGHT_CHROME_PATH = "${pkgs.chromium}/bin/chromium";
    };
    serviceConfig = {
      User = "blacklight";
      Group = "blacklight";
      # "-" prefix: tolerate a missing file (should not happen — the oneshot
      # above creates them first — but do not hard-fail the service over it).
      EnvironmentFile = [
        "-/var/lib/blacklight/session.secret"
        "-/var/lib/blacklight/encryption.key"
      ];
      ExecStart = "${blacklight}/bin/blacklight";
      Restart = "on-failure";
      RestartSec = "5s";
    };
  };

  # blctl on the PATH, for creating the first administrator and running
  # migrations/backups against the same data directory.
  environment.systemPackages = [ blacklight ];

  # ── TLS in front ──────────────────────────────────────────────────────────
  #
  # Caddy terminates HTTPS (automatic Let's Encrypt) and proxies to the
  # server. Ports 80/443 must be reachable: 80 for the ACME HTTP-01 challenge,
  # 443 for clients.
  services.caddy = {
    enable = true;
    virtualHosts."${domain}".extraConfig = ''
      reverse_proxy 127.0.0.1:8080
    '';
  };

  networking.firewall.allowedTCPPorts = [ 80 443 ];

  system.stateVersion = "26.05"; # do not change after first deployment
}
