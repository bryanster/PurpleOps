#!/usr/bin/env bash
#
# Container smoke test: build the image, run it, and prove the claims
# docs/deploy.md makes about it. CI runs this (M0B-012); so can you:
#
#     make docker-smoke
#     deploy/smoke.sh
#
# Every check is one line of output and one exit code. A failing check is not
# fatal to the run — the script keeps going so that one broken thing does not
# hide four others — and the summary at the end is the verdict.
#
# Environment:
#   IMAGE       image reference to build and test (default blacklight:smoke)
#   PLATFORM    --platform for the build, e.g. linux/arm64 (default: native)
#   SKIP_BUILD  non-empty to test an image that already exists
#   TIMEOUT     seconds to wait for a container to report healthy (default 180)

set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

IMAGE="${IMAGE:-blacklight:smoke}"
PLATFORM="${PLATFORM:-}"
SKIP_BUILD="${SKIP_BUILD:-}"
TIMEOUT="${TIMEOUT:-180}"

# Unique per run, so two smoke tests — or the leftovers of a killed one — cannot
# collide over a name.
RUN_ID="$$-${RANDOM}"
CONTAINER="blacklight-smoke-${RUN_ID}"
VOLUME="blacklight-smoke-${RUN_ID}"

failures=0
checks=0

pass() {
	checks=$((checks + 1))
	printf '  \033[32mok\033[0m   %s\n' "$*"
}

fail() {
	checks=$((checks + 1))
	failures=$((failures + 1))
	printf '  \033[31mFAIL\033[0m %s\n' "$*" >&2
}

info() { printf '\n\033[1m%s\033[0m\n' "$*"; }

# check <description> <expected-substring> <command...>
# Passes when the command succeeds and its output contains the substring; an
# empty substring asks only for success.
check() {
	local what="$1" want="$2" out status
	shift 2
	set +e
	out="$("$@" 2>&1)"
	status=$?
	set -e
	if [[ $status -ne 0 ]]; then
		fail "$what (exit $status)"
		printf '       %s\n' "${out:-<no output>}" >&2
		return
	fi
	if [[ -n $want && $out != *"$want"* ]]; then
		fail "$what — expected the output to contain '$want'"
		printf '       %s\n' "${out:-<no output>}" >&2
		return
	fi
	pass "$what"
}

# check_fails is for the assertions that are only worth anything as negatives:
# a health check that cannot report failure is decoration.
check_fails() {
	local what="$1"
	shift
	if "$@" >/dev/null 2>&1; then
		fail "$what"
	else
		pass "$what"
	fi
}

