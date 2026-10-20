#!/bin/sh
set -eu

WS="${COGS_WORKSPACE_ROOT:-/workspace}"
mkdir -p "$WS/apps/admin/data" "$WS/.cogs/run"

ENV_FILE="${COGS_ENV_FILE:-/config/admin.env}"
if [ ! -f "$ENV_FILE" ]; then
	ENV_FILE="/tmp/cogs-admin.env"
	: > "$ENV_FILE"
fi

if ! /usr/local/bin/cogs admin check --dir "$WS/apps/admin" 2>/dev/null; then
	if [ -z "${COGS_ADMIN_USER:-}" ] || [ -z "${COGS_ADMIN_PASSWORD:-}" ]; then
		echo "admin not initialized: set COGS_ADMIN_USER and COGS_ADMIN_PASSWORD" >&2
		exit 1
	fi
	/usr/local/bin/cogs admin init --dir "$WS/apps/admin" \
		--username "$COGS_ADMIN_USER" \
		--password "$COGS_ADMIN_PASSWORD"
fi

export COGS_SUPERVISOR_SOCKET="${COGS_SUPERVISOR_SOCKET:-/tmp/cogs-supervisor.sock}"
/usr/local/bin/cogs supervisor run --workspace "$WS" &
sleep 2

exec /usr/local/bin/cogs-admin serve -env "$ENV_FILE"
