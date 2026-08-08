#!/bin/sh
#
# Container entrypoint. It does exactly one thing before handing over: make sure
# the server has the two keys it cannot start without.
#
# Why this exists at all. BLACKLIGHT_SESSION_SECRET and BLACKLIGHT_ENCRYPTION_KEY
# are required and deliberately have no defaults — internal/config rejects
# placeholders and short values rather than falling back to something guessable.
# That is the right behaviour for a process, and it is the wrong first
# impression for `docker compose up` on a clean clone, which is supposed to
# reach a login page in under a minute (PLAN.md §9). So: if the operator
# supplied a value, it is used and nothing here runs. If they did not, one is
# generated from /dev/urandom, stored beside the database on the data volume,
# and reused on every later start — so restarting neither logs everybody out nor
# orphans an enrolled authenticator. It is announced loudly, never silently.
#
# The generated values live only in the volume. Losing the volume, or moving to
# a second replica, means keys nobody can reproduce — which is why
# docs/deploy.md tells you to set the variables yourself for anything real. It
# matters more for the encryption key than for the session secret: losing the
# first signs everybody out, losing the second means every enrolled
# authenticator has to be set up again.

set -eu

log() { echo "blacklight-entrypoint: $*" >&2; }

die() {
	log "$*"
	exit 1
}

# data_dir echoes the directory the database lives in, which is where a
# generated key is kept: an operator who moved BLACKLIGHT_DB_PATH gets the keys on
# the same volume as the data they belong to, instead of on the container's
# disposable filesystem.
data_dir() {
	db_path="${BLACKLIGHT_DB_PATH:-/var/lib/blacklight/blacklight.duckdb}"
	dirname "$db_path"
}

# ensure_key echoes the value of a required key: whatever the environment
# already carries, an existing file beside the database, or a freshly generated
# 32 bytes persisted there.
#
# $1 is the variable name, for the messages. $2 is the file. $3 is the sentence
# saying what losing this particular one costs, which is not the same for the
# two of them.
ensure_key() {
	name="$1"
	key_file="$2"
	consequence="$3"

	if [ -s "$key_file" ]; then
		# Command substitution strips the trailing newline; the server trims
		# surrounding whitespace as well.
		cat "$key_file"
		return
	fi

	dir="$(dirname "$key_file")"
	[ -d "$dir" ] || die "data directory $dir does not exist — mount a volume there (docs/deploy.md)"

	# 32 bytes is the floor internal/config enforces; base64 is what the
	# documentation tells operators to paste, so the generated value looks like
	# a hand-supplied one. Owner-read-only, and written through a temporary file
	# so a crash mid-write cannot leave a truncated key behind.
	log "$name is not set — generating one and storing it at ${key_file}."
	log "  $consequence"
	log "  It cannot be shared with a second instance. Set $name yourself in production:"
	log "  see docs/deploy.md."

	umask 077
	tmp="${key_file}.tmp.$$"
	head -c 32 /dev/urandom | base64 -w 0 >"$tmp" 2>/dev/null ||
		die "could not write to $dir — the data volume must be writable by uid $(id -u) (docs/deploy.md)"
	mv "$tmp" "$key_file" ||
		die "could not create $key_file"

	cat "$key_file"
}

# ensure_secrets exports both required keys, generating and persisting whichever
# the environment does not already carry.
ensure_secrets() {
	dir="$(data_dir)"

	if [ -z "${BLACKLIGHT_SESSION_SECRET:-}" ]; then
		BLACKLIGHT_SESSION_SECRET="$(ensure_key BLACKLIGHT_SESSION_SECRET "${dir}/session.secret" \
			"Sessions will survive restarts, but not the loss of this volume.")"
		export BLACKLIGHT_SESSION_SECRET
	fi

	if [ -z "${BLACKLIGHT_ENCRYPTION_KEY:-}" ]; then
		BLACKLIGHT_ENCRYPTION_KEY="$(ensure_key BLACKLIGHT_ENCRYPTION_KEY "${dir}/encryption.key" \
			"Losing this volume means every enrolled authenticator has to be set up again.")"
		export BLACKLIGHT_ENCRYPTION_KEY
	fi
}

# Only when actually starting the server. `docker run <image> chromium
# --version`, a shell for debugging, or `blacklight --version` itself all go
# straight through and leave no key behind — the flags that make the server
# print something and exit never reach the point where a secret is used.
case "${1:-}" in
blacklight | /usr/local/bin/blacklight)
	case " $* " in
	*" --version "* | *" -version "* | *" --help "* | *" -h "*) ;;
	*) ensure_secrets ;;
	esac
	;;
esac

# exec, so the server inherits PID 1: SIGTERM from `docker stop` reaches it and
# starts the graceful shutdown rather than being absorbed by a shell.
exec "$@"
