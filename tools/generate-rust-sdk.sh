#!/usr/bin/env bash
#
# Regenerate sdk/rust from api/openapi.yaml.
#
# Run by `make sdk-rust`, which `make generate` depends on. Everything else in
# this repository generates with a tool pinned in go.mod, package-lock.json or
# uv.lock; Rust is the one language with no generator that reads OpenAPI 3.1
# natively, so it uses openapi-generator — which is Java, and stays inside a
# container pinned by digest rather than becoming a JDK every developer has to
# install.
#
# ── Why `docker cp` and not `-v` ─────────────────────────────────────────────
#
# The devcontainer runs docker-outside-of-docker: the daemon is the host's, so
# a bind mount of this repository's path resolves against the *host's*
# filesystem and silently mounts an empty directory. Copying into and out of a
# stopped container works the same way everywhere — in the devcontainer, on a
# developer's machine, and on a CI runner.
#
# ── What is generated and what is not ────────────────────────────────────────
#
# Only three paths are copied back out: src/apis, src/models and
# .openapi-generator. Everything else openapi-generator writes — Cargo.toml,
# src/lib.rs, README.md, docs/, git_push.sh, .travis.yml — stays in the
# container, because those files are written by hand here (see sdk/rust/README.md)
# and a generator that overwrote them would take the crate's metadata, its
# module list and its documentation with it.
#
# Deleting the two generated directories first is what makes a *removed*
# operation show up: without it, the file for an endpoint deleted from the
# document would sit in the tree forever, still compiling, and the drift gate in
# CI would see a clean checkout and agree.

set -euo pipefail

# Pinned by digest, not only by tag: a tag can be moved, and the codegen drift
# gate compares bytes. Upgrading means changing both halves here and committing
# the regenerated SDK in the same change.
IMAGE="openapitools/openapi-generator-cli:v7.24.0@sha256:5bf3dc75f764c584da8e3344c51b2f3f1e74703461d46a035b5ac1d31515cc88"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="${REPO_ROOT}/api/openapi.yaml"
OUT="${REPO_ROOT}/sdk/rust"

# packageName is the crate's, and has to agree with sdk/rust/Cargo.toml — the
# generated code refers to itself as `crate::`, but the api modules import
# `crate::models`, so a mismatch is a compile error rather than a silent one.
#
# No `useSingleRequestParameter`: it reads better for the operations with six
# parameters, and it emits `models::models::EvidenceSide` for the multipart
# upload, which does not compile. See the fix-up below — the same bug reaches
# the plain form too, but in two lines rather than several hundred.
PROPERTIES="packageName=blacklight"
PROPERTIES+=",packageVersion=0.0.0"
PROPERTIES+=",library=reqwest"
PROPERTIES+=",supportAsync=true"
PROPERTIES+=",topLevelApiClient=true"
# Models refer to each other directly rather than through Box, which is what
# makes `engagement.status` a field read rather than a deref. The document has
# no recursive schema, so there is no cycle for the indirection to break.
PROPERTIES+=",avoidBoxedModels=true"

if ! command -v docker >/dev/null 2>&1; then
	echo "generate-rust-sdk: docker is required (the generator is a pinned container image)" >&2
	exit 1
fi

# `make tools` calls this to fetch the image, so the digest lives in one place
# rather than in the Makefile as well. Generation itself then runs offline, like
# every other generator here.
if [[ "${1:-}" == "--pull" ]]; then
	exec docker pull "${IMAGE}"
fi

if [[ ! -f "${SPEC}" ]]; then
	echo "generate-rust-sdk: ${SPEC} not found" >&2
	exit 1
fi

container="$(docker create "${IMAGE}" \
	generate \
	--input-spec /openapi.yaml \
	--generator-name rust \
	--output /out \
	--additional-properties="${PROPERTIES}")"

cleanup() {
	docker rm --force "${container}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker cp "${SPEC}" "${container}:/openapi.yaml" >/dev/null

# The generator prints a warning per operation it cannot fully model — a wall of
# text on a successful run — so its output is held back and printed only if it
# fails. `start --attach` gives us the container's exit code, so a failure fails
# this script rather than producing half a crate.
if ! output="$(docker start --attach "${container}" 2>&1)"; then
	echo "${output}" >&2
	echo "generate-rust-sdk: the generator failed; sdk/rust is unchanged" >&2
	exit 1
fi

rm -rf "${OUT}/src/apis" "${OUT}/src/models" "${OUT}/.openapi-generator"
mkdir -p "${OUT}/src"

docker cp "${container}:/out/src/apis" "${OUT}/src/" >/dev/null
docker cp "${container}:/out/src/models" "${OUT}/src/" >/dev/null
docker cp "${container}:/out/.openapi-generator" "${OUT}/" >/dev/null

# openapi-generator emits `models::models::EvidenceSide` for a multipart form
# field whose schema is a $ref — the module prefix is applied twice. It is the
# only place the output does not compile, and `models::models::` is never a path
# this crate could legitimately contain, so rewriting it is unambiguous.
#
# Upstream: https://github.com/OpenAPITools/openapi-generator/issues/17510
# Delete this, and check, when a generator upgrade fixes it.
if grep -rl 'models::models::' "${OUT}/src" >/dev/null 2>&1; then
	grep -rl 'models::models::' "${OUT}/src" | xargs sed -i 's/models::models::/models::/g'
fi

echo "generate-rust-sdk: sdk/rust regenerated from api/openapi.yaml"
