#!/bin/sh
#
# Container entrypoint. It does exactly one thing before handing over: make sure
# the server has a session-signing secret.
#
# Why this exists at all. PURPLEOPS_SESSION_SECRET is required and deliberately
# has no default — internal/config rejects placeholders and short values rather
# than falling back to something guessable. That is the right behaviour for a
# process, and it is the wrong first impression for `docker compose up` on a
# clean clone, which is supposed to reach a login page in under a minute
# (PLAN.md §9). So: if the operator supplied a secret, it is used and nothing
# here runs. If they did not, one is generated from /dev/urandom, stored beside
# the database on the data volume, and reused on every later start — so
# restarting does not log everybody out. It is announced loudly, never silently.
#
# The generated secret lives only in the volume. Losing the volume, or moving to
# a second replica, means a secret nobody can reproduce — which is why
# docs/deploy.md tells you to set the variable yourself for anything real.

set -eu

log() { echo "purpleops-entrypoint: $*" >&2; }

die() {
	log "$*"
	exit 1
}

# ensure_session_secret exports PURPLEOPS_SESSION_SECRET, generating and
# persisting one if the environment does not already carry it.
ensure_session_secret() {
	if [ -n "${PURPLEOPS_SESSION_SECRET:-}" ]; then
		return
	fi

	# Beside the database rather than at a fixed path, so an operator who moved
	# PURPLEOPS_DB_PATH gets the secret on the same volume as the data it
	# belongs to instead of on the container's disposable filesystem.
	db_path="${PURPLEOPS_DB_PATH:-/var/lib/purpleops/purpleops.duckdb}"
	data_dir="$(dirname "$db_path")"
	secret_file="${data_dir}/session.secret"

	if [ -s "$secret_file" ]; then
		# Command substitution strips the trailing newline; the server trims
		# surrounding whitespace as well.
		PURPLEOPS_SESSION_SECRET="$(cat "$secret_file")"
		export PURPLEOPS_SESSION_SECRET
		return
	fi

	[ -d "$data_dir" ] || die "data directory $data_dir does not exist — mount a volume there (docs/deploy.md)"

	# 32 bytes is the floor internal/config enforces; base64 is what the
	# documentation tells operators to paste, so the generated value looks like
	# a hand-supplied one. Owner-read-only, and written through a temporary file
	# so a crash mid-write cannot leave a truncated secret behind.
	log "PURPLEOPS_SESSION_SECRET is not set — generating one and storing it at ${secret_file}."
	log "  Sessions will survive restarts, but not the loss of this volume, and it cannot be"
	log "  shared with a second instance. Set PURPLEOPS_SESSION_SECRET yourself in production:"
	log "  see docs/deploy.md."

	umask 077
	tmp="${secret_file}.tmp.$$"
	head -c 32 /dev/urandom | base64 -w 0 >"$tmp" 2>/dev/null ||
		die "could not write to $data_dir — the data volume must be writable by uid $(id -u) (docs/deploy.md)"
	mv "$tmp" "$secret_file" ||
		die "could not create $secret_file"

	PURPLEOPS_SESSION_SECRET="$(cat "$secret_file")"
	export PURPLEOPS_SESSION_SECRET
}

# Only when actually starting the server. `docker run <image> chromium
# --version`, a shell for debugging, or `purpleops --version` itself all go
# straight through and leave no secret behind — the flags that make the server
# print something and exit never reach the point where a secret is used.
case "${1:-}" in
purpleops | /usr/local/bin/purpleops)
	case " $* " in
	*" --version "* | *" -version "* | *" --help "* | *" -h "*) ;;
	*) ensure_session_secret ;;
	esac
	;;
esac

# exec, so the server inherits PID 1: SIGTERM from `docker stop` reaches it and
# starts the graceful shutdown rather than being absorbed by a shell.
exec "$@"
