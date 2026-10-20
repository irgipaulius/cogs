#!/bin/sh
set -eu

ENV_FILE="${COGS_ENV_FILE:-/config/.env}"
if [ ! -f "$ENV_FILE" ]; then
	ENV_FILE="/tmp/cogs-empty.env"
	: > "$ENV_FILE"
fi

/usr/local/bin/cogs-todo migrate -env "$ENV_FILE"
exec /usr/local/bin/cogs-todo serve -env "$ENV_FILE"
