#!/bin/sh
#
# The image's HEALTHCHECK command.
#
# GET /api/v1/healthz from inside the container. The endpoint is public and
# unauthenticated by design (internal/httpapi/handlers.go): a health check that
# needs a session reports "unhealthy" exactly when authentication itself breaks,
# which is the one moment an orchestrator most needs the truth.
#
# The endpoint answers 200 when the database responds and 503 when it does not,
# with the same body either way. `curl -f` turns any status >= 400 into a
# non-zero exit, so a database that has gone away makes the container unhealthy
# without this script having to parse anything.

set -eu

# The listen address is configurable, so the port is read from it rather than
# hardcoded. ":8080", "0.0.0.0:8080" and "[::]:8080" all reduce to 8080.
addr="${BLACKLIGHT_ADDR:-:8080}"
port="${addr##*:}"

case "$port" in
0 | '' | *[!0-9]*)
	# Port 0 asks the kernel for a free port, which nothing outside the process
	# can discover — a deployment that does that cannot be health-checked, and
	# saying so beats reporting a healthy container that is not being probed.
	echo "healthcheck: cannot derive a port from BLACKLIGHT_ADDR=${addr}" >&2
	exit 1
	;;
esac

# --max-time under the HEALTHCHECK timeout, so a hung request is this script's
# failure rather than the daemon's kill.
exec curl --fail --silent --show-error --max-time 4 \
	--output /dev/null \
	"http://127.0.0.1:${port}/api/v1/healthz"
