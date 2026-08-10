# M7-004 — Release workflow (GHCR + GitHub Release + changelog)

**Milestone:** M7 · **Size:** L · **Depends on:** M0B-011, M0B-012

## Why

The supported artifact is the container image (`PLAN.md` §8). Without a tag-triggered publish path,
"shipping `v1.0.0`" is a laptop build and a hope. Operators need a pullable multi-arch image,
checksums, and a changelog entry that states what changed and that cutover is greenfield.

## Scope

**In**

- **`CHANGELOG.md`** at repo root (Keep a Changelog style is fine):
  - Unreleased section + **`[1.0.0]`** section prepared for ship (M7-009 fills final date/SHA).
  - Explicit **Upgrade notes** blurb: greenfield vs prior Mongo app; backup before migrate.
- **GitHub Actions workflow** (e.g. `.github/workflows/release.yml`) triggered on push of tags
  matching `v*` (or `v[0-9]*`):
  1. Checkout tagged commit.
  2. Run the same quality bar as CI **or** require CI green on the tagged SHA (document which).
  3. Build multi-arch image (`linux/amd64`, `linux/arm64`) with buildx, **reusing `deploy/Dockerfile`
     cross-compile approach** from M0B-011 (not QEMU `go build`).
  4. Stamp `VERSION`/`COMMIT`/`BUILD_DATE` build-args from the tag and git metadata.
  5. Push to **`ghcr.io/bryanster/blacklight`**:
     - `ghcr.io/bryanster/blacklight:<tag>` (e.g. `v1.0.0`)
     - `ghcr.io/bryanster/blacklight:latest` **only for non-prerelease tags**
  6. Generate checksums for any attached artifacts; at minimum document image digests in the release
     body.
  7. Create a **GitHub Release** for the tag with changelog section body.
- Permissions: least privilege (`contents: write` for release, `packages: write` for GHCR). Use
  `GITHUB_TOKEN`. **Pull requests must not publish.**
- Document operator pull/run in `docs/deploy.md` (image name, tag policy, how `latest` moves).
- Document maintainer release steps in `docs/contributing.md` (or a short `docs/releasing.md` if
  contributing is already long — one page max): tag annotated `vX.Y.Z` on main, push tag, watch
  workflow, verify GHCR + release.
- Optional: attach linux binaries to the Release **only if** cheap; Chromium/PDF will still need the
  image or a host Chrome. Default **image-first**; binaries are not the supported path.

**Out**

- Homebrew, apt repo, Kubernetes manifests.
- SBOM/signing (cosign) — nice follow-up, not v1.0.0 gate unless already trivial.
- Changing app version scheme away from `v1.0.0` (`M7-EPIC` locked).
- Tagging `v1-final` (M7-001) or `v1.0.0` ship decision (M7-009).

## Files

- `CHANGELOG.md`
- `.github/workflows/release.yml` (name flexible)
- `docs/deploy.md`, `docs/contributing.md` (and optional `docs/releasing.md`)
- Possibly `Makefile` helpers (`make release-dry-run` is nice, not required)

## Acceptance criteria

- [ ] Pushing an annotated tag on a clean test tag (or dry-run documented) would build and push
      multi-arch images; verified at least once against GHCR (can use a pre-release tag
      `v1.0.0-rc.1` before M7-009).
- [ ] Image runs: `docker run --rm ghcr.io/bryanster/blacklight:<tag> blacklight --version` prints
      the tag-stamped version.
- [ ] `latest` does not move on `rc` / `alpha` / `beta` tags.
- [ ] GitHub Release body includes changelog excerpt + image name + greenfield note.
- [ ] Fork PRs cannot push packages (workflow `on:` is tags only, or environment protection).
- [ ] `docs/deploy.md` shows pulling the released image, not only `compose build`.
- [ ] Completion notes record image digests for the verification tag.

## Tests

- Workflow validation: act/local dry-run optional; real proof is one pre-release tag publish.
- Existing `deploy/smoke.sh` against the published tag is the strongest check — run it once in
  completion notes (native amd64 runner for Chromium claims).

## Notes for the implementer

- Package name/org: `ghcr.io/bryanster/blacklight` matches `docs/deploy.md` examples.
- Reuse Dockerfile `BUILDPLATFORM` cross-compile; do not regress to emulated Go builds.
- CI already builds on `main` and `v2`; release workflow is additive.
- Never embed secrets in compose examples for GHCR public images.
