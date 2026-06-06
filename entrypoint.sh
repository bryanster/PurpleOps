#!/bin/sh
echo "Waiting for Mongo..."
_mongo_attempts=0
while ! nc -z "$MONGO_HOST" "${MONGO_PORT:-27017}"; do
    _mongo_attempts=$((_mongo_attempts + 1))
    if [ "$_mongo_attempts" -ge 60 ]; then
        echo "Error: MongoDB not reachable after 60 attempts (6s)" >&2
        exit 1
    fi
    sleep 0.1
done
./seed
exec "$@"
