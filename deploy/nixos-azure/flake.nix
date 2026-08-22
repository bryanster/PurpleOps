# Build Blacklight natively (no container) and a NixOS machine image that runs
# it directly as a systemd service on Azure.
#
# The `azure` image variant is the part of nixos-generators that has been
# upstreamed into nixpkgs (NixOS 25.05+): `nixos-rebuild build-image
# --image-variant azure`.
{
  description = "Blacklight on NixOS, built and run natively (no container)";

  # nixos-26.05 — the current stable release. The cloud image builders,
  # including `azure`, moved from nixos-generators into nixpkgs here, so no
  # separate generator input is needed.
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  };

  outputs =
    { self, nixpkgs }:
    let
      # The repository, git-tracked files only: web/node_modules, bin/,
      # web/dist, evidence/ and test artifacts are gitignored, so this is a
      # clean source tree.
      src = ../..;

      lib = nixpkgs.lib;

      # The server + admin CLI, one derivation per system. aarch64-linux lets a
      # developer build and run the binary on an ARM machine; x86_64-linux is
      # what the Azure VM and CI build.
      mkBlacklight =
        system:
        let
          pkg = nixpkgs.legacyPackages.${system}.callPackage ./blacklight.nix { inherit src; };
        in
        {
          blacklight = pkg;
          default = pkg;
        };
    in
    {
      packages = {
        x86_64-linux =
          (mkBlacklight "x86_64-linux")
          // {
            # `nix build .#azure-vhd` -> ./result, a symlink to the fixed-size
            # Gen 1 VHD Azure expects. Equivalent to:
            #   nixos-rebuild build-image --image-variant azure --flake .#blacklight
            azure-vhd =
              let
                image = self.nixosConfigurations.blacklight.config.system.build.images.azure;
              in
              nixpkgs.legacyPackages.x86_64-linux.runCommand "blacklight-azure.vhd" { } ''
                ln -s "${image}/${image.passthru.filePath}" "$out"
              '';
          };
        aarch64-linux = mkBlacklight "aarch64-linux";
      };

      nixosConfigurations.blacklight = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = { inherit src; };
        modules = [ ./configuration.nix ];
      };
    };
}
