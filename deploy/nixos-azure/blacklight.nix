# Build the Blacklight server and admin CLI natively, with the SPA embedded.
#
# callPackage-style: the flake and the NixOS module both pass `src` (the
# git-tracked repository) and get one derivation whose `bin/` holds both
# binaries. No container: the same two commands `make build` runs, done the
# Nix way.
#
# The DuckDB driver is cgo and links a prebuilt C++ static archive
# (duckdb-go-bindings ships lib/linux-{amd64,arm64}), so CGO_ENABLED must be
# on and libstdc++ must be reachable at link time. The SPA is built with npm
# and injected into web/dist so //go:embed all:dist (web/dist_spa.go) resolves.
#
# The version/commit/buildDate arguments are prefixed `bl` because callPackage
# would otherwise fill `version`/`commit`/`buildDate` from pkgs attrs of the
# same name (nixpkgs has a `commit` package), stamping a store path.
{
  lib,
  stdenv,
  buildGoModule,
  buildNpmPackage,
  runCommand,
  src,
  blVersion ? "v1-dev",
  blCommit ? "unknown",
  blBuildDate ? "unknown",
}:
let
  # Stage 1 · the single-page app. `npm run build` type-checks and bundles
  # into web/dist (the Dockerfile's "web" stage, in nix).
  web = buildNpmPackage {
    pname = "blacklight-web";
    version = "0";
    src = "${src}/web";
    npmDepsHash = "sha256-+JEwokWUpbXSuIr+GCHtjCV9ZHaKzpzkCqkM+TUk1lc=";
    installPhase = ''
      runHook preInstall
      mkdir -p "$out"
      cp -r dist "$out/dist"
      runHook postInstall
    '';
  };

  # Stage 2 · the Go module, with web/dist overlaid. `src` is git-tracked only
  # (web/node_modules, bin/, test artifacts are gitignored), so this copy is
  # clean; the one thing missing is build output, which is injected here.
  goSrc = runCommand "blacklight-src" { } ''
    cp -r ${src}/. "$out"
    chmod -R u+w "$out"
    rm -rf "$out/web/dist"
    cp -r "${web}/dist" "$out/web/dist"
  '';

  ldflags = [
    "-s"
    "-w"
    "-X"
    "github.com/bryanster/blacklight/internal/version.version=${blVersion}"
    "-X"
    "github.com/bryanster/blacklight/internal/version.commit=${blCommit}"
    "-X"
    "github.com/bryanster/blacklight/internal/version.buildDate=${blBuildDate}"
  ];
in
buildGoModule {
  pname = "blacklight";
  version = blVersion;
  src = goSrc;
  vendorHash = "sha256-QLaMiEXaBrJNzI2BtCcK0Hk0kOeTcsi7ZChMgwjZ68c=";
  subPackages = [ "cmd/blacklight" "cmd/blctl" ];
  # The spa tag selects web/dist_spa.go (the real embed) over the placeholder.
  # blctl does not import web, so the tag is inert for it (Makefile: `build`).
  tags = [ "spa" ];
  env = {
    CGO_ENABLED = "1";
  };
  # libstdc++ for the DuckDB static archive (its cgo LDFLAGS name -lstdc++).
  buildInputs = [ stdenv.cc.cc.lib ];
  inherit ldflags;
}