cleanup() {
	docker rm --force --volumes "$CONTAINER" >/dev/null 2>&1 || true
	docker volume rm --force "$VOLUME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# run_container starts the image detached on the shared data volume. No
# published port: every HTTP check runs inside the container, so the test cannot
# fail because the host happened to have 8080 busy. Extra arguments are passed
# to `docker run`.
run_container() {
	docker run --detach --name "$CONTAINER" \
		--init \
		${PLATFORM:+--platform "$PLATFORM"} \
		--volume "${VOLUME}:/var/lib/blacklight" \
		"$@" \
		"$IMAGE" >/dev/null
}

# wait_healthy blocks until the container's own HEALTHCHECK passes — the same
# signal compose and any orchestrator would use, rather than a second opinion
# invented here.
wait_healthy() {
	local deadline=$((SECONDS + TIMEOUT)) status
	while ((SECONDS < deadline)); do
		status="$(docker inspect --format '{{.State.Health.Status}}' "$CONTAINER" 2>/dev/null || echo missing)"
		if [[ $status == healthy ]]; then
			return 0
		fi
		if [[ $status == unhealthy ]]; then
			echo "container reported unhealthy" >&2
			return 1
		fi
		if [[ "$(docker inspect --format '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" == "false" ]]; then
			echo "container exited before becoming healthy" >&2
			return 1
		fi
		sleep 2
	done
	echo "timed out after ${TIMEOUT}s waiting for a healthy container" >&2
	return 1
}

# healthy_or_dump waits, records the result as a check, and prints the tail of
# the log when it did not work — a failing smoke test with no log is a support
# request rather than a diagnosis.
healthy_or_dump() {
	if wait_healthy; then
		pass "$1"
	else
		fail "$1"
		docker logs "$CONTAINER" 2>&1 | tail -40 >&2
	fi
}

in_container() { docker exec "$CONTAINER" "$@"; }
get() { in_container curl --fail --silent --show-error --max-time 10 "http://127.0.0.1:8080$1"; }

# ── build ─────────────────────────────────────────────────────────────────────

if [[ -z $SKIP_BUILD ]]; then
	info "Building ${IMAGE}"
	build_args=(
		--file "${REPO_ROOT}/deploy/Dockerfile"
		--tag "$IMAGE"
		--build-arg "VERSION=$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo v1-dev)"
		--build-arg "COMMIT=$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
		--build-arg "BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	)
	if [[ -n $PLATFORM ]]; then
		# --load so the result lands in the local image store rather than only in
		# the build cache, which is where a cross-platform buildx build stops.
		build_args+=(--platform "$PLATFORM" --load)
	fi
	docker build "${build_args[@]}" "$REPO_ROOT"
else
	info "Using the existing ${IMAGE}"
fi

size_mb=$(($(docker image inspect --format '{{.Size}}' "$IMAGE") / 1000 / 1000))

# ── first boot ────────────────────────────────────────────────────────────────
#
# On an empty volume and with no network, which is the claim worth proving:
# nothing is fetched at runtime. A packet capture would be the rigorous version;
# removing the network is the cheap one, and it fails loudly if anything ever
# starts reaching out on startup.

info "First boot — empty volume, no network"
run_container --network none
healthy_or_dump "the container reaches the healthy state with --network none"

check "runs as a non-root user" "" \
	in_container sh -c 'test "$(id -u)" -ne 0'
check "the data directory is writable by that user" "" \
	in_container test -w /var/lib/blacklight
check "the API answers /healthz with status ok" '"status":"ok"' get /api/v1/healthz
check "the API answers /version" '"version"' get /api/v1/version
check "the embedded SPA is served" 'id="root"' get /
check_fails "the SPA is the real build, not the no-frontend placeholder" \
	sh -c "docker exec $CONTAINER curl -fsS http://127.0.0.1:8080/ | grep -q 'not built'"

# ── the admin CLI ─────────────────────────────────────────────────────────────
#
# `docker exec` is what `docker compose exec blacklight …` does, so these run the
# CLI exactly the way docs/cli.md tells an operator to.
#
# The refusal is as much the point as the version: the server in this container
# holds the database, DuckDB admits one process — being in the same container
# does not change that — and an operator who runs a command anyway must get an
# instruction rather than a driver error about locks.
info "blctl, the admin CLI"
check "blctl is on PATH and reports the same build as the server" \
	"$(get /api/v1/version | sed 's/.*"version":"\([^"]*\)".*/\1/')" \
	in_container blctl version
check_fails "blctl refuses the database the running server holds" \
	in_container blctl db info
check "...and says which process to stop and how to run the command anyway" "docker compose run" \
	in_container sh -c 'blctl db info 2>&1 || true'

# ── Chromium ──────────────────────────────────────────────────────────────────

#
# One known false negative: run with PLATFORM set to an architecture the host is
# not, and this fails with "lacks support for the sse3 instruction set" —
# qemu-user does not advertise SSE3 and Chromium refuses to start without it.
# That is the emulator, not the image; on real hardware of either architecture
# it passes. Left as a failure rather than skipped, because a silent skip is how
# a real breakage gets through.
info "Chromium (M6 renders PDFs with it)"
check "BLACKLIGHT_CHROME_PATH points at an executable" "" \
	in_container sh -c 'test -x "$BLACKLIGHT_CHROME_PATH"'
check "chromium --version runs" "Chromium" \
	in_container sh -c 'exec "$BLACKLIGHT_CHROME_PATH" --version'

# ── the health check tells the truth ──────────────────────────────────────────
#
# `curl --fail` is what makes this work: /healthz answers 503 rather than 200
# when the database stops responding (TestHealthzReportsADeadDatabase covers
# that half), and --fail turns any status >= 400 into a non-zero exit. Here the
# probe runs in a container with no server at all, which is the same code path.

info "The health check"
check_fails "blacklight-healthcheck fails when the API does not answer" \
	docker run --rm --network none ${PLATFORM:+--platform "$PLATFORM"} \
	--entrypoint blacklight-healthcheck "$IMAGE"

# ── persistence ───────────────────────────────────────────────────────────────
#
# The equivalent of `docker compose down && docker compose up`: the container is
# destroyed, a new one starts on the same named volume, and what was written
# must still be there.

info "Persistence across a container replacement"
secret_before="$(in_container cat /var/lib/blacklight/session.secret)"
in_container sh -c 'echo persisted > /var/lib/blacklight/evidence/smoke.txt'

docker rm --force "$CONTAINER" >/dev/null
run_container
healthy_or_dump "a replacement container on the same volume is healthy"

check "the database file survived" "" in_container test -s /var/lib/blacklight/blacklight.duckdb
check "evidence/ survived" "persisted" in_container cat /var/lib/blacklight/evidence/smoke.txt
check "the generated session secret survived — nobody was logged out" "$secret_before" \
	in_container cat /var/lib/blacklight/session.secret

# ── summary ───────────────────────────────────────────────────────────────────

printf '\n'
if ((failures > 0)); then
	printf '\033[31m%d of %d checks failed\033[0m — image %s, %d MB\n' \
		"$failures" "$checks" "$IMAGE" "$size_mb" >&2
	exit 1
fi
printf '\033[32mall %d checks passed\033[0m — image %s, %d MB\n' "$checks" "$IMAGE" "$size_mb"
