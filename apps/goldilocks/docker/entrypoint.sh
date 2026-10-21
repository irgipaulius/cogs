#!/bin/sh
set -eu

ENV_FILE="${COGS_ENV_FILE:-/config/.env}"
if [ ! -f "$ENV_FILE" ]; then
	ENV_FILE="/tmp/cogs-empty.env"
	: > "$ENV_FILE"
fi

LOG_DIR="${COGS_LOG_DIR:-/run/cogs}"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/app.log"
touch "$LOG_FILE"

/usr/local/bin/cogs-goldilocks migrate -env "$ENV_FILE"
exec /usr/local/bin/cogs-goldilocks serve -env "$ENV_FILE" >>"$LOG_FILE" 2>&1
